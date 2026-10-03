package convert

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"ets2omsi/internal/archive"
)

// Native package texture resolver.
//
// ConverterPIX silently skips texture files whose name contains a space
// ("tableau de bord.dds") and only exports images that have a .tobj next to
// them. PIT files also reference textures without an extension. To avoid
// losing those package-local textures, the converter now reads the selected
// package itself and extracts the referenced image directly.

var textureImageExts = []string{".dds", ".tga", ".png", ".jpg", ".jpeg", ".bmp"}

func isTextureImageExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range textureImageExts {
		if e == ext {
			return true
		}
	}
	return false
}

func textureExtRank(ext string) int {
	ext = strings.ToLower(ext)
	for i, e := range textureImageExts {
		if e == ext {
			return i
		}
	}
	return len(textureImageExts)
}

// textureTightKey normalizes a virtual texture path so that names differing
// only in spaces/underscores/dashes/dots and extension compare equal:
// "/vehicle/x/Tableau de bord.dds" == "vehicle/x/tableaudebord".
func textureTightKey(p string) string {
	p = normalizeTextureKey(p)
	if ext := path.Ext(p); ext != "" && (isTextureImageExt(ext) || ext == ".tobj" || ext == ".mat") {
		p = strings.TrimSuffix(p, ext)
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '_', '-', '.', '\t':
			return -1
		}
		return r
	}, p)
}

func textureTightBase(p string) string {
	k := textureTightKey(p)
	if i := strings.LastIndex(k, "/"); i >= 0 {
		return k[i+1:]
	}
	return k
}

type pkgEntry struct {
	src  archive.Source
	path string // archive path, normalized with leading slash
}

type packageTextureIndex struct {
	exact    map[string]pkgEntry // lowercase normalized path (with ext) -> entry
	pathTigt map[string]pkgEntry // tight path key -> entry (ambiguous => src nil)
	baseTigt map[string]pkgEntry // tight base key -> entry (ambiguous => src nil)
	tobj     map[string]pkgEntry // tight path key of .tobj -> entry
}

func newPackageTextureIndex() *packageTextureIndex {
	return &packageTextureIndex{exact: map[string]pkgEntry{}, pathTigt: map[string]pkgEntry{}, baseTigt: map[string]pkgEntry{}, tobj: map[string]pkgEntry{}}
}

// add indexes one mount. Mounts must be added from highest to lowest priority;
// the first mount that provides a key wins.
func (ix *packageTextureIndex) add(src archive.Source) {
	pathSeen := map[string]bool{}
	baseSeen := map[string]bool{}
	for _, fi := range src.List() {
		p := fi.Path
		ext := strings.ToLower(path.Ext(p))
		e := pkgEntry{src: src, path: p}
		if ext == ".tobj" {
			k := textureTightKey(p)
			if _, ok := ix.tobj[k]; !ok {
				ix.tobj[k] = e
			}
			continue
		}
		if !isTextureImageExt(ext) {
			continue
		}
		lk := normalizeTextureKey(p)
		if _, ok := ix.exact[lk]; !ok {
			ix.exact[lk] = e
		}
		addTight(ix.pathTigt, pathSeen, textureTightKey(p), e)
		addTight(ix.baseTigt, baseSeen, textureTightBase(p), e)
	}
}

// addTight records e under key. Within one mount, two different files sharing
// a key are ambiguous unless one has a strictly preferred image extension
// (same stem, .dds beats .tga etc.). A key already owned by a higher-priority
// mount is never replaced.
func addTight(m map[string]pkgEntry, seenThisMount map[string]bool, key string, e pkgEntry) {
	if key == "" {
		return
	}
	old, ok := m[key]
	if ok && !seenThisMount[key] {
		return // higher-priority mount owns it
	}
	seenThisMount[key] = true
	if !ok {
		m[key] = e
		return
	}
	if old.src == nil {
		return
	}
	if strings.EqualFold(old.path, e.path) {
		return
	}
	oldStem := strings.TrimSuffix(normalizeTextureKey(old.path), path.Ext(old.path))
	newStem := strings.TrimSuffix(normalizeTextureKey(e.path), path.Ext(e.path))
	if oldStem == newStem {
		if textureExtRank(path.Ext(e.path)) < textureExtRank(path.Ext(old.path)) {
			m[key] = e
		}
		return
	}
	m[key] = pkgEntry{} // ambiguous
}

var tobjImagePath = regexp.MustCompile(`(?i)[\x20-\x7e]+?\.(dds|tga|png|jpe?g|bmp)`)

// tobjTexturePath returns the image path embedded in a binary/text TOBJ.
func tobjTexturePath(data []byte) string {
	for _, m := range tobjImagePath.FindAll(data, -1) {
		s := string(bytes.TrimSpace(m))
		if i := strings.Index(s, "/"); i >= 0 {
			s = s[i:]
		}
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// lookup resolves one PIT texture reference to an image inside the package.
func (ix *packageTextureIndex) lookup(ref string) (pkgEntry, bool) {
	ref = strings.TrimSpace(strings.Trim(ref, "\"'"))
	if ref == "" {
		return pkgEntry{}, false
	}
	// 1. exact image path.
	if isTextureImageExt(path.Ext(ref)) {
		if e, ok := ix.exact[normalizeTextureKey(ref)]; ok {
			return e, true
		}
	}
	// 2. the TOBJ that the reference names: follow its embedded image path.
	if t, ok := ix.tobj[textureTightKey(ref)]; ok && t.src != nil {
		if data, err := t.src.Read(t.path); err == nil {
			if img := tobjTexturePath(data); img != "" {
				if !strings.HasPrefix(img, "/") {
					img = path.Join(path.Dir(t.path), img)
				}
				if e, ok := ix.exact[normalizeTextureKey(img)]; ok {
					return e, true
				}
				if e, ok := ix.pathTigt[textureTightKey(img)]; ok && e.src != nil {
					return e, true
				}
			}
		}
	}
	// 3. same virtual path ignoring spaces/underscores/extension.
	if e, ok := ix.pathTigt[textureTightKey(ref)]; ok && e.src != nil {
		return e, true
	}
	// 4. globally unique base name only (ambiguous names are rejected).
	if strings.Contains(strings.Trim(normalizeTextureKey(ref), "/"), "/") {
		if e, ok := ix.baseTigt[textureTightBase(ref)]; ok && e.src != nil {
			return e, true
		}
	}
	return pkgEntry{}, false
}

// textureOutputPath builds a space-free destination for ref under out, keeping
// the reference's virtual directory so collectTextures can match it exactly.
func textureOutputPath(out, ref, entryPath string) string {
	r := normalizeTextureKey(ref)
	if ext := path.Ext(r); ext != "" && (isTextureImageExt(ext) || ext == ".tobj" || ext == ".mat") {
		r = strings.TrimSuffix(r, ext)
	}
	dir, stem := path.Dir(r), path.Base(r)
	stem = strings.ReplaceAll(stem, " ", "_")
	if stem == "" || stem == "." || stem == "/" {
		stem = strings.ReplaceAll(strings.TrimSuffix(path.Base(entryPath), path.Ext(entryPath)), " ", "_")
	}
	dir = strings.ReplaceAll(dir, " ", "_")
	if dir == "." {
		dir = ""
	}
	return filepath.Join(out, filepath.FromSlash(dir), stem+strings.ToLower(path.Ext(entryPath)))
}

// resolvePackageTextures extracts every PIT texture reference that can be found
// in the mounted package directly, without ConverterPIX. It returns the number
// of extracted references and the set of references it handled. Mounts that
// cannot be opened natively (e.g. HashFS without helper) are skipped silently,
// leaving the old ConverterPIX path in charge.
func resolvePackageTextures(mounts []string, refs []string, out string) (int, map[string]bool) {
	handled := map[string]bool{}
	if len(refs) == 0 {
		return 0, handled
	}
	ix := newPackageTextureIndex()
	opened := []archive.Source{}
	defer func() {
		for _, s := range opened {
			_ = s.Close()
		}
	}()
	// MountPaths are low -> high priority; index high -> low.
	for i := len(mounts) - 1; i >= 0; i-- {
		m := strings.TrimSpace(mounts[i])
		if m == "" {
			continue
		}
		src, err := archive.Open(m, archive.OpenOptions{})
		if err != nil {
			continue
		}
		opened = append(opened, src)
		ix.add(src)
	}
	if len(opened) == 0 {
		return 0, handled
	}
	count := 0
	for _, ref := range refs {
		e, ok := ix.lookup(ref)
		if !ok {
			continue
		}
		data, err := e.src.Read(e.path)
		if err != nil || len(data) == 0 {
			continue
		}
		dst := textureOutputPath(out, ref, e.path)
		if old, err := os.ReadFile(dst); err == nil && !bytes.Equal(old, data) {
			dst = filepath.Join(filepath.Dir(dst), shortHash(e.path)+"_"+filepath.Base(dst))
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			continue
		}
		handled[ref] = true
		count++
	}
	return count, handled
}
