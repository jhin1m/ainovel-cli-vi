package db_test

import (
	"context"
	"sync"
	"testing"

	"github.com/voocel/ainovel-cli/internal/server/db"
	"github.com/voocel/ainovel-cli/internal/server/db/dbtest"
)

// Hai instance khởi động cùng lúc: migration chạy đúng một lần, không lỗi.
func TestMigrateConcurrent(t *testing.T) {
	url := dbtest.NewURL(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pool, err := db.Open(ctx, url)
			if err != nil {
				errs[i] = err
				return
			}
			defer pool.Close()
			errs[i] = db.Migrate(ctx, pool)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("instance %d: %v", i, err)
		}
	}

	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var applied, distinct int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(DISTINCT version_id) FROM goose_db_version WHERE version_id > 0 AND is_applied`).Scan(&applied, &distinct); err != nil {
		t.Fatal(err)
	}
	if applied != distinct || applied == 0 {
		t.Fatalf("migration chạy lặp: %d bản ghi cho %d version", applied, distinct)
	}
}

func TestOpenHidesConnectionString(t *testing.T) {
	_, err := db.Open(context.Background(), "postgres://u:supersecret@[::1")
	if err == nil {
		t.Fatal("phải lỗi")
	}
	if got := err.Error(); contains(got, "supersecret") {
		t.Fatalf("lỗi lộ mật khẩu: %s", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
