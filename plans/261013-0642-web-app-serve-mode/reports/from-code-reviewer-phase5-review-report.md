# Review web SPA (commit ec7e79d)

Gate xanh (typecheck/lint/test 48/build), types.gen.ts khớp Go, không có lỗi Critical/High. XSS/CSRF/API key đạt.

## Medium
- M1 `web/src/features/book-stream/connect.ts:78-85`: EventSource CLOSED do 401/404 bị thử lại vô hạn (truyện bị xoá ở tab khác, session hết hạn) → banner "Đang kết nối lại…" mãi, không về /login. Sửa: fetch lại book trước khi hẹn retry; 401 → login, 404 → dừng; chỉ retry khi lỗi mạng/429/5xx.
- M2 `connect.ts:54-68`, `book-screen.tsx:222`: status đổi chỉ invalidate `['books']`, không invalidate `['book', id]` → pending_text/advance_mode/budget cũ. Sửa: invalidate `qk.book(id)` exact.
- M3 `stream-store.ts:36`, `book-screen.tsx:92`: mỗi rAF tạo lại mảng events (≤2000) và render lại cả màn truyện; rủi ro INP. Sửa: giữ tham chiếu events khi không đổi, selector riêng cho stream text, đẩy subscribe xuống tab "Đang viết".
- M4 `web/src/components/ui/dialog.tsx:15-27`: effect phụ thuộc onClose inline → mất focus khi cha render lại; không có focus trap. Sửa: onClose vào ref, effect chỉ theo `open`, thêm focus trap/inert.

## Low
- L1 reader `$n` không hợp lệ → spinner vô hạn.
- L2 trần ngân sách 50 hard-code 2 chỗ, parseBudget lặp.
- L3 link export có `download` → lỗi 429/409 bị lưu thành file JSON.
- L4 công tắc "Duyệt từng chương" chỉ cao 32px (<44px).
- L5 thiếu icon agent. L6 a11y BookCard aria-label, tabs thiếu aria-controls/phím mũi tên. L7 `useDeleteBook` không dùng. L8 stream nháp bắt đầu giữa câu sau reconnect (giới hạn hợp đồng server). L9 `minimumReleaseAgeExclude` 22 gói — gỡ khi đủ tuổi.

## Câu hỏi mở
- Steer truyện completed (chủ đích: chạy tiếp như TUI).
- Expose MaxBookBudgetUSD cho client?
