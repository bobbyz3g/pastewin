//go:build windows

package native

import (
	"bytes"
	"golang.org/x/sys/windows"
	"image/png"
	"path/filepath"
	"testing"
)

func TestApplicationIcon(t *testing.T) {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	data, err := ApplicationIcon(filepath.Join(dir, "cmd.exe"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
		t.Fatalf("invalid icon: %v", err)
	}
	for _, path := range []string{`\\server\app.exe`, `relative.exe`, `C:app.exe`, filepath.Join(t.TempDir(), "missing.exe")} {
		if _, err := ApplicationIcon(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}

func TestSourceFallbackRejectsStaleEventsAndOwnPanel(t *testing.T) {
	m := &Monitor{}
	m.foreground.Store(uint64(42)<<32 | 123)
	if got := m.fallbackSourcePID(42); got != 123 {
		t.Fatalf("lost event source: %d", got)
	}
	if got := m.fallbackSourcePID(43); got != 0 {
		t.Fatalf("stale source attributed: %d", got)
	}
	m.foreground.Store(uint64(43)<<32 | uint64(windows.GetCurrentProcessId()))
	if got := m.fallbackSourcePID(43); got != 0 {
		t.Fatal("attributed own panel")
	}
}

func TestWebViewHostResolution(t *testing.T) {
	base := map[uint32]sourceProcess{1: {`C:\runtime\msedgewebview2.exe`, 2, 30}, 2: {`C:\runtime\msedgewebview2.exe`, 3, 20}, 3: {`C:\Clash Verge\clash-verge.exe`, 4, 10}}
	lookup := func(id uint32) sourceProcess { return base[id] }
	if got := resolveWebViewHost(1, lookup); got != base[3].path {
		t.Fatal("wrong host", got)
	}
	base[3] = sourceProcess{`C:\DBX\dbx.exe`, 4, 40}
	if got := resolveWebViewHost(1, lookup); got != "" {
		t.Fatal("reused parent PID accepted", got)
	}
	delete(base, 3)
	if got := resolveWebViewHost(1, lookup); got != "" {
		t.Fatal("missing host accepted", got)
	}
	base[2] = sourceProcess{`C:\runtime\msedgewebview2.exe`, 1, 30}
	if got := resolveWebViewHost(1, lookup); got != "" {
		t.Fatal("cycle accepted", got)
	}
}
