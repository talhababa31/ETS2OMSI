package scanner

import (
	"archive/zip"
	"ets2omsi/internal/archive"
	"os"
	"path/filepath"
	"testing"
)

func add(z *zip.Writer, p, s string) { w, _ := z.Create(p); _, _ = w.Write([]byte(s)) }
func TestMultiVehicleStorageGraph(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "multi.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.demo.sii", `SiiNunit
{
@include "/def/vehicle/ai/audi.sii"
@include "/def/vehicle/ai/volvo.sii"
}`)
	add(z, "def/vehicle/traffic_storage_truck.demo.sii", `SiiNunit
{
@include "/def/vehicle/ai/truck/scania.sii"
}`)
	add(z, "def/vehicle/ai/audi.sii", `SiiNunit
{
traffic_vehicle : traffic.audi { name: "Audi A6" }
}`)
	add(z, "def/vehicle/ai/audi/chassis.sii", `SiiNunit
{
accessory_chassis_data : .audi { model: "/vehicle/ai/audi/audi.pmd" variant: standard look: default }
}`)
	add(z, "vehicle/ai/audi/audi.pmd", "PMD")
	add(z, "def/vehicle/ai/volvo.sii", `SiiNunit
{
traffic_vehicle : traffic.volvo {
 name: "Volvo XC90"
}
}`)
	add(z, "def/vehicle/ai/volvo/chassis.sii", `SiiNunit
{
accessory_chassis_data : .volvo {
 model: "/vehicle/ai/shared/carbody.pmd"
 variant: standard
}
}`)
	add(z, "vehicle/ai/shared/carbody.pmd", "PMD")
	add(z, "def/vehicle/ai/truck/scania.sii", `SiiNunit
{
traffic_vehicle : traffic.scania {
 name: "Scania R"
}
}`)
	add(z, "def/vehicle/ai/truck/scania/chassis.sii", `SiiNunit
{
accessory_chassis_data : .scania {
 model: "/vehicle/ai/truck/scania/model.pmd"
}
}`)
	add(z, "vehicle/ai/truck/scania/model.pmd", "PMD")
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
	if r.Stats.VehicleCount != 3 {
		t.Fatalf("want 3 got %d %#v", r.Stats.VehicleCount, r.Vehicles)
	}
	for _, v := range r.Vehicles {
		if !v.Bound {
			t.Fatalf("unexpected unbound %s", v.ID)
		}
		if len(v.Chassis) == 0 {
			t.Fatalf("no chassis %s", v.ID)
		}
		if len(v.Models) == 0 {
			t.Fatalf("no models %s nodes=%#v", v.ID, v.Dependencies)
		}
	}

	for _, v := range r.Vehicles {
		for _, n := range v.Dependencies {
			if v.ID == "traffic.volvo" && n.Path == "/def/vehicle/ai/audi.sii" {
				t.Fatalf("sibling vehicle leaked into Volvo closure")
			}
		}
	}
}

func TestGlobalUnitReferenceAcrossVehicleDefinitions(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "global-unit.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add(z, "def/vehicle/traffic_storage_car.demo.sii", `SiiNunit
{
@include "/def/vehicle/ai/car.sii"
}`)
	add(z, "def/vehicle/ai/car.sii", `SiiNunit
{
traffic_vehicle : traffic.car
{
 name: "Cross File Car"
 accessories[]: .car.chassis
}
}`)
	add(z, "def/vehicle/ai/car/accessories.sii", `SiiNunit
{
vehicle_accessory : .car.chassis
{
 data_path: "/def/vehicle/ai/car/chassis.sii"
}
}`)
	add(z, "def/vehicle/ai/car/chassis.sii", `SiiNunit
{
accessory_chassis_data : .car.chassis.data
{
 model: "/vehicle/ai/car/body.pmd"
 lods[]: "/vehicle/ai/car/lod.pmd"
}
}`)
	add(z, "vehicle/ai/car/body.pmd", "PMD")
	add(z, "vehicle/ai/car/lod.pmd", "PMD")
	_ = z.Close()
	_ = f.Close()
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
		t.Fatalf("vehicles=%d", len(r.Vehicles))
	}
	v := r.Vehicles[0]
	if len(v.Models) != 2 {
		t.Fatalf("expected body + lod via global unit ref, got %v; deps=%+v", v.Models, v.Dependencies)
	}
	if r.Diagnostics.GlobalUnitRefsResolved == 0 {
		t.Fatalf("global unit resolver was not exercised")
	}
}
