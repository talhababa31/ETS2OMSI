package convert

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"

	"ets2omsi/internal/scene"
)

// Generated replacement textures for materials whose texture is not in the
// traffic package (typically shared ETS2 base.scs assets: glass, lamp
// atlases, wheel rims). The UV layout of the missing texture is unknown, so
// every generated texture is UV-independent: a uniform base with a fine,
// tileable surface pattern that reads correctly at any scale.

const (
	genGlass     = "glass"
	genHeadlight = "headlight"
	genTaillight = "taillight"
	genIndicator = "indicator"
	genChrome    = "chrome"
	genRim       = "rim"
	genTire      = "tire"
	genInterior  = "interior"
	genPlate     = "plate"
	genTrim      = "trim"
	genInvisible = "invisible"
	genPaint     = "" // handled by the paint-colour logic
)

func containsAnyWord(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

var solidColourWords = []struct {
	words []string
	c     color.NRGBA
}{
	{[]string{"black", "czarn", "noir", "schwarz"}, color.NRGBA{22, 22, 24, 255}},
	{[]string{"white", "blanc", "bial", "weiss"}, color.NRGBA{232, 232, 235, 255}},
	{[]string{"grey", "gray", "szary", "gris", "grau"}, color.NRGBA{118, 120, 124, 255}},
}

// generatedKind decides what a material without a package texture is, from
// its alias/effect/texture reference and, for lamps, its position.
// solid != nil means a plain colour named by the material itself.
func generatedKind(sc *scene.Scene, i int, ref string) (kind string, solid *color.NRGBA, alpha bool) {
	m := sc.Materials[i]
	sig := strings.ToLower(m.Alias + " " + m.Effect + " " + ref)
	sig = strings.ReplaceAll(sig, "lightmap", "")
	wheel := isWheelScene(sc)
	switch {
	case containsAnyWord(sig, "shadow", "occlusion", "_ao", "trucklight", "flare", "reflection", "env_"):
		return genInvisible, nil, true
	case containsAnyWord(sig, "glass", "window", "glas", "szyb", "vitre", "okno"):
		return genGlass, nil, true
	case containsAnyWord(sig, "blink", "indic", "turn", "clign", "migacz", "kierun", "orange", "amber", "signal"):
		return genIndicator, nil, false
	case containsAnyWord(sig, "tail", "rear_l", "back_l", "stop", "brake", "reverse", "zadn", "tyln", "arriere", "feu_ar"):
		return genTaillight, nil, false
	case containsAnyWord(sig, "head", "phare", "front_l", "przed", "scheinwerfer"):
		return genHeadlight, nil, false
	case containsAnyWord(sig, "lamp", "light", "lights", "reflector", "swiat", "feu", "far_", "lens"):
		if materialFrontness(sc, i) >= 0 {
			return genHeadlight, nil, false
		}
		return genTaillight, nil, false
	case containsAnyWord(sig, "tire", "tyre", "rubber", "opona", "pneu", "guma", "reifen"):
		return genTire, nil, false
	case containsAnyWord(sig, "rim", "wheel", "jante", "felg", "hub", "disc", "disk"):
		return genRim, nil, false
	case containsAnyWord(sig, "chrome", "chrom", "metal", "silver", "exhaust", "mirror_glass"):
		return genChrome, nil, false
	case containsAnyWord(sig, "interior", "inter", "dash", "seat", "tableau", "cockpit", "inside", "sitz", "plast"):
		return genInterior, nil, false
	case containsAnyWord(sig, "plate", "licen", "tablica", "plaque", "kennz"):
		return genPlate, nil, false
	}
	for _, sw := range solidColourWords {
		if containsAnyWord(sig, sw.words...) {
			c := sw.c
			return "", &c, false
		}
	}
	if wheel {
		if m.Class == "rubber" {
			return genTire, nil, false
		}
		return genRim, nil, false
	}
	switch m.Class {
	case "glass":
		return genGlass, nil, true
	case "light":
		if materialFrontness(sc, i) >= 0 {
			return genHeadlight, nil, false
		}
		return genTaillight, nil, false
	case "chrome":
		return genChrome, nil, false
	case "rubber":
		return genTire, nil, false
	}
	return genPaint, nil, false
}

// isWheelScene: a separate wheel model (small, roughly round, narrow).
func isWheelScene(sc *scene.Scene) bool {
	if len(sc.Vertices) == 0 {
		return false
	}
	b := sc.Bounds()
	return b.Height() < 1.3 && b.Length() < 1.3 && b.Width() < .7
}

// materialFrontness > 0 when the material sits in the front half of the car,
// < 0 in the rear half. Front is where the ETS2 front wheel locators are
// (wheel_f*), else +Y as in estimatedWheelPlacements.
func materialFrontness(sc *scene.Scene, mat int) float64 {
	front := 1.0
	var fy, ry float64
	var fn, rn int
	for _, p := range wheelPlacements(*sc) {
		if p.Family == "f" {
			fy += p.Pos.Y
			fn++
		} else if p.Family == "r" {
			ry += p.Pos.Y
			rn++
		}
	}
	if fn > 0 && rn > 0 && fy/float64(fn) < ry/float64(rn) {
		front = -1
	}
	b := sc.Bounds()
	mid := (b.Min.Y + b.Max.Y) / 2
	var sum float64
	n := 0
	for _, t := range sc.Triangles {
		if t.Material != mat {
			continue
		}
		for _, vi := range []int{t.A, t.B, t.C} {
			if vi >= 0 && vi < len(sc.Vertices) {
				sum += sc.Vertices[vi].Position.Y
				n++
			}
		}
	}
	if n == 0 {
		return 0
	}
	return (sum/float64(n) - mid) * front
}

func noise(x, y, seed int) float64 {
	h := uint32(x*374761393 + y*668265263 + seed*1442695041)
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h&0xffff)/65535.0*2 - 1
}

func shade(c color.NRGBA, f float64) color.NRGBA {
	s := func(v uint8) uint8 {
		x := float64(v) * f
		if x > 255 {
			x = 255
		}
		if x < 0 {
			x = 0
		}
		return uint8(x)
	}
	return color.NRGBA{s(c.R), s(c.G), s(c.B), c.A}
}

// renderGenerated draws a 128x128 tileable texture for kind.
func renderGenerated(kind string) *image.NRGBA {
	const n = 128
	im := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var c color.NRGBA
			switch kind {
			case genGlass:
				c = color.NRGBA{52, 64, 74, 165}
			case genInvisible:
				c = color.NRGBA{0, 0, 0, 0}
			case genHeadlight, genTaillight, genIndicator:
				// Lens with a tiled reflector cell pattern.
				base := map[string]color.NRGBA{
					genHeadlight: {226, 230, 236, 255},
					genTaillight: {178, 16, 20, 255},
					genIndicator: {236, 138, 22, 255},
				}[kind]
				cx, cy := float64(x%16)-7.5, float64(y%16)-7.5
				r := math.Sqrt(cx*cx+cy*cy) / 10.6
				f := 1.08 - .28*r + .04*math.Cos(r*9)
				c = shade(base, f)
			case genChrome:
				g := .82 + .18*math.Cos(float64(y)/n*2*math.Pi) + .03*noise(x, y, 1)
				c = shade(color.NRGBA{205, 208, 214, 255}, g)
			case genRim:
				// Brushed metal.
				c = shade(color.NRGBA{168, 171, 176, 255}, 1+.06*noise(x/8, y, 2))
			case genTire:
				c = shade(color.NRGBA{30, 30, 32, 255}, 1+.12*noise(x, y, 3))
			case genInterior:
				c = shade(color.NRGBA{40, 40, 43, 255}, 1+.08*noise(x, y, 4))
			case genPlate:
				c = color.NRGBA{226, 226, 218, 255}
			default: // genTrim
				c = shade(color.NRGBA{30, 30, 33, 255}, 1+.06*noise(x, y, 5))
			}
			im.SetNRGBA(x, y, c)
		}
	}
	return im
}

// writeGeneratedTexture writes gen_<kind>.dds once and returns its name.
func writeGeneratedTexture(texDir, kind string, tr *TextureReport) string {
	name := "gen_" + kind + ".dds"
	p := filepath.Join(texDir, name)
	if _, err := os.Stat(p); err == nil {
		return name
	}
	if err := os.WriteFile(p, encodeDDS(renderGenerated(kind)), 0644); err != nil {
		return ""
	}
	if tr != nil {
		tr.GeneratedFallbacks++
		tr.Files = append(tr.Files, name)
	}
	return name
}

func writeSolidTexture(texDir string, c color.NRGBA, tr *TextureReport) string {
	return writePaintFallback(texDir, c, tr)
}
