package convert

import (
	"ets2omsi/internal/omsi"
	"ets2omsi/internal/scene"
)

// detectVehicleClass picks the vehicle class from the name, else from the
// body shape. basis explains the decision (shown in the UI and report).
func detectVehicleClass(name string, sc scene.Scene) (class, basis string) {
	if c := omsi.ClassFromName(name); c != "" {
		return c, "araç adından"
	}
	b := sc.Bounds()
	l, w, h := b.Length(), b.Width(), b.Height()
	if l <= 0 || h <= 0 {
		return omsi.ClassSedan, "varsayılan"
	}
	// Roof height profile along the length, rear to front.
	front := 1.0
	var fy, ry float64
	var fn, rn int
	for _, p := range wheelPlacements(sc) {
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
	const slices = 20
	top := make([]float64, slices)
	for i := range top {
		top[i] = b.Min.Z
	}
	for _, v := range sc.Vertices {
		t := (v.Position.Y - b.Min.Y) / l // 0 = min Y
		if front < 0 {
			t = 1 - t
		} // now 0 = rear, 1 = front
		i := int(t * slices)
		if i < 0 {
			i = 0
		}
		if i >= slices {
			i = slices - 1
		}
		if v.Position.Z > top[i] {
			top[i] = v.Position.Z
		}
	}
	rel := func(i int) float64 { return (top[i] - b.Min.Z) / h }
	rearMax := func(n int) float64 {
		m := 0.0
		for i := 0; i < n; i++ {
			if r := rel(i); r > m {
				m = r
			}
		}
		return m
	}
	rearEnd := rearMax(2) // last 10 % of the length
	rearBed := rearMax(6) // last 30 %
	switch {
	case h >= 2.25:
		return omsi.ClassMinibus, "yükseklik"
	case h >= 1.85:
		return omsi.ClassVan, "yükseklik"
	case l > 4.6 && rearBed < .72 && h > 1.6:
		return omsi.ClassPickup, "alçak arka kasa"
	case h >= 1.58 && w >= 1.72:
		return omsi.ClassSUV, "yükseklik/genişlik"
	case rearEnd >= .80 && l < 4.35:
		return omsi.ClassHatchback, "yüksek arka profil, kısa gövde"
	case rearEnd >= .80:
		return omsi.ClassWagon, "yüksek arka profil, uzun gövde"
	case h < 1.36:
		return omsi.ClassCoupe, "alçak gövde"
	}
	return omsi.ClassSedan, "alçak bagaj profili"
}
