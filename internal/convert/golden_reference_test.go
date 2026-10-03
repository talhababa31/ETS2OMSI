package convert

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ets2omsi/internal/o3d"
	"ets2omsi/internal/omsi"
	"ets2omsi/internal/scene"
)

func TestGoldenO3DAxisMapping(t *testing.T) {
	sc := scene.Scene{
		Vertices: []scene.Vertex{
			{Position: scene.Vec3{X: -1, Y: -2.5, Z: .3}, Normal: scene.Vec3{Z: 1}},
			{Position: scene.Vec3{X: 1, Y: -2.5, Z: .3}, Normal: scene.Vec3{Z: 1}},
			{Position: scene.Vec3{X: 0, Y: 2.5, Z: 1.5}, Normal: scene.Vec3{Z: 1}},
		},
		Triangles: []scene.Triangle{{A: 0, B: 1, C: 2, Material: 0}},
		Materials: []scene.Material{{Index: 0, Alias: "body", Texture: "body.png"}},
	}
	m := toO3D(sc)
	if got := m.Vertices[2]; got.X != 0 || got.Y != 1.5 || got.Z != 2.5 {
		t.Fatalf("OMSI axis map = %+v; want X=internal X, Y=internal Z, Z=internal Y", got)
	}
	if got := m.Vertices[0]; got.NX != 0 || got.NY != 1 || got.NZ != 0 {
		t.Fatalf("OMSI normal map = %+v", got)
	}
}

func TestGoldenOrientationValidatorUsesXYZWidthHeightLength(t *testing.T) {
	d := t.TempDir()
	sc := scene.Scene{
		Vertices: []scene.Vertex{
			{Position: scene.Vec3{X: -1, Y: -2.5, Z: 0}}, {Position: scene.Vec3{X: 1, Y: -2.5, Z: 0}},
			{Position: scene.Vec3{X: -1, Y: 2.5, Z: 1.5}}, {Position: scene.Vec3{X: 1, Y: 2.5, Z: 1.5}},
		},
		Triangles: []scene.Triangle{{A: 0, B: 1, C: 2, Material: 0}},
		Materials: []scene.Material{{Index: 0, Alias: "body", Texture: "body.png"}},
	}
	p := filepath.Join(d, "body.o3d")
	m := toO3D(sc)
	if err := o3d.WriteFile(p, &m); err != nil {
		t.Fatal(err)
	}
	ok, dims := validateOMSIOrientation(p, sc.Bounds())
	if !ok {
		t.Fatalf("orientation rejected: %v", dims)
	}
	if dims[0] != 2 || dims[1] != 1.5 || dims[2] != 5 {
		t.Fatalf("dims=%v", dims)
	}
}

func TestExactTextureIndexDoesNotFuzzyMatch(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	for _, rel := range []string{"vehicle/a/body.dds", "vehicle/b/body.dds"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel), 0644); err != nil {
			t.Fatal(err)
		}
	}
	idx := map[string]string{}
	r := collectTextures([]string{root}, out, idx)
	if r.Copied != 2 {
		t.Fatalf("copied=%d", r.Copied)
	}
	a := lookupTexture(idx, "/vehicle/a/body.tobj")
	b := lookupTexture(idx, "/vehicle/b/body.tobj")
	if a == "" || b == "" || a == b {
		t.Fatalf("exact paths not isolated: a=%q b=%q", a, b)
	}
	if v := lookupTexture(idx, "body"); v != "" {
		t.Fatalf("ambiguous basename must not resolve: %q", v)
	}
}

func TestPITHintsPreferTextureBase(t *testing.T) {
	p := filepath.Join(t.TempDir(), "car.pit")
	txt := `Material {
 Alias: "paint"
 Texture {
  Tag: "texture[0]:texture_base"
  Value: "/vehicle/car/body.tobj"
 }
 Texture {
  Tag: "texture[1]:texture_nmap"
  Value: "/vehicle/car/body_n.tobj"
 }
}`
	if err := os.WriteFile(p, []byte(txt), 0644); err != nil {
		t.Fatal(err)
	}
	h := pitHints(p)["paint"]
	if len(h) != 1 || !strings.Contains(h[0], "body.tobj") {
		t.Fatalf("hints=%v", h)
	}
}

func TestConverterPIXAliasTextureStemIsDeterministic(t *testing.T) {
	cases := map[string]string{
		"mat_0000_body":         "body",
		"MAT_0042_Fenster":      "fenster",
		"mat_9999_vehiclelight": "vehiclelight",
		"body":                  "",
		"mat_00_body":           "",
		"mat_0000_":             "",
		"mat_0000/foo":          "",
	}
	for in, want := range cases {
		if got := converterPIXAliasTextureStem(in); got != want {
			t.Fatalf("converterPIXAliasTextureStem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGoldenGroundPlaneUsesResolvedWheelContact(t *testing.T) {
	sc := scene.Scene{Vertices: []scene.Vertex{
		{Position: scene.Vec3{X: -1, Y: -2, Z: 0.18}},
		{Position: scene.Vec3{X: 1, Y: 2, Z: 1.5}},
	}}
	wheels := map[string]omsi.Wheel{
		"FL": {Z: 0.34, Radius: 0.31},
		"FR": {Z: 0.35, Radius: 0.32},
		"RL": {Z: 0.33, Radius: 0.30},
		"RR": {Z: 0.36, Radius: 0.33},
	}
	got := groundPlane(sc, wheels)
	if math.Abs(got-0.03) > 1e-9 {
		t.Fatalf("groundPlane = %.6f, want 0.03 from wheel contact median", got)
	}
}

func TestPITHintsNeverPromoteNormalOrMaskToDiffuse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "car.pit")
	txt := `Material {
 Alias: "lamp"
 Texture { Tag: "texture[0]:texture_nmap" Value: "/vehicle/lamp_n.tobj" }
 Texture { Tag: "texture[1]:texture_mask" Value: "/vehicle/lamp_mask.tobj" }
}`
	if err := os.WriteFile(p, []byte(txt), 0644); err != nil {
		t.Fatal(err)
	}
	if h := pitHints(p)["lamp"]; len(h) != 0 {
		t.Fatalf("non-diffuse maps promoted: %v", h)
	}
}
