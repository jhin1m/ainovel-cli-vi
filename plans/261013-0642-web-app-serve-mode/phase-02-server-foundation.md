---
phase: 2
title: "Server foundation"
status: pending
priority: P1
effort: "1d"
dependencies: []
---

# Phase 2: Server foundation

## Overview

Bộ khung `ainovel-cli serve`: config từ env + file secret, Postgres + migration nhúng, middleware HTTP (CSRF, CSP, giới hạn body), `/healthz`, phục vụ SPA nhúng (placeholder). Chưa có nghiệp vụ.

## Requirements

- Functional:
  - `ainovel-cli serve`, config:
    - Env thường: `AINOVEL_LISTEN` (mặc định `127.0.0.1:8080`), `AINOVEL_DATA_DIR` (mặc định `/var/lib/ainovel`), `AINOVEL_PUBLIC_URL`, `AINOVEL_TRUSTED_PROXIES` (CIDR, phẩy ngăn cách; rỗng = không tin header IP nào), `AINOVEL_DEV` (bool).
    - **Secret chỉ đọc từ file**, không nhận qua env (env của server đọc được qua `/proc/<pid>/environ` bởi cùng UID): `AINOVEL_DATABASE_URL_FILE`, `AINOVEL_MASTER_KEY_FILE`. Trên systemd trỏ vào `$CREDENTIALS_DIRECTORY`. Ở dev cho phép `AINOVEL_DATABASE_URL` khi `AINOVEL_DEV=true`.
    - Giới hạn (khai báo một chỗ): `AINOVEL_MAX_JOBS` (4), `AINOVEL_MAX_JOBS_PER_USER` (1), `AINOVEL_MAX_BOOKS_PER_USER` (20), `AINOVEL_MAX_BOOK_BUDGET_USD` (50), `AINOVEL_MAX_SSE_PER_USER` (4), `AINOVEL_MAX_SSE_TOTAL` (200), `AINOVEL_MAX_EXPORTS` (2).
  - Migration tự chạy lúc khởi động, có `pg_advisory_lock`.
  - Graceful shutdown được chia hai bước để phase 4 nối vào: `BeforeShutdown` (đánh dấu job) → dừng HTTP → `Shutdown` supervisor → đóng pool.
- Non-functional:
  - Router stdlib `net/http` ServeMux.
  - CSRF: `http.CrossOriginProtection` (đã xác nhận có trong Go 1.25 bằng `go doc`) + bắt buộc `Content-Type: application/json` cho POST/PUT/PATCH/DELETE.
  - CSP cho HTML: `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'`; thêm `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`.
  - Error JSON thống nhất `{"error":{"code":"...","message":"..."}}`, message tiếng Việt.
  - Không `WriteTimeout` toàn cục; `ReadHeaderTimeout` 10s; route thường bọc `http.TimeoutHandler` 30s; SSE tự quản deadline (phase 4).
  - Hàm `clientIP(r)`: chỉ đọc `CF-Connecting-IP`/`X-Forwarded-For` khi `RemoteAddr` thuộc `AINOVEL_TRUSTED_PROXIES`; ngược lại dùng `RemoteAddr`.

## Architecture

```
internal/server/
  config.go          // env + *_FILE → Config, validate
  server.go          // New(cfg) → *Server; routes(); Run(ctx)
  httpx/
    middleware.go    // requestID, recover, accessLog(slog), maxBytes, requireJSON, securityHeaders
    clientip.go      // trusted proxies
    json.go          // decode(DisallowUnknownFields), writeJSON, writeError
  db/
    db.go            // pgxpool, Migrate(ctx): goose + advisory lock
    migrations/0001_init.sql
  spa.go             // web.Dist: file có thật → FileServer; path không đuôi → index.html; /assets/* immutable; index/sw/manifest no-cache
internal/server/api/ // tạo ở phase 1 (wire types)
web/
  embed.go           // package web; //go:embed all:dist; var Dist embed.FS
  dist/.gitkeep
```

- Dependency mới (xin user duyệt một lần ở validate cho cả plan): `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3`, `golang.org/x/crypto` (argon2), `golang.org/x/time` (rate). Hai cái `x/` hiện **chưa** có trong `go.mod`.
- `.gitignore` đang có `dist/` → thêm `!web/dist/` + `web/dist/*` + `!web/dist/.gitkeep`; kiểm bằng `git check-ignore -v`.
- Postgres dev: ghi lệnh `docker run postgres:17` vào `docs/web-server.md` (phase 6); không đụng `docker-compose.yml` hiện có.

## Related Code Files

- Create: cây trên + `server_test.go`, `httpx/*_test.go`, `spa_test.go`, `db/db_test.go` (integration, skip khi thiếu `AINOVEL_TEST_DATABASE_URL`)
- Modify: `cmd/ainovel-cli/main.go` (subcommand `serve`), `go.mod`/`go.sum`, `.gitignore`

## Implementation Steps

1. `config.go` + test (thiếu file secret → lỗi rõ ràng; env `DATABASE_URL` bị từ chối khi không ở dev; quyền file rộng hơn 0600 → cảnh báo).
2. `httpx` + test: panic → 500 JSON; request id trong header; body quá cỡ → 413; POST không JSON → 415; POST cross-origin → 403; `clientIP` không tin header khi peer không thuộc danh sách.
3. `db.go` + goose embed + advisory lock; integration test migrate song song.
4. `web/embed.go` + `spa.go` + test bằng `fstest.MapFS` (fallback, 404 asset thiếu, header cache, CSP).
5. `server.go`: `/healthz` (ping DB + dung lượng trống của `DATA_DIR` qua `statfs`), `/api/` 404 JSON, SPA, shutdown hai bước.
6. Nối `serve` vào `main.go`; chạy thử với Postgres local.
7. `gofmt -l`, `go vet ./...`, `go test ./internal/server/... -race`.

## Success Criteria

- [ ] `./ainovel-cli serve` lên với file secret; `/healthz` 200; migration chạy đúng một lần khi hai instance khởi động cùng lúc.
- [ ] POST cross-origin bị chặn; same-origin JSON qua; trang HTML có CSP.
- [ ] `go build` chạy được khi `web/dist` chỉ có `.gitkeep`.

## Risk Assessment

- Binary CLI nặng thêm vì pgx/goose — chấp nhận (một binary duy nhất).
- `TimeoutHandler` không dùng được cho SSE/export stream → chỉ bọc route JSON thường.
