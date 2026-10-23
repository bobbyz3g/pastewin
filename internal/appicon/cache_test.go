package appicon

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCacheCoalescesAndCachesFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	c := New(ctx, func(string) ([]byte, error) { calls.Add(1); close(started); <-release; return []byte("png"), nil })
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if string(c.Get(ctx, "same", "app")) != "png" {
				t.Error("missing icon")
			}
		})
	}
	<-started
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate extraction", calls.Load())
	}
	failures := 0
	failed := New(ctx, func(string) ([]byte, error) { failures++; return nil, errors.New("missing") })
	for range 3 {
		if failed.Get(ctx, "missing", "bad") != nil {
			t.Fatal("unexpected icon")
		}
	}
	if failures != 1 {
		t.Fatal("failure was not cached")
	}
}
func TestCacheBoundAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(ctx, func(string) ([]byte, error) { return make([]byte, 32*1024), nil })
	for i := range 300 {
		c.Get(ctx, fmt.Sprint(i), "app")
	}
	if len(c.entries) != 256 {
		t.Fatal("unbounded cache", len(c.entries))
	}
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	blocked := New(ctx, func(string) ([]byte, error) { close(started); <-release; return nil, nil })
	request, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { blocked.Get(request, "one", "app"); close(done) }()
	<-started
	stop()
	<-done
}
