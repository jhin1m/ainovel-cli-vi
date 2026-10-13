package server

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// contentSecurityPolicy áp cho mọi trang HTML. Nội dung truyện do LLM sinh ra là
// dữ liệu không tin cậy: không inline script, không tài nguyên ngoài.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; font-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'"

// noCacheFiles phải luôn được kiểm lại để bản build mới (và service worker mới)
// đến tay người dùng ngay.
var noCacheFiles = map[string]bool{
	"index.html":           true,
	"sw.js":                true,
	"registerSW.js":        true,
	"manifest.webmanifest": true,
}

const placeholderHTML = `<!doctype html><html lang="vi"><head><meta charset="utf-8"><title>AI Novel</title></head>` +
	`<body><p>Giao diện web chưa được build. Chạy <code>make build</code> rồi khởi động lại server.</p></body></html>`

// spaHandler phục vụ SPA nhúng: file có thật thì trả file; đường dẫn không có đuôi
// file (route của SPA) thì trả index.html; asset thiếu thì 404.
func spaHandler(dist fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if serveFile(w, r, dist, name) {
			return
		}
		if path.Ext(name) != "" && name != "index.html" {
			http.NotFound(w, r)
			return
		}
		if serveFile(w, r, dist, "index.html") {
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, placeholderHTML)
	})
}

// serveFile trả file name trong dist nếu có; false nếu không tồn tại hoặc là thư mục.
func serveFile(w http.ResponseWriter, r *http.Request, dist fs.FS, name string) bool {
	if !fs.ValidPath(name) {
		return false
	}
	// FS nhúng chỉ đọc: lỗi duy nhất có thể gặp là không tồn tại hoặc là thư mục.
	data, err := fs.ReadFile(dist, name)
	if err != nil {
		return false
	}
	h := w.Header()
	switch {
	case strings.HasPrefix(name, "assets/"):
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	case noCacheFiles[path.Base(name)]:
		h.Set("Cache-Control", "no-cache")
	default:
		h.Set("Cache-Control", "public, max-age=3600")
	}
	if strings.HasSuffix(name, ".html") {
		h.Set("Content-Security-Policy", contentSecurityPolicy)
	}
	if strings.HasSuffix(name, ".webmanifest") {
		h.Set("Content-Type", "application/manifest+json")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	return true
}
