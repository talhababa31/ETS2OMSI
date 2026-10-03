package convert

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"ets2omsi/internal/scanner"
	"ets2omsi/internal/scene"
)

func writeTestSCS(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "pack.scs")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTextureTightKey(t *testing.T) {
	a := textureTightKey("/vehicle/ai/jazzycat/renault_megane_2/tableau de bord.dds")
	b := textureTightKey("vehicle/ai/jazzycat/renault_megane_2/tableaudebord")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
	if textureTightBase("/x/y/Megane Phare_L.tobj") != "meganepharel" {
		t.Fatalf("base=%q", textureTightBase("/x/y/Megane Phare_L.tobj"))
	}
}

// Jazzycat regression: files with spaces, images without .tobj and
// extensionless PIT references must all be extracted from the package itself.
func TestResolvePackageTexturesSpacesNoTobjAndExtensionless(t *testing.T) {
	pkg := writeTestSCS(t, map[string]string{
		"vehicle/ai/jazzycat/renault_megane_2/tableau de bord.dds": "TDB",
		"vehicle/ai/jazzycat/renault_megane_2/megane phare.dds":    "PHARE",
		"vehicle/ai/jazzycat/renault_megane_2/megane phare_l.dds":  "PHARE_L",
		"vehicle/ai/peugeot_boxer/cargocolor.dds":                  "CARGO",
		"vehicle/ai/jazzycat/vw_lt/kolor wheel.tobj":               "\x01\x0a\xb1\x70\x00\x00\x00\x00\x24\x00\x00\x00/vehicle/ai/jazzycat/vw_lt/kolor wheel.dds",
		"vehicle/ai/jazzycat/vw_lt/kolor wheel.dds":                "KOLOR",
		"vehicle/a/white.dds":                                      "W1",
		"vehicle/b/white.dds":                                      "W2",
	})
	out := t.TempDir()
	refs := []string{
		"/vehicle/ai/jazzycat/renault_megane_2/tableaudebord",
		"/vehicle/ai/jazzycat/renault_megane_2/meganephare",
		"/vehicle/ai/peugeot_boxer/cargocolor",
		"/vehicle/ai/jazzycat/vw_lt/kolorwheel.tobj",
		"/vehicle/c/white",              // ambiguous basename -> must not resolve
		"/vehicle/truck/share/glass_ex", // base.scs only -> must not resolve
	}
	n, handled := resolvePackageTextures([]string{pkg}, refs, out)
	if n != 4 {
		t.Fatalf("resolved=%d handled=%v", n, handled)
	}
	if handled["/vehicle/c/white"] || handled["/vehicle/truck/share/glass_ex"] {
		t.Fatalf("unexpected handled=%v", handled)
	}
	idx := map[string]string{}
	r := collectTextures([]string{out}, t.TempDir(), idx)
	if r.Copied != 4 {
		t.Fatalf("copied=%d files=%v", r.Copied, r.Files)
	}
	for _, f := range r.Files {
		if filepath.Base(f) != f || containsSpace(f) {
			t.Fatalf("unsafe OMSI texture name %q", f)
		}
	}
	for ref, want := range map[string]string{
		"/vehicle/ai/jazzycat/renault_megane_2/tableaudebord": "tableaudebord.dds",
		"/vehicle/ai/jazzycat/renault_megane_2/meganephare":   "meganephare.dds",
		"/vehicle/ai/peugeot_boxer/cargocolor":                "cargocolor.dds",
		"/vehicle/ai/jazzycat/vw_lt/kolorwheel.tobj":          "kolorwheel.dds",
	} {
		if got := lookupTexture(idx, ref); got != want {
			t.Fatalf("%s -> %q want %q", ref, got, want)
		}
	}
	// Material alias path: ConverterPIX alias stems resolve through the index.
	sc := scene.Scene{Materials: []scene.Material{{Alias: "mat_0006_tableaudebord", Effect: "eut2.dif"}}}
	tr := TextureReport{}
	warnings := []string{}
	hints := map[string][]string{"mat_0006_tableaudebord": {"/vehicle/ai/jazzycat/renault_megane_2/tableaudebord"}}
	if un := applyMaterials(&sc, hints, idx, t.TempDir(), &tr, &warnings); len(un) != 0 || sc.Materials[0].Texture != "tableaudebord.dds" {
		t.Fatalf("unresolved=%v material=%+v", un, sc.Materials[0])
	}
}

func TestLookupTextureTightMatchesSpacedFile(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "vehicle", "x", "porte ar.dds")
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, []byte("x"), 0644)
	idx := map[string]string{}
	collectTextures([]string{root}, t.TempDir(), idx)
	if got := lookupTexture(idx, "/vehicle/x/portear"); got != "porte_ar.dds" {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePackageTexturesSkipsUnopenableMount(t *testing.T) {
	n, h := resolvePackageTextures([]string{filepath.Join(t.TempDir(), "missing.scs")}, []string{"/a/b"}, t.TempDir())
	if n != 0 || len(h) != 0 {
		t.Fatalf("n=%d h=%v", n, h)
	}
}

func TestPreviewWindingFollowsNormals(t *testing.T) {
	// One triangle wound clockwise (face normal -Z) with vertex normals +Z.
	sc := scene.Scene{
		Vertices: []scene.Vertex{
			{Position: scene.Vec3{X: 0, Y: 0, Z: 0}, Normal: scene.Vec3{Z: 1}},
			{Position: scene.Vec3{X: 0, Y: 1, Z: 0}, Normal: scene.Vec3{Z: 1}},
			{Position: scene.Vec3{X: 1, Y: 0, Z: 0}, Normal: scene.Vec3{Z: 1}},
		},
		Triangles: []scene.Triangle{{A: 0, B: 1, C: 2}},
		Materials: []scene.Material{{Alias: "body"}},
	}
	p := previewFromScene(scanner.Vehicle{ID: "x"}, sc)
	if len(p.Indices) != 3 {
		t.Fatalf("indices=%v", p.Indices)
	}
	pos := func(i uint32) scene.Vec3 {
		return scene.Vec3{X: float64(p.Positions[i*3]), Y: float64(p.Positions[i*3+1]), Z: float64(p.Positions[i*3+2])}
	}
	a, b, c := pos(p.Indices[0]), pos(p.Indices[1]), pos(p.Indices[2])
	nz := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
	if nz <= 0 {
		t.Fatalf("preview face still opposes its normals (nz=%v)", nz)
	}
}

func containsSpace(s string) bool {
	for _, r := range s {
		if r == ' ' {
			return true
		}
	}
	return false
}
