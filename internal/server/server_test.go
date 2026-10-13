package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/server/db/dbtest"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Listen:    "127.0.0.1:0",
		DataDir:   t.TempDir(),
		MasterKey: []byte("0123456789abcdef0123456789abcdef"),
		Limits:    Limits{MaxJobs: 4, MaxJobsPerUser: 1, MaxBooksPerUser: 20, MaxBookBudgetUSD: 50, MaxSSEPerUser: 4, MaxSSETotal: 200, MaxExports: 2},
	}
}

func TestServerRoutesWithoutDB(t *testing.T) {
	s, err := newServer(testConfig(t), nil, Options{Version: "test", Dist: testDist()})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	// API không tồn tại → 404 JSON.
	resp, err := http.Get(ts.URL + "/api/khong-co")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 404 || !strings.Contains(string(body), `"not_found"`) {
		t.Fatalf("muốn 404 JSON: %d %s", resp.StatusCode, body)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("X-Request-ID") == "" {
		t.Fatalf("thiếu header bảo mật/request id: %v", resp.Header)
	}

	// POST cross-origin → 403; POST không JSON → 415; same-origin JSON qua tới 404.
	req, _ := http.NewRequest("POST", ts.URL+"/api/khong-co", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("cross-origin phải 403, nhận %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", ts.URL+"/api/khong-co", strings.NewReader("a=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 415 {
		t.Fatalf("form phải 415, nhận %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", ts.URL+"/api/khong-co", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", ts.URL)
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("same-origin JSON phải tới handler (404), nhận %d", resp.StatusCode)
	}

	// SPA: route của SPA trả index.html có CSP.
	resp, err = http.Get(ts.URL + "/books/123")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("SPA fallback sai: %d %v", resp.StatusCode, resp.Header)
	}

	// healthz không có DB → 503.
	resp, err = http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatalf("healthz không DB phải 503, nhận %d", resp.StatusCode)
	}
}

func TestServerHealthAndTwoStepShutdown(t *testing.T) {
	cfg := testConfig(t)
	cfg.DatabaseURL = dbtest.NewURL(t)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := New(ctx, cfg, Options{Version: "test", Dist: testDist()})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var order []string
	s.beforeShutdown = append(s.beforeShutdown, func(context.Context) {
		mu.Lock()
		order = append(order, "before")
		mu.Unlock()
	})
	s.afterHTTP = append(s.afterHTTP, func(context.Context) {
		mu.Lock()
		order = append(order, "after")
		mu.Unlock()
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if resp.StatusCode != 200 || body["db"] != "ok" {
		t.Fatalf("healthz: %d %v", resp.StatusCode, body)
	}
	if _, ok := body["disk_free_bytes"]; !ok {
		t.Fatalf("thiếu disk_free_bytes: %v", body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("server không tắt")
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(order, ",") != "before,after" {
		t.Fatalf("thứ tự tắt sai: %v", order)
	}
	select {
	case <-s.Closing():
	default:
		t.Fatal("Closing chưa đóng")
	}
}
