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

	"pastewin/internal/clipimage"

	_ "modernc.org/sqlite"
)

const MaxTextBytes = 1 << 20
const MaxEntries = 500
const MaxImageBytes = 256 << 20

type Entry struct {
	ID          int64  `json:"id"`
	Text        string `json:"text"`
	Preview     string `json:"preview"`
	Characters  int    `json:"characters"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	Pinned      bool   `json:"pinned"`
	HasImage    bool   `json:"hasImage"`
	ImageWidth  int    `json:"imageWidth"`
	ImageHeight int    `json:"imageHeight"`
	ImageBytes  int    `json:"imageBytes"`
}

type Page struct {
	Entries []Entry `json:"entries"`
	Total   int     `json:"total"`
	All     int     `json:"all"`
	Pinned  int     `json:"pinned"`
}

type Store struct {
	db         *sql.DB
	mu         sync.Mutex
	imageLimit int64
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;
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
	if err = migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, imageLimit: MaxImageBytes}, nil
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return errors.New("历史数据库由更新版本创建，请升级 PasteWin")
	}
	if version == 1 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE clip_images (
	 clip_id INTEGER PRIMARY KEY REFERENCES clips(id) ON DELETE CASCADE,
	 png BLOB NOT NULL, thumbnail BLOB NOT NULL,
	 width INTEGER NOT NULL, height INTEGER NOT NULL, byte_size INTEGER NOT NULL);
	 PRAGMA user_version=1;`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

// Exact text is kept, including whitespace; the hash only coalesces exact duplicates.
func (s *Store) Add(text string) error {
	return s.AddContent(text, nil)
}

func (s *Store) AddContent(text string, img *clipimage.Image) error {
	if strings.TrimSpace(text) == "" {
		if img == nil {
			return nil
		}
		text = ""
	}
	if len(text) > MaxTextBytes {
		return errors.New("文本超过 1 MB，未保存")
	}
	if img != nil {
		if err := clipimage.CheckDimensions(img.Width, img.Height); err != nil {
			return err
		}
		if len(img.PNG) == 0 || len(img.PNG) > clipimage.MaxPNGBytes || len(img.Thumbnail) == 0 || len(img.Thumbnail) > 1<<20 || len(img.Hash) != 64 {
			return errors.New("图片数据无效或过大")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	hash := sha256.Sum256([]byte(text))
	key := hex.EncodeToString(hash[:])
	if img != nil {
		key = "image:" + img.Hash + ":" + key
	}
	now := time.Now().UnixMilli()
	_, err = tx.Exec(`INSERT INTO clips(hash,text,characters,created_at,updated_at) VALUES(?,?,?,?,?)
	ON CONFLICT(hash) DO UPDATE SET updated_at=excluded.updated_at`, key, text, utf8.RuneCountInString(text), now, now)
	if err != nil {
		return err
	}
	var id int64
	if err = tx.QueryRow(`SELECT id FROM clips WHERE hash=?`, key).Scan(&id); err != nil {
		return err
	}
	if img != nil {
		_, err = tx.Exec(`INSERT INTO clip_images(clip_id,png,thumbnail,width,height,byte_size) VALUES(?,?,?,?,?,?) ON CONFLICT(clip_id) DO NOTHING`, id, img.PNG, img.Thumbnail, img.Width, img.Height, len(img.PNG)+len(img.Thumbnail))
		if err != nil {
			return err
		}
		var total int64
		if err = tx.QueryRow(`SELECT COALESCE(SUM(byte_size),0) FROM clip_images`).Scan(&total); err != nil {
			return err
		}
		for total > s.imageLimit {
			var victim int64
			var size int64
			err = tx.QueryRow(`SELECT c.id,i.byte_size FROM clips c JOIN clip_images i ON i.clip_id=c.id WHERE c.pinned=0 AND c.id<>? ORDER BY c.updated_at,c.id LIMIT 1`, id).Scan(&victim, &size)
			if errors.Is(err, sql.ErrNoRows) {
				return errors.New("图片空间已满，请取消收藏或删除部分图片后重试")
			}
			if err != nil {
				return err
			}
			if _, err = tx.Exec(`DELETE FROM clips WHERE id=?`, victim); err != nil {
				return err
			}
			total -= size
		}
	}
	// Pins count toward the same hard cap; only unpinned items may be evicted.
	_, err = tx.Exec(`DELETE FROM clips WHERE id IN (SELECT id FROM clips WHERE pinned=0 ORDER BY updated_at ASC,id ASC LIMIT MAX(0,(SELECT COUNT(*) FROM clips)-?))`, MaxEntries)
	if err != nil {
		return err
	}
	if img != nil {
		var kept int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM clips WHERE id=?`, id).Scan(&kept); err != nil {
			return err
		}
		if kept == 0 {
			return errors.New("历史已满且均已收藏，图片未保存")
		}
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
	rows, err := s.db.Query(`SELECT id,substr(text,1,180),characters,created_at,updated_at,pinned,
	 i.clip_id IS NOT NULL,COALESCE(i.width,0),COALESCE(i.height,0),COALESCE(i.byte_size,0)
	 FROM clips LEFT JOIN clip_images i ON i.clip_id=clips.id`+where+` ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.Preview, &entry.Characters, &entry.CreatedAt, &entry.UpdatedAt, &entry.Pinned, &entry.HasImage, &entry.ImageWidth, &entry.ImageHeight, &entry.ImageBytes); err != nil {
			return page, err
		}
		page.Entries = append(page.Entries, entry)
	}
	return page, rows.Err()
}

func (s *Store) Get(id int64) (Entry, error) {
	var e Entry
	err := s.db.QueryRow(`SELECT id,text,characters,created_at,updated_at,pinned,
	 i.clip_id IS NOT NULL,COALESCE(i.width,0),COALESCE(i.height,0),COALESCE(i.byte_size,0)
	 FROM clips LEFT JOIN clip_images i ON i.clip_id=clips.id WHERE id=?`, id).Scan(&e.ID, &e.Text, &e.Characters, &e.CreatedAt, &e.UpdatedAt, &e.Pinned, &e.HasImage, &e.ImageWidth, &e.ImageHeight, &e.ImageBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return e, errors.New("这条记录已被删除")
	}
	return e, err
}

func (s *Store) Image(id int64, thumbnail bool) ([]byte, error) {
	column := "png"
	if thumbnail {
		column = "thumbnail"
	}
	var data []byte
	err := s.db.QueryRow(`SELECT `+column+` FROM clip_images WHERE clip_id=?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("这张图片已被删除")
	}
	return data, err
}

func (s *Store) Pin(id int64, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE clips SET pinned=? WHERE id=?`, pinned, id)
	return err
}
func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM clips WHERE id=?`, id)
	return err
}
func (s *Store) ClearUnpinned() error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// Language returns the saved UI language. Older databases default to Chinese.
func (s *Store) Language() (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key='language'`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "zh-CN", nil
	}
	if err != nil {
		return "", err
	}
	if value != "en" && value != "zh-CN" {
		return "zh-CN", nil
	}
	return value, nil
}
func (s *Store) SetLanguage(language string) error {
	if language != "en" && language != "zh-CN" {
		return errors.New("Unsupported language")
	}
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES('language',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, language)
	return err
}
