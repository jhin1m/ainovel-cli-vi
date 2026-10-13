# Sửa finding review web SPA

Gate: typecheck, lint, test (62), build, `go build ./...` đều xanh. Chỉ sửa trong `web/**`; không đụng `types.gen.ts`, `embed.go`, `dist/.gitkeep`, Go. Không commit.

## Đã sửa
- M1 `connect.ts`: khi EventSource CLOSED, `fetchQuery` lại book. Lỗi mạng/429/5xx thì retry backoff; 401 do client API đưa về login và dừng; 404 và 4xx khác thì dừng, `store.setFailure(message server)` hiện banner đỏ ở màn truyện (thay "Đang kết nối lại"). 404 còn invalidate danh sách. Test FakeEventSource cho network/429/503/book ok/401/404.
- M2: status đổi → `invalidateQueries({queryKey: qk.book(id), exact: true})`; test.
- M3: store giữ nguyên tham chiếu `events` khi chỉ text/reconnecting đổi. `useBookStream` tách thành `useBookStreamStore` + `useStreamSelector`. `Loaded` chỉ subscribe `reconnecting`/`failure`; `DraftSection` chỉ subscribe text, `ActivitySection` chỉ subscribe events (Timeline không còn scrollToIndex khi events không đổi). Test tham chiếu events.
- M4 `dialog.tsx`: onClose qua ref, effect chỉ theo `open`, focus trap Tab/Shift+Tab, Esc, trả focus về nút mở. Test mới `dialog.test.tsx` (cha render lại không cướp focus, trap, trả focus).
- L1: `read.$n` parse chặt `^[1-9]\d{0,8}$`; không hợp lệ → ErrorState (không spinner vô hạn), header không in "Chương NaN".
- L2: dùng `MAX_BUDGET_USD` + `parseBudget` trong `new-book-logic.ts`; thêm `parseRaisedBudget` cho dialog tăng ngân sách; bỏ hằng/hàm trùng; `max` của input và message lỗi lấy từ hằng số.
- L3: export bằng `download()` (fetch + blob) trong `client.ts`, kiểm `res.ok`, 429/409/... hiện FormError tiếng Việt thay vì lưu file JSON; tên file lấy từ Content-Disposition.
- L4: công tắc có vùng chạm 44px (track vẫn 32px).
- L6: BookCard aria-label gồm tên, trạng thái, tiến độ; tabs có `id`/`aria-controls`/`aria-labelledby`, roving tabindex, phím Trái/Phải/Home/End.
- L7: xoá `useDeleteBook` (không có chỗ dùng).
- Hợp đồng mới: steer 503 `worker_starting` và PATCH 409 `book_running` đi qua `errorMessage` (message server), `onSuccess` không chạy nên không báo thành công; không cần đổi code, đã kiểm.

## Bỏ qua
- L5 (icon agent): thuần thẩm mỹ, ngoài phạm vi sửa lỗi.
- L8: giới hạn hợp đồng server (stream nháp bắt đầu giữa câu sau reconnect), không sửa được phía SPA.
- L9: `minimumReleaseAgeExclude` chỉ gỡ khi các gói đủ tuổi; không phải sửa code.

## Lưu ý
- Thay đổi chữ ký messages: `budgetInvalid`, `budget.invalid` thành hàm nhận trần; `library.open` nhận thêm trạng thái và tiến độ.
- Chưa có test cho luồng export bằng blob (chỉ typecheck/lint).

Status: DONE
Summary: Đã sửa M1-M4, L1-L4, L6, L7; bỏ L5, L8, L9 có lý do; gate web và go build xanh.
Concerns: Export blob và tab bàn phím chưa có test tự động; blob tải toàn bộ file vào bộ nhớ (chấp nhận được với EPUB/TXT cỡ truyện).
