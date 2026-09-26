package main

import (
	"context"
	"image"
	"image/color"
	"os"
	"testing"
	"time"

	"pastewin/internal/clipimage"
	"pastewin/internal/native"
)

func TestImageCaptureAndCopyIntegration(t *testing.T) {
	if os.Getenv("PASTEWIN_TEST_CLIPBOARD") != "1" {
		t.Skip("explicit native clipboard test")
	}
	s := testService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m, err := native.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); m.Wait() }()
	s.monitor = m
	s.captures = make(chan captureJob, 1)
	im := image.NewNRGBA(image.Rect(0, 0, 320, 180))
	im.SetNRGBA(3, 2, color.NRGBA{200, 30, 80, 128})
	img, err := clipimage.Normalize(im)
	if err != nil {
		t.Fatal(err)
	}
	dib, err := clipimage.EncodeDIBV5(im)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.WriteImage(img.PNG, dib); err != nil {
		t.Fatal(err)
	}
	select {
	case <-m.Changes:
	case <-time.After(3 * time.Second):
		t.Fatal("clipboard listener did not receive image event")
	}
	s.capture()
	var job captureJob
	select {
	case job = <-s.captures:
	default:
		t.Fatal("image capture did not enqueue snapshot")
	}
	prepared, err := clipimage.Prepare(job.snapshot.Images)
	if err != nil {
		t.Fatal(err)
	}
	s.commitCapture(job, prepared, nil)
	p, err := s.List("", false, 0)
	if err != nil || p.Total != 1 || !p.Entries[0].HasImage {
		t.Fatalf("capture not persisted: %+v %v", p, err)
	}
	id := p.Entries[0].ID
	if err = s.Copy(id); err != nil {
		t.Fatal(err)
	}
	s.capture()
	select {
	case <-s.captures:
		t.Fatal("own image write was captured again")
	default:
	}
	snapshot, err := m.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	round, err := clipimage.Prepare(snapshot.Images)
	if err != nil || round.Hash != img.Hash {
		t.Fatalf("copy lost pixels: %v", err)
	}
	if err = s.store.AddContent("  混合内容\n第二行", img); err != nil {
		t.Fatal(err)
	}
	p, err = s.List("混合内容", false, 0)
	if err != nil || p.Total != 1 {
		t.Fatal("mixed record missing")
	}
	id = p.Entries[0].ID
	if err = s.Copy(id); err != nil {
		t.Fatal(err)
	}
	snapshot, err = m.ReadSnapshot()
	if err != nil || snapshot.Text != "  混合内容\n第二行" || len(snapshot.Images) != 0 {
		t.Fatal("mixed default did not copy text only")
	}
	if err = s.CopyImage(id); err != nil {
		t.Fatal(err)
	}
	snapshot, err = m.ReadSnapshot()
	if err != nil || snapshot.Text != "" || len(snapshot.Images) == 0 {
		t.Fatal("mixed image action did not copy image only")
	}
	if _, err = m.WriteText("PasteWin 图片服务测试完成"); err != nil {
		t.Fatal(err)
	}
}
