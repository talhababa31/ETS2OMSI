package convert

import (
	"os"
	"path/filepath"
	"testing"

	"ets2omsi/internal/scene"
)

// Car along Y; front wheel locators at +Y. Material 0 sits at the front,
// material 1 at the rear.
func lampScene(names ...string) scene.Scene {
	sc := scene.Scene{
		Vertices: []scene.Vertex{
			{Position: scene.Vec3{X: -.8, Y: 2.2, Z: .7}}, {Position: scene.Vec3{X: -.6, Y: 2.2, Z: .7}}, {Position: scene.Vec3{X: -.7, Y: 2.2, Z: .8}},
			{Position: scene.Vec3{X: -.8, Y: -2.2, Z: .8}}, {Position: scene.Vec3{X: -.6, Y: -2.2, Z: .8}}, {Position: scene.Vec3{X: -.7, Y: -2.2, Z: .9}},
		},
		Triangles: []scene.Triangle{{A: 0, B: 1, C: 2, Material: 0}, {A: 3, B: 4, C: 5, Material: 1}},
		Locators: []scene.Locator{
			{Name: "wheel_f_0", Position: scene.Vec3{X: -.8, Y: 1.4}}, {Name: "wheel_f_1", Position: scene.Vec3{X: .8, Y: 1.4}},
			{Name: "wheel_r_0", Position: scene.Vec3{X: -.8, Y: -1.4}}, {Name: "wheel_r_1", Position: scene.Vec3{X: .8, Y: -1.4}},
		},
	}
	for i, n := range names {
		sc.Materials = append(sc.Materials, scene.Material{Index: i, Alias: n})
	}
	return sc
}

func TestGeneratedKind(t *testing.T) {
	sc := lampScene("mat_0000_lights", "mat_0001_lights")
	if k, _, _ := generatedKind(&sc, 0, ""); k != genHeadlight {
		t.Fatalf("front lamp=%q", k)
	}
	if k, _, _ := generatedKind(&sc, 1, ""); k != genTaillight {
		t.Fatalf("rear lamp=%q", k)
	}
	for alias, want := range map[string]string{
		"mat_0014_meganephare":   genHeadlight,
		"mat_0003_stop":          genTaillight,
		"mat_0004_blinker":       genIndicator,
		"mat_0001_glass_ex":      genGlass,
		"mat_0009_shadow":        genInvisible,
		"mat_0007_itdplast":      genInterior,
		"mat_0006_tableaudebord": genInterior,
		"mat_0005_chrome":        genChrome,
		"mat_0023_jante512":      genRim,
	} {
		s := lampScene(alias)
		if k, _, _ := generatedKind(&s, 0, ""); k != want {
			t.Fatalf("%s=%q want %q", alias, k, want)
		}
	}
	s := lampScene("mat_0009_black")
	if k, c, _ := generatedKind(&s, 0, ""); k != "" || c == nil || c.R > 40 {
		t.Fatalf("black solid: %q %v", k, c)
	}
	// Separate wheel model: unknown material -> rim, rubber -> tyre.
	w := scene.Scene{
		Vertices:  []scene.Vertex{{Position: scene.Vec3{X: -.1, Y: -.3, Z: -.3}}, {Position: scene.Vec3{X: .1, Y: .3, Z: .3}}},
		Materials: []scene.Material{{Alias: "mat_0000_golf_x"}, {Alias: "mat_0001_x", Class: "rubber"}},
	}
	if k, _, _ := generatedKind(&w, 0, ""); k != genRim {
		t.Fatalf("wheel default=%q", k)
	}
	if k, _, _ := generatedKind(&w, 1, ""); k != genTire {
		t.Fatalf("wheel rubber=%q", k)
	}
}

func TestMissingPartsGetGeneratedTextures(t *testing.T) {
	d := t.TempDir()
	sc := lampScene("mat_0000_lights", "mat_0001_lights")
	tr := TextureReport{}
	w := []string{}
	applySafeMaterialFallbacks(&sc, []string{"mat_0000_lights", "mat_0001_lights"}, d, &tr, &w)
	if sc.Materials[0].Texture != "gen_headlight.dds" || sc.Materials[1].Texture != "gen_taillight.dds" {
		t.Fatalf("mats=%+v", sc.Materials)
	}
	for _, n := range []string{"gen_headlight.dds", "gen_taillight.dds"} {
		b, err := os.ReadFile(filepath.Join(d, n))
		if err != nil {
			t.Fatal(err)
		}
		im, err := decodeImageBytes(b, ".dds")
		if err != nil || im.Bounds().Dx() != 128 {
			t.Fatalf("%s: %v", n, err)
		}
	}
	if len(w) != 0 {
		t.Fatalf("generated parts must not be warnings: %v", w)
	}
}
