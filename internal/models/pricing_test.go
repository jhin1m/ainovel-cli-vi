package models

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ctx đã huỷ: không gọi mạng được, không ghi gì vào cacheDir và trả về ngay — phía
// gọi chờ hàm này xong là chắc chắn thư mục cache không còn bị ghi (vd. TempDir của test).
func TestRefreshPricingCancelledWritesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		RefreshPricing(ctx, NewModelRegistry(), dir)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RefreshPricing phải trả về ngay khi ctx đã huỷ")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("ctx đã huỷ thì không được tạo/ghi thư mục cache, stat err=%v", err)
	}
}

// Cache còn hạn: nạp vào registry ngay trong lời gọi, không cần mạng.
func TestRefreshPricingLoadsFreshCache(t *testing.T) {
	dir := t.TempDir()
	entry := ModelEntry{Provider: "testprov", ID: "test-model", InputCostPer1M: 1.5, OutputCostPer1M: 3}
	data, err := json.Marshal(modelCache{FetchedAt: time.Now(), Models: []ModelEntry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, cacheFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	reg := NewModelRegistry()
	RefreshPricing(context.Background(), reg, dir)
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	for _, m := range reg.models {
		if m.Provider == entry.Provider && m.ID == entry.ID && m.InputCostPer1M == entry.InputCostPer1M {
			return
		}
	}
	t.Fatal("model trong cache phải được nạp vào registry")
}
