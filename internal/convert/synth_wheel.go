package convert

import (
	"math"

	"ets2omsi/internal/scene"
)

// Synthetic wheels for vehicles whose wheel models are not in the traffic
// package (vans often use shared base.scs wheels) or that define none.

// syntheticWheelRadius estimates the wheel radius from the ETS2 wheel locator
// height: ETS2 vehicle models have their origin on the ground, so the locator
// (hub centre) height is the radius.
func syntheticWheelRadius(hubZ, groundZ, bodyHeight float64) float64 {
	r := hubZ - groundZ
	def := .31
	if bodyHeight > 1.85 {
		def = .35 // vans / minibuses
	}
	if r < .22 || r > .55 {
		r = def
	}
	return r
}

// buildSyntheticWheel builds a tyre (tread + sidewalls) and a rim face with
// five spokes, centred on the origin, axle along X. Material 0 = rim,
// material 1 = tyre, material 2 = dark openings between the spokes.
func buildSyntheticWheel(radius, width float64) scene.Scene {
	const seg = 30
	sc := scene.Scene{Materials: []scene.Material{
		{Index: 0, Alias: "mat_0000_synthetic_rim", Class: "chrome"},
		{Index: 1, Alias: "mat_0001_synthetic_tire", Class: "rubber"},
		{Index: 2, Alias: "mat_0002_synthetic_black"}, // openings between spokes
	}}
	add := func(x, y, z, nx, ny, nz, u, v float64) int {
		sc.Vertices = append(sc.Vertices, scene.Vertex{
			Position: scene.Vec3{X: x, Y: y, Z: z},
			Normal:   scene.Vec3{X: nx, Y: ny, Z: nz},
			UV:       scene.Vec2{X: u, Y: v},
		})
		return len(sc.Vertices) - 1
	}
	quad := func(a, b, c, d, mat int) {
		sc.Triangles = append(sc.Triangles, scene.Triangle{A: a, B: b, C: c, Material: mat}, scene.Triangle{A: a, B: c, C: d, Material: mat})
	}
	hw := width / 2
	inner := radius * .68 // rim edge (tyre sidewall height ~32%)
	for i := 0; i < seg; i++ {
		a0 := 2 * math.Pi * float64(i) / seg
		a1 := 2 * math.Pi * float64(i+1) / seg
		c0, s0, c1, s1 := math.Cos(a0), math.Sin(a0), math.Cos(a1), math.Sin(a1)
		u0, u1 := float64(i)/seg*4, float64(i+1)/seg*4
		// tread
		quad(add(-hw, c0*radius, s0*radius, 0, c0, s0, u0, 0), add(hw, c0*radius, s0*radius, 0, c0, s0, u0, 1),
			add(hw, c1*radius, s1*radius, 0, c1, s1, u1, 1), add(-hw, c1*radius, s1*radius, 0, c1, s1, u1, 0), 1)
		// sidewalls (both sides), slightly bulged inwards to the rim
		for _, side := range []float64{-1, 1} {
			x := side * hw
			xi := side * hw * .82
			a := add(x, c0*radius, s0*radius, side, 0, 0, u0, 0)
			b := add(x, c1*radius, s1*radius, side, 0, 0, u1, 0)
			c := add(xi, c1*inner, s1*inner, side, 0, 0, u1, 1)
			d := add(xi, c0*inner, s0*inner, side, 0, 0, u0, 1)
			if side < 0 {
				quad(a, d, c, b, 1)
			} else {
				quad(a, b, c, d, 1)
			}
		}
	}
	// Rim faces: a recessed dish with five spokes on each side.
	for _, side := range []float64{-1, 1} {
		xr := side * hw * .82
		xd := side * hw * .45 // recessed centre
		for i := 0; i < seg; i++ {
			a0 := 2 * math.Pi * float64(i) / seg
			a1 := 2 * math.Pi * float64(i+1) / seg
			c0, s0, c1, s1 := math.Cos(a0), math.Sin(a0), math.Cos(a1), math.Sin(a1)
			// spoke pattern: five spokes, each 2/32 of the circle wide is dish
			spoke := (i % (seg / 5)) < 3
			mid := inner * .32
			if spoke {
				mid = inner * .9
			}
			uvx := func(c, r float64) float64 { return .5 + c*r/radius*.5 }
			p0 := add(xr, c0*inner, s0*inner, side, 0, 0, uvx(c0, inner), uvx(s0, inner))
			p1 := add(xr, c1*inner, s1*inner, side, 0, 0, uvx(c1, inner), uvx(s1, inner))
			p2 := add(xd, c1*mid, s1*mid, side, 0, 0, uvx(c1, mid), uvx(s1, mid))
			p3 := add(xd, c0*mid, s0*mid, side, 0, 0, uvx(c0, mid), uvx(s0, mid))
			hub := add(xd, 0, 0, side, 0, 0, .5, .5)
			face := 0
			if !spoke {
				face = 2
			}
			if side < 0 {
				quad(p0, p3, p2, p1, face)
				sc.Triangles = append(sc.Triangles, scene.Triangle{A: p3, B: hub, C: p2, Material: 0})
			} else {
				quad(p0, p1, p2, p3, face)
				sc.Triangles = append(sc.Triangles, scene.Triangle{A: p3, B: p2, C: hub, Material: 0})
			}
		}
	}
	return sc
}
