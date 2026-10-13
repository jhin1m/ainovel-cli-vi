# Cook phase 10: Reader

Ngày: 2026-10-13 | Phase: [phase-10](../phase-10-reader-improvements.md) | Review: [from-code-reviewer-to-cook-p10-review-report.md](./from-code-reviewer-to-cook-p10-review-report.md)

Review chấm 8/10: 1 High, 1 Medium, 7 Low. Đã sửa High, Medium và 5/7 Low. 2 Low còn lại ghi ở cuối.

## Đã làm
- `reader-settings.ts`:
  - `systemTheme()` đọc `prefers-color-scheme` (không có `matchMedia` thì `light`).
  - Chưa lưu theme thì reader theo hệ điều hành.
  - `loadSettings()` trả `{ settings, themeChosen }`. `saveSettings(s, themeChosen)` chỉ ghi `theme` khi user tự chọn, nên đổi cỡ chữ không khoá theme đang theo OS.
  - Dữ liệu lưu có `v: 2`. Bản cũ (không có `v`) mà theme là `light` thì coi như chưa chọn, vì bản cũ luôn ghi theme mặc định kể cả khi user chỉ đổi cỡ chữ. Bản cũ có `sepia`/`dark` thì giữ.
- `use-reader-shortcuts.ts`:
  - ← / → sang chương trước/sau, Esc về trang truyện.
  - Bỏ qua khi có modifier, giữ phím, sự kiện đã `defaultPrevented`, focus trong input/textarea/select/contenteditable/`role=dialog`, hoặc khi bảng cài đặt đọc đang mở.
  - Không có chương trước/sau thì không chặn hành vi mặc định.
  - Nút tương ứng có `aria-keyshortcuts`.
- `reading-progress.tsx`:
  - Thanh 3px ở đáy header, `role="progressbar"`, `aria-valuenow` theo bước 5%.
  - Đo bằng rAF. Có `ResizeObserver(body)` nên đo lại cả khi chiều cao trang đổi mà không cuộn.
  - Khi bật reduced-motion thì không có transition.
  - State nằm trong chính thanh nên cuộn không render lại chương.
- `reader-page.tsx`:
  - Header "Tên truyện · Chương N: Tên chương". Tên truyện cắt trước (tối đa 45%), phần chương giữ được nhiều chữ nhất có thể.
  - `setLastRead` khi chương tải xong.
  - `LatestChapterNote` hiện khi `next === 0`:
    - luôn có "Đây là chương mới nhất";
    - `waiting_review`: nút "Viết chương tiếp" (`useNextChapter`, cùng lệnh với thẻ thư viện; khoá khi pending/success; báo lỗi `role=alert`);
    - `running`/`queued`: "AI đang viết chương N" (`role=status`) + link "Xem AI viết".
  - Cột chữ vẫn 65ch. Giao diện cài đặt giữ nguyên, chỉ đổi `rounded-[10px]` sang token `rounded-control` (cùng 10px).
- `book-header.tsx`: thêm đúng một link "Đọc tiếp chương N" cạnh badge trạng thái, chỉ hiện khi chương đó ≤ `completed_count` (cùng quy tắc với thẻ). Không đổi layout phase 9. `book-card.tsx` không đổi vì phase 7 đã nối `lastRead`.
- `messages.ts`: chỉ sửa khối `reader` (`chapterHeading`, `progress`, `writeNext`, `aiWriting`, `watch`). Riêng link ở header dùng lại `library.actions.continueReading`.

## Sửa theo review
- **High:** cuộn làm cả `ReaderPage` render lại và parse lại markdown. Đã chuyển `useScrollProgress` vào trong `ReadingProgress`. Trên app thật, MutationObserver đếm được 0 thay đổi DOM trong `<article>` khi cuộn.
- **Medium:** user cũ bị khoá theme sáng. Đã sửa bằng `v: 2` như mô tả ở trên.
- **L1:** bấm đúng nút theme đang được chọn sẵn theo OS giờ lưu được lựa chọn đó (`onChange(s, pickedTheme)`).
- **L2:** dùng `ResizeObserver` thay cho `resetKey`.
- **L4:** `aria-valuenow` báo theo bước 5% để NVDA không bíp liên tục khi cuộn.
- **L6:** thêm test tích hợp trong `ReaderPage`: đổi cỡ chữ không ghi `theme`, bấm theme đang chọn thì ghi.
- **L7:** thanh chuyển từ đỉnh xuống đáy header nên không nằm dưới tai thỏ khi chạy PWA.

## Tests
- `reader-settings.test.ts` (mới; test cài đặt cũ chuyển sang từ `markdown-view.test.tsx` và đổi theo API mới, giữ nguyên các assert):
  - mock `matchMedia` tối/sáng/không có;
  - theme đã chọn được giữ;
  - đổi cỡ chữ không ghi theme;
  - dữ liệu bản cũ `light` / `sepia`;
  - bản mới chọn `light` khi máy tối vẫn giữ `light`.
- `use-reader-shortcuts.test.ts`: ←/→/Esc; modifier; input/textarea/contenteditable; dialog; tắt khi panel mở; không có chương trước thì không `preventDefault`; gỡ listener khi unmount.
- `reader-page.test.tsx`:
  - header, progressbar, `lastRead`, `aria-keyshortcuts`;
  - chờ duyệt → POST `/next` → "AI đang viết chương 4";
  - đang chạy, hoàn thành;
  - test tích hợp cài đặt;
  - `scrollPercent`.
- `book-screen-layout.test.tsx`: thêm 1 ca cho link "Đọc tiếp chương N" (có, không có, chương không còn tồn tại).

## Verify
- `pnpm -C web typecheck && lint && test && build`: pass, 28 file / 223 test. Đã chạy lại sau khi sửa theo review.
- `go build ./... && go vet …` và `go test ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL`: exit 0, 16 package ok. Phase này không đổi Go.
- App thật: `serve` dev ở cổng 8100 + fakeworker, DB tạm `ainovel_p10` (đã drop), agent-browser headless.
  - 375×812, máy tối, chưa lưu cài đặt: reader tối. `scrollWidth = 375`. Header cắt gọn cả hai phần.
  - Cuối chương thanh lên 100. Có thông báo + nút "Viết chương tiếp". Bấm nút thì chuyển sang "AI đang viết chương 2".
  - 1440×900, máy sáng: reader sáng, cột chữ 677px (65ch).
  - Esc về trang truyện, header ở đó hiện "Đọc tiếp chương 1" đúng link. → sang chương 2 (lastRead = 2, scrollY = 0), ← ở chương 1 không làm gì.
  - Shift+← bị bỏ qua. Khi panel cài đặt mở, ← không chuyển chương.
  - A+ lưu `{v, size, font}` không có theme. Chọn Sepia thì lưu theme.
  - Lưu ý khi kiểm thủ công: service worker PWA giữ bundle cũ sau khi build lại, phải gỡ SW rồi mới thấy bản mới.

## Chưa làm (Low, ghi nhận)
- **L3:** ←/→ có thể chiếm phím của vùng cuộn ngang hoặc widget ARIA. Reader hiện không có widget nào như vậy (markdown không render bảng cuộn ngang hay widget ARIA), nên chưa thêm (YAGNI).
- **L5:** "AI đang viết chương N" không tự cập nhật khi đứng yên trong reader vì `useBook` không poll. Link "Xem AI viết" dẫn về trang truyện, nơi có SSE. Thêm poll trong reader thì tốn request mà ít lợi.

## Ghi chú
- Không commit/push (theo yêu cầu). Không cập nhật docs: phase 12 lo docs.
- Không gọi bước journal và commit của skill cook, vì user đã giới hạn phạm vi file và cấm commit.

## Câu hỏi còn mở
- Không có.

Status: DONE
