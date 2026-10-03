// Package dds decodes the top mip level of DirectDraw Surface textures used by
// ETS2 packages (BC1/DXT1, BC2/DXT3, BC3/DXT5, BC4, BC5 and uncompressed
// RGB(A) formats, legacy and DX10 headers). It is used for the 3D preview and
// for sampling colours; exported OMSI textures are copied unchanged.
package dds

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math/bits"
)

const (
	ddpfAlphaPixels = 0x1
	ddpfFourCC      = 0x4
	ddpfRGB         = 0x40
	ddpfLuminance   = 0x20000
)

type format int

const (
	fmtUnknown format = iota
	fmtBC1
	fmtBC2
	fmtBC3
	fmtBC4
	fmtBC5
	fmtMasked // uncompressed, described by bit masks
)

type header struct {
	width, height uint32
	format        format
	bitCount      uint32
	masks         [4]uint32 // r, g, b, a
	dataOffset    int
}

func parseHeader(b []byte) (header, error) {
	var h header
	if len(b) < 128 || string(b[:4]) != "DDS " {
		return h, errors.New("dds: not a DDS file")
	}
	le := binary.LittleEndian
	h.height = le.Uint32(b[12:])
	h.width = le.Uint32(b[16:])
	pfFlags := le.Uint32(b[80:])
	fourCC := string(b[84:88])
	h.bitCount = le.Uint32(b[88:])
	h.masks = [4]uint32{le.Uint32(b[92:]), le.Uint32(b[96:]), le.Uint32(b[100:]), le.Uint32(b[104:])}
	if pfFlags&ddpfAlphaPixels == 0 {
		h.masks[3] = 0
	}
	h.dataOffset = 128
	if h.width == 0 || h.height == 0 || h.width > 16384 || h.height > 16384 {
		return h, fmt.Errorf("dds: bad size %dx%d", h.width, h.height)
	}
	switch {
	case pfFlags&ddpfFourCC != 0:
		switch fourCC {
		case "DXT1":
			h.format = fmtBC1
		case "DXT2", "DXT3":
			h.format = fmtBC2
		case "DXT4", "DXT5":
			h.format = fmtBC3
		case "ATI1", "BC4U":
			h.format = fmtBC4
		case "ATI2", "BC5U":
			h.format = fmtBC5
		case "DX10":
			if len(b) < 148 {
				return h, errors.New("dds: truncated DX10 header")
			}
			h.dataOffset = 148
			switch le.Uint32(b[128:]) {
			case 70, 71, 72:
				h.format = fmtBC1
			case 73, 74, 75:
				h.format = fmtBC2
			case 76, 77, 78:
				h.format = fmtBC3
			case 79, 80:
				h.format = fmtBC4
			case 82, 83:
				h.format = fmtBC5
			case 27, 28, 29: // R8G8B8A8
				h.format, h.bitCount = fmtMasked, 32
				h.masks = [4]uint32{0xff, 0xff00, 0xff0000, 0xff000000}
			case 87, 91: // B8G8R8A8
				h.format, h.bitCount = fmtMasked, 32
				h.masks = [4]uint32{0xff0000, 0xff00, 0xff, 0xff000000}
			case 88, 93: // B8G8R8X8
				h.format, h.bitCount = fmtMasked, 32
				h.masks = [4]uint32{0xff0000, 0xff00, 0xff, 0}
			default:
				return h, fmt.Errorf("dds: unsupported DXGI format %d", le.Uint32(b[128:]))
			}
		default:
			return h, fmt.Errorf("dds: unsupported FourCC %q", fourCC)
		}
	case pfFlags&(ddpfRGB|ddpfLuminance) != 0 || h.bitCount > 0:
		if h.bitCount != 8 && h.bitCount != 16 && h.bitCount != 24 && h.bitCount != 32 {
			return h, fmt.Errorf("dds: unsupported bit count %d", h.bitCount)
		}
		h.format = fmtMasked
		if pfFlags&ddpfLuminance != 0 && h.masks[1] == 0 && h.masks[2] == 0 {
			h.masks[1], h.masks[2] = h.masks[0], h.masks[0]
		}
	default:
		return h, errors.New("dds: unknown pixel format")
	}
	return h, nil
}

// DecodeConfig returns the size of the top mip level.
func DecodeConfig(b []byte) (int, int, error) {
	h, err := parseHeader(b)
	return int(h.width), int(h.height), err
}

// Decode returns the top mip level as NRGBA.
func Decode(b []byte) (*image.NRGBA, error) {
	h, err := parseHeader(b)
	if err != nil {
		return nil, err
	}
	w, ht := int(h.width), int(h.height)
	img := image.NewNRGBA(image.Rect(0, 0, w, ht))
	data := b[h.dataOffset:]
	switch h.format {
	case fmtMasked:
		return img, decodeMasked(img, data, h)
	default:
		blockSize := 16
		if h.format == fmtBC1 || h.format == fmtBC4 {
			blockSize = 8
		}
		bw, bh := (w+3)/4, (ht+3)/4
		if len(data) < bw*bh*blockSize {
			return nil, errors.New("dds: truncated block data")
		}
		var px [16]color.NRGBA
		for by := 0; by < bh; by++ {
			for bx := 0; bx < bw; bx++ {
				blk := data[(by*bw+bx)*blockSize:]
				switch h.format {
				case fmtBC1:
					decodeColorBlock(blk[:8], &px, true)
				case fmtBC2:
					decodeColorBlock(blk[8:16], &px, false)
					for i := 0; i < 16; i++ {
						a := (blk[i/2] >> (4 * uint(i%2))) & 0xf
						px[i].A = a * 17
					}
				case fmtBC3:
					decodeColorBlock(blk[8:16], &px, false)
					var a [16]uint8
					decodeAlphaBlock(blk[:8], &a)
					for i := range px {
						px[i].A = a[i]
					}
				case fmtBC4:
					var r [16]uint8
					decodeAlphaBlock(blk[:8], &r)
					for i := range px {
						px[i] = color.NRGBA{r[i], r[i], r[i], 255}
					}
				case fmtBC5:
					var r, g [16]uint8
					decodeAlphaBlock(blk[:8], &r)
					decodeAlphaBlock(blk[8:16], &g)
					for i := range px {
						px[i] = color.NRGBA{r[i], g[i], 255, 255}
					}
				}
				for i := 0; i < 16; i++ {
					x, y := bx*4+i%4, by*4+i/4
					if x < w && y < ht {
						img.SetNRGBA(x, y, px[i])
					}
				}
			}
		}
	}
	return img, nil
}

func rgb565(c uint16) color.NRGBA {
	r := uint8(c >> 11 & 0x1f)
	g := uint8(c >> 5 & 0x3f)
	b := uint8(c & 0x1f)
	return color.NRGBA{r<<3 | r>>2, g<<2 | g>>4, b<<3 | b>>2, 255}
}

func mix(a, b color.NRGBA, wa, wb, d int) color.NRGBA {
	return color.NRGBA{
		uint8((int(a.R)*wa + int(b.R)*wb) / d),
		uint8((int(a.G)*wa + int(b.G)*wb) / d),
		uint8((int(a.B)*wa + int(b.B)*wb) / d),
		255,
	}
}

func decodeColorBlock(b []byte, px *[16]color.NRGBA, bc1 bool) {
	c0 := binary.LittleEndian.Uint16(b[0:])
	c1 := binary.LittleEndian.Uint16(b[2:])
	var pal [4]color.NRGBA
	pal[0], pal[1] = rgb565(c0), rgb565(c1)
	if c0 > c1 || !bc1 {
		pal[2] = mix(pal[0], pal[1], 2, 1, 3)
		pal[3] = mix(pal[0], pal[1], 1, 2, 3)
	} else {
		pal[2] = mix(pal[0], pal[1], 1, 1, 2)
		pal[3] = color.NRGBA{0, 0, 0, 0}
	}
	idx := binary.LittleEndian.Uint32(b[4:])
	for i := 0; i < 16; i++ {
		px[i] = pal[idx>>(2*uint(i))&3]
	}
}

func decodeAlphaBlock(b []byte, out *[16]uint8) {
	a0, a1 := int(b[0]), int(b[1])
	var pal [8]int
	pal[0], pal[1] = a0, a1
	if a0 > a1 {
		for i := 1; i < 7; i++ {
			pal[i+1] = ((7-i)*a0 + i*a1) / 7
		}
	} else {
		for i := 1; i < 5; i++ {
			pal[i+1] = ((5-i)*a0 + i*a1) / 5
		}
		pal[6], pal[7] = 0, 255
	}
	var idx uint64
	for i := 0; i < 6; i++ {
		idx |= uint64(b[2+i]) << (8 * uint(i))
	}
	for i := 0; i < 16; i++ {
		out[i] = uint8(pal[idx>>(3*uint(i))&7])
	}
}

func channel(v, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := bits.TrailingZeros32(mask)
	n := bits.OnesCount32(mask)
	x := (v & mask) >> uint(shift)
	max := uint32(1)<<uint(n) - 1
	return uint8(x * 255 / max)
}

func decodeMasked(img *image.NRGBA, data []byte, h header) error {
	bpp := int(h.bitCount / 8)
	w, ht := int(h.width), int(h.height)
	pitch := w * bpp
	if len(data) < pitch*ht {
		return errors.New("dds: truncated pixel data")
	}
	for y := 0; y < ht; y++ {
		row := data[y*pitch:]
		for x := 0; x < w; x++ {
			var v uint32
			for i := 0; i < bpp; i++ {
				v |= uint32(row[x*bpp+i]) << (8 * uint(i))
			}
			c := color.NRGBA{channel(v, h.masks[0]), channel(v, h.masks[1]), channel(v, h.masks[2]), 255}
			if h.masks[3] != 0 {
				c.A = channel(v, h.masks[3])
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return nil
}
