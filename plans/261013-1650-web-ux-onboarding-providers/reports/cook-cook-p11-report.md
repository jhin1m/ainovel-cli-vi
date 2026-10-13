# Cook phase 11: P0 xoá truyện, đổi key, lưu và viết tiếp

Ngày: 2026-10-13 | Phase: [phase-11](../phase-11-p0-book-actions.md) | Review: [from-code-reviewer-to-cook-p11-review-report.md](./from-code-reviewer-to-cook-p11-review-report.md)

Review chấm 7.5/10: 0 Critical/High, 5 Medium, 10 Low. Đã sửa M1–M4 và L1, L2, L4, L5, L7, L8, L9. M5 (server) và các Low còn lại ghi ở cuối.

## Đã làm
- **0.1 Xoá truyện**
  - `queries.ts`: `useDeleteBook(id)` (DELETE, bỏ truyện khỏi `books` + invalidate) và `forgetBook(qc, id)`. Hàm này bỏ cache `book(id)` sau khi đã rời trang, để observer còn mount không tải lại truyện đã xoá.
  - `book-actions-menu.tsx`: nút "⋯" theo mẫu menu button WAI-ARIA, đặt vào slot `actions` của `BookHeader`.
    - `aria-haspopup` / `aria-expanded`.
    - Enter/Space/↓ focus mục đầu, ↑ focus mục cuối; ↑/↓/Home/End di chuyển.
    - Esc đóng và trả focus về nút; Tab hoặc bấm ra ngoài thì đóng.
    - Bị khoá (`deleting`) thì tự đóng.
  - `delete-book-dialog.tsx`: nêu tên truyện + "Không hoàn tác được. Mọi chương sẽ bị xoá.", nút nguy hiểm "Xoá vĩnh viễn".
    - Không dùng `<form>`, nên Enter không xoá.
    - Dùng `mutateAsync`, nên Esc giữa chừng vẫn điều hướng về thư viện.
    - Lỗi hiện ngay trong dialog.
  - `new-book-form.tsx`: lỗi `book_limit` có thêm link "Mở thư viện để xoá truyện cũ".
- **0.2 Đổi key**
  - `change-key-panel.tsx`: dùng chung cho banner và dialog.
    - Luôn có dòng "Key đang dùng: … ••••last4" (hoặc "không còn").
    - Select chỉ liệt kê key cùng provider, có icon.
    - "Dùng key này và viết tiếp" chỉ hiện khi trạng thái là paused/failed/interrupted, đúng tập `handleResume` cho phép.
    - Chưa có key khác thì có link `/settings/keys`.
    - Dùng `mutateAsync`, vì banner biến mất ngay khi PATCH xong.
  - `status-banner.tsx`:
    - `failed` + `stop_reason = provider_rejected`: banner "Nhà cung cấp từ chối key" kèm `last_error`. Nút chính dẫn tới `/books/new?from=id`, nút phụ (có `aria-expanded`) mở chọn key cùng provider.
    - `api_key_id === ''` (trừ completed/deleting): banner "Key của truyện đã bị xoá" có panel đổi key.
    - Hai banner có ô nhập/chọn này không còn là `role=status`.
  - Menu "⋯" → "Đổi key" mở dialog có panel đổi key và nút "Đóng".
  - Trang truyện (`book-screen.tsx`) hiện thông báo kết quả khi truyện còn ở trạng thái lúc thao tác xong (queued thì vẫn hiện khi đã running). Truyện `provider_rejected` không còn nút "Tiếp tục" ở header.
  - `keys.confirmBody`: "…chọn key khác trong trang truyện".
- **0.3 Lưu và viết tiếp**
  - `budget-dialog.tsx`: nút chính "Lưu và viết tiếp" (PATCH → resume), nút phụ "Chỉ lưu". Enter chọn nút chính (`SubmitEvent.submitter`).
  - waiting_review/completed thì chỉ có "Lưu ngân sách".
  - Resume lỗi thì báo "Đã lưu ngân sách mới nhưng chưa viết tiếp được: …" và giữ dialog mở.
  - Xong thì báo cho trang truyện qua `msg.book.budget.saved` / `savedResumed` / `savedPlain`.
- **Bổ sung**
  - Dải "Cần bạn xử lý" dẫn tới trang truyện kèm `?action=`: `budget_exhausted` → `?action=budget` (mở sẵn dialog), `interrupted` → `?action=retry` (resume đúng một lần, kể cả dưới StrictMode). Dùng xong thì xoá param (`replace`).
  - Stepper ẩn bước "Chờ bạn duyệt" khi `advance_mode=auto`, trừ khi truyện đang `waiting_review` thật.
- `/books/new?from=<id>`: điền sẵn ý tưởng bằng `prompt` của truyện cũ. Chọn sẵn key của provider khác provider vừa từ chối (M4, đảo ngược được).
- `RESUMABLE` chuyển sang `api/status.ts`.

## Lệch phạm vi file (có chủ ý)
- `new-book-form.tsx`: ngoài khối `book_limit`, thêm prop `initialIdea` và `avoidProvider`. Cần để làm yêu cầu "điền sẵn prompt cũ" và để không chọn lại đúng provider vừa từ chối.
- Thêm `validateSearch` ở `routes/_app/books/new.tsx` và `routes/_app/books/$id/index.tsx`. Sửa thêm `library-page.tsx`, `progress-stepper.tsx`, `api/status.ts`. Tất cả cho mục "Bổ sung" và chuyển `RESUMABLE`.
- Không sửa server.

## Tests
- `book-actions.test.tsx` (mới, 20 test):
  - Panel đổi key: key đang dùng, lọc cùng provider, PATCH → resume, resume lỗi, trạng thái không resume được, chưa có key.
  - Budget dialog: "Lưu và viết tiếp", Enter, "Chỉ lưu", resume lỗi, waiting_review.
  - Menu: bàn phím, Esc trả focus, bấm ra ngoài.
  - Dialog xoá: DELETE → về `/`, bỏ cache, nút nguy hiểm không có focus mặc định, lỗi ở lại trang.
  - BookScreen: menu mở hai dialog, `?action=budget`, `?action=retry` (StrictMode), retry bị chặn khi đang chạy, truyện mất key → viết tiếp + thông báo một lần, provider_rejected không có "Tiếp tục".
- `status-banner.test.tsx`: banner key bị xoá có select và gửi PATCH; completed không đòi key; provider_rejected có link `/books/new?from=b1` và nút phụ; lỗi thường có "1113" trong chuỗi vẫn là banner lỗi thường (không dò chuỗi).
- `progress-stepper.test.tsx`: auto có 3 bước; auto + waiting_review vẫn có bước 4; auto + completed.
- `library-page.test.tsx`: href `?action=budget`, `?action=retry`; "Xem lỗi" không có action.
- `new-book-form.test.tsx`: link `book_limit`; điền sẵn ý tưởng + chọn key provider khác.

## Verify
- `pnpm -C web typecheck && lint && test && build`: pass, 29 file / 252 test. Chạy lại sau khi sửa theo review.
- `go build ./... && go vet ./internal/server/... ./internal/entry/worker/...` và `go test -count=1 ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL` (ainovel-testpg): exit 0, mọi package ok. Phase này không đổi Go.
- App thật, kiểm trước khi sửa theo review: `serve` dev ở cổng 8111 + fakeworker, DB tạm `ainovel_p11` (đã drop), agent-browser headless.
  - 1440×900, truyện mất key: banner + select chỉ có Key B. Bấm "Dùng key này" thì về banner chờ duyệt và có "Đã đổi key.".
  - Menu "⋯" bằng bàn phím: Enter mở và focus "Đổi key", ↓ sang "Xoá truyện", Esc đóng và focus về nút.
  - Mục "Đổi key" mở dialog. Thấy dialog thiếu nút đóng trên mobile, đã thêm "Đóng".
  - `?action=budget` mở sẵn dialog. Một lần "Lưu và viết tiếp" thì truyện chạy lại tới hoàn thành, param bị xoá khỏi URL.
  - Dialog xoá: Enter không xoá. "Xoá vĩnh viễn" thì về `/`, truyện biến khỏi thư viện.
  - 375×812, provider_rejected (giả lập bằng UPDATE DB): banner đọc rõ, `scrollWidth = 375`. Link "Tạo truyện mới với provider khác" mở `/books/new?from=…` có sẵn ý tưởng.

## Chưa làm / để lại
- **M5 (server):** `books/repo.go` `Count` còn đếm truyện `deleting` (tới ~30s), nên xoá xong tạo ngay vẫn có thể dính `book_limit`. Phase này cấm sửa server. Đề xuất thêm `AND status <> 'deleting'` ở phase 12 hoặc đợt sau.
- L3: `?action=retry` dựa vào trạng thái lúc mở trang, có thể cũ. Server vẫn chặn bằng `cannot_resume` và lỗi hiện ở `cmdError`.
- L6: đổi key lúc truyện đang chạy chỉ có hiệu lực ở lượt sau. Đây là hành vi server, không đổi.
- L10: chưa có test ở mức route cho `/books/new?from=`. Đã kiểm trên app thật và test `NewBookForm` với prop.
- Không chạy docs-manager/git-manager/journal: docs thuộc phase 12, user cấm commit.

## Câu hỏi còn mở
- M5: sửa `Count` bỏ truyện `deleting` ở phase 12 hay để đợt sau?
- M4: đã chọn sẵn key của provider khác cho luồng "Tạo truyện mới với provider khác". Có giữ không?

Status: DONE_WITH_CONCERNS
