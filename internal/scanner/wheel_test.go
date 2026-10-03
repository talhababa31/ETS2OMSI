package scanner

import (
	"archive/zip"
	"ets2omsi/internal/archive"
	"os"
	"path/filepath"
	"testing"
)

func TestWheelAccessorySeparatedFromBodyLOD(t *testing.T) {
	dir := t.TempDir()
	zpath := filepath.Join(dir, "wheel.scs")
	f, _ := os.Create(zpath)
	zw := zip.NewWriter(f)
	add := func(name, body string) { w, _ := zw.Create(name); _, _ = w.Write([]byte(body)) }
	add("def/vehicle/traffic_storage_car.sii", `SiiNunit
{
@include "ai/car.sii"
}`)
	add("def/vehicle/ai/car.sii", `SiiNunit
{
traffic_vehicle : traffic.car
{
 accessories[]: .car.chassis
 accessories[]: .car.fwheel
 accessories[]: .car.rwheel
}
vehicle_accessory : .car.chassis
{
 data_path: "/def/vehicle/ai/car/chassis.sii"
}
vehicle_wheel_accessory : .car.fwheel
{
 offset: 0
 data_path: "/def/vehicle/f_wheel/test.sii"
}
vehicle_wheel_accessory : .car.rwheel
{
 offset: 0
 data_path: "/def/vehicle/r_wheel/test.sii"
}
}`)
	add("def/vehicle/ai/car/chassis.sii", `SiiNunit
{
accessory_chassis_data : .chassis
{
 model: "/vehicle/ai/car/body.pmd"
 lods[]: "/vehicle/ai/car/lod.pmd"
 variant: default
 look: default
}
}`)
	add("def/vehicle/f_wheel/test.sii", `SiiNunit
{
accessory_wheel_data : .fwheel
{
 model: "/vehicle/wheel/car.pmd"
 look: default
}
}`)
	add("def/vehicle/r_wheel/test.sii", `SiiNunit
{
accessory_wheel_data : .rwheel
{
 model: "/vehicle/wheel/car.pmd"
 look: default
}
}`)
	for _, p := range []string{"vehicle/ai/car/body.pmd", "vehicle/ai/car/lod.pmd", "vehicle/wheel/car.pmd"} {
		add(p, "x")
	}
	_ = zw.Close()
	_ = f.Close()
	src, err := archive.Open(zpath, archive.OpenOptions{})
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
		t.Fatalf("body models=%v", v.Models)
	}
	if len(v.WheelModels) != 1 || v.WheelModels[0] != "/vehicle/wheel/car.pmd" {
		t.Fatalf("wheel models=%v", v.WheelModels)
	}
	if len(v.WheelAttachments) != 2 {
		t.Fatalf("wheel attachments=%+v", v.WheelAttachments)
	}
	if v.ModelAssets[0].Role != "main" || v.ModelAssets[1].Role != "lod" {
		t.Fatalf("model assets=%+v", v.ModelAssets)
	}
}
