package convert

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		if !strings.Contains(string(cfg), "body_"+v.ID+".dds") {
			t.Fatalf("cfg %s does not use recoloured texture:\n%s", v.ID, cfg)
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
