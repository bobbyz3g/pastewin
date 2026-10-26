package history

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestDuplicatePreservesPinAndExactText(t *testing.T) {
	s := newStore(t)
	text := "  中文剪贴板\nsecond line  "
	if err := s.Add(text); err != nil {
		t.Fatal(err)
	}
	p, _ := s.List("", false, 0)
	id := p.Entries[0].ID
	if err := s.Pin(id, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(text); err != nil {
		t.Fatal(err)
	}
	p, _ = s.List("剪贴板", true, 0)
	if p.Total != 1 || p.All != 1 {
		t.Fatalf("unexpected counts: %+v", p)
	}
	e, _ := s.Get(id)
	if e.Text != text || !e.Pinned {
		t.Fatalf("lost text or pin: %+v", e)
	}
}
func TestRetentionClearAndLiteralSearch(t *testing.T) {
	s := newStore(t)
	_ = s.Add("pinned %_ text")
	p, _ := s.List("", false, 0)
	id := p.Entries[0].ID
	_ = s.Pin(id, true)
	for i := 0; i < 508; i++ {
		if err := s.Add(fmt.Sprintf("item %04d", i)); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.List("", false, 0)
	if err != nil || p.All != 509 || len(p.Entries) != 50 {
		t.Fatalf("retention/page: %+v %v", p, err)
	}
	p, _ = s.List("%_", false, 0)
	if p.Total != 1 {
		t.Fatal("search should be literal")
	}
	if err := s.ClearUnpinned(); err != nil {
		t.Fatal(err)
	}
	p, _ = s.List("", false, 0)
	if p.All != 1 || p.Entries[0].ID != id {
		t.Fatal("clear removed a pin")
	}
}
func TestPersistenceAndLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Add("\n\t ")
	if err = s.Add(strings.Repeat("x", MaxTextBytes+1)); err == nil {
		t.Fatal("large text accepted")
	}
	_ = s.Add("saved")
	_ = s.SetPaused(true)
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, _ := s.List("", false, 0)
	if p.All != 1 || !s.Paused() {
		t.Fatal("state did not survive restart")
	}
}
