package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testDist() fstest.MapFS {
	return fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>AI Novel</title>")},
		"sw.js":                {Data: []byte("self.x=1")},
		"manifest.webmanifest": {Data: []byte(`{"name":"AI Novel"}`)},
		"assets/app-abc123.js": {Data: []byte("console.log(1)")},
		"icons/icon-192.png":   {Data: []byte("png")},
	}
}

func TestSPAHandler(t *testing.T) {
	h := spaHandler(testDist())
	cases := []struct {
		path, wantBody, wantCache string
		wantCode                  int
		wantCSP                   bool
	}{
		{"/", "AI Novel", "no-cache", 200, true},
		{"/books/abc", "AI Novel", "no-cache", 200, true},
		{"/books/abc/read/3", "AI Novel", "no-cache", 200, true},
		{"/assets/app-abc123.js", "console.log", "public, max-age=31536000, immutable", 200, false},
		{"/assets/missing.js", "", "", 404, false},
		{"/favicon.ico", "", "", 404, false},
		{"/sw.js", "self.x", "no-cache", 200, false},
		{"/manifest.webmanifest", "AI Novel", "no-cache", 200, false},
		{"/icons/icon-192.png", "png", "public, max-age=3600", 200, false},
		// path.Clean khử "..": không thoát ra ngoài dist, chỉ thành route SPA.
		{"/../../etc/passwd", "AI Novel", "no-cache", 200, true},
		{"/index.html", "AI Novel", "no-cache", 200, true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			if rec.Code != tc.wantCode {
				t.Fatalf("status %d, muốn %d", rec.Code, tc.wantCode)
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body %q", rec.Body)
			}
			if tc.wantCache != "" && rec.Header().Get("Cache-Control") != tc.wantCache {
				t.Fatalf("Cache-Control %q, muốn %q", rec.Header().Get("Cache-Control"), tc.wantCache)
			}
			if got := rec.Header().Get("Content-Security-Policy") != ""; got != tc.wantCSP {
				t.Fatalf("CSP có=%v, muốn %v", got, tc.wantCSP)
			}
		})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/manifest.webmanifest", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Fatalf("manifest Content-Type %q", ct)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/", nil))
	if rec.Code != 405 {
		t.Fatalf("POST vào SPA phải 405, nhận %d", rec.Code)
	}
}

func TestSPAHandlerPlaceholderWhenNotBuilt(t *testing.T) {
	h := spaHandler(fstest.MapFS{".gitkeep": {}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "make build") {
		t.Fatalf("thiếu trang placeholder: %d %q", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("placeholder cũng phải có CSP")
	}
}
