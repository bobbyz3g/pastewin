package main

import (
	"log/slog"
	"time"

	"pastewin/internal/history"
)

func (s *ClipboardService) HistorySettings() (history.HistorySettings, error) {
	s.cleanupHistory()
	return s.store.HistorySettings()
}

func (s *ClipboardService) ExpiredCount(days int) (int, error) {
	return s.store.ExpiredCount(days)
}

func (s *ClipboardService) SetRetentionDays(days int) error {
	s.opMu.Lock()
	err := s.store.SetRetentionDays(days)
	s.opMu.Unlock()
	if err != nil {
		return err
	}
	s.changed()
	return nil
}

func (s *ClipboardService) cleanupHistory() {
	s.cleanupHistoryAt(time.Now())
}

func (s *ClipboardService) cleanupHistoryAt(now time.Time) {
	s.opMu.Lock()
	// Include the offset so a timezone change also triggers maintenance.
	date := now.Format("2006-01-02 -0700")
	if s.cleanupDate == date {
		s.opMu.Unlock()
		return
	}
	deleted, err := s.store.CleanupExpired()
	if err == nil {
		s.cleanupDate = date
	}
	s.opMu.Unlock()
	if err != nil {
		slog.Error("history_cleanup_failed", "error", err)
	} else if deleted > 0 {
		s.changed()
	}
}

func (s *ClipboardService) maintainHistory() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupHistory()
		}
	}
}
