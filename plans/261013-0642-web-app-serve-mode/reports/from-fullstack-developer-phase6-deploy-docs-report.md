# Phase 6 deploy + docs report

## Files
- Created: deploy/ainovel-serve.service, deploy/Caddyfile, deploy/backup.sh (0755), .github/workflows/web.yml, docs/web-server.md
- Modified: README.md (mục "Bản web" trước "Cấu hình"), SYNC.md (mục bản web, file upstream bị chạm, cách giải conflict go.mod)
- Makefile: không đổi (target web/build/gen đã đủ)

## Verification
- `make build`: OK (pnpm install frozen, vite build, go build) -> ./ainovel-cli 29MB.
- `./ainovel-cli serve --help`: KHÔNG chạy được. serve LoadConfig trước khi xét args, nên báo thiếu AINOVEL_DATABASE_URL_FILE (exit 1); nếu đủ env thì `--help` bị coi là lệnh con lạ (exit 2). Cần sửa Go (ngoài phạm vi). Docs ghi "serve không có --help".
- systemd-analyze, shellcheck, caddy, actionlint: không có trên macOS, bỏ qua. Đã `bash -n` backup.sh OK, YAML web.yml parse OK.
- File upstream bị sửa (diff-filter=M vi...HEAD): .gitignore, cmd/ainovel-cli/main.go, go.mod, go.sum (+ README.md ở working tree).

## Quyết định
- Unit dùng `%d` (credentials dir, systemd >= 250) cho AINOVEL_*_FILE vì `${CREDENTIALS_DIRECTORY}` không được mở rộng trong Environment=. Docs ghi yêu cầu Debian 12/Ubuntu 24.04+.
- TimeoutStopSec=90 (server drain tối đa 60s theo server.go).
- Caddy: Cloudflare ranges nhập tay (không dùng plugin), `header_up X-Forwarded-For {client_ip}` ghi đè; server giữ XFF mặc định. Không encode path /api/books/*/events. Không thêm CSP/XFO ở Caddy vì app đã đặt.
- backup.sh: spec nhắc restic, em dùng tar (không thêm dependency); đẩy ra ngoài máy do người vận hành. pg_dump nhận URL qua --dbname nên lộ trong `ps` với user khác (ghi chú trong script/docs: chạy bằng root).
- web.yml: pnpm ghim 12.11.2 vì web/package.json KHÔNG có `packageManager` (spec nói theo field này). Có job server (Postgres 17, go test -race) và job web (gen diff, typecheck, lint, test, build, go build). Trigger rộng hơn yêu cầu (web/**, internal/server/**, internal/entry/worker/**, tygo.yaml, Makefile).

## Chưa làm / cần người
- Không có PWA (vite-plugin-pwa, icon) vì web/** ngoài ownership; nếu phase 5 chưa làm thì còn thiếu.
- Staging VPS, SSE qua Cloudflare, cài PWA, restore drill: cần người.
- `make gen && git diff --exit-code` trong CI chưa chạy thử cục bộ (tygo cần mạng).
- Chưa kiểm thật `trusted_proxies static` + `client_ip_headers` trong Caddyfile (không có caddy); cú pháp theo Caddy 2.7+.

Status: DONE_WITH_CONCERNS
Summary: Đủ deploy files, CI, docs, README/SYNC; make build chạy được.
Concerns: `serve --help` không hoạt động (cần sửa Go); package.json thiếu packageManager; các file deploy chưa được kiểm bằng công cụ thật (systemd-analyze/caddy/shellcheck vắng).
