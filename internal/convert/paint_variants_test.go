package convert

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ets2omsi/internal/o3d"
	"ets2omsi/internal/omsi"
	"ets2omsi/internal/scene"
)

// Body texture: white paint with a black trim strip and a red lamp patch.
func writeBodyTexture(t *testing.T, dir string) {
	im := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			c := color.NRGBA{228, 228, 230, 255}
			if y < 2 {
				c = color.NRGBA{20, 20, 22, 255} // trim
			}
			if x > 13 && y > 13 {
				c = color.NRGBA{200, 20, 20, 255} // lamp
			}
			if x == 5 {
				c = color.NRGBA{170, 170, 172, 255} // panel shading
			}
			im.SetNRGBA(x, y, c)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "body.dds"), encodeDDS(im), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRecolorKeepsTrimAndLamps(t *testing.T) {
	d := t.TempDir()
	writeBodyTexture(t, d)
	sc := scene.Scene{Materials: []scene.Material{
		{Alias: "mat_0000_body", Texture: "body.dds"},
		{Alias: "mat_0001_lights", Texture: "body.dds"},
	}}
	rc := newRecolorer(d)
	red, _ := paintByID("kirmizi")
	if !rc.recolorScene(&sc, red) {
		t.Fatal("white body must be recoloured")
	}
	if sc.Materials[0].Texture != "body_kirmizi.dds" || sc.Materials[1].Texture != "body.dds" {
		t.Fatalf("mats=%+v", sc.Materials)
	}
	im, err := decodeTextureFile(filepath.Join(d, "body_kirmizi.dds"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA) }
	if c := at(8, 8); c.R < 140 || c.G > 40 {
		t.Fatalf("paint not red: %v", c)
	}
	if c := at(8, 0); c.R > 40 {
		t.Fatalf("trim changed: %v", c)
	}
	if c := at(15, 15); c != (color.NRGBA{200, 20, 20, 255}) {
		t.Fatalf("lamp changed: %v", c)
	}
	if shade, paint := at(5, 8), at(8, 8); shade.R >= paint.R {
		t.Fatalf("shading lost: %v vs %v", shade, paint)
	}
}

func TestExportColorVariantsWritesOVHs(t *testing.T) {
	stage := t.TempDir()
	tex := filepath.Join(stage, "texture")
	_ = os.MkdirAll(tex, 0755)
	_ = os.MkdirAll(filepath.Join(stage, "model"), 0755)
	writeBodyTexture(t, tex)
	cm := convertedModel{o3dName: "body.o3d", sc: scene.Scene{Materials: []scene.Material{{Alias: "mat_0000_body", Texture: "body.dds"}}}}
	spec := omsi.VehicleSpec{Name: "Car", Type: "car", Materials: materialOverrides(cm.sc)}
	r := newTextureResolver(nil, tex)
	vs, w := exportColorVariants(stage, spec, []convertedModel{cm}, r, newOpaqueFixer(tex), tex, []string{"siyah", "lacivert", "yok"})
	if len(vs) != 2 || len(w) != 0 {
		t.Fatalf("variants=%+v warnings=%v", vs, w)
	}
	for _, v := range vs {
		b, err := os.ReadFile(filepath.Join(stage, v.OVH))
		if err != nil || !strings.Contains(string(b), "model\\model_"+v.ID+".cfg") || !strings.Contains(string(b), v.Label) {
			t.Fatalf("%s: %v\n%s", v.OVH, err, b)
		}
		cfg, _ := os.ReadFile(filepath.Join(stage, "model", "model_"+v.ID+".cfg"))
		if !strings.Contains(string(cfg), "[mesh]\r\nbody_"+v.ID+".o3d") {
			t.Fatalf("cfg %s must load its own body O3D:\n%s", v.ID, cfg)
		}
		// OMSI takes textures from the O3D ([matl] only selects materials),
		// so the recoloured texture must be inside the variant O3D.
		raw, err := os.ReadFile(filepath.Join(stage, "model", "body_"+v.ID+".o3d"))
		if err != nil {
			t.Fatal(err)
		}
		m, err := o3d.Parse(raw)
		if err != nil || len(m.Materials) == 0 || m.Materials[0].Texture != "body_"+v.ID+".dds" {
			t.Fatalf("variant O3D texture: err=%v mats=%+v", err, m.Materials)
		}
	}
	if vs, _ := exportColorVariants(stage, spec, []convertedModel{cm}, r, newOpaqueFixer(tex), tex, []string{"none"}); len(vs) != 0 {
		t.Fatalf("none must disable palette: %+v", vs)
	}
}

// ETS2 spec mask in alpha must not reach OMSI for opaque materials.
func TestOpaqueMaterialsLoseAlpha(t *testing.T) {
	d := t.TempDir()
	im := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = 200, 200, 200, uint8(i*4)
	}
	_ = os.WriteFile(filepath.Join(d, "body.dds"), encodeDDS(im), 0644)
	sc := scene.Scene{Materials: []scene.Material{{Alias: "body", Texture: "body.dds"}, {Alias: "glass", Texture: "body.dds", Alpha: true}}}
	newOpaqueFixer(d).fixScene(&sc)
	if sc.Materials[0].Texture != "body_opq.dds" || sc.Materials[1].Texture != "body.dds" {
		t.Fatalf("mats=%+v", sc.Materials)
	}
	b, _ := os.ReadFile(filepath.Join(d, "body_opq.dds"))
	out, err := decodeImageBytes(b, ".dds")
	if err != nil || hasRealAlpha(out) {
		t.Fatalf("opaque texture still has alpha: %v", err)
	}
	if c := color.NRGBAModel.Convert(out.At(1, 1)).(color.NRGBA); c.R != 200 {
		t.Fatalf("colour changed: %v", c)
	}
}

// atlasScene: a 2-triangle body quad mapped to UV region [0,0.5]x[0,0.5].
func atlasScene(tex string) scene.Scene {
	return scene.Scene{
		Vertices: []scene.Vertex{
			{Position: scene.Vec3{X: 0, Y: 0, Z: 0}, UV: scene.Vec2{X: .02, Y: .02}},
			{Position: scene.Vec3{X: 0, Y: 4, Z: 0}, UV: scene.Vec2{X: .48, Y: .02}},
			{Position: scene.Vec3{X: 0, Y: 4, Z: 1}, UV: scene.Vec2{X: .48, Y: .48}},
			{Position: scene.Vec3{X: 0, Y: 0, Z: 1}, UV: scene.Vec2{X: .02, Y: .48}},
		},
		Triangles: []scene.Triangle{{A: 0, B: 1, C: 2}, {A: 0, B: 2, C: 3}},
		Materials: []scene.Material{{Alias: "mat_0000_car", Texture: tex}},
	}
}

func writeAtlas(t *testing.T, dir, name string, paint color.NRGBA) {
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			c := color.NRGBA{18, 18, 20, 255} // interior / trim: 3/4 of the atlas
			if x < 16 && y < 16 {
				c = paint
				if x == 4 {
					c = color.NRGBA{uint8(float64(paint.R) * .7), uint8(float64(paint.G) * .7), uint8(float64(paint.B) * .7), 255}
				}
			}
			im.SetNRGBA(x, y, c)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), encodeDDS(im), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRecolorColouredPaintInDarkAtlas(t *testing.T) {
	d := t.TempDir()
	writeAtlas(t, d, "car.dds", color.NRGBA{170, 24, 28, 255}) // red car, black-dominated atlas
	sc := atlasScene("car.dds")
	blue, _ := paintByID("mavi")
	if !newRecolorer(d).recolorScene(&sc, blue) {
		t.Fatal("red paint in a dark atlas must be recoloured")
	}
	im, err := decodeTextureFile(filepath.Join(d, sc.Materials[0].Texture))
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA) }
	if c := at(8, 8); c.B < 140 || c.R > 70 {
		t.Fatalf("paint not blue: %v", c)
	}
	if c := at(24, 24); c.R > 30 || c.B > 30 {
		t.Fatalf("interior changed: %v", c)
	}
	if s, p := at(4, 8), at(8, 8); s.B >= p.B {
		t.Fatalf("shading lost: %v vs %v", s, p)
	}
}

func TestRecolorRetintsDiffusePaint(t *testing.T) {
	d := t.TempDir()
	writeAtlas(t, d, "grey.dds", color.NRGBA{200, 200, 200, 255})
	sc := atlasScene("car_t9e161a.png")
	sc.Materials[0].BaseTexture = "grey.dds"
	green, _ := paintByID("yesil")
	if !newRecolorer(d).recolorScene(&sc, green) {
		t.Fatal("tinted paint must be re-tinted")
	}
	im, err := decodeTextureFile(filepath.Join(d, sc.Materials[0].Texture))
	if err != nil {
		t.Fatal(err)
	}
	if c := color.NRGBAModel.Convert(im.At(8, 8)).(color.NRGBA); c.G <= c.R || c.G <= c.B {
		t.Fatalf("not green: %v", c)
	}
}
