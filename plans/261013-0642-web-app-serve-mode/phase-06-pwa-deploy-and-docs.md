---
phase: 6
title: "PWA, deploy, docs"
status: pending
priority: P2
effort: "1.5d"
dependencies: [5]
---

# Phase 6: PWA, deploy, docs

## Overview

Phase này gồm bốn việc:
- PWA: cài được lên điện thoại như một app.
- Build ra một binary duy nhất có nhúng SPA, bằng `make build`.
- Triển khai lên VPS sau Cloudflare, chỉ dùng **systemd**.
- Backup, CI cho web, và tài liệu.

Không sửa `.goreleaser.yml`, `Dockerfile`, `.github/workflows/ci.yml` (file của upstream). Bản release CLI giữ nguyên.

## Requirements

### Chức năng

**PWA**
- Manifest: `lang: vi`, `display: standalone`, icon 192/512 và maskable. Tên app: "AI Novel".
- Service worker:
  - chỉ precache asset build;
  - `navigateFallbackDenylist: [/^\/api\//, /^\/healthz/]`;
  - không runtime cache cho API.

**Build**
- `make build` = `pnpm -C web install --frozen-lockfile && pnpm -C web build && go build -o ainovel-cli ./cmd/ainovel-cli`.

**systemd**
- `deploy/ainovel-serve.service`:
  - `User=ainovel`, `NoNewPrivileges`, `ProtectSystem=strict`, `ReadWritePaths=/var/lib/ainovel`, `PrivateTmp`, `LimitCORE=0`;
  - `KillMode=mixed` (systemd chỉ gửi SIGTERM cho server; server tự tắt worker theo phase 4), `TimeoutStopSec=60`;
  - `MemoryMax`, `TasksMax`;
  - `LoadCredential=database-url:/etc/ainovel/database-url` và `LoadCredential=master-key:/etc/ainovel/master-key`; trỏ `AINOVEL_*_FILE` vào `${CREDENTIALS_DIRECTORY}`;
  - Worker tự đặt `oom_score_adj=500` (phase 1), nên khi OOM kernel giết worker trước server.

**Caddy và Cloudflare**
- `deploy/Caddyfile`: `reverse_proxy 127.0.0.1:8080 { flush_interval -1 }`, không `encode` cho `text/event-stream`, `trusted_proxies` = dải IP Cloudflare.
- Server đặt `AINOVEL_TRUSTED_PROXIES=127.0.0.1/32`. Caddy chỉ nhận kết nối từ Cloudflare.
- Cloudflare: Full (strict). Firewall 443 chỉ mở cho dải Cloudflare. Rule rate limit cho `/api/auth/*`.
- Postgres 17 chỉ bind localhost.

**Backup**
- `deploy/backup.sh`: `pg_dump -Fc` + restic cho `/var/lib/ainovel/users`.
- Docs ghi rõ: **master key backup riêng** (password manager), không để chung với bản dump.

**CI**
- File mới `.github/workflows/web.yml`:
  - `pnpm install`, typecheck, lint, test, build;
  - `make gen && git diff --exit-code` để bắt `types.gen.ts` lệch;
  - `go build` với `web/dist` thật;
  - `go test -race ./internal/server/... ./internal/entry/worker/...` có service Postgres.

### Phi chức năng

- Binary lúc chạy không cần Node.
- Server Go trả `sw.js`, `manifest.webmanifest`, `index.html` với `Cache-Control: no-cache` (đã có ở `spa.go` từ phase 2).

## Related Code Files

- Create:
  - `web/public/icons/*`, cấu hình `vite-plugin-pwa` trong `web/vite.config.ts`;
  - `deploy/ainovel-serve.service`, `deploy/Caddyfile`, `deploy/backup.sh`;
  - `.github/workflows/web.yml`, `docs/web-server.md`.
- Modify:
  - `README.md`: thêm mục "Bản web";
  - `SYNC.md`: liệt kê đủ các file upstream bị chạm (`cmd/ainovel-cli/main.go`, `.gitignore`, `go.mod`/`go.sum`, `README.md`); các thư mục riêng của fork (`internal/server/`, `internal/entry/worker/`, `web/`, `deploy/`, `Makefile`, `tygo.yaml`, `.github/workflows/web.yml`); và cách giải conflict `go.mod`.

## Implementation Steps

1. Cài `vite-plugin-pwa` (đọc ghi chú migration của bản 2.0), thêm icon và manifest. Kiểm Lighthouse báo "Installable".
2. Viết `web.yml`.
3. Viết các file deploy và `docs/web-server.md`, gồm:
   - cài đặt;
   - biến env và file credential;
   - tạo master key: `openssl rand -base64 32 > /etc/ainovel/master-key`, `chmod 0400`;
   - tạo invite;
   - nâng cấp theo đúng thứ tự stop → thay binary → start;
   - backup và restore;
   - giới hạn hiện tại: chưa có UID riêng cho worker.
4. Triển khai lên VPS staging và kiểm:
   - SSE qua Cloudflare chạy > 5 phút không bị 524;
   - cài PWA trên Android và iOS;
   - chạy hết kịch bản end-to-end trong Success Criteria của `plan.md`;
   - `systemctl restart` giữa chừng thì truyện tự chạy tiếp.

## Success Criteria

- [ ] `make build` ra một binary; `serve` chạy được mà không cần thư mục `web/` đi kèm.
- [ ] Cài được PWA; khi có bản mới thì service worker tự lấy bản mới; SSE vẫn chạy khi service worker đang active.
- [ ] CI `web.yml` xanh; `ci.yml` của upstream không đổi.
- [ ] `systemctl restart` trên staging không làm truyện nào thành `failed`.
- [ ] `docs/web-server.md` đủ để người khác tự dựng server.

## Risk Assessment

- **Workbox và SSE:** chưa có nguồn kiểm chứng, nên phải test thật ở bước 4.
- **iOS PWA:** SSE bị ngắt khi khoá màn hình. Đã xử lý bằng kết nối lại và dựng lại timeline (phase 4/5).
- **Cloudflare Free cắt kết nối im lặng sau 100s:** heartbeat 20s. Nếu vẫn bị 524 thì giảm còn 10s.
- **Diễn tập restore toàn phần:** để sau MVP. Script backup và tài liệu restore vẫn làm trong phase này.
