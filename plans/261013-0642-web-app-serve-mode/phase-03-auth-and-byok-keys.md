---
phase: 3
title: "Auth và BYOK keys"
status: completed
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 3: Auth và BYOK keys

## Overview

Đăng ký bằng invite code, đăng nhập email + mật khẩu, session phía server; lưu API key LLM đã mã hoá, chỉ server giải mã ngay trước khi đưa cho worker qua stdin. Danh sách model hợp lệ (có giá) để ngân sách luôn có tác dụng.

## Requirements

- Functional:
  - `POST /api/auth/signup {email, password, display_name, invite_code}`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/me`, `POST /api/me/password {current_password, new_password}` (huỷ mọi session khác).
  - `GET /api/keys` (id, provider, label, last4, created_at — **không bao giờ** trả key), `POST /api/keys {provider, label, api_key, current_password}`, `DELETE /api/keys/{id}` (từ chối nếu có truyện đang chạy dùng key này). Thêm key đòi nhập lại mật khẩu (chặn XSS/phiên bị lộ thêm key lạ).
  - `GET /api/providers` → mỗi provider trong allowlist (dùng chung `internal/entry/worker/providers.go`): label + **danh sách model có giá** = danh sách model chọn lọc tĩnh theo provider (`internal/server/keys/models.go`) ∩ model có giá đầu vào/đầu ra > 0 trong `models.DefaultRegistry()`. Server nạp pricing một lần lúc khởi động (`models.StartPricingRefresh` với cache trong `DATA_DIR`) và làm mới mỗi 24h.
  - CLI admin: `ainovel-cli serve invite create [--uses N] [--expires 7d]` in code ra stdout (chỉ lưu hash).
- Non-functional:
  - Mật khẩu: argon2id dạng PHC, m=19MiB t=2 p=1 (mức tối thiểu OWASP), semaphore 4 hash đồng thời; độ dài 10–128.
  - Session: token 32 byte, DB lưu `sha256`; cookie `__Host-ainovel_session` (HttpOnly, Secure, SameSite=Lax, Path=/); ở `AINOVEL_DEV=true` dùng tên `ainovel_session` (tiền tố `__Host-` bắt buộc Secure và không có Domain); idle 7 ngày, tuyệt đối 30 ngày; tạo token mới khi login.
  - Rate limit (`x/time/rate`, in-memory, IP từ `clientIP` của phase 2): key theo (IP, email) 5/phút + theo IP 30/phút; vượt ngưỡng thì trả 429 kèm `Retry-After`, không khoá tài khoản.
  - Lỗi login chung chung; email không tồn tại vẫn chạy một lần hash giả để thời gian như nhau.
  - Key: AES-256-GCM, nonce 12B ngẫu nhiên, AAD = `user_id|key_id`; **một** master key 32 byte từ `AINOVEL_MASTER_KEY_FILE`; cột `key_version` (mặc định 1) để sau này xoay vòng mà không cần migration. Thiếu file → server không khởi động.
  - Không viết slog handler lọc riêng; thay bằng test khẳng định key/mật khẩu không xuất hiện trong log của toàn bộ flow.

## Architecture

Migration `0002_auth.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS citext;
users(id uuid pk default gen_random_uuid(), email citext unique, password_hash text, display_name text,
      is_admin bool default false, created_at, updated_at)
sessions(token_hash bytea pk, user_id fk on delete cascade, created_at, last_seen_at, expires_at)
invite_codes(code_hash bytea pk, max_uses int, used_count int default 0, expires_at, created_at)
api_keys(id uuid default gen_random_uuid(), user_id fk on delete cascade, provider text, label text,
         ciphertext bytea, nonce bytea, key_version int default 1, last4 text, created_at,
         primary key (id), unique (user_id, id))      -- unique để books tham chiếu FK kép
```

```
internal/server/
  auth/    password.go, session.go, handlers.go, middleware.go (RequireUser → ctx), ratelimit.go
  secret/  box.go (Seal/Open với AAD; key_version)
  keys/    handlers.go, repo.go, models.go (danh sách model chọn lọc), pricing.go (lọc theo registry)
  admin/   invite.go
```

- `keys.repo.DecryptForBook(ctx, bookID)` là **lối duy nhất** để lấy key rõ: `SELECT ... FROM books b JOIN api_keys k ON k.user_id = b.user_id AND k.id = b.api_key_id WHERE b.id = $1`, AAD = `b.user_id|b.api_key_id`. Không có hàm nào giải mã theo key id rời.

## Related Code Files

- Create: các file trên + test; `internal/server/db/migrations/0002_auth.sql`
- Modify: `internal/server/server.go` (route, khởi động pricing refresh), `cmd/ainovel-cli/main.go` (`serve invite create`)
- Read only: `internal/models/{registry.go,pricing.go,models_generated.go}`, `internal/entry/worker/providers.go`

## Implementation Steps

1. `secret/box.go` + test: round-trip; sai AAD → lỗi; sai master key → lỗi.
2. `auth/password.go` + test (đúng/sai, parse PHC, constant-time).
3. Migration + `auth/session.go`; `last_seen_at` cập nhật tối đa 1 lần/phút.
4. Handlers + `RequireUser` + rate limiter + test (429 sau ngưỡng; email khác cùng IP vẫn đăng nhập được).
5. `keys`: CRUD + `DecryptForBook` + `models.go`/`pricing.go`; test: response không bao giờ chứa key; user B xoá/dùng key của A → 404; model không có giá không xuất hiện trong `/api/providers`.
6. `serve invite create`.
7. Integration test full flow: signup bằng invite → login → thêm key → `/api/me` → đổi mật khẩu (sai mật khẩu cũ → 403) → logout → 401; gom log và khẳng định không chứa key/mật khẩu.

## Success Criteria

- [x] Flow auth chạy; invite hết lượt/hết hạn bị từ chối.
- [x] Trong DB key chỉ ở dạng ciphertext; không có trong log/response (test).
- [x] Không có đường nào để user dùng hay giải mã key của user khác (test ở đây; `DecryptForBook` + FK kép làm ở phase 4 vì cần bảng `books`).
- [x] Brute-force bị 429; không khoá được tài khoản người khác chỉ bằng cách gửi sai mật khẩu từ IP khác.

## Risk Assessment

- Rate limit mất khi restart → chấp nhận; thêm rule WAF Cloudflare cho `/api/auth/*` (phase 6).
- Mất master key = mọi key người dùng phải nhập lại → backup riêng (phase 6).
- Đăng ký mở công khai cần xác minh email + Turnstile — ngoài phạm vi MVP.
- Registry chưa có giá cho model mới ra → model đó tạm không chọn được (an toàn hơn là chạy không có trần chi phí).
