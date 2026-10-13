package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const serveUsage = `Cách dùng:
  ainovel-cli serve   chạy server web (không có lệnh con)

Cấu hình qua biến môi trường (bắt buộc: AINOVEL_DATABASE_URL_FILE, AINOVEL_MASTER_KEY_FILE).
Xem docs/web-server.md để biết đầy đủ.
`

// wantsHelp: `serve help`, `serve -h` hoặc `serve --help`.
func wantsHelp(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "help", "-h", "--help":
		return true
	}
	return false
}

// Command chạy `ainovel-cli serve [...]` và trả về exit code.
func Command(args []string, version string, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, serveUsage)
		return 0
	}
	// Báo lệnh con lạ trước khi đọc cấu hình để lỗi rõ ràng kể cả khi thiếu secret.
	if len(args) > 0 {
		fmt.Fprintf(stderr, "serve: lệnh con %q không được hỗ trợ (serve không có lệnh con)\n", strings.Join(args, " "))
		return 2
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg, err := LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 1
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
