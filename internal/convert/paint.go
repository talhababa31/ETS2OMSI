package convert

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"ets2omsi/internal/dds"
	"ets2omsi/internal/scene"
)

// decodeTextureFile decodes a texture copied into the OMSI texture folder.
func decodeTextureFile(p string) (image.Image, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(p)) {
	case ".dds":
		return dds.Decode(b)
	case ".png", ".jpg", ".jpeg":
		im, _, err := image.Decode(bytes.NewReader(b))
		return im, err
	}
	return nil, errors.New("unsupported texture format: " + filepath.Ext(p))
}

// dominantColor returns the most common opaque colour of an image (quantized
// histogram, then averaged inside the winning bucket).
func dominantColor(im image.Image) (color.NRGBA, bool) {
	type acc struct{ n, r, g, b int }
	hist := map[int]*acc{}
	bd := im.Bounds()
	step := 1
	if bd.Dx()*bd.Dy() > 512*512 {
		step = 2
	}
	best, bestN := -1, 0
	for y := bd.Min.Y; y < bd.Max.Y; y += step {
		for x := bd.Min.X; x < bd.Max.X; x += step {
			c := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
			if c.A < 128 {
				continue
			}
			k := int(c.R>>4)<<8 | int(c.G>>4)<<4 | int(c.B>>4)
			a := hist[k]
			if a == nil {
				a = &acc{}
				hist[k] = a
			}
			a.n++
			a.r += int(c.R)
			a.g += int(c.G)
			a.b += int(c.B)
			if a.n > bestN {
				best, bestN = k, a.n
			}
		}
	}
	if best < 0 {
		return color.NRGBA{}, false
	}
	a := hist[best]
	return color.NRGBA{uint8(a.r / a.n), uint8(a.g / a.n), uint8(a.b / a.n), 255}, true
}

func isPaintLikeClass(class string) bool {
	return class == "" || class == "body" || class == "paint"
}

// scenePaintColor estimates the vehicle's paint colour from the resolved
// texture that covers the most body/paint triangles. ETS2 paint materials
// often reference shared base.scs textures that an SCS-only conversion cannot
// read; filling them with the car's own dominant body colour keeps the car
// looking like one paint job instead of having black panels.
func scenePaintColor(sc *scene.Scene, texDir string) (color.NRGBA, bool) {
	weight := map[string]int{}
	for _, t := range sc.Triangles {
		if t.Material < 0 || t.Material >= len(sc.Materials) {
			continue
		}
		m := sc.Materials[t.Material]
		if m.Texture == "" || !isPaintLikeClass(m.Class) || strings.HasPrefix(m.Texture, "fallback_") || strings.HasPrefix(m.Texture, "optional_") || strings.HasPrefix(m.Texture, "semantic_") {
			continue
		}
		weight[m.Texture]++
	}
	best, bestN := "", 0
	for tex, n := range weight {
		if n > bestN || (n == bestN && tex < best) {
			best, bestN = tex, n
		}
	}
	if best == "" {
		return color.NRGBA{}, false
	}
	im, err := decodeTextureFile(filepath.Join(texDir, best))
	if err != nil {
		return color.NRGBA{}, false
	}
	return dominantColor(im)
}

// writePaintFallback writes (once) a solid paint texture and returns its name.
func writePaintFallback(texDir string, c color.NRGBA, tr *TextureReport) string {
	name := fmt.Sprintf("fallback_paint_%02x%02x%02x.png", c.R, c.G, c.B)
	p := filepath.Join(texDir, name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		im := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		for i := 0; i < len(im.Pix); i += 4 {
			im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c.R, c.G, c.B, 255
		}
		f, err := os.Create(p)
		if err != nil {
			return ""
		}
		err = png.Encode(f, im)
		if ce := f.Close(); err == nil {
			err = ce
		}
		if err != nil {
			return ""
		}
		if tr != nil {
			tr.GeneratedFallbacks++
			tr.Files = append(tr.Files, name)
		}
	}
	return name
}

func tintColor(t [3]float64) color.NRGBA {
	c := func(v float64) uint8 {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return uint8(v*255 + .5)
	}
	return color.NRGBA{c(t[0]), c(t[1]), c(t[2]), 255}
}

// bakeTint writes texture × ETS2 diffuse colour as a new PNG next to the
// original and returns its name. ETS2 traffic cars commonly use a grey/white
// body texture coloured by the material's diffuse value; OMSI would show it
// grey without this. Returns "" when the texture cannot be decoded.
func bakeTint(texDir, tex string, tint [3]float64) string {
	im, err := decodeTextureFile(filepath.Join(texDir, tex))
	if err != nil {
		return ""
	}
	tc := tintColor(tint)
	stem := strings.TrimSuffix(tex, filepath.Ext(tex))
	name := fmt.Sprintf("%s_t%02x%02x%02x.png", strings.ReplaceAll(stem, " ", "_"), tc.R, tc.G, tc.B)
	p := filepath.Join(texDir, name)
	if _, err := os.Stat(p); err == nil {
		return name
	}
	b := im.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
			out.SetNRGBA(x, y, color.NRGBA{
				uint8(float64(c.R) * tint[0]),
				uint8(float64(c.G) * tint[1]),
				uint8(float64(c.B) * tint[2]),
				c.A,
			})
		}
	}
	f, err := os.Create(p)
	if err != nil {
		return ""
	}
	err = png.Encode(f, out)
	if ce := f.Close(); err == nil {
		err = ce
	}
	if err != nil {
		_ = os.Remove(p)
		return ""
	}
	return name
}
