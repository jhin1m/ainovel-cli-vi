# Cook phase 12: Docs + kiểm tra cuối

Ngày: 2026-10-13 · Branch: feat/web-app · Không commit (theo yêu cầu).

## Đã làm
- **Docs** (docs-manager rồi sửa theo review):
  - `docs/web-server.md`:
    - Bỏ mã mời và `serve invite`. Câu về email đổi thành rủi ro thật: không xác minh email, có thể bị chiếm email trước, dò email qua 409 `email_taken`.
    - Thêm mục "Giới hạn khi mở đăng ký": bảng giới hạn kèm mã lỗi, chia lượt `AINOVEL_MAX_RUN_MINUTES`/`AINOVEL_SLICE_GRACE_MINUTES`, retry, dọn tài khoản 30 ngày, limiter trong bộ nhớ.
    - Rule Cloudflare là bước bắt buộc, tách theo gói: Free 1 rule gộp, Pro trở lên thêm rule riêng cho signup.
    - Thêm bảng provider và mục kiểm key (có `model`) + `provider_rejected`/1113.
    - Thêm mục "Nâng cấp lên bản này". Tab cũ cần bấm "Cập nhật" do service worker kiểu `prompt`, F5 không đủ.
    - Thêm mục "Rollback": goose dùng `GOOSE_DBSTRING`, không để mật khẩu trên dòng lệnh.
    - Thêm mục "Chạy trên máy bạn".
  - `docs/architecture.md`: bỏ `serve invite create` và package `admin` (bảng package và cây thư mục).
  - `README.md`: mục bản web (đăng ký tự do, Z.ai Coding Plan/Z.ai, so sánh với CLI/TUI). Clone đổi sang HTTPS.
- **Sửa nhỏ khi verify** (đều có test):
  - `internal/server/books/repo.go` `Count`: bỏ truyện `deleting`, nên xoá xong tạo truyện mới được ngay. Thêm ca trong `TestBookValidationAndLimits`. Đã thấy test đỏ trước khi sửa, xanh sau khi sửa.
  - `internal/server/httpx/json.go`: client huỷ request (`context.Canceled`) chỉ ghi log debug, không còn ERROR. Test: `TestWriteErrQuietWhenClientCanceled`.
  - `web/src/features/books/book-stats-strip.tsx`: dải số liệu chỉ có `tabIndex=0` khi tràn ngang. Có test.
  - `web/src/lib/messages.ts`: select ở trang tạo truyện đổi nhãn thành "Key đã lưu". `runHint` thêm gợi ý PATH / `./ainovel-cli`.
  - `web/src/features/onboarding/run-locally.tsx`: clone qua HTTPS. Test kiểm README không còn SSH.
  - `web/src/features/reader/reader-page.tsx`: chưa chọn theme thì reader đổi theo OS ngay cả lúc đang đọc. Có test.
  - `web/src/components/app-shell.tsx` `PageHeader`: tiêu đề thẳng mép với nội dung (16px ở mobile, 0 từ md, icon "quay lại" thẳng mép).
  - Đổi px lẻ sang token: `reader-page`, `auth-layout`, `book-screen`, `stream-text`, `book-tabs`. Grep px lẻ giờ rỗng.

## Phase file lệch với code (docs ghi theo code)
- Trần retry 10 lần/10 phút của `llmretry` chỉ đặt trong `ainovel-cli worker` (`internal/entry/worker/run.go`). CLI/TUI vẫn retry không giới hạn.
- Down của 0005 đổi budget 0 → 0.01, không dùng `NOT VALID`.
- `provider_mismatch` trả 400.

## Verify
- `go build ./... && go vet ./internal/...`: ok.
- `go test -count=1 ./internal/server/... ./internal/entry/worker/... ./internal/host/... ./internal/llmretry/... ./internal/bootstrap/...` với `AINOVEL_TEST_DATABASE_URL` (ainovel-testpg): 16 package ok, exit 0.
- Checksum trước/sau `make gen`: `types-ok`.
- `pnpm -C web typecheck && lint && test && build`: pass (29 file, 254 test).
- Grep `invite|mã mời`: chỉ còn ở migration 0002/0004, `db_test.go` và mục Nâng cấp/Rollback của `docs/web-server.md`.

## Checklist thủ công
Chi tiết ở `browser-check-p12-report.md` (app thật: serve + fakeworker, DB tạm đã drop).
- Đạt:
  - Đăng ký → 3 bước → thêm key trong trang tạo truyện → stepper.
  - Key thứ 21 → 409. Lần kiểm key thứ 4 trong 1 phút → 429. Response không có văn bản provider.
  - Xoá truyện, rồi tạo mới ngay (giới hạn 1 truyện) → 201. Banner đổi key. "Lưu và viết tiếp".
  - Reader ←/→/Esc, "Đọc tiếp chương N".
  - 375/768/1440/1920, light/dark: không tràn ngang, khung ≤1600px, reader ~65ch, có viền focus.
  - Tiêu đề tab có "(n)".
  - Link OpenAI đúng. Link x.ai về trang chủ console.
- Sau khi agent kiểm trình duyệt chạy xong, tôi sửa thêm ba lỗi Low nó tìm ra (theme reader, lề `PageHeader`, log khi client huỷ request). Ba lỗi này được kiểm bằng test, chưa chạy lại trình duyệt.
- Chưa kiểm:
  - Key Z.ai Coding Plan thật (không có key).
  - Link "Lấy key" của DeepSeek (bị trang xác minh người dùng chặn).

## Review
`from-code-reviewer-to-cook-p12-review-report.md`: 8/10, không có lỗi chặn.
- Đã sửa: M1 (Cloudflare theo gói), M2 (service worker), M3 (PATH), L1, L2, L3, L4, L5, L6.
- L7 giữ nguyên: `rounded-md` = 6px khớp chỉ dẫn "tab con ~6px" của phase 5; `text-xs` ở `book-screen.tsx:50` có từ phase trước, là chữ phụ.
- L8 chỉ ghi nhận: truyện kẹt `deleting` khi lỗi IO, có log, được làm tiếp sau restart.

## Không làm
- Không chạy git-manager/journal: user cấm commit, và nhật ký kỹ thuật của bản web đã có.
- Không đổi code để áp trần retry cho CLI: ngoài phạm vi phase.

## Câu hỏi còn mở
- User tự kiểm với key Z.ai Coding Plan thật: lưu dưới `zai-coding` → kiểm OK, chương 1, USD = 0; cùng key dưới `zai` → 1113 / `provider_rejected`.
- Link "Lấy key" của x.ai về trang chủ console thay vì trang API keys. Có đổi sang URL trang API keys không?
- Có muốn áp trần retry của `llmretry` cho cả CLI/TUI không? Hiện chỉ áp trong worker của bản web.

Status: DONE_WITH_CONCERNS
