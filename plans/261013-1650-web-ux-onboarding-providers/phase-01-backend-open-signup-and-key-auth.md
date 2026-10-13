---
phase: 1
title: "Bỏ mã mời + thêm key không cần mật khẩu"
status: completed
priority: P1
effort: "0.75d"
dependencies: []
---

# Phase 1: Bỏ mã mời + thêm key không cần mật khẩu

## Overview
Đăng ký tự do: bỏ mã mời khỏi API, form, CLI `serve invite`, và drop bảng `invite_codes` bằng migration có Down. `POST /api/keys` không đòi `current_password` nữa. Vì hai thay đổi này mở đường lạm dụng, phase này thêm luôn hai lớp chặn: giới hạn signup theo IP/ngày, và giới hạn tạo key kèm trần số key. Phase có chạm nhẹ web (signup form, add-key dialog) để `types.gen.ts` mới vẫn typecheck.

<!-- Red Team: thêm giới hạn signup/tạo key, migration có Down, sửa rủi ro và rollback theo phát hiện 6, 9, 10 — 2026-10-13 -->

## Context
- Brainstorm mục 1–2: `plans/reports/brainstorm-261013-1634-web-app-ux-onboarding-zai-report.md`
- Red-team (phát hiện 6, 9, 10): `plans/261013-1650-web-ux-onboarding-providers/reports/from-red-team-to-planner-261013-1725-web-ux-plan-review-report.md`
- Mật khẩu khi thêm key chỉ dùng để xác thực lại. Key được mã hoá AES-GCM bằng master key của server, không phụ thuộc mật khẩu, nên bỏ bước này không ảnh hưởng mã hoá. CSRF đã có (`internal/server/server.go:154-161`: CrossOriginProtection + RequireJSON).
- **Throttle duy nhất của `POST /api/keys` hiện nằm trong `VerifyUserPassword`** (`userLimiter` 5/phút, `internal/server/auth/handlers.go:118-121`). Bỏ lời gọi này là mất throttle. `Repo.Create` không đếm số key (`internal/server/keys/repo.go:64-81`).
- Signup chỉ bị chặn bởi limiter IP 30/phút, dùng chung với login (`auth/handlers.go:47,176-181,212-216`). IPv6 gom theo /64 (`ipKey`, `:166-174`).
- `auth.Limiter` (`internal/server/auth/ratelimit.go`): token bucket theo phút. Sweep xoá entry nhàn rỗi quá 10 phút (`:63-70`), nên chưa làm được cửa sổ theo ngày.
- Migration: goose v3, embed `internal/server/db/migrations/*.sql`, tự chạy khi `serve` khởi động (`internal/server/db/db.go:44-62`), có advisory lock. Repo có quy ước `-- +goose Down`. Phần Down của `0002_auth.sql:46-48` chạy `DROP TABLE invite_codes;` không có `IF EXISTS`. Không có FK nào trỏ tới `invite_codes`.
- `httpx.Decode` từ chối field lạ (`internal/server/httpx/json.go:83`).

## Requirements
- `POST /api/auth/signup` nhận `email`, `password`, `display_name`. Không còn `invite_code`, không còn lỗi `invite_required`/`invite_invalid`.
- **Limiter theo cửa sổ tuỳ ý:** thêm `auth.NewLimiterPer(n int, per time.Duration)`. `NewLimiter(perMinute)` giữ nguyên và gọi `NewLimiterPer(perMinute, time.Minute)`. Entry chỉ bị sweep khi nhàn rỗi lâu hơn `max(10 phút, per)`, để limiter theo ngày không bị reset sau 10 phút. Có unit test.
- **Signup:** giữ limiter 30/phút cũ, thêm limiter riêng 10 lần/ngày theo `ipKey` (chỉ cho signup, không cho login). Bị chặn thì trả 429 `rate_limited` như các limiter hiện có. Giữ mọi bước validate email/mật khẩu hiện có.
- `ainovel-cli serve invite ...` bị bỏ. `serve` kèm lệnh con lạ thì báo lỗi rõ (không còn liệt kê `invite create`).
- **Migration `0004_drop_invite_codes.sql`:**
  - `-- +goose Up`: `DROP TABLE IF EXISTS invite_codes;`
  - `-- +goose Down`: dựng lại bảng y hệt `0002_auth.sql:22-28`, để goose down qua 0002 vẫn chạy được.
  - Không sửa `0002_auth.sql`, vì DB đã chạy nó rồi.
- **`POST /api/keys`:**
  - Bỏ `current_password` khỏi request và handler.
  - Thêm limiter riêng cho tạo key: 30 lần/ngày/user (`NewLimiterPer(30, 24h)`).
  - Thêm trần `maxKeysPerUser = 20`: đếm trước khi insert, vượt trần thì trả 409 `key_limit` ("Đã đạt tối đa 20 key. Xoá key không dùng rồi thử lại.").
  - **Giữ** dependency `auth` của `keys.Handlers`, vì mọi route đều dùng `h.auth.RequireUser` (`keys/handlers.go:40-43`).
  - Nối limiter mới vào `SweepLimiters` (`internal/server/server.go:271`).
- `POST /api/me/password` giữ yêu cầu mật khẩu cũ. **Giữ** `Service.VerifyUserPassword` và `userLimiter`, vì đổi mật khẩu vẫn gọi hàm này (`auth/handlers.go:372`). Chỉ sửa comment "dùng khi thêm API key" ở `:116` thành "dùng khi đổi mật khẩu". <!-- Updated: Validation Session 1 - giữ VerifyUserPassword cho đổi mật khẩu -->
- Web: form đăng ký bỏ ô mã mời; dialog thêm key bỏ ô mật khẩu; xoá các chuỗi không còn dùng trong `messages.ts`; thêm chuỗi cho lỗi `key_limit`.

## Related Code Files
- Modify: `internal/server/api/account.go` (bỏ `SignupRequest.InviteCode`, `CreateAPIKeyRequest.CurrentPassword`, sửa comment "Phải nhập lại mật khẩu")
- Modify: `internal/server/auth/handlers.go` (khối ~L195–267: `NormalizeInviteCode`, `HashInviteCode`, transaction trừ lượt mời trong signup; limiter signup theo ngày)
- Modify: `internal/server/auth/ratelimit.go` (`NewLimiterPer`, sweep theo cửa sổ) + test
- Modify: `internal/server/auth/password.go` (comment đầu file nhắc invite code)
- Modify: `internal/server/keys/handlers.go` (~L89–92: bỏ `VerifyUserPassword`, thêm limiter + trần), `internal/server/keys/repo.go` (`CountByUser`), `internal/server/server.go` (sweep limiter mới)
- Modify: `internal/server/cmd.go` (help text L20, nhánh `invite create` L51–54, hàm `inviteCreate` ~L83+)
- Delete: `internal/server/admin/invite.go` (package `admin` chỉ có file này → xoá thư mục)
- Create: `internal/server/db/migrations/0004_drop_invite_codes.sql`
- Modify tests: `internal/server/cmd_test.go`, `internal/server/testclient_test.go` (`invite()` L77, `signup(invite, …)` L151), `internal/server/auth_flow_test.go`, `internal/server/books_flow_test.go` (body `/api/keys` có `current_password` L97), `internal/server/auth/auth_unit_test.go`, `internal/server/db/db_test.go` (test Up/Down của 0004)
- Generated: `web/src/api/types.gen.ts` (chạy `make gen`)
- Modify web: `web/src/features/auth/signup-form.tsx`, `web/src/features/keys/add-key-dialog.tsx` (L32 payload, L79–82 ô mật khẩu), `web/src/lib/messages.ts` (khối signup invite và `keys.currentPassword*`; giữ `account.currentPassword`; thêm `keys.limit`)
- Bỏ qua: `internal/tools/novel_context_test.go` có chữ "invite" nhưng không liên quan.

## Implementation Steps
1. `grep -rn -i invite internal/server web/src docs` để có danh sách đầy đủ trước khi sửa.
2. API types: bỏ hai field. Chạy `make gen`.
3. `NewLimiterPer` + sweep theo cửa sổ + test (entry theo ngày còn nguyên sau 11 phút nhàn rỗi).
4. Auth handler: bỏ kiểm tra và trừ mã mời. Signup tạo user như cũ, giữ transaction nếu vẫn cần cho insert user + session. Thêm limiter signup theo ngày.
5. Xoá `admin/invite.go`. Gỡ import `admin` ở `cmd.go` và `testclient_test.go`. Viết lại help của `serve`.
6. Migration 0004 có Up và Down như trên.
7. Keys handler: bỏ verify mật khẩu, thêm limiter + trần, giữ validate provider/label/key.
8. Tests:
   - Helper `signup(email, password)`. Bỏ test sai mật khẩu khi thêm key (`auth_flow_test.go` L45).
   - Thêm assert: signup không cần mã mời → 201/200; thêm key không có `current_password` → thành công; key thứ 21 → 409 `key_limit`; signup thứ 11 từ cùng IP trong ngày → 429.
   - Test migration đặt trong `internal/server/db/db_test.go` (file đã có, dùng `dbtest`): Up tới 0004, Down về 0003, rồi Up lại. Kiểm bằng `to_regclass('public.invite_codes')` và version của goose. Đây là file test duy nhất được phép chứa chữ `invite`. <!-- Updated: Validation Session 1 - chốt file test migration để grep có ngoại lệ rõ -->
   - Sửa helper trong `internal/server/testclient_test.go` để không còn chữ `invite`.
   - Giữ test đổi mật khẩu sai → 403.
9. Web: bỏ field + state trong `signup-form.tsx` và `add-key-dialog.tsx`, dọn `messages.ts`. Chỉ làm đủ để build, vì dialog key sẽ được làm lại ở phase 6.

## Tests / Verify
```bash
go build ./... && go vet ./internal/server/...
AINOVEL_TEST_DATABASE_URL=${AINOVEL_TEST_DATABASE_URL:-} go test ./internal/server/...
make gen && git diff --stat web/src/api/types.gen.ts
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
grep -rn -i "invite\|mã mời" internal/server web/src --exclude-dir=node_modules | grep -v -e migrations/0002 -e migrations/0004 -e internal/server/db/db_test.go   # phải rỗng
```
Chạy mọi lệnh từ gốc repo.
Flow test cần Postgres. Nếu thiếu biến môi trường thì ghi rõ trong báo cáo là flow test đã bị skip.

## Success Criteria
- [x] Không còn `invite` trong `internal/server` và `web/src`, trừ migration 0002/0004 và `internal/server/db/db_test.go`. Lệnh grep ở trên ra rỗng.
- [x] `serve invite create` báo lệnh con không hỗ trợ.
- [x] Migration 0004 chạy được trên DB đã có 0001–0003, và Down/Up lại được (flow test hoặc chạy `serve` với DB local).
- [x] Thêm key không cần mật khẩu; đổi mật khẩu vẫn cần.
- [x] Tạo key có giới hạn 30/ngày/user và trần 20 key. Signup có giới hạn 10/ngày/IP. Có test cho cả hai.
- [x] Lệnh verify pass.

## Kết quả triển khai (2026-10-13)
- Xong toàn bộ bước 1–9. Báo cáo: [reports/cook-cook-p01-report.md](./reports/cook-cook-p01-report.md), review: [reports/from-code-reviewer-to-cook-p01-review-report.md](./reports/from-code-reviewer-to-cook-p01-review-report.md).
- Khác plan có chủ ý: trần 20 key nằm trong `Repo.Create(..., maxKeys)` (transaction khoá dòng user `FOR NO KEY UPDATE`) thay cho `CountByUser` riêng, để hai request song song không cùng lọt trần.
- Thêm sau review: limiter cửa sổ dài (>10 phút) khi map đầy thì bỏ khoá cũ nhất thay vì từ chối; 429 theo ngày dùng `httpx.TooManyToday` (vẫn code `rate_limited`, message "Đã đạt giới hạn trong ngày…").
- Flow test chạy đủ trên Postgres 18 local tạm (không skip).

## Risk Assessment
- Mở đăng ký làm tăng tải. Giới hạn ở phase này cộng với phase 4 (kiểm key, chia lượt chạy, chặn đĩa, dọn tài khoản), và rule Cloudflare bắt buộc ghi ở phase 12. Không làm captcha (user đã chốt, xem "Câu hỏi đã chốt khi validate" trong plan.md).
- Bỏ re-auth khi thêm key: nếu phiên bị chiếm, kẻ gian không đọc được key cũ (chỉ hiện last4). Nhưng chúng **thêm được key của chính chúng** rồi `PATCH api_key_id` để các truyện của nạn nhân chạy bằng key đó (`internal/server/books/handlers.go:250-268`), từ đó đọc được prompt và chương qua log của provider. User đã chấp nhận bỏ re-auth. Phần giảm thiểu: trang truyện hiện "Key đang dùng: nhãn …last4" (phase 11) để user phát hiện key lạ.
- Tab còn chạy SPA cũ gửi `invite_code`/`current_password` sẽ nhận 400 (field lạ). Docs ghi "tải lại trang sau khi nâng cấp" (phase 12).
- **Rollback:**
  - Chạy `deploy/backup.sh` trước khi deploy.
  - Binary cũ chạy trên DB đã có 0004 sẽ trả 500 khi signup (`UPDATE invite_codes`), vì goose không chặn version cao hơn mức binary biết.
  - Rollback đúng cách: dừng service, rồi chọn một trong hai: (a) restore backup (mất dữ liệu phát sinh sau nâng cấp), hoặc (b) chạy Down của 0005 rồi 0004 bằng goose, sau đó mới chạy binary cũ.
  - Dữ liệu mã mời không lấy lại được. Chấp nhận, vì tính năng đã bỏ.
