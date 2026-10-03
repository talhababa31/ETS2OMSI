package pim

import (
	"math"
	"testing"
)

func TestPIMScene(t *testing.T) {
	x := `Material {
 Index: 0
 Alias: "body"
 Effect: "eut2.dif"
}
Piece {
 Material: 0
 Stream {
  Tag: "_POSITION"
  0 ( 0 0 0 )
  1 ( 1 0 0 )
  2 ( 0 1 0 )
 }
 Stream {
  Tag: "_NORMAL"
  0 ( 0 0 1 )
  1 ( 0 0 1 )
  2 ( 0 0 1 )
 }
 Stream {
  Tag: "_UV0"
  0 ( 0 0 )
  1 ( 1 0 )
  2 ( 0 1 )
 }
 Triangles {
  0 ( 0 1 2 )
 }
}
Locator {
 Name: "wheel_fl"
 Position: ( -0.8 0.3 1.2 )
}`
	s, err := Parse(x)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Vertices) != 3 || len(s.Triangles) != 1 || len(s.Materials) != 1 {
		t.Fatalf("%+v", s)
	}
	if math.Abs(s.Vertices[2].Position.Z-1) > 1e-6 {
		t.Fatalf("transform %+v", s.Vertices[2].Position)
	}
	if math.Abs(s.Vertices[2].UV.Y-0) > 1e-6 {
		t.Fatalf("uv %+v", s.Vertices[2].UV)
	}
}

func TestVisiblePartFiltering(t *testing.T) {
	src := `Material {
 Index: 0
 Alias: "m"
}
Piece {
 Index: 0
 Material: 0
 Stream {
  Tag: "_POSITION"
  0 ( 0 0 0 )
  1 ( 1 0 0 )
  2 ( 0 1 0 )
 }
 Triangles {
  0 ( 0 1 2 )
 }
}
Piece {
 Index: 1
 Material: 0
 Stream {
  Tag: "_POSITION"
  0 ( 10 0 0 )
  1 ( 11 0 0 )
  2 ( 10 1 0 )
 }
 Triangles {
  0 ( 0 1 2 )
 }
}
Part {
 Name: "left"
 PieceCount: 1
 LocatorCount: 0
 Pieces: 0
 Locators:
}
Part {
 Name: "right"
 PieceCount: 1
 LocatorCount: 0
 Pieces: 1
 Locators:
}`
	sc, err := ParseWithOptions(src, Options{VisibleParts: map[string]bool{"left": true, "right": false}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Triangles) != 1 || len(sc.Vertices) != 3 {
		t.Fatalf("verts=%d tris=%d", len(sc.Vertices), len(sc.Triangles))
	}
	if sc.Vertices[0].Position.X != 0 {
		t.Fatalf("wrong piece imported: %+v", sc.Vertices[0])
	}
}

func TestSparseMaterialSlotsPreserveTriangleBindings(t *testing.T) {
	src := `Material {
 Index: 0
 Alias: "body"
}
Material {
 Index: 2
 Alias: "front_light"
}
Material {
 Index: 5
 Alias: "rear_light"
}
Piece {
 Index: 0
 Material: 2
 Stream {
  Tag: "_POSITION"
  0 ( 0 0 0 )
  1 ( 1 0 0 )
  2 ( 0 1 0 )
 }
 Triangles {
  0 ( 0 1 2 )
 }
}
Piece {
 Index: 1
 Material: 5
 Stream {
  Tag: "_POSITION"
  0 ( 0 0 1 )
  1 ( 1 0 1 )
  2 ( 0 1 1 )
 }
 Triangles {
  0 ( 0 1 2 )
 }
}`
	sc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Materials) != 6 {
		t.Fatalf("material slots=%d", len(sc.Materials))
	}
	if sc.Materials[2].Alias != "front_light" || sc.Materials[5].Alias != "rear_light" {
		t.Fatalf("sparse slots shifted: m2=%q m5=%q", sc.Materials[2].Alias, sc.Materials[5].Alias)
	}
	if len(sc.Triangles) != 2 || sc.Triangles[0].Material != 2 || sc.Triangles[1].Material != 5 {
		t.Fatalf("triangle material bindings changed: %+v", sc.Triangles)
	}
}
