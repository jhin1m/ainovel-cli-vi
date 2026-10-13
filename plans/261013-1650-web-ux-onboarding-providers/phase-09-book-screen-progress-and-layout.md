---
phase: 9
title: "Trang truyện: tiến độ + layout"
status: completed
priority: P1
effort: "0.75d"
dependencies: [8]
---

# Phase 9: Trang truyện: tiến độ + layout

## Overview
Trang truyện phải cho user biết "AI đang làm gì, tới đâu". Thêm dải "Đang làm" với stepper 4 bước và một dòng tóm tắt của agent đang chạy (1.8). Dải số liệu gồm model + icon, token vào/ra, vòng tiến độ, chi phí (ẩn với truyện thuê bao). Bố cục tab "Đang viết" làm lại: màn rộng 2 cột (bản nháp ≤72ch | timeline); mobile gọn, SteerBox thành nút "Góp ý" mở sheet (1.9).

## Context
- Audit 1.8, 1.9. Brainstorm mục 4 (icon ở trang truyện) và mục 3 (thuê bao hiện token).
- Hiện trạng: `web/src/features/books/book-screen.tsx` (350 dòng: header, tab, bản nháp 42dvh + timeline 48dvh xếp dọc, cột stats 320px), `steer-box.tsx` (60, sticky ở mobile), `web/src/features/book-stream/stream-text.tsx` (36), `timeline.tsx` (113), `use-book-stream.ts`, `stream-store.ts`, `agent-meta.ts`. `msg.book.stats.model` có nhưng chưa dùng.
- Dữ liệu: `book.snapshot{phase, flow, runtime_state, is_running, in_progress_chapter, current_chapter, completed_count, total_chapters, total_word_count, model_name, provider, agents[{name,state,summary,tool}], total_input_tokens, total_output_tokens, total_cost_usd, budget_limit_usd, has_advance_hold, advance_hold_reason}`, `book.subscription`. Snapshot cập nhật qua SSE (stream store).
- Ý nghĩa `phase`/`flow`/`agents[].state`: đọc `internal/entry/worker/mapping.go` và `internal/host` (UISnapshot) để map đúng, không đoán.
- Dùng lại: `ProviderIcon` (phase 6), `progress-ring.tsx` (phase 7), `Dialog size` (phase 5, mobile là bottom sheet), token phase 5.
- `status-banner.tsx`, `budget-dialog.tsx` và menu "⋯" thuộc phase 11. Phase này không sửa logic của chúng, chỉ đặt vị trí trong layout nếu cần.

## Requirements
- `web/src/features/books/progress-stepper.tsx`: 4 bước "Lập dàn ý → Viết chương N → Biên tập → Chờ bạn duyệt". Hàm thuần `stepFromSnapshot(snapshot, status): {index, chapter}` + test (mỗi bước, snapshot null, truyện hoàn thành/lỗi). Bước hiện tại có `aria-current="step"`. Dưới stepper là một dòng `summary` của agent đang chạy (kèm tên agent theo `agent-meta.ts`), cập nhật realtime, `aria-live="polite"` có throttle (không đọc từng token).
- Dải số liệu dưới tiêu đề (thay cột stats 320px): vòng tiến độ "12/40 chương", số chữ, model (icon + tên), token vào/ra (`tabular-nums`, định dạng vi-VN). Không phải thuê bao thì hiện chi phí/ngân sách và nút đổi ngân sách hiện có. Thuê bao thì không hiện USD và nút ngân sách. Mobile thu thành một dòng gọn cuộn ngang được (không làm trang tràn ngang).
- Tab "Đang viết":
  - ≥1280px: lưới 2 cột. Trái là bản nháp (`max-w-[72ch]`, font serif như reader). Phải là timeline. Mỗi cột tự cuộn trong chiều cao viewport còn lại, không có 2 vùng cuộn lồng trong trang cuộn.
  - <1280px: bản nháp trước, timeline sau, chỉ cuộn trang (bỏ chiều cao cố định 42/48dvh nếu gây cuộn lồng).
  - Mobile: SteerBox thu thành nút "Góp ý" (FAB hoặc nút trong thanh dưới) mở `Dialog` dạng sheet chứa ô góp ý + hint. Desktop giữ SteerBox inline.
- Giữ hành vi stream hiện có (`use-book-stream`, auto-scroll, `role=log`). Không đổi `stream-store.ts`.
- `messages.ts`: khối `book` (stepper, stats). Đổi px lẻ sang token trong các file sở hữu.
- Nếu `book-screen.tsx` vẫn > khoảng 250 dòng thì tách `book-header.tsx` / `book-stats-strip.tsx` / `writing-tab.tsx`. Chừa sẵn chỗ (slot/prop `actions`) ở header cho menu "⋯" của phase 11.

## Related Code Files
- Create: `web/src/features/books/progress-stepper.tsx` (+ test), có thể thêm `book-stats-strip.tsx`, `writing-tab.tsx`, `book-header.tsx`
- Modify: `web/src/features/books/book-screen.tsx`, `steer-box.tsx`, `web/src/features/book-stream/stream-text.tsx`, `timeline.tsx`, `web/src/lib/messages.ts`
- Đọc (không sửa): `internal/entry/worker/mapping.go`, `web/src/features/book-stream/{stream-store,agent-meta,use-book-stream}.ts`, `status-banner.tsx`, `budget-dialog.tsx`

## Implementation Steps
1. Đọc `book-screen.tsx` và các file stream. Đọc `mapping.go` để chốt bảng phase → bước.
2. `stepFromSnapshot` + test, rồi `progress-stepper.tsx`.
3. Dải số liệu (dùng `ProviderIcon`, `ProgressRing`), xử lý nhánh thuê bao.
4. Làm lại layout tab "Đang viết" + sheet góp ý ở mobile.
5. Tách component nếu cần, chừa slot `actions` ở header.
6. Kiểm bằng mắt 375 / 1280 / 1600, light/dark, khi truyện đang chạy (SSE) và khi dừng. Nếu không chạy được backend thì ghi rõ chỉ kiểm qua test.

## Tests / Verify
```bash
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] Stepper hiện ở đầu trang. Đúng một bước có `aria-current="step"`, khớp `stepFromSnapshot` (có test cho từng bước). Dòng dưới stepper hiện tên agent và `summary` của agent đang chạy, hoặc ẩn khi không có agent nào chạy.
- [x] Hiện model + icon và token vào/ra. Thuê bao không hiện USD.
- [x] ≥1280px bản nháp không dài quá 72ch. Không còn cuộn lồng trong trang cuộn.
- [x] 375px: không tràn ngang (`document.documentElement.scrollWidth <= 375`). Góp ý mở qua sheet. Bản nháp không còn chiều cao cố định `42dvh`, và SteerBox không còn chiếm chỗ cố định dưới bản nháp.
- [x] Test stream hiện có vẫn pass.

## Risk Assessment
- Map sai phase → bước gây hiểu nhầm. Bảng map phải lấy từ code worker, có test cho từng nhánh.
- `aria-live` đọc quá dày. Throttle và chỉ đọc khi summary đổi.

## Kết quả (2026-10-13)
Đã xong. Báo cáo: [cook-cook-p09-report.md](./reports/cook-cook-p09-report.md), review: [from-code-reviewer-to-cook-p09-review-report.md](./reports/from-code-reviewer-to-cook-p09-review-report.md).
- Bước 1–6 xong. Đã kiểm bằng mắt trên app thật (fakeworker) ở 375 / 1280×800 / 1366×768 / 1600×1000, light + dark, lúc đang chạy (SSE) và lúc dừng/chờ duyệt/hoàn thành.
- Lệch spec có chủ ý:
  - 2 cột cao đúng một màn chỉ bật khi màn ≥1280px **và** phần dưới thanh tab còn ≥ 18rem (`use-columns-layout.ts`). Laptop thấp có banner thì trang xếp dọc, cuộn bình thường, không cuộn lồng.
  - Khi xếp dọc: bản nháp chỉ hiện 4 đoạn cuối kèm nút "Xem toàn bộ"; timeline vẫn là khung cuộn riêng 60dvh vì danh sách được ảo hoá.
  - Truyện hoàn thành: không bước nào có `aria-current` (mọi bước đã qua) + câu "Đã viết xong toàn bộ truyện.".
- Slot `actions` cho menu "⋯" của phase 11: prop `actions` của `BookHeader` (`book-header.tsx`).
