package convert

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"strings"

	"ets2omsi/internal/archive"
	"ets2omsi/internal/dds"
	"ets2omsi/internal/scene"
)

// Texture resolution, rewritten to be deterministic:
//
//  material alias --(PIT, selected look)--> texture object path (.tobj)
//    --(read .tobj from the package, ETS2 binary format)--> image path +
//    U/V addressing --(read image from the package)--> OMSI texture.
//
// Only exact paths are used, plus a same-directory match that ignores
// spaces/underscores (ConverterPIX drops spaces from names). Nothing is ever
// taken from another directory because its file name looks similar. Every
// material gets a MaterialDiag explaining where its texture came from or why
// it is missing.

// MaterialDiag explains the texture decision for one material.
type MaterialDiag struct {
	Model      string `json:"model,omitempty"`
	Alias      string `json:"alias"`
	Class      string `json:"class"`
	Ref        string `json:"ref,omitempty"`        // texture object the material asks for
	TOBJ       string `json:"tobj,omitempty"`       // .tobj found in the package
	Image      string `json:"image,omitempty"`      // image file found in the package
	Addressing string `json:"addressing,omitempty"` // ETS2 U/V addressing
	Output     string `json:"output,omitempty"`     // texture written for OMSI
	Status     string `json:"status"`               // ok | fallback | missing
	Reason     string `json:"reason"`
}

const (
	addrRepeat = iota
	addrClamp
	addrMirror
)

func addrName(a int) string {
	switch a {
	case addrClamp:
		return "clamp"
	case addrMirror:
		return "mirror"
	}
	return "repeat"
}

// ETS2 tobj_addr_t: 0 repeat, 1 clamp, 2 clamp_to_edge, 3 clamp_to_border,
// 4 mirror, 5 mirror_clamp, 6 mirror_clamp_to_edge.
func tobjAddr(b byte) int {
	switch {
	case b >= 1 && b <= 3:
		return addrClamp
	case b >= 4 && b <= 6:
		return addrMirror
	}
	return addrRepeat
}

const tobjMagic = 1890650625

type tobjInfo struct {
	image        string
	addrU, addrV int
}

// parseTOBJ reads the ETS2 texture object format (ConverterPIX
// structs/tobj.h): 40-byte header, then {u32 length, u32 unknown, path}.
func parseTOBJ(b []byte, tobjPath string) (tobjInfo, bool) {
	info := tobjInfo{}
	if len(b) >= 48 && binary.LittleEndian.Uint32(b) == tobjMagic {
		info.addrU, info.addrV = tobjAddr(b[30]), tobjAddr(b[31])
		n := int(binary.LittleEndian.Uint32(b[40:]))
		if n > 0 && 48+n <= len(b) {
			info.image = string(b[48 : 48+n])
		}
	}
	if info.image == "" {
		info.image = tobjTexturePath(b) // tolerant fallback for unknown versions
	}
	info.image = strings.TrimSpace(strings.ReplaceAll(info.image, "\\", "/"))
	if info.image == "" {
		return info, false
	}
	if !strings.HasPrefix(info.image, "/") {
		info.image = path.Join(path.Dir("/"+strings.TrimLeft(tobjPath, "/")), info.image)
	}
	return info, true
}

func tightName(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '_', '-', '\t':
			return -1
		}
		return r
	}, strings.ToLower(name))
}

type pkgFile struct {
	src  archive.Source
	path string
}

// packageFiles indexes the selected package(s): exact paths and, per
// directory, space/underscore-insensitive names.
type packageFiles struct {
	srcs  []archive.Source
	exact map[string]pkgFile
	loose map[string][]pkgFile // dir|tightname(with ext) -> files
}

func openPackageFiles(mounts []string) *packageFiles {
	pf := &packageFiles{exact: map[string]pkgFile{}, loose: map[string][]pkgFile{}}
	for i := len(mounts) - 1; i >= 0; i-- { // high -> low priority
		m := strings.TrimSpace(mounts[i])
		if m == "" {
			continue
		}
		src, err := archive.Open(m, archive.OpenOptions{})
		if err != nil {
			continue
		}
		pf.srcs = append(pf.srcs, src)
		seen := map[string]bool{}
		for _, fi := range src.List() {
			k := strings.ToLower(fi.Path)
			if _, ok := pf.exact[k]; ok {
				continue
			}
			f := pkgFile{src: src, path: fi.Path}
			pf.exact[k] = f
			lk := strings.ToLower(path.Dir(fi.Path)) + "|" + tightName(path.Base(fi.Path))
			if !seen[lk] {
				pf.loose[lk] = nil
				seen[lk] = true
			}
			pf.loose[lk] = append(pf.loose[lk], f)
		}
	}
	if len(pf.srcs) == 0 {
		return nil
	}
	return pf
}

func (pf *packageFiles) Close() {
	if pf == nil {
		return
	}
	for _, s := range pf.srcs {
		_ = s.Close()
	}
}

// find returns the file at p, or the unique file in the same directory whose
// name matches ignoring spaces/underscores/dashes and case.
func (pf *packageFiles) find(p string) (pkgFile, bool) {
	if pf == nil {
		return pkgFile{}, false
	}
	p = "/" + strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"), "/")
	if f, ok := pf.exact[strings.ToLower(p)]; ok {
		return f, true
	}
	c := pf.loose[strings.ToLower(path.Dir(p))+"|"+tightName(path.Base(p))]
	if len(c) == 1 {
		return c[0], true
	}
	return pkgFile{}, false
}

type resolvedTexture struct {
	diag       MaterialDiag
	output     string // file name in texDir ("" when not resolved)
	addrU      int
	addrV      int
	fromNative bool
}

// textureResolver turns PIT texture references into OMSI texture files.
type textureResolver struct {
	pkg       *packageFiles
	texDir    string
	byRef     map[string]resolvedTexture
	byImage   map[string]string // image path|addr -> output name
	usedNames map[string]bool
	// Secondary source, used only when the package cannot be read natively
	// (e.g. HashFS without helper): textures ConverterPIX exported.
	pixIndex map[string]string
}

func newTextureResolver(mounts []string, texDir string) *textureResolver {
	_ = os.MkdirAll(texDir, 0755)
	return &textureResolver{
		pkg:       openPackageFiles(mounts),
		texDir:    texDir,
		byRef:     map[string]resolvedTexture{},
		byImage:   map[string]string{},
		usedNames: map[string]bool{},
	}
}

func (r *textureResolver) Close() { r.pkg.Close() }

// Native reports whether the package itself could be read.
func (r *textureResolver) Native() bool { return r.pkg != nil }

// usePIXExports indexes textures ConverterPIX exported (copied into texDir).
func (r *textureResolver) usePIXExports(roots []string, tr *TextureReport) {
	r.pixIndex = map[string]string{}
	*tr = collectTextures(roots, r.texDir, r.pixIndex)
}

func stripTexExt(p string) string {
	if e := strings.ToLower(path.Ext(p)); e == ".tobj" || isTextureImageExt(e) {
		return strings.TrimSuffix(p, path.Ext(p))
	}
	return p
}

func (r *textureResolver) resolve(ref string) resolvedTexture {
	ref = strings.TrimSpace(strings.Trim(ref, "\"'"))
	if x, ok := r.byRef[ref]; ok {
		return x
	}
	res := resolvedTexture{diag: MaterialDiag{Ref: ref}}
	defer func() { r.byRef[ref] = res }()

	if r.pkg == nil {
		if r.pixIndex != nil {
			if v := lookupTexture(r.pixIndex, ref); v != "" {
				res.output = v
				res.diag.Output, res.diag.Status, res.diag.Reason = v, "ok", "ConverterPIX çıktısından alındı (paket doğrudan okunamadı)"
				return res
			}
		}
		res.diag.Status, res.diag.Reason = "missing", "paket doğrudan okunamadı ve ConverterPIX bu texture'ı çıkaramadı"
		return res
	}

	stem := stripTexExt(ref)
	var img pkgFile
	found := false
	if isTextureImageExt(path.Ext(ref)) {
		img, found = r.pkg.find(ref)
	}
	if !found {
		if t, ok := r.pkg.find(stem + ".tobj"); ok {
			res.diag.TOBJ = t.path
			if b, err := t.src.Read(t.path); err == nil {
				if info, ok := parseTOBJ(b, t.path); ok {
					res.addrU, res.addrV = info.addrU, info.addrV
					res.diag.Addressing = addrName(info.addrU) + "/" + addrName(info.addrV)
					if img, found = r.pkg.find(info.image); !found {
						res.diag.Status = "missing"
						res.diag.Reason = "tobj şu resmi istiyor ama pakette yok: " + info.image
						return res
					}
				}
			}
		}
	}
	if !found {
		// Image without a .tobj next to it.
		for _, e := range textureImageExts {
			if img, found = r.pkg.find(stem + e); found {
				break
			}
		}
	}
	if !found {
		res.diag.Status = "missing"
		res.diag.Reason = "pakette yok (büyük ihtimalle ETS2'nin kendi base.scs dosyasında): " + stem
		return res
	}
	res.diag.Image = img.path
	out, err := r.emit(img, res.addrU, res.addrV)
	if err != nil {
		res.diag.Status, res.diag.Reason = "missing", "resim okunamadı: "+err.Error()
		return res
	}
	res.output, res.fromNative = out, true
	res.diag.Output, res.diag.Status, res.diag.Reason = out, "ok", "paketten bulundu"
	return res
}

// emit writes the image for OMSI. Legacy DXT/RGB DDS with repeat addressing is
// copied byte-for-byte; anything else (DX10 header, TGA/PNG, mirror/clamp) is
// decoded and written as an uncompressed legacy DDS with mipmaps, a format
// OMSI's Direct3D 9 loader always accepts.
func (r *textureResolver) emit(f pkgFile, addrU, addrV int) (string, error) {
	key := strings.ToLower(f.path) + fmt.Sprintf("|%d%d", addrU, addrV)
	if n, ok := r.byImage[key]; ok {
		return n, nil
	}
	data, err := f.src.Read(f.path)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(path.Ext(f.path))
	base := asciiFileStem(strings.TrimSuffix(path.Base(f.path), path.Ext(f.path)))
	mirrorU, mirrorV := addrU == addrMirror, addrV == addrMirror
	var out []byte
	if ext == ".dds" && !mirrorU && !mirrorV && isLegacyDDS(data) {
		out = data
	} else {
		im, err := decodeImageBytes(data, ext)
		if err != nil {
			return "", err
		}
		if mirrorU || mirrorV {
			im = mirrorTile(im, mirrorU, mirrorV)
			base += "_mir"
		}
		out = encodeDDS(im)
	}
	name := base + ".dds"
	if r.usedNames[strings.ToLower(name)] {
		name = shortHash(key) + "_" + name
	}
	if err := os.WriteFile(filepath.Join(r.texDir, name), out, 0644); err != nil {
		return "", err
	}
	r.usedNames[strings.ToLower(name)] = true
	r.byImage[key] = name
	return name, nil
}

// prepareScene resolves every material's texture reference, rewrites mirrored
// and clamped UVs so OMSI's repeat addressing shows the same result, and
// returns an index usable by applyMaterials plus per-material diagnostics.
func (r *textureResolver) prepareScene(sc *scene.Scene, hints map[string][]string, model string) (map[string]string, []MaterialDiag) {
	index := map[string]string{}
	diags := []MaterialDiag{}
	for i := range sc.Materials {
		m := &sc.Materials[i]
		alias := strings.ToLower(strings.TrimSpace(m.Alias))
		refs := append([]string{}, hints[alias]...)
		if strings.Contains(m.Alias, "/") || strings.Contains(m.Alias, "\\") {
			refs = append(refs, m.Alias)
		}
		var chosen *resolvedTexture
		var first *resolvedTexture
		for _, ref := range uniqueStringsLocal(refs) {
			x := r.resolve(ref)
			if first == nil {
				first = &x
			}
			if x.output != "" {
				chosen = &x
				index[normalizeTextureKey(ref)] = x.output
				break
			}
		}
		d := MaterialDiag{Status: "fallback", Reason: "ETS2 materyali texture belirtmiyor"}
		if chosen != nil {
			d = chosen.diag
			transformMaterialUVs(sc, i, chosen.addrU, chosen.addrV)
		} else if first != nil {
			d = first.diag
		}
		d.Model, d.Alias, d.Class = model, m.Alias, materialClass(*m)
		diags = append(diags, d)
	}
	return index, diags
}

// transformMaterialUVs maps ETS2 mirror/clamp addressing onto OMSI repeat:
// mirror -> texture doubled with its mirror image and u' = u/2;
// clamp -> UV clamped to [0,1].
func transformMaterialUVs(sc *scene.Scene, mat, addrU, addrV int) {
	if addrU == addrRepeat && addrV == addrRepeat {
		return
	}
	done := map[int]bool{}
	fix := func(v float64, a int) float64 {
		switch a {
		case addrMirror:
			return v / 2
		case addrClamp:
			if v < 0 {
				return 0
			}
			if v > 1 {
				return 1
			}
		}
		return v
	}
	for _, t := range sc.Triangles {
		if t.Material != mat {
			continue
		}
		for _, vi := range []int{t.A, t.B, t.C} {
			if vi < 0 || vi >= len(sc.Vertices) || done[vi] {
				continue
			}
			done[vi] = true
			uv := &sc.Vertices[vi].UV
			uv.X, uv.Y = fix(uv.X, addrU), fix(uv.Y, addrV)
		}
	}
}

func mirrorTile(im image.Image, u, v bool) *image.NRGBA {
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	W, H := w, h
	if u {
		W = 2 * w
	}
	if v {
		H = 2 * h
	}
	out := image.NewNRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		sy := y
		if sy >= h {
			sy = 2*h - 1 - y
		}
		for x := 0; x < W; x++ {
			sx := x
			if sx >= w {
				sx = 2*w - 1 - x
			}
			out.Set(x, y, im.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return out
}

func isLegacyDDS(b []byte) bool {
	if len(b) < 128 || string(b[:4]) != "DDS " {
		return false
	}
	flags := binary.LittleEndian.Uint32(b[80:])
	if flags&0x4 == 0 {
		return true // uncompressed legacy RGB(A)
	}
	switch string(b[84:88]) {
	case "DXT1", "DXT3", "DXT5":
		return true
	}
	return false
}

func decodeImageBytes(b []byte, ext string) (image.Image, error) {
	switch ext {
	case ".dds":
		return dds.Decode(b)
	case ".tga":
		return decodeTGA(b)
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	return im, err
}

// encodeDDS writes an uncompressed A8R8G8B8 legacy DDS with a full mip chain.
func encodeDDS(im image.Image) []byte {
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	levels := []*image.NRGBA{toNRGBA(im)}
	for lw, lh := w, h; lw > 1 || lh > 1; {
		lw, lh = max1(lw/2), max1(lh/2)
		levels = append(levels, halve(levels[len(levels)-1], lw, lh))
	}
	var buf bytes.Buffer
	le := binary.LittleEndian
	hdr := make([]byte, 128)
	copy(hdr, "DDS ")
	le.PutUint32(hdr[4:], 124)
	le.PutUint32(hdr[8:], 0x1|0x2|0x4|0x8|0x1000|0x20000) // caps|height|width|pitch|pixelformat|mipmapcount
	le.PutUint32(hdr[12:], uint32(h))
	le.PutUint32(hdr[16:], uint32(w))
	le.PutUint32(hdr[20:], uint32(w*4))
	le.PutUint32(hdr[28:], uint32(len(levels)))
	le.PutUint32(hdr[76:], 32)
	le.PutUint32(hdr[80:], 0x40|0x1) // RGB | ALPHAPIXELS
	le.PutUint32(hdr[88:], 32)
	le.PutUint32(hdr[92:], 0x00ff0000)
	le.PutUint32(hdr[96:], 0x0000ff00)
	le.PutUint32(hdr[100:], 0x000000ff)
	le.PutUint32(hdr[104:], 0xff000000)
	le.PutUint32(hdr[108:], 0x1000|0x8|0x400000) // texture|complex|mipmap
	buf.Write(hdr)
	for _, l := range levels {
		for i := 0; i < len(l.Pix); i += 4 {
			buf.Write([]byte{l.Pix[i+2], l.Pix[i+1], l.Pix[i], l.Pix[i+3]})
		}
	}
	return buf.Bytes()
}

func max1(x int) int {
	if x < 1 {
		return 1
	}
	return x
}

func toNRGBA(im image.Image) *image.NRGBA {
	if n, ok := im.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := im.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.Set(x, y, im.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

func halve(src *image.NRGBA, w, h int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a, n int
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					sx, sy := x*2+dx, y*2+dy
					if sx >= sw || sy >= sh {
						continue
					}
					c := src.NRGBAAt(sx, sy)
					r, g, b, a, n = r+int(c.R), g+int(c.G), b+int(c.B), a+int(c.A), n+1
				}
			}
			if n > 0 {
				out.SetNRGBA(x, y, color.NRGBA{uint8(r / n), uint8(g / n), uint8(b / n), uint8(a / n)})
			}
		}
	}
	return out
}

// decodeTGA supports uncompressed and RLE true-colour/greyscale TGA.
func decodeTGA(b []byte) (image.Image, error) {
	if len(b) < 18 {
		return nil, fmt.Errorf("tga: too short")
	}
	idLen, cmapType, typ := int(b[0]), b[1], b[2]
	w, h := int(binary.LittleEndian.Uint16(b[12:])), int(binary.LittleEndian.Uint16(b[14:]))
	bpp, desc := int(b[16]), b[17]
	if cmapType != 0 || (typ != 2 && typ != 3 && typ != 10 && typ != 11) || w == 0 || h == 0 {
		return nil, fmt.Errorf("tga: unsupported type %d", typ)
	}
	ps := bpp / 8
	if ps != 1 && ps != 3 && ps != 4 {
		return nil, fmt.Errorf("tga: unsupported depth %d", bpp)
	}
	p := 18 + idLen
	px := make([]byte, 0, w*h*ps)
	if typ == 2 || typ == 3 {
		if p+w*h*ps > len(b) {
			return nil, fmt.Errorf("tga: truncated")
		}
		px = append(px, b[p:p+w*h*ps]...)
	} else {
		for len(px) < w*h*ps {
			if p >= len(b) {
				return nil, fmt.Errorf("tga: truncated rle")
			}
			hd := int(b[p])
			p++
			n := hd&0x7f + 1
			if hd&0x80 != 0 {
				if p+ps > len(b) {
					return nil, fmt.Errorf("tga: truncated rle")
				}
				for i := 0; i < n; i++ {
					px = append(px, b[p:p+ps]...)
				}
				p += ps
			} else {
				if p+n*ps > len(b) {
					return nil, fmt.Errorf("tga: truncated rle")
				}
				px = append(px, b[p:p+n*ps]...)
				p += n * ps
			}
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	top := desc&0x20 != 0
	for i := 0; i < w*h; i++ {
		x, y := i%w, i/w
		if !top {
			y = h - 1 - y
		}
		q := px[i*ps:]
		var c color.NRGBA
		switch ps {
		case 1:
			c = color.NRGBA{q[0], q[0], q[0], 255}
		case 3:
			c = color.NRGBA{q[2], q[1], q[0], 255}
		case 4:
			c = color.NRGBA{q[2], q[1], q[0], q[3]}
		}
		out.SetNRGBA(x, y, c)
	}
	return out, nil
}

var turkishASCII = strings.NewReplacer("ı", "i", "İ", "I", "ş", "s", "Ş", "S", "ğ", "g", "Ğ", "G", "ü", "u", "Ü", "U", "ö", "o", "Ö", "O", "ç", "c", "Ç", "C")

// asciiFileStem makes an OMSI-safe file name stem. The O3D stores texture
// names in the ANSI code page, so any other character would make OMSI look
// for a different file than the one on disk.
func asciiFileStem(s string) string {
	s = turkishASCII.Replace(s)
	b := strings.Builder{}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		out = "texture"
	}
	return out
}
