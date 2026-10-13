---
phase: 4
title: "Kiểm tra key + chống lạm dụng khi mở đăng ký"
status: completed
priority: P1
effort: "1d"
dependencies: [3]
---

# Phase 4: Kiểm tra key + chống lạm dụng khi mở đăng ký

<!-- Red Team: phase mới theo phát hiện 1, 3b, 7, 8, 9 — 2026-10-13 -->

## Overview
Thêm `POST /api/keys/test`, chỉ kiểm key **đã lưu của chính user**, có giới hạn theo user, theo IP, theo ngày và giới hạn toàn cục. Response không chứa chi tiết thừa. Thêm ba lớp bảo vệ tài nguyên server khi đăng ký mở: chia lượt chạy theo thời gian, chặn tạo truyện khi đĩa sắp đầy, dọn tài khoản bỏ hoang. UI gọi endpoint này ở phase 6 và 8.

## Context
- Báo cáo red-team: `plans/261013-1650-web-ux-onboarding-providers/reports/from-red-team-to-planner-261013-1725-web-ux-plan-review-report.md` (phát hiện 1, 3, 7, 8, 9).
- Phase 1 đã có: `auth.NewLimiterPer(n, per)` (limiter theo cửa sổ tuỳ ý, sweep đúng theo cửa sổ), limiter signup theo IP/ngày, giới hạn tạo key và trần 20 key/user.
- Phase 3 đã có: `bootstrap.ClassifyProviderError(err) (kind, code)` với tập kind đóng.
- Hiện trạng:
  - Giải mã key chỉ có `DecryptForBook` (`internal/server/keys/repo.go:115-133`, comment "lối duy nhất lấy key rõ"). AAD = `secret.KeyAAD(userID, keyID)`.
  - `internal/server/keys/catalog.go:10` đã import `internal/entry/worker`, nên dùng `worker.LookupProvider` / `buildConfig` không tạo vòng import.
  - Bucket IP: `ipKey` chưa export (`internal/server/auth/handlers.go:166-174`, IPv6 gom theo /64). `httpx.ClientIPFrom` đã export.
  - Vòng dọn định kỳ: `internal/server/server.go:262-276` (`PurgeExpired`, `SweepLimiters`). Ngưỡng đĩa: `lowDiskBytes`, `diskFree` (`server.go:278-294`).
  - Lập lịch: `jobs/db.go:33` (`ORDER BY updated_at`), `supervisor.go:142-147` (trần toàn cục), `proc.go:284-300` (`ExitShutdown → paused`), lệnh `shutdown` hiện có (`supervisor.go:505`).
  - Giới hạn: `internal/server/config.go:125-155` (`loadLimits`).

## Requirements
- **`POST /api/keys/test`**
  - Request `api.TestKeyRequest{ KeyID string json:"key_id" }`. Không nhận key thô. `httpx.Decode` từ chối field lạ.
  - Giải mã bằng hàm mới `Repo.DecryptOwned(ctx, sessionUserID, keyID)`: `WHERE user_id = $1 AND id::text = $2`, AAD dựng từ **user của phiên**. Sửa comment ở `repo.go:115` thành "hai lối lấy key rõ, cả hai đều buộc chủ sở hữu". `key_id` của user khác trả 404 `key_not_found`, giống `handleDelete`.
  - Giới hạn, kiểm theo đúng thứ tự (bị chặn thì trả 429 `rate_limited` kèm `Retry-After`):
    1. Theo user: 3/phút và 20/ngày.
    2. Theo IP bucket: 60/ngày. Export `auth.IPKey(r)` từ `ipKey`.
    3. Toàn cục: semaphore 4 request đang chạy, không chờ. Hết chỗ thì trả 429 `busy`.
  - Mọi limiter mới phải được nối vào `SweepLimiters` (hoặc vòng sweep tương đương).
  - Gọi provider:
    - Dựng client bằng cùng factory với worker: export một hàm nhỏ trong `internal/entry/worker` bọc `buildConfig` + factory model của bootstrap. Không tự viết client litellm riêng.
    - Model: `recommended` của provider, fallback là model đầu tiên. Prompt "ping", `max_tokens` 1–8, context timeout 20s.
    - Gọi `Generate` **một lần**, không qua `llmretry` hay vòng agentcore.
  - Response 200 `api.TestKeyResult{ OK bool, Kind string, Code string, Message string, Model string }`:
    - `Kind` thuộc tập đóng của phase 3 (`ok` khi thành công).
    - `Code` chỉ có khi nằm trong allowlist (vd. `1113`).
    - `Message` là câu tiếng Việt do server viết theo `Kind`. **Không bao giờ** chép văn bản lỗi của provider (có thể chứa org id, last4, số dư).
  - Log phía server: user id, provider, kind, code. Không log key, không log body của provider.
  - Test chạy bằng `httptest.Server` giả provider. Base URL chỉ inject được qua tham số nội bộ dùng cho test, không có đường nào từ input.
- **Chia lượt chạy theo thời gian** (chặn việc giữ slot mãi, áp cho mọi truyện):
  - Env `AINOVEL_MAX_RUN_MINUTES`, mặc định 60. Giá trị 0 nghĩa là tắt.
  - Chỉ cắt khi đủ **cả ba** điều kiện:
    1. Lượt chạy đã quá N phút.
    2. Server đã hết slot toàn cục (`len(s.procs) >= MaxJobs`). Còn slot trống thì truyện đang chờ tự được chạy, không cần cắt.
    3. Có truyện `queued` của user khác **đủ điều kiện chạy ngay**: user đó chưa chạm `MaxJobsPerUser` và truyện không nằm trong `backoff`. Dùng đúng điều kiện `blocked` của `schedule()` (`supervisor.go:146-147`). Viết thành hàm thuần dùng chung, tránh hai nơi lệch nhau.
    <!-- Updated: Validation Session 1 - thêm điều kiện slot đầy + truyện chờ đủ điều kiện để tránh cắt vô ích, cắt lặp mỗi tick -->
  - Khi cắt:
    - Supervisor gửi `shutdown` và đánh dấu proc `sliced`.
    - `outcome()` với `sliced` đưa truyện về `queued`, `pending_action = resume`, `updated_at = now()`, tức xuống cuối hàng đợi (`jobs/db.go:33` sắp theo `updated_at`).
    - Truyện không bị tính là lỗi và không tăng `resume_failures` (`Failed = false` trong `finishResult`, `jobs/db.go:87`).
  - Chạy kiểm tra ở vòng lập lịch hiện có, không thêm goroutine riêng.
  - Test supervisor với fakeworker:
    - Đủ ba điều kiện thì cắt. Truyện bị cắt về `queued` và resume đúng chương.
    - Thiếu một trong ba thì không cắt: còn slot trống; truyện chờ là của cùng user; user của truyện chờ đã chạm `MaxJobsPerUser`.
- **Chặn khi đĩa sắp đầy:** `POST /api/books` trả 507 `disk_full` ("Máy chủ sắp hết dung lượng, thử lại sau") khi dung lượng trống < `lowDiskBytes` (1 GiB).
  - `diskFree` và `lowDiskBytes` nằm ở package `server` (`internal/server/diskfree_unix.go`, `server.go:279`), còn handler ở package `books`. Vì vậy `server.go` truyền vào `books` một hàm `func() (free uint64, ok bool)` đã bọc `diskFree(cfg.DataDir)`, qua constructor hoặc field.
  - `ok = false` (nền tảng không đo được, `diskfree_other.go`) thì không chặn.
  - Test inject hàm giả.
- **Dọn tài khoản bỏ hoang:** trong vòng dọn định kỳ, xoá user `created_at < now() - 30 ngày` mà không có `api_keys`, không có `books` và không có `sessions`. Một câu `DELETE … WHERE NOT EXISTS …`. Ghi log số dòng bị xoá. Có flow test.
- `make gen`.

## Related Code Files
- Create: `internal/server/keys/check.go`, `internal/server/keys/check_test.go`
- Modify: `internal/server/keys/handlers.go` (route test), `internal/server/keys/repo.go` (`DecryptOwned` + comment), `internal/server/api/account.go` (`TestKeyRequest`, `TestKeyResult`), `internal/server/auth/handlers.go` (export `IPKey`), `internal/entry/worker/config.go` (export hàm dựng model cho kiểm key)
- Modify: `internal/server/jobs/supervisor.go`, `proc.go`, `db.go` (chia lượt), `supervisor_test.go`; `internal/server/config.go` (`AINOVEL_MAX_RUN_MINUTES`)
- Modify: `internal/server/books/handlers.go` (chặn đĩa), `internal/server/server.go` (wiring route, sweep limiter, dọn tài khoản)
- Modify test: `internal/server/books_flow_test.go` hoặc `auth_flow_test.go` (dọn tài khoản, 507)
- Generated: `web/src/api/types.gen.ts`

## Implementation Steps
1. Đọc `keys/{repo,handlers,catalog}.go`, `auth/ratelimit.go` (đã sửa ở phase 1), `jobs/{supervisor,proc,db}.go`, `server.go:240-300`, `config.go:120-160`.
2. `DecryptOwned` + test chặn chủ sở hữu.
3. `check.go`: limiter, semaphore, dựng client, gọi một lần, map sang `TestKeyResult`. Test:
   - provider giả trả OK → `ok`
   - 401 → `auth_failed`
   - 429 kèm code `"1113"` → `plan_mismatch` / `1113`
   - message của provider có chuỗi "org-XXXX" → không xuất hiện trong response
   - đúng 1 request tới provider
   - lần thứ 4 trong một phút → 429
   - `key_id` của người khác → 404
4. Chia lượt + test.
5. Chặn đĩa + dọn tài khoản + test.
6. `make gen`, chạy verify.

## Tests / Verify
```bash
go build ./... && go vet ./internal/server/... ./internal/entry/worker/...
go test ./internal/server/... ./internal/entry/worker/...
make gen
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] `POST /api/keys/test` chỉ nhận `key_id` thuộc chính user. Key của người khác → 404. Gửi body có `api_key` → 400.
- [x] Có giới hạn theo user (phút và ngày), theo IP/ngày và đồng thời toàn cục. Có test cho mỗi giới hạn.
- [x] Response chỉ chứa kind/code đóng và message do server viết. Không có văn bản của provider, không có key.
- [x] Mỗi lần kiểm gọi provider đúng 1 lần, timeout 20s.
- [x] Lượt chạy quá `AINOVEL_MAX_RUN_MINUTES` nhường slot khi server hết slot và có truyện của user khác đủ điều kiện chạy. Truyện xuống cuối hàng và không bị tính là lỗi. Có test âm cho từng điều kiện thiếu.
- [x] Đĩa dưới ngưỡng → tạo truyện trả 507.
- [x] Tài khoản rỗng quá 30 ngày bị dọn. Tài khoản có key, có truyện hoặc có session thì không bị xoá.

## Kết quả (2026-10-13)
Đã xong, verify pass. Báo cáo: [cook](./reports/cook-cook-p04-report.md), [review](./reports/from-code-reviewer-to-cook-p04-review-report.md) (7/10, đã sửa 2 High + 2 Medium + 1 Low).

Lệch so với spec (đều có chủ ý):
- Lượt bị cắt giữ `pending_action`/`pending_text` như khi drain (`COALESCE(pending_action, 'resume')`), không ép thành `resume`. Ép thành `resume` thì truyện bị cắt trước khi lưu tiến độ sẽ thành `completed` rỗng và mất prompt.
- Khi cắt, mọi truyện `queued` khác của cùng user cũng bị đẩy xuống cuối hàng. Nếu không, user vừa được nhả trần sẽ lấy lại chính slot đó.
- Bấm tạm dừng đúng lúc đang cắt: `handlePause` chờ worker thoát rồi đổi `queued` thành `paused`. Worker thoát với `stop_reason = user` thì giữ `paused`.
- Dọn tài khoản bỏ hoang không xoá admin.
- Có sửa thêm các file test helper `internal/server/testclient_test.go` (`newTestEnvOpts`), `internal/server/config_test.go` và `internal/entry/worker/config_test.go`.

## Risk Assessment
- Limiter nằm trong bộ nhớ nên mất khi restart, và đầy 100k khoá thì đóng an toàn (`ratelimit.go:36-55`). Lớp chặn chính là rule rate limit của Cloudflare cho `/api/auth/signup` và `/api/keys*`, bắt buộc ở bước deploy (phase 12).
- Chia lượt có thể cắt giữa chương. Worker shutdown theo đường hiện có (đã dùng khi deploy và drain), nên resume tiếp được. Test phải chứng minh truyện resume đúng chương.
- Mỗi lần kiểm key tốn một chút quota của user. Đã giới hạn 20 lần/ngày.
- Dọn tài khoản là thao tác xoá không hoàn tác. Điều kiện chặt (không key, không truyện, không session, quá 30 ngày) và có test âm.
