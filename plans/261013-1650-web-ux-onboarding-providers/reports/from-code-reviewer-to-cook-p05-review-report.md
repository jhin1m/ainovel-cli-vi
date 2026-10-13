# Review phase 5: design tokens, layout rộng, tiêu đề tab

**Điểm: 8/10**. Không có lỗi chặn. Có 2 điểm Medium cần ghi vào plan hoặc sửa nhỏ.

## Phạm vi
- Sửa: `web/src/styles/{tokens,app}.css`, `web/src/components/app-shell.tsx`, `web/src/components/ui/{dialog,button,field,states}.tsx`, `dialog.test.tsx`, `lib/messages.ts` (khối `title`), `api/queries.ts` (`useBooks`).
- Mới: `lib/{cover-tone,needs-attention,use-tab-title}.ts` cùng test, `components/app-shell.test.ts`.
- Kiểm tra: `typecheck`, `lint`, `test` (13 file, 81 test) và `build` đều pass khi chạy lại. Đã đọc CSS sinh ra trong `web/dist/assets/*.css`: có đủ `shadow-card`, `border-line-card`, `rounded-card`, `rounded-control` (10px), `text-xl` 1.375rem, `text-2xl` 1.75rem, `.card-hover`, `.callout`, `max-w-[1600px]`, `max-h-[90dvh]`.

## Tiêu chí nghiệm thu
| Tiêu chí | Kết quả |
|---|---|
| Token có cặp light/dark. `--ink-warm` đạt ≥ 4.5:1 | Đạt. Đã tự tính lại: 4.70 (trên `#f7eee3`), 4.94 (trên `--bg`), 7.48 / 9.08 (dark). Thêm: 5.40 trên `--surface` trắng. Chữ trắng trên 12 tông bìa đạt từ 5.79 đến 10.62. |
| Khung ≤ 1600px, padding theo breakpoint, 375px không tràn, reader giữ nguyên | **Đạt một phần.** Khung, mobile và reader đều ổn. Lề thực tế không phải 16/24/32 (xem M1). |
| Dialog nhận `size`, test cũ pass | Đạt |
| Tab có tên trang và "(n)" cả khi tab ẩn, n giảm khi xử lý xong | Đạt (n cập nhật trong tối đa 15s, hoặc ngay khi stream hay mutation invalidate `qk.books`) |
| Không còn `text-[Npx]` / `rounded-[Npx]` trong `components/` | Đạt (grep ra rỗng) |

## Critical
Không có.

## High
Không có.

## Medium

**M1. Lề trang bị cộng đôi ở md+, và các phase sau chưa có bước gỡ.** `web/src/components/app-shell.tsx:67`
- Shell dùng `md:px-6 xl:px-8`. Các trang feature vẫn giữ lề riêng: library `px-5`, keys/account `px-4`, new-book `px-5`, book-screen `px-4 lg:px-6`. Lề thực tế vì vậy là:
  - md: 40–44px
  - xl: 48–52px
  - book-screen ở lg/xl: 48 / 56px
- Spec yêu cầu 16 → 24 → 32.
- Ở dải md–lg, banner lỗi/ngân sách (`book-screen.tsx:237,242`) và `steer-box` (border-t, góc vuông dưới lg) không còn chạm mép mà thành dải lơ lửng.
- Lý do chọn cách này hợp lý: không được sửa `features/**`, còn mobile phải giữ được phần tràn sát mép. Mức này chấp nhận được **tạm thời**.
- Vấn đề là grep phase 06–09 không thấy bước nào gỡ lề riêng của trang. Nếu không thêm, lề đôi sẽ ở lại vĩnh viễn.
- Đề xuất cho lead: thêm vào phase 6–9 bước "trang bỏ lề ngang ở md+ (vd. `px-5 md:px-0`) để shell sở hữu gutter", và ghi độ lệch so với spec vào báo cáo phase 5.

**M2. Dialog `lg`/`xl` dính sát mép màn hình ở dải sm–md.** `web/src/components/ui/dialog.tsx:71,78`
- Backdrop không có padding, panel là `w-full max-w-2xl|4xl`.
- Với viewport từ 640 đến 672px (lg) hoặc từ 640 đến 896px (xl), gồm cả tablet dọc 768/820, panel tràn hết bề ngang. Góc `sm:rounded-card` chạm mép và thiếu khoảng thở.
- Hiện chưa caller nào dùng `lg`/`xl` (cả 3 Dialog trong features đều mặc định `md`), nên lỗi còn tiềm ẩn. Phase sau dùng là gặp ngay.
- Sửa: thêm `sm:p-4` cho div backdrop. `md` không bị ảnh hưởng.

## Low
- **L1. Bóng hover bị "dính" sau khi chạm trên mobile.** `web/src/styles/app.css:134-151`. `.card-hover:hover` nâng bóng và nhích thẻ. Trên màn cảm ứng, hiệu ứng ở lại sau khi chạm. Nên bọc trong `@media (hover: hover)`. Thêm nữa, vì không đặt trong layer nên `box-shadow` của nó thay cả chuỗi bóng của Tailwind, nên `ring-*` trên thẻ biến mất khi hover. Đây là đánh đổi chấp nhận được cho quyết định 3, chỉ cần biết trước.
- **L2. Polling khi tab ẩn: chi phí và phiên.** `web/src/api/queries.ts:64`
  - `useBooks` giờ chạy trên mọi route `_app`, kể cả reader, và gửi 4 request/phút cho mỗi tab, kể cả tab ẩn.
  - Phía server rẻ: một query có chỉ mục theo `user_id`, không N+1. Nhưng trên điện thoại khi đang đọc thì vẫn tốn pin và dữ liệu.
  - Tác dụng phụ: một tab bị bỏ quên sẽ liên tục gia hạn `last_seen_at`, nên mốc hết hạn khi nhàn rỗi 7 ngày (`internal/server/auth/session.go:19`) không bao giờ tới. Chỉ còn hạn tuyệt đối 30 ngày.
  - Chrome giới hạn timer của tab ẩn xuống khoảng 1 phút, nên "(n)" thực tế cập nhật chậm tới ~1 phút.
  - Tuỳ chọn: `refetchInterval: () => (document.hidden ? 60_000 : 15_000)`. Không bắt buộc.
- **L3. Đổi `--radius-control` 12 → 10 làm lệch bo góc lồng nhau.** `features/books/book-screen.tsx:188/203` có tablist `rounded-control p-1` (10px) bọc tab `rounded-[9px]`, nên góc trong gần bằng góc ngoài (đúng ra khoảng 6px). Thuộc phase 9. Các chỗ khác dùng `rounded-control` vẫn ổn. Nút giờ khớp bo góc với ô nhập (vốn đã là 10px).
- **L4. Cỡ chữ đổi dây chuyền sang chỗ khác.**
  - `text-xl` 20 → 22px áp vào `__root.tsx:10`, logo sidebar và tiêu đề dialog.
  - `text-2xl` 24 → 28px áp vào `PageHeader` và h1 của book-screen (`book-screen.tsx:253`). Ở 375px, tên truyện dài sẽ xuống thêm dòng.
  - Nút 15 → 16px.
  - Tất cả đúng thang chữ của spec. Ghi lại để phase 7–9 kiểm bằng mắt.
- **L5. Trùng chuỗi.** `msg.title.library` và `msg.title.newBook` trùng `msg.nav.*`. Có thể trỏ thẳng sang `msg.nav`. Chỉ là nit DRY.
- **L6. Login/signup chỉ hiện "AI Novel".** Hai trang này nằm ngoài shell. Spec ghi "Tab hiện tên trang", nhưng ở đây không có "(n)" nên chấp nhận được.

## Đánh giá các quyết định có chủ ý
1. **Gutter**: chấp nhận được tạm thời, nhưng phải đưa vào plan (M1).
2. **`0 0 #0000` thay cho `none`**: đúng. Trong chuỗi `box-shadow` của Tailwind, `none` sẽ làm cả khai báo invalid và mất luôn ring. `border-line-card` trong suốt ở light, ăn theo `--border` ở dark: đúng ý spec. `@theme inline` sinh `--shadow-card: var(--shadow-card)` trong `@layer theme`, nhưng `tokens.css` không nằm trong layer nên thắng, không thành vòng lặp. Cách này giống pattern radius sẵn có.
3. **`.card-hover` không layer**: thắng `shadow-card` như mong muốn (xem L1). `.callout` để trong `@layer components`: đúng.
4. **Chỉ ghi đè xl/2xl/3xl**: đúng, vì xs–lg trùng mặc định của Tailwind. Đổi radius 12 → 10: xem L3.
5. **`useTabTitle` trong shell**:
   - Hook được gọi trước nhánh `return` của reader, nên thứ tự hook không đổi giữa các render.
   - Cleanup trả tiêu đề về tên app, và chạy ổn với StrictMode.
   - Khi gặp 401, `queryClient.clear()` rồi điều hướng về /login; shell unmount nên tiêu đề cũng được reset.
   - Polling nền: xem L2.
6. **Dialog `size` + `max-h-[90dvh] overflow-y-auto`**: tương thích ngược, mặc định `md`. Trong 2 dialog hiện có không có popover tuyệt đối nào bị cắt. Riêng mép màn hình thì xem M2.

## Hồi quy / hợp đồng
- `routeKind` giữ nguyên chữ ký. `routeTitle` là export mới. `buttonClass`/`inputClass` giữ nguyên API; caller truyền thêm `text-base` (`new-book-form.tsx:240`) chỉ là thừa.
- `needsAttention` dùng `KnownBookStatus` từ `api/status.ts`, đủ 4 trạng thái. Test đối chiếu với `BOOK_STATUSES`.
- `coverTone` dùng FNV-1a và có test phủ đủ 6 tông.
- Không lộ dữ liệu, không đụng auth. Nhánh reader không đổi.

## Việc nên làm (theo thứ tự)
1. Thêm `sm:p-4` cho backdrop của Dialog (M2, 1 dòng).
2. Lead bổ sung bước gỡ lề ngang của trang ở md+ vào phase 6–9, và ghi lệch spec vào báo cáo phase 5 (M1).
3. Tuỳ chọn: bọc `.card-hover:hover` trong `@media (hover: hover)` (L1), và giãn polling khi tab ẩn (L2).

## Câu hỏi mở
- Có chấp nhận việc tab ẩn làm mất mốc hết phiên khi nhàn rỗi 7 ngày (L2) không, hay muốn giãn chu kỳ poll lúc ẩn?

Status: DONE_WITH_CONCERNS
Summary: Phase 5 đạt các tiêu chí: token/tương phản (đã tính lại), Dialog `size`, "(n)" khi tab ẩn, không còn px lẻ trong components. Gate pass. Riêng lề trang bị cộng đôi ở md+ (lệch spec 16/24/32, chưa có bước gỡ trong plan), và Dialog lg/xl dính mép ở viewport 640–896px.
