package convert

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"ets2omsi/internal/o3d"
	"ets2omsi/internal/omsi"
	"ets2omsi/internal/scene"
)

const plateFontChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// inkPixels renders s on a white canvas and counts the dark pixels.
func inkPixels(s string) int {
	pc := &plateCanvas{im: image.NewNRGBA(image.Rect(0, 0, 200, 100)), sx: 2, sy: 2}
	for i := range pc.im.Pix {
		pc.im.Pix[i] = 255
	}
	pc.text(s, 50, 25, 40, color.NRGBA{0, 0, 0, 255})
	n := 0
	for i := 0; i < len(pc.im.Pix); i += 4 {
		if pc.im.Pix[i] < 128 {
			n++
		}
	}
	return n
}

func TestPlateFontRendersEveryGlyph(t *testing.T) {
	seen := map[string]rune{}
	for _, r := range plateFontChars {
		g, ok := plateFontGlyphs[r]
		if !ok {
			t.Fatalf("glyph %q missing", r)
		}
		rows := strings.Split(g, "/")
		if len(rows) != fontRows {
			t.Fatalf("glyph %q has %d rows", r, len(rows))
		}
		for _, row := range rows {
			if len(row) != len(rows[0]) {
				t.Fatalf("glyph %q is ragged: %q", r, g)
			}
		}
		if o, dup := seen[g]; dup {
			t.Fatalf("glyphs %q and %q are identical", o, r)
		}
		seen[g] = r
		if n := inkPixels(string(r)); n < 60 {
			t.Fatalf("glyph %q renders only %d ink pixels", r, n)
		}
	}
	if n := inkPixels(" "); n != 0 {
		t.Fatalf("space must render no ink, got %d", n)
	}
	if a, b := inkPixels("A"), inkPixels("AA"); b < 2*a-4 {
		t.Fatalf("second glyph missing: %d vs %d", a, b)
	}
}

func TestPlateTextFormats(t *testing.T) {
	tr := regexp.MustCompile(`^(0[1-9]|[1-7][0-9]|8[01]) ([A-Z] [1-9][0-9]{3}|[A-Z]{2} [1-9][0-9]{2,3}|[A-Z]{3} [1-9][0-9]{1,2})$`)
	de := regexp.MustCompile(`^[A-Z]{1,3} [A-Z]{1,2} [1-9][0-9]{0,3}$`)
	texts := map[string]bool{}
	for i := 0; i < 400; i++ {
		seed := "vehicle." + string(rune('a'+i%26)) + "|" + strings.Repeat("x", i/26)
		a, b := plateText(PlateTR, seed), plateText(PlateDE, seed)
		if !tr.MatchString(a) || strings.ContainsAny(a, "QWX") {
			t.Fatalf("bad Turkish plate %q", a)
		}
		if !de.MatchString(b) || len(strings.ReplaceAll(b, " ", "")) > 8 {
			t.Fatalf("bad German plate %q", b)
		}
		if a != plateText(PlateTR, seed) || b != plateText(PlateDE, seed) {
			t.Fatal("plate text is not deterministic")
		}
		texts[a] = true
	}
	if len(texts) < 380 {
		t.Fatalf("only %d different Turkish plates out of 400", len(texts))
	}
	for _, s := range []string{"", "TR", "bogus"} {
		if NormalizePlateStyle(s) != PlateTR {
			t.Fatalf("%q must default to Turkish", s)
		}
	}
	if NormalizePlateStyle(" DE ") != PlateDE || NormalizePlateStyle("none") != PlateNone {
		t.Fatal("style ids")
	}
}

func TestRenderPlateStyles(t *testing.T) {
	enc := func(style, text string) []byte {
		var b bytes.Buffer
		_ = png.Encode(&b, renderPlate(style, text, plateAspectEU))
		return b.Bytes()
	}
	if !bytes.Equal(enc(PlateTR, "34 ABC 123"), enc(PlateTR, "34 ABC 123")) {
		t.Fatal("plate image is not deterministic")
	}
	for _, c := range []struct{ style, text string }{{PlateTR, "34 ABC 123"}, {PlateDE, "B AB 1234"}} {
		im := renderPlate(c.style, c.text, plateAspectEU)
		if im.Bounds().Dx() != plateTexW || im.Bounds().Dy() != plateTexH {
			t.Fatalf("size %v", im.Bounds())
		}
		if band := im.NRGBAAt(20, plateTexH*3/4); band.B < 120 || band.R > 60 {
			t.Fatalf("%s: no blue band: %v", c.style, band)
		}
		if bg := im.NRGBAAt(plateTexW-20, 15); bg.R < 220 || bg.B < 220 {
			t.Fatalf("%s: plate not white: %v", c.style, bg)
		}
		ink := 0
		for x := 100; x < plateTexW-20; x++ {
			if p := im.NRGBAAt(x, plateTexH/2); p.R < 80 && p.B < 80 {
				ink++
			}
		}
		if ink < 40 {
			t.Fatalf("%s: registration not drawn (%d ink pixels)", c.style, ink)
		}
	}
	if bytes.Equal(enc(PlateTR, "34 ABC 123"), enc(PlateTR, "06 ABC 123")) {
		t.Fatal("text does not change the image")
	}
	for _, s := range PlateStyles {
		if b, err := PlateSamplePNG(s.ID); err != nil || len(b) == 0 {
			t.Fatalf("sample %s: %v", s.ID, err)
		}
		if (s.ID == PlateNone) != (s.Example == "") {
			t.Fatalf("example of %s: %q", s.ID, s.Example)
		}
	}
}

// plateScene: a body quad and one plate quad per car end (aspect 4.7); the
// front plate shares a vertex with a body triangle.
func plateScene() scene.Scene {
	v := func(x, y, z float64) scene.Vertex {
		return scene.Vertex{Position: scene.Vec3{X: x, Y: y, Z: z}, UV: scene.Vec2{X: .25, Y: .75}}
	}
	sc := scene.Scene{
		Materials: []scene.Material{{Alias: "mat_0000_body"}, {Alias: "mat_0001_license_plate"}},
		Vertices: []scene.Vertex{
			v(-.9, -2, 0), v(.9, -2, 0), v(.9, 2, 1.2), v(-.9, 2, 1.2),
			v(-.26, 2, .3), v(.26, 2, .3), v(.26, 2, .41), v(-.26, 2, .41),
			v(-.26, -2, .5), v(.26, -2, .5), v(.26, -2, .61), v(-.26, -2, .61),
		},
	}
	sc.Triangles = []scene.Triangle{{A: 0, B: 1, C: 2}, {A: 0, B: 2, C: 4}}
	for _, q := range []int{4, 8} {
		sc.Triangles = append(sc.Triangles, scene.Triangle{A: q, B: q + 1, C: q + 2, Material: 1}, scene.Triangle{A: q, B: q + 2, C: q + 3, Material: 1})
	}
	return sc
}

func TestPlateMakerTexturesPlates(t *testing.T) {
	tex := t.TempDir()
	sc := plateScene()
	if !isPlateMaterial(&sc, 1, "") || isPlateMaterial(&sc, 0, "") {
		t.Fatal("plate material detection")
	}
	lamp := scene.Scene{Materials: []scene.Material{{Alias: "plate_light"}}}
	if isPlateMaterial(&lamp, 0, "") {
		t.Fatal("a plate lamp is not a plate")
	}
	pm := newPlateMaker(tex, PlateTR, "vehicle.test_car")
	if n := pm.prepare(&sc, nil); n != 1 {
		t.Fatalf("plate materials = %d", n)
	}
	if un := pm.handled([]string{"mat_0001_license_plate", "mat_0000_body"}); len(un) != 1 || un[0] != "mat_0000_body" {
		t.Fatalf("handled = %v", un)
	}
	name := sc.Materials[1].Texture
	if !strings.HasPrefix(name, "gen_plate_tr_") || !strings.HasSuffix(name, ".dds") || sc.Materials[0].Texture != "" {
		t.Fatalf("textures: %+v", sc.Materials)
	}
	if asciiFileStem(strings.TrimSuffix(name, ".dds")) != strings.TrimSuffix(name, ".dds") {
		t.Fatalf("texture name not ASCII-safe: %s", name)
	}
	im, err := decodeTextureFile(filepath.Join(tex, name))
	if err != nil || im.Bounds().Dx() != plateTexW {
		t.Fatalf("plate texture: %v", err)
	}
	if pm.text("") == "" || !strings.Contains(name, strings.ReplaceAll(pm.text(""), " ", "")) {
		t.Fatalf("registration %q vs texture %s", pm.text(""), name)
	}
	if pm.aspect < 4.5 || pm.aspect > 4.9 {
		t.Fatalf("aspect = %v", pm.aspect)
	}
	// The body keeps its UVs: the shared vertex was copied for the plate.
	if sc.Vertices[4].UV != (scene.Vec2{X: .25, Y: .75}) || len(sc.Vertices) != 13 {
		t.Fatalf("shared vertex: uv=%v n=%d", sc.Vertices[4].UV, len(sc.Vertices))
	}
	// Plate UVs cover 0..1, the front plate mirrored so it reads from outside.
	for _, tri := range sc.Triangles[2:] {
		for _, vi := range []int{tri.A, tri.B, tri.C} {
			uv := sc.Vertices[vi].UV
			if uv.X < 0 || uv.X > 1 || uv.Y < 0 || uv.Y > 1 {
				t.Fatalf("plate uv %v", uv)
			}
		}
	}
	front, rear := sc.Vertices[sc.Triangles[2].B], sc.Vertices[sc.Triangles[4].B] // x = +.26, bottom
	if front.UV != (scene.Vec2{X: 0, Y: 1}) || rear.UV != (scene.Vec2{X: 1, Y: 1}) {
		t.Fatalf("front %v rear %v", front.UV, rear.UV)
	}
	// Deterministic per vehicle, different per variant and per vehicle.
	again := newPlateMaker(t.TempDir(), PlateTR, "vehicle.test_car")
	sc2 := plateScene()
	again.prepare(&sc2, nil)
	if sc2.Materials[1].Texture != name {
		t.Fatalf("not deterministic: %s vs %s", sc2.Materials[1].Texture, name)
	}
	if pm.texture("siyah") == name || pm.text("siyah") == "" {
		t.Fatal("variant must get its own registration")
	}
	if other := newPlateMaker(t.TempDir(), PlateTR, "vehicle.other_car"); other.texture("") == name {
		t.Fatal("different vehicles share a registration")
	}
	// No plate style: nothing changes.
	sc3 := plateScene()
	if pm := newPlateMaker(tex, PlateNone, "x"); pm != nil || pm.prepare(&sc3, nil) != 0 || sc3.Materials[1].Texture != "" || pm.text("") != "" {
		t.Fatal("PlateNone must leave plates alone")
	}
}

func TestColorVariantsGetOwnPlates(t *testing.T) {
	stage := t.TempDir()
	tex := filepath.Join(stage, "texture")
	_ = os.MkdirAll(tex, 0755)
	_ = os.MkdirAll(filepath.Join(stage, "model"), 0755)
	writeBodyTexture(t, tex)
	sc := plateScene()
	sc.Materials[0].Texture = "body.dds"
	pm := newPlateMaker(tex, PlateDE, "vehicle.test_car")
	pm.prepare(&sc, nil)
	cm := convertedModel{o3dName: "body.o3d", sc: sc}
	spec := omsi.VehicleSpec{Name: "Car", Type: "car", Materials: materialOverrides(cm.sc)}
	vs, _ := exportColorVariants(stage, spec, []convertedModel{cm}, newTextureResolver(nil, tex), newOpaqueFixer(tex), pm, tex, []string{"siyah", "lacivert"})
	if len(vs) != 2 {
		t.Fatalf("variants = %+v", vs)
	}
	seen := map[string]bool{pm.text(""): true}
	for _, v := range vs {
		if v.Plate == "" || seen[v.Plate] {
			t.Fatalf("variant %s plate %q", v.ID, v.Plate)
		}
		seen[v.Plate] = true
		raw, err := os.ReadFile(filepath.Join(stage, "model", "body_"+v.ID+".o3d"))
		if err != nil {
			t.Fatal(err)
		}
		m, err := o3d.Parse(raw)
		if err != nil || len(m.Materials) != 2 || m.Materials[1].Texture != pm.texture(v.ID) {
			t.Fatalf("variant %s O3D materials: %v %+v", v.ID, err, m.Materials)
		}
	}
	if miss := validateAllO3DTextureRefs(stage); len(miss) != 0 {
		t.Fatalf("missing textures: %v", miss)
	}
	// One plate texture for the vehicle and one per variant.
	if len(pm.files) != 3 {
		t.Fatalf("plate files = %v", pm.files)
	}
}
