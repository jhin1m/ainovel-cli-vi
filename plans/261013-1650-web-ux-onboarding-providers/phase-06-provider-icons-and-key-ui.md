---
phase: 6
title: "Icon provider + UI key"
status: completed
priority: P1
effort: "0.75d"
dependencies: [4, 5]
---

# Phase 6: Icon provider + UI key

## Overview
Icon thương hiệu cho mọi provider (SVG inline, nguồn `@lobehub/icons-static-svg`, MIT). Lưới thẻ provider thay `<select>`. Form thêm key tách thành component dùng lại được (dialog ở trang key; phase 8 nhúng vào trang tạo truyện). Thêm nút "Kiểm tra key" gọi `POST /api/keys/test` (phase 4), chỉ cho key **đã lưu**.

<!-- Red Team: lưu rồi kiểm, tự kiểm sau khi lưu, hiện message server thay vì văn bản provider theo phát hiện 1, 8, 11 — 2026-10-13 --> Trang key và trang tài khoản dùng bề ngang rộng.

## Context
- Brainstorm mục 3 (nút kiểm tra key, nhãn provider), mục 4 (icon), mục 5 (settings 2 cột).
- API (sau phase 2–3, xem `web/src/api/types.gen.ts`): `ProviderInfo{name,label,subscription,models[]}`, `ModelOption{…, recommended}`, `TestKeyRequest{key_id}` (chỉ key đã lưu của chính user), `TestKeyResult{ok, kind, code, message, model}` (`kind` thuộc tập đóng: `ok`, `auth_failed`, `plan_mismatch`, `rate_limited`, `quota`, `network`, `unknown`; `message` do server viết). Lỗi giới hạn: 429 `rate_limited`/`busy`. Lỗi tạo key: 409 `key_limit` (tối đa 20 key). `POST /api/keys` không còn `current_password`.
- Hiện trạng: `web/src/features/keys/add-key-dialog.tsx` (107 dòng, `<select>` provider), `keys-page.tsx` (149), `account-page.tsx` (113). Query: `web/src/api/queries.ts` (`useProviders`, `useKeys`, `useCreateKey`, `useDeleteKey`). UI primitives + token mới từ phase 5 (`Dialog size`, `shadow-card`, `rounded-card`…). lucide-react không có icon thương hiệu.
- Provider name trong allowlist: `openrouter, anthropic, gemini, openai, deepseek, qwen, zai-coding, zai, glm, grok`.

## Requirements
- `web/src/components/provider-icon.tsx`:
  - `<ProviderIcon name size? className? />`, SVG inline, `aria-hidden` (tên provider luôn hiện bằng chữ bên cạnh).
  - Lấy SVG từ `@lobehub/icons-static-svg` (bản mới nhất ổn định trên unpkg/jsdelivr, vd. `https://unpkg.com/@lobehub/icons-static-svg@<ver>/icons/<slug>.svg`). Chỉ copy path, **không** thêm dependency. Header file ghi nguồn, phiên bản, giấy phép MIT + copyright LobeHub.
  - Map: openrouter, anthropic (claude), gemini, openai, deepseek, qwen, zai-coding + zai (icon Z.ai), glm (zhipu/chatglm), grok (xai). Ưu tiên bản màu nếu đọc tốt ở cả light/dark; bản mono thì dùng `currentColor`. Provider lạ → ô chữ cái đầu.
  - Test: render đủ 10 provider không lỗi, provider lạ có fallback.
- `web/src/features/keys/provider-meta.ts`: `keyUrl` (trang lấy key chính thức của từng provider) + mô tả 1 dòng (vd. Z.ai Coding Plan: "Gói thuê bao, tính bằng token, không trừ USD"). **Mở từng URL để xác nhận còn sống** trước khi ghi. Không có cảnh báo điều khoản.
- `web/src/features/keys/provider-picker.tsx`: lưới thẻ `role="radiogroup"`, mỗi thẻ `role="radio"`: icon + nhãn + badge "Thuê bao" (màu `--ink-warm`) nếu `subscription`. Phím mũi tên/Home/End điều hướng, roving tabindex, vùng chạm ≥44px. Lưới 2 cột (mobile) → 3–4 cột (≥768). Link "Lấy key ↗" nằm **ngoài** thẻ radio (tránh phần tử tương tác lồng nhau), mở tab mới `rel="noopener noreferrer"`.
- `web/src/features/keys/add-key-form.tsx`: chọn provider → nhãn (mặc định = nhãn provider) → API key (`type="password"`, có nút hiện/ẩn) → nút chính "Lưu key". Props `onSaved(key)`, `defaultProvider?`, `compact?`.
  - Lưu xong thì **tự gọi kiểm tra một lần** với `key_id` vừa tạo, để bắt lỗi chọn sai provider (vd. key gói Coding Plan nhưng chọn `zai`) trước khi có truyện. Trạng thái: đang kiểm / OK ("Key hoạt động với {model}") / lỗi (hiện `message` do server viết, kèm `code` nếu có, vd. 1113). Nếu `kind = plan_mismatch` hoặc `auth_failed` thì gợi ý "Xoá key này và thêm lại dưới đúng provider".
  - Bị 429 khi kiểm thì hiện "Đã kiểm quá nhiều lần, thử lại sau". Key vẫn được lưu. `onSaved` vẫn được gọi, kết quả kiểm không chặn việc lưu.
  - Lỗi 409 `key_limit` hiện `msg.keys.limit` kèm link tới trang key.
  - Lỗi field gắn `aria-describedby`; lỗi đầu tiên được focus.
- `add-key-dialog.tsx`: chỉ còn là vỏ `Dialog size="lg"` bọc `AddKeyForm`.
- `queries.ts`: thêm `useTestKey()` (mutation, không invalidate).
- `keys-page.tsx`: mỗi key hiện icon + nhãn provider + label + `…last4` + ngày, kèm nút phụ "Kiểm tra" (gọi `useTestKey` với `key_id`, hiện kết quả ngay dưới thẻ). ≥1024px dùng lưới 2–3 cột. Empty state dẫn tới form thêm key. Đổi px lẻ sang token.
- `account-page.tsx`: ≥1024px chia 2 cột (thông tin tài khoản | đổi mật khẩu). Đổi mật khẩu vẫn cần mật khẩu cũ.
- `messages.ts`: khối `keys` (+ `account` nếu cần). Chuỗi tiếng Việt có dấu đầy đủ.

## Related Code Files
- Create: `web/src/components/provider-icon.tsx` (+ test), `web/src/features/keys/provider-meta.ts`, `web/src/features/keys/provider-picker.tsx` (+ test bàn phím), `web/src/features/keys/add-key-form.tsx` (+ test luồng kiểm/lưu)
- Modify: `web/src/features/keys/add-key-dialog.tsx`, `keys-page.tsx`, `account-page.tsx`, `web/src/api/queries.ts`, `web/src/lib/messages.ts`

## Implementation Steps
1. Đọc các file keys + `queries.ts` + `types.gen.ts` + `components/ui/*`.
2. Tải SVG (curl về scratchpad), rút gọn path, viết `provider-icon.tsx`.
3. `provider-meta.ts` (xác nhận URL).
4. `provider-picker.tsx` + test bàn phím (mũi tên chuyển chọn, Tab ra khỏi nhóm).
5. `useTestKey` + `add-key-form.tsx` + test (mock `fetch` theo kiểu `web/src/test/render.tsx` và các test hiện có). Test: lưu → tự gọi `/api/keys/test` với `key_id`; kiểm lỗi `plan_mismatch` hiện gợi ý; 429 không chặn `onSaved`; 409 `key_limit` hiện thông báo.
6. Gói dialog, sửa keys-page/account-page.
7. Kiểm bằng mắt 375 / 1440, light/dark.

## Tests / Verify
```bash
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] Mọi provider có icon. Chọn provider bằng lưới thẻ, dùng được hoàn toàn bằng bàn phím.
- [x] Lưu key xong thì tự kiểm và hiện OK/lỗi (có mã gốc, vd. 1113). Trang key có nút "Kiểm tra" cho từng key đã lưu. UI không bao giờ gửi key thô tới endpoint kiểm tra.
- [x] `AddKeyForm` dùng được ngoài dialog (phase 8 sẽ dùng).
- [x] Danh sách key có icon. Trang key/tài khoản dùng bề ngang ở ≥1024px, 375px không tràn.
- [x] File icon ghi rõ nguồn + MIT.

## Risk Assessment
- Logo thương hiệu: chỉ dùng để nhận diện provider (nominative use), không sửa logo.
- SVG màu có thể chìm trên dark mode. Dùng bản mono `currentColor` cho logo đó.
- Kích thước bundle: chỉ path cần thiết, ước tính < 15 KB.

## Kết quả (2026-10-13)
Hoàn thành. Báo cáo: [cook-cook-p06-report.md](./reports/cook-cook-p06-report.md), review: [from-code-reviewer-to-cook-p06-review-report.md](./reports/from-code-reviewer-to-cook-p06-review-report.md).
- Thêm ngoài danh sách file: `features/keys/key-check-status.tsx` (dùng chung cho form và trang key); sửa `components/ui/dialog.tsx` (focus trap bỏ qua phần tử `tabindex=-1`, review High) + 1 test.
- `AddKeyForm` có thêm prop `onClose?` (nút Huỷ/Xong). Sau khi lưu, form đổi thành khối kết quả; parent không cần giữ form mount để `onSaved` chạy.

