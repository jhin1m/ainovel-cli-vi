---
phase: 10
title: "Reader"
status: completed
priority: P1
effort: "0.5d"
dependencies: [9]
---

# Phase 10: Reader

## Overview
Theme reader mặc định theo hệ điều hành (1.10). Thêm tiện ích đọc cơ bản (1.11): phím tắt, header có tên truyện + tên chương, thanh tiến độ cuộn, nhớ chỗ đọc, thông báo ở chương mới nhất kèm nút "Viết chương tiếp" khi truyện đang chờ duyệt. Cột chữ giữ 65ch.

## Context
- Audit 1.10, 1.11.
- Hiện trạng: `web/src/features/reader/reader-page.tsx` (181 dòng, header chỉ "Chương n"), `reader-settings.ts` (46 dòng, theme mặc định `light`), `markdown-view.tsx`. Route `web/src/routes/_app/books/$id/read.$n.tsx`. App shell nhánh `kind === 'reader'` (full width, phase 5 không đổi). `msg.reader.last` có nhưng chưa dùng.
- Dùng lại: `web/src/lib/last-read.ts` (phase 7: `getLastRead`/`setLastRead`), `useBook`, `useChapters`, `useChapter`, `useBookCommand` (lệnh "viết tiếp" giống thẻ thư viện ở phase 7), tên chương lấy từ dữ liệu `useChapters`.

## Requirements
- **1.10:** chưa lưu cài đặt thì theme reader = `prefers-color-scheme`. Đã lưu thì giữ lựa chọn. Có test cho hàm đọc mặc định (mock `matchMedia`).
- **Phím tắt:** ← / → chuyển chương trước/sau, `Esc` về trang truyện. Bỏ qua khi focus trong input/textarea/contenteditable, khi có modifier, hoặc khi dialog cài đặt đang mở. Ghi phím tắt vào `aria-keyshortcuts` của nút tương ứng.
- **Header:** "Tên truyện · Chương N: Tên chương" (cắt gọn ở mobile).
- **Thanh tiến độ cuộn** mỏng ở đỉnh, `role="progressbar"` + `aria-valuenow`. Tắt chuyển động khi reduced-motion.
- **Nhớ chỗ đọc:** mở chương thì `setLastRead(bookId, n)`. Thẻ thư viện (phase 7) và trang truyện hiện "Đọc tiếp chương N" nếu có. Nếu phase 7 chưa nối vào trang truyện thì thêm một link nhỏ ở header trang truyện. Chỉ chạm đúng chỗ đó, không sửa layout phase 9.
- **Chương mới nhất:** cuối chương cuối cùng hiện `msg.reader.last` ("Đây là chương mới nhất"). Nếu truyện `waiting_review` thì thêm nút "Viết chương tiếp" (cùng lệnh với thẻ thư viện). Truyện `running` thì hiện "AI đang viết chương N" + link về trang truyện.
- Cột chữ giữ 65ch. Không đổi giao diện cài đặt reader ngoài theme mặc định.
- `messages.ts`: khối `reader`.

## Related Code Files
- Modify: `web/src/features/reader/reader-page.tsx`, `reader-settings.ts`, `web/src/lib/messages.ts`
- Create: `web/src/features/reader/use-reader-shortcuts.ts` (+ test), có thể thêm `reader-settings.test.ts`
- Có thể sửa nhỏ: `web/src/features/books/book-screen.tsx` hoặc `book-header.tsx` (chỉ link "Đọc tiếp chương N"), `book-card.tsx` (chỉ khi phase 7 chưa nối `lastRead`)

## Implementation Steps
1. Đọc reader files, `last-read.ts`, cách `book-card.tsx` gọi lệnh viết tiếp.
2. Theme mặc định + test.
3. `use-reader-shortcuts.ts` + test (phím, bỏ qua khi đang gõ).
4. Header, thanh tiến độ, `setLastRead`, khối chương mới nhất.
5. Kiểm bằng mắt 375 / 1440, light/dark/theme OS.

## Tests / Verify
```bash
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] Máy đang tối, mở reader lần đầu thì reader tối.
- [x] ←/→/Esc hoạt động, không bắt phím khi đang gõ.
- [x] Header có tên truyện + tên chương. Có thanh tiến độ cuộn.
- [x] Thư viện/trang truyện hiện "Đọc tiếp chương N" đúng chương vừa đọc.
- [x] Chương mới nhất có thông báo. Truyện chờ duyệt viết tiếp được từ reader.

**Trạng thái (2026-10-13):** xong, verify web + Go pass. Chi tiết: [reports/cook-cook-p10-report.md](./reports/cook-cook-p10-report.md), review: [reports/from-code-reviewer-to-cook-p10-review-report.md](./reports/from-code-reviewer-to-cook-p10-review-report.md).

## Risk Assessment
- Phím ←/→ có thể đụng cuộn ngang hoặc trình đọc màn hình. Chỉ bắt khi không có modifier và focus không ở trong control.
