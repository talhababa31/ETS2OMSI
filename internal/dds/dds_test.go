package dds

import (
	"image"
	"image/png"
	"os"
	"testing"
)

func loadPNG(t *testing.T, p string) image.Image {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return im
}

// Fixtures: ImageMagick encoded testdata/grad.png to DDS, and decoded each
// DDS back to *_ref.png. Our decoder must match ImageMagick's decoder.
func TestDecodeAgainstReference(t *testing.T) {
	for _, tc := range []struct {
		file     string
		tolRGB   int
		tolAlpha int
	}{
		{"testdata/rgba.dds", 1, 1},
		{"testdata/dxt5.dds", 2, 1},
		{"testdata/dxt1.dds", 2, 1},
	} {
		ref := loadPNG(t, tc.file[:len(tc.file)-4]+"_ref.png")
		b, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		im, err := Decode(b)
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if im.Bounds() != ref.Bounds() {
			t.Fatalf("%s: bounds %v want %v", tc.file, im.Bounds(), ref.Bounds())
		}
		for y := 0; y < im.Bounds().Dy(); y++ {
			for x := 0; x < im.Bounds().Dx(); x++ {
				r0, g0, b0, a0 := ref.At(x, y).RGBA()
				c := im.NRGBAAt(x, y)
				// reference is non-premultiplied only where alpha > 0; compare
				// colour where alpha is meaningful.
				if a0>>8 > 0 && c.A > 0 {
					nr, ng, nb := int(r0*255/a0), int(g0*255/a0), int(b0*255/a0)
					if abs(nr-int(c.R)) > tc.tolRGB || abs(ng-int(c.G)) > tc.tolRGB || abs(nb-int(c.B)) > tc.tolRGB {
						t.Fatalf("%s (%d,%d): got %v want %d,%d,%d", tc.file, x, y, c, nr, ng, nb)
					}
				}
				if abs(int(a0>>8)-int(c.A)) > tc.tolAlpha {
					t.Fatalf("%s (%d,%d): alpha %d want %d", tc.file, x, y, c.A, a0>>8)
				}
			}
		}
	}
}

func TestRejectsGarbage(t *testing.T) {
	if _, err := Decode([]byte("not a dds")); err == nil {
		t.Fatal("expected error")
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
