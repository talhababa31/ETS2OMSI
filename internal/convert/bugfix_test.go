package convert

import (
	"testing"

	"ets2omsi/internal/scene"
)

func TestASCIIFileStem(t *testing.T) {
	for in, want := range map[string]string{
		"tableau de bord": "tableau_de_bord",
		"światła_tył":     "_wiat_a_ty_",
		"kırmızı kapı":    "kirmizi_kapi",
		"body":            "body",
		"???":             "texture",
	} {
		got := asciiFileStem(in)
		if want == "_wiat_a_ty_" {
			want = "wiat_a_ty" // leading/trailing '_' trimmed
		}
		if got != want {
			t.Fatalf("%q -> %q want %q", in, got, want)
		}
		for _, r := range got {
			if r > 127 {
				t.Fatalf("non-ASCII in %q", got)
			}
		}
	}
}

func TestFlareAndShadowAreInvisibleEvenWhenTextured(t *testing.T) {
	d := t.TempDir()
	sc := scene.Scene{Materials: []scene.Material{
		{Alias: "mat_0000_lamp_flare", Effect: "eut2.flare"},
		{Alias: "mat_0001_car_shadow", Effect: "eut2.shadowonly"},
		{Alias: "mat_0002_body", Effect: "eut2.dif.spec.add.env"},
	}}
	idx := map[string]string{"vehicle/a/flare": "flare.dds", "vehicle/a/shadow": "shadow.dds", "vehicle/a/body": "body.dds"}
	hints := map[string][]string{"mat_0000_lamp_flare": {"/vehicle/a/flare"}, "mat_0001_car_shadow": {"/vehicle/a/shadow"}, "mat_0002_body": {"/vehicle/a/body"}}
	tr := TextureReport{}
	w := []string{}
	applyMaterials(&sc, hints, idx, d, &tr, &w)
	for i := 0; i < 2; i++ {
		if m := sc.Materials[i]; m.Texture != "gen_invisible.dds" || !m.Alpha {
			t.Fatalf("helper %d: %+v", i, m)
		}
	}
	if m := sc.Materials[2]; m.Texture != "body.dds" || m.Alpha {
		t.Fatalf("body: %+v", m)
	}
	if !effectUsesAlpha("eut2.dif.decal.over") {
		t.Fatal("decals must use alpha")
	}
}
