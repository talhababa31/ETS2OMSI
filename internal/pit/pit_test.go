package pit

import (
	"ets2omsi/internal/pixtext"
	"testing"
)

func TestVariantVisibility(t *testing.T) {
	s := `Variant {
 Name: "left"
 Part {
  Name: "left_part"
  Attribute { 
   Format: INT
   Tag: "visible"
   Value: ( 1 )
  }
 }
 Part {
  Name: "right_part"
  Attribute {
   Format: INT
   Tag: "visible"
   Value: ( 0 )
  }
 }
}`
	secs, err := pixtext.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	v := VariantFromSections(secs, "left")
	if !v.Found || !v.VisibleParts["left_part"] || v.VisibleParts["right_part"] {
		t.Fatalf("%+v", v)
	}
}
