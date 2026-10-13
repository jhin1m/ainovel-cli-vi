package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) ErrorDetail {
	t.Helper()
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body không phải JSON lỗi: %q", rec.Body.String())
	}
	return body.Error
}

func TestRecoverReturnsJSON500(t *testing.T) {
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("bùm") }),
		RequestContext(nil), Recover)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 || decodeError(t, rec).Code != "internal" {
		t.Fatalf("panic phải thành 500 JSON: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("thiếu X-Request-ID")
	}
}

func TestMaxBytes(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]any
		if err := Decode(r, &v); err != nil {
			WriteErr(w, r, err)
			return
		}
		w.WriteHeader(204)
	}), MaxBytes(16))
	// Content-Length lớn: chặn ngay.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/", strings.NewReader(`{"a":"`+strings.Repeat("x", 100)+`"}`)))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("muốn 413, nhận %d", rec.Code)
	}
	// Không có Content-Length (chunked): MaxBytesReader chặn khi đọc.
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"a":"`+strings.Repeat("x", 100)+`"}`))
	req.ContentLength = -1
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("muốn 413 khi stream, nhận %d", rec.Code)
	}
}

func TestRequireJSON(t *testing.T) {
	h := RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	cases := []struct {
		method, ct string
		want       int
	}{
		{"POST", "text/plain", 415},
		{"POST", "application/x-www-form-urlencoded", 415},
		{"DELETE", "", 415},
		{"POST", "application/json", 204},
		{"PATCH", "application/json; charset=utf-8", 204},
		{"GET", "", 204},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "/", strings.NewReader("{}"))
		if tc.ct != "" {
			req.Header.Set("Content-Type", tc.ct)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s %q: muốn %d, nhận %d", tc.method, tc.ct, tc.want, rec.Code)
		}
	}
}

func TestCrossOriginBlocksForeignPost(t *testing.T) {
	mw, err := CrossOrigin()
	if err != nil {
		t.Fatal(err)
	}
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))

	req := httptest.NewRequest("POST", "http://novel.example.com/api/x", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 || decodeError(t, rec).Code != "cross_origin" {
		t.Fatalf("POST cross-site phải 403 JSON: %d %s", rec.Code, rec.Body)
	}

	req = httptest.NewRequest("POST", "http://novel.example.com/api/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("Origin lạ phải 403, nhận %d", rec.Code)
	}

	req = httptest.NewRequest("POST", "http://novel.example.com/api/x", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("same-origin phải qua, nhận %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	mk := func(remote string, headers map[string]string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return r
	}
	cases := []struct {
		name    string
		req     *http.Request
		trusted []netip.Prefix
		want    string
	}{
		{"peer không tin cậy, header bị bỏ qua", mk("203.0.113.9:1234", map[string]string{"CF-Connecting-IP": "1.2.3.4", "X-Forwarded-For": "5.6.7.8"}), trusted, "203.0.113.9"},
		{"không có proxy tin cậy", mk("10.0.0.1:1", map[string]string{"CF-Connecting-IP": "1.2.3.4"}), nil, "10.0.0.1"},
		{"proxy tin cậy + CF", mk("10.0.0.1:1", map[string]string{"CF-Connecting-IP": "1.2.3.4"}), trusted, "1.2.3.4"},
		{"proxy tin cậy + XFF", mk("10.0.0.1:1", map[string]string{"X-Forwarded-For": "9.9.9.9, 1.2.3.4, 10.0.0.2"}), trusted, "1.2.3.4"},
		{"XFF rác", mk("10.0.0.1:1", map[string]string{"X-Forwarded-For": "abc"}), trusted, "10.0.0.1"},
		{"IPv4-mapped", mk("[::ffff:203.0.113.9]:1", nil), nil, "203.0.113.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClientIP(tc.req, tc.trusted).String(); got != tc.want {
				t.Fatalf("muốn %s, nhận %s", tc.want, got)
			}
		})
	}
}

func TestDecodeRejectsUnknownAndHidesValues(t *testing.T) {
	var v struct {
		Email string `json:"email"`
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"a@b.c","password":"hunter2-secret"}`))
	err := Decode(r, &v)
	if err == nil {
		t.Fatal("field lạ phải lỗi")
	}
	if strings.Contains(err.Error(), "hunter2-secret") {
		t.Fatalf("lỗi lộ giá trị: %v", err)
	}
	r = httptest.NewRequest("POST", "/", strings.NewReader(`{"email":123}`))
	if err := Decode(r, &v); err == nil || !strings.Contains(err.Error(), "email") {
		t.Fatalf("sai kiểu phải nêu field: %v", err)
	}
}
