---
phase: 8
title: "Form tạo truyện"
status: completed
priority: P1
effort: "1d"
dependencies: [7]
---

# Phase 8: Form tạo truyện

## Overview
Làm lại trang tạo truyện cho người mới. Thêm key ngay tại trang. Gợi ý không ghi đè chữ đã gõ (1.3). Có "Tuỳ chỉnh thêm" ghép vào prompt, không đổi API (1.4). Ngân sách có ngữ cảnh: chip nhanh, ước tính số chương, giá token đưa vào chi tiết; provider thuê bao thì ẩn ngân sách (1.5). Model "Đề xuất" chọn sẵn, nhãn bậc giá chỉ hiện khi provider có đủ model để so (1.6). Màn rộng chia 2 cột với thẻ cấu hình sticky (1.7).

<!-- Red Team: bỏ P2 2.2, điều kiện bậc giá, loại truyện thuê bao khỏi ước tính, key inline lưu rồi tự kiểm theo phát hiện 1, 4, 15 — 2026-10-13 -->

## Context
- Audit 1.3–1.7; brainstorm mục 6 (thêm key inline). `plans/reports/ux-audit-261013-1645-web-app-improvements-report.md`.
- Hiện trạng: `web/src/features/books/new-book-form.tsx` (248 dòng: state `prompt/key/model/budget/advance`, chip `msg.newBook.ideas` ghi đè prompt, `<select>` key + model, ô budget mặc định `DEFAULT_BUDGET_USD`), `new-book-logic.ts` (validate, `parseBudget`), test `new-book-form.test.tsx`, `new-book-logic.test.ts`. Route `web/src/routes/_app/books/new.tsx`. Format: `web/src/lib/format.ts`.
- API: `POST /api/books {prompt, api_key_id, model, budget_usd, advance_mode}`. Provider thuê bao (`ProviderInfo.subscription`) thì server bỏ qua `budget_usd` (gửi 0). `ModelOption.recommended`, giá `input_cost_per_1m`/`output_cost_per_1m`.
- Dùng lại: `AddKeyForm` (`web/src/features/keys/add-key-form.tsx`, phase 6), `ProviderIcon`, token phase 5.

## Requirements
- **Bố cục (1.7):** ≥1024px chia 2 cột. Trái: ý tưởng, gợi ý, tuỳ chỉnh thêm, xem trước prompt. Phải: thẻ "Cấu hình" `sticky top-…` gồm key, model, ngân sách + ước tính, chế độ duyệt chương, nút "Bắt đầu viết". Mobile 1 cột, nút submit cuối form. Toàn trang giới hạn khoảng `max-w-6xl` bên trong khung 1600px.
- **Key inline:** chưa có key nào thì thẻ cấu hình hiện `AddKeyForm compact`. Lưu xong thì tự chọn key đó và đóng form. `AddKeyForm` tự kiểm key sau khi lưu (phase 6). Nếu kết quả là `plan_mismatch`/`auth_failed` thì giữ cảnh báo cạnh select key để user sửa trước khi bấm "Bắt đầu viết". Không chặn submit. Đã có key thì select key có icon provider + mục "+ Thêm key khác" mở `AddKeyForm` tại chỗ. Không phải rời trang.
- **Gợi ý (1.3):** ô trống thì chip điền thẳng. Ô có chữ thì thay thế và hiện "Hoàn tác" 5s (khôi phục nội dung cũ). Không dùng `confirm()`.
- **Mẫu theo thể loại (1.4):** `web/src/features/books/prompt-presets.ts` có khoảng 8 mẫu ý tưởng (tu tiên, ngôn tình, đô thị, trinh thám, kinh dị, khoa huyễn…), thay 4 ý tưởng cố định.
- **Tuỳ chỉnh thêm (1.4):** `<details>` thu gọn mặc định. Chip thể loại (6), độ dài (ngắn ~5–10 chương / vừa / dài), giọng văn (nghiêm túc / hài / u tối); ô "nhân vật chính", "bối cảnh". Hàm thuần `composePrompt(idea, options)` trong `new-book-logic.ts` ghép thành đoạn tiếng Việt nối sau ý tưởng (vd. "Thể loại: … Độ dài mong muốn: khoảng 5–10 chương. …"). Có khối "Xem trước nội dung gửi cho AI" (chỉ đọc). Không đổi API. Kiểm độ dài sau khi ghép vẫn ≤ giới hạn prompt hiện có.
- **Model (1.6):** đổi provider/key thì chọn sẵn model `recommended` (fallback model đầu). Option model hiện badge "Đề xuất" (màu `--ink-warm`) và nhãn bậc giá "rẻ / cân bằng / chất lượng" tính theo giá tương đối trong cùng provider (tertile của `input+output`, cùng giá thì cùng bậc). **Chỉ hiện bậc giá khi provider có ≥3 model có giá** (deepseek/grok chỉ có 1 model nên nhãn vô nghĩa, `internal/server/keys/models.go:22-25`). Provider thuê bao thì không hiện bậc giá. Có thể giữ `<select>` native (chữ kèm "· Đề xuất") hoặc dùng listbox tuỳ biến; ưu tiên native cho a11y/mobile.
- **Ngân sách (1.5):**
  - Provider thuê bao: ẩn ô ngân sách, hiện "Dùng quota gói Z.ai Coding Plan, tính bằng token, không trừ USD".
  - Provider khác: chip nhanh 2 / 5 / 10 / 20 US$ + ô số. Dưới ô hiện "≈ X–Y chương với model này" khi có số đo thật: chi phí/chương = Σ`total_cost_usd` / Σ`completed_count` của các truyện cùng `model` mà user đã có (`useBooks`), **loại truyện `subscription`** (chi phí của chúng là 0, sẽ kéo ước tính sai), Y = ngân sách / chi phí, X = 0.75·Y làm tròn. Chưa có số đo thì hiện "Chưa có số đo cho model này; ước tính xuất hiện sau chương đầu tiên." (không bịa hằng số). Hàm thuần `estimateChapters(budget, books, model)` + test (có ca truyện thuê bao cùng model bị loại).
  - Giá token chuyển vào `<details>` "Chi tiết giá" bằng lời thường ("$0.30 mỗi 1 triệu token đầu vào…").
- **Lỗi form:** focus ô lỗi đầu tiên. (Hiện lỗi `keyId` dưới select key là P2 2.2, để đợt sau theo brief.) Lỗi 507 `disk_full` từ server hiện thông báo chung "Máy chủ sắp hết dung lượng, thử lại sau".
- Nếu `new-book-form.tsx` vượt khoảng 200 dòng thì tách `prompt-section.tsx` + `config-panel.tsx` trong `features/books/`.
- `messages.ts`: khối `newBook`. Đổi px lẻ sang token.

## Related Code Files
- Modify: `web/src/features/books/new-book-form.tsx`, `new-book-logic.ts`, `new-book-form.test.tsx`, `new-book-logic.test.ts`, `web/src/routes/_app/books/new.tsx`, `web/src/lib/format.ts` (nếu cần định dạng USD/token), `web/src/lib/messages.ts`
- Create: `web/src/features/books/prompt-presets.ts`; có thể thêm `prompt-section.tsx`, `config-panel.tsx`
- Dùng lại (không sửa): `features/keys/add-key-form.tsx`, `components/provider-icon.tsx`

## Implementation Steps
1. Đọc form, logic, test, route, `add-key-form.tsx`, `types.gen.ts` (ProviderInfo/ModelOption/Book).
2. Logic thuần trước: `composePrompt`, `estimateChapters`, `priceTier`, `pickDefaultModel` + test.
3. `prompt-presets.ts`, phần prompt (gợi ý + hoàn tác + tuỳ chỉnh + xem trước).
4. Thẻ cấu hình (key inline, model, ngân sách/thuê bao, chế độ duyệt) + bố cục 2 cột sticky.
5. Cập nhật `new-book-form.test.tsx`: chip không ghi đè (có hoàn tác); 0 key → thấy form key; thuê bao → không có ô ngân sách, request gửi `budget_usd: 0`; model đề xuất được chọn sẵn.
6. Kiểm bằng mắt 375 / 1024 / 1440.

## Tests / Verify
```bash
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] User 0 key tạo được truyện đầu tiên mà không rời trang.
- [x] Chip gợi ý không làm mất chữ đã gõ (có hoàn tác 5s).
- [x] Tuỳ chỉnh thêm ghép đúng vào prompt, có xem trước. API không đổi.
- [x] Thuê bao: không có ô ngân sách. Trả theo token: chip ngân sách + ước tính (khi có số đo) + chi tiết giá.
- [x] Model đề xuất chọn sẵn. Nhãn bậc giá chỉ có ở provider có ≥3 model có giá.
- [x] ≥1024px: 2 cột, nút "Bắt đầu viết" luôn thấy được.

## Risk Assessment
- Ghép độ dài vào prompt chưa chắc Architect tôn trọng số chương. Ghi vào báo cáo phase để user thử 2–3 truyện. Không thêm trường API.
- Form phình to: tách component theo ranh giới (prompt / cấu hình), giữ state ở form cha.

## Bổ sung (orchestrator, sau phase 6)
- Thêm key inline ở trang tạo truyện dùng `AddKeyForm` của phase 6 như hiện có; KHÔNG chặn tạo truyện theo kết quả "Kiểm tra key" (kiểm là tuỳ chọn, có giới hạn). Không thêm callback `onChecked`.
- Phase 5 để lề trang cộng dồn ở ≥768px: bỏ lề riêng của các trang thuộc phase này ở ≥768px (xem ghi chú trong plan.md).

## Kết quả (cook, 2026-10-13)
- Xong cả 6 bước. Báo cáo: [reports/cook-cook-p08-report.md](./reports/cook-cook-p08-report.md). Review: [reports/from-code-reviewer-to-cook-p08-review-report.md](./reports/from-code-reviewer-to-cook-p08-review-report.md).
- Tách `prompt-section.tsx` + `config-panel.tsx`; state ở form cha.
- `<form>` tạo truyện chỉ bọc cột trái. Thẻ cấu hình nằm ngoài vì `AddKeyForm` là một `<form>`; các ô và nút gửi nối vào bằng `form={formId}`.
- Lệch spec có chủ ý:
  - "+ Thêm key khác" là nút dưới select, không phải option trong select.
  - Lỗi `keyId` hiện dưới khu vực key. Lỗi này chỉ xảy ra khi chưa có key nào; không hiện thì submit thất bại im lặng.
  - Icon provider đặt cạnh select, vì `<option>` native không chứa được SVG.
- Phase 11 chỉ sửa khối thông báo lỗi `book_limit`: nay là `serverError` trong `new-book-form.tsx` (truyền xuống `ConfigPanel`).
