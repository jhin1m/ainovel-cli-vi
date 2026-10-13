# Review phase 7: Onboarding + thư viện

**Điểm: 8/10.** Đạt gần hết spec, test có ý nghĩa (README drift guard, gcTime thật sự xoá cache, assert không lồng phần tử tương tác). Có 1 lỗi a11y High: thẻ truyện không có focus ring khi dùng bàn phím.

## Phạm vi
- Đã đọc: last-read.ts(+test), progress-ring.tsx, getting-started.tsx(+test), run-locally.tsx(+test), book-card.tsx(+test), library-page.tsx(+test), queries.test.tsx, phần diff của queries.ts và messages.ts thuộc phase này.
- Touchpoint: book-screen.tsx (bookTitle, useNextChapter), status-banner.tsx, use-tab-title.ts, app-shell.tsx, add-key-form.tsx, main.tsx (default QueryClient), styles/app.css.
- Lệnh: `pnpm -C web typecheck && lint && test && build` đều qua (22 file, 134 test). Lint 0 lỗi.

## Critical
Không có.

## High

**H1. Không thấy focus ring của thẻ truyện (WCAG 2.4.7)** — `web/src/features/books/book-card.tsx:143` + `:158`
- `<article>` có `overflow-hidden`. Link tiêu đề tắt outline của chính nó (`focus-visible:outline-none`) và vẽ outline lên `::after` với `outline-offset-2`. `::after` là `inset-0` theo article, nên outline nằm ngoài padding box 2–4px và **bị overflow-hidden cắt mất hoàn toàn**.
- Đã kiểm bằng HTML tối giản trên Chrome headless: cùng cấu trúc, có `overflow:hidden` thì không thấy outline, bỏ `overflow:hidden` thì thấy. Bị ở mọi thẻ, cả chế độ forced-colors.
- Lần kiểm bằng mắt đã làm chỉ xem layout, không thử Tab.
- Sửa (chọn 1):
  - Giữ `overflow-hidden`, đổi sang outline bên trong thẻ: `focus-visible:after:-outline-offset-2` (hoặc `after:outline-offset-[-2px]`).
  - Bỏ `overflow-hidden` khỏi article, thêm `rounded-t-card` cho dải bìa (`:144`).

## Medium

**M1. Quay lại tab có thể phải chờ tới 60s mới poll lại; comment sai** — `web/src/api/queries.ts:69`, `web/src/main.tsx:19`
- Comment ghi "quay lại tab thì refetchOnWindowFocus tải ngay", nhưng default toàn cục là `refetchOnWindowFocus: false` và `useBooks` không ghi đè.
- query-core 5.104 chỉ tính lại `refetchInterval` sau mỗi lần query cập nhật (`queryObserver.ts` `onQueryUpdate → #updateTimers`). Khi tab đang ẩn, interval đã đặt là 60s. User quay lại tab sẽ thấy dữ liệu cũ tối đa 60s, trái với spec "15s khi tab đang hiển thị".
- Sửa: thêm `refetchOnWindowFocus: true` vào `useBooks`. staleTime 15s nên chỉ tải lại khi dữ liệu đã cũ; sau lần tải đó interval tự về 15s. Thêm 1 test cho phần nối hook nếu muốn: test hiện tại chỉ kiểm hàm `booksPollInterval`.

**M2. "Viết tiếp" trên thẻ bấm lại được trước khi danh sách cập nhật, gây lỗi 409 giả** — `web/src/features/books/book-card.tsx:115-129`
- `useBookCommand.onSuccess` chỉ `setQueryData(qk.book(id))` và gọi `invalidateQueries(qk.books)` mà không await. Vì vậy `isPending` tắt ngay, trong khi prop `book` của thẻ vẫn là `waiting_review` cho tới khi refetch xong.
- Bấm lần 2 trong khoảng đó: server trả `not_waiting_review` (`internal/server/books/handlers.go:463`), thẻ hiện `role=alert` "Truyện không ở trạng thái chờ duyệt". Trang truyện không bị vì đọc `qk.book` đã được set.
- Server có kiểm điều kiện nên không hỏng dữ liệu. Đây chỉ là lỗi UX.
- Sửa: `disabled={next.isPending || next.isSuccess}`.

## Low

- **L1** `web/src/lib/messages.ts:88` có `library.progressLabel` nhưng không dùng. `:81` có `library.words`, không còn ai dùng sau khi làm lại thẻ. Xoá đi, hoặc dùng `progressLabel` cho `aria-label` của ProgressRing: hiện vòng chỉ đọc "30%", thiếu ngữ cảnh. Spec chỉ ghi "x%" nên cả hai cách đều được.
- **L2** `web/src/features/onboarding/run-locally.tsx:43-54`: khi chép thành công, screen reader không được báo. `aria-label` cố định "Chép lệnh: …" che chữ "Đã chép", còn vùng `role=status` chỉ hiện khi lỗi. Đưa `t.copied` vào vùng status khi `state === 'copied'`.
- **L3** `web/src/lib/messages.ts:101-102`: nút trong dải "Cần bạn xử lý" ghi "Tăng ngân sách" / "Chạy lại", nhưng chỉ dẫn tới trang truyện (`library-page.tsx:47-54`). Nhãn hứa một hành động mà nút không làm. Cân nhắc đổi thành "Xem ngân sách" / "Mở để chạy lại", hoặc giữ nguyên nếu chấp nhận.
- **L4** `web/src/features/books/library-page.tsx:82-119`: `<main>` chỉ bọc lưới thẻ. h1, checklist và dải xử lý nằm ngoài main, còn `<header>` cấp trang thành landmark `banner`. Pattern này đã có từ trước, nhưng phase này thêm nội dung quan trọng nằm ngoài main. Nên chuyển `<main>` lên bọc cả trang.
- **L5** Chỉ ghi chú: `GettingStarted` không hiện nếu `useKeys` lỗi (`library-page.tsx:97`). Chấp nhận được, nhưng là im lặng.

## Touchpoint / contract
- `bookTitle` giữ nguyên chữ ký và hành vi; book-screen không bị ảnh hưởng. Props của `BookCard` không đổi.
- `useBooks`: app-shell (`use-tab-title.ts`) dùng chung query key, nên `refetchIntervalInBackground: true` làm tiền tố "(n)" chạy đúng khi tab ẩn.
- `useCreateKey` có `gcTime: 0`: không hỏng trạng thái khi form còn mount. Observer gắn vào mutation ngay trong `mutate()` nên GC bị huỷ. `optionalRemove` chỉ xoá khi không còn observer. Khối "Đã lưu" của add-key-form đọc state cục bộ `saved`, không đọc `create.data` sau unmount. Test `queries.test.tsx` chứng minh cache rỗng sau unmount: không có gcTime 0 thì test này fail.
- Lệnh "viết tiếp" dùng đúng `useNextChapter` (POST `/next`), giống book-screen. Không thêm endpoint.
- Lệnh cài khớp README L201-206, URL repo lấy từ `git clone`, không lấy từ go.mod. Test đọc README thật để chặn lệch nội dung.

## Acceptance criteria
- [x] User mới thấy checklist, bước 1 có `.callout` + `aria-current="step"`. Tick bằng `invalidate(keys)`.
- [x] Lưới 1/2/3/4 cột (sm/lg/90rem). Dải xử lý dùng `needsAttention`. Thẻ chờ duyệt có "Viết tiếp".
- [x] Bìa theo `coverTone`, số liệu Literata `tabular-nums` định dạng vi-VN, ProgressRing có `aria-label`, ProviderIcon, thuê bao hiện token.
- [x] RunLocally: lệnh đúng theo README, copy có try/catch, không gọi backend, link mở tab mới có `noopener noreferrer`.
- [x] Không lồng phần tử tương tác (test assert `a a, a button, …`). **Chưa đạt phần focus-visible của (e): xem H1.**
- [x] Không còn px lẻ trong các file phase này sở hữu (`min-[90rem]` là breakpoint, chấp nhận được).

## Việc nên làm theo thứ tự
1. H1: sửa outline-offset hoặc bỏ overflow-hidden, rồi Tab qua thẻ ở light/dark để kiểm.
2. M1: thêm `refetchOnWindowFocus: true` cho `useBooks` và sửa comment.
3. M2: thêm `|| next.isSuccess` vào điều kiện disabled.
4. L1–L4 tuỳ ý.

## Câu hỏi còn mở
- L3: nhãn hành động trong dải xử lý có nên phản ánh đúng việc "chỉ mở trang truyện" không? Đây là quyết định UX của user.

Status: DONE_WITH_CONCERNS
Summary: Phase 7 đạt spec; typecheck/lint/test/build đều qua. Còn 1 lỗi a11y High: focus ring của thẻ bị overflow-hidden cắt mất (đã tái hiện trên Chrome). Thêm 2 lỗi Medium: poll 60s kéo dài sau khi quay lại tab, và "Viết tiếp" bấm lại được gây 409 giả.
