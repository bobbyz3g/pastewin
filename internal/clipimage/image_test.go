package clipimage

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func sample() *image.NRGBA {
	im := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	colors := []color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 128}, {0, 0, 255, 255}, {0, 0, 0, 0}, {230, 120, 90, 64}, {255, 255, 255, 255}}
	for i, c := range colors {
		im.SetNRGBA(i%3, i/3, c)
	}
	return im
}

func TestTransparentRoundTripAndEncodingIndependentHash(t *testing.T) {
	src := sample()
	dib, err := EncodeDIBV5(src)
	if err != nil {
		t.Fatal(err)
	}
	round, err := DecodeDIB(dib)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src.Pix, round.Pix) {
		t.Fatal("DIB roundtrip changed colour, orientation or transparency")
	}
	var hashes []string
	for _, level := range []png.CompressionLevel{png.NoCompression, png.BestCompression} {
		var data bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: level}).Encode(&data, src); err != nil {
			t.Fatal(err)
		}
		prepared, err := Prepare([]Encoded{{"PNG", data.Bytes()}})
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, prepared.Hash)
		decoded, err := DecodePNG(prepared.PNG)
		if err != nil || !bytes.Equal(decoded.Pix, src.Pix) {
			t.Fatalf("PNG roundtrip: %v", err)
		}
	}
	prepared, err := Prepare([]Encoded{{"PNG", []byte("broken")}, {"DIB", dib}})
	if err != nil || prepared.Hash != hashes[0] || hashes[0] != hashes[1] {
		t.Fatalf("fallback/dedup failed: %v", err)
	}
}

func TestDIB24PaddingAndOrientation(t *testing.T) {
	for _, topDown := range []bool{false, true} {
		data := make([]byte, 40+24)
		binary.LittleEndian.PutUint32(data, 40)
		binary.LittleEndian.PutUint32(data[4:], 3)
		h := int32(2)
		if topDown {
			h = -h
		}
		binary.LittleEndian.PutUint32(data[8:], uint32(h))
		binary.LittleEndian.PutUint16(data[12:], 1)
		binary.LittleEndian.PutUint16(data[14:], 24)
		for row := 0; row < 2; row++ {
			for x := 0; x < 3; x++ {
				p := 40 + row*12 + x*3
				data[p], data[p+1], data[p+2] = byte(row+1), byte(x+10), 200
			}
		}
		decoded, err := DecodeDIB(data)
		if err != nil {
			t.Fatal(err)
		}
		wantB := byte(2)
		if topDown {
			wantB = 1
		}
		if c := decoded.NRGBAAt(2, 0); c != (color.NRGBA{200, 12, wantB, 255}) {
			t.Fatalf("unexpected pixel: %v", c)
		}
	}
}

func TestDIBPaletteAndBitfields(t *testing.T) {
	// A one-bit row with a two-entry BGRA colour table.
	data := make([]byte, 52)
	binary.LittleEndian.PutUint32(data, 40)
	binary.LittleEndian.PutUint32(data[4:], 2)
	binary.LittleEndian.PutUint32(data[8:], 1)
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 1)
	data[42] = 255
	data[45] = 255
	data[48] = 0x40
	pixels, err := DecodeDIB(data)
	if err != nil {
		t.Fatal(err)
	}
	if pixels.NRGBAAt(0, 0) != (color.NRGBA{255, 0, 0, 255}) || pixels.NRGBAAt(1, 0) != (color.NRGBA{0, 255, 0, 255}) {
		t.Fatal("palette decoded incorrectly")
	}
	// RGB565 with masks stored after a 40-byte header.
	data = make([]byte, 56)
	binary.LittleEndian.PutUint32(data, 40)
	binary.LittleEndian.PutUint32(data[4:], 1)
	binary.LittleEndian.PutUint32(data[8:], 1)
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 16)
	binary.LittleEndian.PutUint32(data[16:], 3)
	binary.LittleEndian.PutUint32(data[40:], 0xf800)
	binary.LittleEndian.PutUint32(data[44:], 0x07e0)
	binary.LittleEndian.PutUint32(data[48:], 0x001f)
	binary.LittleEndian.PutUint16(data[52:], 0x07e0)
	pixels, err = DecodeDIB(data)
	if err != nil {
		t.Fatal(err)
	}
	if pixels.NRGBAAt(0, 0) != (color.NRGBA{0, 255, 0, 255}) {
		t.Fatal("RGB565 decoded incorrectly")
	}
}

func TestRejectMalformedAndOversizedImages(t *testing.T) {
	valid, _ := EncodeDIBV5(sample())
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { return b[:39] },
		func(b []byte) []byte { return b[:len(b)-1] },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[4:], 0x7fffffff); return b },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[8:], 0x80000000); return b },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[32:], 0xffffffff); return b },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[44:], 0xff0000); return b },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[16:], 1); return b },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[112:], 0xffffffff); return b },
	} {
		if _, err := DecodeDIB(mutate(bytes.Clone(valid))); err == nil {
			t.Fatal("malformed DIB accepted")
		}
	}
	if err := CheckDimensions(5000, 5000); err == nil {
		t.Fatal("pixel limit not enforced")
	}
	if _, err := DecodePNG([]byte("not a png")); err == nil {
		t.Fatal("invalid PNG accepted")
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, sample()); err != nil {
		t.Fatal(err)
	}
	oversized := encoded.Bytes()
	// Keep a valid IHDR CRC so rejection is caused by the dimension guard,
	// before a decoder can allocate memory based on attacker-controlled sizes.
	binary.BigEndian.PutUint32(oversized[16:20], 16_000_001)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	if _, err := DecodePNG(oversized); err == nil || err.Error() != "图片尺寸无效或超过 1600 万像素，未保存" {
		t.Fatalf("PNG dimension guard: %v", err)
	}
}

func TestThumbnailPreservesTransparency(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 512, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 512; x++ {
			src.SetNRGBA(x, y, color.NRGBA{240, 100, 50, 128})
		}
	}
	prepared, err := Normalize(src)
	if err != nil {
		t.Fatal(err)
	}
	thumb, err := DecodePNG(prepared.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if thumb.Bounds().Dx() != 256 || thumb.Bounds().Dy() != 128 || thumb.NRGBAAt(20, 20) != (color.NRGBA{240, 100, 50, 128}) {
		t.Fatal("thumbnail lost aspect ratio or transparency")
	}
}

func FuzzDecodeDIB(f *testing.F) {
	valid, _ := EncodeDIBV5(sample())
	f.Add(valid)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = DecodeDIB(data) })
}
