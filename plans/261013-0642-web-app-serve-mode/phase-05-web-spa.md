---
phase: 5
title: "Web SPA"
status: pending
priority: P1
effort: "4d"
dependencies: [4]
---

# Phase 5: Web SPA

## Overview

Giao diện tiếng Việt, ưu tiên điện thoại, dễ dùng hơn TUI: tạo truyện từ một câu, xem engine viết realtime, đọc truyện đẹp, can thiệp và duyệt chương bằng nút bấm.

## Requirements

### Màn hình

| Route | Nội dung |
|---|---|
| `/login`, `/signup` | Form; signup có ô invite code |
| `/` | Thư viện truyện. Mỗi thẻ: tiêu đề, trạng thái (badge), thanh tiến độ chương, số chữ, chi phí $ / ngân sách. Nút "Truyện mới" |
| `/books/new` | Ô lớn "Bạn muốn viết truyện gì?" kèm vài gợi ý mẫu (chip). Chọn key, rồi chọn model từ `/api/providers` (chỉ model có giá). Ngân sách $ (mặc định 5). Công tắc "Duyệt từng chương". Chưa có key thì dẫn sang `/settings/keys` |
| `/books/$id` | Xem chi tiết bên dưới |
| `/books/$id/read/$n` | Trình đọc: chương trước/sau; cỡ chữ; serif/sans; nền sáng/sepia/tối (lưu `localStorage`, bọc try/catch) |
| `/settings/keys` | Danh sách key (provider, nhãn, ••••last4). Thêm key (phải nhập lại mật khẩu), xoá key |
| `/settings/account` | Đổi mật khẩu (phải có mật khẩu cũ), đăng xuất |

Màn hình `/books/$id`:
- **Header:** trạng thái; các nút Dừng / Tiếp tục / Xuất; công tắc "Duyệt từng chương" (gọi `PATCH`).
- **Tab Đang viết:** văn bản đang stream + timeline của agent.
- **Tab Chương:** danh sách chương đã xong.
- **Tab Dàn ý:** gọi `/outline` khi mở tab.
- **Ô "Góp ý cho AI" (steer):** cố định ở dưới cùng.
- **Banner theo trạng thái:**
  - `waiting_review`: "Chương N đã xong — Đọc / Viết chương tiếp".
  - `budget_exhausted`: "Đã dùng hết $X — Tăng ngân sách" (mở dialog gọi `PATCH`).
  - `paused` kèm `HasAdvanceHold`: hiển thị lý do lấy từ `AdvanceHoldReason`.
  - `failed`: hiện `last_error` rút gọn và nút "Thử lại".

### Non-functional

- Hoạt động ở màn hình 360px. Vùng chạm ≥ 44px. Dùng `100dvh` và safe-area.
- Mọi màn hình đều có trạng thái loading, lỗi và rỗng.
- Thông báo lỗi lấy từ `error.message` của API (đã là tiếng Việt).
- **Nội dung do LLM sinh ra là dữ liệu không tin cậy:**
  - Render markdown chương bằng `react-markdown` **không** dùng `rehype-raw`.
  - Link chỉ chấp nhận `http`/`https` và có `rel="noopener noreferrer"`.
  - Event và stream chỉ render dạng text node.
  - Không dùng `dangerouslySetInnerHTML` ở đâu cả. Thêm rule ESLint để chặn.

## Architecture

### Stack

Ghim version lúc cài, kiểm bằng `npm view`:
- Vite 8, React 19.
- TypeScript: thử 7.x; nếu không hợp với `@tanstack/router-plugin` hoặc `typescript-eslint` thì ghim 5.9.
- `@tanstack/react-router` + `@tanstack/router-plugin` (file-based, `autoCodeSplitting`), `@tanstack/react-query`.
- Tailwind v4 (`@tailwindcss/vite`), shadcn/ui (chỉ thêm component nào dùng đến).
- `@tanstack/react-virtual`, `react-markdown`.
- Font: `@fontsource-variable/literata`, `@fontsource-variable/be-vietnam-pro` (subset vietnamese + latin).
- Test: Vitest + Testing Library.
- pnpm.

### Kiểu dữ liệu và client

- Không dùng thư viện i18n. Chuỗi hiển thị để trong `src/lib/messages.ts`. Số, ngày, tiền dùng `Intl.*('vi-VN')`.
- Type API: `tygo` sinh `web/src/api/types.gen.ts` từ `internal/server/api`. Package này đã chứa DTO của worker/SSE (`Event`, `BookSnapshot`) và REST từ phase 1–4. Commit file sinh ra; `make gen` + CI kiểm diff.
- `src/api/client.ts`: fetch same-origin với `Content-Type: application/json`, ném `ApiError{code,message}`. 401 thì chuyển về `/login`. 429 thì báo "Thao tác quá nhanh, thử lại sau".

### Hook `src/features/book-stream/use-book-stream.ts`

Mở `EventSource('/api/books/{id}/events')`. Mỗi lần mở (kể cả kết nối lại) thì **xoá timeline**, vì server gửi lại snapshot + ring. Xử lý từng loại message:

| Message | Việc |
|---|---|
| `snapshot` | `setQueryData(['book', id])` |
| `status` | `setQueryData(['book', id])`. Nếu chuyển sang `waiting_review`/`completed`/`budget_exhausted` thì `invalidateQueries(['book', id, 'chapters'])` |
| `event` | Gom vào `Map<id, Event>` (upsert theo `id`, giữ thứ tự lần đầu xuất hiện), flush mỗi frame. Tối đa 2000 |
| `stream` | Lưu ở store ngoài React (`useSyncExternalStore`). Gặp `clear` thì xoá |

- Khi `CompletedCount` tăng thì `invalidateQueries(['book', id, 'chapters'])`.
- Không tự đóng EventSource khi truyện dừng. Truyện có thể được tiếp tục, server vẫn giữ kết nối và chỉ gửi ping.

### Hiển thị

- **Timeline:** dùng virtualized list. Tự cuộn xuống khi đang ở đáy, có nút "Mới nhất". Mỗi agent (Architect / Writer / Editor) có nhãn tiếng Việt và icon. Event có `finished=false` hiện spinner.
- **Văn bản stream:** append vào DOM theo từng frame. Đoạn đã xong bọc `React.memo`, kèm `content-visibility: auto`.
- **Reader:** font Literata, `max-width: 65ch`, `line-height: 1.75`, `font-size: clamp(1.0625rem, 1rem + .3vw, 1.25rem)`.
- **Design tokens:** khai báo một chỗ trong `src/styles/tokens.css` (sáng + tối), theo design đã chốt ở mục "Design đã chốt" bên dưới. Không tự bịa màu/font khác.

### Design đã chốt (13/10/2026)

- Canvas: https://claude.ai/artifact/EgXyM2GeFvdMeudAEDVoKC (riêng tư). Bản sao mã nguồn từng màn hình nằm ở `./design/*.dc.html` cạnh plan này — đây là nguồn chuẩn về layout, copy tiếng Việt, khoảng cách, trạng thái; dựng lại bằng React + Tailwind, không nhúng file `.dc.html`.
- Hướng "Thư phòng mực":
  - Font: **Literata** (tiêu đề, văn bản truyện, reader) + **Be Vietnam Pro** (UI). Tự host qua fontsource, subset vietnamese + latin.
  - Màu (sáng): nền `#F4F5F3`, bề mặt `#FFFFFF`, chữ `#15171A`, chữ phụ `#5A606A`, viền `#E2E4E7`, viền input `#D5D8DC`, accent `#0E6E62` (hover `#0A5249`), accent nhạt `#E2F0EC`.
  - Màu trạng thái (chữ / nền): đang viết `#0E6E62`/`#E2F0EC`; chờ duyệt `#3346A8`/`#E7EAF8`; hết ngân sách `#8A4B08`/`#FBEFDF`; lỗi `#B42318`/`#FCE9E7`; tạm dừng & xếp hàng `#4B5563`/`#ECEEF0`; hoàn thành `#FFFFFF`/`#15171A`. Agent chip: Writer = accent, Editor = `#3346A8`, Architect = `#7A3E12`/`#F3E8DE`, Người dùng = nền `#15171A`.
  - Reader: sáng `#FFFFFF`/`#1B1D21`, sepia `#F4ECDC`/`#3A2E21`, tối `#121417`/`#DADDE2`; cỡ chữ 15–24px, mặc định 18px.
  - Bo góc: thẻ 14–18px, input/nút 10–12px, pill 999px. Chạm tối thiểu 44px. Icon: stroke SVG inline (hoặc lucide-react), không emoji.
- Map màn hình → route: `Main` → `/`, `NewBook` → `/books/new`, `BookLive` → `/books/$id` (mobile), `Desktop` → `/books/$id` (≥ md), `ReviewWait` → banner `waiting_review` trong `/books/$id`, `Reader` → `/books/$id/read/$n`, `Keys` → `/settings/keys` + `/settings/account`, `StatusBanners` → component banner/badge/toast dùng chung. Login/Signup chưa có artboard: form đơn giản cùng ngôn ngữ thiết kế.

### Dev

Vite chạy ở :5173, proxy `/api` sang `127.0.0.1:8080` (đặt `proxyTimeout: 0` cho SSE). Go chạy với `AINOVEL_DEV=true`; khi đó cookie tên `ainovel_session`.

### Cấu trúc thư mục

```
web/
  package.json, pnpm-lock.yaml, vite.config.ts, tsconfig.json, eslint.config.js, index.html
  src/main.tsx, src/routes/**, src/api/{client.ts,types.gen.ts,queries.ts}
  src/features/{auth,books,book-stream,reader,keys}/**
  src/components/ui/** (shadcn), src/lib/{messages.ts,format.ts}, src/styles/{tokens.css,app.css}
tygo.yaml (ở gốc repo)
Makefile (target web, gen, build)
```

## Related Code Files

- Create: toàn bộ `web/` (trừ `embed.go` và `dist/.gitkeep` đã có từ phase 2), `tygo.yaml`, `Makefile`.
- Modify: `internal/server/api/*.go`, nếu UI cần thêm field (ghi rõ trong commit).

## Implementation Steps

1. Đọc `./design/*.dc.html`, viết `tokens.css` theo mục "Design đã chốt".
2. Scaffold Vite + Router + Query + Tailwind + shadcn + ESLint (rule cấm `dangerouslySetInnerHTML`) + Vitest. Cấu hình proxy dev. Kiểm `pnpm build` ra `web/dist` và Go nhúng được.
3. `tygo` + `client.ts` + các query hook.
4. Trang auth + guard cho route.
5. Màn hình quản lý key.
6. Thư viện truyện + màn hình tạo truyện (model lấy từ `/api/providers`).
7. Màn hình truyện:
   - `use-book-stream`, kèm test với EventSource giả: upsert start/finish cùng id, kết nối lại thì thay timeline.
   - Timeline, stream, steer.
   - Các banner theo trạng thái; dialog tăng ngân sách.
   - Dừng / tiếp tục, xuất file.
8. Reader + phần cài đặt đọc, kèm test: markdown chứa `<img onerror>` và `<script>` thì không render thành phần tử.
9. Rà lại trên 360px, dark mode, các trạng thái lỗi. Khi mất mạng hiện "Đang kết nối lại…".
10. Gate:
    - `pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build`.
    - Sau đó `go build` và chạy end-to-end trên trình duyệt bằng `/ak:agent-browser`, chụp màn hình các bước chính.

## Success Criteria

- [ ] 3 người thử chưa từng dùng CLI tự tạo được truyện, đọc chương 1 và bấm "Chương tiếp" mà không cần hướng dẫn.
- [ ] Mở 2 tab cùng một truyện: cả hai nhận event. Tắt mạng 30s rồi bật lại: timeline được dựng lại, không có dòng trùng, không còn spinner treo.
- [ ] Stream một chương khoảng 10k chữ trên Chrome có CPU throttle 4x: INP < 200ms.
- [ ] Không còn chuỗi hiển thị tiếng Anh/Trung sót lại (trừ tên model). Không có `dangerouslySetInnerHTML`.

## Risk Assessment

- **TS 7 chưa tương thích plugin:** ghim 5.9, quyết định ngay ở bước 2.
- **`Summary`/`Detail` của event dài và nặng kỹ thuật:** chỉ hiển thị `summary`, `detail` thu gọn.
- **Phạm vi UI dễ phình ra:** bám đúng bảng màn hình ở trên, việc khác đưa vào backlog.
