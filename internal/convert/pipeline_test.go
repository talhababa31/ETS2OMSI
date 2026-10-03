package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ets2omsi/internal/scanner"
	"ets2omsi/internal/scene"
)

func TestConversionAssetsPreferSemanticMainAndLOD(t *testing.T) {
	v := scanner.Vehicle{Models: []string{"/vehicle/car/detail.pmd", "/vehicle/car/lod.pmd", "/vehicle/car/main.pmd"}, ModelAssets: []scanner.ModelAsset{
		{Path: "/vehicle/car/detail.pmd", Role: "detail"},
		{Path: "/vehicle/car/lod.pmd", Role: "lod"},
		{Path: "/vehicle/car/main.pmd", Role: "main"},
	}}
	got := conversionAssets(v)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if got[0].Role != "main" || got[0].Path != "/vehicle/car/main.pmd" {
		t.Fatalf("main=%+v", got[0])
	}
	if got[1].Role != "lod" {
		t.Fatalf("lod=%+v", got[1])
	}
}

func TestBuildManifest(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "model"), 0755)
	_ = os.WriteFile(filepath.Join(d, "model", "body.o3d"), []byte("abc"), 0644)
	got := buildManifest(d)
	if len(got) != 1 || got[0].Path != "model/body.o3d" || got[0].SHA256 == "" {
		t.Fatalf("%+v", got)
	}
}

func TestTexturelessGlassGetsSemanticOMSIGlass(t *testing.T) {
	d := t.TempDir()
	sc := scene.Scene{Materials: []scene.Material{{Index: 0, Alias: "mat_0001_glass_ex", Effect: "eut2.glass"}}}
	tr := TextureReport{}
	warnings := []string{}
	unresolved := applyMaterials(&sc, map[string][]string{}, map[string]string{}, d, &tr, &warnings)
	if len(unresolved) != 0 {
		t.Fatalf("textureless glass must not be unresolved: %#v", unresolved)
	}
	if sc.Materials[0].Texture != "gen_glass.dds" || !sc.Materials[0].Alpha || sc.Materials[0].Class != "glass" {
		t.Fatalf("material=%+v", sc.Materials[0])
	}
	if _, err := os.Stat(filepath.Join(d, "gen_glass.dds")); err != nil {
		t.Fatalf("semantic glass not generated: %v", err)
	}
}

func TestExplicitMissingGlassTextureStillFails(t *testing.T) {
	d := t.TempDir()
	sc := scene.Scene{Materials: []scene.Material{{Index: 0, Alias: "mat_0001_glass_ex", Effect: "eut2.glass"}}}
	tr := TextureReport{}
	warnings := []string{}
	hints := map[string][]string{"mat_0001_glass_ex": {"/vehicle/car/glass.tobj"}}
	unresolved := applyMaterials(&sc, hints, map[string]string{}, d, &tr, &warnings)
	if len(unresolved) != 1 || unresolved[0] != "mat_0001_glass_ex" {
		t.Fatalf("explicit missing glass texture must remain fatal: %#v", unresolved)
	}
}

func TestSafeFallbackConvertsUnresolvedBodyAndGlass(t *testing.T) {
	d := t.TempDir()
	sc := scene.Scene{Materials: []scene.Material{
		{Index: 0, Alias: "mat_0000_body", Effect: "eut2.dif"},
		{Index: 1, Alias: "mat_0001_glass_ex", Effect: "eut2.glass"},
	}}
	tr := TextureReport{}
	warnings := []string{}
	applySafeMaterialFallbacks(&sc, []string{"mat_0000_body", "mat_0001_glass_ex"}, d, &tr, &warnings)
	if sc.Materials[0].Texture != "fallback_body.png" {
		t.Fatalf("body fallback=%q", sc.Materials[0].Texture)
	}
	if sc.Materials[1].Texture != "gen_glass.dds" || !sc.Materials[1].Alpha {
		t.Fatalf("glass fallback=%+v", sc.Materials[1])
	}
	for _, name := range []string{"fallback_body.png", "gen_glass.dds"} {
		if _, err := os.Stat(filepath.Join(d, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	// Glass fallback is by design and must not be a warning; the body one is.
	if len(warnings) != 1 || !strings.Contains(warnings[0], "mat_0000_body") {
		t.Fatalf("warnings=%#v", warnings)
	}
	// A second pass (LOD with the same material) must not duplicate it.
	sc.Materials[0].Texture = ""
	applySafeMaterialFallbacks(&sc, []string{"mat_0000_body"}, d, &tr, &warnings)
	if len(warnings) != 1 {
		t.Fatalf("duplicate warnings=%#v", warnings)
	}
}

func TestEffectUsesAlpha(t *testing.T) {
	for eff, want := range map[string]bool{
		"eut2.dif.spec.add.env":           false, // car body: alpha = spec mask
		"eut2.dif.spec.add.env.nofresnel": false,
		"eut2.dif.a":                      true,
		"eut2.dif.spec.a.over":            true,
		"eut2.dif.blend_over":             true,
		"eut2.glass":                      true,
		"eut2.dif":                        false,
	} {
		if got := effectUsesAlpha(eff); got != want {
			t.Fatalf("%s: got %v want %v", eff, got, want)
		}
	}
}
