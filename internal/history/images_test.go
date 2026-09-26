package history

import (
	"bytes"
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"pastewin/internal/clipimage"
)

func testImage(t *testing.T, red byte) *clipimage.Image {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	im.SetNRGBA(2, 1, color.NRGBA{red, 100, 200, 128})
	result, err := clipimage.Normalize(im)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestImagePersistenceDedupAndCascade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	img := testImage(t, 40)
	if err = s.AddContent("", img); err != nil {
		t.Fatal(err)
	}
	page, _ := s.List("", false, 0)
	id := page.Entries[0].ID
	if err = s.Pin(id, true); err != nil {
		t.Fatal(err)
	}
	if err = s.AddContent("", img); err != nil {
		t.Fatal(err)
	}
	if err = s.AddContent("  图片说明\n", img); err != nil {
		t.Fatal(err)
	}
	if err = s.Add("  图片说明\n"); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e, err := s.Get(id)
	if err != nil || !e.Pinned || !e.HasImage || e.ImageWidth != 8 || e.ImageHeight != 4 {
		t.Fatalf("metadata/pin lost: %+v %v", e, err)
	}
	full, err := s.Image(id, false)
	if err != nil || !bytes.Equal(full, img.PNG) {
		t.Fatal("image lost on restart")
	}
	thumb, err := s.Image(id, true)
	if err != nil || !bytes.Equal(thumb, img.Thumbnail) {
		t.Fatal("thumbnail lost on restart")
	}
	page, err = s.List("图片说明", false, 0)
	if err != nil || page.Total != 2 || page.All != 3 {
		t.Fatalf("mixed search/dedup: %+v %v", page, err)
	}
	for _, item := range page.Entries {
		if item.Text != "" {
			t.Fatal("list must not load full content")
		}
	}
	if err = s.ClearUnpinned(); err != nil {
		t.Fatal(err)
	}
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM clip_images`).Scan(&count)
	if count != 1 {
		t.Fatal("clear left image payloads or deleted pin")
	}
	if err = s.Delete(id); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM clip_images`).Scan(&count)
	if count != 0 {
		t.Fatal("delete left image payload")
	}
	if _, err = s.Image(id, false); err == nil {
		t.Fatal("deleted image still readable")
	}
}

func TestImageQuotaEvictsOnlyUnpinnedAndRollsBack(t *testing.T) {
	s := newStore(t)
	a, b, c := testImage(t, 10), testImage(t, 20), testImage(t, 30)
	s.imageLimit = int64(len(a.PNG) + len(a.Thumbnail) + len(c.PNG) + len(c.Thumbnail))
	if err := s.AddContent("pinned", a); err != nil {
		t.Fatal(err)
	}
	p, _ := s.List("", false, 0)
	pin := p.Entries[0].ID
	s.Pin(pin, true)
	if err := s.AddContent("old", b); err != nil {
		t.Fatal(err)
	}
	if err := s.AddContent("new", c); err != nil {
		t.Fatal(err)
	}
	p, _ = s.List("old", false, 0)
	if p.Total != 0 {
		t.Fatal("old unpinned image not evicted")
	}
	p, _ = s.List("new", false, 0)
	if p.Total != 1 {
		t.Fatal("new image not saved")
	}
	s.Pin(p.Entries[0].ID, true)
	if err := s.AddContent("blocked", b); err == nil {
		t.Fatal("quota ignored pins")
	}
	p, _ = s.List("", false, 0)
	if p.All != 2 || p.Pinned != 2 {
		t.Fatal("failed insertion changed history")
	}
	// Text is not charged to the image quota.
	if err := s.Add("plain text"); err != nil {
		t.Fatal(err)
	}
}

func TestOldDatabaseMigrationPreservesTextAndSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(" 旧数据\n")
	p, _ := s.List("", false, 0)
	id := p.Entries[0].ID
	s.Pin(id, true)
	s.SetPaused(true)
	// Version zero has exactly the original clips/settings schema.
	if _, err = s.db.Exec(`DROP TABLE clip_images;PRAGMA user_version=0`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e, err := s.Get(id)
	if err != nil || e.Text != " 旧数据\n" || !e.Pinned || e.HasImage || !s.Paused() {
		t.Fatalf("migration lost data: %+v %v", e, err)
	}
	if err = s.AddContent("", testImage(t, 60)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`PRAGMA user_version=2`); err != nil {
		t.Fatal(err)
	}
	if newer, err := Open(path); err == nil {
		newer.Close()
		t.Fatal("accepted future database version")
	}
}

func TestEntryLimitCleansImagePayloads(t *testing.T) {
	s := newStore(t)
	if err := s.AddContent("", testImage(t, 50)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxEntries; i++ {
		if err := s.Add(string(rune(0x4e00 + i))); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM clip_images`).Scan(&count)
	if count != 0 {
		t.Fatal("entry eviction orphaned image data")
	}
	if _, err := s.db.Exec(`UPDATE clips SET pinned=1`); err != nil {
		t.Fatal(err)
	}
	if err := s.AddContent("", testImage(t, 70)); err == nil {
		t.Fatal("full pinned history silently accepted image")
	}
	p, _ := s.List("", false, 0)
	if p.All != MaxEntries || p.Pinned != MaxEntries {
		t.Fatal("pins lost")
	}
}
