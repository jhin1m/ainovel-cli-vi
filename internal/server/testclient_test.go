package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/db/dbtest"
	"github.com/voocel/ainovel-cli/internal/server/keys"
)

// logBuffer gom toàn bộ log của server trong test để kiểm không lộ bí mật.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// captureLogs chuyển slog mặc định sang buffer trong suốt test.
func captureLogs(t *testing.T) *logBuffer {
	t.Helper()
	buf := &logBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

type testEnv struct {
	t   *testing.T
	srv *Server
	ts  *httptest.Server
}

// newTestEnv dựng server thật trên Postgres test riêng, chế độ dev (cookie không Secure).
func newTestEnv(t *testing.T, mutate func(*Config)) *testEnv {
	t.Helper()
	return newTestEnvOpts(t, mutate, nil)
}

// newTestEnvOpts như newTestEnv, cho sửa thêm Options (vd. hàm đo dung lượng đĩa).
func newTestEnvOpts(t *testing.T, mutate func(*Config), mutateOpts func(*Options)) *testEnv {
	t.Helper()
	cfg := testConfig(t)
	cfg.Dev = true
	cfg.DatabaseURL = dbtest.NewURL(t)
	if mutate != nil {
		mutate(&cfg)
	}
	opts := Options{Version: "test", Dist: testDist(), Catalog: keys.NewCatalog(models.NewModelRegistry())}
	if mutateOpts != nil {
		mutateOpts(&opts)
	}
	s, err := New(context.Background(), cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		ts.Close()
		s.shutdownRest(context.Background())
	})
	return &testEnv{t: t, srv: s, ts: ts}
}

// client là một trình duyệt giả: có cookie jar, gửi JSON same-origin.
type client struct {
	t    *testing.T
	base string
	hc   *http.Client
	ip   string // X-Forwarded-For giả (chỉ có tác dụng khi tin proxy)
}

func (e *testEnv) newClient() *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: e.t, base: e.ts.URL, hc: &http.Client{Jar: jar}}
}

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
}

func (r response) errCode() string {
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.body, &e)
	return e.Error.Code
}

func (c *client) do(method, path string, body any) response {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.base)
	if c.ip != "" {
		req.Header.Set("X-Forwarded-For", c.ip)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return response{status: resp.StatusCode, header: resp.Header, body: data}
}

// signup đăng ký và đăng nhập (cookie lưu trong jar).
func (c *client) signup(email, password string) {
	c.t.Helper()
	r := c.do("POST", "/api/auth/signup", map[string]string{
		"email": email, "password": password, "display_name": strings.Split(email, "@")[0],
	})
	if r.status != http.StatusCreated {
		c.t.Fatalf("signup %s: %d %s", email, r.status, r.body)
	}
}
