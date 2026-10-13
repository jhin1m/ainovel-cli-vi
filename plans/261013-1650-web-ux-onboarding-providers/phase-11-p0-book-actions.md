---
phase: 11
title: "P0: xoá truyện, đổi key, lưu và viết tiếp"
status: completed
priority: P1
effort: "0.5d"
dependencies: [10]
---

# Phase 11: P0: xoá truyện, đổi key, lưu và viết tiếp

## Overview
Gỡ 3 chỗ user bị kẹt. (0.1) Xoá được truyện. (0.2) Đổi key khi key cũ bị xoá hoặc lỗi. (0.3) Nâng ngân sách xong thì viết tiếp ngay. Đăng ký mở (phase 1) nên user sẽ chạm `MaxBooksPerUser` thường hơn, vì vậy 0.1 bắt buộc phải có.

## Context
- Audit P0 0.1–0.3.
- Server có sẵn:
  - `DELETE /api/books/{id}` (`internal/server/books/handlers.go`). Trạng thái `deleting` có badge. Lỗi `book_limit` (L200) báo "hãy xoá bớt truyện cũ".
  - `PATCH /api/books/{id}` nhận `api_key_id` + `budget_usd`. Handler L226–241 đổi `budget_exhausted → paused` khi nâng ngân sách (L269).
  - Lệnh resume: `useBookCommand(id, 'resume')` trong `web/src/api/queries.ts` (kiểm tên lệnh thật trong file).
- Web:
  - `web/src/features/books/book-screen.tsx` (sau phase 9 có thể đã tách `book-header.tsx` với slot `actions`).
  - `status-banner.tsx` (138 dòng) + test. `budget-dialog.tsx` (59 dòng). `msg.book.budget.saved` có nhưng chưa dùng. `keys.confirmBody` nói "cho đến khi bạn chọn key khác" nhưng chưa có chỗ chọn.
- Truyện thuê bao (`book.subscription`) không có ngân sách. Nút ngân sách đã bị ẩn ở phase 9.
- Lỗi provider từ chối (phase 3): `book.stop_reason === 'provider_rejected'`, `book.last_error` chứa message tiếng Việt kèm mã gốc. **Không dò chuỗi `last_error`** để phân loại.
- Provider của truyện cố định từ lúc tạo. `PATCH api_key_id` khác provider bị chặn (`provider_mismatch`, `internal/server/books/handlers.go` ~L255-262, chuỗi lỗi ở L261). Vì vậy lỗi sai gói hoặc sai provider (1113) không sửa được bằng cách đổi key cùng provider.

<!-- Red Team: phân loại bằng stop_reason, hành động tạo truyện mới cho provider_rejected, hiện key đang dùng theo phát hiện 2, 6, 11 — 2026-10-13 -->

## Requirements
- **0.1 Xoá truyện:**
  - `queries.ts`: `useDeleteBook()` gọi `DELETE`, xong thì invalidate `books`, bỏ cache `book(id)`.
  - Menu "⋯" ở header trang truyện (đặt vào slot `actions`, cạnh nút Xuất). Menu dùng được bằng bàn phím: `aria-haspopup`, `aria-expanded`, Esc đóng, focus trở lại nút.
  - Mục "Xoá truyện" mở `Dialog` xác nhận: nêu tên truyện + "Không hoàn tác được. Mọi chương sẽ bị xoá." Nút nguy hiểm "Xoá vĩnh viễn". Xoá xong điều hướng về thư viện. Lỗi thì hiện trong dialog.
  - Lỗi `book_limit` ở trang tạo truyện: thêm link "Mở thư viện để xoá truyện cũ" (chỉ sửa chuỗi/khối lỗi trong `new-book-form.tsx`, không đổi layout).
- **0.2 Đổi key:**
  - `status-banner.tsx`: khi `book.api_key_id === ''` hiện banner "Key của truyện đã bị xoá". Khi `book.stop_reason === 'provider_rejected'` thì hiện "Nhà cung cấp từ chối key" kèm `last_error`, rồi chọn hành động theo nội dung lỗi:
    - Lỗi do gói hoặc provider (mã 1113): nút chính "Tạo truyện mới với provider khác" dẫn tới trang tạo truyện, điền sẵn prompt cũ nếu lấy được từ dữ liệu truyện. Nút phụ là chọn key cùng provider (trường hợp user có key khác đúng gói).
    - Các lỗi khác: banner có select key **cùng provider** (`useKeys` lọc theo `book.provider`, hiện icon) + nút "Dùng key này" (PATCH `api_key_id`) + nút phụ "Dùng key này và viết tiếp" (PATCH rồi resume nếu trạng thái cho phép). Chưa có key cùng provider thì hiện link tới form thêm key.
  - Menu "⋯" có mục "Đổi key" mở cùng nội dung trong dialog.
  - `change-key-panel.tsx` luôn hiện dòng "Key đang dùng: {label} …{last4}" (lấy từ `useKeys` theo `book.api_key_id`), để user nhận ra key lạ nếu phiên bị chiếm.
- **0.3 Lưu và viết tiếp:** `budget-dialog.tsx` có nút chính "Lưu và viết tiếp" (PATCH → resume) và nút phụ "Chỉ lưu". Xong thì hiện `msg.book.budget.saved`. Lỗi resume thì báo rõ nhưng vẫn giữ ngân sách đã lưu.
- `messages.ts`: khối `book` (menu, xoá, đổi key). Sửa `keys.confirmBody` cho đúng ("…cho đến khi bạn chọn key khác trong trang truyện").
- Test: banner key bị xoá có select + gửi PATCH; banner `stop_reason = provider_rejected` hiện "Tạo truyện mới với provider khác"; panel hiện key đang dùng; budget dialog gọi PATCH rồi resume; dialog xoá gọi DELETE và điều hướng.

## Related Code Files
- Modify: `web/src/api/queries.ts`, `web/src/features/books/book-screen.tsx` (hoặc `book-header.tsx`), `status-banner.tsx`, `status-banner.test.tsx`, `budget-dialog.tsx`, `web/src/features/books/new-book-form.tsx` (chỉ thông báo `book_limit`), `web/src/lib/messages.ts`
- Create: `web/src/features/books/book-actions-menu.tsx`, `delete-book-dialog.tsx`, `change-key-panel.tsx` (dùng chung cho banner + dialog), test tương ứng

## Implementation Steps
1. Đọc `queries.ts`, header trang truyện, `status-banner.tsx`, `budget-dialog.tsx`, handler PATCH/DELETE phía server (để biết trạng thái nào cho resume).
2. `useDeleteBook` + `delete-book-dialog.tsx` + `book-actions-menu.tsx`, gắn vào header.
3. `change-key-panel.tsx`, dùng trong banner và trong menu.
4. Budget dialog "Lưu và viết tiếp".
5. Link `book_limit` ở form tạo truyện.
6. Test + kiểm bằng mắt.

## Tests / Verify
```bash
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
go test ./internal/server/...   # bảo đảm hành vi PATCH/DELETE không đổi (không sửa server ở phase này)
```

## Success Criteria
- [x] Xoá được truyện từ trang truyện, có xác nhận, về thư viện.
- [x] Truyện mất key: banner cho chọn key cùng provider và chạy tiếp được.
- [x] Truyện bị provider từ chối (`stop_reason = provider_rejected`): banner nêu nguyên nhân và dẫn tới tạo truyện mới với provider khác.
- [x] Nâng ngân sách: một nút là truyện chạy lại.
- [x] Menu "⋯" dùng được bằng bàn phím.

## Risk Assessment
- Resume ngay sau PATCH có thể đụng trạng thái chuyển tiếp (`deleting`/`queued`). Chỉ hiện nút "và viết tiếp" ở trạng thái server cho phép, đọc từ handler.
- Xoá là thao tác không hoàn tác. Dialog phải nêu tên truyện, nút nguy hiểm không được là nút mặc định khi nhấn Enter.

## Bổ sung (orchestrator, sau phase 7)
- Dải "Cần bạn xử lý" (phase 7) có nút "Tăng ngân sách"/"Chạy lại" hiện chỉ mở trang truyện. Cho các nút này mở trang truyện kèm search param (vd. `?action=budget` / `?action=retry`) để trang truyện tự mở dialog ngân sách hoặc thực hiện đúng hành động; xoá param sau khi dùng. Có test.
- Stepper tiến độ (phase 9, `features/books/progress-stepper.tsx`): ẩn bước "Chờ bạn duyệt" khi truyện ở chế độ tự động (`advance_mode=auto`). Có test.

## Kết quả (2026-10-13)
- Hoàn thành, verify pass. Báo cáo: [reports/cook-cook-p11-report.md](./reports/cook-cook-p11-report.md), review: [reports/from-code-reviewer-to-cook-p11-review-report.md](./reports/from-code-reviewer-to-cook-p11-review-report.md).
- Đã làm cả hai mục "Bổ sung": `?action=budget|retry` từ dải "Cần bạn xử lý"; stepper ẩn bước "Chờ bạn duyệt" khi `advance_mode=auto`.
- Lệch phạm vi file có chủ ý: `new-book-form.tsx` thêm prop `initialIdea`/`avoidProvider` (điền sẵn prompt, chọn sẵn key provider khác), `routes/_app/books/new.tsx` và `routes/_app/books/$id/index.tsx` thêm `validateSearch`, `library-page.tsx`, `progress-stepper.tsx`, `api/status.ts` (`RESUMABLE`).
- Để lại: `internal/server/books/repo.go` `Count` còn đếm truyện `deleting` (~30s) nên xoá xong tạo ngay vẫn có thể dính `book_limit`. Phase này không sửa server.
