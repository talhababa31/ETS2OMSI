package convert

import (
	"math"
	"strings"
)

// A small embedded bitmap font for generated number plates: A-Z, 0-9 and
// space on a 7-row grid ('#' = ink). Glyphs are not drawn as squares: every
// ink cell is a node and neighbouring nodes are joined by round-capped
// strokes, so diagonals come out smooth, close to the DIN 1451 style of real
// plates. Glyph width is the row length, which makes I, 1 and 0 narrower.

const fontRows = 7

var plateFontGlyphs = map[rune]string{
	'A': "..#../.#.#./#...#/#...#/#####/#...#/#...#",
	'B': "####./#...#/#...#/####./#...#/#...#/####.",
	'C': ".###./#...#/#..../#..../#..../#...#/.###.",
	'D': "###../#..#./#...#/#...#/#...#/#..#./###..",
	'E': "#####/#..../#..../####./#..../#..../#####",
	'F': "#####/#..../#..../####./#..../#..../#....",
	'G': ".###./#...#/#..../#.###/#...#/#...#/.###.",
	'H': "#...#/#...#/#...#/#####/#...#/#...#/#...#",
	'I': "#/#/#/#/#/#/#",
	'J': "....#/....#/....#/....#/....#/#...#/.###.",
	'K': "#...#/#..#./#.#../##.../#.#../#..#./#...#",
	'L': "#..../#..../#..../#..../#..../#..../#####",
	'M': "#...#/##.##/#.#.#/#.#.#/#...#/#...#/#...#",
	'N': "#...#/#...#/##..#/#.#.#/#..##/#...#/#...#",
	'O': ".###./#...#/#...#/#...#/#...#/#...#/.###.",
	'P': "####./#...#/#...#/####./#..../#..../#....",
	'Q': ".###./#...#/#...#/#...#/#.#.#/#..#./.##.#",
	'R': "####./#...#/#...#/####./#.#../#..#./#...#",
	'S': ".###./#...#/#..../.###./....#/#...#/.###.",
	'T': "#####/..#../..#../..#../..#../..#../..#..",
	'U': "#...#/#...#/#...#/#...#/#...#/#...#/.###.",
	'V': "#...#/#...#/#...#/#...#/.#.#./.#.#./..#..",
	'W': "#...#/#...#/#...#/#.#.#/#.#.#/##.##/#...#",
	'X': "#...#/#...#/.#.#./..#../.#.#./#...#/#...#",
	'Y': "#...#/#...#/.#.#./..#../..#../..#../..#..",
	'Z': "#####/....#/...#./..#../.#.../#..../#####",
	'0': ".##./#..#/#..#/#..#/#..#/#..#/.##.",
	'1': "..#/.##/#.#/..#/..#/..#/..#",
	'2': ".###./#...#/....#/...#./..#../.#.../#####",
	'3': ".###./#...#/....#/..##./....#/#...#/.###.",
	'4': "...#./..##./.#.#./#..#./#####/...#./...#.",
	'5': "#####/#..../####./....#/....#/#...#/.###.",
	'6': ".###./#..../#..../####./#...#/#...#/.###.",
	'7': "#####/....#/...#./..#../..#../..#../..#..",
	'8': ".###./#...#/#...#/.###./#...#/#...#/.###.",
	'9': ".###./#...#/#...#/.####/....#/....#/.###.",
}

// glyphCells returns the ink grid of r (nil for space or an unknown rune).
func glyphCells(r rune) [][]bool {
	s, ok := plateFontGlyphs[r]
	if !ok {
		return nil
	}
	rows := strings.Split(s, "/")
	out := make([][]bool, len(rows))
	for y, row := range rows {
		out[y] = make([]bool, len(row))
		for x, c := range row {
			out[y][x] = c == '#'
		}
	}
	return out
}

// glyphColumns is the width of r in grid cells (5 for space).
func glyphColumns(r rune) int {
	if g := glyphCells(r); len(g) > 0 {
		return len(g[0])
	}
	return 5
}

// plateGlyphStrokes caches glyphStrokes for every glyph of the font.
var plateGlyphStrokes = func() map[rune][][4]float64 {
	out := map[rune][][4]float64{}
	for r := range plateFontGlyphs {
		out[r] = buildGlyphStrokes(r)
	}
	return out
}()

// buildGlyphStrokes returns the stroke centre lines of r in grid units
// (column, row) as x0,y0,x1,y1. Orthogonal neighbours are always joined;
// diagonal neighbours only when neither cell between them is ink, which keeps
// inner corners (E, F, L) square. A lone node is a zero-length stroke (a dot).
func buildGlyphStrokes(r rune) [][4]float64 {
	g := glyphCells(r)
	on := func(x, y int) bool { return y >= 0 && y < len(g) && x >= 0 && x < len(g[y]) && g[y][x] }
	segs := [][4]float64{}
	for y := range g {
		for x := range g[y] {
			if !g[y][x] {
				continue
			}
			joined := false
			add := func(x1, y1 int) {
				segs = append(segs, [4]float64{float64(x), float64(y), float64(x1), float64(y1)})
				joined = true
			}
			if on(x+1, y) {
				add(x+1, y)
			}
			if on(x, y+1) {
				add(x, y+1)
			}
			if on(x+1, y+1) && !on(x+1, y) && !on(x, y+1) {
				add(x+1, y+1)
			}
			if on(x-1, y+1) && !on(x-1, y) && !on(x, y+1) {
				add(x-1, y+1)
			}
			if !joined && !on(x-1, y) && !on(x, y-1) && !on(x-1, y-1) && !on(x+1, y-1) {
				add(x, y)
			}
		}
	}
	return segs
}

// textMetrics sizes the font for a cap height h (any unit): stroke width,
// node pitch and the gap between letters.
type textMetrics struct {
	h, stroke, pitchX, pitchY, gap, space float64
}

func newTextMetrics(h float64) textMetrics {
	s := .14 * h
	return textMetrics{h: h, stroke: s, pitchX: (.6*h - s) / 4, pitchY: (h - s) / (fontRows - 1), gap: .1 * h, space: .32 * h}
}

// width of one glyph without the following gap.
func (m textMetrics) glyphWidth(r rune) float64 {
	if r == ' ' {
		return m.space - m.gap
	}
	return float64(glyphColumns(r)-1)*m.pitchX + m.stroke
}

// textWidth of a whole string, gaps included.
func (m textMetrics) textWidth(s string) float64 {
	w := 0.
	for i, r := range s {
		if i > 0 {
			w += m.gap
		}
		w += m.glyphWidth(r)
	}
	return w
}

// glyphInk reports whether point (x, y) is inside glyph r drawn with its top
// left corner at (ox, oy); sx condenses the glyph horizontally (1 = normal).
func (m textMetrics) glyphInk(r rune, ox, oy, sx, x, y float64) bool {
	px := (x-ox)/sx - m.stroke/2
	py := y - oy - m.stroke/2
	rad := m.stroke / 2
	for _, s := range plateGlyphStrokes[r] {
		ax, ay := s[0]*m.pitchX, s[1]*m.pitchY
		bx, by := s[2]*m.pitchX, s[3]*m.pitchY
		if segDist2(px, py, ax, ay, bx, by) <= rad*rad {
			return true
		}
	}
	return false
}

// segDist2 is the squared distance from (px,py) to segment a-b.
func segDist2(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := 0.
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l))
	}
	ex, ey := px-ax-t*dx, py-ay-t*dy
	return ex*ex + ey*ey
}
