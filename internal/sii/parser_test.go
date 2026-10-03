package sii

import "testing"

func TestParseIncludeAndUnit(t *testing.T) {
	data := []byte(`SiiNunit
{
@include "cars/audi.sui"
traffic_vehicle : traffic.audi_a6 {
 name: "Audi A6"
 variant[]: standard
 look[]: black
}
}`)
	d, err := Parse("/def/vehicle/traffic_storage_car.test.sii", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Includes) != 1 || d.Includes[0] != "/def/vehicle/cars/audi.sui" {
		t.Fatalf("includes: %#v", d.Includes)
	}
	if len(d.Units) != 1 || d.Units[0].Type != "traffic_vehicle" {
		t.Fatalf("units: %#v", d.Units)
	}
	if First(d.Units[0], "name") != "Audi A6" {
		t.Fatalf("name")
	}
}

func TestParseHistoricalCommentsAndIndexedArray(t *testing.T) {
	data := []byte(`SiiNunit
{
# @include "disabled.sii"
@include 'enabled.sui' // old mods use inline comments
/* traffic_vehicle : traffic.disabled { name: "No" } */
traffic_vehicle : traffic.real {
 name: "Real Car" # trailing hash comment
 gearbox[0]: (0,4.5)
 variant[]: standard
}
}`)
	d, err := Parse("/def/vehicle/traffic_storage.jazzycat.sii", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Includes) != 1 || d.Includes[0] != "/def/vehicle/enabled.sui" {
		t.Fatalf("includes=%#v", d.Includes)
	}
	if len(d.Units) != 1 || d.Units[0].Name != "traffic.real" {
		t.Fatalf("units=%#v", d.Units)
	}
	if First(d.Units[0], "name") != "Real Car" {
		t.Fatalf("name=%q", First(d.Units[0], "name"))
	}
}

func TestParseUnitHeaderWithBraceOnNextLine(t *testing.T) {
	data := []byte(`SiiNunit
{
traffic_vehicle_data : traffic.bentley_arnage
{
 name: "Bentley Arnage"
 model: "/vehicle/ai/jazzycat/bentley_arnage/ai.pmd"
 variant: default
 look: default
}
}`)
	d, err := Parse("/def/vehicle/ai/jazzycat/bentley_arnage_jazzy.sii", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Units) != 1 {
		t.Fatalf("want 1 unit, got %#v warnings=%v", d.Units, d.Warnings)
	}
	if d.Units[0].Type != "traffic_vehicle_data" || d.Units[0].Name != "traffic.bentley_arnage" {
		t.Fatalf("wrong unit: %#v", d.Units[0])
	}
	if First(d.Units[0], "model") != "/vehicle/ai/jazzycat/bentley_arnage/ai.pmd" {
		t.Fatalf("model field not parsed: %#v", d.Units[0].Fields)
	}
}
