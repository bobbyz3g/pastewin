package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"pastewin/internal/appicon"
	"strings"
	"testing"
)

func TestSourceIconRoute(t *testing.T) {
	s := testService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	s.icons = appicon.New(ctx, func(path string) ([]byte, error) { calls++; return []byte("fixture"), nil })
	if err := s.store.AddContent("fixture", nil, `C:\Apps\test.exe`); err != nil {
		t.Fatal(err)
	}
	p, _ := s.store.List("", false, 0)
	e := p.Entries[0]
	h := s.imageAssets(http.NotFoundHandler())
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(method, "/source-icon/"+e.SourceKey+".png", nil))
		if r.Code != 200 || r.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("bad response: %d", r.Code)
		}
		if method == "HEAD" && r.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
	if calls != 1 {
		t.Fatal("icon extracted repeatedly")
	}
	for _, path := range []string{"../test.exe", strings.Repeat("a", 64) + ".png", e.SourceKey + ".jpg"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", "/source-icon/"+path, nil))
		if r.Code != 404 {
			t.Fatal("accepted", path)
		}
	}
	s.store.Delete(e.ID)
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/source-icon/"+e.SourceKey+".png", nil))
	if r.Code != 404 {
		t.Fatal("served orphan")
	}
}
