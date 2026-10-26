package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"pastewin/internal/history"
	"pastewin/internal/native"
)

func TestPausedHistoryLoadCleansOldData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Add("old synthetic record"); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE clips SET updated_at=?`, time.Now().AddDate(0, 0, -40).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s := &ClipboardService{store: store, paused: true}
	page, err := s.List("", false, 0)
	if err != nil || page.All != 0 {
		t.Fatalf("paused load retained expired data: %+v, %v", page, err)
	}
}

func TestRetentionMaintenancePreservesPendingCapture(t *testing.T) {
	for _, action := range []string{"daily", "settings"} {
		t.Run(action, func(t *testing.T) {
			s := testService(t)
			job := captureJob{snapshot: native.Snapshot{Text: "pending capture"}, epoch: s.captureEpoch}
			if action == "daily" {
				s.cleanupHistory()
			} else if err := s.SetRetentionDays(7); err != nil {
				t.Fatal(err)
			}
			s.commitCapture(job, nil, nil)
			page, err := s.List("", false, 0)
			if err != nil || page.Total != 1 {
				t.Fatalf("maintenance discarded current capture: %+v, %v", page, err)
			}
		})
	}
}

func TestFailedMaintenanceCanRetrySameDay(t *testing.T) {
	s := testService(t)
	if err := s.store.Close(); err != nil {
		t.Fatal(err)
	}
	s.cleanupHistoryAt(time.Date(2026, 10, 26, 12, 0, 0, 0, time.Local))
	if s.cleanupDate != "" {
		t.Fatal("failed maintenance suppressed same-day retry")
	}
}
