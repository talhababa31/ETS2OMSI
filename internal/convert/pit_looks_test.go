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

// Layout follows ConverterPIX Model::saveToPit / Material::toPixDefinitionPre147.
const testPIT = `Header {
	FormatVersion: 1
	Source: "ConverterPIX"
	Type: "Trait"
	Name: "car"
}
Global {
	LookCount: 2
	VariantCount: 1
	PartCount: 1
	MaterialCount: 2
}
Look {
	Name: "default"
	Material {
		Alias: "mat_0000_body"
		Effect: "eut2.dif.spec.add.env"
		Flags: 0
		AttributeCount: 1
		TextureCount: 2
		Attribute {
			Format: FLOAT3
			Tag: "diffuse"
			Value: ( &3f800000  &00000000  &00000000 )
		}
		Texture {
			Tag: "texture[0]:texture_base"
			Value: "/vehicle/ai/car/body"
		}
		Texture {
			Tag: "texture[1]:texture_reflection"
			Value: "/vehicle/share/reflection"
		}
	}
	Material {
		Alias: "mat_0001_glass"
		Effect: "eut2.glass"
		Flags: 0
		AttributeCount: 0
		TextureCount: 0
	}
}
Look {
	Name: "blue"
	Material {
		Alias: "mat_0000_body"
		Effect: "eut2.dif.spec.add.env"
		Flags: 0
		AttributeCount: 1
		TextureCount: 1
		Attribute {
			Format: FLOAT3
			Tag: "diffuse"
			Value: ( &00000000  &00000000  &3f800000 )
		}
		Texture {
			Tag: "texture[0]:texture_base"
			Value: "/vehicle/ai/car/body_blue"
		}
	}
	Material {
		Alias: "mat_0001_glass"
		Effect: "eut2.glass"
		Flags: 0
		AttributeCount: 0
		TextureCount: 0
	}
}
`

func writeTestPIT(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "car.pit")
	if err := os.WriteFile(p, []byte(testPIT), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPITLookSelectionAndDiffuse(t *testing.T) {
	p := writeTestPIT(t)
	for _, tc := range []struct {
		look, tex string
		tint      [3]float64
	}{
		{"", "/vehicle/ai/car/body", [3]float64{1, 0, 0}},
		{"default", "/vehicle/ai/car/body", [3]float64{1, 0, 0}},
		{"BLUE", "/vehicle/ai/car/body_blue", [3]float64{0, 0, 1}},
		{"missing", "/vehicle/ai/car/body", [3]float64{1, 0, 0}},
	} {
		sc := scene.Scene{Materials: []scene.Material{{Alias: "mat_0000_body"}, {Alias: "mat_0001_glass"}}}
		hints := loadPITMaterials(&sc, p, tc.look)
		if got := hints["mat_0000_body"]; len(got) != 1 || got[0] != tc.tex {
			t.Fatalf("look %q: hints=%v", tc.look, hints)
		}
		m := sc.Materials[0]
		if !m.HasTint || m.Tint != tc.tint {
			t.Fatalf("look %q: tint=%v has=%v", tc.look, m.Tint, m.HasTint)
		}
		if sc.Materials[1].HasTint {
			t.Fatalf("glass must not be tinted")
		}
	}
	// pitHints keeps working (first/default look only, no mixing).
	if h := pitHints(p)["mat_0000_body"]; len(h) != 1 || h[0] != "/vehicle/ai/car/body" {
		t.Fatalf("pitHints=%v", h)
	}
}

func TestParsePixFloats(t *testing.T) {
	v := parsePixFloats("( &3f800000  &3f000000  0.25 )")
	if len(v) != 3 || v[0] != 1 || v[1] != .5 || v[2] != .25 {
		t.Fatalf("%v", v)
	}
}

// Grey ETS2 body texture × red diffuse must become a red OMSI texture.
func TestTintIsBakedIntoResolvedTexture(t *testing.T) {
	d := t.TempDir()
	im := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = 200, 200, 200, 255
	}
	f, _ := os.Create(filepath.Join(d, "body.png"))
	_ = png.Encode(f, im)
	_ = f.Close()
	sc := scene.Scene{Materials: []scene.Material{{Alias: "mat_0000_body", HasTint: true, Tint: [3]float64{1, 0, 0}}}}
	tr := TextureReport{}
	w := []string{}
	if un := applyMaterials(&sc, map[string][]string{"mat_0000_body": {"/vehicle/ai/car/body"}}, map[string]string{"vehicle/ai/car/body": "body.png"}, d, &tr, &w); len(un) != 0 {
		t.Fatalf("unresolved=%v", un)
	}
	m := sc.Materials[0]
	if m.HasTint || !strings.HasPrefix(m.Texture, "body_t") {
		t.Fatalf("material=%+v", m)
	}
	out, err := decodeTextureFile(filepath.Join(d, m.Texture))
	if err != nil {
		t.Fatal(err)
	}
	if c := color.NRGBAModel.Convert(out.At(1, 1)).(color.NRGBA); c.R != 200 || c.G != 0 || c.B != 0 {
		t.Fatalf("baked colour %v", c)
	}
	// O3D keeps neutral diffuse once baked.
	if o := toO3D(sc); o.Materials[0].Diffuse != [4]float32{1, 1, 1, 1} {
		t.Fatalf("diffuse=%v", o.Materials[0].Diffuse)
	}
}

func TestUnresolvedPaintUsesETS2Diffuse(t *testing.T) {
	d := t.TempDir()
	sc := scene.Scene{Materials: []scene.Material{{Alias: "mat_0000_body", Class: "body", HasTint: true, Tint: [3]float64{0, 0, 1}}}}
	tr := TextureReport{}
	w := []string{}
	applySafeMaterialFallbacks(&sc, []string{"mat_0000_body"}, d, &tr, &w)
	if got := sc.Materials[0].Texture; got != "fallback_paint_0000ff.png" {
		t.Fatalf("fallback=%q", got)
	}
}

func TestCloneDoesNotShareVertices(t *testing.T) {
	a := scene.Scene{Vertices: []scene.Vertex{{}}}
	b := a.Clone()
	b.Translate(1, 0, 0)
	b2 := a.Clone()
	b2.Translate(1, 0, 0)
	if a.Vertices[0].Position.X != 0 || b2.Vertices[0].Position.X != 1 {
		t.Fatalf("shared vertices: a=%v b2=%v", a.Vertices[0].Position, b2.Vertices[0].Position)
	}
}
