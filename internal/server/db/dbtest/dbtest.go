// Package dbtest tạo database Postgres riêng cho từng test integration.
// Test bị bỏ qua khi thiếu AINOVEL_TEST_DATABASE_URL.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/db"
)

// EnvURL là biến môi trường trỏ tới một Postgres dùng cho test.
const EnvURL = "AINOVEL_TEST_DATABASE_URL"

// NewURL tạo một database trống, trả về chuỗi kết nối tới nó; tự xoá khi test xong.
func NewURL(t testing.TB) string {
	t.Helper()
	base := os.Getenv(EnvURL)
	if base == "" {
		t.Skip("bỏ qua: chưa đặt " + EnvURL)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("kết nối Postgres test: %v", err)
	}
	defer admin.Close(context.Background())

	var b [6]byte
	_, _ = rand.Read(b[:])
	name := "ainovel_test_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("tạo database test: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, base)
		if err != nil {
			return
		}
		defer conn.Close(context.Background())
		_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvURL, err)
	}
	u.Path = "/" + name
	return u.String()
}

// New tạo database riêng, chạy migration và trả về pool (tự đóng khi test xong).
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, NewURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}
