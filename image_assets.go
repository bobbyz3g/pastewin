package main

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Only record IDs are accepted; this route never exposes arbitrary disk paths.
// It runs inside Wails' WebView asset handler, not a separate listening server.
func (s *ClipboardService) imageAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/source-icon/") {
			s.sourceIcon(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/history-image/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/history-image/"), "/")
		if len(parts) != 2 || (parts[1] != "thumbnail.png" && parts[1] != "image.png") {
			http.NotFound(w, r)
			return
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}
		if s.store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		data, err := s.store.Image(id, parts[1] == "thumbnail.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	})
}

func (s *ClipboardService) sourceIcon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/source-icon/"), ".png")
	if _, err := hex.DecodeString(key); err != nil || len(key) != 64 || !strings.HasSuffix(r.URL.Path, ".png") {
		http.NotFound(w, r)
		return
	}
	if s.store == nil || s.icons == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	path, err := s.store.SourcePath(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	data := s.icons.Get(ctx, key, path)
	if len(data) == 0 {
		slog.Info("source_icon_default", "source_key", key)
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}
