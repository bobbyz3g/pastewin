package history

import (
	"database/sql"
	"errors"
	"strconv"
	"time"
)

const DefaultRetentionDays = 30

// HistorySettings reports the retention policy and logical payload size,
// including favorites, UTF-8 text, original images and thumbnails.
type HistorySettings struct {
	RetentionDays int   `json:"retentionDays"`
	UsedBytes     int64 `json:"usedBytes"`
}

type rowQuerier interface {
	QueryRow(string, ...any) *sql.Row
}

func validRetentionDays(days int) bool { return days == 7 || days == 30 || days == 90 }

func retentionDays(db rowQuerier) (int, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key='retention_days'`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultRetentionDays, nil
	}
	if err != nil {
		return 0, err
	}
	days, err := strconv.Atoi(value)
	if err != nil || !validRetentionDays(days) {
		return DefaultRetentionDays, nil
	}
	return days, nil
}

func retentionCutoff(now time.Time, days int) int64 {
	// Calendar arithmetic preserves local midnight across daylight-saving changes.
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1-days).UnixMilli()
}

func (s *Store) HistorySettings() (HistorySettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	days, err := retentionDays(s.db)
	if err != nil {
		return HistorySettings{}, err
	}
	result := HistorySettings{RetentionDays: days}
	err = s.db.QueryRow(`SELECT COALESCE(SUM(length(CAST(text AS BLOB))),0) + (SELECT COALESCE(SUM(byte_size),0) FROM clip_images) FROM clips`).Scan(&result.UsedBytes)
	return result, err
}

// ExpiredCount previews how many unpinned records the proposed policy removes.
func (s *Store) ExpiredCount(days int) (int, error) {
	if !validRetentionDays(days) {
		return 0, errors.New("无效的保留期限")
	}
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM clips WHERE pinned=0 AND updated_at<?`, retentionCutoff(s.now(), days)).Scan(&count)
	return count, err
}

// SetRetentionDays atomically saves the policy and removes expired records.
func (s *Store) SetRetentionDays(days int) error {
	if !validRetentionDays(days) {
		return errors.New("无效的保留期限")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO settings(key,value) VALUES('retention_days',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, strconv.Itoa(days)); err != nil {
		return err
	}
	if _, err = cleanupExpired(tx, s.now()); err != nil {
		return err
	}
	return tx.Commit()
}

func cleanupExpired(tx *sql.Tx, now time.Time) (int64, error) {
	days, err := retentionDays(tx)
	if err != nil {
		return 0, err
	}
	result, err := tx.Exec(`DELETE FROM clips WHERE pinned=0 AND updated_at<?`, retentionCutoff(now, days))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupExpired removes records older than the configured local calendar range.
func (s *Store) CleanupExpired() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	count, err := cleanupExpired(tx, s.now())
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}
