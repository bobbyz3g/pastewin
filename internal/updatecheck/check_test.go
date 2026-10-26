package updatecheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCheckRelease(t *testing.T) {
	for _, tt := range []struct {
		name, current, body, wantStatus, wantVersion string
		code                                         int
		wantErr                                      error
	}{
		{"numeric patch", "0.3.9", `{"tag_name":"0.3.10"}`, "available", "0.3.10", 200, nil},
		{"optional prefix", "0.3.5", `{"tag_name":"v0.4.0"}`, "available", "0.4.0", 200, nil},
		{"same", "0.3.5", `{"tag_name":"0.3.5"}`, "current", "0.3.5", 200, nil},
		{"local ahead", "1.0.0", `{"tag_name":"0.9.99"}`, "ahead", "0.9.99", 200, nil},
		{"draft", "0.3.5", `{"tag_name":"0.3.6","draft":true}`, "", "", 200, ErrNoRelease},
		{"prerelease", "0.3.5", `{"tag_name":"0.3.6","prerelease":true}`, "", "", 200, ErrNoRelease},
		{"bad tag", "0.3.5", `{"tag_name":"0.3.6-beta"}`, "", "", 200, ErrVersion},
		{"bad json", "0.3.5", `{`, "", "", 200, ErrServer},
		{"trailing json", "0.3.5", `{"tag_name":"0.3.6"} {}`, "", "", 200, ErrServer},
		{"missing tag", "0.3.5", `{}`, "", "", 200, ErrVersion},
		{"missing release", "0.3.5", ``, "", "", 404, ErrNoRelease},
		{"rate limit", "0.3.5", ``, "", "", 429, ErrRateLimit},
		{"server", "0.3.5", ``, "", "", 503, ErrServer},
		{"oversized", "0.3.5", strings.Repeat(" ", maxResponse+1), "", "", 200, ErrServer},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != endpoint || r.Method != http.MethodGet || r.Header.Get("User-Agent") == "" {
					t.Fatalf("unexpected request: %v", r)
				}
				return &http.Response{StatusCode: tt.code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})}
			got, err := check(t.Context(), client, tt.current)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got.Status != tt.wantStatus || got.Version != tt.wantVersion {
				t.Fatalf("result = %+v", got)
			}
			if got.Status == "available" {
				url, err := ReleaseURL(got.Tag)
				if err != nil || !strings.HasSuffix(url, got.Tag) {
					t.Fatalf("release URL = %q, %v", url, err)
				}
			}
		})
	}
}

func TestCheckCancellationAndRateLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return nil, r.Context().Err()
	})}
	if _, err := check(ctx, client, "0.3.5"); !errors.Is(err, ErrNetwork) {
		t.Fatalf("cancel error: %v", err)
	}
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Header: http.Header{"X-Ratelimit-Remaining": []string{"0"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := check(t.Context(), client, "0.3.5"); !errors.Is(err, ErrRateLimit) {
		t.Fatalf("rate limit error: %v", err)
	}
}

func TestReleaseURLRejectsInvalidTags(t *testing.T) {
	for _, tag := range []string{"", "../other", "https://example.com", "0.3.5/../../", "0.3.5?x=1", "0.3", "+0.3.5", "0.03.5", "18446744073709551616.0.0"} {
		if url, err := ReleaseURL(tag); err == nil {
			t.Errorf("accepted %q as %q", tag, url)
		}
	}
}
