# Cook phase 5: design tokens + layout rộng + tiêu đề tab

Ngày: 2026-10-13 | Phase: [phase-05](../phase-05-design-tokens-and-wide-layout.md) | Review: [from-code-reviewer-to-cook-p05-review-report.md](./from-code-reviewer-to-cook-p05-review-report.md)

Review chấm 8/10, 0 Critical/High. Đã sửa M2 + L1. M1 chuyển thành việc của phase 6–9 (ghi trong plan.md). Các Low còn lại ghi ở dưới.

## Đã làm
- `styles/tokens.css` (light + dark):
  - `--ink-warm`/`--ink-warm-bg`; `--cover-1..6` (teal, chàm, gạch, rêu, mận, đất).
  - `--shadow-card(-hover)`. Dark dùng `0 0 #0000` thay `none` để danh sách box-shadow của Tailwind còn hợp lệ.
  - `--border-card`: light trong suốt, dark = `--border`.
  - `--radius-control` 12 → 10px.
- `styles/app.css`:
  - `@theme` thêm `line-card`, `warm`, `warm-bg`, `cover-1..6`, `shadow-card(-hover)`, `rounded-pill`.
  - Thang chữ: xl 22 / 2xl 28 / 3xl 36. xs/sm/base/lg giữ mặc định Tailwind, vốn đã là 12/14/16/18.
  - `.callout` nằm trong `@layer components`.
  - `.card-hover` không đặt trong layer, chỉ áp khi `hover: hover`, tắt khi `prefers-reduced-motion`.
- `app-shell.tsx`:
  - Khung `max-w-[1600px]`, bỏ nhánh `max-w-3xl`/`max-w-6xl`. Reader không đổi.
  - Lề `md:px-6 xl:px-8`.
  - `routeTitle(pathname)` + `useTabTitle`.
  - Bỏ `text-[15px]`.
- `ui/dialog.tsx`: prop `size` (`md` mặc định / `lg` / `xl`), `rounded-card`, `max-h-[90dvh] overflow-y-auto`. Backdrop có `sm:p-4` để dialog lớn không chạm mép.
- `ui/button.tsx`, `ui/field.tsx`, `ui/states.tsx`: đổi px lẻ sang token. Ô nhập `text-base` (16px). `ErrorState` dùng `shadow-card border-line-card`. `status-badge.tsx` không có px lẻ nên không đổi.
- `lib/cover-tone.ts` (FNV-1a → 1..6), `lib/needs-attention.ts` (`waiting_review`/`failed`/`budget_exhausted`/`interrupted`), `lib/use-tab-title.ts` (`formatTabTitle` thuần + hook; khi unmount thì trả tiêu đề về tên app).
- `messages.ts`: thêm khối `title`. `queries.ts`: `useBooks` thêm `refetchIntervalInBackground: true`.

## Tương phản (WCAG)
| Cặp | Light | Dark |
|---|---|---|
| `--ink-warm` trên `--ink-warm-bg` | 4.70 | 7.48 |
| `--ink-warm` trên `--bg` | 4.94 | 9.08 |

Chữ trắng trên 12 tông bìa đạt từ 5.79 đến 10.62.

## Tests mới
- `cover-tone.test.ts`: ổn định, nằm trong 1..6, rải đủ 6 tông.
- `needs-attention.test.ts`.
- `use-tab-title.test.ts`.
- `app-shell.test.ts`: `routeKind`, `routeTitle`.
- `dialog.test.tsx`: thêm ca `size`.

## Verify
- `pnpm -C web typecheck && lint && test && build`: pass, 13 file / 81 test. Đã chạy lại sau khi sửa theo review.
- `go build ./... && go vet ...`: pass.
- `go test ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL`: 16 package ok, 646 PASS, 0 FAIL, 0 SKIP. Flow test DB đã chạy.
- Grep `text-[Npx]|rounded-[Npx]` trong `web/src/components`: không còn.
- Chạy app thật (binary `serve` ở cổng 8091, DB tạm `ainovel_p05`, đã xoá sau khi kiểm):
  - 1920px, dark: khung rộng 1600px.
  - Tiêu đề "(2) Thư viện · AI Novel". Chuyển một truyện `failed` → `completed` thì sau 1 lượt poll còn "(1)".
  - Trang Cài đặt: "(1) Cài đặt · AI Novel".
  - **Chưa kiểm bằng mắt** 375/768/1440px và light mode: Chrome không thu được viewport (resize không có tác dụng; iframe và popup bị chặn). Dưới 768px, shell không thêm lề và không đặt chiều rộng mới. Thay đổi duy nhất ở mobile là chữ nút và pill tạo truyện 15 → 16px, nên rủi ro tràn ngang thấp.

## Lệch spec / để lại
- **M1, lề trang:** spec ghi 16 → 24 → 32 ở shell. Shell chỉ thêm lề từ md, vì các trang (thuộc phase sau) đã có `px-4`/`px-5` và có phần tử cần tràn sát mép ở mobile. Từ md trở lên lề đang cộng dồn (40–56px). plan.md đã ghi rõ việc cho phase 6–9: bỏ lề ngang riêng của trang ở md trở lên.
- **L3:** tab con trong `book-screen` lệch bo góc do `--radius-control` giờ là 10px. Phase 9 xử lý.
- **L4:** cỡ chữ mới (xl/2xl, nút 16px) lan sang các trang hiện có. Đây là chủ ý của thang chữ.
- **L5:** `msg.title.library`/`newBook` trùng chuỗi với `msg.nav`. Giữ nguyên để khối `title` tự đủ, theo plan.
- **L6:** trang login/signup ngoài shell, tiêu đề tab chỉ là "AI Novel".

## Câu hỏi chưa giải quyết
- Poll `/api/books` 15s ở mọi trang `_app` (kể cả reader và lúc tab ẩn) là yêu cầu của plan. Reviewer chỉ ra hai hệ quả:
  - Tab mở quên đóng sẽ làm mới phiên liên tục, nên hạn 7 ngày không hoạt động không bao giờ tới; chỉ còn hạn cứng 30 ngày.
  - Tốn pin/data trên điện thoại.

  Cách sửa: giãn poll lúc tab ẩn, vd. `refetchInterval: () => (document.hidden ? 60_000 : 15_000)`. Cách này giảm tải nhưng không giải quyết chuyện gia hạn phiên. Chưa đổi; cần user quyết.

Status: DONE_WITH_CONCERNS
