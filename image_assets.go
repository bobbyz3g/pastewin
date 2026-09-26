package main

import (
	"net/http"
	"strconv"
	"strings"
)

// Only record IDs are accepted; this route never exposes arbitrary disk paths.
// It runs inside Wails' WebView asset handler, not a separate listening server.
func (s *ClipboardService) imageAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
