package omsi

import (
	"math"
	"strings"
	"testing"
)

func TestGoldenOVHUsesWorkingAICarRelativePaths(t *testing.T) {
	s := OVH(VehicleSpec{Name: "Car", Type: "car", Length: 4.7, Width: 1.9, Height: 1.5, Wheels: map[string]Wheel{"FL": {X: -.8, Y: 1.3, Z: .3, Radius: .3}, "FR": {X: .8, Y: 1.3, Z: .3, Radius: .3}, "RL": {X: -.8, Y: -1.3, Z: .3, Radius: .3}, "RR": {X: .8, Y: -1.3, Z: .3, Radius: .3}}})
	for _, want := range []string{`..\..\Sounds\AI_Cars\sound.cfg`, `..\..\Scripts\AI_Cars\main_AI.osc`, `[registration_free]`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q\n%s", want, s)
		}
	}
	if strings.Contains(s, `..\..\..\Scripts`) {
		t.Fatalf("old three-level script path survived")
	}
}

func TestGoldenModelCFGSeparateWheelsAndLOD(t *testing.T) {
	v := VehicleSpec{Wheels: map[string]Wheel{"FL": {X: -.8, Y: 1.3, Z: .3}, "FR": {X: .8, Y: 1.3, Z: .3}, "RL": {X: -.8, Y: -1.3, Z: .3}, "RR": {X: .8, Y: -1.3, Z: .3}}, WheelFiles: map[string]string{"FL": "wheel_fl.o3d", "FR": "wheel_fr.o3d", "RL": "wheel_rl.o3d", "RR": "wheel_rr.o3d"}, LODs: []LOD{{ScreenSize: .035, File: "lod_1.o3d"}}}
	s := ModelCFG(v)
	for _, want := range []string{"[LOD]\r\n0.0800", "wheel_fl.o3d", "Wheel_Rotation_0_L", "Axle_Steering_0_R", "[LOD]\r\n0.0350", "lod_1.o3d"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q\n%s", want, s)
		}
	}
}

func TestEstimateAIPhysicsUsesNameAndDimensions(t *testing.T) {
	f30 := EstimateAIPhysics("BMW F30", 4.62, 1.81, 1.43)
	if f30.Mass < 1.45 || f30.Mass > 1.65 {
		t.Fatalf("F30 mass=%v", f30.Mass)
	}
	x6 := EstimateAIPhysics("BMW X6", 4.94, 2.0, 1.70)
	if x6.Mass < 2.35 || x6.Mass > 2.75 {
		t.Fatalf("X6 mass=%v", x6.Mass)
	}
	unknown := EstimateAIPhysics("Unknown Sedan", 4.7, 1.82, 1.45)
	if unknown.Mass < 1.2 || unknown.Mass > 1.9 {
		t.Fatalf("unknown sedan mass=%v", unknown.Mass)
	}
}

func TestOVHPhysicsFormattingHasNoMissingArgs(t *testing.T) {
	v := VehicleSpec{Name: "BMW_F30", Type: "car", Length: 4.62, Width: 1.81, Height: 1.43, Physics: EstimateAIPhysics("BMW F30", 4.62, 1.81, 1.43), Wheels: map[string]Wheel{"FL": {X: -.78, Y: 1.35, Z: .31, Radius: .31}, "FR": {X: .78, Y: 1.35, Z: .31, Radius: .31}, "RL": {X: -.78, Y: -1.45, Z: .31, Radius: .31}, "RR": {X: .78, Y: -1.45, Z: .31, Radius: .31}}}
	s := OVH(v)
	if strings.Contains(s, "%!") {
		t.Fatalf("fmt mismatch in OVH:\n%s", s)
	}
	if !strings.Contains(s, "[mass]\n1.5500") {
		t.Fatalf("physics mass not emitted:\n%s", s)
	}
}

func TestSuspensionStaticSagIsSmall(t *testing.T) {
	for _, mass := range []float64{0.9, 1.5, 2.5} {
		s := SuspensionFor(mass)
		front := mass * frontLoadShare * gravity / s.FrontSpring
		rear := mass * (1 - frontLoadShare) * gravity / s.RearSpring
		for _, sag := range []float64{front, rear} {
			if sag < .025 || sag > .065 {
				t.Fatalf("mass %.1f: static sag %.3f m outside 2.5-6.5 cm (%+v)", mass, sag, s)
			}
		}
		if s.FrontMaxForce < 3*mass*frontLoadShare*gravity {
			t.Fatalf("max force too low: %+v", s)
		}
		zeta := s.FrontDamper / (2 * math.Sqrt(s.FrontSpring*mass*frontLoadShare))
		if zeta < .3 || zeta > .7 {
			t.Fatalf("damping ratio %.2f", zeta)
		}
	}
	ovh := OVH(VehicleSpec{Name: "x", Type: "car", Physics: PhysicsProfile{Mass: 1.5}})
	if !strings.Contains(ovh, "achse_feder\n") || strings.Contains(ovh, "achse_feder\n40.00") {
		t.Fatalf("old soft spring still emitted:\n%s", ovh)
	}
}
