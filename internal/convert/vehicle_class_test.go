package convert

import (
	"testing"

	"ets2omsi/internal/omsi"
	"ets2omsi/internal/scene"
)

// profile builds a body as points along Y (front at +Y) with the given roof
// heights from rear to front.
func profile(length, width float64, roof []float64) scene.Scene {
	sc := scene.Scene{}
	n := len(roof)
	for i, z := range roof {
		y := -length/2 + length*float64(i)/float64(n-1)
		sc.Vertices = append(sc.Vertices,
			scene.Vertex{Position: scene.Vec3{X: -width / 2, Y: y, Z: 0}},
			scene.Vertex{Position: scene.Vec3{X: width / 2, Y: y, Z: z}})
	}
	sc.Locators = []scene.Locator{{Name: "wheel_f_0", Position: scene.Vec3{Y: length / 3}}, {Name: "wheel_r_0", Position: scene.Vec3{Y: -length / 3}}}
	return sc
}

func TestDetectVehicleClass(t *testing.T) {
	sedan := profile(4.7, 1.8, []float64{1.0, 1.05, 1.05, 1.1, 1.4, 1.45, 1.45, 1.4, 1.1, 1.0})
	hatch := profile(4.0, 1.7, []float64{1.35, 1.42, 1.45, 1.45, 1.45, 1.4, 1.2, 1.0, .95, .9})
	wagon := profile(4.8, 1.8, []float64{1.4, 1.45, 1.47, 1.47, 1.47, 1.45, 1.2, 1.0, .95, .9})
	van := profile(5.9, 2.0, []float64{2.5, 2.55, 2.55, 2.55, 2.55, 2.5, 2.3, 1.6, 1.3, 1.2})
	suv := profile(4.6, 1.85, []float64{1.55, 1.65, 1.68, 1.68, 1.68, 1.6, 1.3, 1.15, 1.1, 1.0})
	pickup := profile(5.2, 1.85, []float64{1.05, 1.05, 1.05, 1.05, 1.75, 1.8, 1.75, 1.3, 1.2, 1.1})
	for name, tc := range map[string]struct {
		sc   scene.Scene
		want string
	}{
		"sedan": {sedan, omsi.ClassSedan}, "hatch": {hatch, omsi.ClassHatchback}, "wagon": {wagon, omsi.ClassWagon},
		"van": {van, omsi.ClassMinibus}, "suv": {suv, omsi.ClassSUV}, "pickup": {pickup, omsi.ClassPickup},
	} {
		if got, basis := detectVehicleClass("Unknown Car", tc.sc); got != tc.want {
			t.Fatalf("%s: got %s (%s) want %s", name, got, basis, tc.want)
		}
	}
	if got, _ := detectVehicleClass("Mercedes Sprinter", sedan); got != omsi.ClassVan {
		t.Fatalf("name must win: %s", got)
	}
	if got, _ := detectVehicleClass("Ford Ka", sedan); got != omsi.ClassHatchback {
		t.Fatalf("Ford Ka: %s", got)
	}
}

func TestClassChangesPhysicsAndSpeed(t *testing.T) {
	van := omsi.EstimateAIPhysicsClass("x", omsi.ClassVan, 5.9, 2.0, 2.5)
	hb := omsi.EstimateAIPhysicsClass("x", omsi.ClassHatchback, 3.9, 1.7, 1.45)
	if van.Mass <= hb.Mass || van.Class != omsi.ClassVan || hb.Profile != "Hatchback" {
		t.Fatalf("van=%+v hb=%+v", van, hb)
	}
	if omsi.AIConstFileClass(omsi.ClassVan) == omsi.AIConstFileClass(omsi.ClassCoupe) {
		t.Fatal("AI top speed must depend on class")
	}
}
