package main

import (
	"pastewin/internal/native"
	"testing"
)

func TestConsecutiveCopiesUpdateSourceImmediately(t *testing.T) {
	s := testService(t)
	capture := func(source string) {
		s.commitCapture(captureJob{snapshot: native.Snapshot{Text: "fixture", Source: source}, epoch: s.captureEpoch}, nil, nil)
	}
	capture(`C:\Apps\clash-verge.exe`)
	p, _ := s.store.List("", false, 0)
	id := p.Entries[0].ID
	s.store.Pin(id, true)
	for _, source := range []string{`C:\Apps\dbx.exe`, `C:\Apps\DingTalk.exe`, ""} {
		capture(source)
		e, err := s.store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if !e.Pinned {
			t.Fatal("lost pin")
		}
		if source == "" {
			if e.SourceKey != "" {
				t.Fatal("unknown source retained previous icon")
			}
			continue
		}
		got, err := s.store.SourcePath(e.SourceKey)
		if err != nil || got != source {
			t.Fatalf("source not updated: %q %v", got, err)
		}
	}
}
