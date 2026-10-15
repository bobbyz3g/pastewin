package history

import (
	"path/filepath"
	"testing"
)

func TestLanguagePersistsWithoutChangingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	if language, err := s.Language(); err != nil || language != "zh-CN" {
		t.Fatalf("default: %q, %v", language, err)
	}
	if err := s.Add(" 中文 original text "); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLanguage("unsupported"); err == nil {
		t.Fatal("accepted unsupported language")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if language, err := s.Language(); err != nil || language != "en" {
		t.Fatalf("reopened: %q, %v", language, err)
	}
	if !s.Paused() {
		t.Fatal("language change reset pause state")
	}
	page, err := s.List("", false, 0)
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("history changed: %+v %v", page, err)
	}
	entry, err := s.Get(page.Entries[0].ID)
	if err != nil || entry.Text != " 中文 original text " {
		t.Fatalf("text changed: %+v %v", entry, err)
	}
	if err := s.SetLanguage("zh-CN"); err != nil {
		t.Fatal(err)
	}
	if language, _ := s.Language(); language != "zh-CN" {
		t.Fatal("cannot switch back")
	}
}

func TestUnknownStoredLanguageFallsBack(t *testing.T) {
	s := newStore(t)
	if _, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES('language','unknown')`); err != nil {
		t.Fatal(err)
	}
	if language, err := s.Language(); err != nil || language != "zh-CN" {
		t.Fatalf("fallback: %q %v", language, err)
	}
}
