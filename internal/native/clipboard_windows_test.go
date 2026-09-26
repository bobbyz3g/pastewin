package native

import (
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"testing"

	"pastewin/internal/clipimage"

	"golang.org/x/sys/windows"
)

// This integration test writes only synthetic content, but changes the system
// clipboard. It is explicitly opt-in and never runs with normal go test ./....
func TestImageClipboardIntegration(t *testing.T) {
	if os.Getenv("PASTEWIN_TEST_CLIPBOARD") != "1" {
		t.Skip("set PASTEWIN_TEST_CLIPBOARD=1 for native clipboard integration")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m, err := Watch(ctx)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer func() { cancel(); m.Wait() }()
	pixels := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	pixels.SetNRGBA(0, 0, color.NRGBA{255, 30, 40, 255})
	pixels.SetNRGBA(2, 1, color.NRGBA{30, 200, 70, 128})
	img, err := clipimage.Normalize(pixels)
	if err != nil {
		t.Fatal(err)
	}
	dib, err := clipimage.EncodeDIBV5(pixels)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := m.WriteImage(img.PNG, dib)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := m.ReadSnapshot()
	if err != nil || snapshot.Sequence != seq || snapshot.Text != "" {
		t.Fatalf("image snapshot metadata failed: written sequence=%d, read sequence=%d, text bytes=%d, err=%v", seq, snapshot.Sequence, len(snapshot.Text), err)
	}
	round, err := clipimage.Prepare(snapshot.Images)
	if err != nil || round.Hash != img.Hash {
		t.Fatalf("native PNG/DIB roundtrip changed image: %v", err)
	}
	u16, _ := windows.UTF16FromString("  中文测试\n第二行")
	text := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(text[i*2:], v)
	}
	if _, err = m.writeFormats([]clipboardFormat{{13, text}, {format("PNG"), []byte("bad png")}, {17, dib}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = m.ReadSnapshot()
	if err != nil || snapshot.Text != "  中文测试\n第二行" {
		t.Fatal("mixed snapshot lost text")
	}
	round, err = clipimage.Prepare(snapshot.Images)
	if err != nil || round.Hash != img.Hash {
		t.Fatalf("native DIB fallback failed: %v", err)
	}
	for _, name := range []string{"ExcludeClipboardContentFromMonitorProcessing", "CanIncludeInClipboardHistory"} {
		if _, err = m.writeFormats([]clipboardFormat{{format("PNG"), img.PNG}, {format(name), []byte{0, 0, 0, 0}}}); err != nil {
			t.Fatal(err)
		}
		snapshot, err = m.ReadSnapshot()
		if err != nil || snapshot.Text != "" || len(snapshot.Images) != 0 {
			t.Fatal("history opt-out was ignored")
		}
	}
	if _, err = m.WriteText("PasteWin 图片回归测试完成"); err != nil {
		t.Fatal(err)
	}
	// Format zero is invalid. The first successful write must remain owned by
	// Windows, and the error still needs a sequence for self-write suppression.
	seq, err = m.writeFormats([]clipboardFormat{{13, text}, {0, []byte{1}}})
	if err == nil || seq == 0 {
		t.Fatal("partial write did not return error and sequence")
	}
	snapshot, err = m.ReadSnapshot()
	if err != nil || snapshot.Sequence != seq || snapshot.Text != "  中文测试\n第二行" {
		t.Fatal("partial write freed transferred data or lost final sequence")
	}
}
