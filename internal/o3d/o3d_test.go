package o3d

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	m := &Model{Transform: IdentityTransform(), Vertices: []Vertex{{X: 0}, {X: 1}, {Y: 1}}, Triangles: []Triangle{{0, 1, 2, 0}}, Materials: []Material{{Diffuse: [4]float32{1, 1, 1, 1}, Texture: "x.png"}}}
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	r, e := Parse(b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Vertices) != 3 || len(r.Triangles) != 1 || r.Materials[0].Texture != "x.png" {
		t.Fatalf("bad roundtrip %+v", r)
	}
}
