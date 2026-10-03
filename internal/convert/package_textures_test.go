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
