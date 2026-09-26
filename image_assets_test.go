package main

import (
	"bytes"
	"fmt"
	"image"
	"net/http"
	"net/http/httptest"
	"testing"

	"pastewin/internal/clipimage"
)

func TestImageAssetsBoundToHistoryIDs(t *testing.T) {
	s := testService(t)
	img, err := clipimage.Normalize(image.NewNRGBA(image.Rect(0, 0, 300, 100)))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.AddContent("", img); err != nil {
		t.Fatal(err)
	}
	page, _ := s.store.List("", false, 0)
	id := page.Entries[0].ID
	handler := s.imageAssets(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) }))
	for _, size := range []string{"thumbnail", "image"} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", fmt.Sprintf("/history-image/%d/%s.png", id, size), nil))
		want := img.PNG
		if size == "thumbnail" {
			want = img.Thumbnail
		}
		if r.Code != 200 || r.Header().Get("Content-Type") != "image/png" || r.Header().Get("Cache-Control") != "no-store" || !bytes.Equal(r.Body.Bytes(), want) {
			t.Fatalf("bad image response: %d", r.Code)
		}
	}
	for _, path := range []string{"/history-image/../history.db", "/history-image/0/image.png", "/history-image/9999/image.png", "/history-image/1/../../history.db", "/history-image/1/arbitrary.png"} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 404 {
			t.Fatalf("accepted path %s", path)
		}
	}
	s.store.Delete(id)
	r := httptest.NewRecorder()
	handler.ServeHTTP(r, httptest.NewRequest("GET", fmt.Sprintf("/history-image/%d/image.png", id), nil))
	if r.Code != 404 {
		t.Fatal("deleted image still served")
	}
}
