# Review phase 6: Icon provider + UI key

Ngày: 2026-10-13. Phạm vi: `web/src/components/provider-icon.tsx` (+test), `web/src/features/keys/{provider-meta.ts, provider-picker.tsx(+test), add-key-form.tsx(+test), key-check-status.tsx, add-key-dialog.tsx, keys-page.tsx, account-page.tsx}`, `useTestKey` trong `web/src/api/queries.ts`, khối `keys`/`account` trong `web/src/lib/messages.ts`.

Đã chạy lại: `vitest run src/features/keys src/components/provider-icon.test.tsx` cho 3 file, 25 test, đều pass. Thêm một probe tạm (đã xoá) để kiểm focus trap của Dialog.

**Điểm: 7.5/10.** Logic lưu, tự kiểm, 429 và 409 đúng spec. Không gửi key thô tới `/api/keys/test`. Icon khớp upstream. Còn 1 lỗi a11y thật ở focus trap, xuất hiện từ phase này.

## Đã đạt (đã kiểm, không phải đọc lướt)
- Icon đủ 10 provider, `zai-coding` dùng chung logo Z.ai, provider lạ có fallback, `Object.hasOwn` chặn `toString`. Path và `fill`/`fill-rule` của cả 9 slug khớp `@lobehub/icons-static-svg@1.95.1` (đã curl unpkg và so). Header có nguồn, phiên bản, MIT, LobeHub.
- Picker: `radiogroup`/`radio`, roving tabindex, mũi tên/Home/End có quay vòng, thẻ cao ≥44px (`min-h-11`/`min-h-14`), lưới 2 cột rồi 3 cột từ md. Link "Lấy key" nằm ngoài thẻ radio, `rel="noopener noreferrer"`.
- Form: lưu xong gọi `check.mutate(key.id)` và chỉ gửi `{key_id}`. 429 không chặn `onSaved`. 409 `key_limit` hiện `msg.keys.limit` kèm link. `aria-describedby` và focus vào lỗi đầu tiên đều đúng. `setApiKey('')` khi lưu thành công. Hook đều gọi trước early return, không lệch thứ tự hook.
- `AddKeyForm` dùng được ngoài dialog (`onClose` không bắt buộc, có `compact`). Dialog chỉ còn là vỏ `size="lg"`.
- Trang key: mỗi `KeyCard` có `useTestKey` riêng, lưới `lg:2 / 2xl:3`, empty state có nút thêm key. Trang tài khoản chia 2 cột ở `lg`, đổi mật khẩu vẫn gửi `current_password`, đăng xuất vẫn giữ `onSettled → /login`. Xoá key không đổi.
- Contract: chỉ thêm export `useTestKey`. `AddKeyDialog` giữ nguyên props.

## Findings

### High
1. **Focus thoát khỏi modal (Shift+Tab).** Vị trí: `web/src/components/ui/dialog.tsx:45-60`, lỗi lộ ra do `provider-picker.tsx:73` và `add-key-form.tsx:83`.
   - Nguyên nhân 1: selector `button:not([disabled])` vẫn bắt cả các radio có `tabindex="-1"`, nên `items[0]` là radio đầu tiên, một phần tử không bao giờ nhận focus khi Tab.
   - Nguyên nhân 2: khi focus nằm trên phần tử `tabIndex={-1}` bên trong panel (thẻ `<p>` "Đã lưu key…"), không nhánh nào bắt được.
   - Đã tái hiện bằng probe user-event. Chọn "Z.ai" (radio thứ 2) rồi Shift+Tab thì focus sang nút `outside` ở trang nền. Sau khi lưu, focus đang ở `<p>`, Shift+Tab cũng ra `outside`.
   - Cách sửa: trong `onKey`, lọc `items = [...].filter(el => el.tabIndex >= 0)`. Lấy `idx = items.indexOf(active)`. Nếu `idx === -1` (focus ở panel hoặc ở phần tử -1) thì `preventDefault` rồi focus `first`/`last` theo hướng Tab. Thêm test cho Dialog với radiogroup roving và cho khối kết quả.

### Medium
2. **Tự kiểm và `onSaved` phụ thuộc component còn mount.** Vị trí: `add-key-form.tsx:106-115`.
   - Callback truyền vào `mutate()` không chạy nếu component đã unmount (TanStack Query 5.104). Nếu user bấm Esc hoặc đóng dialog khi đang lưu, key vẫn được lưu nhưng `onSaved` và lượt tự kiểm bị bỏ qua mà không báo gì.
   - Phase 8 có cùng rủi ro: nếu parent ẩn form ngay trong `onSaved`, kết quả kiểm không bao giờ hiện.
   - Cách sửa: dùng `create.mutateAsync(...).then(key => { …; check.mutate(key.id); onSaved(key) })` (promise vẫn resolve sau khi unmount). Hoặc ghi rõ trong JSDoc của `onSaved` rằng parent phải giữ form mount để hiện kết quả kiểm. Nên thêm `onChecked?(result)` cho phase 8.

### Low
3. **Key thô còn trong bộ nhớ của React Query.** Vị trí: `add-key-form.tsx:106`. `create.variables.api_key` vẫn giữ key trong lúc khối kết quả còn hiện, và `MutationCache` giữ tiếp tới hết `gcTime` (mặc định 5 phút). Key không có trong DOM. Cách sửa: gọi `create.reset()` trong `onSuccess` và/hoặc đặt `gcTime: 0` trong `useCreateKey` (`queries.ts:136`).
4. **Trình quản lý mật khẩu có thể ghi đè mật khẩu đăng nhập.** Vị trí: `add-key-form.tsx:178-179`. `type="password"` kèm `autoComplete="off"` thì Chrome và 1Password vẫn đề nghị lưu/cập nhật mật khẩu site bằng API key. Giờ form không còn ô mật khẩu thật nên rủi ro này rõ hơn. Cách sửa: thêm `data-1p-ignore`, `data-lpignore="true"`, `autoComplete="off"`. Hoặc dùng `type="text"` với `[-webkit-text-security:disc]` khi đang ẩn.
5. **Nút hiện/ẩn key luôn dùng icon `Eye`.** Vị trí: `add-key-form.tsx:198`. Trạng thái chỉ phân biệt bằng màu, và `msg.keys.hideKey` không được dùng ở đâu. Cách sửa: dùng `EyeOff` khi `showKey`, và bỏ `hideKey` khỏi `messages.ts` (hoặc dùng nó).
6. **Mất focus bàn phím sau khi bấm "Kiểm tra".** Vị trí: `keys-page.tsx:55`. Nút bị `disabled` lúc đang chờ nên focus rơi về `body`. Cách sửa: dùng `aria-disabled` và chặn trong `onClick`, hoặc giữ nút bật và bỏ qua lần bấm khi `isPending`.
7. **Không còn nút thêm key khi danh sách đang tải hoặc tải lỗi.** Vị trí: `keys-page.tsx:100-138`. Trước phase này nút luôn hiện. Nếu `GET /api/keys` lỗi thì user không thêm được key. Cách sửa: hiện nút ở cả nhánh lỗi.
8. **Còn px lẻ ở trang key.** Vị trí: `keys-page.tsx:152,161` vẫn dùng `min-h-[52px]`, trái với yêu cầu "đổi px lẻ sang token". Cách sửa: `min-h-13` hoặc `min-h-12`.
9. **Không trả được focus khi đóng dialog.** Vị trí: `keys-page.tsx:112`. Mở dialog từ nút ở empty state rồi lưu thì nút đó bị unmount (danh sách thay chỗ), nên `prev.focus()` của Dialog rơi vào phần tử đã tách khỏi DOM. Cách sửa: trong `onClose`, focus nút "Thêm key" dưới danh sách, hoặc dùng ref chung cho cả hai nút.

## Ghi chú (không cần sửa)
- `aria-describedby` của radiogroup trỏ tới đoạn chứa cả link, nên trình đọc màn hình đọc kèm "Lấy key … (mở tab mới)". Hơi dài nhưng chấp nhận được.
- URL lấy key: kiểm lại thì OpenAI, DeepSeek, x.ai trả 403 (trang chặn bot), các URL còn lại trả 200, đúng với chú thích trong `provider-meta.ts`. `grok` trỏ về trang gốc `console.x.ai`.
- `text-[15px]` đổi sang `text-base` làm chữ lớn hơn 1px. Đây là thay đổi có chủ ý theo token, nhưng nên kiểm bằng mắt ở 375px.

## Việc nên làm theo thứ tự
1. Sửa focus trap của Dialog (#1) và thêm test.
2. Đổi sang `mutateAsync` hoặc ghi rõ contract `onSaved` cho phase 8 (#2).
3. Sửa các mục Low #3 và #4 (bộ nhớ và trình quản lý mật khẩu), các mục còn lại làm khi tiện.

## Câu hỏi còn mở
- Phase 8 có cần biết kết quả tự kiểm không, ví dụ để chặn tạo truyện khi `plan_mismatch`? Nếu có thì nên thêm `onChecked` ngay bây giờ.

Status: DONE_WITH_CONCERNS
Summary: Phase 6 đạt gần đủ tiêu chí. Logic lưu, tự kiểm, 429 và 409 đúng, không lộ key thô ra endpoint kiểm. Có 1 lỗi High đã tái hiện: focus thoát khỏi Dialog khi Shift+Tab do roving tabindex và khối kết quả. Có 1 Medium về callback `mutate` bị bỏ qua khi unmount, còn lại là Low.
