package convert

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"ets2omsi/internal/scene"
)

// Number plates. ETS2 draws the registration at run time, so a converted
// plate stays blank. Every plate material instead gets a generated plate
// texture with a random but fixed registration per vehicle and colour
// variant, and the plate triangles get planar UVs over each plate, because
// the ETS2 UV layout of the plate is unknown. OMSI's own route
// ([registration_free] + [texttexture] + [useTextTexture]) needs a plate font
// whose name could not be verified and a registrations.txt in every map, so
// the text is baked into the texture.

// Plate styles.
const (
	PlateTR           = "tr"   // Turkey: white, blue TR band, "34 ABC 123"
	PlateDE           = "de"   // Germany: white, blue EU band with D, "B AB 1234"
	PlateNone         = "none" // blank plate, as before
	DefaultPlateStyle = PlateTR
)

// PlateStyle is one plate style offered in the UI.
type PlateStyle struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Example string `json:"example,omitempty"`
}

const plateSampleSeed = "ETS2OMSI"

// PlateStyles lists the styles, each with the registration the sample image
// (PlateSamplePNG) shows.
var PlateStyles = func() []PlateStyle {
	out := []PlateStyle{{ID: PlateTR, Label: "Türkiye"}, {ID: PlateDE, Label: "Almanya"}, {ID: PlateNone, Label: "Boş"}}
	for i := range out {
		if out[i].ID != PlateNone {
			out[i].Example = plateText(out[i].ID, plateSampleSeed)
		}
	}
	return out
}()

// NormalizePlateStyle maps user input to a style id; "" and unknown values
// are the default (Turkish).
func NormalizePlateStyle(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case PlateDE, "german", "d":
		return PlateDE
	case PlateNone, "blank", "off":
		return PlateNone
	}
	return DefaultPlateStyle
}

// plateRand is a splitmix64 generator seeded by a string hash, so a vehicle
// always gets the same registration.
type plateRand struct{ s uint64 }

func newPlateRand(seed string) *plateRand {
	h := fnv.New64a()
	h.Write([]byte(seed))
	return &plateRand{s: h.Sum64()}
}

func (r *plateRand) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *plateRand) intn(n int) int { return int(r.next() % uint64(n)) }

func (r *plateRand) letters(set string, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = set[r.intn(len(set))]
	}
	return string(b)
}

// digits: n digits without a leading zero.
func (r *plateRand) digits(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('0' + r.intn(10))
	}
	if n > 0 {
		b[0] = byte('1' + r.intn(9))
	}
	return string(b)
}

const (
	trPlateLetters = "ABCDEFGHIJKLMNOPRSTUVYZ" // no Q, W, X and no Turkish diacritics
	dePlateLetters = "ABCDEFGHIJKLMNOPRSTUVWXYZ"
)

var (
	trBigProvinces = []int{34, 34, 6, 35, 16, 7, 1, 42, 27, 41}
	deDistricts    = []string{"B", "M", "HH", "K", "F", "S", "D", "DO", "E", "HB", "H", "L", "DD", "N", "BO", "WI", "MS", "KA", "MA", "A", "AC", "BI", "BN", "GE", "KI", "MZ", "HD", "FR", "OS", "OL", "PB", "KS", "LB", "ES", "RT", "GI", "SB", "TR", "RE", "BOR"}
	deBanned       = map[string]bool{"SS": true, "SA": true, "HJ": true, "KZ": true, "NS": true}
)

// plateText returns a realistic registration for style, the same for the
// same seed: "34 ABC 123" (Turkey) or "B AB 1234" (Germany). Only A-Z, 0-9
// and single spaces between the groups.
func plateText(style, seed string) string {
	r := newPlateRand(style + "|" + seed)
	if NormalizePlateStyle(style) == PlateDE {
		city := deDistricts[r.intn(len(deDistricts))]
		letters := ""
		for letters == "" || deBanned[letters] {
			letters = r.letters(dePlateLetters, 1+r.intn(2))
		}
		n := min(1+r.intn(4), 8-len(city)-len(letters)) // at most 8 characters
		return city + " " + letters + " " + r.digits(n)
	}
	p := 1 + r.intn(81)
	if r.intn(3) == 0 {
		p = trBigProvinces[r.intn(len(trBigProvinces))]
	}
	switch r.intn(3) {
	case 0:
		return fmt.Sprintf("%02d %s %s", p, r.letters(trPlateLetters, 1), r.digits(4))
	case 1:
		return fmt.Sprintf("%02d %s %s", p, r.letters(trPlateLetters, 2), r.digits(3+r.intn(2)))
	}
	return fmt.Sprintf("%02d %s %s", p, r.letters(trPlateLetters, 3), r.digits(2+r.intn(2)))
}

// Plate texture: 512x128, drawn in millimetres on an EU plate 110 mm high
// whose width follows the plate geometry.
const (
	plateTexW, plateTexH = 512, 128
	plateMMH             = 110.
	plateAspectEU        = 520. / 110
	plateSealRune        = '*' // layout slot of the German stickers
)

var (
	plateWhite  = color.NRGBA{246, 246, 241, 255}
	plateInk    = color.NRGBA{20, 20, 22, 255}
	plateBlue   = color.NRGBA{0, 51, 153, 255}
	plateYellow = color.NRGBA{255, 204, 0, 255}
	deHUColours = []color.NRGBA{{255, 128, 0, 255}, {0, 112, 192, 255}, {255, 214, 0, 255}, {150, 90, 40, 255}, {236, 120, 170, 255}, {40, 160, 70, 255}}
)

// plateCanvas draws anti-aliased shapes given in plate millimetres.
type plateCanvas struct {
	im     *image.NRGBA
	sx, sy float64 // pixels per millimetre
}

// fill blends c into every pixel of the box (x0,y0)-(x1,y1) by the share of
// its 4x4 sub-samples that inside reports.
func (pc *plateCanvas) fill(x0, y0, x1, y1 float64, c color.NRGBA, inside func(x, y float64) bool) {
	const ss = 4
	b := pc.im.Bounds()
	for py := max(0, int(y0*pc.sy)); py < min(b.Dy(), int(math.Ceil(y1*pc.sy))); py++ {
		for px := max(0, int(x0*pc.sx)); px < min(b.Dx(), int(math.Ceil(x1*pc.sx))); px++ {
			n := 0
			for j := 0; j < ss; j++ {
				y := (float64(py) + (float64(j)+.5)/ss) / pc.sy
				for i := 0; i < ss; i++ {
					if inside((float64(px)+(float64(i)+.5)/ss)/pc.sx, y) {
						n++
					}
				}
			}
			if n == 0 {
				continue
			}
			a := float64(n) / (ss * ss)
			o := pc.im.NRGBAAt(px, py)
			mix := func(d, s uint8) uint8 { return uint8(float64(d)*(1-a) + float64(s)*a + .5) }
			pc.im.SetNRGBA(px, py, color.NRGBA{mix(o.R, c.R), mix(o.G, c.G), mix(o.B, c.B), 255})
		}
	}
}

func inRoundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}

func (pc *plateCanvas) disc(cx, cy, r float64, c color.NRGBA) {
	pc.fill(cx-r, cy-r, cx+r, cy+r, c, func(x, y float64) bool { return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r })
}

// star draws a five-pointed star (outer radius r) with a point upwards.
func (pc *plateCanvas) star(cx, cy, r float64, c color.NRGBA) {
	ri := r * .382
	p2x, p2y := ri*math.Sin(math.Pi/5), ri*math.Cos(math.Pi/5)
	pc.fill(cx-r, cy-r, cx+r, cy+r, c, func(x, y float64) bool {
		dx, dy := x-cx, cy-y
		rho := math.Hypot(dx, dy)
		if rho > r {
			return false
		}
		t := math.Mod(math.Atan2(dx, dy)+2*math.Pi, 2*math.Pi/5)
		if t > math.Pi/5 {
			t = 2*math.Pi/5 - t
		}
		qx, qy := rho*math.Sin(t), rho*math.Cos(t)
		// inside when on the centre's side of the edge (0,r)-(p2x,p2y)
		return (p2x-0)*(qy-r)-(p2y-r)*(qx-0) <= 0
	})
}

// text draws s with cap height h centred on (cx, cy).
func (pc *plateCanvas) text(s string, cx, cy, h float64, c color.NRGBA) {
	m := newTextMetrics(h)
	x := cx - m.textWidth(s)/2
	for i, r := range s {
		if i > 0 {
			x += m.gap
		}
		pc.glyph(m, r, x, cy-h/2, 1, c)
		x += m.glyphWidth(r)
	}
}

func (pc *plateCanvas) glyph(m textMetrics, r rune, ox, oy, sx float64, c color.NRGBA) {
	if _, ok := plateFontGlyphs[r]; !ok {
		return
	}
	pc.fill(ox, oy, ox+m.glyphWidth(r)*sx, oy+m.h, c, func(x, y float64) bool { return m.glyphInk(r, ox, oy, sx, x, y) })
}

func clampPlateAspect(a float64) float64 {
	if a <= 0 || math.IsNaN(a) {
		return plateAspectEU
	}
	return math.Max(2.5, math.Min(7, a))
}

// renderPlate draws the plate texture for a plate whose width/height ratio is
// aspect. The text is condensed when it does not fit.
func renderPlate(style, text string, aspect float64) *image.NRGBA {
	style = NormalizePlateStyle(style)
	w, h := plateMMH*clampPlateAspect(aspect), plateMMH
	pc := &plateCanvas{im: image.NewNRGBA(image.Rect(0, 0, plateTexW, plateTexH)), sx: plateTexW / w, sy: plateTexH / h}
	for y := 0; y < plateTexH; y++ {
		for x := 0; x < plateTexW; x++ {
			pc.im.SetNRGBA(x, y, shade(plateWhite, 1+.012*noise(x, y, 7)))
		}
	}
	left := 7.
	if style != PlateNone {
		// Blue band on the left: EU stars + D, or TR.
		band := 42.
		pc.fill(0, 0, band, h, plateBlue, func(x, y float64) bool { return x <= band && inRoundRect(x, y, 3.5, 3.5, band+8, h-3.5, 5) })
		bx := (3.5 + band) / 2
		if style == PlateDE {
			for k := 0; k < 12; k++ {
				a := float64(k) * math.Pi / 6
				pc.star(bx+12.5*math.Sin(a), 34-12.5*math.Cos(a), 2.9, plateYellow)
			}
			pc.text("D", bx, 82, 25, plateWhite)
		} else {
			pc.text("TR", bx, 80, 24, plateWhite)
		}
		left = band + 5
	}
	// Black border.
	pc.fill(0, 0, w, h, plateInk, func(x, y float64) bool {
		return inRoundRect(x, y, 1.5, 1.5, w-1.5, h-1.5, 6) && !inRoundRect(x, y, 3.5, 3.5, w-3.5, h-3.5, 4.5)
	})
	if style == PlateNone || text == "" {
		return pc.im
	}
	m := newTextMetrics(75)
	runes := []rune(text)
	if style == PlateDE {
		if i := strings.IndexRune(text, ' '); i >= 0 {
			runes[i] = plateSealRune // the stickers sit between district and letters
		}
	}
	sealW := .42 * m.h
	adv := func(r rune) float64 {
		if r == plateSealRune {
			return sealW
		}
		return m.glyphWidth(r)
	}
	total := 0.
	for i, r := range runes {
		if i > 0 {
			total += m.gap
		}
		total += adv(r)
	}
	room := w - 6 - left
	sx := math.Min(1, room/total)
	x, oy := left+(room-total*sx)/2, (h-m.h)/2
	for i, r := range runes {
		if i > 0 {
			x += m.gap * sx
		}
		aw := adv(r) * sx
		if r == plateSealRune {
			rad := aw / 2
			hu := deHUColours[int(newPlateRand(text).next()%uint64(len(deHUColours)))]
			pc.disc(x+rad, h/2-rad-2, rad, shade(hu, .7))
			pc.disc(x+rad, h/2-rad-2, rad*.88, hu)
			pc.disc(x+rad, h/2-rad-2, rad*.3, plateWhite)
			pc.disc(x+rad, h/2+rad+2, rad, color.NRGBA{120, 120, 120, 255})
			pc.disc(x+rad, h/2+rad+2, rad*.88, color.NRGBA{226, 226, 220, 255})
			pc.disc(x+rad, h/2+rad+2, rad*.4, color.NRGBA{70, 70, 74, 255})
		} else {
			pc.glyph(m, r, x, oy, sx, plateInk)
		}
		x += aw
	}
	return pc.im
}

// PlateSamplePNG renders the example plate of a style for the settings UI.
func PlateSamplePNG(style string) ([]byte, error) {
	style = NormalizePlateStyle(style)
	text := ""
	if style != PlateNone {
		text = plateText(style, plateSampleSeed)
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, renderPlate(style, text, plateAspectEU))
	return buf.Bytes(), err
}

var (
	plateWords    = []string{"plate", "licen", "tablica", "plaque", "kennz", "plaka", "targa", "matric", "nummernschild", "kenteken"}
	plateNotWords = []string{"templat", "frame", "holder", "rahmen", "halter", "bracket", "screw", "bolt"}
)

// isPlateMaterial: the material's alias, effect, texture or PIT texture refs
// name a number plate, and it is not a lamp, glass, trim ... that merely
// mentions one (generatedKind decides that).
func isPlateMaterial(sc *scene.Scene, i int, refs string) bool {
	m := sc.Materials[i]
	sig := strings.ToLower(m.Alias + " " + m.Effect + " " + m.Texture + " " + m.BaseTexture + " " + refs)
	if containsAnyWord(sig, plateNotWords...) {
		return false
	}
	named := containsAnyWord(sig, plateWords...)
	for _, tok := range strings.FieldsFunc(sig, func(r rune) bool { return (r < 'a' || r > 'z') && (r < '0' || r > '9') }) {
		named = named || tok == "lp"
	}
	if !named {
		return false
	}
	k, _, _ := generatedKind(sc, i, refs)
	return k == genPlate || k == genPaint
}

// mapPlateUVs gives the plate triangles planar UVs: one rectangle per car end
// over that plate's X/Z extent, mirrored so the text reads left to right from
// outside (X right, Y forward, Z up: a plate at the +Y end is seen looking
// towards -Y, where right is -X). Vertices that other surfaces or the other
// plate share are copied first. It returns the width/height ratio of the
// largest plate, 0 when there is none.
func mapPlateUVs(sc *scene.Scene, mats map[int]bool) float64 {
	nv := len(sc.Vertices)
	ok := func(t scene.Triangle) bool {
		return t.A >= 0 && t.B >= 0 && t.C >= 0 && t.A < nv && t.B < nv && t.C < nv
	}
	b := sc.Bounds()
	mid := (b.Min.Y + b.Max.Y) / 2
	shared := make([]bool, nv)
	ends := map[int][]int{}
	for ti, t := range sc.Triangles {
		if !ok(t) {
			continue
		}
		if !mats[t.Material] {
			shared[t.A], shared[t.B], shared[t.C] = true, true, true
			continue
		}
		e := 1
		if sc.Vertices[t.A].Position.Y+sc.Vertices[t.B].Position.Y+sc.Vertices[t.C].Position.Y < 3*mid {
			e = -1
		}
		ends[e] = append(ends[e], ti)
	}
	owner := map[int]int{}
	best, bestArea := 0., 0.
	for _, e := range []int{1, -1} {
		if len(ends[e]) == 0 {
			continue
		}
		copies := map[int]int{}
		verts := []int{}
		for _, ti := range ends[e] {
			t := &sc.Triangles[ti]
			for _, vi := range []*int{&t.A, &t.B, &t.C} {
				if o, seen := owner[*vi]; (seen && o != e) || (*vi < nv && shared[*vi]) {
					c, done := copies[*vi]
					if !done {
						c = len(sc.Vertices)
						sc.Vertices = append(sc.Vertices, sc.Vertices[*vi])
						copies[*vi] = c
					}
					*vi = c
				}
				if _, seen := owner[*vi]; !seen {
					owner[*vi] = e
					verts = append(verts, *vi)
				}
			}
		}
		lo, hi := sc.Vertices[verts[0]].Position, sc.Vertices[verts[0]].Position
		for _, vi := range verts {
			p := sc.Vertices[vi].Position
			lo.X, lo.Z = math.Min(lo.X, p.X), math.Min(lo.Z, p.Z)
			hi.X, hi.Z = math.Max(hi.X, p.X), math.Max(hi.Z, p.Z)
		}
		w, h := hi.X-lo.X, hi.Z-lo.Z
		if w < 1e-4 || h < 1e-4 {
			continue
		}
		for _, vi := range verts {
			p := sc.Vertices[vi].Position
			u := (p.X - lo.X) / w
			if e > 0 {
				u = 1 - u
			}
			sc.Vertices[vi].UV = scene.Vec2{X: u, Y: (hi.Z - p.Z) / h}
		}
		if w*h > bestArea {
			best, bestArea = w/h, w*h
		}
	}
	return best
}

// plateMaker gives the plates of one vehicle their generated textures: one
// registration for the vehicle and one per colour variant.
type plateMaker struct {
	texDir, style, seed string
	aspect              float64
	mats                map[string]bool   // lower-case aliases of the plate materials
	names, texts        map[string]string // variant ("" = the vehicle) -> texture, registration
	files               []string
}

// newPlateMaker returns nil for PlateNone: plates then stay blank
// (gen_plate). All methods accept a nil maker.
func newPlateMaker(texDir, style, vehicleID string) *plateMaker {
	style = NormalizePlateStyle(style)
	if style == PlateNone {
		return nil
	}
	return &plateMaker{texDir: texDir, style: style, seed: vehicleID, mats: map[string]bool{}, names: map[string]string{}, texts: map[string]string{}}
}

func plateKey(alias string) string { return strings.ToLower(strings.TrimSpace(alias)) }

// prepare finds the plate materials of sc, maps their UVs and gives them the
// vehicle's plate texture. Call it after applyMaterials and before the safe
// fallbacks (which then leave the plates alone). It returns the number of
// plate materials. The first scene with plates (the body) sets the aspect.
func (p *plateMaker) prepare(sc *scene.Scene, hints map[string][]string) int {
	if p == nil {
		return 0
	}
	mats := map[int]bool{}
	for i, m := range sc.Materials {
		if isPlateMaterial(sc, i, strings.Join(hints[plateKey(m.Alias)], " ")) {
			mats[i] = true
			p.mats[plateKey(m.Alias)] = true
		}
	}
	if len(mats) == 0 {
		return 0
	}
	if a := mapPlateUVs(sc, mats); a > 0 && p.aspect == 0 {
		p.aspect = a
	}
	p.retexture(sc, "")
	return len(mats)
}

// retexture points the plate materials of sc at the plate texture of a
// colour variant ("" = the vehicle itself). UVs must already be mapped.
func (p *plateMaker) retexture(sc *scene.Scene, variant string) {
	if p == nil || len(p.mats) == 0 {
		return
	}
	for i := range sc.Materials {
		m := &sc.Materials[i]
		if !p.mats[plateKey(m.Alias)] {
			continue
		}
		n := p.texture(variant)
		if n == "" {
			return
		}
		m.Texture, m.BaseTexture, m.HasTint = n, "", false
	}
}

// texture writes (once) the plate texture of a variant and returns its name.
// The gen_ prefix keeps the paint recolouring away from it.
func (p *plateMaker) texture(variant string) string {
	if n, ok := p.names[variant]; ok {
		return n
	}
	text := plateText(p.style, p.seed+"|"+variant)
	name := "gen_plate_" + p.style + "_" + strings.ReplaceAll(text, " ", "") + ".dds"
	if err := os.WriteFile(filepath.Join(p.texDir, name), encodeDDS(renderPlate(p.style, text, p.aspect)), 0644); err != nil {
		name = ""
	} else {
		p.files = append(p.files, name)
	}
	p.names[variant], p.texts[variant] = name, text
	return name
}

// text is the registration of a variant whose texture was written.
func (p *plateMaker) text(variant string) string {
	if p == nil || p.names[variant] == "" {
		return ""
	}
	return p.texts[variant]
}

// handled drops the plate materials from applyMaterials' unresolved labels:
// their texture is generated, so they need no fallback or strict failure.
func (p *plateMaker) handled(unresolved []string) []string {
	if p == nil {
		return unresolved
	}
	out := []string{}
	for _, u := range unresolved {
		if !p.mats[plateKey(u)] {
			out = append(out, u)
		}
	}
	return out
}
