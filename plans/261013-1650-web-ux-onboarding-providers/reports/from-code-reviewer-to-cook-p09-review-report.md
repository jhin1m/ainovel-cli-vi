# Review phase 9: trang truyện (tiến độ + layout)

Điểm: **7/10**

## Phạm vi
- Mới: `web/src/features/books/{progress-stepper.tsx, progress-stepper.test.tsx, book-stats-strip.tsx, book-header.tsx, writing-tab.tsx, book-screen-layout.test.tsx}`
- Sửa: `web/src/features/books/{book-screen.tsx, steer-box.tsx}`, `web/src/features/book-stream/{stream-text.tsx, timeline.tsx}`, `web/src/lib/messages.ts` (khối `book.stats`, `book.stepper`, `book.steer.open/hasPending/close`)
- Kiểm: `pnpm -C web typecheck && lint && test && build` đều pass (24 file, 193 test). `stream-store.ts`, `use-book-stream.ts`, `agent-meta.ts`, `status-banner.tsx`, `budget-dialog.tsx` không đổi (git diff rỗng).
- Đã đối chiếu với code Go: `internal/store/progress.go` (vòng đời `InProgressChapter`/`CompletedChapters`), `internal/flow/router.go:149-155` (flow `reviewing` không còn đường nào đặt trong production), `internal/host/observer.go:133-157` + `observer_tools.go:43-44,157-168` (nội dung `summary`).

## Đánh giá chung
Bố cục, slot `actions`, nhánh thuê bao, sheet góp ý và a11y nhìn chung làm đúng spec. Throttle `aria-live` đúng (trailing, luôn áp giá trị cuối, không đọc khi giá trị quay lại như cũ). Contract của `StreamText`/`Timeline`/`SteerBox` tương thích (prop mới đều có mặc định, ngoài `writing-tab.tsx` không ai dùng). Lỗi chính nằm đúng ở rủi ro mà spec đã nêu: map sai bước ở trường hợp hay gặp nhất là editor duyệt chương vừa xong.

## Critical
Không có.

## High

### H1. Lúc editor duyệt, stepper báo đã viết xong một chương chưa hề được viết
- `web/src/features/books/progress-stepper.tsx:46` và `:52`
- Theo code: `MarkChapterComplete(N)` đặt `InProgressChapter = 0`, `CompletedChapters` có N, `CurrentChapter = N+1` (`internal/store/progress.go:170-175`). Sau đó engine giao editor duyệt chương/cung, flow vẫn là `writing` (`router.go:149-152`: không có đường production nào đặt `reviewing`). Như vậy nhánh "editor đang chạy" là đường thật duy nhất dẫn tới bước Biên tập.
- Kịch bản: viết xong chương 7, editor đang duyệt → snapshot `{completed_count: 7, in_progress_chapter: 0}` cùng agent `editor` ở trạng thái `working` → `chapter = done + 1 = 8` → `{index: 2, chapter: 8}`. Bước 2 (đã qua, có dấu ✓, đọc là "(Đã xong)") hiện **"Viết chương 8"**, trong khi chương 8 chưa viết và editor đang duyệt chương 7. Header lại hiện "Chương 8/40". User được báo sai đúng ở việc phase này cần làm rõ ("AI đang làm gì, tới đâu").
- Test còn khoá luôn hành vi sai: `progress-stepper.test.tsx:46-47` (`flow reviewing` với `completed_count: 2` thì mong `chapter: 3`) và `:91` (chỉ kiểm nhãn "Biên tập").
- Cách sửa: khi bước là Biên tập mà `in_progress_chapter === 0` thì lấy chương vừa xong:
  ```ts
  const editing = agent.includes('editor') || EDIT_FLOWS.has(s.flow)
  if (editing) {
    const ch = s.in_progress_chapter > 0 ? s.in_progress_chapter : Math.max(done, 1)
    return { index: 2, chapter: ch }
  }
  ```
  Sửa test thành `{completed_count: 2, agents:[editor working]}` → `{index: 2, chapter: 2}`, thêm assertion rằng bước đã qua có nhãn "Viết chương 2". Rewrite/polish vẫn đúng vì `StartChapter` đặt `InProgressChapter = X` (`progress.go:132`).

## Medium

### M1. Dưới 1280px bản nháp không còn tự bám theo chữ mới, và timeline bị đẩy xuống dưới cả chương
- `web/src/features/books/writing-tab.tsx:20`, `web/src/features/book-stream/stream-text.tsx:17-20`
- Dưới xl, `StreamText` không có class cuộn nên `boxRef` không phải khung cuộn, `el.scrollTop = el.scrollHeight` không làm gì. Bản nháp cao theo nội dung (một chương 20–40 nghìn ký tự, giữ tối đa 200 nghìn ký tự theo `MAX_STREAM_CHARS`). Kịch bản trên điện thoại 375px: user mở trang lúc writer đang viết chương, thấy đầu chương. Chữ mới xuất hiện ở đáy, cách màn hình hàng chục lần cuộn, và trang không đi theo. Timeline nằm dưới toàn bộ bản nháp nên gần như không dùng được. Khi `streamClear()` chạy (`observer.go`, lúc dispatch xong), bản nháp biến mất, trang co lại và nhảy vị trí. Spec chỉ yêu cầu bỏ 42dvh, nhưng mục (b) của review có "auto-scroll bản nháp", và mục này đang hồi quy dưới 1280px.
- Đề xuất (cần user chọn vì chạm vào yêu cầu spec): (1) dưới xl chỉ hiện vài đoạn cuối, kèm nút "Xem toàn bộ bản nháp"; hoặc (2) thêm nút nổi "Tới đoạn mới nhất" kiểu nút `latest` của Timeline, dùng `scrollIntoView` lên đoạn cuối; hoặc (3) dưới xl đặt timeline trước bản nháp. Cách (1) giữ được cả "không cuộn lồng" lẫn "thấy chữ mới".

### M2. Màn ≥1280px nhưng thấp vẫn có cuộn lồng khi có banner
- `web/src/features/books/book-screen.tsx:133` (`xl:h-dvh xl:overflow-y-auto`), `:189` (`xl:min-h-[18rem]`)
- Ước tính ở xl: header khoảng 66, dải số liệu 50, stepper 76–106, tab 52, khoảng cách 4×16, `pb-6` 24, tổng khoảng 330–360px. Nếu thêm `StatusBanner` (chờ duyệt/tạm dừng/lỗi, khoảng 130px + 16) hoặc dải "Đang kết nối lại" (khoảng 36) hay `cmdError`, thì trên laptop 1366×768 (innerHeight khoảng 650–690) phần còn lại nhỏ hơn 288px. Khi đó `Loaded` tự cuộn và hai cột bên trong cũng cuộn, tức có cuộn lồng, trái với tiêu chí "Không còn cuộn lồng trong trang cuộn". `Loaded` cũng không focus được nên người dùng bàn phím không cuộn được khung ngoài. Lần kiểm bằng mắt (1280×800, 1600×1000, không có banner) không đụng tới trường hợp này.
- Đề xuất: chỉ khoá chiều cao khi màn đủ cao, ví dụ `[@media(min-width:1280px)_and_(min-height:760px)]:h-dvh` (cùng điều kiện cho các lớp `min-h-0`/`flex-1` liên quan), còn lại thì dùng bố cục cuộn trang như ở <xl. Hoặc thu `StatusBanner` thành một dòng ở xl. Nên kiểm thêm 1366×650 có banner chờ duyệt.

### M3. Dòng agent hiện nguyên chuỗi kỹ thuật của host và screen reader cũng đọc nguyên văn
- `web/src/features/books/progress-stepper.tsx:84`, `:92`
- `summary` do host tạo theo hai mẫu: `"engine → writer (Viết chương 3…)"` (`observer.go:137`) và `"writer → draft_chapter"` (`observer_tools.go:44`). Giao diện hiện "Người viết | writer → draft_chapter", còn vùng live đọc "Người viết: writer mũi tên draft gạch dưới chapter". Tên tool tiếng Anh và tiền tố `engine →` chẳng giúp user hiểu "AI đang làm gì". Test ở `progress-stepper.test.tsx:112-127` dùng đúng các chuỗi thô này nên không bắt được lỗi.
- Đề xuất: viết một hàm thuần `agentActivity(agent)` để bỏ tiền tố `^\S+ → `, map vài tool chính sang tiếng Việt qua `msg.book.stepper.tools` (draft_chapter, commit_chapter, save_review, novel_context…), tool lạ thì dùng câu chung kiểu "Đang xử lý", và viết test cho hàm này. Không cần sửa Go.

## Low

### L1. Còn cỡ chữ px lẻ trong file phase 9 sở hữu (trái thang chữ ghi ở `styles/app.css:56-57`: "Không dùng cỡ px lẻ ngoài thang này")
- `timeline.tsx:21` `text-[14px]` → `text-sm`; `:25,:28,:29` `text-[12px]` → `text-xs`
- `book-stats-strip.tsx:13`, `book-header.tsx:56` `text-[15px]` → `text-base` hoặc `text-sm`
- `book-header.tsx:95` `md:text-[28px]` không có tác dụng vì `text-2xl` đã là 28px (`app.css:61`). Nếu ý là mobile nhỏ hơn thì dùng `text-xl md:text-2xl`.
- `stream-text.tsx:32` `text-[17px] leading-[1.7]`: chấp nhận được nếu cố ý theo reader, nhưng nên ghi chú hoặc dùng biến dùng chung với reader.

### L2. Dải số liệu là điểm dừng Tab ở mọi cỡ màn
- `book-stats-strip.tsx:41`: `tabIndex={0}` cần cho khung cuộn ngang ở mobile (axe `scrollable-region-focusable`), nhưng từ md khung không cuộn mà vẫn là một điểm dừng Tab kèm viền focus vô nghĩa. Có thể chấp nhận. Muốn gọn thì dùng `md:tabindex` không được (đây là thuộc tính, không phải class), nên chỉ đặt `tabIndex` khi `scrollWidth > clientWidth` (ResizeObserver), hoặc giữ nguyên và ghi lý do.

### L3. Góp ý chỉ còn ở tab "Đang viết"
- `writing-tab.tsx:50-73`: trước đây `SteerBox` nằm ngoài các tab, ở tab Chương/Dàn ý vẫn góp ý được. Giờ sang tab khác là mất cả ô inline lẫn FAB. Đây là đổi hành vi nhỏ, spec đặt góp ý trong tab "Đang viết" nên có thể là cố ý. Cần xác nhận với user. Thêm nữa, `book-screen.tsx:146` `pb-24` vẫn chừa chỗ cho FAB ở cả các tab không có FAB.

### L4. Cột `actions` để trống vẫn tạo thêm khoảng cách
- `book-header.tsx:90`: `md:grid-cols-[...auto_auto]` cùng `md:gap-x-4`. Khi phase 11 chưa truyền `actions`, cột 4 rộng 0 nhưng vẫn có thêm một khoảng 16px, nên nút điều khiển lệch 16px so với mép phải. Sửa: chỉ dùng 4 cột khi có `actions` (`actions ? 'md:grid-cols-[auto_minmax(0,1fr)_auto_auto]' : 'md:grid-cols-[auto_minmax(0,1fr)_auto]'`, viết nguyên chuỗi để Tailwind quét được).

### L5. Tên agent bị đọc hai lần ở chế độ duyệt (browse mode)
- `progress-stepper.tsx:91` (chip hiện ra, không `aria-hidden`) và `:95` (vùng live sr-only "Người viết: …"). Khi duyệt bằng mũi tên, screen reader đọc "Người viết" rồi lại "Người viết: …". Sửa: thêm `aria-hidden` cho chip, vì vùng live đã có đủ thông tin.

### L6. Trùng lặp và thiếu test nối dây
- `msg.book.stats.providerModel` gần trùng `msg.library.providerModel` (khác định dạng). Chấp nhận được, chỉ nhắc để thống nhất.
- Không có test nào render `BookScreen`/`Loaded`. Phần nối dây (truyện thuê bao thì `onBudget` undefined, nút "Tăng ngân sách" mở `BudgetDialog`, `ReviewSwitch` nằm trong stepper, tab panel đúng id) chưa được test. Nên thêm một test render `Loaded` với mock `useBook`.
- Test 42dvh (`book-screen-layout.test.tsx:82`) chỉ so chuỗi `innerHTML`, khá yếu nhưng đủ để chặn hồi quy.

## Đối chiếu Success Criteria
| Tiêu chí | Kết quả |
|---|---|
| Stepper ở đầu trang, đúng một `aria-current`, có test từng bước, dòng agent hiện/ẩn | Đạt về cấu trúc. Chỗ lệch có chủ ý: truyện hoàn thành không có bước nào hiện tại. **Sai nội dung bước ở H1.** Stepper nằm sau header, dải số liệu và banner, chấp nhận được. |
| Model + icon, token vào/ra, thuê bao không có USD | Đạt (`ProviderIcon` có `aria-hidden`, sr-only nêu provider + model, `tabular-nums`, vi-VN). |
| ≥1280px bản nháp ≤72ch, không cuộn lồng | 72ch đạt. Không cuộn lồng chỉ đạt khi màn đủ cao (M2). |
| 375px không tràn ngang, sheet góp ý, không 42dvh, SteerBox không chiếm chỗ cố định | Đạt (theo kiểm bằng mắt của cook và đọc code: `-mx-4 overflow-x-auto px-4` + `w-max`). |
| Test stream hiện có vẫn pass | Đạt (193/193). |

## Hồi quy hành vi cũ
- Dừng/tiếp tục/xuất file: giữ nguyên (`ExportMenu` chuyển sang `book-header.tsx` không đổi logic).
- `StatusBanner`, `BudgetDialog`: giữ nguyên, có thêm nút ngân sách ở dải số liệu (ẩn với thuê bao).
- `ReviewSwitch`: chuyển vào `aside` của stepper, hoạt động như cũ.
- Tab Chương/Dàn ý, `role=tablist`/`tabpanel`, điều hướng bằng phím: giữ nguyên.
- Timeline: tự cuộn, `role=log`, nút "Mới nhất" vẫn còn.
- Bản nháp tự cuộn: **hồi quy dưới 1280px (M1)**.
- SSE (`useBookStreamStore` ở `Loaded`): không đổi.
- Góp ý ngoài tab "Đang viết": bị bỏ (L3).

## Edge case đã soát
- `useThrottled`: đổi dồn dập thì gom lại và áp giá trị cuối sau ≤4s. Giá trị quay về như cũ thì huỷ hẹn giờ. Unmount thì xoá timer. Giá trị lúc mount không bị đọc. OK.
- `activeAgent` sắp theo tên nên kết quả ổn định. Agent cũ còn `working` sau khi worker chết bị bỏ qua vì chỉ xét khi `status === 'running'`. OK.
- Lưới xl: `h-dvh`, `min-h-0`, `grid-rows-[minmax(0,1fr)]`, `h-full` trong flex item đã được flex đều hợp lệ trên trình duyệt hiện đại. OK.
- `dl > div > dt + dd` là HTML hợp lệ. `section[aria-labelledby]` cho landmark "Đang làm". FAB có nhãn bắt đầu bằng chữ hiển thị (label-in-name). OK.
- z-index: FAB z-30 < Dialog z-50, menu xuất file z-20. Khi đang cuộn giữa trang, FAB có thể che nút "Mới nhất" của timeline (cùng góc phải dưới), lỗi nhỏ.

## Việc nên làm (theo thứ tự)
1. Sửa H1 và test sai đi kèm.
2. M3: bỏ tiền tố, map tool sang tiếng Việt cho dòng agent và vùng live.
3. M2: giới hạn chế độ khoá chiều cao theo `min-height`, kiểm 1366×650 có banner.
4. M1: hỏi user chọn cách hiện bản nháp dưới 1280px (phần đuôi + "Xem toàn bộ", hoặc nút "Tới đoạn mới nhất").
5. Low: đổi px lẻ sang token, `aria-hidden` cho chip, bỏ cột `actions` khi trống, thêm test nối dây cho `Loaded`.

## Theo dõi plan
Requirements của phase 9 đã làm xong về khối lượng. Chưa nên đánh dấu xong trước khi sửa H1, vì Risk Assessment của spec nêu đúng rủi ro này. M1 và M2 liên quan tới tiêu chí layout. L3 cần user xác nhận.

## Câu hỏi còn mở
- Dưới 1280px nên hiện bản nháp đầy đủ (đúng chữ của spec) hay chỉ phần đuôi (giữ việc thấy chữ mới)?
- Bỏ góp ý ở tab Chương/Dàn ý là cố ý?

Status: DONE_WITH_CONCERNS
Summary: Bố cục, slot `actions`, nhánh thuê bao và a11y đạt spec, các lệnh kiểm đều pass. Nhưng `stepFromSnapshot` báo sai chương ("Viết chương N+1 ✓") lúc editor duyệt chương vừa xong, tức đúng trường hợp hay gặp nhất; ngoài ra bản nháp dưới 1280px mất tự cuộn và màn xl thấp có banner vẫn cuộn lồng.
