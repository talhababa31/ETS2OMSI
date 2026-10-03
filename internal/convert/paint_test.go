package convert

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ets2omsi/internal/scene"
)

// Regression for black panels in OMSI: an unresolved paint material must take
// the car's own dominant body colour, not a near-black placeholder.
func TestUnresolvedPaintUsesDominantBodyColour(t *testing.T) {
	d := t.TempDir()
	im := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			c := color.NRGBA{235, 235, 238, 255} // white paint
			if x < 3 {
				c = color.NRGBA{20, 20, 20, 255} // some trim
			}
			im.SetNRGBA(x, y, c)
		}
	}
	f, _ := os.Create(filepath.Join(d, "body.png"))
	_ = png.Encode(f, im)
	_ = f.Close()
	sc := scene.Scene{
		Materials: []scene.Material{
			{Alias: "mat_0000_body", Texture: "body.png", Class: "body"},
			{Alias: "mat_0001_paint", Class: "paint"},
		},
		Triangles: []scene.Triangle{{Material: 0}, {Material: 0}, {Material: 1}},
	}
	tr := TextureReport{}
	warnings := []string{}
	applySafeMaterialFallbacks(&sc, []string{"mat_0001_paint"}, d, &tr, &warnings)
	got := sc.Materials[1].Texture
	if !strings.HasPrefix(got, "fallback_paint_") {
		t.Fatalf("paint fallback=%q", got)
	}
	pi, err := decodeTextureFile(filepath.Join(d, got))
	if err != nil {
		t.Fatal(err)
	}
	c := color.NRGBAModel.Convert(pi.At(0, 0)).(color.NRGBA)
	if c.R < 220 || c.G < 220 || c.B < 220 {
		t.Fatalf("paint colour %v is not the body colour", c)
	}
}

func TestBodyFallbackWithoutInfoIsNotBlack(t *testing.T) {
	d := t.TempDir()
	sc := scene.Scene{Materials: []scene.Material{{Alias: "mat_0000_body", Class: "body"}}}
	tr := TextureReport{}
	w := []string{}
	applySafeMaterialFallbacks(&sc, []string{"mat_0000_body"}, d, &tr, &w)
	im, err := decodeTextureFile(filepath.Join(d, sc.Materials[0].Texture))
	if err != nil {
		t.Fatal(err)
	}
	if c := color.NRGBAModel.Convert(im.At(0, 0)).(color.NRGBA); c.R < 150 {
		t.Fatalf("body fallback too dark: %v", c)
	}
}
