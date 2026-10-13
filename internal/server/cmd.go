package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/voocel/ainovel-cli/internal/server/admin"
	"github.com/voocel/ainovel-cli/internal/server/db"
)

const serveUsage = `Cách dùng:
  ainovel-cli serve                                         chạy server web
  ainovel-cli serve invite create [--uses N] [--expires 7d] tạo mã mời

Cấu hình qua biến môi trường (bắt buộc: AINOVEL_DATABASE_URL_FILE, AINOVEL_MASTER_KEY_FILE).
Xem docs/web-server.md để biết đầy đủ.
`

// Command chạy `ainovel-cli serve [...]` và trả về exit code.
func Command(args []string, version string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, serveUsage)
		return 0
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg, err := LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 1
	}
	if len(args) > 0 {
		if len(args) >= 2 && args[0] == "invite" && args[1] == "create" {
			return inviteCreate(cfg, args[2:], stdout, stderr)
		}
		fmt.Fprintf(stderr, "serve: lệnh con %q không được hỗ trợ (có: invite create [--uses N] [--expires 7d])\n", strings.Join(args, " "))
		return 2
	}
	for _, w := range cfg.Warnings {
		slog.Warn(w)
	}
	if !cfg.Dev && len(cfg.TrustedProxies) == 0 {
		slog.Warn("chưa đặt AINOVEL_TRUSTED_PROXIES: sau reverse proxy mọi client sẽ cùng một IP, rate limit đăng nhập dùng chung cho tất cả")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Tín hiệu thứ hai (Ctrl+C lần nữa) giết process ngay theo hành vi mặc định.
	go func() {
		<-ctx.Done()
		stop()
	}()

	srv, err := New(ctx, cfg, Options{Version: version})
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 1
	}
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 1
	}
	return 0
}

// inviteCreate: `ainovel-cli serve invite create [--uses N] [--expires 7d]` in mã mời ra stdout.
func inviteCreate(cfg Config, args []string, stdout, stderr io.Writer) int {
	uses, ttlText := 1, "7d"
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			fmt.Fprintf(stderr, "serve invite create: %s thiếu giá trị\n", args[i])
			return 2
		}
		switch args[i] {
		case "--uses":
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "serve invite create: --uses phải là số nguyên > 0")
				return 2
			}
			uses = n
		case "--expires":
			ttlText = args[i+1]
		default:
			fmt.Fprintf(stderr, "serve invite create: tham số %q không hợp lệ\n", args[i])
			return 2
		}
		i++
	}
	ttl, err := admin.ParseTTL(ttlText)
	if err != nil {
		fmt.Fprintln(stderr, "serve invite create:", err)
		return 2
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(stderr, "serve invite create:", err)
		return 1
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(stderr, "serve invite create:", err)
		return 1
	}
	code, err := admin.CreateInvite(ctx, pool, uses, ttl)
	if err != nil {
		fmt.Fprintln(stderr, "serve invite create:", err)
		return 1
	}
	fmt.Fprintln(stdout, code)
	return 0
}
