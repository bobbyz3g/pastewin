package clipimage

import (
	"encoding/binary"
	"errors"
	"image"
	"math/bits"
)

var invalidDIB = errors.New("图片位图数据损坏或格式暂不支持")

// DecodeDIB accepts Windows uncompressed RGB and bitfield DIBs. Compressed,
// linked-profile and calibrated colour-space DIBs are rejected, never guessed.
func DecodeDIB(data []byte) (*image.NRGBA, error) {
	if len(data) < 40 || len(data) > MaxRawBytes {
		return nil, invalidDIB
	}
	u32 := func(i int) uint32 { return binary.LittleEndian.Uint32(data[i:]) }
	u16 := func(i int) uint16 { return binary.LittleEndian.Uint16(data[i:]) }
	header := int(u32(0))
	if (header != 40 && header != 52 && header != 56 && header != 108 && header != 124) || header > len(data) {
		return nil, invalidDIB
	}
	w, signedH := int(int32(u32(4))), int64(int32(u32(8)))
	h := int(signedH)
	if h < 0 {
		h = -h
	}
	if err := CheckDimensions(w, h); err != nil {
		return nil, err
	}
	bpp, compression := int(u16(14)), u32(16)
	if u16(12) != 1 || (bpp != 1 && bpp != 4 && bpp != 8 && bpp != 16 && bpp != 24 && bpp != 32) {
		return nil, invalidDIB
	}
	if compression != 0 && compression != 3 && compression != 6 {
		return nil, invalidDIB
	}
	if compression != 0 && bpp != 16 && bpp != 32 {
		return nil, invalidDIB
	}
	if compression == 6 && header != 40 && header < 56 {
		return nil, invalidDIB
	}
	if header >= 108 && u32(56) != 0x73524742 && u32(56) != 0x57696e20 {
		return nil, invalidDIB
	}
	if header == 124 && (u32(112) != 0 || u32(116) != 0) {
		return nil, invalidDIB
	}
	offset := header
	masks := [4]uint32{0x00ff0000, 0x0000ff00, 0x000000ff, 0}
	if bpp == 16 {
		masks = [4]uint32{0x7c00, 0x03e0, 0x001f, 0}
	}
	if compression != 0 {
		count := 3
		if header >= 56 || compression == 6 {
			count = 4
		}
		if header == 40 {
			offset += count * 4
		}
		if 40+count*4 > len(data) {
			return nil, invalidDIB
		}
		for i := 0; i < count; i++ {
			masks[i] = u32(40 + i*4)
		}
		var used uint32
		for i, m := range masks {
			if m == 0 {
				if i < 3 || compression == 6 {
					return nil, invalidDIB
				}
				continue
			}
			v := m >> bits.TrailingZeros32(m)
			if v&(v+1) != 0 || m&used != 0 || (bpp == 16 && m > 0xffff) {
				return nil, invalidDIB
			}
			used |= m
		}
	}
	paletteCount := int64(u32(32))
	if bpp <= 8 {
		if paletteCount == 0 {
			paletteCount = 1 << bpp
		}
		if paletteCount > 1<<bpp {
			return nil, invalidDIB
		}
	}
	paletteStart := offset
	if paletteCount*4 > int64(len(data)-offset) {
		return nil, invalidDIB
	}
	offset += int(paletteCount) * 4
	stride := ((int64(w)*int64(bpp) + 31) / 32) * 4
	if stride*int64(h) > int64(len(data)-offset) {
		return nil, invalidDIB
	}
	pixels := image.NewNRGBA(image.Rect(0, 0, w, h))
	channel := func(value, mask uint32) byte {
		if mask == 0 {
			return 255
		}
		shift := bits.TrailingZeros32(mask)
		value = (value & mask) >> shift
		max := mask >> shift
		return byte((uint64(value)*255 + uint64(max)/2) / uint64(max))
	}
	for y := 0; y < h; y++ {
		sy := y
		if signedH > 0 {
			sy = h - 1 - y
		}
		row := data[offset+sy*int(stride):]
		for x := 0; x < w; x++ {
			i := y*pixels.Stride + x*4
			out := pixels.Pix[i : i+4]
			switch bpp {
			case 1, 4, 8:
				index := int(row[x*bpp/8]>>(8-bpp-(x*bpp%8))) & ((1 << bpp) - 1)
				if int64(index) >= paletteCount {
					return nil, invalidDIB
				}
				p := paletteStart + index*4
				out[0], out[1], out[2], out[3] = data[p+2], data[p+1], data[p], 255
			case 24:
				p := x * 3
				out[0], out[1], out[2], out[3] = row[p+2], row[p+1], row[p], 255
			default:
				var value uint32
				if bpp == 16 {
					value = uint32(binary.LittleEndian.Uint16(row[x*2:]))
				} else {
					value = binary.LittleEndian.Uint32(row[x*4:])
				}
				for c := 0; c < 4; c++ {
					out[c] = channel(value, masks[c])
				}
			}
		}
	}
	return pixels, nil
}

// EncodeDIBV5 produces top-down BGRA with explicit straight-alpha masks and sRGB.
func EncodeDIBV5(pixels *image.NRGBA) ([]byte, error) {
	w, h := pixels.Bounds().Dx(), pixels.Bounds().Dy()
	if err := CheckDimensions(w, h); err != nil {
		return nil, err
	}
	data := make([]byte, 124+w*h*4)
	put := func(i int, v uint32) { binary.LittleEndian.PutUint32(data[i:], v) }
	put(0, 124)
	put(4, uint32(w))
	put(8, uint32(-int32(h)))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	put(16, 3)
	put(20, uint32(w*h*4))
	put(40, 0x00ff0000)
	put(44, 0x0000ff00)
	put(48, 0x000000ff)
	put(52, 0xff000000)
	put(56, 0x73524742)
	put(108, 4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := y*pixels.Stride + x*4
			d := 124 + (y*w+x)*4
			data[d], data[d+1], data[d+2], data[d+3] = pixels.Pix[s+2], pixels.Pix[s+1], pixels.Pix[s], pixels.Pix[s+3]
		}
	}
	return data, nil
}
