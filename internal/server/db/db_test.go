package db_test

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

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

// 0004 bỏ bảng invite_codes; Down dựng lại bảng, Up lại thì bỏ tiếp.
func TestMigration0004DropInviteCodesUpDown(t *testing.T) {
	url := dbtest.NewURL(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	// Đọc migration từ thư mục nguồn (cwd của test là thư mục package).
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, os.DirFS("migrations"))
	if err != nil {
		t.Fatal(err)
	}
	tableExists := func() bool {
		t.Helper()
		var name sql.NullString
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.invite_codes')::text`).Scan(&name); err != nil {
			t.Fatal(err)
		}
		return name.Valid
	}
	wantVersion := func(want int64) {
		t.Helper()
		v, err := provider.GetDBVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if v != want {
			t.Fatalf("version goose = %d, muốn %d", v, want)
		}
	}

	if v, err := provider.GetDBVersion(ctx); err != nil || v < 4 {
		t.Fatalf("Migrate phải chạy tới ít nhất 0004: %d %v", v, err)
	}
	if tableExists() {
		t.Fatal("sau 0004 bảng invite_codes phải bị drop")
	}
	if _, err := provider.DownTo(ctx, 3); err != nil {
		t.Fatal(err)
	}
	wantVersion(3)
	if !tableExists() {
		t.Fatal("Down 0004 phải dựng lại bảng invite_codes")
	}
	if _, err := provider.UpTo(ctx, 4); err != nil {
		t.Fatal(err)
	}
	wantVersion(4)
	if tableExists() {
		t.Fatal("Up lại 0004 phải drop bảng invite_codes")
	}
}

// 0005 nới CHECK budget_usd thành >= 0 cho truyện thuê bao. Down chạy được khi đã
// có truyện budget 0 và để lại dữ liệu mà binary cũ vẫn UPDATE được; Up lại thì
// trả truyện thuê bao về budget 0.
func TestMigration0005BudgetSubscriptionUpDown(t *testing.T) {
	url := dbtest.NewURL(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, os.DirFS("migrations"))
	if err != nil {
		t.Fatal(err)
	}
	constraint := func() (def string, validated bool) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
			WHERE conrelid = 'books'::regclass AND conname = 'books_budget_usd_check'`).Scan(&def, &validated); err != nil {
			t.Fatalf("đọc ràng buộc books_budget_usd_check: %v", err)
		}
		return def, validated
	}
	insertBook := func(budget float64) (string, error) {
		t.Helper()
		var uid, id string
		if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash)
			VALUES ('sub' || gen_random_uuid()::text || '@test.local', 'x') RETURNING id::text`).Scan(&uid); err != nil {
			t.Fatal(err)
		}
		err := pool.QueryRow(ctx, `INSERT INTO books (user_id, prompt, status, provider, model, budget_usd, advance_mode)
			VALUES ($1, 'p', 'paused', 'zai-coding', 'glm-4.7', $2, 'auto') RETURNING id::text`, uid, budget).Scan(&id)
		return id, err
	}
	budgetOf := func(id string) float64 {
		t.Helper()
		var v float64
		if err := pool.QueryRow(ctx, `SELECT budget_usd FROM books WHERE id::text = $1`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	if v, err := provider.GetDBVersion(ctx); err != nil || v < 5 {
		t.Fatalf("Migrate phải chạy tới ít nhất 0005: %d %v", v, err)
	}
	const relaxed = "CHECK ((budget_usd >= (0)::double precision))"
	if def, ok := constraint(); def != relaxed || !ok {
		t.Fatalf("ràng buộc sau 0005 sai: %q validated=%v", def, ok)
	}
	sub, err := insertBook(0)
	if err != nil {
		t.Fatalf("sau 0005 phải nhận budget 0: %v", err)
	}
	if _, err := insertBook(-1); err == nil {
		t.Fatal("budget âm phải bị từ chối")
	}

	if _, err := provider.DownTo(ctx, 4); err != nil {
		t.Fatalf("Down 0005 khi đã có truyện budget 0: %v", err)
	}
	if def, ok := constraint(); def != "CHECK ((budget_usd > (0)::double precision))" || !ok {
		t.Fatalf("ràng buộc sau Down 0005 sai: %q validated=%v", def, ok)
	}
	// Binary cũ cập nhật trạng thái truyện lúc khởi động và khi claim hàng đợi:
	// dòng của truyện thuê bao phải UPDATE được sau Down.
	if _, err := pool.Exec(ctx, `UPDATE books SET status = 'queued' WHERE id::text = $1`, sub); err != nil {
		t.Fatalf("sau Down 0005 truyện thuê bao phải UPDATE được: %v", err)
	}
	if b := budgetOf(sub); !(b > 0) {
		t.Fatalf("Down 0005 phải đổi budget 0 sang giá trị dương: %v", b)
	}
	if _, err := insertBook(0); err == nil {
		t.Fatal("sau Down 0005 budget 0 phải bị từ chối")
	}

	if _, err := provider.UpTo(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if def, ok := constraint(); def != relaxed || !ok {
		t.Fatalf("ràng buộc sau Up lại 0005 sai: %q validated=%v", def, ok)
	}
	if b := budgetOf(sub); b != 0 {
		t.Fatalf("Up lại 0005 phải trả truyện thuê bao về budget 0: %v", b)
	}
	if _, err := insertBook(0); err != nil {
		t.Fatalf("Up lại 0005 phải nhận budget 0: %v", err)
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
