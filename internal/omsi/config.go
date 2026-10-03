package omsi

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type MaterialOverride struct {
	Texture   string `json:"texture"`
	Instance  int    `json:"instance"` // nth material of the mesh with this texture
	AlphaMode int    `json:"alpha_mode"`
	Class     string `json:"class,omitempty"`
	Glow      string `json:"glow,omitempty"` // lamp glass lit while this variable is on
}
type LOD struct {
	ScreenSize float64            `json:"screen_size"`
	File       string             `json:"file"`
	Materials  []MaterialOverride `json:"materials,omitempty"`
}
type Wheel struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Radius float64 `json:"radius"`
	Width  float64 `json:"width"`
}

// PointLight is one [light_enh_2] lamp. Positions and directions are in
// model.cfg vehicle axes (x right, y forward, z up), the frame the wheel
// origin_trans lines use; only the O3D meshes store Y up / Z forward.
type PointLight struct {
	Kind                 string // LightHead ...
	X, Y, Z, DX, DY, DZ  float64
	R, G, B              int
	Size                 float64
	Variable             string // fading variable; "" = not exported
	Strength             float64
	ConeInner, ConeOuter float64 // degrees; 0 = 180/210
	ZOffset              float64
	Params, Cone         int     // +1 star, +2 no fog effect; fog cone 0/1
	TimeConst            float64 // seconds to 63 %; 0 = 0.03
}

// Light functions of a converted car.
const (
	LightHead    = "head"
	LightTail    = "tail"
	LightBrake   = "brake"
	LightBlinker = "blinker"
	LightReverse = "reverse"
)

// AI car light variables. OMSI writes AI_Light (0 off, 0.5 parking light,
// 1 on, 2 headlight flash) and AI_Brakelight on every AI vehicle; the
// flashing blinker outputs come from the stock Scripts\AI_Cars\main_AI.osc
// (declared in AI_Cars\lights_varlist.txt, which the OVH loads).
const (
	VarLight    = "AI_Light"
	VarBrake    = "AI_Brakelight"
	VarBlinkerL = "lights_blinker_l"
	VarBlinkerR = "lights_blinker_r"
)

// NewLight returns a lamp of the given function at (x, y, z) shining along
// dirY (+1 forward, -1 backward), with the cone/fog/timing values of stock
// OMSI vehicle lamps. A reverse light gets no variable: OMSI AI cars never
// reverse in traffic and their scripts have no reverse-light variable.
func NewLight(kind string, x, y, z, dirY float64) PointLight {
	l := PointLight{Kind: kind, X: x, Y: y, Z: z, DY: dirY, ConeInner: 180, ConeOuter: 210, Strength: 1, ZOffset: .1, Params: 3, TimeConst: .03}
	switch kind {
	case LightHead:
		l.R, l.G, l.B, l.Size, l.Variable = 255, 245, 220, .18, VarLight
		l.ConeInner, l.ConeOuter, l.ZOffset, l.Params, l.Cone, l.TimeConst = 150, 200, .15, 1, 1, .1
	case LightTail:
		l.R, l.G, l.B, l.Size, l.Variable, l.Strength = 255, 0, 0, .14, VarLight, .6
	case LightBrake:
		l.R, l.G, l.B, l.Size, l.Variable = 255, 0, 0, .18, VarBrake
	case LightBlinker:
		l.R, l.G, l.B, l.Size, l.Variable = 255, 160, 0, .13, VarBlinkerR
		if x < 0 {
			l.Variable = VarBlinkerL
		}
	case LightReverse:
		l.R, l.G, l.B, l.Size, l.Strength, l.TimeConst = 255, 255, 240, .12, .8, .05
	}
	return l
}

type PhysicsProfile struct {
	Profile       string  `json:"profile"`
	Mass          float64 `json:"mass_t"`
	AIDeltaHeight float64 `json:"ai_deltaheight"`
	Class         string  `json:"class,omitempty"`
}

type VehicleSpec struct {
	Name                  string
	Type                  string
	Length, Width, Height float64
	Wheels                map[string]Wheel
	WheelFiles            map[string]string
	Materials             []MaterialOverride
	LODs                  []LOD
	Lights                []PointLight
	HighDetailScreenSize  float64
	Physics               PhysicsProfile
	ModelCFGName          string // default "model.cfg"
	BodyFile              string // default "body.o3d"
	ColorLabel            string // shown in the OMSI friendly name
}

// WriteVariant writes a colour variant of an already written vehicle:
// model\model_<id>.cfg and <Name>_<id>.ovh (O3D files are shared).
func WriteVariant(dir string, v VehicleSpec, id, label string) (string, error) {
	v.ModelCFGName = "model_" + id + ".cfg"
	v.ColorLabel = label
	if err := os.WriteFile(filepath.Join(dir, "model", v.ModelCFGName), []byte(ansiText(ModelCFG(v))), 0644); err != nil {
		return "", err
	}
	ovh := v.Name + "_" + id + ".ovh"
	return ovh, os.WriteFile(filepath.Join(dir, ovh), []byte(ansiText(OVH(v))), 0644)
}

func Write(dir string, v VehicleSpec) error {
	if v.Name == "" {
		v.Name = "ETS2_AI_Vehicle"
	}
	if err := os.MkdirAll(filepath.Join(dir, "model"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "script"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "model", "model.cfg"), []byte(ansiText(ModelCFG(v))), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, v.Name+".ovh"), []byte(ansiText(OVH(v))), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "script", "AI_constfile.txt"), []byte(AIConstFileClass(v.Physics.Class)), 0644)
}
func ModelCFG(v VehicleSpec) string {
	var b strings.Builder
	b.WriteString("; Generated by ETS2OMSI V2.7.0\r\n")
	high := v.HighDetailScreenSize
	if high <= 0 || high > 1 {
		high = 0.080
	}
	if len(v.LODs) > 0 {
		fmt.Fprintf(&b, "\r\n[LOD]\r\n%.4f\r\n", high)
	}
	body := v.BodyFile
	if body == "" {
		body = "body.o3d"
	}
	writeMeshBlock(&b, body, v.Materials)
	// Lights belong to the [mesh] before them and move with its animation,
	// so they follow the static body, not a rotating wheel. OMSI draws a
	// model's lights whatever [LOD] level their mesh is in, so they are
	// written once: repeating them per level would stack the coronas.
	writeLights(&b, v.Lights)
	writeWheelBlocks(&b, v)
	for _, lod := range v.LODs {
		if strings.TrimSpace(lod.File) == "" {
			continue
		}
		fmt.Fprintf(&b, "\r\n[LOD]\r\n%.4f\r\n", math.Max(0, lod.ScreenSize))
		writeMeshBlock(&b, lod.File, lod.Materials)
		// Keep wheels visible at lower ETS2-provided LODs too. Repeating the same
		// OMSI wheel mesh is cheap and avoids wheel pop/disappearance.
		writeWheelBlocks(&b, v)
	}
	return b.String()
}

// writeLights writes the 24-line [light_enh_2] blocks: pos, dir, up, omni,
// rotating, rgb, size, inner/outer cone, variable, factor, z-offset,
// parameters, fog cone, timeconst and an empty bitmap line (standard glow).
func writeLights(b *strings.Builder, lights []PointLight) {
	for _, l := range lights {
		if strings.TrimSpace(l.Variable) == "" {
			continue
		}
		sz := l.Size
		if sz <= 0 {
			sz = .25
		}
		st := l.Strength
		if st <= 0 {
			st = 1
		}
		in, out := l.ConeInner, l.ConeOuter
		if out <= 0 {
			in, out = 180, 210
		}
		tc := l.TimeConst
		if tc <= 0 {
			tc = .03
		}
		fmt.Fprintf(b, "\r\n[light_enh_2]\r\n%.6f\r\n%.6f\r\n%.6f\r\n%.6f\r\n%.6f\r\n%.6f\r\n0\r\n0\r\n1\r\n0\r\n0\r\n%d\r\n%d\r\n%d\r\n%.3f\r\n%g\r\n%g\r\n%s\r\n%.3f\r\n%.3f\r\n%d\r\n%d\r\n%.3f\r\n\r\n",
			l.X, l.Y, l.Z, l.DX, l.DY, l.DZ, clamp255(l.R), clamp255(l.G), clamp255(l.B), sz, in, out, l.Variable, st, math.Max(0, l.ZOffset), l.Params, l.Cone, tc)
	}
}

func writeMeshBlock(b *strings.Builder, file string, mats []MaterialOverride) {
	fmt.Fprintf(b, "\r\n[mesh]\r\n%s\r\n", file)
	writeMaterials(b, mats)
}

func writeWheelBlocks(b *strings.Builder, v VehicleSpec) {
	for _, slot := range []string{"FL", "FR", "RL", "RR"} {
		c, ok := v.Wheels[slot]
		if !ok {
			continue
		}
		file := ""
		if v.WheelFiles != nil {
			file = strings.TrimSpace(v.WheelFiles[slot])
		}
		if file == "" {
			continue
		}
		side := "L"
		if slot == "FR" || slot == "RR" {
			side = "R"
		}
		axle := 0
		if slot == "RL" || slot == "RR" {
			axle = 1
		}
		fmt.Fprintf(b, "\r\n[mesh]\r\n%s\r\n[newanim]\r\norigin_trans\r\n%.6f\r\n%.6f\r\n%.6f\r\nanim_rot\r\nWheel_Rotation_%d_%s\r\n57.2957795130823\r\n", file, c.X, c.Y, c.Z, axle, side)
		fmt.Fprintf(b, "[newanim]\r\norigin_rot_y\r\n-90\r\nanim_trans\r\nAxle_Suspension_%d_%s\r\n1\r\n", axle, side)
		if axle == 0 {
			fmt.Fprintf(b, "[newanim]\r\norigin_trans\r\n%.6f\r\n%.6f\r\n%.6f\r\norigin_rot_y\r\n90\r\nanim_rot\r\nAxle_Steering_0_%s\r\n57.2957795130823\r\n", c.X, c.Y, c.Z, side)
		}
	}
}

func writeMaterials(b *strings.Builder, m []MaterialOverride) {
	for _, x := range m {
		if x.Texture == "" {
			continue
		}
		fmt.Fprintf(b, "\r\n[matl]\r\n%s\r\n%d\r\n", x.Texture, x.Instance)
		if x.AlphaMode > 0 {
			fmt.Fprintf(b, "[matl_alpha]\r\n%d\r\n", x.AlphaMode)
		}
		if strings.TrimSpace(x.Glow) != "" {
			writeGlow(b, x)
		}
	}
}

// writeGlow lights lamp glass up with its lamp: [matl_change] switches the
// slot to a [matl_item] (a copy of the plain material) whose night map, the
// lamp texture itself, glows at full strength while the variable is on.
// OMSI rounds the variable and shows item n for n = 1..items, so AI_Light
// (2 = headlight flash) gets a second item.
func writeGlow(b *strings.Builder, x MaterialOverride) {
	fmt.Fprintf(b, "[matl_change]\r\n%s\r\n%d\r\n%s\r\n", x.Texture, x.Instance, x.Glow)
	items := 1
	if x.Glow == VarLight {
		items = 2
	}
	for i := 0; i < items; i++ {
		fmt.Fprintf(b, "[matl_item]\r\n[matl_nightmap]\r\n%s\r\n", x.Texture)
	}
}
func clamp255(x int) int {
	if x < 0 {
		return 0
	}
	if x > 255 {
		return 255
	}
	return x
}

func OVH(v VehicleSpec) string {
	typ := strings.ToLower(v.Type)
	mass := v.Physics.Mass
	if mass <= 0 {
		mass = 1.5
		switch typ {
		case "bus":
			mass = 12
		case "truck":
			mass = 8
		case "van":
			mass = 2.5
		case "trailer":
			mass = 7
		}
	}
	w, l, h := safeDim(v.Width, 1.8), safeDim(v.Length, 4.5), safeDim(v.Height, 1.5)
	front, rear := axle(v.Wheels, l)
	wb := math.Abs(front.Y - rear.Y)
	if wb < 1 {
		wb = l * .58
		front.Y = wb / 2
		rear.Y = -wb / 2
	}
	rf := front.Radius
	if rf <= 0 {
		rf = math.Max(.25, math.Min(.55, h*.22))
	}
	rr := rear.Radius
	if rr <= 0 {
		rr = rf
	}
	fo, fi := widths(v.Wheels, "FL", "FR", w)
	ro, ri := widths(v.Wheels, "RL", "RR", w)
	inv := math.Tan(26*math.Pi/180) / wb
	cog := math.Max(.35, math.Min(1.1, h*.28))
	ix := .47 * mass * (l*l + h*h) / 12
	iy := .47 * mass * (w*w + h*h) / 12
	iz := .47 * mass * (w*w + l*l) / 12
	sus := SuspensionForClass(mass, v.Physics.Class)
	classLabel := "AI Traffic"
	if c, ok := LookupClass(v.Physics.Class); ok {
		classLabel = "AI Traffic · " + c.Label
	}
	if v.ColorLabel != "" {
		classLabel += " · " + v.ColorLabel
	}
	modelCFG := v.ModelCFGName
	if modelCFG == "" {
		modelCFG = "model.cfg"
	}
	return fmt.Sprintf(`; Generated by ETS2OMSI V2.7.0
[registration_free]

[friendlyname]
ETS2OMSI
%s
%s

[model]
model\%s

[sound]
..\..\Sounds\AI_Cars\sound.cfg

[varnamelist]
2
..\..\Scripts\AI_Cars\AI_varlist.txt
..\..\Scripts\AI_Cars\lights_varlist.txt

[script]
1
..\..\Scripts\AI_Cars\main_AI.osc

[constfile]
1
script\AI_constfile.txt

[rowdy_factor]
-0.5
0.5

[set_camera_outside_center]
0
0
%.5f

[mass]
%.4f

[momentofintertia]
%.4f
%.4f
%.4f

[boundingbox]
%.5f
%.5f
%.5f
0
0
%.5f

[schwerpunkt]
%.5f

[rollwiderstand]
%.3f

[rot_pnt_long]
%.5f

[inv_min_turnradius]
%.7f

[ai_deltaheight]
%.4f

[newachse]
achse_long
%.6f
achse_maxwidth
%.5f
achse_minwidth
%.5f
achse_raddurchmesser
%.5f
achse_feder
%.2f
achse_maxforce
%.2f
achse_daempfer
%.2f
achse_antrieb
0
achse_inertia_inv
0.015

[newachse]
achse_long
%.6f
achse_maxwidth
%.5f
achse_minwidth
%.5f
achse_raddurchmesser
%.5f
achse_feder
%.2f
achse_maxforce
%.2f
achse_daempfer
%.2f
achse_antrieb
1
achse_inertia_inv
0.015
`, v.Name, classLabel, modelCFG, h*.56, mass, ix, iy, iz, w, l, h, h/2, cog, 133.3333333*mass, rear.Y, inv, v.Physics.AIDeltaHeight, front.Y, fo, fi, rf*2, sus.FrontSpring, sus.FrontMaxForce, sus.FrontDamper, rear.Y, ro, ri, rr*2, sus.RearSpring, sus.RearMaxForce, sus.RearDamper)
}

// Suspension holds OMSI axle values in OMSI's units (mass in t, so spring
// kN/m, damper kN·s/m, max force kN).
type Suspension struct {
	FrontSpring, FrontDamper, FrontMaxForce float64
	RearSpring, RearDamper, RearMaxForce    float64
}

const (
	gravity          = 9.81
	frontLoadShare   = 0.56 // typical front-engine passenger car
	rideFrequencyHz  = 2.3  // firm ride: small static sag, no bouncing
	rearFreqFactor   = 1.08 // rear slightly stiffer (flat ride)
	dampingRatio     = 0.45
	maxForceOverLoad = 4.0
)

// SuspensionFor derives axle spring/damper values from the vehicle mass.
// The former fixed ratio (≈40 kN/m for 1.5 t) let a car sag ~18 cm in OMSI,
// sinking wheels into the arches; this targets ~4-5 cm static sag.
func SuspensionFor(mass float64) Suspension { return SuspensionForClass(mass, "") }

// SuspensionForClass uses the class ride frequency and axle load split.
func SuspensionForClass(mass float64, class string) Suspension {
	if mass <= 0 {
		mass = 1.5
	}
	freq, share := rideFrequencyHz, frontLoadShare
	if c, ok := LookupClass(class); ok {
		freq, share = c.RideHz, c.FrontLoad
	}
	axle := func(m, f float64) (k, c, fmax float64) {
		w := 2 * math.Pi * f
		k = m * w * w
		c = 2 * dampingRatio * math.Sqrt(k*m)
		fmax = maxForceOverLoad * m * gravity
		return
	}
	mf, mr := mass*share, mass*(1-share)
	s := Suspension{}
	s.FrontSpring, s.FrontDamper, s.FrontMaxForce = axle(mf, freq)
	s.RearSpring, s.RearDamper, s.RearMaxForce = axle(mr, freq*rearFreqFactor)
	return s
}

// EstimateAIPhysics creates an OMSI AI physics profile from the model name and
// measured exterior dimensions. It is intentionally a driving/suspension profile,
// not a claim of exact manufacturer curb weight. Known model-family hints improve
// common traffic cars; unknown cars fall back to geometry.
func EstimateAIPhysics(name string, length, width, height float64) PhysicsProfile {
	n := strings.ToLower(name)
	l := safeDim(length, 4.5)
	w := safeDim(width, 1.8)
	h := safeDim(height, 1.5)
	profile := "passenger car"
	mult := 1.0
	minMass, maxMass := 0.90, 2.15

	containsAny := func(words ...string) bool {
		for _, x := range words {
			if strings.Contains(n, x) {
				return true
			}
		}
		return false
	}
	largeSUV := containsAny(" x6", "x6 ", " x7", "x7 ", "q7", "q8", "xc90", "cayenne", "touareg", "range rover", "land cruiser", "escalade", "tahoe", "suburban")
	suv := largeSUV || containsAny(" x1", " x3", " x4", " x5", "q3", "q5", "glk", "glc", " gle", " gls", "ml ", "tiguan", "rav4", "cr-v", "crv", "x-trail", "qashqai", "sorento", "sportage", "kuga", "pajero", "cherokee", "duster", "santa fe", "xc60") || (h > 1.60 && w > 1.78)
	van := containsAny("transporter", "multivan", "caravelle", "vito", "sprinter", "transit", "ducato", "boxer", "jumper") || h > 1.90
	compact := l < 4.15 && h < 1.60
	if van {
		profile, mult, minMass, maxMass = "van/large MPV", 1.22, 1.65, 3.20
	} else if largeSUV {
		profile, mult, minMass, maxMass = "large SUV", 1.28, 2.10, 3.00
	} else if suv {
		profile, mult, minMass, maxMass = "SUV/crossover", 1.17, 1.45, 2.65
	} else if compact {
		profile, mult, minMass, maxMass = "compact car", .94, .85, 1.55
	}

	// Envelope-volume estimate is deliberately conservative for hollow traffic
	// meshes. 0.122 t/m^3 reproduces a ~1.45-1.55 t midsize sedan envelope.
	mass := 0.122 * l * w * h * mult
	// A few common platform/model tokens are useful anchors when present in the
	// mod's display name. They remain OMSI physics targets, not spec-sheet claims.
	switch {
	case containsAny("f30"):
		mass, profile = 1.55, "BMW F30-class sedan"
	case containsAny(" x6", "x6 "):
		mass, profile = 2.55, "BMW X6-class large SUV"
	case containsAny("q7"):
		mass, profile = 2.40, "Audi Q7-class large SUV"
	case containsAny("xc90"):
		mass, profile = 2.30, "Volvo XC90-class large SUV"
	}
	if mass < minMass {
		mass = minMass
	}
	if mass > maxMass {
		mass = maxMass
	}

	return PhysicsProfile{Profile: profile, Mass: math.Round(mass*100) / 100, AIDeltaHeight: 0}
}

func safeDim(x, d float64) float64 {
	if x < .2 || x > 40 {
		return d
	}
	return x
}
func axle(m map[string]Wheel, l float64) (Wheel, Wheel) {
	f, ok := m["FL"]
	if !ok {
		f = m["FR"]
	}
	r, ok := m["RL"]
	if !ok {
		r = m["RR"]
	}
	if f.Y == 0 && r.Y == 0 {
		f.Y = l * .29
		r.Y = -l * .29
	}
	return f, r
}
func widths(m map[string]Wheel, a, b string, w float64) (float64, float64) {
	x1, x2 := math.Abs(m[a].X), math.Abs(m[b].X)
	half := math.Max(x1, x2)
	if half < .3 {
		half = w * .42
	}
	outer := half * 2
	inner := math.Max(.5, outer-math.Max(.1, w*.12))
	return outer, inner
}
func AIConstFile() string { return AIConstFileClass("") }

// AIConstFileClass writes the AI constants with the class top speed.
func AIConstFileClass(class string) string {
	vmax := 160
	if c, ok := LookupClass(class); ok {
		vmax = c.VMax
	}
	return fmt.Sprintf("[const]\r\nAI_v_max\r\n%d\r\n\r\n[const]\r\nAI_v_max_erreich\r\n%d", vmax, vmax+30) + "\r\n\r\n[const]\r\nAI_M_gas_max\r\n1000\r\n\r\n[const]\r\nAI_F_brems_max\r\n10000\r\n\r\n[const]\r\nAI_lights_blinkgeberintervall\r\n0.300\r\n\r\n[newcurve]\r\nAI_Motorkennlinie\r\n\r\n[pnt]\r\n-40\r\n2\r\n\r\n[pnt]\r\n0\r\n0.3\r\n\r\n[pnt]\r\n15\r\n1\r\n\r\n[pnt]\r\n40\r\n1\r\n\r\n[pnt]\r\n80\r\n1\r\n\r\n[pnt]\r\n220\r\n2\r\n"
}

var ansiReplacer = strings.NewReplacer("ı", "i", "İ", "I", "ş", "s", "Ş", "S", "ğ", "g", "Ğ", "G", "ü", "u", "Ü", "U", "ö", "o", "Ö", "O", "ç", "c", "Ç", "C", "·", "-", "→", "->", "—", "-", "–", "-")

// ansiText makes OMSI text files plain ASCII: OMSI reads them in the ANSI code
// page, so UTF-8 text (Turkish letters, "·") showed up garbled in game.
func ansiText(s string) string {
	s = ansiReplacer.Replace(s)
	b := strings.Builder{}
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}
