---
title: "Bản web cho ainovel-cli-vi (chế độ serve + SPA)"
description: "Thêm `ainovel-cli serve` (HTTP + SSE, auth, BYOK, Postgres). Mỗi truyện chạy bằng một process `ainovel-cli worker` riêng. Kèm React SPA nhúng vào binary, mở đường cho PWA/app."
status: in-progress
priority: P1
effort: "13d"
tags: [web, server, go, react, pwa, byok]
created: 2026-10-13
---

# Bản web cho ainovel-cli-vi

## Overview

Luồng người dùng: mở trình duyệt → đăng nhập → dán API key của mình (BYOK) → gõ một câu ý tưởng → engine viết truyện dài. Trong lúc đó người dùng xem tiến độ realtime, đọc chương, can thiệp, duyệt từng chương, rồi xuất TXT/EPUB. Logic engine giữ nguyên, chỉ thêm lớp vỏ bên ngoài.

Các quyết định đã chốt với user (13/10/2026):

| Chủ đề | Quyết định |
|---|---|
| Chi phí LLM | BYOK, không có thanh toán |
| Quan hệ với novel-hub | Sản phẩm riêng. Nút "đăng sang novel-hub" để sau |
| Backend | Go lo toàn bộ: `ainovel-cli serve` (auth, Postgres, BYOK, supervisor, SSE) + React SPA nhúng qua `embed.FS` |
| Vị trí code | Trong fork này: `internal/server/`, `internal/entry/worker/`, `web/`, `deploy/` |
| Quy mô | Public trên VPS, nhiều user |
| Phạm vi MVP | Gọn: auth, key, tạo truyện từ 1 câu, realtime, đọc, steer, review/next, dừng/tiếp, ngân sách, xuất TXT/EPUB, PWA cơ bản |

Các quyết định kiến trúc (lấy từ scout và red-team, xem `reports/`):

- **Mỗi lượt chạy của một truyện là một process con** (`ainovel-cli worker`). Process này **thoát mỗi khi engine dừng**: chờ duyệt, hết ngân sách, người dùng dừng, xong truyện, hoặc lỗi. Tiếp tục, "chương tiếp", hay steer khi đang dừng đều spawn một process mới. Lý do không dùng nhiều `host.Host` trong một process:
  - `lang.Set`, `slog.SetDefault`, `~/.ainovel` và registry pricing là trạng thái toàn cục của process;
  - kênh event chỉ có một consumer;
  - một panic sẽ làm sập cả server.
- **Giao thức JSON Lines qua stdin/stdout.** API key chỉ đi trong message `init` gửi qua stdin.
- **Wire types có json tag** nằm ở `internal/server/api`. Worker map từ struct của engine sang các type này.
- **Trạng thái "chờ duyệt"** dùng đúng điều kiện TUI đang dùng: `AdvanceMode=="review" && !IsRunning && Phase=="writing"`.
- **Ngân sách luôn có hiệu lực:**
  - chỉ cho chọn model có giá trong registry;
  - server gửi kèm `pricing` cho worker;
  - có trạng thái `budget_exhausted` và cho phép nâng ngân sách.
- **Server đọc store trực tiếp** để lấy chương, outline và xuất file (`store.NewStore`, `exp.Run` với `OutPath` riêng). Không cần worker cho các việc này.

## Kiến trúc

```
Browser (React SPA / PWA)
   │  REST JSON + SSE (cookie session, CSRF, CSP)
   ▼
ainovel-cli serve ── Postgres (users, sessions, invites, api_keys, books)
   │  spawn /proc/self/exe worker · stdin init/lệnh · stdout JSONL
   ├── worker (book A, chạy 1 lượt)  HOME=/data/users/<uid>/home  cwd=OutputDir=/data/users/<uid>/books/<bookID>
   └── worker (book B, chạy 1 lượt)
         └── host.Host (engine nguyên bản)
```

## Phases

| # | Phase | Phụ thuộc | Effort | Status |
|---|-------|-----------|--------|--------|
| 1 | [Worker stdio mode](./phase-01-worker-stdio-mode.md) | — | 2d | Completed |
| 2 | [Server foundation](./phase-02-server-foundation.md) | — (làm song song với 1 được) | 1d | Completed |
| 3 | [Auth và BYOK keys](./phase-03-auth-and-byok-keys.md) | 2 | 1d | Completed |
| 4 | [Supervisor, Books API, SSE](./phase-04-job-supervisor-and-books-api.md) | 1, 3 | 3.5d | Pending |
| 5 | [Web SPA](./phase-05-web-spa.md) | 4 | 4d | Pending |
| 6 | [PWA, deploy, docs](./phase-06-pwa-deploy-and-docs.md) | 5 | 1.5d | Pending |

## Ngoài phạm vi MVP

- Tính năng engine chưa đưa lên web: đồng sáng tác (cocreate), import truyện có sẵn, simulate, đổi model theo vai trò hoặc giữa chừng, sửa chương bằng tay.
- Tích hợp và tiền: đăng sang novel-hub, thanh toán/credit.
- Tài khoản: xác minh email, quên mật khẩu (MVP thay bằng invite code), Bearer token cho app native.
- Vận hành và bảo mật:
  - giữ timeline qua lần restart server;
  - UID riêng cho worker;
  - xoay vòng master key (đã có sẵn cột `key_version`);
  - diễn tập restore toàn phần.
- Khác: đọc truyện offline, UI đa ngôn ngữ, Docker image cho bản web, bản release goreleaser có kèm web.

## Success Criteria

- [ ] `ainovel-cli serve` chạy được với Postgres; `ainovel-cli`, `--headless` và TUI không đổi hành vi.
- [ ] Hai user chạy truyện song song mà log, rules, store, key đều tách biệt. User A không đọc, điều khiển hay dùng key được gì của B (có test).
- [ ] Kịch bản end-to-end trên trình duyệt chạy hết:
  1. đăng ký bằng invite;
  2. lưu key;
  3. tạo truyện từ 1 câu ở chế độ duyệt;
  4. thấy event realtime;
  5. đọc chương 1;
  6. bấm "Chương tiếp";
  7. steer khi đang chạy và khi đã dừng;
  8. dừng rồi tiếp;
  9. hết ngân sách → nâng ngân sách → chạy tiếp;
  10. xuất EPUB.
- [ ] Restart hoặc deploy server giữa chừng: truyện đang chạy và đang xếp hàng đều tự tiếp tục, không truyện nào thành `failed`.
- [ ] API key không xuất hiện ở bất kỳ đâu sau đây: DB dạng rõ, log, env của server hoặc worker, response API.
- [ ] Gate xanh:
  - `gofmt -l`, `go vet ./...`, `go test ./...`;
  - `-race` cho `internal/server/...` và `internal/entry/worker/...`;
  - `pnpm -C web typecheck lint test build`.
- [ ] Cài được PWA trên Android và iOS qua HTTPS.

## Rủi ro chính

| Rủi ro | Giảm thiểu |
|---|---|
| Sync upstream bị conflict | File upstream bị chạm: `cmd/ainovel-cli/main.go`, `.gitignore`, `go.mod`/`go.sum`, `README.md`. Mọi thứ khác nằm trong thư mục riêng của fork. Không sửa `ci.yml`, `.goreleaser.yml`, `Dockerfile`. Ghi chú trong `SYNC.md`. |
| SSRF hoặc thực thi lệnh qua config | Worker dựng config theo từng field. Provider lấy từ allowlist biên dịch sẵn. Không có `base_url`/`type`/`extra`/`roles`. Notify tắt. Decoder đặt `DisallowUnknownFields`. |
| Ngân sách không có tác dụng | Chỉ chọn model có giá; worker merge `pricing` vào registry trước khi chạy. |
| Nhiều worker ăn hết RAM/CPU | Giới hạn toàn server + mỗi user. Worker có `oom_score_adj=500`. Systemd đặt `MemoryMax`. |
| Mất master key → mất toàn bộ API key | Backup master key tách khỏi DB, có tài liệu. |
| Engine đổi chữ ký `host` khi sync | Worker dùng interface hẹp + golden test cho DTO. |
| Worker cùng UID với server | Không để secret trong env (server đọc từ file credential). Notify tắt nên không còn đường exec. UID riêng để sau MVP. |

## Câu hỏi mở

Không còn câu nào. Xem Validation Log.

## Validation Log

### Session — 2026-10-13

| Câu hỏi | Trả lời |
|---|---|
| Hạ tầng | Có sẵn VPS và domain trên Cloudflare → phase 6 deploy thẳng staging theo systemd + Caddy + Cloudflare Full (strict) |
| Tên app | **AI Novel** (manifest PWA, `<title>`) |
| Dependency | Đồng ý cả danh sách. Go: pgx v5, goose v3, x/crypto, x/time. Web: React 19, Vite 8, TanStack Router/Query/Virtual, Tailwind 4, shadcn/ui, react-markdown, vite-plugin-pwa, fontsource, Vitest |
| Giới hạn mặc định | Giữ nguyên: 4 job toàn server, 1 job/user, 20 truyện/user, trần $50/truyện (chỉnh qua env) |
| Design | Đã duyệt canvas https://claude.ai/artifact/EgXyM2GeFvdMeudAEDVoKC; bản sao ở `./design/`; token và map route ở phase 5 |

## Ranh giới khi chạy tự động

Agent tự làm hết phase 1–6 trên nhánh `feat/web-app` (tách từ `vi`), commit theo từng phase, **không push, không merge**. Các bước sau cần người, agent bỏ qua và liệt kê lại trong báo cáo cuối:

- Smoke test với API key LLM thật (phase 1 bước 8, phase 4 bước 7).
- Triển khai staging lên VPS, kiểm SSE qua Cloudflare, cài PWA trên điện thoại (phase 6 bước 4).
- Thử với 3 người dùng thật (phase 5).

Thay cho các bước đó, agent phải chạy kịch bản end-to-end bằng fake worker / provider giả cục bộ và trình duyệt headless.

Postgres cho test: `docker run -d --name ainovel-pg -e POSTGRES_PASSWORD=dev -p 55432:5432 postgres:17`, rồi đặt `AINOVEL_TEST_DATABASE_URL=postgres://postgres:dev@localhost:55432/postgres?sslmode=disable`.

Báo cáo cuối: `plans/261013-0642-web-app-serve-mode/reports/cook-final-report.md`.

## Red Team Review

### Session — 2026-10-13

**Findings:** 15 (15 accepted, 0 rejected; #10 chấp nhận có điều chỉnh)
**Severity breakdown:** 2 Critical, 9 High, 4 Medium
**Reviewers:** Security Adversary, Failure Mode Analyst, Assumption Destroyer, Scope & Complexity Critic. Mọi phát hiện đều có trích dẫn file:line. Hai lỗi Critical đã được kiểm lại trực tiếp trong code.

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | `waiting_review` lấy từ `HasAdvanceHold` (sai). Đúng là `panels_sidebar.go:235` | Critical | Accept | Phase 1, 4, 5 |
| 2 | `api_key_id` không ràng buộc chủ sở hữu → dùng được key người khác | Critical | Accept | Phase 3, 4 |
| 3 | Vòng đời worker khi engine dừng chưa định nghĩa → giữ slot, `/next` không có đường chạy | High | Accept | Phase 1, 4 |
| 4 | Gọi host blocking trên luồng đọc stdin → dừng/tắt bị kẹt | High | Accept | Phase 1 |
| 5 | Restart/deploy biến truyện thành `failed`/`paused`; race với flock; thay binary giữa chừng | High | Accept | Phase 4, 6 |
| 6 | Hết ngân sách là chết hẳn, không nâng được | High | Accept | Phase 1, 4, 5 |
| 7 | Model không có giá → cost $0 → ngân sách vô hiệu | High | Accept | Phase 1, 3, 4 |
| 8 | `init.config` map thẳng vào `bootstrap.Config` (SSRF, `notify.command`); rules đọc theo cwd | High | Accept | Phase 1 |
| 9 | `advance_mode` không thuộc `bootstrap.Config` | High | Accept | Phase 1, 4 |
| 10 | Worker cùng UID với server | High | Accept (modified): secret qua file thay vì env, notify tắt, `oom_score_adj`; UID riêng để sau | Phase 1, 2, 6 |
| 11 | Export/SSE trong process server có thể làm sập cả hệ thống | High | Accept | Phase 2, 4 |
| 12 | Wire format dựa vào struct không tag; event start/finish trùng id; DB lặp state của engine; SSE replay làm quá mức; `ReplayQueue(0)` dễ hỏng | Medium | Accept | Phase 1, 4, 5 |
| 13 | Steer khi đang dừng ghi đè `PendingSteer` | Medium | Accept | Phase 1 |
| 14 | XSS từ nội dung LLM; IP rate limit sai sau proxy; cookie `__Host-` hỏng ở dev; đổi mật khẩu không hỏi mật khẩu cũ | Medium | Accept | Phase 2, 3, 5, 6 |
| 15 | Phạm vi thừa (keyring nhiều version, benchmark, redact handler); thiếu dependency x/crypto, x/time; sửa file release của upstream; hai cách deploy; tiêu chí không đo được | Medium | Accept | Tất cả |

### Whole-Plan Consistency Sweep

Danh sách thay đổi quyết định:
- worker thoát khi engine dừng; action `start|resume|steer|advance`;
- `advance_mode` là field top-level của init;
- `llm` thay cho `config.providers`;
- có `pricing`;
- status có thêm `budget_exhausted`, `deleting`;
- bảng `books` có `pending_action`/`pending_text`, bỏ các cột đếm, FK kép;
- SSE không còn seq/`Last-Event-ID`/`reset`, ring 300 upsert theo id;
- secret qua `*_FILE`;
- một master key;
- deploy chỉ systemd;
- không sửa `ci.yml`/`.goreleaser.yml`/`Dockerfile`.

Đã viết lại toàn bộ `plan.md` và `phase-01`…`phase-06` theo danh sách trên. Đã grep tìm thuật ngữ cũ: `HasAdvanceHold` (chỉ còn trong vai trò lý do tạm dừng), `continue` (không còn là action hay lệnh), `advance_one`, `Last-Event-ID`, `unix_ms`, `AINOVEL_MASTER_KEYS`, `KillMode=control-group`, `ReplayQueue`. Báo cáo research đã sửa nhận định "exp.Run chỉ đọc".

Mâu thuẫn còn tồn đọng: không có.

<!-- slug: web-app-serve-mode -->
