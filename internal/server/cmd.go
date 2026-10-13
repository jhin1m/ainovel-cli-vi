package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// Command chạy `ainovel-cli serve [...]` và trả về exit code.
func Command(args []string, version string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "serve: lệnh con %q không được hỗ trợ\n", args[0])
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
