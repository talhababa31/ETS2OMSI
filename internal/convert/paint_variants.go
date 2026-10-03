package convert

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"

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

// paintBase reports whether a texture's dominant colour is a recolourable
// paint base (white / silver / light grey) and returns its luminance.
func paintBase(im image.Image) (float64, bool) {
	dom, ok := dominantColor(im)
	if !ok {
		return 0, false
	}
	l, s := lumSat(dom)
	return l, s < .2 && l > .45
}

// recolorImage maps paint texels to target, keeping their shading relative to
// the dominant paint luminance. Dark, saturated or transparent texels stay.
func recolorImage(im image.Image, baseLum float64, target color.NRGBA) *image.NRGBA {
	b := im.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	tr, tg, tb := float64(target.R), float64(target.G), float64(target.B)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(im.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			l, s := lumSat(c)
			rel := l / baseLum
			w := smooth(.24, .12, s) * smooth(.38, .62, rel)
			if w <= 0 {
				out.SetNRGBA(x, y, c)
				continue
			}
			shade := math.Min(rel, 1.35)
			nr, ng, nb := tr*shade, tg*shade, tb*shade
			if shade > 1 { // highlights go towards white
				h := (shade - 1) / .35
				nr, ng, nb = nr+(255-nr)*h*.5, ng+(255-ng)*h*.5, nb+(255-nb)*h*.5
			}
			mix := func(a uint8, v float64) uint8 {
				x := float64(a)*(1-w) + v*w
				if x > 255 {
					x = 255
				}
				return uint8(x)
			}
			out.SetNRGBA(x, y, color.NRGBA{mix(c.R, nr), mix(c.G, ng), mix(c.B, nb), c.A})
		}
	}
	return out
}

// recolorer caches paint analysis and written textures per conversion.
type recolorer struct {
	texDir string
	base   map[string]float64 // texture -> base luminance (absent = not paint)
	seen   map[string]bool
	out    map[string]string
}

func newRecolorer(texDir string) *recolorer {
	return &recolorer{texDir: texDir, base: map[string]float64{}, seen: map[string]bool{}, out: map[string]string{}}
}

func (r *recolorer) texture(tex string, p PaintColor) string {
	if tex == "" || strings.HasPrefix(tex, "gen_") || strings.HasPrefix(tex, "optional_") || strings.HasPrefix(tex, "semantic_") {
		return tex
	}
	if strings.HasPrefix(tex, "fallback_paint_") || tex == "fallback_body.png" {
		if n := writePaintFallback(r.texDir, p.rgb, nil); n != "" {
			return n
		}
		return tex
	}
	key := tex + "|" + p.ID
	if n, ok := r.out[key]; ok {
		return n
	}
	if !r.seen[tex] {
		r.seen[tex] = true
		if im, err := decodeTextureFile(filepath.Join(r.texDir, tex)); err == nil {
			if l, ok := paintBase(im); ok {
				r.base[tex] = l
			}
		}
	}
	l, ok := r.base[tex]
	if !ok {
		r.out[key] = tex
		return tex
	}
	im, err := decodeTextureFile(filepath.Join(r.texDir, tex))
	if err != nil {
		r.out[key] = tex
		return tex
	}
	name := strings.TrimSuffix(tex, filepath.Ext(tex)) + "_" + p.ID + ".dds"
	if err := os.WriteFile(filepath.Join(r.texDir, name), encodeDDS(recolorImage(im, l, p.rgb)), 0644); err != nil {
		r.out[key] = tex
		return tex
	}
	r.out[key] = name
	return name
}

// recolorScene changes the textures of the paint materials of a scene.
// It reports whether anything was recoloured.
func (r *recolorer) recolorScene(sc *scene.Scene, p PaintColor) bool {
	changed := false
	for i := range sc.Materials {
		m := &sc.Materials[i]
		if !isPaintLikeClass(materialClass(*m)) || m.Alpha {
			continue
		}
		if k, _, _ := generatedKind(sc, i, ""); k != genPaint {
			if !strings.HasPrefix(m.Texture, "fallback_paint_") && m.Texture != "fallback_body.png" {
				continue // a known non-paint part (trim, interior, lamp ...)
			}
		}
		if n := r.texture(m.Texture, p); n != m.Texture {
			m.Texture = n
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
}

// colorVariantSet describes material overrides of one variant.
type colorVariantSet struct {
	variant ColorVariant
	body    []omsi.MaterialOverride
	lods    [][]omsi.MaterialOverride
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
	collect := func(v ColorVariant, mats func(i int, cm *convertedModel) []omsi.MaterialOverride) {
		set := colorVariantSet{variant: v, lods: make([][]omsi.MaterialOverride, len(spec.LODs))}
		for i := range converted {
			cm := &converted[i]
			if i == 0 {
				set.body = mats(i, cm)
			} else if k, ok := lodOf[cm.o3dName]; ok {
				set.lods[k] = mats(i, cm)
			}
		}
		if set.body == nil || sameOverrides(set.body, spec.Materials) {
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
		collect(ColorVariant{ID: id, Label: "ETS2 " + look, Kind: "look"}, func(i int, cm *convertedModel) []omsi.MaterialOverride {
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
			return materialOverrides(clone)
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
			collect(ColorVariant{ID: p.ID, Label: p.Label, Kind: "paint"}, func(i int, cm *convertedModel) []omsi.MaterialOverride {
				clone := cm.sc
				clone.Materials = append([]scene.Material(nil), cm.sc.Materials...)
				rc.recolorScene(&clone, p)
				opaque.fixScene(&clone)
				return materialOverrides(clone)
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
		vs := spec
		vs.Materials = set.body
		vs.LODs = append([]omsi.LOD(nil), spec.LODs...)
		for k := range vs.LODs {
			if set.lods[k] != nil {
				vs.LODs[k].Materials = set.lods[k]
			}
		}
		ovh, err := omsi.WriteVariant(stage, vs, set.variant.ID, set.variant.Label)
		if err != nil {
			warnings = append(warnings, "colour variant "+set.variant.ID+": "+err.Error())
			continue
		}
		set.variant.OVH = ovh
		out = append(out, set.variant)
	}
	return out, warnings
}
