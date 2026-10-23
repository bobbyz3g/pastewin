package appicon

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type result struct {
	ready     chan struct{}
	data      []byte
	key, path string
}

// Cache bounds extraction to one worker, 32 pending requests and 256 results.
// Failures are cached too. No polling or clipboard locks are involved.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*result
	order   []string
	jobs    chan *result
	ctx     context.Context
}

func New(ctx context.Context, extract func(string) ([]byte, error)) *Cache {
	c := &Cache{entries: make(map[string]*result), jobs: make(chan *result, 32), ctx: ctx}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case r := <-c.jobs:
				if ctx.Err() != nil {
					return
				}
				started := time.Now()
				data, err := extract(r.path)
				if err == nil && len(data) <= 32*1024 {
					r.data = data
				}
				if len(r.data) == 0 {
					slog.Warn("source_icon_extract_failed", "source_key", r.key, "source_path", r.path, "error", err, "bytes", len(data), "duration_ms", time.Since(started).Milliseconds())
				} else {
					slog.Info("source_icon_extracted", "source_key", r.key, "source_path", r.path, "bytes", len(data), "duration_ms", time.Since(started).Milliseconds())
				}
				close(r.ready)
			}
		}
	}()
	return c
}
func (c *Cache) Get(ctx context.Context, key, path string) []byte {
	c.mu.Lock()
	r := c.entries[key]
	if r == nil {
		if len(c.entries) >= 256 {
			removed := false
			for i, k := range c.order {
				select {
				case <-c.entries[k].ready:
					delete(c.entries, k)
					c.order = append(c.order[:i], c.order[i+1:]...)
					removed = true
				default:
				}
				if removed {
					break
				}
			}
			if !removed {
				c.mu.Unlock()
				return nil
			}
		}
		r = &result{ready: make(chan struct{}), key: key, path: path}
		select {
		case c.jobs <- r:
			c.entries[key] = r
			c.order = append(c.order, key)
		default:
			slog.Warn("source_icon_queue_full", "source_key", key)
			c.mu.Unlock()
			return nil
		}
	}
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		slog.Warn("source_icon_wait_cancelled", "source_key", key, "error", ctx.Err())
		return nil
	case <-c.ctx.Done():
		return nil
	case <-r.ready:
		return r.data
	}
}
