---
phase: 7
title: "Onboarding + thư viện"
status: completed
priority: P1
effort: "0.75d"
dependencies: [6]
---

# Phase 7: Onboarding + thư viện

## Overview
Trang chủ trả lời được câu "bắt đầu từ đâu". User mới thấy checklist "Bắt đầu trong 3 bước", tự tick theo dữ liệu thật. User đã có truyện thấy CTA tạo truyện, dải "Cần bạn xử lý", lưới thẻ truyện có "bìa", số liệu lớn, vòng tiến độ và nút nhanh theo trạng thái (audit 1.1). Thêm khối "Chạy trên máy bạn" (PA4). Tạo `lib/last-read.ts` để thẻ hiện "Đọc tiếp chương N" (reader ghi giá trị này ở phase 10).

## Context
- Brainstorm mục 6. Audit 1.1, "Hướng thị giác" 3–4, 7. Research PA4: `plans/reports/research-261013-1645-local-execution-report.md` (mục PA4: liên kết cài đặt, lệnh mẫu, so sánh hosted và local).
- Hiện trạng: `web/src/features/books/library-page.tsx` (90 dòng), `book-card.tsx` (101 dòng, cả thẻ là một `<Link>`). `useBooks` poll 15s, `useKeys` có sẵn. Route `web/src/routes/_app/index.tsx`.
- Dữ liệu thẻ: `Book{id, prompt, status, provider, model, budget_usd, subscription, snapshot}`. `snapshot{book_title, completed_count, total_chapters, total_word_count, in_progress_chapter, total_input_tokens, total_output_tokens, total_cost_usd}` có thể `null`. Trạng thái: `web/src/api/status.ts` (`queued, running, waiting_review, paused, budget_exhausted, completed, failed, interrupted, deleting`).
- Token/helper từ phase 5: `shadow-card`, `rounded-card`, `text-warm`/`bg-warm`, `.callout`, `lib/cover-tone.ts`. `ProviderIcon` từ phase 6.
- Lệnh cài CLI: lấy đúng từ `README.md` mục "## Cài đặt" (~L199). Đọc cả lưu ý L212 về script/`go install`/image để không ghi lệnh sai.

## Requirements
- **Checklist "Bắt đầu trong 3 bước"** (`web/src/features/onboarding/getting-started.tsx`):
  1. Thêm API key (xong khi `keys.length > 0`) → link `/settings/keys`, hoặc nói rõ có thể thêm ngay ở trang tạo truyện.
  2. Tạo truyện đầu tiên (xong khi `books.length > 0`) → `/books/new`.
  3. Theo dõi và đọc chương đầu (xong khi có truyện `completed_count ≥ 1`) → link tới truyện mới nhất.
  - Hiện khi chưa xong cả 3 bước. Ẩn hẳn khi đã xong (không lưu cờ "đã đóng"; YAGNI). Bước hiện tại được nhấn bằng `.callout`. Bước đã xong có dấu tick và vẫn đọc được bằng screen reader ("Đã xong").
- **Thư viện có truyện:**
  - Header: tiêu đề trang Literata 28–36px + nút chính "Viết truyện mới". Thương hiệu chỉ hiện ở <lg (desktop đã có ở sidebar).
  - Dải "Cần bạn xử lý" (`.callout`): lọc bằng `needsAttention(books)` từ `web/src/lib/needs-attention.ts` (phase 5 tạo; gồm `waiting_review` / `failed` / `budget_exhausted` / `interrupted`). Không tự lọc lại, để tiêu đề tab và dải luôn khớp. Mỗi dòng có nút hành động. Ẩn nếu rỗng. <!-- Red Team: dùng chung selector theo phát hiện 13 — 2026-10-13 -->
  - Lưới thẻ: 1 cột (<640) → 2 (≥640) → 3 (≥1024) → 4 (≥1440).
- **`book-card.tsx` làm lại:**
  - Dải "bìa" 4px trên đầu thẻ (hoặc ô chữ cái đầu 48px) theo `coverTone(book.id)`.
  - Tiêu đề (`snapshot.book_title` hoặc đoạn đầu prompt) là `<Link>` duy nhất của thẻ. Các nút là `<button>`/`<Link>` riêng, không lồng nhau.
  - Số liệu lớn (Literata 600, `tabular-nums`): "12/40 chương", "38.200 chữ" (định dạng `vi-VN`, có thể dùng `lib/format.ts`). Vòng tiến độ nhỏ SVG thay thanh 6px, có `aria-label` "x%".
  - Badge trạng thái + icon provider + tên model ngắn. Truyện thuê bao hiện token thay USD.
  - Nút nhanh theo trạng thái: chờ duyệt → "Đọc chương N" + "Viết tiếp" (gọi lệnh continue/approve hiện có trong `queries.ts` `useBookCommand`, xem `book-screen.tsx` để dùng đúng lệnh); hoàn thành → "Đọc từ đầu"; lỗi → "Xem lỗi" (tới trang truyện); có `lastRead` → "Đọc tiếp chương N".
  - Hover nâng bóng (phase 5), tôn trọng reduced-motion.
- **`web/src/lib/last-read.ts`:** `getLastRead(bookId): number | null`, `setLastRead(bookId, n)`. localStorage key `ainovel:last-read:{bookId}`, bọc try/catch, có test. Reader gọi `setLastRead` ở phase 10.
- **"Chạy trên máy bạn" (PA4)** `web/src/features/onboarding/run-locally.tsx`: thẻ gọn đặt cuối thư viện (và ở empty state), có `<details>` mở rộng:
  - 2–3 dòng so sánh: web (theo dõi trên điện thoại, server lo việc chạy) và CLI/TUI trên máy (dùng mọi provider kể cả base URL riêng, máy bạn chạy, không cần server).
  - Lệnh cài + lệnh chạy mẫu lấy từ README (khối code có nút copy, dùng `navigator.clipboard` trong try/catch). Link tới README trên repo `https://github.com/jhin1m/ainovel-cli-vi` (lấy từ lệnh `git clone` trong `README.md` mục "## Cài đặt"). **Không** lấy từ `go.mod`, vì module path `github.com/voocel/ainovel-cli` là của upstream tiếng Trung. <!-- Updated: Validation Session 1 - sửa nguồn URL repo -->
  - Không gọi backend.
- `messages.ts`: khối `onboarding` + `library`. Đổi px lẻ trong file mình sở hữu sang token.

## Related Code Files
- Create: `web/src/features/onboarding/getting-started.tsx` (+ test tick theo dữ liệu), `web/src/features/onboarding/run-locally.tsx`, `web/src/lib/last-read.ts` (+ test), `web/src/components/progress-ring.tsx` (dùng lại ở phase 9)
- Modify: `web/src/features/books/library-page.tsx`, `web/src/features/books/book-card.tsx`, `web/src/lib/messages.ts`
- Đọc (không sửa): `web/src/features/books/book-screen.tsx` (lấy đúng lệnh "viết tiếp"), `web/src/api/queries.ts`, `README.md`

## Implementation Steps
1. Đọc `library-page.tsx`, `book-card.tsx`, `queries.ts`, `status.ts`, `format.ts`, đoạn lệnh trong `book-screen.tsx`.
2. `last-read.ts`, `progress-ring.tsx` + test.
3. `getting-started.tsx` + test (3 tổ hợp dữ liệu: chưa có gì / có key / có truyện có chương).
4. Làm lại `book-card.tsx`. Test: thẻ chờ duyệt có 2 nút; không có `<a>` lồng `<button>`.
5. `library-page.tsx`: checklist / dải xử lý / lưới / run-locally.
6. Kiểm bằng mắt 375 / 1024 / 1440 / 1920, light/dark.

## Tests / Verify
```bash
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] User mới (0 key, 0 truyện) thấy checklist, bước 1 được nhấn. Thêm key xong thì tick bước 1 mà không cần tải lại (invalidate `keys`).
- [x] Thư viện có truyện: lưới 2–4 cột. Dải "Cần bạn xử lý" đúng trạng thái. Truyện chờ duyệt viết tiếp được ngay từ thẻ.
- [x] Thẻ có bìa màu ổn định theo id, số liệu lớn, vòng tiến độ, icon provider. Truyện thuê bao hiện token.
- [x] Khối "Chạy trên máy bạn" có lệnh đúng theo README, copy được.
- [x] Không có phần tử tương tác lồng nhau. Repo không có axe hay eslint-plugin-jsx-a11y, nên test của `book-card` assert `container.querySelector('a a, a button, button a, button button')` là `null`.

## Risk Assessment
- Lệnh "viết tiếp" từ thẻ phải giống hệt nút ở trang truyện. Đọc `book-screen.tsx` và tái sử dụng hook, không tự tạo endpoint.
- Poll 15s nên tick checklist bước 3 trễ tối đa 15s. Chấp nhận được.

## Bổ sung (orchestrator, sau phase 5)
- Poll `useBooks`: 15s khi tab đang hiển thị, 60s khi tab ẩn (`refetchInterval` theo `document.visibilityState`, vẫn `refetchIntervalInBackground: true` để tiền tố "(n)" hoạt động). Có test.
- `useCreateKey` (`web/src/api/queries.ts`): đặt `gcTime: 0` cho mutation để key thô không nằm trong cache React Query sau khi lưu.

## Kết quả (2026-10-13)
Đã xong. Verify web pass (22 file / 134 test), Go build/vet/test có DB pass. Báo cáo: [cook-cook-p07-report.md](./reports/cook-cook-p07-report.md), review: [from-code-reviewer-to-cook-p07-review-report.md](./reports/from-code-reviewer-to-cook-p07-review-report.md).
- Phase 10 chỉ cần gọi `setLastRead(bookId, n)` trong reader; thẻ đã nối sẵn "Đọc tiếp chương N" qua `getLastRead`.
- `ProgressRing` (`components/progress-ring.tsx`) sẵn cho phase 9.
