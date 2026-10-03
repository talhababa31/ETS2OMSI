package scanner

import (
	"archive/zip"
	"ets2omsi/internal/archive"
	"os"
	"path/filepath"
	"testing"
)

func TestJazzycatLegacyStorageName(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "jazzycat.scs")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage.jazzycat.sii", `SiiNunit
{
@include "ai/jazzycat/bentley_arnage_jazzy.sii"
}`)
	add(z, "def/vehicle/ai/jazzycat/bentley_arnage_jazzy.sii", `SiiNunit
{
traffic_vehicle : traffic.bentley_arnage {
 name: "Bentley Arnage"
 type: car
}
}`)
	add(z, "def/vehicle/ai/jazzycat/bentley_arnage_jazzy/chassis.sii", `SiiNunit
{
accessory_chassis_data : chassis.bentley_arnage {
 model: "/vehicle/ai/jazzycat/bentley_arnage/model.pmd"
}
}`)
	add(z, "vehicle/ai/jazzycat/bentley_arnage/model.pmd", "PMD")
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats.VehicleCount != 1 {
		t.Fatalf("want 1 vehicle got %d warnings=%v diag=%+v", r.Stats.VehicleCount, r.Warnings, r.Diagnostics)
	}
	if r.Diagnostics.TrafficStorageFiles != 1 {
		t.Fatalf("legacy dotted storage not counted: %+v", r.Diagnostics)
	}
	if r.Vehicles[0].DisplayName != "Bentley Arnage" {
		t.Fatalf("wrong vehicle: %+v", r.Vehicles[0])
	}
	if r.Vehicles[0].VehicleType != "car" {
		t.Fatalf("unit type fallback not used: %+v", r.Vehicles[0])
	}
	if len(r.Vehicles[0].Models) != 1 {
		t.Fatalf("model not resolved: %+v", r.Vehicles[0])
	}
}

func TestOfficialInfixStorageAndNestedArchiveRoot(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "nested.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "pack_root/def/vehicle/traffic_storage_car.jazzycat_brazil.sii", `SiiNunit
{
@include "ai/jazzycat/chevrolet_cruze_jazzy.sui"
}`)
	add(z, "pack_root/def/vehicle/ai/jazzycat/chevrolet_cruze_jazzy.sui", `SiiNunit
{
traffic_vehicle : traffic.chev_cruze {
 name: "Chevrolet Cruze"
 type: car
}
}`)
	add(z, "pack_root/def/vehicle/ai/jazzycat/chevrolet_cruze_jazzy/chassis.sii", `SiiNunit
{
accessory_chassis_data : chassis.chev_cruze {
 model: "/vehicle/ai/jazzycat/chevrolet_cruze/ai.pmd"
}
}`)
	add(z, "pack_root/vehicle/ai/jazzycat/chevrolet_cruze/ai.pmd", "PMD")
	z.Close()
	f.Close()
	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats.VehicleCount != 1 {
		t.Fatalf("want 1 got %d diag=%+v warnings=%v", r.Stats.VehicleCount, r.Diagnostics, r.Warnings)
	}
	if r.Diagnostics.LogicalRootPrefix != "/pack_root" {
		t.Fatalf("nested root not detected: %q", r.Diagnostics.LogicalRootPrefix)
	}
	if len(r.Vehicles[0].Models) != 1 {
		t.Fatalf("nested PMD not resolved: %+v", r.Vehicles[0])
	}
}

func TestChassisCandidateWhenNoTrafficVehicle(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "odd.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage.odd.sii", `SiiNunit
{
@include "ai/odd/chassis_only.sii"
}`)
	add(z, "def/vehicle/ai/odd/chassis_only.sii", `SiiNunit
{
accessory_chassis_data : chassis.odd_car {
 model: "/vehicle/ai/odd/model.pmd"
}
}`)
	add(z, "vehicle/ai/odd/model.pmd", "PMD")
	z.Close()
	f.Close()
	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats.VehicleCount != 0 {
		t.Fatalf("candidate must not be invented as finalized vehicle")
	}
	if len(r.Candidates) != 1 {
		t.Fatalf("want diagnostic candidate got %+v", r.Candidates)
	}
	if r.Candidates[0].Confidence < .7 {
		t.Fatalf("expected storage-linked confidence: %+v", r.Candidates[0])
	}
}

func TestLegacyStorageDirectIncludeWithExplicitPMD(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "jazzycat_v29_style.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.jazzycat.sii", `SiiNunit
{
@include "ai/jazzycat/bentley_arnage_jazzy.sii"
@include "ai/jazzycat/volvo_850_jazzy.sii"
}`)
	add(z, "def/vehicle/ai/jazzycat/bentley_arnage_jazzy.sii", `SiiNunit
{
legacy_ai_model : traffic.bentley_arnage {
 name: "Bentley Arnage"
 model: "/vehicle/ai/jazzycat/bentley_arnage/ai.pmd"
 variant: default
 look: default
}
}`)
	add(z, "def/vehicle/ai/jazzycat/volvo_850_jazzy.sii", `SiiNunit
{
legacy_ai_model : traffic.volvo_850 {
 name: "Volvo 850"
 model: "/vehicle/ai/jazzycat/volvo_850/ai.pmd"
}
}`)
	add(z, "vehicle/ai/jazzycat/bentley_arnage/ai.pmd", "PMD")
	add(z, "vehicle/ai/jazzycat/volvo_850/ai.pmd", "PMD")
	z.Close()
	f.Close()

	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats.VehicleCount != 2 {
		t.Fatalf("want 2 legacy vehicles got %d warnings=%v diag=%+v vehicles=%+v", r.Stats.VehicleCount, r.Warnings, r.Diagnostics, r.Vehicles)
	}
	if r.Diagnostics.TrafficVehicleUnits != 0 {
		t.Fatalf("fixture must exercise legacy path; got %d modern roots", r.Diagnostics.TrafficVehicleUnits)
	}
	if r.Diagnostics.LegacyStorageRoots != 2 {
		t.Fatalf("want 2 legacy roots got %+v", r.Diagnostics)
	}
	for _, v := range r.Vehicles {
		if !v.Bound || len(v.Models) != 1 {
			t.Fatalf("legacy root not safely bound: %+v", v)
		}
		if v.DiscoveryBasis != "legacy storage direct include + explicit PMD reference" {
			t.Fatalf("unexpected basis: %s", v.DiscoveryBasis)
		}
	}
}

func TestLegacyStorageDoesNotGuessModelPath(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "no_guess.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.old.sii", `SiiNunit
{
@include "ai/old/mystery_car.sii"
}`)
	add(z, "def/vehicle/ai/old/mystery_car.sii", `SiiNunit
{
legacy_ai_model : traffic.mystery { name: "Mystery Car" }
}`)
	// A similarly named PMD exists, but there is no explicit reference to it.
	add(z, "vehicle/ai/old/mystery_car.pmd", "PMD")
	z.Close()
	f.Close()
	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats.VehicleCount != 0 {
		t.Fatalf("scanner guessed a vehicle/model link: %+v", r.Vehicles)
	}
}

func TestLegacyStorageSeparateLineUnitBrace(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "jazzycat_real_style.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.jazzycat.sii", `SiiNunit
{
@include "ai/jazzycat/bentley_arnage_jazzy.sii"
}`)
	add(z, "def/vehicle/ai/jazzycat/bentley_arnage_jazzy.sii", `SiiNunit
{
traffic_vehicle_data : traffic.bentley_arnage
{
 name: "Bentley Arnage"
 model: "/vehicle/ai/jazzycat/bentley_arnage/ai.pmd"
 variant: default
 look: default
}
}`)
	add(z, "vehicle/ai/jazzycat/bentley_arnage/ai.pmd", "PMD")
	z.Close()
	f.Close()

	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats.VehicleCount != 1 {
		t.Fatalf("want 1 legacy vehicle got %d warnings=%v diag=%+v", r.Stats.VehicleCount, r.Warnings, r.Diagnostics)
	}
	if r.Diagnostics.LegacyStorageRoots != 1 || r.Diagnostics.LegacyModelLinkedIncludes != 1 {
		t.Fatalf("legacy bridge did not bind multiline unit: %+v", r.Diagnostics)
	}
	if len(r.Vehicles[0].Models) != 1 {
		t.Fatalf("PMD not resolved: %+v", r.Vehicles[0])
	}
}

func TestSameDocumentVehicleAccessoryResolvesModel(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "unitrefs.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.demo.sii", `SiiNunit
{
@include "/def/vehicle/ai/demo.sii"
}`)
	add(z, "def/vehicle/ai/demo.sii", `SiiNunit
{
traffic_vehicle : traffic.demo
{
 name: "Demo Car"
 accessories[]: .demo.acc
}
vehicle_accessory : .demo.acc
{
 model: "/vehicle/ai/demo/demo.pmd"
 variant: default
 look: default
}
}`)
	add(z, "vehicle/ai/demo/demo.pmd", "PMD")
	z.Close()
	f.Close()
	src, _ := archive.Open(p, archive.OpenOptions{})
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Vehicles) != 1 || len(r.Vehicles[0].Models) != 1 {
		t.Fatalf("unit reference not resolved: %+v", r.Vehicles)
	}
	if r.Vehicles[0].Readiness != "ready" {
		t.Fatalf("readiness=%s missing=%v", r.Vehicles[0].Readiness, r.Vehicles[0].Missing)
	}
}

func TestV2PackageBridgeLinksLegacyTrafficToExactChassis(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "jazzycat_bridge.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.jazzycat.sii", `SiiNunit
{
@include "ai/jazzycat/alfa_155_jazzy.sii"
}`)
	add(z, "def/vehicle/ai/jazzycat/alfa_155_jazzy.sii", `SiiNunit
{
traffic_vehicle : traffic.alfa_155
{
 name: "Alfa 155"
 type: car
 accessory: .some_old_reference
}
}`)
	// The old traffic root has no explicit file path to the chassis.  Real traffic
	// packs commonly pair traffic.foo with chassis.foo in a separate definition.
	add(z, "def/vehicle/ai/jazzycat/alfa_155/chassis.sii", `SiiNunit
{
accessory_chassis_data : chassis.alfa_155
{
 model: "/vehicle/ai/jazzycat/alfa_155/alfa_155.pmd"
 lods[]: "/vehicle/ai/jazzycat/alfa_155/alfa_155_lod.pmd"
}
}`)
	add(z, "vehicle/ai/jazzycat/alfa_155/alfa_155.pmd", "PMD")
	add(z, "vehicle/ai/jazzycat/alfa_155/alfa_155_lod.pmd", "PMD")
	z.Close()
	f.Close()
	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Vehicles) != 1 {
		t.Fatalf("vehicles=%+v", r.Vehicles)
	}
	v := r.Vehicles[0]
	if len(v.Models) != 2 || v.Models[0] != "/vehicle/ai/jazzycat/alfa_155/alfa_155.pmd" {
		t.Fatalf("package bridge did not resolve body/LOD: %+v", v)
	}
	if v.Readiness != "ready" {
		t.Fatalf("readiness=%s blocking=%v", v.Readiness, v.Blocking)
	}
	if r.Diagnostics.PackageModelBridges != 1 {
		t.Fatalf("bridge count=%d", r.Diagnostics.PackageModelBridges)
	}
}

func TestV2PackageBridgeUsesUniqueRootDirectoryForRenamedChassisID(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "rootdir_bridge.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.jazzycat.sii", `SiiNunit
{
@include "ai/jazzycat/bentley_arnage_jazzy.sii"
}`)
	add(z, "def/vehicle/ai/jazzycat/bentley_arnage_jazzy.sii", `SiiNunit
{
traffic_vehicle : traffic.bentl_arnage
{
 name: "Bentley Arnage"
 type: car
}
}`)
	add(z, "def/vehicle/ai/jazzycat/bentley_arnage/chassis.sii", `SiiNunit
{
accessory_chassis_data : chassis.arnage
{
 model: "/vehicle/ai/jazzycat/bentley_arnage/arnage.pmd"
}
}`)
	add(z, "vehicle/ai/jazzycat/bentley_arnage/arnage.pmd", "PMD")
	z.Close()
	f.Close()
	src, err := archive.Open(p, archive.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Vehicles) != 1 || len(r.Vehicles[0].Models) != 1 {
		t.Fatalf("dir bridge failed: %+v", r.Vehicles)
	}
	if r.Vehicles[0].Readiness != "ready" {
		t.Fatalf("readiness=%s", r.Vehicles[0].Readiness)
	}
}
