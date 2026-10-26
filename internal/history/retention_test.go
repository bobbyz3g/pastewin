package history

import (
	"path/filepath"
	"testing"
	"time"
	_ "time/tzdata"
)

func TestRetentionCalendarBoundaryAndUnpin(t *testing.T) {
	s := newStore(t)
	now := time.Date(2026, 10, 26, 14, 30, 0, 0, time.FixedZone("local", 8*3600))
	s.now = func() time.Time { return now }
	for _, text := range []string{"expired", "boundary", "favorite"} {
		if err := s.Add(text); err != nil {
			t.Fatal(err)
		}
	}
	cutoff := retentionCutoff(now, 7)
	if _, err := s.db.Exec(`UPDATE clips SET updated_at=? WHERE text IN ('expired','favorite')`, cutoff-1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE clips SET updated_at=? WHERE text='boundary'`, cutoff); err != nil {
		t.Fatal(err)
	}
	var favorite int64
	if err := s.db.QueryRow(`SELECT id FROM clips WHERE text='favorite'`).Scan(&favorite); err != nil {
		t.Fatal(err)
	}
	if err := s.Pin(favorite, true); err != nil {
		t.Fatal(err)
	}
	count, err := s.ExpiredCount(7)
	if err != nil || count != 1 {
		t.Fatalf("preview: %d, %v", count, err)
	}
	if err := s.SetRetentionDays(7); err != nil {
		t.Fatal(err)
	}
	page, err := s.List("", false, 0)
	if err != nil || page.All != 2 || page.Pinned != 1 {
		t.Fatalf("cleanup: %+v, %v", page, err)
	}
	if err := s.Pin(favorite, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(favorite); err == nil {
		t.Fatal("expired favorite survived unpin")
	}
}

func TestRetentionDuplicateRefreshAndUsage(t *testing.T) {
	s := newStore(t)
	now := time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	img := testImage(t, 30)
	text := "中文\x00x"
	if err := s.AddContent(text, img, `C:\Apps\sample.exe`); err != nil {
		t.Fatal(err)
	}
	page, _ := s.List("", false, 0)
	id := page.Entries[0].ID
	now = now.AddDate(0, 0, 31)
	if err := s.AddContent(text, img, `C:\Apps\sample.exe`); err != nil {
		t.Fatal(err)
	}
	if count, err := s.CleanupExpired(); err != nil || count != 0 {
		t.Fatalf("refreshed record removed: %d %v", count, err)
	}
	if err := s.Pin(id, true); err != nil {
		t.Fatal(err)
	}
	settings, err := s.HistorySettings()
	want := int64(len(text) + len(img.PNG) + len(img.Thumbnail))
	if err != nil || settings.UsedBytes != want || settings.RetentionDays != 30 {
		t.Fatalf("usage: %+v, %v; want %d", settings, err, want)
	}
	if err := s.Pin(id, false); err != nil {
		t.Fatal(err)
	}
	now = now.AddDate(0, 0, 31)
	if count, err := s.CleanupExpired(); err != nil || count != 1 {
		t.Fatalf("cleanup: %d %v", count, err)
	}
	settings, err = s.HistorySettings()
	if err != nil || settings.UsedBytes != 0 {
		t.Fatalf("usage after cleanup: %+v %v", settings, err)
	}
	var sources int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM app_sources`).Scan(&sources); err != nil || sources != 0 {
		t.Fatalf("orphan sources: %d %v", sources, err)
	}
}

func TestRetentionPersistenceAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetRetentionDays(90); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRetentionDays(0); err == nil {
		t.Fatal("invalid policy accepted")
	}
	if _, err := s.ExpiredCount(365); err == nil {
		t.Fatal("invalid preview accepted")
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	settings, err := s.HistorySettings()
	if err != nil || settings.RetentionDays != 90 {
		t.Fatalf("saved policy: %+v %v", settings, err)
	}
}

func TestRetentionCutoffUsesCalendarDays(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, now := range []time.Time{
		time.Date(2026, 3, 10, 15, 0, 0, 0, location),
		time.Date(2026, 11, 3, 15, 0, 0, 0, location),
		time.Date(2027, 1, 2, 15, 0, 0, 0, location),
	} {
		cutoff := time.UnixMilli(retentionCutoff(now, 7)).In(location)
		wantDate := now.AddDate(0, 0, -6).Format("2006-01-02")
		if cutoff.Format("2006-01-02") != wantDate || cutoff.Hour() != 0 || cutoff.Minute() != 0 {
			t.Fatalf("bad local cutoff: %v", cutoff)
		}
	}
}
