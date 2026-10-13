# Kiểm thủ công bản web, phase 12 (2026-10-13)

## Cách dựng
- `pnpm -C web build` rồi `go build ./cmd/ainovel-cli` và `go build ./internal/server/jobs/fakeworker` (binary để trong scratchpad, không vào repo).
- `serve` dev ở `127.0.0.1:8111` với: `AINOVEL_DEV=true`, `AINOVEL_DATABASE_URL=postgres://postgres:ci@127.0.0.1:55439/ainovel_p12`, master key tạm, `AINOVEL_DEV_WORKER_BIN=fakeworker`, `AINOVEL_MAX_BOOKS_PER_USER=1`, `AINOVEL_PUBLIC_URL=http://127.0.0.1:8111`.
- Trình duyệt: agent-browser headless. Gọi API bằng curl với cookie phiên.
- Xong việc: đã tắt server, đóng browser, `DROP DATABASE ainovel_p12`. Không sửa code/docs, không commit.
- Ảnh chụp: `/private/tmp/claude-501/-Users-jhin1m-Desktop-ducanh-project-ainovel-cli-vi/c9bf23f0-4ff6-44f2-b1af-eba39708b0d2/scratchpad/p12-shots/`. Ảnh nằm ngoài repo.
- Request ra ngoài: 5 lần kiểm key OpenRouter với key giả (2 lần tự kiểm khi lưu key trong UI, 3 lần gọi `/api/keys/test`), cùng 3 trang "Lấy key".

## Kết quả

| # | Mục | Kết quả | Bằng chứng |
|---|-----|---------|------------|
| 1a | Đăng ký chỉ cần tên, email, mật khẩu (không có mã mời) | Đạt | Form chỉ có 3 ô: "Tên hiển thị", "Email", "Mật khẩu". Đăng ký xong về `/` (`01-signup.png`). |
| 1b | Checklist "Bắt đầu trong 3 bước" | Đạt | Thư viện hiện "Đã xong 0/3 bước", có nút "Thêm key" và "Tạo truyện" (`02-onboarding.png`). |
| 1c | Thêm key ngay trong trang tạo truyện | Đạt | Lưu key xong vẫn ở `/books/new`. Select chuyển sang "OpenRouter · Key A · ••••0001". Lần tự kiểm báo auth_failed bằng thông điệp tiếng Việt do server viết (`04-new-book-key-added.png`). |
| 1d | Nhãn khi mở "Thêm key khác" | Đạt | Select có nhãn "Key đã lưu", input có nhãn "API key" (cây a11y và `label[for]`). |
| 1e | Tạo truyện, thấy stepper | Đạt | Trang truyện có stepper "Lập dàn ý → Viết chương → Biên tập → Chờ bạn duyệt" và các bước tiến dần (`06-book-stepper.png`, `07-book-after-ch1.png`). |
| 2a | Key thứ 21 → 409 | Đạt | 20 lần `POST /api/keys` trả 201, lần 21 trả 409 `{"code":"key_limit","message":"Đã đạt tối đa 20 key…"}`. |
| 2b | Lần kiểm key thứ 4 trong 1 phút → 429 | Đạt | Lần 1–3 trả 200, lần 4 trả 429 `rate_limited` kèm `Retry-After: 20`. |
| 2c | Response kiểm key không chứa văn bản của provider | Đạt | Body chỉ có `{ok,kind:"auth_failed",message:<câu tiếng Việt của server>,model}`, không có chuỗi nào của OpenRouter. |
| 3a | Xoá truyện → về thư viện | Đạt | Bấm "⋯ → Xoá truyện → Xoá vĩnh viễn" thì về `/`. |
| 3b | Tạo truyện ngay sau khi xoá, không bị book_limit (MAX_BOOKS=1) | Đạt | Qua UI: xoá xong tạo ngay được (201). Qua API, đo sát hơn: khi đang có 1 truyện thì tạo thêm trả 409 `book_limit`. `DELETE` truyện đang chạy trả 202, `POST /api/books` ngay sau đó trả 201. |
| 3c | Xoá key đang dùng (truyện đã dừng) → banner đổi key | Đạt | Banner "Key của truyện đã bị xoá", có "Key đang dùng: không còn" và link "Thêm key OpenRouter" (`13-book-key-missing-banner.png`). |
| 3d | Hết ngân sách → "Lưu và viết tiếp" | Đạt | Ngân sách 0,5 US$, fakeworker tính 0,4 US$/chương nên dừng ở "Đã dùng hết 0,50 US$". Dialog "Tăng ngân sách" (`09-budget-dialog.png`): bấm "Lưu và viết tiếp" thì viết tiếp chương 2 tới "Chương 2 đã xong". |
| 4a | Reader theo theme của OS | Đạt (có lưu ý L1) | Tải trang ở light: nền reader `#fff`. Tải ở dark: nền `rgb(18,20,23)`. Đổi theme OS khi reader đang mở thì reader **không** đổi theo, xem L1. |
| 4b | Phím ←/→/Esc | Đạt | Ở ch1, → sang `/read/2`. Ở chương cuối, → đứng yên. ← về `/read/1`. ← ở ch1 đứng yên. Esc về `/books/:id`. |
| 4c | "Đọc tiếp chương N" | Đạt | Trang truyện có link "Đọc tiếp chương 2". Thẻ thư viện (truyện completed) có "Đọc tiếp chương 2" (`15-library-continue-reading.png`). Khi truyện ở waiting_review thì thẻ hiện "Đọc chương N", đúng thiết kế trong `quickActions`. |
| 5a | Không tràn ngang | Đạt | Thư viện, tạo truyện, trang truyện, cài đặt key, reader ở 375/768/1440/1920, light và dark: mọi trường hợp đều có `scrollWidth == innerWidth` (ảnh `{library,new,book,keys,reader}-{w}-{light,dark}.png`). |
| 5b | Lề không cộng dồn, nội dung ≤1600px khi ≥1440 | Đạt (có lưu ý L2) | Ở 1920: khung shell 1600px, cột nội dung 1296px (padding 32px). Ở 1440: shell 1440px, nội dung 1136px. Riêng `PageHeader` có thêm `px-3`, xem L2. |
| 5c | Reader ~65ch | Đạt | Ở 768/1440/1920: prose 677px, xấp xỉ 64,9ch (`max-w-[65ch]`). Ở 375: 335px, có lề 20px. |
| 5d | Viền focus khi Tab | Đạt | Thư viện và trang truyện, light và dark: mọi nút/link nhận focus đều có `outline: 2px solid var(--accent)`, offset 2px. Nhìn rõ trên nút accent (`crop-focus-light.png`, `crop-focus-dark.png`). Đọc giá trị ngay sau Tab sẽ thấy màu trắng vì `transition-colors` đang đổi `outline-color` trong 150ms. Không phải lỗi. |
| 5e | Dải "Số liệu truyện": tabindex theo bề rộng | Đạt | 375px: `tabindex=0`, scrollWidth 804 > 375. 768px và 1440px: không có tabindex. Ở 1440, Tab đi thẳng từ "Thao tác khác" sang "Tăng ngân sách". |
| 6 | Truyện cần xử lý → tiêu đề "(n)" | Đạt | waiting_review: "(1) Truyện · AI Novel". budget và failed/error: "(1) Thư viện · AI Novel". Khi tab ẩn: xoá truyện lỗi từ tab khác, sau khoảng 60s tiêu đề tab ẩn đổi thành "Thư viện · AI Novel". Log lúc đổi có `visibilityState=hidden`. |
| 7a | "Lấy key" OpenAI `https://platform.openai.com/api-keys` | Đạt | Curl và headless đều bị Cloudflare chặn (403 / "Just a moment…"). Chạy headed thì về `https://platform.openai.com/login?next=%2Fapi-keys`, tiêu đề "OpenAI Platform": đăng nhập xong sẽ vào trang API keys. |
| 7b | "Lấy key" DeepSeek `https://platform.deepseek.com/api_keys` | Chưa kiểm | Curl trả 405. Cả headless và headed đều gặp AWS WAF "Human Verification", tiêu đề "Let's confirm you are human". Path trùng trang API keys chính thức nhưng chưa xác nhận được bằng trình duyệt. User tự mở để xác nhận. |
| 7c | "Lấy key" x.ai `https://console.x.ai/` | Đạt một phần | Chạy headed thì về `https://console.x.ai/home`, tiêu đề "Welcome — SpaceXSI API & Developer Tools for Grok \| SpaceXSI Console". Đây là trang chủ console (có mục "Manage API keys"), không phải thẳng trang tạo key. Xem L4. |
| — | Z.ai Coding Plan với key thật | Chưa kiểm | Không có key Z.ai thật. |

## Lỗi và lưu ý tìm thấy (không mục nào chặn phát hành)
- **L1 (Low): reader không theo khi đổi theme OS lúc đang mở.** `systemTheme()` chỉ đọc `matchMedia` lúc khởi tạo, không lắng nghe sự kiện `change`. Phần còn lại của app đổi ngay theo CSS. Ví dụ: tải reader ở dark, đổi OS sang light thì body sáng nhưng khung reader vẫn tối, phải tải lại trang. File nghi ngờ: `web/src/features/reader/reader-settings.ts` (L27–29, L63–65), `web/src/features/reader/reader-page.tsx`.
- **L2 (Low): `PageHeader` cộng thêm `px-3` vào lề của cột.** Trên trang Cài đặt ở 1440px, tiêu đề "Cài đặt" bắt đầu ở x=284 còn nội dung ở x=272. Ở 375px là 12px so với 16px, nên lệch vài px. File: `web/src/components/app-shell.tsx` (`PageHeader`, `px-3`), dùng ở `web/src/features/keys/keys-page.tsx`, `account-page.tsx` và `routes/_app/books/new.tsx`.
- **L3 (Low, log): client huỷ request bị ghi là ERROR và trả 503.** Khi chuyển trang hoặc đóng tab, các request `GET /api/books…` bị huỷ sinh 11 dòng ERROR "lỗi xử lý request" với `err="context canceled"`, status 503. Log nhiễu khi vận hành. File nghi ngờ: `internal/server/httpx/json.go:81`. Nên bỏ qua hoặc hạ mức log khi lỗi là `context.Canceled`.
- **L4 (Info): link x.ai đưa tới trang chủ console.** Có thể đổi sang trang API keys trực tiếp nếu x.ai có URL ổn định. File: `web/src/features/keys/provider-meta.ts:49`.
- **L5 (Info): nhãn "Chương N/3" dễ gây hiểu nhầm.** Trên header trang truyện ở trạng thái waiting_review: sau khi xong 2/3 chương, header ghi "Chương 3/3" (chương sắp viết), trong khi vòng tiến độ ghi 2/3. Nên cân nhắc ghi "Sắp viết chương 3/3" hoặc ẩn khi đang chờ duyệt.
- Ghi chú: `DELETE /api/books/{id}` không gửi `Content-Type: application/json` thì trả 415. SPA đã gửi header này nên không ảnh hưởng UI, chỉ cần biết khi gọi API bằng tay.
- Ghi chú môi trường: daemon agent-browser tự khởi động lại một lần giữa chừng, mất cookie phiên nên phải đăng nhập lại. Đây là lỗi công cụ, không phải lỗi app.

Status: DONE_WITH_CONCERNS
Summary: Checklist phase 12 đạt gần hết: đăng ký và onboarding, thêm key tại chỗ, giới hạn 409/429, xoá truyện/key, ngân sách, reader, responsive, focus, tiêu đề "(n)" kể cả khi tab ẩn. Còn lại: reader không theo khi đổi theme OS lúc đang mở (L1), lệch lề nhỏ ở PageHeader (L2), log ERROR khi client huỷ request (L3). Link DeepSeek bị WAF chặn nên chưa xác nhận được, Z.ai thật chưa kiểm.
