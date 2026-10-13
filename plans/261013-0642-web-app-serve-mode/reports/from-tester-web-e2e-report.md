# E2E trình duyệt web SPA (phase 5) — report

Môi trường: `ainovel-cli serve` dev (AINOVEL_DEV=true, 127.0.0.1:28431), worker giả `fakeworker`, Postgres DB riêng `ainovel_e2e` (đã drop). Trình duyệt: agent-browser (session riêng, không dùng Chrome của user). Không gọi LLM thật.

## Kịch bản

| # | Kịch bản | Kết quả | Ghi chú |
|---|---|---|---|
| 1 | Đăng ký bằng mã mời | PASS | 1280px |
| 2 | Đăng nhập sai mật khẩu → báo lỗi; đúng → vào thư viện | PASS | 360px |
| 3 | Thêm API key OpenRouter (giá trị giả) | PASS | key hiện dạng `••••0000` |
| 4 | Tạo truyện `[fake:fast]`, chế độ duyệt từng chương | PASS | |
| 5 | Màn truyện: timeline, stream realtime, banner "Chờ duyệt" | PASS | 1280px, 360px |
| 6 | Bấm "Viết chương tiếp" → chương 2, 3 → "Hoàn thành" | PASS | sau fix: header hiện 3/3 |
| 7 | Reader: đổi cỡ chữ (+2), kiểu chữ không chân, nền Sepia | PASS | cỡ 20px, font Be Vietnam Pro |
| 8 | Reader: tải lại trang vẫn giữ cài đặt | PARTIAL | cỡ/kiểu chữ giữ (localStorage `ainovel.reader`). Nền Sepia sau reload chưa xác minh bằng ảnh |
| 9 | Reader 360px: panel cài đặt, chương sau/trước | PASS | "Chương sau" chỉ hiện khi chương đã có |
| 10 | Truyện chậm (không marker), Dừng → Tiếp tục | PASS | cửa sổ chạy chỉ ~5s/truyện nên bấm Dừng ngay sau khi tạo |
| 11 | Steer khi chờ duyệt | PASS | 200, truyện chạy tiếp chương 2 |
| 12 | Steer khi truyện đã hoàn thành | UNCLEAR | 200 nhưng không thấy thông báo; xem "Concerns" |
| 13 | Export TXT / EPUB tải được (UI + fetch) | PASS | TXT 3937 B, EPUB 4743 B (magic `PK`), Content-Type/Disposition đúng |
| 14 | "Đang kết nối lại…" khi tắt server rồi bật lại | PASS | banner hiện, tự nối lại sau khi server lên |
| 15 | Đăng xuất → redirect /login | PASS | |
| 16 | Không lỗi console JS | PASS | probe xác nhận log capture hoạt động; không có lỗi trên 7 trang |
| 17 | Không tràn ngang ở 360px | PASS | scrollWidth = 360 trên 6 trang. Chip lọc thư viện tràn trong `nav overflow-x-auto` (có chủ đích) |
| 18 | Dark mode (scheme hệ thống) | PASS | |
| 19 | Light mode | PASS | session riêng `--color-scheme light` |

## Bug đã sửa (web/**)

1. **CTA chữ tàng hình (cả light và dark)**. `web/src/styles/app.css`: rule `a { color: var(--accent) }` và `a:hover` nằm ngoài layer, nên luôn thắng utility Tailwind (`text-on-accent`, `hover:text-on-accent`). Hệ quả: nút "Truyện mới" trên thanh dưới mobile có chữ và icon cùng màu với nền (rgb 63,181,163), không đọc được. Các link dạng nút khác cũng bị ảnh hưởng.
   - Repro: mở `/` ở 360px, `getComputedStyle` của link "Truyện mới" (bottom > 700): color = background.
   - Fix: đưa `a`, `a:hover` vào `@layer base`. Sau fix: color rgb(8,32,28) trên nền rgb(63,181,163).
2. **Nhãn chương vượt tổng**. `web/src/features/books/book-screen.tsx`: header hiện "Chương 4/3" khi truyện hoàn thành. Fix: clamp `current_chapter` theo `total_chapters`. Sau fix: "Chương 3/3". Thẻ thư viện đã đúng từ đầu (dùng `completed_count`).

Gate sau fix: `typecheck`, `lint`, `test` (7 file, 48 test), `build` đều xanh. Binary Go đã build lại để embed bundle mới.

## Bug còn lại (phía Go, KHÔNG sửa)

1. **`snapshot.current_chapter` lệch khỏi tổng sau khi hoàn thành.** Repro: tạo truyện `[fake:fast]`, chờ `status=completed`, `GET /api/books/{id}` → `snapshot.current_chapter=4`, `total_chapters=3`, `completed_count=3`. Web đã clamp, nhưng mọi consumer khác của snapshot vẫn nhận giá trị sai. Gợi ý: `MarkComplete` hoặc phần dựng snapshot nên giữ `current_chapter <= total`.

## Concerns

- **Steer trên truyện đã hoàn thành.** Server (`internal/server/books/handlers.go`, `handleSteer`) trả 200, chuyển truyện sang `queued` rồi về `completed`, `pending_text` rỗng sau đó. UI xoá textbox và không hiện thông báo rõ ràng (chưa thấy `role=status`). `SteerBox` chỉ disable khi `status === 'deleting'`, nên ô nhập vẫn bật cho truyện đã xong. Cần quyết định sản phẩm: chặn steer khi `completed`, hay cho chạy lại có chủ đích kèm thông báo.
- Thời gian chạy của `fakeworker` không marker `fast` ~5s/truyện, nên test pause/steer đang chạy phải bấm rất nhanh. Kết quả đúng nhưng cửa sổ hẹp.
- Nền Sepia sau reload chưa xác minh bằng screenshot.

## Screenshot

Thư mục: `/private/tmp/claude-501/-Users-jhin1m-Desktop-ducanh-project-ainovel-cli-vi/b43df03e-441e-44ab-94a7-766aad6ec175/scratchpad/e2e/shots/`

- `01-signup-desktop.png`, `02-new-book-desktop.png`
- `03-book-review-desktop.png` (banner chờ duyệt)
- `04-reader-desktop.png`, `05-reader-settings-desktop.png`, `06-reader-sepia-sans-desktop.png`
- `07-export-menu-desktop.png`
- `08-book-auto-running-desktop.png`, `09-book-paused-desktop.png`, `10-steer-after-complete-desktop.png`
- `11-reconnecting-desktop.png`
- `12-library-mobile-360-dark.png` (trước fix CTA), `13-library-mobile-360-dark-fixed.png` (sau fix)
- `14-new-book-mobile-360.png`, `15-book-review-mobile-360.png`
- `16-reader-settings-mobile-360.png`, `17-reader-dark-chapter2-mobile-360.png`
- `18-login-error-mobile-360.png`
- `19-library-light-mobile-360.png`
- `20-steer-waiting-review.png`

File export đã tải về (kiểm tra nội dung): `.../e2e/dl-book.epub`, `.../e2e/dl-book.txt`.

## Dọn dẹp

- Đã tắt server và worker (không còn process `e2e`), đóng cả 2 session agent-browser.
- Đã drop DB `ainovel_e2e`.
- Đã xoá master key và file mã mời trong scratchpad. Log server còn ở `.../e2e/server.log` (0 ERROR, 0 WARN).
- Không commit. Chỉ đổi 2 file: `web/src/styles/app.css`, `web/src/features/books/book-screen.tsx`.

## Câu hỏi chưa giải quyết

1. Steer trên truyện đã hoàn thành nên: bị chặn ở UI/server, hay chạy lại có chủ đích? Hiện hành vi không rõ với người dùng.
2. Có cần sửa `current_chapter` ở phía Go (bug 1) trong phase này, hay để sang phase sau?

Status: DONE_WITH_CONCERNS
Summary: 19 kịch bản chạy trên 360px và 1280px, light/dark. Đa số PASS. Sửa 2 bug web (CTA chữ tàng hình do CSS unlayered; nhãn "Chương 4/3"). Gate web xanh. Còn 1 bug Go (current_chapter lệch) và 1 điểm steer trên truyện đã xong cần quyết định sản phẩm.
Concerns/Blockers: steer trên truyện hoàn thành chưa rõ hành vi mong muốn; bug current_chapter phía server chưa sửa (ngoài phạm vi); nền Sepia sau reload chưa xác minh bằng ảnh.
