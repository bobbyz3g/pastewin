// Package diagnostics provides bounded asynchronous local diagnostic logging.
package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const maxFileBytes = 10 << 20

// Log drops excess records instead of blocking application work. Close drains
// accepted records; callers must stop producing diagnostics before closing it.
type Log struct {
	mu      sync.RWMutex
	closed  bool
	queue   chan []byte
	done    chan struct{}
	dropped atomic.Uint64
	sink    *rollingFile
}

func Open(dir string) (*Log, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	sink := &rollingFile{dir: dir, pid: os.Getpid(), limit: maxFileBytes}
	now := time.Now()
	if err := sink.rotate(now); err != nil {
		return nil, err
	}
	l := &Log{queue: make(chan []byte, 256), done: make(chan struct{}), sink: sink}
	go l.run()
	return l, nil
}

func (l *Log) Logger() *slog.Logger { return slog.New(slog.NewJSONHandler(l, nil)) }
func (l *Log) Write(p []byte) (int, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.closed {
		return 0, os.ErrClosed
	}
	if len(p) > 16<<10 {
		l.dropped.Add(1)
		return len(p), nil
	}
	select {
	case l.queue <- append([]byte(nil), p...):
	default:
		l.dropped.Add(1)
	}
	return len(p), nil
}
func (l *Log) Close() error {
	l.mu.Lock()
	if !l.closed {
		l.closed = true
		close(l.queue)
	}
	l.mu.Unlock()
	<-l.done
	return nil
}
func (l *Log) run() {
	defer close(l.done)
	defer l.sink.close()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	// Report storage failures to stderr once until a successful write. Never
	// recursively log through this writer or terminate clipboard capture.
	failed := false
	write := func(p []byte) {
		if err := l.sink.write(time.Now(), p); err != nil {
			if !failed {
				fmt.Fprintln(os.Stderr, "PasteWin log write failed:", err)
			}
			failed = true
		} else {
			failed = false
		}
	}
	dropped := func() {
		if n := l.dropped.Swap(0); n > 0 {
			p, _ := json.Marshal(map[string]any{"time": time.Now().Format(time.RFC3339Nano), "level": "WARN", "msg": "log_records_dropped", "count": n})
			write(append(p, '\n'))
		}
	}
	for {
		select {
		case p, ok := <-l.queue:
			dropped()
			if !ok {
				return
			}
			write(p)
		case now := <-ticker.C:
			if l.sink.day != now.Format("2006-01-02") {
				if err := l.sink.rotate(now); err != nil {
					fmt.Fprintln(os.Stderr, "PasteWin log rotation failed:", err)
				}
			}
			if err := l.sink.cleanup(now); err != nil {
				fmt.Fprintln(os.Stderr, "PasteWin log cleanup failed:", err)
			}
		}
	}
}

type rollingFile struct {
	dir   string
	pid   int
	limit int64
	file  *os.File
	day   string
	part  int
	size  int64
}

var logName = regexp.MustCompile(`^pastewin-(\d{4}-\d{2}-\d{2})-(\d+)-(\d+)\.jsonl$`)

func (r *rollingFile) close() {
	if r.file != nil {
		r.file.Close()
		r.file = nil
	}
}
func (r *rollingFile) cleanup(now time.Time) error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return err
	}
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -6)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := logName.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		date, err := time.ParseInLocation("2006-01-02", m[1], now.Location())
		if err != nil || !date.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(r.dir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
func (r *rollingFile) rotate(now time.Time) error {
	r.close()
	day := now.Format("2006-01-02")
	if day != r.day {
		r.day = day
		r.part = 0
	}
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		m := logName.FindStringSubmatch(entry.Name())
		if entry.IsDir() || m == nil || m[1] != day || m[2] != strconv.Itoa(r.pid) {
			continue
		}
		part, err := strconv.Atoi(m[3])
		if err == nil && part > r.part {
			r.part = part
		}
	}
	// Continue the current process's last volume if reopened, never truncate.
	for {
		name := filepath.Join(r.dir, "pastewin-"+r.day+"-"+strconv.Itoa(r.pid)+"-"+strconv.Itoa(r.part)+".jsonl")
		f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		if stat.Size() >= r.limit {
			f.Close()
			r.part++
			continue
		}
		r.file = f
		r.size = stat.Size()
		break
	}
	if err := r.cleanup(now); err != nil {
		fmt.Fprintln(os.Stderr, "PasteWin log cleanup failed:", err)
	}
	return nil
}
func (r *rollingFile) write(now time.Time, p []byte) error {
	if r.file == nil || r.day != now.Format("2006-01-02") {
		if err := r.rotate(now); err != nil {
			return err
		}
	}
	if r.size > 0 && r.size+int64(len(p)) > r.limit {
		r.part++
		if err := r.rotate(now); err != nil {
			return err
		}
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return err
}
