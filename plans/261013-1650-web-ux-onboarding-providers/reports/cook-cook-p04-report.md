# Cook phase 4: kiểm tra key + chống lạm dụng khi mở đăng ký

Ngày: 2026-10-13 | Phase: [phase-04](../phase-04-backend-key-check-and-abuse-limits.md) | Review: [from-code-reviewer-to-cook-p04-review-report.md](./from-code-reviewer-to-cook-p04-review-report.md). Review chấm 7/10; đã sửa 2 High, 2 Medium và 1 Low, tất cả đều nằm ở phần chia lượt.

## Đã làm
- **`POST /api/keys/test`** (`internal/server/keys/check.go`, mới):
  - Chỉ nhận `{key_id}`. Body có field lạ (`api_key`) trả 400. Key của user khác trả 404 `key_not_found`.
  - Giới hạn kiểm theo thứ tự: user 3/phút → user 20/ngày → IP 60/ngày (`auth.IPKey`) → semaphore 4, không chờ. Quá giới hạn trả 429 `rate_limited`; hết chỗ trả 429 `busy`. Mọi 429 đều có `Retry-After`. Các limiter mới đã nối vào `SweepLimiters`.
  - Model dựng bằng `worker.NewProbeModel` (`buildConfig` + `bootstrap.NewModelSet`), cùng allowlist và BaseURL cố định với worker. Dùng model "Đề xuất", không có thì lấy model đầu tiên. Gửi "ping", `max_tokens` 8, timeout 20s, gọi `Generate` đúng 1 lần.
  - Response `TestKeyResult{ok, kind, code, message, model}`: `kind` thuộc tập đóng, `code` chỉ lấy mã trong allowlist, `message` do server viết. Quá thời gian thì `kind = network`.
  - Log chỉ ghi user id, provider, kind, code.
- **`keys.Repo.DecryptOwned`**: lọc `WHERE user_id = $1 AND id::text = $2`, AAD dựng từ user của phiên. Đã sửa comment về hai lối lấy key rõ.
- **API**: thêm `TestKeyRequest`, `TestKeyResult` và các hằng `KeyCheck*`; đã chạy `make gen`.
- **Chia lượt** (`jobs`):
  - `Config.MaxRun` lấy từ `AINOVEL_MAX_RUN_MINUTES`: mặc định 60, 0 là tắt, trần 10080, giá trị âm hoặc không phải số thì lỗi.
  - Hàm `blockedLocked` dùng chung cho `schedule()` và `sliceVictimLocked`.
  - Chỉ cắt khi đủ ba điều kiện: hết slot, lượt chạy quá `MaxRun`, và có truyện của user khác chạy được ngay. Chọn lượt chạy lâu nhất, mỗi lúc chỉ cắt một worker.
  - Khi cắt: đẩy mọi truyện `queued` của user đó xuống cuối hàng, đánh dấu `sliced`, rồi `terminate`.
  - `outcome()` với lượt bị cắt (thoát bằng shutdown hoặc bị SIGKILL) đưa truyện về `queued`, giữ pending như khi drain, không tính lỗi. Nếu worker thoát với `stop_reason = user` thì giữ `paused`.
- **`handlePause`**: nếu worker đang bị cắt thì `StopAndWait` rồi đổi `queued` thành `paused`.
- **Chặn đĩa**: `books.Deps.DiskFree` và `Limits.MinFreeDiskBytes = lowDiskBytes` (1 GiB). Dưới ngưỡng thì `POST /api/books` trả 507 `disk_full`; không đo được thì không chặn. `server.go` truyền `s.diskFree` vào, test thay được qua `Options.DiskFree`. `/healthz` dùng chung hàm này.
- **Dọn tài khoản bỏ hoang**:
  - `auth.PurgeAbandonedUsers` là một câu `DELETE … NOT EXISTS`: xoá tài khoản tạo quá 30 ngày, không có key, truyện hay session, và không phải admin.
  - Gọi trong `Server.cleanup()` theo thứ tự PurgeExpired → purge → sweep, có log số tài khoản bị xoá.

## Tests mới
- **keys** (`check_test.go`, chạy trên DB thật, provider giả qua httptest, đi qua factory bootstrap thật):
  - Các kết quả ok, 401 → `auth_failed`, 429 + `1113` → `plan_mismatch`/`1113`, 429 thường → `rate_limited`.
  - Response không chứa `org-SECRET42`, số dư, last4 hay key.
  - Đúng 1 request tới provider, và provider nhận đúng key đã lưu.
  - Quá thời gian → `network`, chỉ 1 lần gọi.
  - Quyền sở hữu: 404, 400 khi gửi `api_key`, 401 khi chưa đăng nhập, request bị từ chối thì 0 lần gọi provider.
  - Giới hạn user theo phút (lần thứ 4 → 429 kèm Retry-After) và theo ngày; giới hạn theo IP (user khác cùng IP vẫn bị chặn).
  - Lần thứ 5 đồng thời → 429 `busy`; khi nhả chỗ thì kiểm được tiếp.
  - Mọi kind đều có message; model kiểm là model "Đề xuất".
- **worker**: `TestNewProbeModelUsesAllowlist`.
- **jobs**:
  - Unit: `TestOutcomeSliced`, `TestSliceVictimConditions` (slot, MaxRun, cùng user, user chạm trần, backoff, chọn lượt lâu nhất, chỉ một lượt cắt).
  - Integration với fakeworker:
    - Cắt và nhường slot: đúng chuỗi status; lượt sau viết tiếp đúng chương; mỗi chương viết xong đúng một lần; `resume_failures = 0`.
    - Không cắt khi truyện chờ là của cùng user, khi user của truyện chờ đã chạm trần, hoặc khi còn slot trống.
    - Truyện chờ của cùng user bị đẩy xuống cuối hàng. Bỏ phần đẩy hàng thì test này đỏ, đã thử.
- **server**:
  - `TestBookCreateBlockedWhenDiskLow`: 507, không tạo truyện; không đo được thì 201; đủ dung lượng thì 201.
  - `TestCleanupAbandonedAccounts`: xoá tài khoản rỗng và tài khoản chỉ còn session hết hạn; giữ tài khoản mới, admin, có key, có truyện, có session. Log có `count=2`.
  - `TestLoadConfigMaxRunMinutes` và các ca lỗi của env mới.

## Verify (với `AINOVEL_TEST_DATABASE_URL` của container ainovel-testpg)
- `go build ./... && go vet ./internal/...`: pass; gofmt sạch.
- `go test -count=1 ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...`: pass, các test DB không bị skip.
- `-race` cho `jobs`, `keys`, `server`, `books`, `worker`: pass.
- `make gen`: đã sinh lại; chạy lại không ra diff.
- `pnpm -C web typecheck && lint && test (63/63) && build`: pass.

## Lệch so với phase file
- Lượt bị cắt giữ pending như khi drain, không ép thành `resume` (review M2: tránh mất prompt và truyện `completed` rỗng).
- Khi cắt có đẩy truyện chờ của cùng user xuống cuối hàng (review H1). Gợi ý "không cắt" của reviewer vẫn để user khác chờ qua nhiều truyện, trái tiêu chí "không giữ slot quá N phút".
- Có sửa `books/handlers.go` `handlePause` (review H2), dù file này phase 4 chỉ ghi cho phần chặn đĩa.
- Dọn tài khoản không xoá admin.
- Có sửa thêm các file test helper: `testclient_test.go`, `config_test.go`, `worker/config_test.go`.

## Lưu ý / chưa làm
- **Flaky có sẵn, không do phase 4:** khoảng 1/6 lần chạy `./internal/server/`, một test flow (`TestBookAuthorization` hoặc `TestBookValidationAndLimits`) fail với lỗi `TempDir RemoveAll: directory not empty`.
  - Nguyên nhân: `models.StartPricingRefresh` (`internal/models/pricing.go:74`) chạy goroutine tải giá qua mạng rồi ghi cache vào `DataDir/cache`, không gắn với context.
  - Chạy lặp từng test thì 15/15 pass.
  - Đề xuất sửa ở phase 12 hoặc một việc riêng: cho refresh nhận ctx, hoặc tắt refresh trong test.
- Request bị 429 `busy` vẫn trừ lượt user và IP. Đây là đúng thứ tự trong spec.
- Dọn tài khoản không xoá thư mục home trên đĩa. Tài khoản bỏ hoang không có truyện nên thư mục thường rỗng hoặc không tồn tại.
- Phase 12 cần ghi vào docs: `POST /api/keys/test`, `AINOVEL_MAX_RUN_MINUTES`, 507 `disk_full`, dọn tài khoản sau 30 ngày, rule Cloudflare cho `/api/keys*`.
- Không commit (theo ràng buộc). Không chạy docs-manager vì docs thuộc phase 12. Không viết journal.

## Câu hỏi còn mở
- `max_tokens = 8` với model có thinking (`gemini-2.5-flash`, `gpt-5-mini`…): provider thật có trả 400 hoặc response rỗng không? Nếu có, key hợp lệ sẽ bị báo `unknown`. Cần kiểm bằng key thật ở phase 12.

Status: DONE_WITH_CONCERNS
