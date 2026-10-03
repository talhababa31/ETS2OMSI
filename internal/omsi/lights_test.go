package omsi

import (
	"strings"
	"testing"
)

func lightBlocks(cfg string) [][]string {
	out := [][]string{}
	parts := strings.Split(cfg, "[light_enh_2]\r\n")
	for _, p := range parts[1:] {
		lines := strings.Split(p, "\r\n")
		if len(lines) < 24 {
			lines = append(lines, make([]string, 24-len(lines))...)
		}
		out = append(out, lines[:24])
	}
	return out
}

func testCar() VehicleSpec {
	return VehicleSpec{
		Wheels:     map[string]Wheel{"FL": {X: -.8, Y: 1.3, Z: .3}, "FR": {X: .8, Y: 1.3, Z: .3}, "RL": {X: -.8, Y: -1.3, Z: .3}, "RR": {X: .8, Y: -1.3, Z: .3}},
		WheelFiles: map[string]string{"FL": "wheel_fl.o3d", "FR": "wheel_fr.o3d", "RL": "wheel_rl.o3d", "RR": "wheel_rr.o3d"},
		LODs:       []LOD{{ScreenSize: .035, File: "lod_1.o3d"}, {ScreenSize: .012, File: "lod_2.o3d"}},
		Lights: []PointLight{
			NewLight(LightHead, -.65, 2.2, .7, 1), NewLight(LightHead, .65, 2.2, .7, 1),
			NewLight(LightTail, -.7, -2.25, .85, -1), NewLight(LightBrake, -.7, -2.25, .85, -1),
			NewLight(LightBlinker, -.78, 2.15, .7, 1), NewLight(LightBlinker, .78, -2.2, .85, -1),
			NewLight(LightReverse, -.5, -2.25, .85, -1),
		},
	}
}

func TestLightEnh2BlockHas24LinesInStockOrder(t *testing.T) {
	cfg := ModelCFG(VehicleSpec{Lights: []PointLight{NewLight(LightBrake, -.7, -2.25, .9, -1)}})
	b := lightBlocks(cfg)
	if len(b) != 1 {
		t.Fatalf("blocks %d:\n%s", len(b), cfg)
	}
	want := []string{"-0.700000", "-2.250000", "0.900000", "0.000000", "-1.000000", "0.000000", "0", "0", "1", "0", "0", "255", "0", "0", "0.180", "180", "210", "AI_Brakelight", "1.000", "0.100", "3", "0", "0.030", ""}
	for i, w := range want {
		if b[0][i] != w {
			t.Fatalf("line %d = %q, want %q\n%s", i+1, b[0][i], w, cfg)
		}
	}
	if !strings.HasSuffix(cfg, "0.030\r\n\r\n") {
		t.Fatalf("bitmap line must be written even for the last block: %q", cfg[len(cfg)-20:])
	}
	head := lightBlocks(ModelCFG(VehicleSpec{Lights: []PointLight{NewLight(LightHead, .6, 2.2, .7, 1)}}))[0]
	if head[4] != "1.000000" || head[15] != "150" || head[16] != "200" || head[17] != "AI_Light" || head[19] != "0.150" || head[20] != "1" || head[21] != "1" || head[22] != "0.100" {
		t.Fatalf("head lamp preset %q", head)
	}
}

func TestLightsFollowBodyMeshOnceAndSkipReverse(t *testing.T) {
	v := testCar()
	cfg := ModelCFG(v)
	if strings.Contains(cfg, "lights_rueckfahr") {
		t.Fatalf("unverified reverse variable emitted:\n%s", cfg)
	}
	// 6 lamps with a variable; the reverse lamp has none
	if n := strings.Count(cfg, "[light_enh_2]"); n != 6 {
		t.Fatalf("%d light blocks, want 6 (once, not per LOD level):\n%s", n, cfg)
	}
	body, light, wheel, lod := strings.Index(cfg, "[mesh]\r\nbody.o3d"), strings.Index(cfg, "[light_enh_2]"), strings.Index(cfg, "[mesh]\r\nwheel_fl.o3d"), strings.Index(cfg, "lod_1.o3d")
	if !(body >= 0 && body < light && light < wheel && wheel < lod) {
		t.Fatalf("lights must follow the static body mesh, before the animated wheels (body %d light %d wheel %d lod %d):\n%s", body, light, wheel, lod, cfg)
	}
	if strings.LastIndex(cfg, "[light_enh_2]") > wheel {
		t.Fatalf("a light belongs to a wheel mesh:\n%s", cfg)
	}
	vars := []string{}
	for _, b := range lightBlocks(cfg) {
		vars = append(vars, b[17])
	}
	if got := strings.Join(vars, ","); got != "AI_Light,AI_Light,AI_Light,AI_Brakelight,lights_blinker_l,lights_blinker_r" {
		t.Fatalf("variables %s", got)
	}
	if ansiText(cfg) != cfg {
		t.Fatalf("model.cfg is not ASCII")
	}
}

func TestNewLightPresets(t *testing.T) {
	if l := NewLight(LightBlinker, -.7, 2, .7, 1); l.Variable != VarBlinkerL || l.R != 255 || l.B != 0 {
		t.Fatalf("left blinker %+v", l)
	}
	if l := NewLight(LightBlinker, .7, -2, .7, -1); l.Variable != VarBlinkerR || l.DY != -1 {
		t.Fatalf("right blinker %+v", l)
	}
	if l := NewLight(LightReverse, .4, -2, .7, -1); l.Variable != "" {
		t.Fatalf("reverse lamp must not be exported: %+v", l)
	}
	if l := NewLight(LightTail, .7, -2, .7, -1); l.Variable != VarLight || l.Strength >= 1 || l.Params != 3 || l.Cone != 0 {
		t.Fatalf("tail lamp %+v", l)
	}
}

func TestLampGlassGlowUsesMatlChange(t *testing.T) {
	cfg := ModelCFG(VehicleSpec{Materials: []MaterialOverride{
		{Texture: "body.dds"},
		{Texture: "lights.dds", AlphaMode: 2, Glow: VarLight},
		{Texture: "lights.dds", Instance: 1, Glow: VarBrake},
	}})
	head := "[matl]\r\nlights.dds\r\n0\r\n[matl_alpha]\r\n2\r\n[matl_change]\r\nlights.dds\r\n0\r\nAI_Light\r\n[matl_item]\r\n[matl_nightmap]\r\nlights.dds\r\n[matl_item]\r\n[matl_nightmap]\r\nlights.dds\r\n"
	brake := "[matl]\r\nlights.dds\r\n1\r\n[matl_change]\r\nlights.dds\r\n1\r\nAI_Brakelight\r\n[matl_item]\r\n[matl_nightmap]\r\nlights.dds\r\n"
	if !strings.Contains(cfg, head) || !strings.Contains(cfg, brake) || strings.Count(cfg, "[matl_item]") != 3 {
		t.Fatalf("glow blocks:\n%s", cfg)
	}
	if strings.Contains(cfg, "[matl_lightmap]") || strings.Contains(cfg, "matl_emit") {
		t.Fatalf("unexpected material keyword:\n%s", cfg)
	}
}
