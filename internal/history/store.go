package history

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

const MaxTextBytes = 1 << 20
const MaxEntries = 500

type Entry struct {
	ID         int64  `json:"id"`
	Text       string `json:"text"`
	Preview    string `json:"preview"`
	Characters int    `json:"characters"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
	Pinned     bool   `json:"pinned"`
}

type Page struct {
	Entries []Entry `json:"entries"`
	Total   int     `json:"total"`
	All     int     `json:"all"`
	Pinned  int     `json:"pinned"`
}

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
	CREATE TABLE IF NOT EXISTS clips (
	 id INTEGER PRIMARY KEY AUTOINCREMENT, hash TEXT NOT NULL UNIQUE, text TEXT NOT NULL,
	 characters INTEGER NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
	 pinned INTEGER NOT NULL DEFAULT 0);
	CREATE INDEX IF NOT EXISTS clips_recent ON clips(updated_at DESC);
	CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Exact text is kept, including whitespace; the hash only coalesces exact duplicates.
func (s *Store) Add(text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if len(text) > MaxTextBytes {
		return errors.New("文本超过 1 MB，未保存")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	hash := sha256.Sum256([]byte(text))
	now := time.Now().UnixMilli()
	_, err = tx.Exec(`INSERT INTO clips(hash,text,characters,created_at,updated_at) VALUES(?,?,?,?,?)
	ON CONFLICT(hash) DO UPDATE SET updated_at=excluded.updated_at`, hex.EncodeToString(hash[:]), text, utf8.RuneCountInString(text), now, now)
	if err != nil {
		return err
	}
	// Pins count toward the same hard cap; only unpinned items may be evicted.
	_, err = tx.Exec(`DELETE FROM clips WHERE id IN (SELECT id FROM clips WHERE pinned=0 ORDER BY updated_at ASC,id ASC LIMIT MAX(0,(SELECT COUNT(*) FROM clips)-?))`, MaxEntries)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) List(query string, pinned bool, offset int) (Page, error) {
	page := Page{Entries: []Entry{}}
	if offset < 0 {
		offset = 0
	}
	where := " WHERE instr(lower(text),lower(?))>0"
	args := []any{query}
	if pinned {
		where += " AND pinned=1"
	}
	if err := s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(pinned),0) FROM clips`).Scan(&page.All, &page.Pinned); err != nil {
		return page, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM clips`+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	args = append(args, 50, offset)
	rows, err := s.db.Query(`SELECT id,substr(text,1,180),characters,created_at,updated_at,pinned FROM clips`+where+` ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.Preview, &entry.Characters, &entry.CreatedAt, &entry.UpdatedAt, &entry.Pinned); err != nil {
			return page, err
		}
		page.Entries = append(page.Entries, entry)
	}
	return page, rows.Err()
}

func (s *Store) Get(id int64) (Entry, error) {
	var e Entry
	err := s.db.QueryRow(`SELECT id,text,characters,created_at,updated_at,pinned FROM clips WHERE id=?`, id).Scan(&e.ID, &e.Text, &e.Characters, &e.CreatedAt, &e.UpdatedAt, &e.Pinned)
	if errors.Is(err, sql.ErrNoRows) {
		return e, errors.New("这条记录已被删除")
	}
	return e, err
}

func (s *Store) Pin(id int64, pinned bool) error {
	_, err := s.db.Exec(`UPDATE clips SET pinned=? WHERE id=?`, pinned, id)
	return err
}
func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM clips WHERE id=?`, id)
	return err
}
func (s *Store) ClearUnpinned() error {
	_, err := s.db.Exec(`DELETE FROM clips WHERE pinned=0`)
	return err
}
func (s *Store) Paused() bool {
	var value string
	_ = s.db.QueryRow(`SELECT value FROM settings WHERE key='paused'`).Scan(&value)
	return value == "true"
}
func (s *Store) SetPaused(paused bool) error {
	value := "false"
	if paused {
		value = "true"
	}
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES('paused',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, value)
	return err
}
