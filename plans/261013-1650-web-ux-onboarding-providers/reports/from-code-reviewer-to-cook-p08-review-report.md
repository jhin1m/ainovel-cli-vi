# Review phase 8: form tạo truyện

**Điểm: 8/10.** Phần lớn đạt spec: bỏ lồng form đúng hướng, body API giữ đủ 5 trường, gói thuê bao gửi `budget_usd: 0`, logic thuần có test. Còn 1 lỗi High làm hỏng Success Criterion 2 (chip gợi ý làm mất chữ đã gõ). Lỗi này đã tái hiện bằng test probe.

Phạm vi: `new-book-form.tsx`, `config-panel.tsx`, `prompt-section.tsx`, `prompt-presets.ts`, `new-book-logic.ts`, hai file test, khối `msg.newBook`, `routes/_app/books/new.tsx`.

## Kết quả verify

- `pnpm -C web typecheck && lint && test && build`: exit 0. 22 file test, 158 test pass, build OK.
- Đã chạy test probe trên bản copy ở scratchpad, không đụng vào cây thật:
  - P1: bấm hai lần liên tiếp vào cùng một chip làm mất nút "Hoàn tác".
  - P2: bấm chip A rồi chip B, sau đó bấm "Hoàn tác" thì ô ý tưởng về nội dung của chip A, chữ gốc mất.
  - P4: nội dung ghép dài hơn 4000 ký tự thì bị chặn, form hiện lỗi và focus về ô ý tưởng.
  - P5: khi chưa có key nào, form hiện lỗi `keyId`.

## High

**H1. Chip gợi ý làm mất chữ đã gõ, dù đã có hoàn tác.** Vị trí: `prompt-section.tsx:35-40`.
- Lỗi 1, bấm cùng chip hai lần (hoặc double-click). Lần bấm thứ hai có `idea === text`, nên code chạy `setUndo(null)`. Nút "Hoàn tác" biến mất và chữ gốc không khôi phục được nữa (P1).
- Lỗi 2, bấm chip A rồi chip B trong 5s để so hai gợi ý. Lần bấm sau ghi `prev` bằng văn bản của chip A, nên "Hoàn tác" chỉ trả về chip A (P2).
- Hai lỗi này vi phạm Success Criterion "Chip gợi ý không làm mất chữ đã gõ".
- Đề xuất: lưu thêm văn bản chip vừa áp vào state undo, `{ prev, label, applied }`. Khi bấm chip:
  ```ts
  if (idea === text) return // bấm lại cùng chip: không làm gì, giữ undo
  const prev = undo && idea === undo.applied ? undo.prev : idea
  setUndo(prev.trim() === '' ? null : { prev, label, applied: text })
  ```
  Thêm 2 test cho hai kịch bản trên.

## Medium

**M1. Thẻ cấu hình sticky có thể cao hơn khung nhìn, khi đó nút "Bắt đầu viết" không còn "luôn thấy được".** Vị trí: `config-panel.tsx:58`.
- Ước tính chiều cao thẻ khi đã có key: khoảng 750px (key, model, chi tiết giá, chip, ô ngân sách, ước tính, công tắc, nút).
- Khi chưa có key, hoặc đang mở form thêm key với lưới provider 2 cột: thẻ cao trên 850px.
- Laptop 1366×768 chỉ có khoảng 650px khung nhìn. Thẻ sticky cao hơn khung nhìn thì phần dưới bị cắt, nút submit nằm ngoài màn hình. Cột phải còn cao hơn cột trái nên sticky không giúp gì.
- Chưa thấy bằng chứng đã làm bước 6 (kiểm bằng mắt ở 1024 và 1440).
- Đề xuất: thêm `lg:max-h-[calc(100dvh-2rem)] lg:overflow-y-auto` cho `<section>`, hoặc cho khối nút submit `sticky bottom-0 bg-surface` bên trong thẻ. Sau đó kiểm ở 1024×768.

## Low

1. **Enter trong ô có thuộc tính `form` sẽ submit trên trình duyệt thật.** Áp dụng cho ô ngân sách, "Nhân vật chính" và "Bối cảnh".
   - Theo HTML spec, default button là nút submit đầu tiên có form owner là form, kể cả nút nối qua `form=` (`config-panel.tsx:158-160`).
   - Test không bắt được hành vi này: user-event chỉ tìm `form.querySelector('button[type=submit]')` trong con của form, nên probe ra 0 request.
   - Ô ngân sách: hành vi giống form cũ, chấp nhận được.
   - Hai ô text trong "Tuỳ chỉnh thêm" (`prompt-section.tsx:238-245`) là hành vi mới. User gõ tên nhân vật rồi Enter sẽ tạo truyện ngay và bắt đầu tốn tiền. Có thể chặn Enter ở `TextOption`.
2. **Nút "Hoàn tác" tự biến mất kể cả khi đang có focus.** Vị trí: `prompt-section.tsx:29-33`.
   - User bàn phím Tab tới nút thì nút mất, focus rơi về body.
   - Đề xuất: dừng timer khi nút có focus hoặc đang hover.
3. **Ước tính có hai chỗ hiện sai.**
   - Ngân sách nhỏ hơn chi phí 1 chương thì hiện "≈ 0 chương" (`new-book-logic.ts:145-146`).
   - Lúc `useBooks` đang tải, `books.data ?? []` (`new-book-form.tsx:138`) làm hiện thoáng câu "Chưa có số đo…", dù có thể đã có số đo.
   - Đề xuất: thêm câu riêng cho trường hợp `max === 0`, và không hiện gì khi `books.isPending`.
4. **Ô chọn model có hai lỗi nhỏ về a11y.** Vị trí: `config-panel.tsx:190-191`.
   - `aria-describedby` trỏ tới `${ids}-badges`, nhưng phần tử này không render khi model không có badge (idref treo).
   - `errors.model` không có node lỗi riêng. Ô select bị disabled nên bước "focus ô lỗi đầu tiên" không làm gì. Câu `noModels` vẫn được đọc nên ảnh hưởng nhỏ.
5. **Keys refetch thất bại sau khi lưu key (đã có ít nhất 1 key) thì form dùng nhầm key cũ.**
   - `keyId` trỏ tới key mới nhưng key đó chưa có trong list. `?? keys.data[0]` (`new-book-form.tsx:69`) lặng lẽ chọn key cũ, select hiện key cũ và submit gửi key cũ.
   - Trường hợp refetch chỉ chậm thì an toàn: `onSuccess` của `useCreateKey` trả về promise `invalidateQueries`, `mutateAsync` chờ promise này xong mới gọi `onSaved`.
   - Đề xuất: `keyId ? find(...) : keys.data[0]`. Không tìm thấy thì để `keyId` báo lỗi.
6. **Có hai control cùng accessible name "API key"** khi mở "Thêm key khác": ô select và ô password của AddKeyForm. Nên đặt nhãn khác cho select, ví dụ "Key dùng cho truyện này".
7. **Còn vài kích thước arbitrary**: `text-[0.9375rem]`, `text-[1.75rem]`, `text-[1.0625rem]`, `!w-30`, `min-h-13`. Phase 5 không có token cỡ chữ nên chấp nhận được.

## Kiểm theo yêu cầu

**(a) Success Criteria**

| Tiêu chí | Kết quả |
|---|---|
| 1. User 0 key tạo được truyện mà không rời trang | Đạt, có test. |
| 2. Chip gợi ý không làm mất chữ đã gõ | Không đạt, xem H1. |
| 3. Tuỳ chỉnh thêm ghép vào prompt, có xem trước, API không đổi | Đạt. |
| 4. Thuê bao không có ô ngân sách; trả theo token có chip, ước tính, chi tiết giá | Đạt. |
| 5. Model đề xuất chọn sẵn; bậc giá chỉ khi có ít nhất 3 model có giá | Đạt. |
| 6. ≥1024px: 2 cột, nút "Bắt đầu viết" luôn thấy được | Mới đạt phần 2 cột. Phần "luôn thấy được" chưa được chứng minh, xem M1. |

**(b) Hồi quy**
- Body gửi đúng 5 trường; có test kiểm tập key.
- Thuê bao gửi `budget_usd: 0`.
- Ngoài các file phase 8, `msg.newBook.*` chỉ được dùng ở `routes/_app/books/new.tsx` (`title`). Các key bị xoá không còn chỗ nào dùng (typecheck pass).

**(c) Contract công khai:** không đổi. Không sửa `add-key-form`, `provider-icon`, `provider-picker`.

**(d) Pattern**
- Có `aria-invalid` và `aria-describedby` trên textarea và ô ngân sách.
- Có focus vào lỗi đầu tiên, `role="status"` cho thông báo hoàn tác, lỗi 507 `disk_full` hiện câu chung.
- Selector `[&_[role=radiogroup]]:grid-cols-2` có specificity 0,2,0 nên thắng `md:grid-cols-3`. Cách này phụ thuộc cấu trúc DOM bên trong ProviderPicker; đã có comment giải thích, chấp nhận được.

**(e) Edge case**

| Trường hợp | Kết quả |
|---|---|
| Đổi key khi AddKeyForm đang mở | OK. Kết quả kiểm vẫn hiện tới khi bấm "Xong" và có ghi tên key, nên không gây hiểu nhầm. |
| Key mới chưa có trong list do refetch chậm | An toàn, `onSaved` chạy sau khi refetch xong. Chỉ còn trường hợp refetch lỗi (Low 5). |
| Timer hoàn tác khi unmount | Có cleanup. |
| `priceTiers` với giá 0 hoặc giá trùng | Đúng: loại model giá 0, cùng giá thì cùng bậc, hai mức giá thì ra rẻ và chất lượng. |
| `estimateChapters` chia cho 0 | Có chặn (`chapters <= 0 \|\| !(cost > 0)`). |
| Nội dung ghép dài hơn 4000 ký tự | Client chặn bằng cùng quy tắc đếm như server (`books/handlers.go:188`). Bộ đếm hiện độ dài sau ghép (P4). |
| Enter trong ô ngân sách | Có submit trên trình duyệt thật, xem Low 1. |

Status: DONE_WITH_CONCERNS
Summary: Phase 8 chạy đúng và verify xanh, nhưng chip gợi ý vẫn làm mất chữ đã gõ khi bấm lại cùng chip hoặc bấm liên tiếp hai chip (H1, đã tái hiện). Ngoài ra chưa chứng minh nút "Bắt đầu viết" luôn thấy được ở màn ≥1024px thấp (M1). Cần sửa H1 trước khi đóng phase.
