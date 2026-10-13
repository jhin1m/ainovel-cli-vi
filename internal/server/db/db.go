// Package db mở pool Postgres và chạy migration nhúng trong binary.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open mở pool kết nối và ping thử.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// Không đưa err gốc ra: thông báo của pgx có thể chứa chuỗi kết nối (mật khẩu).
		return nil, fmt.Errorf("chuỗi kết nối Postgres không hợp lệ")
	}
	cfg.MaxConns = 20
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("mở pool Postgres: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("không kết nối được Postgres: %w", err)
	}
	return pool, nil
}

// Migrate chạy mọi migration còn thiếu. Dùng advisory lock của Postgres nên hai
// instance khởi động cùng lúc vẫn chỉ một bên chạy migration.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("khởi tạo migration: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("chạy migration: %w", err)
	}
	return nil
}
