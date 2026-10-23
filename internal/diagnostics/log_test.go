package diagnostics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRotationAppendAndRetention(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.Local)
	for _, name := range []string{"pastewin-2026-10-16-8-0.jsonl", "pastewin-2026-10-17-8-0.jsonl", "other-2026-10-01.log"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r := &rollingFile{dir: dir, pid: 42, limit: 10}
	defer r.close()
	for _, data := range []string{"first\n", "next\n"} {
		if err := r.write(now, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	r.close()
	reopened := &rollingFile{dir: dir, pid: 42, limit: 10}
	defer reopened.close()
	if err := reopened.write(now, []byte("end\n")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "pastewin-2026-10-23-42-1.jsonl"))
	if err != nil || string(data) != "next\nend\n" {
		t.Fatalf("append lost records: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pastewin-2026-10-17-8-0.jsonl")); err != nil {
		t.Fatal("removed seventh calendar day", err)
	}
	if err := reopened.write(now.AddDate(0, 0, 1), []byte("day\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pastewin-2026-10-16-8-0.jsonl")); !os.IsNotExist(err) {
		t.Fatal("expired file retained")
	}
	if _, err := os.Stat(filepath.Join(dir, "other-2026-10-01.log")); err != nil {
		t.Fatal("unrelated file removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "pastewin-2026-10-24-42-0.jsonl")); err != nil {
		t.Fatal("day did not rotate")
	}
}

func TestAsyncCloseDrainsAndReportsOverflow(t *testing.T) {
	l, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l.Logger().Info("app_start", "pid", 42)
	// Oversize rejection deterministically exercises the drop summary without sleeps.
	l.Write(make([]byte, 17<<10))
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { l.Logger().Info("event", "identified", false) })
	}
	wg.Wait()
	l.Close()
	l.Close()
	files, err := filepath.Glob(filepath.Join(l.sink.dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	dropped := false
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatal(err)
			}
			if row["msg"] == "log_records_dropped" {
				dropped = true
			} else {
				count++
			}
		}
	}
	if count != 21 || !dropped {
		t.Fatalf("drain count=%d dropped=%v", count, dropped)
	}
	if _, err := l.Write([]byte("late")); err == nil {
		t.Fatal("accepted after close")
	}
}
