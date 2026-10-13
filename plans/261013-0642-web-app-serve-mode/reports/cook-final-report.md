# Báo cáo cuối: bản web `ainovel-cli serve`

Ngày: 2026-10-13 · Nhánh: `feat/web-app` (tách từ `vi`, **chưa push/merge**) · Chế độ: `/ak:cook --auto`

## Kết quả

Cả 6 phase đã làm xong, mỗi phase có commit riêng, đã đánh dấu `completed` trong file phase và `plan.md`.

| Phase | Commit chính | Commit sửa sau review/test |
|---|---|---|
| — tài liệu thiết kế | 025fc74 | |
| 1 Worker stdio | bc74c5c | 1c09ffa, a214cc8 |
| 2 Server foundation | 1e1fe15 | 612799f, e40b7c8 |
| 3 Auth + BYOK | 6da0b8e | 5527b86 |
| 4 Supervisor, Books API, SSE | b570fad | 0ffd2c4 |
| 5 Web SPA | ec7e79d | 427ea1e |
| 6 PWA, deploy, docs | 2ac8c9f | 0f697cd |
| docs/journal | e2aed41, 409b66b | |

Mỗi phase có `code-reviewer` + tester riêng; report trong `reports/`.

## Gate (chạy lần cuối trên HEAD)

| Lệnh | Kết quả |
|---|---|
| `gofmt -l .` | rỗng |
| `go vet ./...` | sạch |
| `go test ./...` | xanh |
| `AINOVEL_TEST_DATABASE_URL=postgres://postgres:dev@localhost:55432/postgres?sslmode=disable go test -race -count=1 ./internal/server/... ./internal/entry/worker/...` | xanh (10 package) |
| `pnpm -C web typecheck` / `lint` / `test` / `build` | xanh (vitest 63/63) |
| `make build` | ra `./ainovel-cli`; `serve --help` exit 0 |
| Chạy binary ở thư mục khác (không có `web/`) với `serve` | `/` 200, `/manifest.webmanifest` 200, `/healthz` 200, `/api/me` 401; SIGTERM tắt êm |
| E2E trình duyệt headless (server dev + fake worker, 360px/1280px, sáng/tối) | luồng chính PASS, không lỗi JS, không tràn ngang |
| File deploy qua docker (reviewer) | `caddy adapt`, shellcheck, actionlint, `systemd-analyze verify` hợp lệ |

## Bug thật được bắt và sửa trong quá trình

- PATCH truyện luôn 500 (kiểu tham số SQL `$3` mâu thuẫn).
- Xoá truyện treo vĩnh viễn khi claim lỗi đúng lúc xoá; spawn lỗi không đóng `done`.
- Lượt lỗi xoá mất `pending_action` → truyện 0 chương thành `completed`.
- Drain làm `/next` chạy lặp (viết chương chưa duyệt, tốn tiền user); steer áp hai lần.
- Lượt `start` bị gián đoạn chạy lại → tạo lại truyện/`failed` (worker giờ thử Resume trước).
- Abort trước khi worker ready bị mất; steer báo 202 nhưng bị mất (nay 503 `worker_starting`).
- Export trả bản cũ sau khi viết lại chương; export tạo lại thư mục mồ côi khi xoá.
- `last_error` lộ stderr thô; prompt hiện như steer đang chờ; stderr log không giới hạn.
- SPA: SSE reconnect vô hạn khi 401/404, dữ liệu truyện cũ sau đổi trạng thái, render lại cả màn mỗi frame, dialog mất focus, CTA chữ tàng hình.
- Deploy: không lấy được TLS sau Cloudflare, worker OOM làm restart cả service, hướng dẫn `go.mod` kéo dependency đòi Go 1.26, timeout export vượt proxy.

## Quyết định ngoài plan (chọn phương án an toàn nhất)

- Worker action `steer` khi dừng gọi `Continue(merge(PendingSteer, text))` (giống TUI, một lần phán quyết, đúng cả với truyện đã xong).
- Không sửa `internal/host`: `stop_reason` suy từ tín hiệu cấu trúc; thêm `pending_steer` → `paused`. Rủi ro còn: lỗi gate ở review mode có thể báo nhầm `review_wait`.
- Worker poll snapshot 3s; chỉ `SetAdvanceMode` khi khác; job hàng đợi bị bỏ khi stopping.
- Action `start` thử `Resume()` trước; thư mục đã có tiến độ thì khôi phục.
- Dependency ghim để giữ `go 1.25.5`: pgx v5.11.0, goose v3.27.0, x/crypto v0.55.0, x/time v0.15.0 (SYNC.md hướng dẫn giữ khi merge upstream).
- `0001_init.sql` tạo extension `citext`. Type REST đặt trong `internal/server/api` cho tygo.
- `waiting_review` giữ nguyên khi khởi động (spec ghi coi là interrupted).
- `resume_failures` theo bảng spec (tăng khi lượt lỗi, reset khi lượt sạch); ≥3 lần lúc khởi động → `failed`, thông báo viết trung tính.
- Lượt lỗi giữ `pending_action`; chỉ lượt sạch mới xoá. Khi drain, worker tự dừng vẫn được ghi kết quả.
- DELETE trả 202, xoá nền (chờ export đang chạy). Thêm PATCH `api_key_id` (cùng provider).
- PATCH `budget_usd` khi `queued`/`running` → 409 `book_running` (worker chỉ đọc ngân sách lúc init).
- Fake worker + `AINOVEL_DEV_WORKER_BIN` (chỉ dev) cho test/e2e.
- Header IP: `AINOVEL_CLIENT_IP_HEADER` (X-Forwarded-For | CF-Connecting-IP).
- Export timeout 80s (dưới 90s của Caddy, 100s của Cloudflare).
- SPA: TypeScript 5.9.3 (typescript-eslint chưa nhận 6.x); font Be Vietnam Pro bản không variable; không dùng shadcn (component Tailwind tự viết); bảng màu tối tự suy ra; trần ngân sách 50 US$ là hằng số client (server chưa expose `AINOVEL_MAX_BOOK_BUDGET_USD`); reader bỏ ảnh, link chỉ http/https; stream giữ 200k ký tự, timeline 2000 event.
- PWA `registerType: prompt` + kiểm bản mới mỗi 60 phút; không runtime cache.
- Deploy: Cloudflare Origin Certificate; `OOMPolicy=continue`; backup bằng `pg_dump` + tar (spec ghi restic), docs yêu cầu đẩy bản sao ra ngoài VPS; Postgres ≥13.
- `serve --help`, `serve invite [create] --help` in cách dùng không cần cấu hình.

## Bước thủ công còn lại

1. Smoke worker + server với API key LLM thật: `review_wait` → `/next`, steer, pause/resume, hết ngân sách → nâng → chạy tiếp, SIGTERM giữa chừng rồi khởi động lại, export EPUB.
2. Kiểm danh sách model chọn lọc (`internal/server/keys/models.go`) với API thật.
3. Deploy staging VPS theo `docs/web-server.md`: Origin Cert, systemd, Caddy, `systemctl restart` không làm truyện nào `failed`, SSE qua Cloudflare >5 phút, thử OOM một worker.
4. Push nhánh để CI `web.yml` chạy lần đầu (pnpm/action-setup đọc `packageManager`).
5. Cài PWA trên Android/iOS, Lighthouse installable, SSE khi service worker đang active.
6. Thử với 3 người dùng chưa từng dùng CLI; đo INP khi stream chương ~10k chữ với CPU throttle 4x.
7. Diễn tập backup/restore và đẩy bản backup ra ngoài VPS.
8. Review rồi merge `feat/web-app` vào `vi` (chưa làm theo ranh giới tự động).

## Ghi chú review chưa sửa (Low, chấp nhận cho MVP)

- `/healthz` public lộ version; SPA không có ETag; không có ReadTimeout cho body.
- `.gitignore` dòng `tasks` match mọi thư mục tên `tasks`.
- `snapshot.current_chapter` có thể lớn hơn tổng chương khi truyện xong (thấy với fake worker; web đã clamp).
- So sánh `id::text` không dùng index khoá chính; bảng hub chỉ gỡ khi xoá truyện; race nhỏ kiểm-rồi-ghi ở giới hạn số truyện.
- Bản nháp stream bắt đầu giữa câu sau reconnect (server không phát lại stream).
- Icon agent chưa có; `minimumReleaseAgeExclude` (22 gói) nên gỡ khi các bản đủ tuổi.
- Export dạng blob và phím mũi tên ở tabs chưa có test tự động; 503 `worker_starting` chưa có test HTTP riêng.

## Câu hỏi còn mở

- Có expose `AINOVEL_MAX_BOOK_BUDGET_USD` cho client không?
- Có tách đếm "server sập" và "worker lỗi" thành hai cột không?
- `waiting_review` lúc khởi động: giữ nguyên (hiện tại) hay coi là interrupted (spec)?
- Backup: thêm restic tuỳ chọn hay giữ tar?
