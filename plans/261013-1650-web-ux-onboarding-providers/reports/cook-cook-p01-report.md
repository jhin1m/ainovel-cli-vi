# Cook phase 1: bỏ mã mời + thêm key không cần mật khẩu

Ngày: 2026-10-13 | Phase: [phase-01](../phase-01-backend-open-signup-and-key-auth.md) | Review: [from-code-reviewer-to-cook-p01-review-report.md](./from-code-reviewer-to-cook-p01-review-report.md) (8.5/10, DONE_WITH_CONCERNS → đã sửa M1, M2, L2, L5)

## Đã làm
- API: bỏ `SignupRequest.invite_code`, `CreateAPIKeyRequest.current_password`; `make gen` cập nhật `types.gen.ts`.
- Auth: bỏ toàn bộ logic mã mời (`NormalizeInviteCode`, `HashInviteCode`, transaction trừ lượt). Signup insert user trực tiếp. Thêm `signupLimiter` 10/ngày/IP (tính sau validate, trước argon2) cạnh limiter 30/phút cũ. Giữ `VerifyUserPassword` + `userLimiter` cho đổi mật khẩu.
- `auth.NewLimiterPer(n, per)`; `NewLimiter` gọi lại nó; sweep theo `max(10 phút, per)`. Limiter cửa sổ dài khi map đầy (100k khoá) thì bỏ khoá cũ nhất thay vì từ chối (fix review M1).
- `httpx.TooManyToday`: 429 `rate_limited` với message "Đã đạt giới hạn trong ngày…" (fix M2).
- Keys: bỏ re-auth; limiter 30/ngày/user; trần 20 key trong `Repo.Create(..., maxKeys)` bằng transaction khoá dòng user `FOR NO KEY UPDATE` → 409 `key_limit`. `keys.Handlers.SweepLimiters` nối vào vòng sweep hằng giờ ở `server.go`.
- CLI: xoá `internal/server/admin/`; `serve` từ chối mọi lệnh con (exit 2) trước khi đọc cấu hình; help chỉ cho `help`/`-h`/`--help`.
- Migration `0004_drop_invite_codes.sql`: Up `DROP TABLE IF EXISTS`, Down dựng lại bảng y hệt 0002.
- Web: bỏ ô mã mời (signup), ô mật khẩu (add-key dialog); dọn `messages.ts`; thêm `keys.limit`.
- Tests: `TestLimiterPerDay`, `TestLimiterFullMap`, `TestSignupDailyLimitPerIP`, `TestKeyCreateLimits`, `TestCommandRejectsSubcommands`, `TestMigration0004DropInviteCodesUpDown`; helper `signup(email, password)`; bỏ test sai mật khẩu khi thêm key, test mã mời hết lượt/hết hạn. Giữ test đổi mật khẩu sai → 403.

## Khác plan (có chủ ý)
- Không thêm `CountByUser`; đếm + insert trong cùng transaction có khoá để chống race (reviewer xác nhận an toàn hơn).
- Subtitle trang đăng ký đổi thành "Chỉ cần email, mật khẩu và tên hiển thị."; `goSignup` → "Đăng ký".

## Verify (đều pass)
- `go build ./... && go vet ./internal/server/...`
- `go test -count=1 ./internal/server/... ./internal/entry/worker/...` với Postgres 18 local tạm (`AINOVEL_TEST_DATABASE_URL` trỏ cluster trong scratchpad, đã tắt sau khi chạy) → flow test chạy thật, không skip. `-race` cho `auth`, `httpx` pass.
- `make gen` → diff `types.gen.ts` chỉ bỏ 2 field + comment.
- `pnpm -C web typecheck && lint && test (63/63) && build` pass.
- Grep `invite|mã mời` trong `internal/server web/src` (trừ 0002/0004/db_test.go) rỗng.
- `ainovel-cli serve invite create` → `serve: lệnh con "invite create" không được hỗ trợ`, exit 2.

## Chưa làm / để phase sau
- Docs (`docs/web-server.md`, `docs/architecture.md`) vẫn nhắc mã mời → phase 12.
- Review low chưa sửa: L1 (lượt ngày bị tiêu cả khi `email_taken`/đã đủ 20 key), L3 (chưa có test song song cho trần 20 key), L4 (số 20 lặp ở Go + web), L6 (test migration chưa down về 0). Không ảnh hưởng AC.
- Không commit (theo ràng buộc).

## Câu hỏi còn mở
- Reviewer hỏi 10 signup/ngày/IP có quá chặt với IPv4 dùng chung của nhà mạng di động VN (CGNAT) không. Con số này user đã chốt khi validate nên giữ nguyên; nếu log cho thấy người thật bị chặn thì cân nhắc nâng ở đợt sau.

Status: DONE
