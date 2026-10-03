package convert

import (
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ets2omsi/internal/scene"
)

// makeTOBJ builds a texture object in the ETS2 binary layout.
func makeTOBJ(img string, addrU, addrV byte) string {
	b := make([]byte, 48+len(img))
	binary.LittleEndian.PutUint32(b, tobjMagic)
	b[24] = 2 // generic
	b[30], b[31] = addrU, addrV
	binary.LittleEndian.PutUint32(b[40:], uint32(len(img)))
	copy(b[48:], img)
	return string(b)
}

// 2x1 legacy uncompressed DDS: left pixel red, right pixel blue.
func makeDDS() string {
	im := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	im.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	im.SetNRGBA(1, 0, color.NRGBA{0, 0, 255, 255})
	return string(encodeDDS(im))
}

func TestTextureResolverDeterministic(t *testing.T) {
	px := makeDDS()
	pkg := writeTestSCS(t, map[string]string{
		"vehicle/ai/meg/body.tobj":            makeTOBJ("/vehicle/ai/meg/body.dds", 0, 0),
		"vehicle/ai/meg/body.dds":             px,
		"vehicle/ai/meg/tableau de bord.tobj": makeTOBJ("/vehicle/ai/meg/tableau de bord.dds", 0, 0),
		"vehicle/ai/meg/tableau de bord.dds":  px,
		"vehicle/ai/meg/side.tobj":            makeTOBJ("side.dds", 4, 0), // relative path, mirror U
		"vehicle/ai/meg/side.dds":             px,
		"vehicle/ai/boxer/cargocolor.dds":     px, // no .tobj
		"vehicle/ai/other/glass_ex.dds":       px, // same name elsewhere: must NOT be used
	})
	r := newTextureResolver([]string{pkg}, t.TempDir())
	defer r.Close()
	if !r.Native() {
		t.Fatal("zip package must be readable natively")
	}
	for ref, wantOK := range map[string]bool{
		"/vehicle/ai/meg/body":          true,
		"/vehicle/ai/meg/tableaudebord": true, // ConverterPIX drops spaces
		"/vehicle/ai/meg/side":          true,
		"/vehicle/ai/boxer/cargocolor":  true,
		"/vehicle/truck/share/glass_ex": false, // base.scs only
		"/vehicle/ai/meg/nothing":       false,
	} {
		x := r.resolve(ref)
		if (x.output != "") != wantOK {
			t.Fatalf("%s: output=%q diag=%+v", ref, x.output, x.diag)
		}
		if !wantOK && (x.diag.Status != "missing" || x.diag.Reason == "") {
			t.Fatalf("%s: missing reason not explained: %+v", ref, x.diag)
		}
		if strings.Contains(x.output, " ") {
			t.Fatalf("space in OMSI texture name %q", x.output)
		}
	}
	side := r.resolve("/vehicle/ai/meg/side")
	if side.addrU != addrMirror || side.diag.Addressing != "mirror/repeat" {
		t.Fatalf("mirror not read from tobj: %+v", side)
	}
	// Mirrored texture is doubled: red blue | blue red.
	b, _ := os.ReadFile(filepath.Join(r.texDir, side.output))
	im, err := decodeImageBytes(b, ".dds")
	if err != nil {
		t.Fatal(err)
	}
	if im.Bounds().Dx() != 4 {
		t.Fatalf("mirror tile width %d", im.Bounds().Dx())
	}
	want := []color.NRGBA{{255, 0, 0, 255}, {0, 0, 255, 255}, {0, 0, 255, 255}, {255, 0, 0, 255}}
	for x, w := range want {
		if c := color.NRGBAModel.Convert(im.At(x, 0)).(color.NRGBA); c != w {
			t.Fatalf("x=%d got %v want %v", x, c, w)
		}
	}
	// Plain legacy DDS is copied unchanged.
	body := r.resolve("/vehicle/ai/meg/body")
	if got, _ := os.ReadFile(filepath.Join(r.texDir, body.output)); string(got) != px {
		t.Fatal("legacy DDS must be copied byte-for-byte")
	}
}

func TestPrepareSceneMirrorHalvesUAndExplains(t *testing.T) {
	pkg := writeTestSCS(t, map[string]string{
		"vehicle/c/side.tobj": makeTOBJ("/vehicle/c/side.dds", 4, 0),
		"vehicle/c/side.dds":  makeDDS(),
	})
	r := newTextureResolver([]string{pkg}, t.TempDir())
	defer r.Close()
	sc := scene.Scene{
		Vertices:  []scene.Vertex{{UV: scene.Vec2{X: 1.5, Y: .25}}, {UV: scene.Vec2{X: -.5, Y: .5}}, {UV: scene.Vec2{X: .5, Y: 1}}},
		Triangles: []scene.Triangle{{A: 0, B: 1, C: 2, Material: 1}},
		Materials: []scene.Material{{Alias: "mat_0000_glass_ex"}, {Alias: "mat_0001_side"}},
	}
	hints := map[string][]string{"mat_0000_glass_ex": {"/vehicle/truck/share/glass_ex"}, "mat_0001_side": {"/vehicle/c/side"}}
	idx, diags := r.prepareScene(&sc, hints, "body.o3d")
	if sc.Vertices[0].UV.X != .75 || sc.Vertices[1].UV.X != -.25 || sc.Vertices[0].UV.Y != .25 {
		t.Fatalf("mirror UVs: %+v", sc.Vertices)
	}
	if len(diags) != 2 || diags[0].Status != "missing" || diags[1].Status != "ok" || diags[1].Image != "/vehicle/c/side.dds" {
		t.Fatalf("diags=%+v", diags)
	}
	tr := TextureReport{}
	w := []string{}
	un := applyMaterials(&sc, hints, idx, r.texDir, &tr, &w)
	if len(un) != 1 || sc.Materials[1].Texture == "" {
		t.Fatalf("unresolved=%v mats=%+v", un, sc.Materials)
	}
}

func TestDecodeTGA(t *testing.T) {
	// 1x2 uncompressed 24-bit, bottom-up: first stored row is the bottom.
	b := make([]byte, 18, 24)
	b[2], b[12], b[14], b[16] = 2, 1, 2, 24
	b = append(b, 0, 0, 255 /*red bottom*/, 255, 0, 0 /*blue top*/)
	im, err := decodeTGA(b)
	if err != nil {
		t.Fatal(err)
	}
	if c := im.At(0, 0).(color.NRGBA); c.B != 255 || c.R != 0 {
		t.Fatalf("top pixel %v", c)
	}
}
