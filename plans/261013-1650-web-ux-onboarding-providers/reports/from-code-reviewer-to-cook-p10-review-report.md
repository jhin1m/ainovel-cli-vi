# Review phase 10 (Reader)

Phạm vi: `web/src/features/reader/{reader-settings.ts,reader-settings.test.ts,use-reader-shortcuts.ts,use-reader-shortcuts.test.ts,reading-progress.tsx,reader-page.tsx,reader-page.test.tsx,markdown-view.test.tsx}`, `web/src/features/books/book-header.tsx` (phần link "Đọc tiếp"), `book-screen-layout.test.tsx` (test BookHeader), `web/src/lib/messages.ts` (khối `reader`).

Đã chạy lại: `pnpm -C web typecheck` sạch, `pnpm -C web lint` sạch, `pnpm -C web test` 220/220 pass.

## Acceptance criteria

| Tiêu chí | Kết quả | Ghi chú |
|---|---|---|
| Máy tối + chưa lưu thì reader tối | Đạt | `systemTheme()` + `loadSettings` dùng làm fallback; có test mock `matchMedia`. Riêng người dùng cũ thì xem M1. |
| ←/→/Esc, không bắt phím khi đang gõ | Đạt | Bỏ qua khi có modifier, phím giữ (repeat), `defaultPrevented`, input/textarea/select/contenteditable/`role=dialog`, và khi bảng cài đặt mở. |
| Header có tên truyện + tên chương, có thanh tiến độ | Đạt | Thanh tiến độ gây re-render cả trang, xem H1. |
| Thư viện/trang truyện hiện "Đọc tiếp chương N" | Đạt | `book-header.tsx:91-121`, cùng quy tắc `≤ completed_count` với thẻ thư viện. |
| Chương mới nhất có thông báo, chờ duyệt thì viết tiếp được | Đạt | Dùng `useNextChapter`, tức `useBookCommand(id,'next')`, đúng lệnh với `book-card.tsx:116`. |
| Cột 65ch, `messages.ts` chỉ sửa khối `reader`, không đổi layout phase 9 | Đạt | |

Kiểm phụ:
- **Contract:** `loadSettings`/`saveSettings` chỉ có caller là `reader-page.tsx` và test (đã grep). `sanitizeSettings` thêm tham số tuỳ chọn nên không phá caller cũ.
- **Hành vi cũ:** route vẫn `key={id}`, `scrollTo(0,0)` theo `n` không đổi. Đường lỗi, đường loading và nav giữ nguyên.
- **Pattern:** dùng `rounded-control` (token có trong `styles/tokens.css`), `msg`, `useBookCommand`. `useNavigate` đúng kiểu TanStack.
- **Stale state khi đổi `n`:** `useChapter` không dùng `placeholderData` nên `c` về `undefined` khi đổi chương. `setLastRead` chỉ chạy theo `c.number`. `LatestChapterNote` luôn unmount giữa hai chương (chỉ chương cuối mới có `next===0`), nên `isSuccess` không bị giữ từ chương trước. Không có lỗi ở mục này.

## Findings

### H1 (High): kéo trang làm cả ReaderPage và markdown chạy lại mỗi khi phần trăm đổi
- **Vị trí:** `reader-page.tsx:144` (`useScrollProgress` nằm trong `ReaderPage`), `reader-page.tsx:239` (`MarkdownView`), `markdown-view.tsx:28-38` (không memo, object `components` và component `img` inline bị tạo mới mỗi lần render).
- **Kịch bản:** mỗi lần phần trăm (đã làm tròn) đổi, `setPct` re-render toàn bộ `ReaderPage`. `ReactMarkdown` phải parse lại cả chương (remark → hast → React), và vì `img` là component mới mỗi lần nên subtree đó còn bị remount. Một chương dài làm điều này khoảng 100 lần. Lướt nhanh trên điện thoại thì có vài chục lần mỗi giây. Đo thử (test tạm trong jsdom, đã xoá): chương 42k ký tự mất khoảng **7.2 ms mỗi lần re-render** trên máy Mac. Điện thoại tầm trung chậm hơn 3–5 lần, tức khoảng 20–35 ms, vượt ngân sách 16 ms của một khung hình nên trang sẽ giật khi cuộn. Đây đúng là đường đọc chính.
- **Sửa:** chuyển state xuống component lá, ví dụ `<ReadingProgress resetKey={...} />` tự gọi `useScrollProgress` bên trong, để `ReaderPage` không re-render khi cuộn. Nên làm thêm: `export const MarkdownView = memo(...)` và đưa object `components` ra hằng ở cấp module. Thêm một test đếm số lần render (hoặc spy) để khoá hành vi này.

### M1 (Medium, tuỳ việc bản web đã có người dùng thật chưa): người dùng cũ bị khoá theme sáng
- **Vị trí:** `reader-settings.ts:55`.
- **Kịch bản:** `saveSettings` trước phase này luôn ghi đủ object, kể cả `theme: 'light'` mặc định. Ai từng chỉ đổi cỡ chữ hay kiểu chữ thì storage đã có `theme:'light'`, nên `themeChosen=true` và reader không bao giờ theo chế độ tối của máy nữa, dù họ chưa từng chọn theme.
- **Sửa:** thêm trường phiên bản, ví dụ `v: 2`, khi lưu. Bản ghi không có `v` mà `theme === 'light'` thì coi là chưa chọn theme. Nếu bản web chưa phát hành thì bỏ qua và ghi chú lý do.

### L1 (Low): bấm đúng theme đang sáng (do hệ điều hành chọn) không ghim được theme
- **Vị trí:** `reader-page.tsx:165`.
- **Kịch bản:** máy đang tối, nút "Tối" đang `aria-pressed`. Người dùng bấm "Tối" để chốt, nhưng `s.theme === settings.theme` nên không lưu. Khi máy chuyển sang sáng thì reader cũng sáng theo, ngược với ý người dùng.
- **Sửa:** để `SettingsPanel` báo nguồn thay đổi, ví dụ `onChange(s, { theme: true })` từ nút theme, hoặc so sánh theo nút được bấm thay vì so giá trị.

### L2 (Low): thanh tiến độ không đo lại khi chiều cao trang đổi mà không có sự kiện cuộn
- **Vị trí:** `reading-progress.tsx:15-37` (chỉ đo lại theo `resetKey`, `scroll`, `resize`).
- **Kịch bản:** `useBook` trả về muộn làm `LatestChapterNote` hiện ra. Hoặc mở/đóng bảng cài đặt làm header sticky cao lên, hoặc web font serif tải xong. `scrollHeight` đổi nhưng giá trị trên thanh vẫn là số cũ cho tới lần cuộn sau.
- **Sửa:** thêm `ResizeObserver(document.body)` gọi `schedule`. Khi đó có thể bỏ phần lớn `resetKey`.

### L3 (Low): ← → có thể lấy mất phím của vùng cuộn ngang hoặc widget ARIA
- **Vị trí:** `use-reader-shortcuts.ts:15`.
- **Kịch bản:** markdown do LLM sinh có thể có bảng hoặc khối `pre` dài. Chrome cho các vùng cuộn được nhận focus; khi focus ở đó, bấm → sẽ sang chương thay vì cuộn ngang. Tương tự với `role=slider/tablist/radiogroup/listbox/menu`. Hiện reader chưa có widget nào như vậy.
- **Sửa:** thêm `[role="slider"],[role="tablist"],[role="radiogroup"],[role="listbox"],[role="menu"]` vào selector. Có thể bỏ qua thêm khi `target.scrollWidth > target.clientWidth`.

### L4 (Low): giá trị `aria-valuenow` đổi liên tục khi cuộn
- **Vị trí:** `reading-progress.tsx:45-51`.
- **Kịch bản:** NVDA mặc định phát tiếng bíp khi progressbar đang hiển thị đổi giá trị, nên người dùng NVDA có thể nghe bíp liên tục lúc cuộn. Spec bắt buộc `role=progressbar` nên vẫn giữ, chỉ ghi nhận rủi ro. Có thể cân nhắc chỉ cập nhật `aria-valuenow` theo bước 5% hoặc 10%.

### L5 (Low): "AI đang viết chương N" không tự cập nhật trong reader
- **Vị trí:** `reader-page.tsx:97`.
- **Kịch bản:** reader không theo dõi SSE. Sau khi bấm "Viết chương tiếp", dòng trạng thái giữ nguyên cho tới khi người dùng focus lại cửa sổ (refetch) hoặc chuyển trang. Khi editor đang duyệt, `in_progress_chapter=0` nên câu fallback hiện `chapter+1`, gần đúng. Chấp nhận được vì đã có link "Xem AI viết". Nếu muốn sửa thì có thể dùng `refetchInterval` cho `useBook` khi status là running/queued.

### L6 (Low): thiếu test tích hợp
- Chưa có test ReaderPage cho: bấm → thì `navigate` sang `/read/(n+1)`, phím tắt bị tắt khi bảng cài đặt mở, và đổi cỡ chữ trong UI thì không ghi `theme` vào storage. Hook và hàm lưu đã có unit test, nhưng phần nối dây trong `reader-page.tsx:156-170` chưa được khoá.

### L7 (Low): thanh tiến độ có thể nằm dưới tai thỏ khi chạy PWA standalone
- **Vị trí:** `reading-progress.tsx:51`. Header có `pt-safe` nhưng thanh lại đặt `top-0`.
- **Sửa:** đổi sang `top-[env(safe-area-inset-top)]`, hoặc đặt thanh ở đáy header (`bottom-0`).

## Điểm: 8/10

Đủ mọi acceptance criterion, contract sạch, theo đúng pattern có sẵn, không thấy regress. Trừ điểm chủ yếu vì H1 (giật khi cuộn trên đường đọc chính) và M1 (migration settings cũ). Nên sửa H1 trước khi đóng phase. M1 cần chốt bản web đã có người dùng thật hay chưa.

## Câu hỏi còn mở
- Bản web đã có người dùng thật lưu `ainovel.reader` chưa? Câu trả lời quyết định có cần sửa M1 hay không.
