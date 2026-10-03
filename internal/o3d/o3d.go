package o3d

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

type Vertex struct {
	X, Y, Z    float32
	NX, NY, NZ float32
	U, V       float32
}

type Triangle struct {
	A, B, C  uint32
	Material uint16
}

type Material struct {
	Diffuse       [4]float32
	Specular      [3]float32
	Emission      [3]float32
	SpecularPower float32
	Texture       string
}

type Model struct {
	Vertices  []Vertex
	Triangles []Triangle
	Materials []Material
	Transform [16]float32
}

func IdentityTransform() [16]float32 {
	return [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

func WriteFile(path string, m *Model) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Write(f, m)
}

func Write(w io.Writer, m *Model) error {
	if m == nil {
		return fmt.Errorf("o3d: nil model")
	}
	if len(m.Materials) > math.MaxUint16 {
		return fmt.Errorf("o3d: too many materials: %d", len(m.Materials))
	}
	for _, t := range m.Triangles {
		if int(t.A) >= len(m.Vertices) || int(t.B) >= len(m.Vertices) || int(t.C) >= len(m.Vertices) {
			return fmt.Errorf("o3d: triangle index out of range")
		}
		if int(t.Material) >= len(m.Materials) && len(m.Materials) != 0 {
			return fmt.Errorf("o3d: triangle material %d out of range", t.Material)
		}
	}
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	put := func(v any) error { return binary.Write(bw, binary.LittleEndian, v) }

	// OMSI O3D v7: 0x84,0x19,version; extended header options + encryption key.
	if err := put([3]byte{0x84, 0x19, 7}); err != nil {
		return err
	}
	// Bit 0 = 32-bit triangle indices. Bit 1 (alternative encryption seed) is not needed for unencrypted files.
	if err := put(uint8(1)); err != nil {
		return err
	}
	if err := put(uint32(0xffffffff)); err != nil {
		return err
	}

	if err := put(uint8(0x17)); err != nil {
		return err
	}
	if err := put(uint32(len(m.Vertices))); err != nil {
		return err
	}
	for _, v := range m.Vertices {
		vals := [...]float32{v.X, v.Y, v.Z, v.NX, v.NY, v.NZ, v.U, v.V}
		if err := put(vals); err != nil {
			return err
		}
	}

	if err := put(uint8(0x49)); err != nil {
		return err
	}
	if err := put(uint32(len(m.Triangles))); err != nil {
		return err
	}
	for _, t := range m.Triangles {
		// O3D stores three uint32 indices followed by a uint16 material index.
		if err := put(t.A); err != nil {
			return err
		}
		if err := put(t.B); err != nil {
			return err
		}
		if err := put(t.C); err != nil {
			return err
		}
		if err := put(t.Material); err != nil {
			return err
		}
	}

	if len(m.Materials) > 0 {
		if err := put(uint8(0x26)); err != nil {
			return err
		}
		if err := put(uint16(len(m.Materials))); err != nil {
			return err
		}
		for _, mat := range m.Materials {
			vals := [...]float32{
				mat.Diffuse[0], mat.Diffuse[1], mat.Diffuse[2], mat.Diffuse[3],
				mat.Specular[0], mat.Specular[1], mat.Specular[2],
				mat.Emission[0], mat.Emission[1], mat.Emission[2], mat.SpecularPower,
			}
			if err := put(vals); err != nil {
				return err
			}
			b := cp1252BestEffort(mat.Texture)
			if len(b) > 255 {
				b = b[:255]
			}
			// Pascal string: one-byte length then bytes. Python struct '<Np' uses exactly this representation.
			if err := put(uint8(len(b))); err != nil {
				return err
			}
			if _, err := bw.Write(b); err != nil {
				return err
			}
		}
	}

	if err := put(uint8(0x79)); err != nil {
		return err
	}
	tr := m.Transform
	zero := true
	for _, v := range tr {
		if v != 0 {
			zero = false
			break
		}
	}
	if zero {
		tr = IdentityTransform()
	}
	if err := put(tr); err != nil {
		return err
	}
	return bw.Flush()
}

// Read validates the subset produced by Write and is used by tests/quality control.
func Read(r io.Reader) (*Model, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

func Parse(b []byte) (*Model, error) {
	if len(b) < 8 || b[0] != 0x84 || b[1] != 0x19 {
		return nil, fmt.Errorf("o3d: invalid header")
	}
	version := b[2]
	off := 3
	longHeader := version > 3
	longTri := false
	if longHeader {
		if off+5 > len(b) {
			return nil, io.ErrUnexpectedEOF
		}
		opts := b[off]
		off++
		longTri = opts&1 != 0
		key := binary.LittleEndian.Uint32(b[off:])
		off += 4
		if key != 0xffffffff {
			return nil, fmt.Errorf("o3d: encrypted files not supported by validator")
		}
	}
	m := &Model{Transform: IdentityTransform()}
	readCount := func() (uint32, error) {
		if longHeader {
			if off+4 > len(b) {
				return 0, io.ErrUnexpectedEOF
			}
			v := binary.LittleEndian.Uint32(b[off:])
			off += 4
			return v, nil
		}
		if off+2 > len(b) {
			return 0, io.ErrUnexpectedEOF
		}
		v := uint32(binary.LittleEndian.Uint16(b[off:]))
		off += 2
		return v, nil
	}
	f32 := func() (float32, error) {
		if off+4 > len(b) {
			return 0, io.ErrUnexpectedEOF
		}
		v := math.Float32frombits(binary.LittleEndian.Uint32(b[off:]))
		off += 4
		return v, nil
	}
	for off < len(b) {
		sec := b[off]
		off++
		switch sec {
		case 0x17:
			n, e := readCount()
			if e != nil {
				return nil, e
			}
			if uint64(n)*32 > uint64(len(b)-off) {
				return nil, fmt.Errorf("o3d: invalid vertex count")
			}
			m.Vertices = make([]Vertex, n)
			for i := range m.Vertices {
				a := make([]float32, 8)
				for j := range a {
					a[j], e = f32()
					if e != nil {
						return nil, e
					}
				}
				m.Vertices[i] = Vertex{a[0], a[1], a[2], a[3], a[4], a[5], a[6], a[7]}
			}
		case 0x49:
			n, e := readCount()
			if e != nil {
				return nil, e
			}
			m.Triangles = make([]Triangle, n)
			for i := range m.Triangles {
				var a, c, d uint32
				var mat uint16
				if longTri {
					if off+14 > len(b) {
						return nil, io.ErrUnexpectedEOF
					}
					a = binary.LittleEndian.Uint32(b[off:])
					c = binary.LittleEndian.Uint32(b[off+4:])
					d = binary.LittleEndian.Uint32(b[off+8:])
					mat = binary.LittleEndian.Uint16(b[off+12:])
					off += 14
				} else {
					if off+8 > len(b) {
						return nil, io.ErrUnexpectedEOF
					}
					a = uint32(binary.LittleEndian.Uint16(b[off:]))
					c = uint32(binary.LittleEndian.Uint16(b[off+2:]))
					d = uint32(binary.LittleEndian.Uint16(b[off+4:]))
					mat = binary.LittleEndian.Uint16(b[off+6:])
					off += 8
				}
				m.Triangles[i] = Triangle{a, c, d, mat}
			}
		case 0x26:
			if off+2 > len(b) {
				return nil, io.ErrUnexpectedEOF
			}
			n := int(binary.LittleEndian.Uint16(b[off:]))
			off += 2
			m.Materials = make([]Material, n)
			for i := 0; i < n; i++ {
				vals := make([]float32, 11)
				var e error
				for j := range vals {
					vals[j], e = f32()
					if e != nil {
						return nil, e
					}
				}
				if off >= len(b) {
					return nil, io.ErrUnexpectedEOF
				}
				ln := int(b[off])
				off++
				if off+ln > len(b) {
					return nil, io.ErrUnexpectedEOF
				}
				tex := string(b[off : off+ln])
				off += ln
				m.Materials[i] = Material{Diffuse: [4]float32{vals[0], vals[1], vals[2], vals[3]}, Specular: [3]float32{vals[4], vals[5], vals[6]}, Emission: [3]float32{vals[7], vals[8], vals[9]}, SpecularPower: vals[10], Texture: tex}
			}
		case 0x54:
			// Writer does not emit bones. Reject instead of guessing length.
			return nil, fmt.Errorf("o3d: bone section not supported by validator")
		case 0x79:
			for i := 0; i < 16; i++ {
				v, e := f32()
				if e != nil {
					return nil, e
				}
				m.Transform[i] = v
			}
		default:
			return nil, fmt.Errorf("o3d: unknown section 0x%02X at %d", sec, off-1)
		}
	}
	return m, nil
}

// O3D uses CP1252. ASCII/Latin-1 filenames are kept exactly; unsupported runes become '_'.
func cp1252BestEffort(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r >= 32 && r <= 255 {
			out = append(out, byte(r))
		} else if r == '\\' || r == '/' || r == '.' || r == '_' || r == '-' {
			out = append(out, byte(r))
		} else {
			out = append(out, '_')
		}
	}
	return out
}
