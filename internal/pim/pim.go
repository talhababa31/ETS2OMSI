package pim

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"ets2omsi/internal/pixtext"
	"ets2omsi/internal/scene"
)

type Options struct {
	// VisibleParts is a lower/upper-case insensitive map of PIM part names. When
	// non-nil, only pieces and locators belonging to visible parts are imported.
	VisibleParts map[string]bool
}

func ParseFile(p string) (scene.Scene, error) { return ParseFileWithOptions(p, Options{}) }
func ParseFileWithOptions(p string, opt Options) (scene.Scene, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return scene.Scene{}, err
	}
	s, err := ParseWithOptions(string(b), opt)
	s.Source = p
	return s, err
}
func Parse(text string) (scene.Scene, error) { return ParseWithOptions(text, Options{}) }
func ParseWithOptions(text string, opt Options) (scene.Scene, error) {
	secs, err := pixtext.Parse(text)
	if err != nil {
		return scene.Scene{}, err
	}
	out := scene.Scene{}
	mats := map[int]scene.Material{}
	nextMat := 0
	allowedPieces, allowedLocators := allowedPartIndices(secs, opt.VisibleParts)
	for _, sec := range secs {
		switch strings.ToLower(sec.Name) {
		case "material":
			idx := parseIntDefault(pixtext.First(sec, "Index"), nextMat)
			if idx >= nextMat {
				nextMat = idx + 1
			}
			mats[idx] = scene.Material{Index: idx, Alias: pixtext.First(sec, "Alias"), Effect: pixtext.First(sec, "Effect")}
		case "piece":
			idx := parseIntDefault(pixtext.First(sec, "Index"), -1)
			if allowedPieces != nil && !allowedPieces[idx] {
				continue
			}
			if err := parsePiece(sec, &out); err != nil {
				return scene.Scene{}, err
			}
		case "locator":
			idx := parseIntDefault(pixtext.First(sec, "Index"), -1)
			if allowedLocators != nil && !allowedLocators[idx] {
				continue
			}
			parseLocator(sec, &out)
		}
	}
	// Material numbers in PIX/PIM are slot IDs, not a dense ordinal list.
	// Legacy AI traffic models frequently have sparse slots (for example
	// 0, 2, 5, 9). Compressing them shifts triangle -> material bindings and
	// can put rear-light textures on front lights or body textures on rims.
	// Preserve the original slot index exactly.
	maxMat := -1
	for k := range mats {
		if k > maxMat {
			maxMat = k
		}
	}
	for _, tr := range out.Triangles {
		if tr.Material > maxMat {
			maxMat = tr.Material
		}
	}
	if maxMat >= 0 {
		out.Materials = make([]scene.Material, maxMat+1)
		for i := 0; i <= maxMat; i++ {
			out.Materials[i] = scene.Material{Index: i, Alias: fmt.Sprintf("material_%d", i)}
		}
		for k, m := range mats {
			if k >= 0 && k < len(out.Materials) {
				out.Materials[k] = m
			}
		}
	}
	return out, nil
}

func allowedPartIndices(secs []*pixtext.Section, visible map[string]bool) (map[int]bool, map[int]bool) {
	if visible == nil {
		return nil, nil
	}
	normalized := map[string]bool{}
	for k, v := range visible {
		normalized[strings.ToLower(strings.TrimSpace(k))] = v
	}
	pieces := map[int]bool{}
	locators := map[int]bool{}
	matched := false
	for _, sec := range secs {
		if !strings.EqualFold(sec.Name, "Part") {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(pixtext.First(sec, "Name")))
		if name == "" || !normalized[name] {
			continue
		}
		matched = true
		for _, n := range intsFromString(pixtext.First(sec, "Pieces")) {
			pieces[n] = true
		}
		for _, n := range intsFromString(pixtext.First(sec, "Locators")) {
			locators[n] = true
		}
	}
	if !matched {
		return nil, nil
	} // safer fallback for malformed/legacy PITs
	return pieces, locators
}

func intsFromString(s string) []int {
	r := strings.NewReplacer("(", " ", ")", " ", ";", " ", ",", " ")
	return parseInts(strings.Fields(r.Replace(s)))
}

var texcoord0RE = regexp.MustCompile(`(?i)_?TEXCOORD0(?:[^0-9]|$)`)

// baseUVStream returns the tag of the UV stream the base texture uses.
// ConverterPIX names UV streams "_UV<n>" and lists the shader texcoords they
// feed in "Aliases" ("_TEXCOORD0" ...). The base texture samples TEXCOORD0,
// which is not always stored in _UV0.
func baseUVStream(streams []*pixtext.Section) string {
	for _, st := range streams {
		tag := strings.ToUpper(strings.Trim(pixtext.First(st, "Tag"), "\"'"))
		if !strings.HasPrefix(tag, "_UV") && !strings.HasPrefix(tag, "UV") {
			continue
		}
		for k, v := range st.Props {
			if strings.EqualFold(k, "Aliases") && texcoord0RE.MatchString(strings.Join(v, " ")) {
				return tag
			}
		}
	}
	for _, st := range streams {
		tag := strings.ToUpper(strings.Trim(pixtext.First(st, "Tag"), "\"'"))
		switch tag {
		case "_UV0", "_TEXCOORD0", "TEXCOORD0", "UV0":
			return tag
		}
	}
	return "_UV0"
}

func parsePiece(sec *pixtext.Section, out *scene.Scene) error {
	mat := parseIntDefault(pixtext.First(sec, "Material"), 0)
	streams := pixtext.Children(sec, "Stream")
	pos := map[int]scene.Vec3{}
	norm := map[int]scene.Vec3{}
	uv := map[int]scene.Vec2{}
	baseUV := baseUVStream(streams)
	for _, st := range streams {
		tag := strings.ToUpper(strings.Trim(pixtext.First(st, "Tag"), "\"'"))
		if tag == "" {
			tag = strings.ToUpper(strings.Trim(pixtext.First(st, "Alias"), "\"'"))
		}
		for _, r := range st.Rows {
			vals := parseFloats(r.Values)
			switch tag {
			case "_POSITION", "POSITION":
				if len(vals) >= 3 {
					pos[r.Index] = scsVec(vals[0], vals[1], vals[2])
				}
			case "_NORMAL", "NORMAL":
				if len(vals) >= 3 {
					norm[r.Index] = scsVec(vals[0], vals[1], vals[2])
				}
			case baseUV:
				// ConverterPIX writes ETS2's DirectX-style UVs unchanged and
				// OMSI uses the same convention: no V flip. (Flipping read the
				// wrong atlas region, e.g. tail lights on the headlights.)
				if len(vals) >= 2 {
					uv[r.Index] = scene.Vec2{X: vals[0], Y: vals[1]}
				}
			}
		}
	}
	// Some PIX writers put data in a child named Vertices; streams remain authoritative.
	max := -1
	for i := range pos {
		if i > max {
			max = i
		}
	}
	if max < 0 {
		return nil
	}
	base := len(out.Vertices)
	for i := 0; i <= max; i++ {
		p, ok := pos[i]
		if !ok {
			p = scene.Vec3{}
		}
		n := norm[i]
		t := uv[i]
		out.Vertices = append(out.Vertices, scene.Vertex{Position: p, Normal: n, UV: t})
	}
	tris := pixtext.Children(sec, "Triangles")
	for _, ts := range tris {
		for _, r := range ts.Rows {
			iv := parseInts(r.Values)
			if len(iv) >= 3 {
				out.Triangles = append(out.Triangles, scene.Triangle{A: base + iv[0], B: base + iv[1], C: base + iv[2], Material: mat})
			}
		}
	}
	return nil
}
func parseLocator(sec *pixtext.Section, out *scene.Scene) {
	name := pixtext.First(sec, "Name")
	if name == "" {
		name = pixtext.First(sec, "Alias")
	}
	hook := pixtext.First(sec, "Hookup")
	vals := numbersFromString(pixtext.First(sec, "Position"))
	if len(vals) < 3 {
		vals = numbersFromString(pixtext.First(sec, "Translation"))
	}
	p := scene.Vec3{}
	if len(vals) >= 3 {
		p = scsVec(vals[0], vals[1], vals[2])
	}
	out.Locators = append(out.Locators, scene.Locator{Name: name, Hookup: hook, Position: p})
}
func scsVec(x, y, z float64) scene.Vec3 { return scene.Vec3{X: x, Y: -z, Z: y} }
func parseIntDefault(s string, d int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return d
}
func parseInts(in []string) []int {
	out := []int{}
	for _, s := range in {
		if n, err := strconv.Atoi(strings.Trim(s, "(),")); err == nil {
			out = append(out, n)
		}
	}
	return out
}
func parseFloats(in []string) []float64 {
	out := []float64{}
	for _, s := range in {
		if v, ok := parseFloat(s); ok {
			out = append(out, v)
		}
	}
	return out
}
func parseFloat(s string) (float64, bool) {
	s = strings.Trim(strings.TrimSpace(s), "(),")
	if s == "" {
		return 0, false
	}
	if strings.HasPrefix(s, "&") {
		h := strings.TrimPrefix(s, "&")
		if u, err := strconv.ParseUint(h, 16, 32); err == nil {
			return float64(math.Float32frombits(uint32(u))), true
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}
func numbersFromString(s string) []float64 {
	r := strings.NewReplacer("(", " ", ")", " ", ";", " ", ",", " ")
	return parseFloats(strings.Fields(r.Replace(s)))
}
