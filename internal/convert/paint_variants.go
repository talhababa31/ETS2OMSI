package convert

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"

	"ets2omsi/internal/o3d"
	"ets2omsi/internal/omsi"
	"ets2omsi/internal/pixtext"
	"ets2omsi/internal/scene"
)

// Colour variants so OMSI traffic is not all one colour. Two sources:
//   1. ETS2 looks: every extra look in the PIT (real colours of the mod).
//   2. Paint palette: the car's light, unsaturated paint texels are recoloured
//      with shading preserved; trims, lamps and glass keep their colours.
// Every variant becomes its own .ovh + model_<id>.cfg sharing the O3D files.

// PaintColor is one palette entry.
type PaintColor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hex   string `json:"hex"`
	rgb   color.NRGBA
}

func pc(id, label string, r, g, b uint8) PaintColor {
	return PaintColor{ID: id, Label: label, Hex: fmt.Sprintf("#%02x%02x%02x", r, g, b), rgb: color.NRGBA{r, g, b, 255}}
}

// PaintPalette: common traffic car colours.
var PaintPalette = []PaintColor{
	pc("beyaz", "Beyaz", 236, 236, 238),
	pc("siyah", "Siyah", 24, 25, 28),
	pc("gumus", "Gümüş", 176, 180, 186),
	pc("gri", "Füme / gri", 96, 100, 106),
	pc("lacivert", "Lacivert", 26, 40, 84),
	pc("mavi", "Mavi", 38, 92, 170),
	pc("kirmizi", "Kırmızı", 158, 22, 26),
	pc("bordo", "Bordo", 92, 18, 30),
	pc("yesil", "Koyu yeşil", 28, 68, 46),
	pc("bej", "Bej", 196, 182, 148),
	pc("kahve", "Kahverengi", 88, 60, 42),
	pc("sari", "Sarı", 226, 180, 30),
	pc("turuncu", "Turuncu", 214, 104, 28),
}

// DefaultPaintColors are used when the user picks nothing.
var DefaultPaintColors = []string{"siyah", "gumus", "gri", "lacivert", "kirmizi"}

func paintByID(id string) (PaintColor, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, p := range PaintPalette {
		if p.ID == id {
			return p, true
		}
	}
	return PaintColor{}, false
}

func lumSat(c color.NRGBA) (lum, sat float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	lum = .2126*r + .7152*g + .0722*b
	if mx > 0 {
		sat = (mx - mn) / mx
	}
	return
}

func smooth(e0, e1, x float64) float64 {
	t := (x - e0) / (e1 - e0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func hsv(c color.NRGBA) (h, sat, val float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	d := mx - mn
	val = mx
	if mx > 0 {
		sat = d / mx
	}
	switch {
	case d == 0:
		h = 0
	case mx == r:
		h = math.Mod((g-b)/d, 6) * 60
	case mx == g:
		h = ((b-r)/d + 2) * 60
	default:
		h = ((r-g)/d + 4) * 60
	}
	if h < 0 {
		h += 360
	}
	return
}

func hueDist(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// paintProfile is the measured paint colour of one material.
type paintProfile struct {
	c        color.NRGBA
	lum      float64
	hue, sat float64
	coverage float64 // share of the material's surface with this colour
}

// measurePaint finds the colour that covers the largest part of the
// material's SURFACE: the texture is sampled at the UV of every triangle and
// weighted by the triangle's 3D area, so interior/trim parts of an atlas do
// not outvote the body. Without triangles the whole image is used.
func measurePaint(im image.Image, sc *scene.Scene, mat int) (paintProfile, bool) {
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	type acc struct{ wt, r, g, bl float64 }
	hist := map[int]*acc{}
	total := 0.0
	add := func(c color.NRGBA, wt float64) {
		if c.A < 100 || wt <= 0 {
			return
		}
		k := int(c.R>>4)<<8 | int(c.G>>4)<<4 | int(c.B>>4)
		a := hist[k]
		if a == nil {
			a = &acc{}
			hist[k] = a
		}
		a.wt += wt
		a.r += float64(c.R) * wt
		a.g += float64(c.G) * wt
		a.bl += float64(c.B) * wt
		total += wt
	}
	sample := func(u, v float64) color.NRGBA {
		u, v = u-math.Floor(u), v-math.Floor(v)
		x, y := int(u*float64(w)), int(v*float64(h))
		if x >= w {
			x = w - 1
		}
		if y >= h {
			y = h - 1
		}
		return color.NRGBAModel.Convert(im.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
	}
	used := false
	if sc != nil {
		for _, t := range sc.Triangles {
			if t.Material != mat || t.A >= len(sc.Vertices) || t.B >= len(sc.Vertices) || t.C >= len(sc.Vertices) || t.A < 0 || t.B < 0 || t.C < 0 {
				continue
			}
			p, q, r := sc.Vertices[t.A], sc.Vertices[t.B], sc.Vertices[t.C]
			ux, uy, uz := q.Position.X-p.Position.X, q.Position.Y-p.Position.Y, q.Position.Z-p.Position.Z
			vx, vy, vz := r.Position.X-p.Position.X, r.Position.Y-p.Position.Y, r.Position.Z-p.Position.Z
			area := .5 * math.Sqrt(math.Pow(uy*vz-uz*vy, 2)+math.Pow(uz*vx-ux*vz, 2)+math.Pow(ux*vy-uy*vx, 2))
			// centroid + the three edge midpoints
			for _, wts := range [][3]float64{{1. / 3, 1. / 3, 1. / 3}, {.5, .5, 0}, {0, .5, .5}, {.5, 0, .5}} {
				u := p.UV.X*wts[0] + q.UV.X*wts[1] + r.UV.X*wts[2]
				v := p.UV.Y*wts[0] + q.UV.Y*wts[1] + r.UV.Y*wts[2]
				add(sample(u, v), area/4)
			}
			used = true
		}
	}
	if !used {
		step := 1
		if w*h > 512*512 {
			step = 2
		}
		for y := 0; y < h; y += step {
			for x := 0; x < w; x += step {
				add(color.NRGBAModel.Convert(im.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA), 1)
			}
		}
	}
	if total <= 0 {
		return paintProfile{}, false
	}
	var best *acc
	for _, a := range hist {
		if best == nil || a.wt > best.wt {
			best = a
		}
	}
	c := color.NRGBA{uint8(best.r / best.wt), uint8(best.g / best.wt), uint8(best.bl / best.wt), 255}
	l, _ := lumSat(c)
	hh, ss, _ := hsv(c)
	pp := paintProfile{c: c, lum: l, hue: hh, sat: ss}
	// coverage of everything close to that colour, not just one bucket
	for _, a := range hist {
		ac := color.NRGBA{uint8(a.r / a.wt), uint8(a.g / a.wt), uint8(a.bl / a.wt), 255}
		if paintWeight(ac, pp) > .5 {
			pp.coverage += a.wt
		}
	}
	pp.coverage /= total
	return pp, true
}

// paintWeight: how much a texel belongs to the measured paint (0..1).
func paintWeight(c color.NRGBA, p paintProfile) float64 {
	l, s := lumSat(c)
	hh, _, _ := hsv(c)
	pl := math.Max(p.lum, .04)
	rel := l / pl
	switch {
	case p.sat < .18 && p.lum < .16: // black / very dark paint
		return smooth(.30, .18, s) * smooth(p.lum*2.6+.12, p.lum*1.6+.06, l)
	case p.sat < .18: // white, silver, grey
		return smooth(.26, .14, s) * smooth(.32, .55, rel) * smooth(1.9, 1.5, rel)
	default: // coloured paint
		return smooth(30, 16, hueDist(hh, p.hue)) * smooth(p.sat*.3, p.sat*.6, s) * smooth(.22, .42, rel) * smooth(2.2, 1.7, rel)
	}
}

// recolorPaint maps paint texels to target, keeping each texel's shading
// relative to the measured paint colour.
func recolorPaint(im image.Image, p paintProfile, target color.NRGBA) *image.NRGBA {
	b := im.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	tr, tg, tb := float64(target.R), float64(target.G), float64(target.B)
	pl := math.Max(p.lum, .04)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(im.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			w := paintWeight(c, p)
			if w <= 0 {
				out.SetNRGBA(x, y, c)
				continue
			}
			l, _ := lumSat(c)
			shade := math.Min((l+.03)/(pl+.03), 1.4)
			nr, ng, nb := tr*shade, tg*shade, tb*shade
			if shade > 1 { // highlights go towards white
				hl := (shade - 1) / .4 * .5
				nr, ng, nb = nr+(255-nr)*hl, ng+(255-ng)*hl, nb+(255-nb)*hl
			}
			mix := func(a uint8, v float64) uint8 {
				x := float64(a)*(1-w) + v*w
				if x > 255 {
					x = 255
				}
				if x < 0 {
					x = 0
				}
				return uint8(x)
			}
			out.SetNRGBA(x, y, color.NRGBA{mix(c.R, nr), mix(c.G, ng), mix(c.B, nb), c.A})
		}
	}
	return out
}

// recolorer caches written textures per conversion.
type recolorer struct {
	texDir string
	out    map[string]string
	images map[string]image.Image
}

func newRecolorer(texDir string) *recolorer {
	return &recolorer{texDir: texDir, out: map[string]string{}, images: map[string]image.Image{}}
}

func (r *recolorer) load(tex string) image.Image {
	if im, ok := r.images[tex]; ok {
		return im
	}
	im, err := decodeTextureFile(filepath.Join(r.texDir, tex))
	if err != nil {
		im = nil
	}
	r.images[tex] = im
	return im
}

// minPaintCoverage: the measured colour must cover at least this share of
// the material's surface to count as its paint.
const minPaintCoverage = .30

// recolorMaterial returns the new texture for material i, or "" if it is not
// a paint material.
func (r *recolorer) recolorMaterial(sc *scene.Scene, i int, p PaintColor) string {
	m := sc.Materials[i]
	if m.Texture == "" || strings.HasPrefix(m.Texture, "gen_") {
		return ""
	}
	if strings.HasPrefix(m.Texture, "fallback_paint_") || strings.HasPrefix(m.Texture, "fallback_body") {
		return writePaintFallback(r.texDir, p.rgb, nil)
	}
	// ETS2 diffuse-tinted paint: re-tint the original grey texture.
	if m.BaseTexture != "" {
		key := m.BaseTexture + "|tint|" + p.ID
		if n, ok := r.out[key]; ok {
			return n
		}
		n := bakeTint(r.texDir, m.BaseTexture, [3]float64{float64(p.rgb.R) / 255, float64(p.rgb.G) / 255, float64(p.rgb.B) / 255})
		r.out[key] = n
		return n
	}
	im := r.load(m.Texture)
	if im == nil {
		return ""
	}
	prof, ok := measurePaint(im, sc, i)
	if !ok || prof.coverage < minPaintCoverage {
		return ""
	}
	key := fmt.Sprintf("%s|%02x%02x%02x|%s", m.Texture, prof.c.R, prof.c.G, prof.c.B, p.ID)
	if n, ok := r.out[key]; ok {
		return n
	}
	name := strings.TrimSuffix(m.Texture, filepath.Ext(m.Texture)) + "_" + p.ID + ".dds"
	if prev, used := r.out[name]; used && prev != key {
		name = shortHash(key) + "_" + name
	}
	if err := os.WriteFile(filepath.Join(r.texDir, name), encodeDDS(recolorPaint(im, prof, p.rgb)), 0644); err != nil {
		return ""
	}
	r.out[key], r.out[name] = name, key
	return name
}

// recolorScene changes the textures of the paint materials of a scene and
// reports whether anything was recoloured.
func (r *recolorer) recolorScene(sc *scene.Scene, p PaintColor) bool {
	changed := false
	for i := range sc.Materials {
		m := &sc.Materials[i]
		if !isPaintLikeClass(materialClass(*m)) || m.Alpha {
			continue
		}
		if k, _, _ := generatedKind(sc, i, ""); k != genPaint {
			continue // a known non-paint part (trim, interior, lamp ...)
		}
		if n := r.recolorMaterial(sc, i, p); n != "" && n != m.Texture {
			m.Texture = n
			m.HasTint = false
			changed = true
		}
	}
	return changed
}

// pitLookNames lists the look names in a PIT, in file order.
func pitLookNames(file string) []string {
	secs, err := pixtext.ParseFile(file)
	if err != nil {
		return nil
	}
	out := []string{}
	var walk func(*pixtext.Section)
	walk = func(s *pixtext.Section) {
		if strings.EqualFold(s.Name, "Look") {
			if n := strings.TrimSpace(pixtext.First(s, "Name")); n != "" {
				out = append(out, n)
			}
			return
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	for _, s := range secs {
		walk(s)
	}
	return uniqueStringsLocal(out)
}

// ColorVariant is one exported colour version of a vehicle.
type ColorVariant struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	OVH   string `json:"ovh"`
	Kind  string `json:"kind"` // look | paint
	Hex   string `json:"hex,omitempty"`
}

// colorVariantSet holds the recoloured scenes of one variant. OMSI's [matl]
// only SELECTS a material by the texture name stored in the O3D; it cannot
// swap textures. So every variant gets its own O3D files with its textures.
type colorVariantSet struct {
	variant ColorVariant
	body    *scene.Scene
	lods    []*scene.Scene
}

func sanitizeID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	b := strings.Builder{}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

// chosenLook mirrors pitLookMaterials: wanted look, else "default", else first.
func chosenLook(names []string, want string) string {
	for _, cand := range []string{strings.TrimSpace(want), "default"} {
		for _, n := range names {
			if cand != "" && strings.EqualFold(n, cand) {
				return n
			}
		}
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func sameOverrides(a, b []omsi.MaterialOverride) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Texture != b[i].Texture {
			return false
		}
	}
	return true
}

// exportColorVariants writes one .ovh + model cfg per extra ETS2 look and per
// palette colour. colors: palette ids (nil = DefaultPaintColors, ["none"] = no
// palette variants).
func exportColorVariants(stage string, spec omsi.VehicleSpec, converted []convertedModel, resolver *textureResolver, opaque *opaqueFixer, texDir string, colors []string) ([]ColorVariant, []string) {
	var sets []colorVariantSet
	warnings := []string{}
	if len(converted) == 0 {
		return nil, nil
	}
	lodOf := map[string]int{}
	for k, l := range spec.LODs {
		lodOf[l.File] = k
	}
	collect := func(v ColorVariant, mats func(i int, cm *convertedModel) scene.Scene) {
		set := colorVariantSet{variant: v, lods: make([]*scene.Scene, len(spec.LODs))}
		for i := range converted {
			cm := &converted[i]
			if i == 0 {
				sc := mats(i, cm)
				set.body = &sc
			} else if k, ok := lodOf[cm.o3dName]; ok {
				sc := mats(i, cm)
				set.lods[k] = &sc
			}
		}
		if set.body == nil || sameOverrides(materialOverrides(*set.body), spec.Materials) {
			return
		}
		sets = append(sets, set)
	}

	// 1. Extra ETS2 looks of the main model.
	looks := pitLookNames(converted[0].pix.PIT)
	current := chosenLook(looks, converted[0].look)
	for _, look := range looks {
		if strings.EqualFold(look, current) {
			continue
		}
		id := "look_" + sanitizeID(look)
		collect(ColorVariant{ID: id, Label: "ETS2 " + look, Kind: "look"}, func(i int, cm *convertedModel) scene.Scene {
			clone := cm.sc.Clone()
			for j := range clone.Materials {
				clone.Materials[j].Texture, clone.Materials[j].HasTint = "", false
			}
			hints := loadPITMaterials(&clone, cm.pix.PIT, look)
			idx, _ := resolver.prepareScene(&clone, hints, cm.o3dName)
			tr := TextureReport{}
			w := []string{}
			if un := applyMaterials(&clone, hints, idx, texDir, &tr, &w); len(un) > 0 {
				applySafeMaterialFallbacks(&clone, un, texDir, &tr, &w)
			}
			opaque.fixScene(&clone)
			return clone
		})
	}

	// 2. Paint palette.
	if !(len(colors) == 1 && strings.EqualFold(colors[0], "none")) {
		if colors == nil {
			colors = DefaultPaintColors
		}
		rc := newRecolorer(texDir)
		painted := 0
		for _, id := range uniqueStringsLocal(colors) {
			p, ok := paintByID(id)
			if !ok {
				continue
			}
			before := len(sets)
			collect(ColorVariant{ID: p.ID, Label: p.Label, Kind: "paint", Hex: p.Hex}, func(i int, cm *convertedModel) scene.Scene {
				clone := cm.sc
				clone.Materials = append([]scene.Material(nil), cm.sc.Materials...)
				rc.recolorScene(&clone, p)
				opaque.fixScene(&clone)
				return clone
			})
			if len(sets) > before {
				painted++
			}
		}
		if painted == 0 && len(colors) > 0 {
			warnings = append(warnings, "no recolourable paint texture found; palette colour variants were not created")
		}
	}

	out := []ColorVariant{}
	for _, set := range sets {
		id := set.variant.ID
		vs := spec
		vs.BodyFile = "body_" + id + ".o3d"
		if err := writeVariantO3D(filepath.Join(stage, "model", vs.BodyFile), *set.body); err != nil {
			warnings = append(warnings, "colour variant "+id+": "+err.Error())
			continue
		}
		vs.Materials = materialOverrides(*set.body)
		vs.LODs = append([]omsi.LOD(nil), spec.LODs...)
		for k := range vs.LODs {
			if set.lods[k] == nil {
				continue
			}
			f := strings.TrimSuffix(vs.LODs[k].File, ".o3d") + "_" + id + ".o3d"
			if err := writeVariantO3D(filepath.Join(stage, "model", f), *set.lods[k]); err != nil {
				continue // keep the shared LOD
			}
			vs.LODs[k].File = f
			vs.LODs[k].Materials = materialOverrides(*set.lods[k])
		}
		ovh, err := omsi.WriteVariant(stage, vs, id, set.variant.Label)
		if err != nil {
			warnings = append(warnings, "colour variant "+id+": "+err.Error())
			continue
		}
		set.variant.OVH = ovh
		out = append(out, set.variant)
	}
	return out, warnings
}

func writeVariantO3D(path string, sc scene.Scene) error {
	m := toO3D(sc)
	if err := o3d.WriteFile(path, &m); err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = o3d.Parse(b)
	return err
}
