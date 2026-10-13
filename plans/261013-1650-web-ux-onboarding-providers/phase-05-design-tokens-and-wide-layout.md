---
phase: 5
title: "Design tokens + layout rộng + tiêu đề tab"
status: completed
priority: P1
effort: "0.5d"
dependencies: [1]
---

# Phase 5: Design tokens + layout rộng + tiêu đề tab

## Overview
Làm nền thị giác cho các phase UI sau: token bóng, màu phụ ấm, thang chữ, bo góc, bảng màu "bìa" truyện. Nới khung nội dung lên ≤1600px (reader giữ 65ch). Dialog có cỡ lớn hơn. Tiêu đề tab theo route, thêm tiền tố "(n)" với n = số truyện đang cần user xử lý (audit 1.2, **không** dùng Notification API).

Có thể chạy song song với phase 2–4, nhưng **chỉ trong `git worktree` riêng**: `make gen` của phase 2 sinh field TS bắt buộc mới và có thể làm typecheck của phase này đỏ tạm thời. Không có worktree riêng thì chạy tuần tự sau phase 4. Trong worktree mới, chạy `pnpm -C web install --frozen-lockfile` trước, vì `node_modules` chưa có. <!-- Updated: Validation Session 1 - lệnh pnpm theo packageManager và CI -->

<!-- Red Team: định nghĩa lại "(n)", thêm queries.ts, điều kiện chạy song song theo phát hiện 12, 13 — 2026-10-13 -->

## Context
- Audit, mục "Hướng thị giác" 1–8, và mục 1.2: `plans/reports/ux-audit-261013-1645-web-app-improvements-report.md`
- Brainstorm mục 5 (layout rộng).
- Hiện trạng: Tailwind 4.3 (`@theme` trong `web/src/styles/app.css`), token màu trong `web/src/styles/tokens.css` (light + dark), font Be Vietnam Pro (UI) + Literata (serif). Khung: `web/src/components/app-shell.tsx:28` (`max-w-7xl`), `:48` (`max-w-6xl` cho trang truyện, `max-w-3xl` cho trang khác), reader dùng nhánh `kind === 'reader'`. Dialog: `web/src/components/ui/dialog.tsx:67` (`max-w-md`, `rounded-[18px]`).
- Hiện có nhiều cỡ px lẻ (`text-[13px]` ×28, `text-[15px]` ×15, `rounded-[14px]` ×7…).

## Requirements
- Giữ tinh thần "Thư phòng mực": teal là màu hành động duy nhất, Be Vietnam Pro + Literata. Không đổi màu chủ đạo, không đổi font.
- `tokens.css` thêm (cả light và dark):
  - Bóng: `--shadow-card: 0 1px 2px rgb(0 0 0/.04), 0 4px 12px rgb(0 0 0/.04)`, `--shadow-card-hover` đậm hơn một bậc. Dark mode: bóng `none`, giữ viền.
  - Màu phụ ấm: `--ink-warm: #9a5b1e` / dark `#e3a96b`; `--ink-warm-bg: #f7eee3` / dark `#2e2216`. Kiểm tương phản AA trên nền tương ứng.
  - 6 tông "bìa" trầm: teal, chàm, gạch, rêu, mận, đất (`--cover-1..6`), mỗi tông có bản dark.
- `app.css` `@theme`: thang chữ `xs 12 · sm 14 · base 16 · lg 18 · xl 22 · 2xl 28 · 3xl 36`; bo góc `--radius-control: 10px`, `--radius-card: 16px`, pill; ánh xạ các token trên thành utility (`shadow-card`, `bg-warm`, `text-warm`, `rounded-card`, `rounded-control`…). Chữ thân ≥14px, ô nhập 16px (chống zoom iOS).
- Helper `web/src/lib/cover-tone.ts`: `coverTone(id: string): 1..6` (hash ổn định) + test.
- `app-shell.tsx`: một khung `max-w-[1600px]` cho mọi trang trừ reader; padding trang 16 (mobile) → 24 (≥768) → 32 (≥1280); bỏ nhánh `max-w-3xl`/`max-w-6xl`. Trang nào cần cột hẹp (form, settings) sẽ tự giới hạn bên trong ở phase sau. Mobile 375px không tràn ngang.
- `dialog.tsx`: prop `size?: 'md' | 'lg' | 'xl'` (mặc định `md`, giữ tương thích), dùng `rounded-card`. Cập nhật `dialog.test.tsx`.
- `components/ui/*`, `status-badge.tsx`: đổi px lẻ sang token mới. Thẻ/khối dùng `bg-surface shadow-card`, bỏ viền ở light mode. Khối nhấn có class dùng chung (vd. `.callout`: `bg-accent-soft` + viền trái 3px accent). Hover thẻ nâng bóng + `translateY(-1px)` 150ms, tắt khi `prefers-reduced-motion`.
- Tiêu đề tab (1.2):
  - `web/src/lib/use-tab-title.ts`: `useTabTitle(title?: string)` đặt `document.title = "[(n) ]{title} · {appName}"`.
  - n = `needsAttention(books).length`. Truyện cần xử lý là truyện ở trạng thái chờ duyệt, lỗi, hết ngân sách hoặc bị ngắt (`waiting_review`/`failed`/`budget_exhausted`/`interrupted`, khớp dải ở phase 7); lấy mã trạng thái thật trong `web/src/api/status.ts`. Hàm thuần `needsAttention` đặt ở `web/src/lib/needs-attention.ts`, dùng chung với dải "Cần bạn xử lý" ở phase 7. n = 0 thì không có tiền tố.
  - Không làm máy trạng thái so sánh giữa các lần poll lúc tab ẩn. TanStack Query v5 dừng `refetchInterval` khi tab ẩn (`refetchIntervalInBackground` mặc định false), và `refetchOnWindowFocus` đã tắt (`web/src/main.tsx:19`), nên cách đó luôn ra 0.
  - `useBooks` (`web/src/api/queries.ts:58-63`) thêm `refetchIntervalInBackground: true`, để tiêu đề vẫn cập nhật khi tab ẩn.
  - Gọi hook ở `app-shell.tsx`. Tiêu đề từng route: map đơn giản theo pathname trong shell, hoặc route tự gọi `useTabTitle`. Chọn cách ít file nhất. Chuỗi đặt trong `messages.ts` khối `title`.
  - Test unit cho `needsAttention` và hàm dựng tiêu đề (tách khỏi hook).
- Không đụng file của feature (`features/**`). Các phase sau tự đổi px lẻ trong file mình sở hữu.

## Related Code Files
- Modify: `web/src/styles/tokens.css`, `web/src/styles/app.css`
- Modify: `web/src/components/app-shell.tsx`, `web/src/components/ui/{dialog,button,field,states}.tsx`, `web/src/components/status-badge.tsx`, `web/src/components/ui/dialog.test.tsx`
- Create: `web/src/lib/cover-tone.ts` (+ `.test.ts`), `web/src/lib/use-tab-title.ts`, `web/src/lib/needs-attention.ts` (+ test)
- Modify: `web/src/lib/messages.ts` (chỉ khối `title`), `web/src/api/queries.ts` (chỉ thêm `refetchIntervalInBackground: true` vào `useBooks`)

## Implementation Steps
1. Đọc `tokens.css`, `app.css`, `app-shell.tsx`, `components/ui/*`.
2. Thêm token + `@theme`. Build thử để chắc utility sinh ra đúng.
3. Sửa shell + dialog + ui primitives.
4. `cover-tone.ts`, `needs-attention.ts`, `use-tab-title.ts` + test.
5. Chạy app (`pnpm -C web dev`, cần backend; nếu không có backend thì xem trang login/signup) ở 375 / 768 / 1440 / 1920px, light + dark.

## Tests / Verify
```bash
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] Token mới có đủ cặp light/dark. Chữ `--ink-warm` trên `--ink-warm-bg` và trên `--bg` đạt tỉ lệ tương phản ≥ 4.5:1 ở cả light và dark. Ghi 4 số đo vào báo cáo phase.
- [x] Khung nội dung ≤1600px, padding theo breakpoint, 375px không cuộn ngang. Reader không đổi.
- [x] Dialog nhận `size`. Test cũ pass.
- [x] Tab hiện tên trang. Có truyện cần xử lý thì tiêu đề có "(n)", kể cả khi tab đang ẩn. Xử lý xong thì n giảm.
- [x] Không còn `text-[Npx]`/`rounded-[Npx]` trong `components/`.

## Kết quả (2026-10-13)
Xong, verify pass. Báo cáo: [cook-cook-p05-report.md](./reports/cook-cook-p05-report.md), review: [from-code-reviewer-to-cook-p05-review-report.md](./reports/from-code-reviewer-to-cook-p05-review-report.md).
- Tương phản `--ink-warm`: light 4.70 (trên `--ink-warm-bg`) / 4.94 (trên `--bg`); dark 7.48 / 9.08.
- Lệch spec có chủ ý: khung chỉ thêm lề từ 768px (`md:px-6 xl:px-8`). Ở mobile các trang tự lo lề 16–20px vì có banner/thanh góp ý tràn sát mép. Từ md trở lên lề đang cộng dồn với `px-4/px-5` của trang; phase 6–9 bỏ lề ngang riêng của trang mình (xem plan.md).

## Risk Assessment
- Bỏ viền ở light mode có thể làm mờ ranh giới thẻ trên nền `--bg`. Kiểm bằng mắt; nếu cần thì giữ viền rất nhạt `--border` 50%.
- Nới khung làm các trang chưa sửa (form, settings) kéo dài dòng chữ tạm thời cho tới phase 6–8. Chấp nhận được vì làm tuần tự trong cùng đợt.
