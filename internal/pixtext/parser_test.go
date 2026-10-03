package pixtext

import "testing"

func TestParse(t *testing.T) {
	x := `Piece {
 Material: 2
 Stream {
  Tag: "_POSITION"
  0 ( &3f800000 0 2.5 )
 }
 Triangles { 
 }
}`
	s, err := Parse(x)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 1 || First(s[0], "Material") != "2" || len(Children(s[0], "Stream")) != 1 {
		t.Fatalf("%+v", s)
	}
}
