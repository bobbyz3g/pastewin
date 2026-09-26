package main

import (
	"context"
	"image"
	"path/filepath"
	"testing"

	"pastewin/internal/clipimage"
	"pastewin/internal/history"
	"pastewin/internal/native"
)

func testService(t *testing.T) *ClipboardService {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return &ClipboardService{store: store, ctx: context.Background()}
}

func TestPendingCaptureInvalidatedByPauseClearAndShutdown(t *testing.T) {
	for _, action := range []string{"pause-resume", "clear", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			s := testService(t)
			job := captureJob{snapshot: native.Snapshot{Text: "pending"}, epoch: s.captureEpoch}
			img, err := clipimage.Normalize(image.NewNRGBA(image.Rect(0, 0, 3, 3)))
			if err != nil {
				t.Fatal(err)
			}
			// The job was read before this action and completes decoding afterwards.
			switch action {
			case "pause-resume":
				if err := s.SetPaused(true); err != nil {
					t.Fatal(err)
				}
				if err := s.SetPaused(false); err != nil {
					t.Fatal(err)
				}
			case "clear":
				if err := s.ClearUnpinned(); err != nil {
					t.Fatal(err)
				}
			case "shutdown":
				ctx, cancel := context.WithCancel(context.Background())
				s.ctx = ctx
				cancel()
			}
			s.commitCapture(job, img, nil)
			p, err := s.store.List("", false, 0)
			if err != nil || p.All != 0 {
				t.Fatalf("stale capture committed: %+v %v", p, err)
			}
		})
	}
}

func TestMalformedImageStillPreservesText(t *testing.T) {
	s := testService(t)
	job := captureJob{snapshot: native.Snapshot{Text: "  keep text\n", Images: []clipimage.Encoded{{Format: "PNG", Data: []byte("broken")}}}}
	img, err := clipimage.Prepare(job.snapshot.Images)
	if err == nil {
		t.Fatal("expected decode failure")
	}
	s.commitCapture(job, img, err)
	p, err := s.store.List("keep", false, 0)
	if err != nil || p.Total != 1 {
		t.Fatalf("text fallback lost: %+v %v", p, err)
	}
	e, _ := s.store.Get(p.Entries[0].ID)
	if e.Text != job.snapshot.Text || e.HasImage {
		t.Fatal("fallback changed text")
	}
}
