---
phase: 2
title: "Provider Z.ai + chế độ thuê bao"
status: completed
priority: P1
effort: "1d"
dependencies: [1]
---

# Phase 2: Provider Z.ai + chế độ thuê bao

## Overview
Thêm hai provider web: `zai-coding` ("Z.ai Coding Plan", thuê bao) và `zai` ("Z.ai", trả theo token). Đổi nhãn `glm` thành "GLM (bigmodel.cn)". Provider thuê bao không có ngân sách USD: truyện lưu `budget_usd = 0`, worker tắt ngân sách và báo chi phí bằng 0, UI hiện token. Catalog thêm cờ `recommended` cho model đề xuất (dùng ở phase 8).

<!-- Red Team: bỏ subscriptionModels / đổi chữ ký PricingFor; zero chi phí cho thuê bao; sửa fakeworker, fixture web, migration 0005 có Down; giá zai theo phát hiện 4, 10, 12, 14 — 2026-10-13 -->

## Context
- Brainstorm mục 3. Coding Plan chỉ hợp lệ với endpoint `/api/coding/paas/v4` và các model trong gói. Sai endpoint hoặc sai model thì provider trả 429 code 1113 "insufficient balance".
- Red-team (phát hiện 4, 10, 12, 14): `plans/261013-1650-web-ux-onboarding-providers/reports/from-red-team-to-planner-261013-1725-web-ux-plan-review-report.md`
- Hiện trạng:
  - Allowlist web: `internal/entry/worker/providers.go` (`allowedProviders`, BaseURL lấy từ `bootstrap.ProviderPresets()`). Preset `glm` không có BaseURL, nên litellm dùng mặc định `https://open.bigmodel.cn/api/paas/v4`.
  - litellm v1.8.10 có provider `glm`. Với z.ai thì dùng `Type: "glm"` + BaseURL cố định, vì `ProviderConfig.ProviderType()` ưu tiên `Type` (`internal/bootstrap/config.go:161-164`). `RequiresAPIKey` trả false khi có `Type`, nhưng không sao vì protocol đã bắt buộc `api_key` (`protocol.go:124`).
  - Context window: `ResolveContextWindow` có đường dự phòng tra theo tên model (`internal/bootstrap/config.go:472-485`), nên `zai-coding/glm-4.6` vẫn nhận 202752.
  - **Registry đã có các model GLM, giá > 0, context 202752** (`internal/models/models_generated.go:71-77`: glm-4.6, 4.7, 5, 5.1; provider `glm`). Lookup không có prefix trả entry đầu tiên khớp (`internal/models/registry.go:82`). `MergeModels` không ghi đè giá bằng 0 (`registry.go:127-131`). `applyPricing` bắt Provider/ID của mục giá phải khớp entry đã resolve (`internal/entry/worker/config.go:58-66`).
  - Engine tính chi phí theo tên model (`internal/host/usage.go:712`). Vì vậy truyện thuê bao vẫn có `TotalCostUSD` tính theo giá glm, và phải chặn ở tầng worker.
  - `internal/entry/worker/protocol.go:118-144`: validate init đòi `budget_usd > 0` và pricing có giá > 0.
  - `internal/server/keys/catalog.go` (`Providers`, `Allowed`, `PricingFor(model)`, `resolve` chỉ tra theo model id, `:96-112`) và `internal/server/keys/models.go` (`curatedModels`).
  - `internal/server/jobs/supervisor.go:407-445` `sendInit`. `internal/server/jobs/fakeworker/main.go:437-440` dừng `StopBudget` khi `cost + chapterCost > BudgetUSD`. Với budget 0 thì dừng ngay ở chương 1.
  - `internal/server/books/handlers.go:152` `validBudget`, `:176` create, `:226-241` patch budget.
  - DB: `internal/server/db/migrations/0003_books.sql:11` `budget_usd double precision NOT NULL CHECK (budget_usd > 0)`, tên ràng buộc mặc định của Postgres là `books_budget_usd_check`.
  - Web test dựng literal có kiểu: `web/src/features/books/new-book-logic.test.ts:5-8`, `web/src/features/books/new-book-form.test.tsx:17-20`. tygo sinh `bool` thành field TS bắt buộc.

## Requirements
- Allowlist theo thứ tự hiển thị: openrouter, anthropic, gemini, openai, deepseek, qwen, **zai-coding**, **zai**, glm, grok.
- Hai provider z.ai khai báo trong `providers.go` (chỉ dành cho web, không thêm vào preset CLI): `{Name, Label, Type: "glm", BaseURL: <hằng số>, Subscription}`. BaseURL là hằng số biên dịch sẵn, không bao giờ lấy từ input.
  - `zai-coding`: `https://api.z.ai/api/coding/paas/v4`, `Subscription: true`.
  - `zai`: `https://api.z.ai/api/paas/v4`, `Subscription: false`.
- Nhãn `glm` trên web: "GLM (bigmodel.cn)". Đổi trong bảng web của `providers.go`, không đổi preset CLI.
- `buildConfig` truyền `Type` vào `ProviderConfig`. Kiểm `cfg.ValidateBase()` vẫn pass.
- **Giá và ngân sách (giữ đơn giản):**
  - **Không** thêm `subscriptionModels`. **Giữ** chữ ký `PricingFor(model)` và cách kiểm giá trong `applyPricing`. Server gửi mục giá registry như mọi provider khác. Mục giá này là entry `glm`, giá > 0, nên init qua được.
  - Worker suy ra subscription từ allowlist (`LookupProvider(name).Subscription`). Không thêm field vào protocol.
  - Provider thuê bao: `budget_usd` phải bằng 0 (gửi khác 0 thì init lỗi). Ngân sách tắt (`BudgetConfig{BookUSD: 0}`; `budgetExhausted` vốn đã cần `BudgetLimitUSD > 0`).
  - `mapSnapshot` đặt `TotalCostUSD = 0` và `BudgetLimitUSD = 0` cho truyện thuê bao. Token vẫn giữ nguyên. Nhờ vậy server không lưu chi phí USD giả.
  - Provider khác: validate như cũ (budget > 0, giá > 0).
- **Catalog:**
  - `curatedModels["zai-coding"]`, `curatedModels["zai"]`, dùng các ID đã có trong registry. **Xác minh danh sách model trong gói, context window và max output với docs z.ai lúc làm** (WebFetch `https://docs.z.ai/devpack/overview`, `https://docs.z.ai/api-reference`, trang model GLM). Ghi URL nguồn và ngày kiểm vào comment. Model không có trong registry thì không đưa vào (giữ quy tắc "không có giá thì không hiện").
  - **Giá `zai`:** kiểm bảng giá Z.ai (`https://docs.z.ai` mục pricing). Nếu khác giá registry (lấy từ bigmodel/OpenRouter) thì thêm `priceOverrides map[string]map[string]api.ModelPricing` cho `zai`, ghi nguồn và ngày, và chỉ khi đó mới đổi thành `PricingFor(provider, model)` (cập nhật interface ở `supervisor.go:29` cùng fake trong test). Nếu không kiểm được giá thì `zai` chỉ hiện những model đã có giá kiểm được.
  - `api.ProviderInfo` thêm `Subscription bool json:"subscription"`. `api.ModelOption` thêm `Recommended bool json:"recommended"`. Mỗi provider chọn một model đề xuất qua map `recommendedModels` (model rẻ mà viết văn tốt, vd. gemini-2.5-flash, claude-sonnet-4-5, deepseek-chat, glm-4.6).
- **Books API:**
  - `api.Book` thêm `Subscription bool json:"subscription"`, suy ra từ provider khi chuyển row sang API (repo hoặc handler; không thêm cột).
  - Create: provider thuê bao thì bỏ qua `budget_usd` trong request và lưu 0. Provider khác thì `validBudget` như cũ.
  - Patch `budget_usd` cho truyện thuê bao → 400 `budget_not_applicable`.
  - `sendInit`: truyện thuê bao gửi `BudgetUSD: 0`.
- **Fakeworker (bắt buộc):** khi `init.BudgetUSD == 0` thì bỏ kiểm budget, khớp với `budgetExhausted` (`stop_reason.go:9-11`).
- **Migration `0005_books_budget_subscription.sql`:**
  - `-- +goose Up`: `ALTER TABLE books DROP CONSTRAINT IF EXISTS books_budget_usd_check, ADD CONSTRAINT books_budget_usd_check CHECK (budget_usd >= 0);`
  - `-- +goose Down`: `ALTER TABLE books DROP CONSTRAINT IF EXISTS books_budget_usd_check, ADD CONSTRAINT books_budget_usd_check CHECK (budget_usd > 0) NOT VALID;` (`NOT VALID` để Down không lỗi khi đã có truyện thuê bao).
  - **Đã đổi khi cook (review High):** `NOT VALID` vẫn bị Postgres kiểm ở mọi UPDATE, nên sau Down binary cũ lỗi ở `recoverAll`/`claim` và không khởi động được. Down thực tế: `UPDATE books SET budget_usd = 0.01 WHERE budget_usd = 0` rồi thêm `CHECK (budget_usd > 0)` validated; Up đặt lại `budget_usd = 0` cho `provider = 'zai-coding'`. Test 0005 có bước UPDATE dòng thuê bao sau Down.
  - Flow test đọc `pg_constraint` để chắc tên và biểu thức của ràng buộc.
  - Test Down/Up của 0005 đặt trong `internal/server/db/db_test.go`, cạnh test của 0004 (phase 1): chèn một truyện `budget_usd = 0`, Down về 0004 (không lỗi nhờ `NOT VALID`), rồi Up lại. <!-- Updated: Validation Session 1 - chốt chỗ đặt test migration -->
- `make gen` cập nhật `types.gen.ts`. Phase này sửa 2 fixture web (thêm `subscription`/`recommended`) để `pnpm -C web typecheck` pass, ngoài ra không sửa UI.

## Related Code Files
- Modify: `internal/entry/worker/providers.go`, `config.go` (`buildConfig` + Type), `protocol.go` (nhánh budget thuê bao), `mapping.go` (`mapSnapshot` zero chi phí), test tương ứng (`*_test.go` trong cùng package)
- Modify: `internal/server/keys/catalog.go`, `models.go`, `catalog_test.go`
- Modify: `internal/server/api/account.go` (ProviderInfo, ModelOption), `internal/server/api/books.go` (Book)
- Modify: `internal/server/jobs/supervisor.go` (`sendInit`), `internal/server/jobs/fakeworker/main.go` (bỏ kiểm budget khi 0)
- Modify (chỉ khi cần override giá `zai`): interface `PricingFor` trong `supervisor.go:29` + fake ở `supervisor_test.go`
- Modify: `internal/server/books/handlers.go`, `repo.go` (map Subscription)
- Create: `internal/server/db/migrations/0005_books_budget_subscription.sql`
- Generated: `web/src/api/types.gen.ts`
- Modify web test fixture: `web/src/features/books/new-book-logic.test.ts`, `web/src/features/books/new-book-form.test.tsx` (chỉ thêm field mới vào literal)
- Modify test: `internal/server/books_flow_test.go` (thêm ca truyện thuê bao), `internal/server/db/db_test.go` (Down/Up 0005)

## Implementation Steps
1. WebFetch docs z.ai: chốt danh sách model, context window và giá `zai`. Ghi kết quả vào comment `models.go`.
2. `providers.go`: thêm field `Type`, `Subscription` vào `Provider`. Tách bảng web: preset (openrouter…grok) + 2 mục z.ai + override nhãn glm. Giữ chữ ký `Providers()`/`LookupProvider()`.
3. `config.go`/`protocol.go`/`mapping.go`: nhánh thuê bao. Unit test:
   - init thuê bao budget 0 + giá registry → OK
   - init thuê bao budget > 0 → lỗi
   - init thường giá 0 → lỗi như cũ
   - `buildConfig` cho `zai-coding` có Type `glm` + BaseURL coding
   - `mapSnapshot` thuê bao → cost 0, token giữ nguyên
   - test dùng `models.NewModelRegistry()` thật, để bắt lỗi resolve theo tên
4. Catalog + models: `curatedModels`, `recommendedModels`, override giá `zai` nếu cần. Test: `zai-coding` có model; `glm` có nhãn mới; mỗi provider có tối đa 1 model `recommended`.
5. API types + `make gen` + sửa 2 fixture web.
6. Books handlers/repo + migration 0005 + fakeworker. Flow test: tạo truyện `zai-coding` không gửi budget → 201, `budget_usd = 0`, `subscription = true`, truyện chạy tới `completed`/`waiting_review` (không phải `budget_exhausted`); patch budget → 400.
7. `sendInit` gửi budget 0 cho thuê bao.

## Tests / Verify
```bash
go build ./... && go vet ./internal/server/... ./internal/entry/worker/...
go test ./internal/entry/worker/... ./internal/server/...
make gen
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] `GET /api/providers` có `zai-coding` (subscription=true, có model) và `zai`; `glm` hiện "GLM (bigmodel.cn)".
- [x] Không có đường nào để input người dùng đặt BaseURL/Type (allowlist là hằng số).
- [x] Truyện thuê bao tạo được với budget 0, chạy qua được chương 1 với fakeworker. Worker không áp ngân sách. Snapshot có token, chi phí USD bằng 0.
- [x] Model id, context window (và giá `zai`) có nguồn docs z.ai ghi trong comment.
- [x] Migration 0005 có Down chạy được kể cả khi đã có truyện budget 0.
- [x] Test mới pass, test cũ không phải nới điều kiện. `pnpm -C web typecheck` pass.

## Risk Assessment
- litellm `glm` với BaseURL z.ai có thể khác nhẹ (vd. tham số thinking). Kiểm thủ công bằng key thật ở phase 3/12. Nếu lệch thì thử `Type: "openai"` (z.ai tương thích OpenAI) và ghi lại lý do.
- Thuê bao vẫn đi qua mục giá của `glm` để init được. Chi phí chỉ bị đặt về 0 ở `mapSnapshot`. Chỗ nào khác đọc chi phí trực tiếp từ engine (log, export) cũng phải dùng snapshot đã map. Grep `TotalCostUSD` trong `internal/entry/worker` để chắc.
- Truyện thuê bao không có ngân sách chặn lượt chạy dài. Việc này giao cho cơ chế chia lượt theo thời gian ở phase 4.
- Chạy song song với phase 5 trong cùng working tree có thể làm typecheck của phase 5 đỏ tạm thời. Xem plan.md: chỉ chạy song song khi dùng worktree riêng.

## Kết quả cook (2026-10-13)
- Xong cả 7 bước. Báo cáo: [reports/cook-cook-p02-report.md](./reports/cook-cook-p02-report.md). Review: [reports/from-code-reviewer-to-cook-p02-review-report.md](./reports/from-code-reviewer-to-cook-p02-review-report.md).
- Giá `zai` khác registry → đã thêm `priceOverrides` và đổi sang `PricingFor(provider, model)` (interface `supervisor.go` + fake).
- Docs z.ai (kiểm 2026-10-13): Coding Plan chỉ nhận GLM-5.3/5.3-Flash; `glm-5.1` → GLM-5.3, `glm-4.7` → GLM-5.3-Flash. Registry chưa có glm-5.3 nên `zai-coding` = `glm-4.7` (đề xuất), `glm-5.1`. Không đưa `glm-4.6` vào gói (docs không nhắc).

