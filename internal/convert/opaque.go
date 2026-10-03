package convert

import (
	"bytes"
	"encoding/binary"
	"image"
	"os"
	"path/filepath"
	"strings"

	"ets2omsi/internal/scene"
)

// ETS2 stores the specular/reflection mask in the alpha channel of opaque
// textures. OMSI uses texture alpha for transparency, which made body panels
// patchy and see-through in game. Every texture used by an opaque material is
// therefore written without an alpha channel.

type opaqueFixer struct {
	texDir string
	cache  map[string]string // texture -> opaque texture name
}

func newOpaqueFixer(texDir string) *opaqueFixer {
	return &opaqueFixer{texDir: texDir, cache: map[string]string{}}
}

func hasRealAlpha(im image.Image) bool {
	b := im.Bounds()
	if n, ok := im.(*image.NRGBA); ok {
		for i := 3; i < len(n.Pix); i += 4 {
			if n.Pix[i] < 250 {
				return true
			}
		}
		return false
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := im.At(x, y).RGBA(); a < 0xfa00 {
				return true
			}
		}
	}
	return false
}

// opaque returns a texture name whose alpha is fully opaque.
func (f *opaqueFixer) opaque(tex string) string {
	if tex == "" {
		return tex
	}
	if n, ok := f.cache[tex]; ok {
		return n
	}
	f.cache[tex] = tex
	im, err := decodeTextureFile(filepath.Join(f.texDir, tex))
	if err != nil || !hasRealAlpha(im) {
		return tex
	}
	name := strings.TrimSuffix(tex, filepath.Ext(tex)) + "_opq.dds"
	if err := os.WriteFile(filepath.Join(f.texDir, name), encodeDDSOpaque(im), 0644); err != nil {
		return tex
	}
	f.cache[tex] = name
	return name
}

// fixScene points every opaque material at an alpha-free texture.
func (f *opaqueFixer) fixScene(sc *scene.Scene) {
	for i := range sc.Materials {
		m := &sc.Materials[i]
		if m.Alpha {
			continue
		}
		m.Texture = f.opaque(m.Texture)
	}
}

// encodeDDSOpaque writes an uncompressed X8R8G8B8 DDS (no alpha channel) with
// a full mip chain.
func encodeDDSOpaque(im image.Image) []byte {
	b := encodeDDS(im)
	le := binary.LittleEndian
	le.PutUint32(b[80:], 0x40) // DDPF_RGB only, no ALPHAPIXELS
	le.PutUint32(b[104:], 0)   // no alpha mask
	for i := 128 + 3; i < len(b); i += 4 {
		b[i] = 0xff
	}
	return bytes.Clone(b)
}
