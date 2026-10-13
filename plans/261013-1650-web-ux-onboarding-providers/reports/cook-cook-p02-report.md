# Cook phase 2: provider Z.ai + chế độ thuê bao

Ngày: 2026-10-13 | Phase: [phase-02](../phase-02-backend-zai-providers-and-subscription.md) | Review: [from-code-reviewer-to-cook-p02-review-report.md](./from-code-reviewer-to-cook-p02-review-report.md) (8/10, DONE_WITH_CONCERNS → đã sửa High + Medium)

## Đã làm
- Worker: `Provider` thêm `Type`, `Subscription`. Bảng `webProviders` gồm `zai-coding` (Coding Plan, `https://api.z.ai/api/coding/paas/v4`, thuê bao) và `zai` (`https://api.z.ai/api/paas/v4`). Cả hai dùng `Type: "glm"` và BaseURL là hằng số. Nhãn web của `glm` là "GLM (bigmodel.cn)". Thêm `IsSubscription`. `buildConfig` truyền `Type`. `validateInit`: provider thuê bao bắt buộc `budget_usd = 0`, các provider khác giữ như cũ. `mapSnapshot(s, subscription)` đặt chi phí và trần ngân sách về 0, giữ nguyên token.
- Catalog: `curatedModels` thêm `zai-coding` (glm-4.7, glm-5.1) và `zai` (glm-4.7, glm-5.1, glm-4.6, glm-4.5-air). Thêm `recommendedModels` (mỗi provider 1 model) và `priceOverrides` cho `zai` theo bảng giá Z.ai. `PricingFor(provider, model)` chỉ thay phần giá, giữ Provider/ID của mục registry, nên `applyPricing` vẫn khớp.
- API: `ProviderInfo.subscription`, `ModelOption.recommended`, `Book.subscription` (suy ra từ provider trong `scanBook`). Đã chạy `make gen`.
- Books: khi tạo truyện thuê bao thì bỏ qua `budget_usd` và lưu 0. Với provider khác, `validBudget` giữ nguyên nhưng chạy sau bước tra key. PATCH budget của truyện thuê bao trả 400 `budget_not_applicable`.
- Supervisor: interface đổi thành `PricingFor(provider, model)`. `sendInit` ép budget 0 cho truyện thuê bao.
- Fakeworker: `BudgetUSD == 0` thì bỏ kiểm ngân sách và không cộng chi phí.
- Migration `0005_books_budget_subscription.sql`:
  - Up: nới CHECK thành `>= 0` rồi đặt lại budget 0 cho `zai-coding`.
  - Down: đổi budget 0 thành 0.01 rồi dựng lại CHECK `> 0` (validated).
- Web: thêm `subscription`/`recommended` vào 2 fixture test. Không sửa UI.
- Tests mới:
  - worker: `TestDecodeInitSubscription` (registry thật), `TestBuildConfigZaiProviders`, `TestMapSnapshotSubscriptionZeroesCost`; mở rộng `TestProvidersAllowlist` (thứ tự, nhãn, cờ).
  - keys: `TestCatalogZaiAndRecommended`, `TestPricingForZaiOverride`.
  - db: `TestMigration0005BudgetSubscriptionUpDown`.
  - flow: `TestBookSubscription` (tạo không gửi budget → 201, budget 0, subscription=true, chạy tới `completed`, chi phí 0, PATCH → 400, kiểm `pg_constraint`).

## Nguồn docs z.ai (kiểm 2026-10-13, ghi trong comment `models.go`)
- devpack/overview:
  - Coding Plan chỉ nhận GLM-5.3 và GLM-5.3-Flash.
  - `glm-4.7` được chuyển sang GLM-5.3-Flash.
  - `glm-5.1`/`glm-5.2` được chuyển sang GLM-5.3.
  - Registry chưa có glm-5.3 nên dùng hai mã cũ.
- devpack/quick-start: endpoint OpenAI là `https://api.z.ai/api/coding/paas/v4`.
- guides/llm/glm-4.7 và glm-5.1: context 200K, output tối đa 128K. Khớp với registry (202752).
- guides/overview/pricing:
  - glm-4.6/4.7: 0.6 / 2.2 (cache 0.11)
  - glm-5.1: 1.4 / 4.4 (cache 0.26)
  - glm-4.5-air: 0.2 / 1.1 (cache 0.03)
  - Khác giá registry nên phải override.

## Sửa theo review
- **High:** Down của 0005 dùng `NOT VALID`. Postgres vẫn kiểm ràng buộc này ở mọi UPDATE, nên binary cũ lỗi ở `recoverAll`/`claim` và không khởi động được. Đã đổi cách Down như mô tả ở trên. Test có thêm bước UPDATE dòng thuê bao sau Down.
- **Medium:** model đề xuất của `zai-coding` đổi từ `glm-5.1` sang `glm-4.7`. Gói vẫn trừ hạn mức theo token, và GLM-5.3 tốn khoảng gấp 3.
- **Low (giữ nguyên):**
  - Thứ tự lỗi khi tạo truyện thay đổi: budget sai + key không tồn tại giờ trả 404 thay vì 400. Phase yêu cầu tra key trước.
  - `IsSubscription` dựng lại danh sách mỗi lần gọi. Chi phí không đáng kể.

## Verify (đều pass)
- `go build ./... && go vet ./internal/server/... ./internal/entry/worker/...`
- `go test -count=1 ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL` (container ainovel-testpg). Test DB và flow chạy thật, không skip.
- `make gen`: diff `types.gen.ts` chỉ thêm 3 field. Phần bỏ invite là của phase 1, chưa commit.
- `pnpm -C web typecheck && lint && test (63/63) && build`

## Lưu ý cho phase sau
- UI chưa xử lý truyện thuê bao: sẽ hiện "$0 / $0", và dialog ngân sách gọi tới thì nhận 400. Phase 8–9 làm phần này. Không deploy riêng phase 2 lên prod.
- Rollback sau 0005: truyện thuê bao có budget 0.01. Binary cũ không có `zai-coding` nên các truyện này chỉ dừng với lỗi init. Phase 12 cần ghi điều này vào mục rollback của docs.
- Chưa kiểm bằng key thật. litellm `glm` + BaseURL z.ai sẽ kiểm ở phase 3/12; nếu lệch thì thử `Type: "openai"`.
- Không chạy docs-manager (docs thuộc phase 12), không commit (theo ràng buộc), không viết journal.

## Câu hỏi còn mở
- Có nên thêm glm-5.3/5.3-Flash khi registry có giá không? Hiện gói dùng mã cũ, phía Z.ai tự chuyển sang model mới.

Status: DONE
