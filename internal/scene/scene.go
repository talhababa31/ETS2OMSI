package scene

import "math"

type Vec2 struct{ X, Y float64 }
type Vec3 struct{ X, Y, Z float64 }

type Material struct {
	Index   int    `json:"index"`
	Alias   string `json:"alias"`
	Effect  string `json:"effect,omitempty"`
	Texture string `json:"texture,omitempty"`
	Class   string `json:"class,omitempty"`
	Alpha   bool   `json:"alpha,omitempty"`
	// Tint is the ETS2 material "diffuse" colour (0..1) that multiplies the
	// texture. HasTint is false when the material is neutral/white.
	Tint    [3]float64 `json:"tint,omitempty"`
	HasTint bool       `json:"has_tint,omitempty"`
	// BaseTexture is the untinted texture when Tint was baked into Texture;
	// colour variants re-tint it instead of recolouring the baked result.
	BaseTexture string `json:"base_texture,omitempty"`
}
type Vertex struct {
	Position Vec3 `json:"position"`
	Normal   Vec3 `json:"normal"`
	UV       Vec2 `json:"uv"`
}
type Triangle struct {
	A, B, C  int
	Material int
}
type Locator struct {
	Name     string `json:"name"`
	Hookup   string `json:"hookup,omitempty"`
	Position Vec3   `json:"position"`
}
type Scene struct {
	Vertices  []Vertex   `json:"vertices"`
	Triangles []Triangle `json:"triangles"`
	Materials []Material `json:"materials"`
	Locators  []Locator  `json:"locators,omitempty"`
	Source    string     `json:"source,omitempty"`
}
type Bounds struct{ Min, Max Vec3 }

func (s Scene) Bounds() Bounds {
	b := Bounds{Min: Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}, Max: Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}}
	if len(s.Vertices) == 0 {
		return Bounds{}
	}
	for _, v := range s.Vertices {
		p := v.Position
		if p.X < b.Min.X {
			b.Min.X = p.X
		}
		if p.Y < b.Min.Y {
			b.Min.Y = p.Y
		}
		if p.Z < b.Min.Z {
			b.Min.Z = p.Z
		}
		if p.X > b.Max.X {
			b.Max.X = p.X
		}
		if p.Y > b.Max.Y {
			b.Max.Y = p.Y
		}
		if p.Z > b.Max.Z {
			b.Max.Z = p.Z
		}
	}
	return b
}
func (b Bounds) Width() float64  { return b.Max.X - b.Min.X }
func (b Bounds) Length() float64 { return b.Max.Y - b.Min.Y }
func (b Bounds) Height() float64 { return b.Max.Z - b.Min.Z }
func (s *Scene) GroundAndCenter() {
	b := s.Bounds()
	cx := (b.Min.X + b.Max.X) / 2
	cy := (b.Min.Y + b.Max.Y) / 2
	dz := b.Min.Z
	for i := range s.Vertices {
		s.Vertices[i].Position.X -= cx
		s.Vertices[i].Position.Y -= cy
		s.Vertices[i].Position.Z -= dz
	}
	for i := range s.Locators {
		s.Locators[i].Position.X -= cx
		s.Locators[i].Position.Y -= cy
		s.Locators[i].Position.Z -= dz
	}
}
func (s *Scene) Translate(dx, dy, dz float64) {
	for i := range s.Vertices {
		s.Vertices[i].Position.X += dx
		s.Vertices[i].Position.Y += dy
		s.Vertices[i].Position.Z += dz
	}
	for i := range s.Locators {
		s.Locators[i].Position.X += dx
		s.Locators[i].Position.Y += dy
		s.Locators[i].Position.Z += dz
	}
}
func (s Scene) TriangleCountByMaterial() map[int]int {
	m := map[int]int{}
	for _, t := range s.Triangles {
		m[t.Material]++
	}
	return m
}

// AppendTranslated merges another scene while preserving UVs/normals/materials.
// The source is not mutated. It is used for SCS wheel/accessory composition.
func (s *Scene) AppendTranslated(src Scene, dx, dy, dz float64) {
	vbase := len(s.Vertices)
	mbase := len(s.Materials)
	for _, v := range src.Vertices {
		v.Position.X += dx
		v.Position.Y += dy
		v.Position.Z += dz
		s.Vertices = append(s.Vertices, v)
	}
	for _, m := range src.Materials {
		m.Index = mbase + len(s.Materials) - mbase
		s.Materials = append(s.Materials, m)
	}
	for _, t := range src.Triangles {
		t.A += vbase
		t.B += vbase
		t.C += vbase
		t.Material += mbase
		s.Triangles = append(s.Triangles, t)
	}
	for _, l := range src.Locators {
		l.Position.X += dx
		l.Position.Y += dy
		l.Position.Z += dz
		s.Locators = append(s.Locators, l)
	}
}

func (v Vec3) Length() float64 { return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z) }

// Clone returns a deep copy, so translating it never moves the original.
func (s Scene) Clone() Scene {
	c := s
	c.Vertices = append([]Vertex(nil), s.Vertices...)
	c.Triangles = append([]Triangle(nil), s.Triangles...)
	c.Materials = append([]Material(nil), s.Materials...)
	c.Locators = append([]Locator(nil), s.Locators...)
	return c
}
