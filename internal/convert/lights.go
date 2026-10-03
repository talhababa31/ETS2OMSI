package convert

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"ets2omsi/internal/omsi"
	"ets2omsi/internal/scene"
)

// Vehicle lights. ETS2 light locators give the lamp positions when the model
// has them; otherwise the lamp materials do: each lamp material is split per
// car end and side, and the lit face of every such cluster becomes an OMSI
// [light_enh_2]. Positions stay in the internal frame (X right, Y forward,
// Z up) after the centring/ground translation, which is also the model.cfg
// vehicle frame of the wheel origin_trans lines; toO3D swaps only the mesh
// vertices to Y up / Z forward.

const lampAny = "lamp" // a lamp whose function follows from where it sits

// lampEndZone: lamps sit in the outer part of each car end (share of the
// length from the middle); roof, interior and mirror lights do not.
const lampEndZone = .25

var (
	lampNotWords = []string{"shadow", "occlusion", "_ao", "trucklight", "reflection", "env_", "interior", "dash", "cockpit", "inside", "seat", "headrest", "headliner", "plate", "licen", "kennz", "fog", "beacon", "strobe", "siren", "taxi", "highlight", "gauge", "display", "wheel", "paint", "light_gr", "lightgr", "light_blue", "lightblue", "light_brown", "light_green", "light_beige"}
	lampWords    = []string{"light", "lamp", "flare", "lens", "reflector", "blink", "indic", "winker", "clign", "brake", "brems", "revers", "rueckfahr", "scheinwerfer", "phare", "feu_", "swiat"}
	lampFnWords  = []struct {
		fn    string
		words []string
	}{
		{omsi.LightBlinker, []string{"blink", "indic", "turn", "winker", "clign", "migacz", "kierun", "flasher", "orange", "amber", "signal"}},
		{omsi.LightReverse, []string{"revers", "backup", "back_up", "rueckfahr", "cofan", "recul"}},
		{omsi.LightBrake, []string{"brake", "stop", "brems", "frein"}},
		{omsi.LightTail, []string{"tail", "rear", "back", "zadn", "tyln", "arriere", "heck"}},
		{omsi.LightHead, []string{"head", "phare", "front", "przed", "scheinwerfer", "beam", "drl", "daytime"}},
	}
	lightOrder = map[string]int{omsi.LightHead: 0, omsi.LightTail: 1, omsi.LightBrake: 2, omsi.LightBlinker: 3, omsi.LightReverse: 4}
)

// lampFunction classifies a material (alias + effect) or locator (name +
// hookup): "" = no lamp, a light function the name states, or lampAny.
func lampFunction(sig string) string {
	sig = strings.ReplaceAll(strings.ToLower(sig), "lightmap", "")
	if containsAnyWord(sig, lampNotWords...) || !containsAnyWord(sig, lampWords...) {
		return ""
	}
	for _, f := range lampFnWords {
		if containsAnyWord(sig, f.words...) {
			return f.fn
		}
	}
	return lampAny
}

// frontSign is +1 when the car's front is +Y (the default, as in
// estimatedWheelPlacements) and -1 when the ETS2 front wheel locators
// (wheel_f*) sit behind the rear ones.
func frontSign(sc *scene.Scene) float64 {
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
		return -1
	}
	return 1
}

// lampCluster is the part of one lamp material at one car end and side.
type lampCluster struct {
	front, left bool
	sum         scene.Vec3 // area-weighted triangle centres
	w           float64
	min, max    scene.Vec3
}

func newLampCluster(front, left bool) *lampCluster {
	in := math.Inf(1)
	return &lampCluster{front: front, left: left, min: scene.Vec3{X: in, Y: in, Z: in}, max: scene.Vec3{X: -in, Y: -in, Z: -in}}
}

func (c *lampCluster) add(w float64, p, q, r scene.Vec3) {
	for _, v := range []scene.Vec3{p, q, r} {
		c.min = scene.Vec3{X: math.Min(c.min.X, v.X), Y: math.Min(c.min.Y, v.Y), Z: math.Min(c.min.Z, v.Z)}
		c.max = scene.Vec3{X: math.Max(c.max.X, v.X), Y: math.Max(c.max.Y, v.Y), Z: math.Max(c.max.Z, v.Z)}
	}
	c.sum = scene.Vec3{X: c.sum.X + (p.X+q.X+r.X)/3*w, Y: c.sum.Y + (p.Y+q.Y+r.Y)/3*w, Z: c.sum.Z + (p.Z+q.Z+r.Z)/3*w}
	c.w += w
}

func (c *lampCluster) centre() scene.Vec3 {
	return scene.Vec3{X: c.sum.X / c.w, Y: c.sum.Y / c.w, Z: c.sum.Z / c.w}
}

// ok: lamp-sized. A body panel whose name happens to say "light" is not.
func (c *lampCluster) ok() bool {
	return c.w > 0 && c.max.X-c.min.X <= 1 && c.max.Y-c.min.Y <= 1 && c.max.Z-c.min.Z <= .8
}

func (c *lampCluster) merge(o *lampCluster) {
	c.sum = scene.Vec3{X: c.sum.X + o.sum.X, Y: c.sum.Y + o.sum.Y, Z: c.sum.Z + o.sum.Z}
	c.w += o.w
	c.min = scene.Vec3{X: math.Min(c.min.X, o.min.X), Y: math.Min(c.min.Y, o.min.Y), Z: math.Min(c.min.Z, o.min.Z)}
	c.max = scene.Vec3{X: math.Max(c.max.X, o.max.X), Y: math.Max(c.max.Y, o.max.Y), Z: math.Max(c.max.Z, o.max.Z)}
}

type lampMaterial struct {
	fn            string
	total, inEnds float64         // triangle area overall / in the end zones
	clusters      [4]*lampCluster // front L, front R, rear L, rear R
}

// analyzeLamps splits every lamp material of the scene into end/side clusters.
func analyzeLamps(sc *scene.Scene) map[int]*lampMaterial {
	out := map[int]*lampMaterial{}
	for i, m := range sc.Materials {
		if fn := lampFunction(m.Alias + " " + m.Effect); fn != "" {
			out[i] = &lampMaterial{fn: fn}
		}
	}
	b := sc.Bounds()
	length := b.Length()
	if len(out) == 0 || length < .5 {
		return map[int]*lampMaterial{}
	}
	mid, fs := (b.Min.Y+b.Max.Y)/2, frontSign(sc)
	for _, t := range sc.Triangles {
		lm := out[t.Material]
		if lm == nil || t.A < 0 || t.B < 0 || t.C < 0 || t.A >= len(sc.Vertices) || t.B >= len(sc.Vertices) || t.C >= len(sc.Vertices) {
			continue
		}
		p, q, r := sc.Vertices[t.A].Position, sc.Vertices[t.B].Position, sc.Vertices[t.C].Position
		ux, uy, uz := q.X-p.X, q.Y-p.Y, q.Z-p.Z
		vx, vy, vz := r.X-p.X, r.Y-p.Y, r.Z-p.Z
		w := math.Max(1e-9, math.Sqrt(math.Pow(uy*vz-uz*vy, 2)+math.Pow(uz*vx-ux*vz, 2)+math.Pow(ux*vy-uy*vx, 2))/2)
		cx, cy := (p.X+q.X+r.X)/3, (p.Y+q.Y+r.Y)/3
		lm.total += w
		rel := (cy - mid) * fs / length
		if math.Abs(rel) < lampEndZone {
			continue
		}
		lm.inEnds += w
		k := 0
		if rel < 0 {
			k += 2
		}
		if cx >= 0 {
			k++
		}
		if lm.clusters[k] == nil {
			lm.clusters[k] = newLampCluster(rel > 0, cx < 0)
		}
		lm.clusters[k].add(w, p, q, r)
	}
	return out
}

// lamp reports whether the material really is lamp glass at the car ends.
func (lm *lampMaterial) lamp() bool {
	if lm == nil || lm.total <= 0 || lm.inEnds < .5*lm.total {
		return false
	}
	n := 0
	for _, c := range lm.clusters {
		if c == nil {
			continue
		}
		if !c.ok() {
			return false
		}
		n++
	}
	return n > 0
}

// lampGlow returns the variable lamp glass i lights up with, or "" when its
// function is unclear: brake glass glows with AI_Brakelight, a one-sided
// indicator with its blinker, head/tail glass (or one shared lamp atlas)
// with AI_Light. Flares are invisible in OMSI and reverse lamps have no AI
// variable.
func lampGlow(sc *scene.Scene, lamps map[int]*lampMaterial, i int) string {
	lm := lamps[i]
	if !lm.lamp() {
		return ""
	}
	m := sc.Materials[i]
	if strings.TrimSpace(m.Texture) == "" || strings.HasPrefix(strings.ToLower(m.Texture), "gen_"+genInvisible) || strings.Contains(strings.ToLower(m.Alias+" "+m.Effect), "flare") {
		return ""
	}
	var left, right float64
	for _, c := range lm.clusters {
		if c != nil && c.left {
			left += c.w
		} else if c != nil {
			right += c.w
		}
	}
	switch lm.fn {
	case omsi.LightReverse:
		return ""
	case omsi.LightBrake:
		return omsi.VarBrake
	case omsi.LightBlinker:
		if right <= .02*(left+right) {
			return omsi.VarBlinkerL
		}
		if left <= .02*(left+right) {
			return omsi.VarBlinkerR
		}
		return ""
	}
	return omsi.VarLight
}

// lampSpot is one lamp before it becomes an OMSI light.
type lampSpot struct {
	fn     string
	front  bool
	pos    scene.Vec3
	outer  float64 // |x| of the lamp's outer edge
	centre bool    // on the centre line (third brake light): no blinker there
	w      float64
}

// positional resolves head/tail/any by the end of the car the lamp is at.
func positional(fn string, front bool) string {
	switch fn {
	case lampAny, omsi.LightHead, omsi.LightTail:
		if front {
			return omsi.LightHead
		}
		return omsi.LightTail
	}
	return fn
}

func locatorSpots(sc *scene.Scene, fs, mid float64) []lampSpot {
	out := []lampSpot{}
	for _, l := range sc.Locators {
		fn := lampFunction(l.Name + " " + l.Hookup)
		if fn == "" {
			continue
		}
		front := (l.Position.Y-mid)*fs > 0
		out = append(out, lampSpot{fn: positional(fn, front), front: front, pos: l.Position, outer: math.Abs(l.Position.X) + .12, centre: math.Abs(l.Position.X) < .15, w: 1})
	}
	return out
}

func materialSpots(sc *scene.Scene, fs float64) []lampSpot {
	lamps := analyzeLamps(sc)
	ids := make([]int, 0, len(lamps))
	for i := range lamps {
		ids = append(ids, i)
	}
	sort.Ints(ids)
	out := []lampSpot{}
	for _, i := range ids {
		lm := lamps[i]
		if lm.total <= 0 || lm.inEnds < .5*lm.total {
			continue
		}
		for end := 0; end < 4; end += 2 {
			cl := []*lampCluster{}
			l, r := lm.clusters[end], lm.clusters[end+1]
			centre := false
			if l != nil && r != nil && math.Abs(l.centre().X-r.centre().X) < .5 {
				// both halves of one centre lamp (third brake light)
				m := *l
				m.merge(r)
				cl, centre = append(cl, &m), true
			} else {
				for _, c := range []*lampCluster{l, r} {
					if c != nil {
						cl = append(cl, c)
					}
				}
			}
			for _, c := range cl {
				if !c.ok() {
					continue
				}
				p := c.centre()
				// the lit face: the lamp's outermost point along its beam
				if (fs > 0) == c.front {
					p.Y = c.max.Y
				} else {
					p.Y = c.min.Y
				}
				outer := c.max.X
				if c.left {
					outer = -c.min.X
				}
				out = append(out, lampSpot{fn: positional(lm.fn, c.front), front: c.front, pos: p, outer: outer, centre: centre, w: c.w})
			}
		}
	}
	return out
}

// detectLights finds the car's lamps: ETS2 light locators first, lamp
// material clusters for every function/end the locators do not cover. Tail
// and brake lights share their lamps when only one is found, and blinkers
// that no locator or indicator material places sit at the outer edge of
// the head/tail lamps. Reverse lamps are returned without a variable (not
// exported).
func detectLights(sc scene.Scene) ([]omsi.PointLight, string) {
	if len(sc.Vertices) == 0 && len(sc.Locators) == 0 {
		return nil, "no geometry; no lights exported"
	}
	fs := frontSign(&sc)
	b := sc.Bounds()
	mid := (b.Min.Y + b.Max.Y) / 2
	key := func(fn string, front bool) string { return fmt.Sprintf("%s/%t", fn, front) }
	spots := locatorSpots(&sc, fs, mid)
	byLocator := map[string]bool{}
	for _, s := range spots {
		byLocator[key(s.fn, s.front)] = true
	}
	sources := []string{}
	if len(spots) > 0 {
		sources = append(sources, "ETS2 light locators")
	}
	nMat := 0
	for _, s := range materialSpots(&sc, fs) {
		if !byLocator[key(s.fn, s.front)] {
			spots = append(spots, s)
			nMat++
		}
	}
	if nMat > 0 {
		sources = append(sources, "lamp material clusters")
	}
	has := func(fn string, front bool) bool {
		for _, s := range spots {
			if s.fn == fn && s.front == front {
				return true
			}
		}
		return false
	}
	derive := func(from, to string, front bool) int {
		n := 0
		for _, s := range append([]lampSpot(nil), spots...) {
			if s.fn != from || s.front != front {
				continue
			}
			d := s
			d.fn = to
			if to == omsi.LightBlinker {
				if s.centre || s.pos.X == 0 {
					continue
				}
				d.pos.X = math.Copysign(math.Max(math.Abs(s.pos.X), s.outer-.04), s.pos.X)
			}
			spots = append(spots, d)
			n++
		}
		return n
	}
	shared := 0
	if !has(omsi.LightTail, false) {
		shared += derive(omsi.LightBrake, omsi.LightTail, false)
	}
	if !has(omsi.LightBrake, false) {
		shared += derive(omsi.LightTail, omsi.LightBrake, false)
	}
	if shared > 0 {
		sources = append(sources, "tail and brake lights share their lamps")
	}
	edge := 0
	if !has(omsi.LightBlinker, true) {
		edge += derive(omsi.LightHead, omsi.LightBlinker, true)
	}
	if !has(omsi.LightBlinker, false) {
		edge += derive(omsi.LightTail, omsi.LightBlinker, false)
	}
	if edge > 0 {
		sources = append(sources, "blinkers at the outer edge of the head/tail lamps")
	}
	spots = mergeSpots(spots)
	if len(spots) == 0 {
		return nil, "no ETS2 light locators or lamp materials found; no lights exported"
	}
	out := make([]omsi.PointLight, 0, len(spots))
	reverse := 0
	for _, s := range spots {
		dir := fs
		if !s.front {
			dir = -fs
		}
		if s.fn == omsi.LightBlinker && s.centre {
			continue // a blinker needs a side
		}
		if s.fn == omsi.LightReverse {
			reverse++
		}
		out = append(out, omsi.NewLight(s.fn, s.pos.X, s.pos.Y, s.pos.Z, dir))
	}
	basis := strings.Join(sources, " + ")
	if reverse > 0 {
		basis += "; reverse lamps found but not exported (OMSI AI cars never reverse and have no reverse-light variable)"
	}
	return out, basis
}

// mergeSpots joins lamps of the same function closer than 25 cm (lamp glass
// and its reflector as two materials), keeps at most four per function and
// car end, and sorts them head, tail, brake, blinker, reverse; front first;
// left to right.
func mergeSpots(in []lampSpot) []lampSpot {
	out := []lampSpot{}
	for _, s := range in {
		merged := false
		for i := range out {
			o := &out[i]
			if o.fn != s.fn || o.front != s.front || math.Hypot(math.Hypot(o.pos.X-s.pos.X, o.pos.Y-s.pos.Y), o.pos.Z-s.pos.Z) >= .25 {
				continue
			}
			w := o.w + s.w
			o.pos = scene.Vec3{X: (o.pos.X*o.w + s.pos.X*s.w) / w, Y: (o.pos.Y*o.w + s.pos.Y*s.w) / w, Z: (o.pos.Z*o.w + s.pos.Z*s.w) / w}
			o.outer, o.centre, o.w = math.Max(o.outer, s.outer), o.centre && s.centre, w
			merged = true
			break
		}
		if !merged {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.fn != b.fn {
			return lightOrder[a.fn] < lightOrder[b.fn]
		}
		if a.front != b.front {
			return a.front
		}
		if a.w != b.w {
			return a.w > b.w
		}
		return a.pos.X < b.pos.X
	})
	kept := []lampSpot{}
	count := map[string]int{}
	for _, s := range out {
		k := fmt.Sprintf("%s/%t", s.fn, s.front)
		if count[k] < 4 {
			kept = append(kept, s)
			count[k]++
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		a, b := kept[i], kept[j]
		if a.fn != b.fn {
			return lightOrder[a.fn] < lightOrder[b.fn]
		}
		if a.front != b.front {
			return a.front
		}
		return a.pos.X < b.pos.X
	})
	return kept
}

// lightCounts counts the exported lights by function.
func lightCounts(lights []omsi.PointLight) (int, map[string]int) {
	n, kinds := 0, map[string]int{}
	for _, l := range lights {
		if strings.TrimSpace(l.Variable) == "" {
			continue
		}
		n++
		kinds[l.Kind]++
	}
	if n == 0 {
		return 0, nil
	}
	return n, kinds
}
