// Package clipimage converts bounded clipboard bitmaps to canonical PNG images.
package clipimage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/draw"
	"image/png"
)

const MaxPixels = 16_000_000
const MaxRawBytes = 64 << 20
const MaxPNGBytes = 20 << 20
const ThumbnailSize = 256

type Encoded struct {
	Format string
	Data   []byte
}

type Image struct {
	PNG           []byte
	Thumbnail     []byte
	Width, Height int
	Hash          string
}

func CheckDimensions(w, h int) error {
	if w <= 0 || h <= 0 || int64(w) > MaxPixels || int64(h) > MaxPixels || int64(w)*int64(h) > MaxPixels {
		return errors.New("图片尺寸无效或超过 1600 万像素，未保存")
	}
	return nil
}

func DecodePNG(data []byte) (*image.NRGBA, error) {
	if len(data) > MaxRawBytes {
		return nil, errors.New("图片超过 64 MiB")
	}
	c, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err = CheckDimensions(c.Width, c.Height); err != nil {
		return nil, err
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if n, ok := im.(*image.NRGBA); ok {
		return n, nil
	}
	n := image.NewNRGBA(im.Bounds())
	draw.Draw(n, n.Bounds(), im, im.Bounds().Min, draw.Src)
	return n, nil
}

// Prepare tries representations in source preference order; malformed PNG must
// not prevent a valid DIB fallback from being captured.
func Prepare(candidates []Encoded) (*Image, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	var last error
	for _, candidate := range candidates {
		var pixels *image.NRGBA
		if candidate.Format == "PNG" {
			pixels, last = DecodePNG(candidate.Data)
		} else if candidate.Format == "DIB" {
			pixels, last = DecodeDIB(candidate.Data)
		} else {
			last = errors.New("不支持的图片格式")
		}
		if last != nil {
			continue
		}
		var result *Image
		result, last = Normalize(pixels)
		if last == nil {
			return result, nil
		}
	}
	return nil, last
}

func Normalize(pixels *image.NRGBA) (*Image, error) {
	w, h := pixels.Bounds().Dx(), pixels.Bounds().Dy()
	if err := CheckDimensions(w, h); err != nil {
		return nil, err
	}
	hash := sha256.New()
	var dimensions [8]byte
	binary.LittleEndian.PutUint32(dimensions[:4], uint32(w))
	binary.LittleEndian.PutUint32(dimensions[4:], uint32(h))
	hash.Write(dimensions[:])
	for y := 0; y < h; y++ {
		row := pixels.Pix[y*pixels.Stride : y*pixels.Stride+w*4]
		// RGB under fully transparent pixels is invisible and should not affect deduplication.
		for x := 0; x < len(row); x += 4 {
			if row[x+3] == 0 {
				row[x], row[x+1], row[x+2] = 0, 0, 0
			}
		}
		hash.Write(row)
	}
	var full limitedPNG
	if err := png.Encode(&full, pixels); err != nil {
		return nil, err
	}
	thumb := thumbnail(pixels)
	var small bytes.Buffer
	if err := png.Encode(&small, thumb); err != nil {
		return nil, err
	}
	return &Image{full.Bytes(), small.Bytes(), w, h, hex.EncodeToString(hash.Sum(nil))}, nil
}

type limitedPNG struct{ bytes.Buffer }

func (b *limitedPNG) Write(p []byte) (int, error) {
	if b.Len()+len(p) > MaxPNGBytes {
		return 0, errors.New("图片编码后超过 20 MiB，未保存")
	}
	return b.Buffer.Write(p)
}

func thumbnail(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w <= ThumbnailSize && h <= ThumbnailSize {
		return src
	}
	tw, th := ThumbnailSize, max(1, h*ThumbnailSize/w)
	if h > w {
		th, tw = ThumbnailSize, max(1, w*ThumbnailSize/h)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, tw, th))
	// Area averaging in premultiplied space avoids dark fringes on transparent images.
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			var r, g, b, a, n uint64
			for sy := y * h / th; sy < (y+1)*h/th; sy++ {
				for sx := x * w / tw; sx < (x+1)*w/tw; sx++ {
					i := sy*src.Stride + sx*4
					alpha := uint64(src.Pix[i+3])
					n++
					r += uint64(src.Pix[i]) * alpha
					g += uint64(src.Pix[i+1]) * alpha
					b += uint64(src.Pix[i+2]) * alpha
					a += alpha
				}
			}
			i := y*dst.Stride + x*4
			if a != 0 {
				dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2] = byte(r/a), byte(g/a), byte(b/a)
			}
			dst.Pix[i+3] = byte(a / n)
		}
	}
	return dst
}
