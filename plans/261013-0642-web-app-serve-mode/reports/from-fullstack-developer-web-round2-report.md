# Web SPA round 2 report

## Gate (xanh)
typecheck, lint (0 warning), test (48 tests, 7 files), build; `go build ./...` OK. Không thêm dependency (react-virtual, react-markdown đã ghim từ đợt 1). Không chạy trình duyệt/server thật.

## Files
- New: features/book-stream/{stream-store,connect,use-book-stream,timeline,stream-text,agent-meta}.ts(x) + stream-store.test.ts; features/books/{book-screen,book-tabs,status-banner,budget-dialog,steer-box}.tsx + status-banner.test.tsx; features/reader/{reader-page,markdown-view,reader-settings}.ts(x) + markdown-view.test.tsx; routes/_app/books/$id/read.$n.tsx.
- Changed: routes/_app/books/$id/index.tsx, components/app-shell.tsx (sidebar >=lg, ẩn bottom nav ở trang truyện + reader, reader full-screen không shell), lib/messages.ts (msg.book, msg.reader), styles/app.css (màu agent, .reader-prose, .draft-para), routeTree.gen.ts (tự sinh khi build).

## Quyết định
- Store ngoài React chỉ giữ events/streamText/reconnecting (flush mỗi rAF); snapshot/status đi thẳng vào cache Query trong connect.ts (test với FakeEventSource).
- Reset timeline ở `onopen`; `onerror` -> "Đang kết nối lại…". Nếu trình duyệt đóng hẳn (HTTP lỗi, readyState CLOSED, vd 429 giới hạn SSE) thì tự mở lại sau 3s, tăng gấp đôi tới 30s.
- Status đổi -> invalidate `books`; sang waiting_review/completed/budget_exhausted hoặc completed_count tăng -> invalidate chapters.
- Bản nháp stream: text node, tách đoạn theo dòng trống, mỗi đoạn React.memo + content-visibility; giữ tối đa 200k ký tự cuối. Event tối đa 2000 (bỏ cũ nhất).
- Map agent theo chuỗi chứa architect/writer/editor (không rõ giá trị Agent từ engine); còn lại là "Hệ thống".
- Reader: không rehype-raw, `img` thay bằng alt text, link chỉ http/https + rel noopener noreferrer. Cài đặt lưu localStorage `ainovel.reader`, có clamp 15-24px.
- Dialog tăng ngân sách: max 50 US$ hard-code (như đợt 1); mở từ banner budget_exhausted (lúc đó không chạy) nên khớp hợp đồng 409 `book_running` mới; lỗi server hiện qua FormError. Công tắc "Duyệt từng chương" chỉ PATCH advance_mode.
- Dark mode theo prefers-color-scheme (palette đợt 1). Export: `<details>` với 2 link tải `?format=epub|txt`, vô hiệu khi chưa có chương.
- Steer box sticky đáy cột chính; bỏ qua `pending_text` rỗng.

## Chưa làm / mở
- Chưa kiểm 360px/dark/INP trên trình duyệt thật; timeline ảo không test được trong jsdom (không có layout).
- Giá trị `Agent` và `Category` thật của Event chưa xác nhận; chi phí tối đa 50 vẫn hard-code, nên có endpoint expose.
- Thông báo tăng ngân sách thành công: dialog đóng, banner chuyển sang "tạm dừng" (server chuyển trạng thái); chưa có toast.

Status: DONE_WITH_CONCERNS
Summary: Màn truyện, stream, banner, dialog ngân sách, steer, export, reader + sanitization, layout desktop và test đã xong; gate FE và go build xanh.
Concerns/Blockers: Chưa chạy trình duyệt thật (360px, dark, hiệu năng stream); map agent theo suy đoán.
