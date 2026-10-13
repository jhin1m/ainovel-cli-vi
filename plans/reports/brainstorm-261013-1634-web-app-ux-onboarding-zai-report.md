# Brainstorm: web app UX, đăng ký tự do, Z.ai Coding Plan, icon provider, layout rộng

Ngày: 2026-10-13 · Branch: feat/web-app · Trạng thái: user đã duyệt toàn bộ

## Vấn đề
- Web khó dùng: người mới không biết bắt đầu từ đâu, cảm giác "đơn giản".
- Key Z.ai Coding Plan → 429, thông báo "giới hạn tốc độ" gây hiểu nhầm.
- Đăng ký cần mã mời → bỏ. Thêm key bắt nhập lại mật khẩu → bỏ.
- Provider chỉ là `<select>` chữ → cần icon.
- Layout hẹp (nội dung `max-w-3xl` = 768px) → rộng gần full.

## Phát hiện từ scout
- Invite: `internal/server/auth/handlers.go:195-267`, CLI `internal/server/cmd.go:20-125`, bảng `invite_codes` ở `db/migrations/0002_auth.sql:22-28`, form `web/src/features/auth/signup-form.tsx`.
- Password khi thêm key chỉ để re-auth (`keys/handlers.go:89`, `auth/handlers.go:118-134`). Mã hoá = AES-GCM master key server, không phụ thuộc password → bỏ không ảnh hưởng crypto. CSRF đã có (`httpx/middleware.go:175`).
- GLM preset không có baseURL (`bootstrap/setup.go:59`) → litellm mặc định `open.bigmodel.cn/api/paas/v4`. Không có đường nào tới `api.z.ai/api/coding/paas/v4`. Allowlist provider cố định chống SSRF (`entry/worker/providers.go:9-14`).
- Theo docs z.ai: Coding Plan chỉ hợp lệ với endpoint `/api/coding/paas/v4` + model trong gói; sai → 429 code 1113 "insufficient balance". Worker gắn nhãn mọi 429 là "bị giới hạn tốc độ" (`imp/call.go:104`), retry vô ích.
- Model catalog chỉ hiện model có giá > 0 (`keys/catalog.go:98-111`); registry litellm chưa có glm-4.7.
- Layout: `components/app-shell.tsx:28,31,48`; dialog `ui/dialog.tsx:67`. Không có icon thương hiệu (lucide không có).

## Phương án đã cân nhắc
| Chủ đề | Chọn | Loại |
|---|---|---|
| Guest | Bỏ (user hiểu nhầm là chạy local; engine chạy server, local = CLI) | tài khoản ẩn danh; guest chỉ xem |
| Base URL Z.ai | Provider riêng `zai-coding`, URL hard-code | cho user nhập base URL (mở lại SSRF) |
| Ngân sách thuê bao | Bỏ ngân sách USD, chỉ hiện token | giá ảo theo bảng giá API |
| Icon | Inline SVG copy từ `@lobehub/icons-static-svg` (MIT) | cài package (nặng), simple-icons (thiếu brand) |
| Độ rộng | Gần full (≤1600px), reader giữ 65ch | ~1150px; rộng cả reader |

## Giải pháp đã duyệt
1. **Đăng ký tự do**: bỏ `invite_code` khỏi API/form, bỏ CLI `serve invite`, migration drop `invite_codes`. Giữ IP limiter + `MaxBooksPerUser`. Cập nhật `docs/web-server.md`.
2. **Thêm key không cần password**: bỏ `current_password` ở `POST /api/keys` + dialog. Đổi mật khẩu vẫn cần password cũ.
3. **Z.ai Coding Plan**:
   - Provider `zai-coding` ("Z.ai Coding Plan"), BaseURL `https://api.z.ai/api/coding/paas/v4`, model của gói (xác minh mã model khi làm).
   - Cờ `subscription`: catalog không cần giá; truyện không áp ngân sách USD, hiển thị token.
   - Đổi label `glm` → "GLM (bigmodel.cn)".
   - Worker giữ mã/nội dung lỗi gốc provider; map 1113 → thông báo dễ hiểu; 429/1113 = fatal, không retry.
   - Nút "Kiểm tra key" khi thêm key (request tối thiểu tới provider).
4. **Icon provider**: `provider-icon.tsx` với SVG inline ~9 provider; lưới thẻ provider (icon + tên + link lấy key) thay `<select>`; icon ở danh sách key + trang truyện.
5. **Layout rộng**: bỏ `max-w-7xl`, nội dung `max-w-[1600px]`; danh sách truyện grid 2-4 cột; settings 2 cột; dialog `max-w-lg/xl`; reader giữ nguyên.
6. **Onboarding**: home rỗng → checklist "Bắt đầu trong 3 bước" (key → tạo truyện → theo dõi/đọc) tự tick theo trạng thái; trang tạo truyện thêm key inline nếu chưa có; home có truyện → CTA nổi bật + thẻ truyện có trạng thái/số chương/tiến độ.

## Ngoài phạm vi
Guest, quên mật khẩu, xác thực email, base URL tuỳ ý, đổi giao diện reader.

## Rủi ro
- Đăng ký mở → chia sẻ RAM/đĩa server; xem lại `AINOVEL_MAX_BOOKS_PER_USER` và giới hạn worker đồng thời.
- Mã model Coding Plan thay đổi theo thời gian → xác minh với docs z.ai lúc làm.
- Nguyên nhân 429 là suy luận từ docs + code; cần xác nhận bằng key thật sau khi sửa.
- Bỏ ngân sách cho thuê bao → truyện có thể chạy dài; vẫn bị giới hạn bởi quota 5h của z.ai và số chương.
- Nút kiểm tra key tốn một ít token/quota mỗi lần bấm.

## Tiêu chí thành công
- Đăng ký chỉ cần email + mật khẩu + tên; không còn chuỗi "mã mời" trong UI/API.
- Thêm key không có ô mật khẩu.
- Key Z.ai Coding Plan thật: kiểm tra key OK, tạo truyện chạy được chương 1.
- Key sai gói/endpoint: thông báo nêu rõ nguyên nhân, không retry lặp.
- Mọi chỗ chọn/hiện provider có icon.
- Màn ≥1440px: nội dung dùng gần hết bề ngang; mobile 375px không tràn ngang.
- User mới từ đăng ký → tạo truyện đầu tiên không phải rời trang tạo truyện để thêm key.

## Câu hỏi còn mở
- Danh sách model chính xác của Coding Plan hiện tại (GLM-4.7 / GLM-5.x) + context window để cấu hình worker.
- Có cần giới hạn đăng ký chặt hơn (vd. captcha Cloudflare Turnstile) khi mở công khai không.
