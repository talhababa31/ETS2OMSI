package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestV2ScanProjectReturnsOnlyBoundCars(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "traffic.scs")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	add := func(name, body string) { w, _ := z.Create(name); _, _ = w.Write([]byte(body)) }
	add("def/vehicle/traffic_storage_car.demo.sii", `SiiNunit
{
@include "/def/vehicle/ai/car.sii"
}`)
	add("def/vehicle/traffic_storage_bus.demo.sii", `SiiNunit
{
@include "/def/vehicle/ai/bus.sii"
}`)
	add("def/vehicle/ai/car.sii", `SiiNunit
{
traffic_vehicle : traffic.car
{
 name: "Car"
 type: car
 model: "/vehicle/ai/car/ai.pmd"
}
}`)
	add("def/vehicle/ai/bus.sii", `SiiNunit
{
traffic_vehicle : traffic.bus
{
 name: "Bus"
 type: bus
 model: "/vehicle/ai/bus/ai.pmd"
}
}`)
	add("vehicle/ai/car/ai.pmd", "PMD")
	add("vehicle/ai/bus/ai.pmd", "PMD")
	z.Close()
	f.Close()
	pr, err := ScanProject(ScanOptions{PackagePath: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Report.Vehicles) != 1 || pr.Report.Vehicles[0].VehicleType != "car" {
		t.Fatalf("vehicles=%+v", pr.Report.Vehicles)
	}
	if len(pr.MountPaths) != 1 || filepath.Clean(pr.MountPaths[0]) != filepath.Clean(p) {
		t.Fatalf("mounts=%v", pr.MountPaths)
	}
}
