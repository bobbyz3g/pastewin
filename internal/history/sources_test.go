package history

import (
	"path/filepath"
	"testing"
)

func TestSourcesFollowLatestCopyAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AddContent("sample", nil, `C:\Apps\first.exe`); err != nil {
		t.Fatal(err)
	}
	page, _ := s.List("", false, 0)
	e := page.Entries[0]
	first := e.SourceKey
	if e.SourceName != "first" || len(first) != 64 {
		t.Fatalf("bad source: %+v", e)
	}
	s.Pin(e.ID, true)
	if err = s.AddContent("sample", nil, `C:\Apps\second.exe`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e, err = s.Get(e.ID)
	if err != nil || !e.Pinned || e.SourceName != "second" || e.SourceKey == first {
		t.Fatalf("recopy lost metadata: %+v %v", e, err)
	}
	if _, err = s.SourcePath(first); err == nil {
		t.Fatal("unreferenced source exposed")
	}
	if err = s.AddContent("sample", nil); err != nil {
		t.Fatal(err)
	}
	e, _ = s.Get(e.ID)
	if e.SourceKey != "" || e.SourceName != "" {
		t.Fatal("unknown copy retained false attribution")
	}
	var count int
	s.db.QueryRow(`SELECT count(*) FROM app_sources`).Scan(&count)
	if count != 0 {
		t.Fatal("orphan sources", count)
	}
}

func TestVersionOneMigrationPreservesImages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AddContent("image fixture", testImage(t, 60)); err != nil {
		t.Fatal(err)
	}
	p, _ := s.List("", false, 0)
	id := p.Entries[0].ID
	s.Pin(id, true)
	s.SetPaused(true)
	if _, err = s.db.Exec(`DROP TRIGGER prune_source_delete; DROP TRIGGER prune_source_update; DROP INDEX clips_source; ALTER TABLE clips DROP COLUMN source_key; DROP TABLE app_sources; PRAGMA user_version=1`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e, err := s.Get(id)
	if err != nil || !e.HasImage || !e.Pinned || !s.Paused() || e.SourceKey != "" || e.Text != "image fixture" {
		t.Fatalf("migration: %+v %v", e, err)
	}
	if data, err := s.Image(id, false); err != nil || len(data) == 0 {
		t.Fatal("lost image", err)
	}
}
