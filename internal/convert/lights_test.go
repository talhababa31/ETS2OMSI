package convert

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ets2omsi/internal/omsi"
	"ets2omsi/internal/scene"
)

// lampQuad adds a flat w x h lamp face (x by z) at y, centred on (cx, cz).
func lampQuad(sc *scene.Scene, mat int, cx, y, cz, w, h float64) {
	b := len(sc.Vertices)
	for _, p := range [][2]float64{{-w / 2, -h / 2}, {w / 2, -h / 2}, {w / 2, h / 2}, {-w / 2, h / 2}} {
		sc.Vertices = append(sc.Vertices, scene.Vertex{Position: scene.Vec3{X: cx + p[0], Y: y, Z: cz + p[1]}})
	}
	sc.Triangles = append(sc.Triangles, scene.Triangle{A: b, B: b + 1, C: b + 2, Material: mat}, scene.Triangle{A: b, B: b + 2, C: b + 3, Material: mat})
}

// carScene: a 1.8 x 4.6 x 1.4 m body (material 0) plus the given materials.
func carScene(mats ...scene.Material) scene.Scene {
	sc := scene.Scene{Materials: append([]scene.Material{{Alias: "mat_0000_body", Texture: "body.dds"}}, mats...)}
	for _, p := range []scene.Vec3{{X: -.9, Y: -2.3}, {X: .9, Y: 2.3, Z: 1.4}, {X: .9, Y: -2.3, Z: .7}} {
		sc.Vertices = append(sc.Vertices, scene.Vertex{Position: p})
	}
	sc.Triangles = append(sc.Triangles, scene.Triangle{A: 0, B: 1, C: 2})
	return sc
}

func lightsOf(ls []omsi.PointLight, kind string) []omsi.PointLight {
	out := []omsi.PointLight{}
	for _, l := range ls {
		if l.Kind == kind {
			out = append(out, l)
		}
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestDetectLightsFromLocators(t *testing.T) {
	sc := carScene()
	for _, l := range []scene.Locator{
		{Name: "wheel_f_0", Position: scene.Vec3{X: -.8, Y: 1.4, Z: .3}}, {Name: "wheel_r_0", Position: scene.Vec3{X: -.8, Y: -1.4, Z: .3}},
		{Name: "light_head_l", Position: scene.Vec3{X: -.65, Y: 2.2, Z: .7}}, {Name: "light_head_r", Position: scene.Vec3{X: .65, Y: 2.2, Z: .7}},
		{Name: "brake_l", Position: scene.Vec3{X: -.7, Y: -2.25, Z: .85}}, {Name: "brake_r", Position: scene.Vec3{X: .7, Y: -2.25, Z: .85}},
		{Name: "blinker_fl", Position: scene.Vec3{X: -.78, Y: 2.15, Z: .7}}, {Name: "blinker_fr", Position: scene.Vec3{X: .78, Y: 2.15, Z: .7}},
		{Name: "blinker_rl", Position: scene.Vec3{X: -.75, Y: -2.2, Z: .85}}, {Name: "blinker_rr", Position: scene.Vec3{X: .75, Y: -2.2, Z: .85}},
		{Name: "reverse_l", Position: scene.Vec3{X: -.5, Y: -2.25, Z: .85}},
	} {
		sc.Locators = append(sc.Locators, l)
	}
	ls, basis := detectLights(sc)
	if !strings.Contains(basis, "locators") || !strings.Contains(basis, "reverse") {
		t.Fatalf("basis %q", basis)
	}
	head := lightsOf(ls, omsi.LightHead)
	if len(head) != 2 || !near(head[0].X, -.65) || !near(head[0].Y, 2.2) || !near(head[0].Z, .7) || head[0].DY != 1 || head[0].Variable != omsi.VarLight {
		t.Fatalf("head lights %+v", head)
	}
	brake := lightsOf(ls, omsi.LightBrake)
	if len(brake) != 2 || brake[0].DY != -1 || brake[0].Variable != omsi.VarBrake || !near(brake[1].X, .7) {
		t.Fatalf("brake lights %+v", brake)
	}
	if tail := lightsOf(ls, omsi.LightTail); len(tail) != 2 || tail[0].Variable != omsi.VarLight || !near(tail[0].Y, -2.25) {
		t.Fatalf("tail lights must share the brake lamps: %+v", tail)
	}
	bl := lightsOf(ls, omsi.LightBlinker)
	if len(bl) != 4 {
		t.Fatalf("blinkers %+v", bl)
	}
	for _, l := range bl {
		if want := map[bool]string{true: omsi.VarBlinkerL, false: omsi.VarBlinkerR}[l.X < 0]; l.Variable != want {
			t.Fatalf("blinker at x=%.2f uses %q", l.X, l.Variable)
		}
		if (l.Y > 0) != (l.DY > 0) {
			t.Fatalf("blinker direction %+v", l)
		}
	}
	if rev := lightsOf(ls, omsi.LightReverse); len(rev) != 1 || rev[0].Variable != "" {
		t.Fatalf("reverse lamp must be found but not get a variable: %+v", rev)
	}
	n, kinds := lightCounts(ls)
	if n != 10 || kinds[omsi.LightHead] != 2 || kinds[omsi.LightBlinker] != 4 || kinds[omsi.LightReverse] != 0 {
		t.Fatalf("counts %d %v", n, kinds)
	}
}

func TestDetectLightsFromLampMaterials(t *testing.T) {
	sc := carScene(scene.Material{Alias: "mat_0003_vehicle_lights", Effect: "eut2.lamp.add.env", Texture: "lights.dds"})
	lampQuad(&sc, 1, -.65, 2.25, .7, .3, .15)
	lampQuad(&sc, 1, .65, 2.25, .7, .3, .15)
	lampQuad(&sc, 1, -.7, -2.28, .85, .2, .1)
	lampQuad(&sc, 1, .7, -2.28, .85, .2, .1)
	ls, basis := detectLights(sc)
	for _, w := range []string{"lamp material clusters", "share", "outer edge"} {
		if !strings.Contains(basis, w) {
			t.Fatalf("basis %q lacks %q", basis, w)
		}
	}
	head := lightsOf(ls, omsi.LightHead)
	if len(head) != 2 || !near(head[0].X, -.65) || !near(head[0].Y, 2.25) || !near(head[0].Z, .7) || head[0].DY != 1 {
		t.Fatalf("head lights %+v", head)
	}
	tail, brake := lightsOf(ls, omsi.LightTail), lightsOf(ls, omsi.LightBrake)
	if len(tail) != 2 || len(brake) != 2 || !near(tail[1].X, .7) || !near(brake[1].Y, -2.28) || brake[1].DY != -1 {
		t.Fatalf("tail %+v brake %+v", tail, brake)
	}
	bl := lightsOf(ls, omsi.LightBlinker)
	if len(bl) != 4 {
		t.Fatalf("blinkers %+v", bl)
	}
	// front left blinker at the lamp's outer edge: |x| = 0.65 + 0.15 - 0.04
	if !near(bl[0].X, -.76) || bl[0].Variable != omsi.VarBlinkerL || bl[0].DY != 1 {
		t.Fatalf("front left blinker %+v", bl[0])
	}
	if n, _ := lightCounts(ls); n != 10 {
		t.Fatalf("exported %d lights", n)
	}
}

func TestDetectLightsIndicatorMaterialAndLocatorMix(t *testing.T) {
	sc := carScene(
		scene.Material{Alias: "mat_0001_lights", Effect: "eut2.lamp", Texture: "lights.dds"},
		scene.Material{Alias: "mat_0002_blinker", Effect: "eut2.lamp", Texture: "blink.dds"},
	)
	lampQuad(&sc, 1, -.7, -2.28, .85, .2, .1)
	lampQuad(&sc, 1, .7, -2.28, .85, .2, .1)
	lampQuad(&sc, 2, -.8, 2.1, .65, .08, .05)
	lampQuad(&sc, 2, .8, 2.1, .65, .08, .05)
	sc.Locators = []scene.Locator{{Name: "head_light_l", Position: scene.Vec3{X: -.6, Y: 2.2, Z: .7}}, {Name: "head_light_r", Position: scene.Vec3{X: .6, Y: 2.2, Z: .7}}}
	ls, basis := detectLights(sc)
	if !strings.Contains(basis, "ETS2 light locators + lamp material clusters") {
		t.Fatalf("basis %q", basis)
	}
	if head := lightsOf(ls, omsi.LightHead); len(head) != 2 || !near(head[1].X, .6) {
		t.Fatalf("locator head lights %+v", head)
	}
	bl := lightsOf(ls, omsi.LightBlinker)
	if len(bl) != 4 || !near(bl[0].X, -.8) || !near(bl[0].Y, 2.1) {
		t.Fatalf("front blinkers must come from the indicator material: %+v", bl)
	}
}

func TestDetectLightsFollowsFrontWheelLocators(t *testing.T) {
	sc := carScene(scene.Material{Alias: "lamp", Effect: "eut2.lamp", Texture: "l.dds"})
	lampQuad(&sc, 1, -.65, -2.25, .7, .3, .15)
	lampQuad(&sc, 1, .65, -2.25, .7, .3, .15)
	sc.Locators = []scene.Locator{{Name: "wheel_f_0", Position: scene.Vec3{X: -.8, Y: -1.4, Z: .3}}, {Name: "wheel_r_0", Position: scene.Vec3{X: -.8, Y: 1.4, Z: .3}}}
	ls, _ := detectLights(sc)
	head := lightsOf(ls, omsi.LightHead)
	if len(head) != 2 || head[0].DY != -1 || !near(head[0].Y, -2.25) || len(lightsOf(ls, omsi.LightTail)) != 0 {
		t.Fatalf("front is -Y here: %+v", ls)
	}
}

func TestDetectLightsIgnoresNonLamps(t *testing.T) {
	sc := carScene(
		scene.Material{Alias: "interior_light", Effect: "eut2.lamp", Texture: "a.dds"},
		scene.Material{Alias: "mat_0002_light_grey_trim", Texture: "b.dds"},
		scene.Material{Alias: "mat_0003_lights", Texture: "c.dds"},
		scene.Material{Alias: "glass_window", Effect: "eut2.glass", Texture: "d.dds"},
		scene.Material{Alias: "license_plate_light", Effect: "eut2.lamp", Texture: "e.dds"},
	)
	lampQuad(&sc, 1, 0, .2, 1.2, .3, .1)     // roof light in the middle of the car
	lampQuad(&sc, 2, -.6, 2.2, .4, .3, .1)   // grey trim at the front
	lampQuad(&sc, 3, -.45, 2.0, .7, .9, 1.2) // a panel far too big for a lamp
	lampQuad(&sc, 3, .45, 2.0, .7, .9, 1.2)
	lampQuad(&sc, 4, -.5, 2.0, 1.0, .9, .4)
	lampQuad(&sc, 5, 0, -2.3, .5, .2, .05)
	if ls, basis := detectLights(sc); len(ls) != 0 || !strings.Contains(basis, "no lights") {
		t.Fatalf("non-lamps became lights (%s): %+v", basis, ls)
	}
	if ls, _ := detectLights(scene.Scene{}); len(ls) != 0 {
		t.Fatalf("empty scene: %+v", ls)
	}
}

// The lights stay in the scene frame after the pipeline's centring and
// ground translation; the O3D mesh is the one that swaps Y/Z. So a light
// sits on its lamp: light (x, y, z) = O3D vertex (X, Z, Y).
func TestLightPositionsMatchO3DAxesAndTranslation(t *testing.T) {
	sc := carScene(scene.Material{Alias: "vehicle_lights", Effect: "eut2.lamp", Texture: "lights.dds"})
	lampQuad(&sc, 1, -.65, 2.25, .7, .3, .15)
	lampQuad(&sc, 1, .65, 2.25, .7, .3, .15)
	lampQuad(&sc, 1, -.7, -2.28, .85, .2, .1)
	lampQuad(&sc, 1, .7, -2.28, .85, .2, .1)
	sc.Translate(.1, -.3, -.05) // centring + ground plane, as Vehicle() does
	ls, _ := detectLights(sc)
	head := lightsOf(ls, omsi.LightHead)
	if len(head) != 2 {
		t.Fatalf("head %+v", head)
	}
	m := toO3D(sc)
	// vertices 3..6 are the front-left lamp quad
	var x, up, long float64
	for _, v := range m.Vertices[3:7] {
		x += float64(v.X) / 4
		up += float64(v.Y) / 4
		long = math.Max(long, float64(v.Z))
	}
	l := head[0]
	if math.Abs(l.X-x) > 1e-5 || math.Abs(l.Z-up) > 1e-5 || math.Abs(l.Y-long) > 1e-5 {
		t.Fatalf("light (%.3f %.3f %.3f) is not on its O3D lamp (X %.3f, Z %.3f, Y %.3f)", l.X, l.Y, l.Z, x, long, up)
	}
	if !near(l.X, -.55) || !near(l.Y, 1.95) || !near(l.Z, .65) {
		t.Fatalf("translation not applied: %+v", l)
	}
	cfg := omsi.ModelCFG(omsi.VehicleSpec{Lights: ls[:1]})
	if !strings.Contains(cfg, "[light_enh_2]\r\n-0.550000\r\n1.950000\r\n0.650000\r\n0.000000\r\n1.000000\r\n0.000000\r\n") {
		t.Fatalf("model.cfg must carry x, y forward, z up:\n%s", cfg)
	}
}

func TestMaterialOverridesInstanceAndLampGlow(t *testing.T) {
	sc := carScene(
		scene.Material{Alias: "mat_0001_body2", Texture: "body.dds"},
		scene.Material{Alias: "mat_0002_vehicle_lights", Effect: "eut2.lamp", Texture: "lights.dds"},
		scene.Material{Alias: "brake_lamp", Effect: "eut2.lamp", Texture: "brake.dds"},
		scene.Material{Alias: "blinker_left", Effect: "eut2.lamp", Texture: "blink.dds"},
		scene.Material{Alias: "lamp_flare", Effect: "eut2.flare.vehicle", Texture: "gen_invisible.dds"},
		scene.Material{Alias: "glass_ex", Effect: "eut2.glass", Texture: "glass.dds", Alpha: true},
		scene.Material{Alias: "reverse_light", Effect: "eut2.lamp", Texture: "rev.dds"},
		scene.Material{Alias: "blinker_both", Effect: "eut2.lamp", Texture: "blink.dds"},
	)
	sc.Triangles = append(sc.Triangles, scene.Triangle{A: 0, B: 1, C: 2, Material: 1})
	for _, x := range []float64{-.65, .65} {
		lampQuad(&sc, 2, x, 2.25, .7, .3, .15)
		lampQuad(&sc, 3, x, -2.28, .85, .2, .1)
		lampQuad(&sc, 5, x, 2.25, .7, .1, .1)
		lampQuad(&sc, 7, x/2, -2.28, .7, .1, .1)
		lampQuad(&sc, 8, x*1.2, 2.2, .7, .05, .05)
	}
	lampQuad(&sc, 4, -.8, 2.1, .65, .08, .05)
	lampQuad(&sc, 6, 0, .5, 1.1, 1.4, .6)
	got := materialOverrides(sc)
	want := []struct {
		inst int
		glow string
	}{{0, ""}, {1, ""}, {0, omsi.VarLight}, {0, omsi.VarBrake}, {0, omsi.VarBlinkerL}, {0, ""}, {0, ""}, {0, ""}, {1, ""}}
	for i, w := range want {
		if got[i].Instance != w.inst || got[i].Glow != w.glow {
			t.Fatalf("material %d (%s): instance %d glow %q, want %d %q", i, sc.Materials[i].Alias, got[i].Instance, got[i].Glow, w.inst, w.glow)
		}
	}
	cfg := omsi.ModelCFG(omsi.VehicleSpec{Materials: got})
	if !strings.Contains(cfg, "[matl]\r\nbody.dds\r\n1\r\n") || !strings.Contains(cfg, "[matl_change]\r\nbrake.dds\r\n0\r\nAI_Brakelight\r\n[matl_item]\r\n[matl_nightmap]\r\nbrake.dds\r\n") {
		t.Fatalf("model.cfg materials:\n%s", cfg)
	}
}

func TestColorVariantsKeepLightsAndGlow(t *testing.T) {
	stage := t.TempDir()
	tex := filepath.Join(stage, "texture")
	_ = os.MkdirAll(tex, 0755)
	_ = os.MkdirAll(filepath.Join(stage, "model"), 0755)
	writeBodyTexture(t, tex)
	sc := scene.Scene{Materials: []scene.Material{{Alias: "mat_0000_body", Texture: "body.dds"}, {Alias: "mat_0001_lights", Effect: "eut2.lamp", Texture: "lights.dds"}}}
	for _, p := range []scene.Vec3{{X: -.9, Y: -2.3}, {X: .9, Y: 2.3, Z: 1.4}, {X: .9, Y: -2.3, Z: .7}} {
		sc.Vertices = append(sc.Vertices, scene.Vertex{Position: p, UV: scene.Vec2{X: .3, Y: .5}})
	}
	sc.Triangles = []scene.Triangle{{A: 0, B: 1, C: 2}}
	lampQuad(&sc, 1, -.65, 2.25, .7, .3, .15)
	lampQuad(&sc, 1, .65, -2.25, .7, .3, .15)
	ls, _ := detectLights(sc)
	cm := convertedModel{o3dName: "body.o3d", sc: sc}
	spec := omsi.VehicleSpec{Name: "Car", Type: "car", Materials: materialOverrides(sc), Lights: ls}
	vs, _ := exportColorVariants(stage, spec, []convertedModel{cm}, newTextureResolver(nil, tex), newOpaqueFixer(tex), nil, tex, []string{"siyah"})
	if len(vs) != 1 {
		t.Fatalf("variants %+v", vs)
	}
	n, _ := lightCounts(ls)
	cfg, _ := os.ReadFile(filepath.Join(stage, "model", "model_"+vs[0].ID+".cfg"))
	if c := strings.Count(string(cfg), "[light_enh_2]"); n == 0 || c != n {
		t.Fatalf("variant cfg has %d lights, want %d:\n%s", c, n, cfg)
	}
	if !strings.Contains(string(cfg), "[matl_change]\r\nlights.dds\r\n0\r\nAI_Light\r\n") {
		t.Fatalf("variant lamp glass does not glow:\n%s", cfg)
	}
}
