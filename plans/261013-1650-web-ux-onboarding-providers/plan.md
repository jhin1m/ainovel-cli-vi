---
title: "Đợt web UX: đăng ký tự do, Z.ai, icon provider, layout rộng, onboarding"
description: "Bỏ mã mời và mật khẩu khi thêm key; thêm provider Z.ai Coding Plan (thuê bao, tính bằng token) và Z.ai; giữ lỗi gốc của provider; icon provider; layout gần full; onboarding 3 bước; P0 + P1 từ audit UX; khối 'Chạy trên máy bạn'."
status: completed
priority: P1
effort: "7-8d"
tags: [web, server, go, react, ux, onboarding, providers, zai]
created: 2026-10-13
validated: 2026-10-13
blockedBy: []
blocks: []
---

# Đợt web UX + onboarding + providers

> **Trạng thái: ĐÃ VALIDATE (2026-10-13). Cả 13 phase (1–12 và 4b) đã xong (2026-10-13).** Còn việc user tự kiểm với key Z.ai thật và link DeepSeek (xem [báo cáo phase 12](./reports/cook-cook-p12-report.md)). Đã qua red-team và validate. Chi tiết ở `## Validation Log` và [báo cáo validate](./reports/from-validator-to-planner-261013-1734-web-ux-plan-validation-report.md).

## Overview

Bản web (`ainovel-cli serve` + SPA trong `web/`) đã chạy được (plan `261013-0642-web-app-serve-mode`, completed). Đợt này sửa các điểm khiến web khó dùng và lỗi Z.ai Coding Plan (429/1113). Engine giữ nguyên. Chỉ sửa lớp server, worker và SPA.

Nguồn (đọc khi cần chi tiết, mỗi phase đã tóm phần liên quan):
- Brainstorm đã duyệt: `plans/reports/brainstorm-261013-1634-web-app-ux-onboarding-zai-report.md`
- Audit UX: `plans/reports/ux-audit-261013-1645-web-app-improvements-report.md`
- Research chạy local: `plans/reports/research-261013-1645-local-execution-report.md` (chỉ lấy PA4)
- Brief: `plans/reports/briefs/brief-plan-web-ux-round.md`

## Quyết định đã chốt (không hỏi lại)

| Chủ đề | Quyết định |
|---|---|
| Đăng ký | Tự do. Bỏ mã mời ở API, form, CLI `serve invite`, và drop bảng `invite_codes` bằng migration. Không làm guest. |
| Thêm API key | Không cần mật khẩu. Đổi mật khẩu vẫn cần mật khẩu cũ. |
| Provider | Thêm `zai-coding` "Z.ai Coding Plan" (`https://api.z.ai/api/coding/paas/v4`, cờ subscription: không ngân sách USD, hiện token) và `zai` "Z.ai" (`https://api.z.ai/api/paas/v4`, tính theo token). `glm` đổi nhãn thành "GLM (bigmodel.cn)". Không hiện cảnh báo điều khoản. Giữ allowlist chống SSRF. |
| Lỗi provider | Worker giữ mã và nội dung lỗi gốc. 1113 hiện thông báo rõ và không retry. Có nút "Kiểm tra key". Xác minh mã model và context window với docs z.ai lúc làm. |
| Icon | SVG inline lấy từ `@lobehub/icons-static-svg` (MIT), không cài package. Lưới thẻ provider thay `<select>`. |
| Layout | Nội dung gần full (≤1600px). Reader giữ 65ch. |
| Onboarding | Checklist "Bắt đầu trong 3 bước". Thêm key ngay ở trang tạo truyện. |
| Audit UX | Làm toàn bộ P0 (0.1–0.3), toàn bộ P1 (1.1–1.11; mục 1.2 chỉ làm tiền tố tiêu đề tab, không dùng Notification API) và mục "Hướng thị giác". P2 để sau. |
| Local runner | Không làm đợt này. Chỉ thêm khối "Chạy trên máy bạn" (CLI) trong web và docs (PA4). |


Quyết định phụ do planner chốt (điểm còn mở trong báo cáo; đều đảo ngược được):
- Model "Đề xuất" (1.6): cờ `recommended` đặt trong catalog server, cạnh `curatedModels`, để chỉ có một nguồn sự thật.
- Ước tính số chương theo ngân sách (1.5): chỉ dùng số đo thật của user, tức chi phí/chương của các truyện cùng model và không tính truyện thuê bao. Chưa có số đo thì nói rõ "ước tính xuất hiện sau chương đầu tiên", không bịa hằng số.
- Độ dài truyện (1.4): ghép vào prompt, không thêm `target_chapters` vào API (YAGNI). Kiểm thủ công sau khi làm.
- Subscription: suy ra từ provider, không thêm cột DB. Truyện thuê bao lưu `budget_usd = 0`, worker báo chi phí USD = 0. Vẫn dùng mục giá registry để init. Migration nới CHECK thành `>= 0`. (Phase 2 đã đổi sang `PricingFor(provider, model)` vì giá Z.ai khác registry, đúng nhánh phase file cho phép; Down của 0005 đổi budget 0 → 0.01 thay vì `NOT VALID`, xem phase 2.)
- Giới hạn tài nguyên khi mở đăng ký (red-team, xem phase 1 và 4):
  - signup 10/ngày/IP, cộng limiter 30/phút cũ
  - tạo key 30/ngày/user, tối đa 20 key
  - "Kiểm tra key" chỉ áp cho key đã lưu của chính user: 3/phút + 20/ngày/user, 60/ngày/IP, tối đa 4 đồng thời; response chỉ có mã đóng
  - `llmretry` tối đa 10 lần hoặc 10 phút
  - chia lượt chạy `AINOVEL_MAX_RUN_MINUTES` (mặc định 60)
  - chặn tạo truyện khi đĩa sắp đầy
  - dọn tài khoản rỗng sau 30 ngày
  - rule rate limit của Cloudflare là bước deploy bắt buộc
  - Không làm captcha đợt này (user đã chốt khi validate).

## Phases

| # | Phase | Lớp | Effort | Phụ thuộc | Status |
|---|-------|-----|--------|-----------|--------|
| 1 | [Bỏ mã mời + thêm key không cần mật khẩu (kèm giới hạn)](./phase-01-backend-open-signup-and-key-auth.md) | Go + chạm nhẹ web | 0.75d | — | Completed |
| 2 | [Provider Z.ai + chế độ thuê bao](./phase-02-backend-zai-providers-and-subscription.md) | Go (+ 2 fixture web) | 1d | 1 | Completed |
| 3 | [Lỗi gốc provider + chặn retry vô ích](./phase-03-backend-provider-errors-and-retry.md) | Go | 1d | 2 | Completed |
| 4 | [Kiểm tra key + chống lạm dụng khi mở đăng ký](./phase-04-backend-key-check-and-abuse-limits.md) | Go | 1d | 3 | Completed |
| 4b | [Cắt lượt ở ranh giới chương + sửa test flaky + kiểm key không báo nhầm](./phase-04b-slice-at-chapter-boundary.md) | Go | 0.5d | 4 | Completed |
| 5 | [Design tokens + layout rộng + tiêu đề tab](./phase-05-design-tokens-and-wide-layout.md) | web | 0.5d | 1 | Completed |
| 6 | [Icon provider + UI key](./phase-06-provider-icons-and-key-ui.md) | web | 0.75d | 4, 5 | Completed |
| 7 | [Onboarding + thư viện](./phase-07-onboarding-and-library.md) | web | 0.75d | 6 | Completed |
| 8 | [Form tạo truyện](./phase-08-create-book-form.md) | web | 1d | 7 | Completed |
| 9 | [Trang truyện: tiến độ + layout](./phase-09-book-screen-progress-and-layout.md) | web | 0.75d | 8 | Completed |
| 10 | [Reader](./phase-10-reader-improvements.md) | web | 0.5d | 9 | Completed |
| 11 | [P0: xoá truyện, đổi key, lưu và viết tiếp](./phase-11-p0-book-actions.md) | web | 0.5d | 10 | Completed |
| 12 | [Docs + kiểm tra cuối](./phase-12-docs-and-final-verification.md) | docs + verify | 0.5d | 1–11 | Completed |

### Thứ tự và chạy song song

```
1 ──► 2 ──► 3 ──► 4 ──┐
│                     ├──► 6 ──► 7 ──► 8 ──► 9 ──► 10 ──► 11 ──► 12
└──► 5 ───────────────┘
```

- Mặc định chạy tuần tự 1 → 12, mỗi phase một pane Claude mới. Phiên cook vượt khoảng 35% context giữa phase thì dừng ở ranh giới bước, ghi tiến độ vào phase file rồi làm tiếp ở pane mới. Phase 3 và 4 là hai phase nặng nhất.
- Chỉ có một chỗ chạy song song được: phase 5 chạy cùng lúc với phase 2–4 (sau khi phase 1 xong), **và chỉ khi dùng `git worktree` riêng**. Lý do: `make gen` ở phase 2–4 sinh field TS bắt buộc mới, có thể làm typecheck của phase 5 đỏ tạm thời. Không có worktree riêng thì chạy tuần tự. Worktree mới chưa có `node_modules`: chạy `pnpm -C web install --frozen-lockfile` trước. Merge phase 5 về sau khi phase 4 xong rồi chạy lại verify web. Phase 5 chỉ đụng `web/src/styles/*`, `components/app-shell.tsx`, `components/ui/*`, `components/status-badge.tsx`, `lib/use-tab-title.ts`, `lib/needs-attention.ts`, `lib/cover-tone.ts`, `lib/messages.ts` (khối `title`), `api/queries.ts` (một dòng trong `useBooks`). Phase 2–4 không đụng các file đó.
- Phase 2, 3 và 4 đều sinh lại `web/src/api/types.gen.ts` (`make gen`), nên không chạy song song với nhau.

### File dùng chung giữa nhiều phase (sửa tuần tự, chỉ thêm khối của mình)

| File | Phase sửa | Quy tắc |
|---|---|---|
| `web/src/lib/messages.ts` | 1, 5–11 | Mỗi phase thêm hoặc sửa đúng namespace của mình (`auth`/`keys.limit`, `title`, `keys`, `onboarding`/`library`, `newBook`, `book`, `reader`). Không đổi tên key của phase khác. |
| `web/src/api/types.gen.ts` | 1, 2, 3, 4 | Chỉ sinh bằng `make gen`, không sửa tay. |
| `web/src/api/queries.ts` | 5, 6, 11 | 5 chỉ thêm `refetchIntervalInBackground: true` vào `useBooks`; 6 thêm `useTestKey`; 11 thêm `useDeleteBook`. |
| `web/src/lib/needs-attention.ts` | 5 (tạo), 7 (dùng) | 7 chỉ import, không định nghĩa lại danh sách trạng thái. |
| `web/src/features/books/new-book-logic.test.ts`, `new-book-form.test.tsx` | 2, 8 | 2 chỉ thêm field mới vào fixture; 8 viết lại test theo form mới. |
| `web/src/features/books/book-card.tsx` | 7, 10 | 10 chỉ nối "Đọc tiếp chương N" qua `lib/last-read.ts` mà 7 đã tạo (nếu 7 chưa nối sẵn). |
| `web/src/features/books/book-screen.tsx` (hoặc `book-header.tsx` nếu 9 tách) | 9, 10, 11 | 9 dựng lại layout và chừa slot `actions`; 10 chỉ thêm link "Đọc tiếp chương N"; 11 gắn menu "⋯". Phase 9 đã tách: header ở `book-header.tsx` (prop `actions` cho menu "⋯", prop `controls` cho nút dừng/tiếp tục/xuất). |
| `web/src/features/books/new-book-form.tsx` | 8, 11 | 11 chỉ sửa khối thông báo lỗi `book_limit` (biến `serverError`, hiện trong `config-panel.tsx`). |
| `internal/server/api/*.go` | 1, 2, 3, 4 | Mỗi phase sửa struct của mình. Sau đó chạy `make gen`. |
| `internal/server/jobs/proc.go` | 3, 4 | 3 sửa nhánh `provider_rejected` trong `outcome()`; 4 thêm nhánh `sliced` (chia lượt). |
| `internal/server/keys/handlers.go`, `repo.go` | 1, 4 | 1: limiter + trần khi tạo key; 4: route kiểm key + `DecryptOwned`. |
| `internal/server/auth/ratelimit.go`, `handlers.go` | 1, 4 | 1: `NewLimiterPer` + limiter signup; 4: export `IPKey`. |
| `internal/server/server.go` | 1, 4 | 1: sweep limiter tạo key; 4: route kiểm key, sweep limiter, dọn tài khoản, truyền hàm đo đĩa vào `books`. |
| `internal/server/books/handlers.go` | 2, 4 | 2: budget thuê bao + `budget_not_applicable`; 4: chặn đĩa 507. |
| `internal/server/jobs/supervisor.go` | 2, 4 | 2: `sendInit` budget 0 cho thuê bao; 4: chia lượt. |
| `internal/entry/worker/config.go` | 2, 4 | 2: `buildConfig` truyền `Type`; 4: export hàm dựng model cho kiểm key. |
| `internal/server/books_flow_test.go`, `auth_flow_test.go` | 1, 2, 4 | Mỗi phase thêm ca test của mình, không sửa assert của phase khác. |

### Việc phase 5 để lại cho phase 6–9
- Lề trang: `AppShell` thêm `md:px-6 xl:px-8` (mobile 0). Trang nào phase mình viết lại thì bỏ `px-4`/`px-5` ngang của khối ngoài cùng **ở md trở lên** (vd. `px-5 md:px-0`), để lề tổng là 16/24/32. Mobile giữ lề riêng của trang (banner, thanh góp ý tràn sát mép). Phase 6: `keys-page`, `account-page`; 7: `library-page`; 8: `new-book-form`; 9: `book-screen`.
- Dùng token mới thay cỡ px lẻ trong file mình sở hữu: `shadow-card`, `border-line-card`, `rounded-card`/`rounded-control` (control giờ 10px; tab con trong thanh tab của `book-screen` nên ~6px, phase 9), `text-warm`/`bg-warm-bg`, `bg-cover-1..6` + `coverTone(id)`, `.callout`, `.card-hover`. Dialog có `size="lg" | "xl"`.
- `needsAttention()` ở `web/src/lib/needs-attention.ts` (phase 7 dùng cho dải "Cần bạn xử lý").

## Lệnh verify chung

```bash
# Go (test DB tự skip nếu thiếu AINOVEL_TEST_DATABASE_URL; nếu có Postgres local thì đặt biến này để chạy đủ flow test)
go build ./... && go vet ./internal/server/... ./internal/entry/worker/...
go test ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...
make gen   # sinh lại web/src/api/types.gen.ts sau khi đổi internal/server/api
# Web (script lấy đúng từ web/package.json)
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

### Mục docs phase 12 phải ghi (từ phase 4 và 4b)
- `POST /api/keys/test`; kết quả `ok` cả khi provider từ chối tham số (400/422 không nói về key/số dư) hoặc trả 200 nhưng hết `max_tokens` trước khi có chữ.
- `AINOVEL_MAX_RUN_MINUTES` (mặc định 60, 0 = tắt chia lượt) và `AINOVEL_SLICE_GRACE_MINUTES` (mặc định 20, 0 = cắt ngay, bỏ chương đang viết dở; cả hai trong 0..10080). Lượt bị cắt dừng sau khi chương đang viết commit; quá thời gian ân hạn thì dừng hẳn. Slot có thể bị giữ tối đa MAX_RUN + SLICE_GRACE.
- 507 `disk_full`; dọn tài khoản rỗng sau 30 ngày; rule Cloudflare cho `/api/keys*`.

## Success Criteria (toàn đợt)

- [x] Đăng ký chỉ cần email + mật khẩu + tên. Không còn chuỗi "mã mời"/`invite` trong UI, API, CLI, docs. Ngoại lệ có chủ ý: migration 0002 và 0004, test migration trong `internal/server/db/db_test.go`, và mục "Nâng cấp lên bản này"/"Rollback" của `docs/web-server.md` (nhắc `invite_codes` bị drop).
- [x] Thêm key không có ô mật khẩu. Đổi mật khẩu vẫn đòi mật khẩu cũ. Tạo key và đăng ký đều có giới hạn theo ngày.
- [ ] Key Z.ai Coding Plan thật: lưu dưới `zai-coding` thì tự kiểm OK, tạo truyện chạy được chương 1, không có ô ngân sách, hiện token, chi phí USD = 0. *(Chưa kiểm: không có key thật. User tự kiểm theo checklist phase 12.)*
- [ ] Key sai gói hoặc sai endpoint (1113): thông báo nêu rõ nguyên nhân và mã gốc (`stop_reason = provider_rejected`), model chỉ bị gọi một lần, không retry ở tầng nào. *(Đã chứng minh bằng test đếm số lần gọi ở phase 3; chưa kiểm với key thật.)*
- [x] "Kiểm tra key" chỉ nhận key đã lưu của chính user, có giới hạn user/IP/ngày/đồng thời, response không chứa văn bản của provider.
- [x] Không user nào giữ slot job quá `AINOVEL_MAX_RUN_MINUTES` + `AINOVEL_SLICE_GRACE_MINUTES` (mặc định 60 + 20 phút) khi server hết slot và có truyện của user khác đủ điều kiện chạy đang chờ. Lượt bị cắt viết nốt chương đang dở rồi mới nhường slot. 429 kéo dài không giữ slot quá 10 phút.
- [x] Migration 0004/0005 có Down chạy được. Docs có quy trình nâng cấp và rollback.
- [x] Mọi chỗ chọn hoặc hiện provider có icon.
- [x] Màn ≥1440px dùng gần hết bề ngang (≤1600px). Mobile 375px không tràn ngang. Reader vẫn 65ch.
- [x] User mới đi từ đăng ký tới truyện đầu tiên mà không phải rời trang tạo truyện để thêm key.
- [x] P0: xoá được truyện; đổi được key khi key cũ bị xoá; "Lưu và viết tiếp" sau khi nâng ngân sách.
- [x] P1 1.1–1.11 đạt tiêu chí trong từng phase.
- [x] Mọi lệnh verify chung pass.

## Rủi ro chính

- Mã model Coding Plan và giá Z.ai đổi theo thời gian. Phase 2 phải xác minh với docs z.ai và ghi nguồn vào comment `curatedModels`.
- Nguyên nhân 429/1113 được suy từ docs và code. Cần key thật để xác nhận (phase 3, 4 và 12).
- agentcore có thể bọc lỗi model khiến cờ không-retry bị bỏ qua. Phase 3 phải chứng minh bằng test đếm số lần gọi.
- Đăng ký mở: limiter nằm trong bộ nhớ, mất khi restart. Lớp chặn chính là rule Cloudflare, bắt buộc ở bước deploy (phase 12).
- Rollback sau khi chạy migration 0004/0005: binary cũ trên DB mới sẽ hỏng signup và truyện thuê bao. Phải backup trước khi deploy, rollback bằng restore hoặc goose Down (phase 1 và 12).
- `messages.ts` bị nhiều phase sửa: làm tuần tự, mỗi phase chỉ sửa namespace của mình.

## Câu hỏi đã chốt khi validate (2026-10-13)

User đã chốt ba câu hỏi mà red-team để mở. Không hỏi lại:
- **Captcha / xác minh email:** không làm đợt này (phương án A). Nếu log cho thấy signup hàng loạt (vd. hơn 50 tài khoản/ngày, hoặc limiter 10/ngày/IP bị chạm thường xuyên) thì thêm Cloudflare Turnstile cho `/api/auth/signup` ở đợt sau.
- **Đổi key giữa `zai` và `zai-coding`:** không cho. Truyện chỉ đổi key cùng provider (`provider_mismatch` giữ nguyên). Truyện tạo nhầm provider thì hướng user tạo truyện mới (phase 3 và 11).
- **Con số giới hạn:** chấp nhận toàn bộ. Signup 10/ngày/IP (cộng 30/phút cũ). Tạo key 30/ngày/user, tối đa 20 key. Kiểm key 3/phút + 20/ngày/user, 60/ngày/IP, tối đa 4 đồng thời, chỉ key đã lưu. `AINOVEL_MAX_RUN_MINUTES` = 60. Chặn tạo truyện khi đĩa < 1 GiB. Dọn tài khoản rỗng sau 30 ngày.

Còn lại một việc xác minh, không phải quyết định: danh sách model Coding Plan (GLM-4.6/4.7/5.x), context window và giá Z.ai. Phase 2 kiểm với docs z.ai lúc làm.

## Red Team Review

### Session — 2026-10-13
**Báo cáo:** [reports/from-red-team-to-planner-261013-1725-web-ux-plan-review-report.md](./reports/from-red-team-to-planner-261013-1725-web-ux-plan-review-report.md)
**Findings:** 15 (15 chấp nhận, trong đó 5 chấp nhận một phần; các đề xuất con bị bác có ghi lý do trong báo cáo)
**Severity breakdown:** 4 Critical, 9 High, 2 Medium
**Không đảo ngược quyết định user nào** (đăng ký tự do, thêm key không mật khẩu, không captcha, thuê bao không ngân sách USD).

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | "Kiểm tra key" thành oracle thử key bị đánh cắp | Critical | Accept (bác chế độ key thô) | Phase 4, 6, 8, 12 |
| 2 | `proc.go` ghi đè message lỗi, 1113 không tới user | Critical | Accept | Phase 3, 11 |
| 3 | Hết slot job: 429 retry vô hạn + thuê bao chạy không giới hạn | Critical | Accept | Phase 3, 4, 12 |
| 4 | Thiết kế giá thuê bao hỏng và thừa | Critical | Accept | Phase 2, 8 |
| 5 | Sai tầng retry và sai đường thoát của worker | High | Accept | Phase 3 |
| 6 | Bỏ re-auth làm mất throttle tạo key, không có trần | High | Accept | Phase 1, 11 |
| 7 | Đường giải mã cho `key_id` không có sẵn, dễ thành IDOR | High | Accept | Phase 4 |
| 8 | Không giới hạn egress đồng thời; trả nguyên văn lỗi provider | High | Accept (bác Transport riêng) | Phase 4, 6 |
| 9 | Đăng ký mở: giới hạn theo tài khoản, không dọn, không chặn đĩa | High | Accept (bác semaphore argon2, LRU) | Phase 1, 4, 12 |
| 10 | Migration 0004/0005 và rollback trên prod | High | Accept (bác placeholder `invite_code`) | Phase 1, 2, 12 |
| 11 | Lời khuyên 1113 không làm được với truyện đã tạo | High | Accept | Phase 3, 6, 11 |
| 12 | `make gen` làm vỡ typecheck web và làn song song | High | Accept | Phase 2, plan.md |
| 13 | Bộ đếm "(n)" trên tiêu đề tab không chạy được | High | Accept | Phase 5, 7, plan.md |
| 14 | Giá `zai` lấy nhầm của `glm` | Medium | Accept | Phase 2 |
| 15 | Phase 8 lấn sang P2; bậc giá vô nghĩa | Medium | Accept (bác tách phase, bác đổi thứ tự) | Phase 8 |

### Whole-Plan Consistency Sweep
- Danh sách thay đổi quyết định: kiểm key chỉ theo `key_id`; giới hạn user/IP/ngày/đồng thời; bỏ `subscriptionModels` và giữ `PricingFor(model)`; lỗi dùng `stop_reason = provider_rejected` thay cho dò chuỗi; sửa retry ở lớp model của bootstrap thay vì `imp`/`llmretry`; "(n)" = số truyện cần xử lý; thêm phase 4; đổi số phase 4–11 thành 5–12.
- Đã grep toàn bộ plan tìm các thuật ngữ cũ: `subscriptionModels`, `PricingFor(provider`, `{provider, api_key}`, `6 lần/phút`, "không đổi limiter", "mà không lưu key", "bỏ dependency auth", "đường giải mã hiện có", dò `last_error`, "đổi trạng thái lúc tab ẩn", `phase 3/11`, `1 → 11`. Chỉ còn hai chỗ, đều có chủ ý: phase 2 ghi rõ "Không thêm `subscriptionModels`", và đổi sang `PricingFor(provider, model)` chỉ khi cần override giá `zai`.
- Đã kiểm `dependencies` của các phase (6 → [4, 5]; 12 → [1..11]), link trong `plan.md`, bảng quyền sở hữu file (thêm `proc.go`, `keys/*`, `auth/*`, `server.go`, `needs-attention.ts`, fixture web), và các tham chiếu chéo giữa phase (3 ↔ 4 ↔ 6 ↔ 8 ↔ 11 ↔ 12).
- `ak plan status`: 12 phase, được nhận đủ.
- Mâu thuẫn còn tồn đọng: **0**. Ba câu hỏi mở (captcha/xác minh email, PATCH `zai` ↔ `zai-coding`, các con số giới hạn) là quyết định sản phẩm và không chặn việc triển khai.

Thay đổi cấu trúc: thêm phase 4 mới (kiểm tra key + chống lạm dụng). Các phase cũ 4–11 được đánh số lại thành 5–12. Phase 3 đổi tên file thành `phase-03-backend-provider-errors-and-retry.md`.

## Validation Log

### Session 1 — 2026-10-13
**Báo cáo:** [reports/from-validator-to-planner-261013-1734-web-ux-plan-validation-report.md](./reports/from-validator-to-planner-261013-1734-web-ux-plan-validation-report.md)
**Kết quả:** ĐÃ VALIDATE, cook được.
**Số câu hỏi đã hỏi:** 0. Mọi quyết định lấy từ brief và từ phần user đã chốt: giới hạn red-team, không captcha, chỉ đổi key cùng provider. Không phát hiện điểm mới cần user quyết.

#### Đã kiểm
- Mỗi phase tự đủ để cook ở phiên mới: đạt. Đã thêm quy tắc dừng khi vượt khoảng 35% context.
- Lệnh verify: đã đổi sang `pnpm -C web <script>` chạy từ gốc repo, khớp `packageManager`, CI và Makefile. Đã sửa lỗi `cd web` làm hỏng grep/`go test` chạy sau nó (phase 1, 11). Phase 2–4 đã có đủ lint/test.
- Phụ thuộc thứ tự: frontmatter khớp sơ đồ.
- File song song: chỉ cặp 5 ∥ 2–4 (trong worktree), không trùng file. Bảng file dùng chung đã bổ sung 5 dòng.
- Acceptance criteria: đã sửa 5 chỗ mơ hồ thành đo được (phase 5, 7, 9 ×2, 12).

#### Verification Results
- Claims checked: ~65
- Verified: 59 | Failed: 6 (đã sửa) | Unverified: 0
- Tier: Full
- Failures:
  - `npm run` sai package manager (`web/package.json` packageManager pnpm, `.github/workflows/web.yml:60-69`)
  - URL repo lấy từ `go.mod:1` là của upstream; đổi sang `README.md:204`
  - `diskFree` thuộc package `server`, không gọi được từ `books`; đổi sang truyền hàm vào
  - repo không có axe/jsx-a11y (`web/eslint.config.js`)
  - lệch dòng `books/handlers.go` (`provider_mismatch` ở L261)
  - `VerifyUserPassword` vẫn được đổi mật khẩu dùng (`auth/handlers.go:372`), phải giữ

#### Thay đổi đẩy xuống phase
- Phase 1: giữ `VerifyUserPassword`; test migration ở `internal/server/db/db_test.go`; grep có ngoại lệ cho file này.
- Phase 2: test Down/Up 0005 ở `db_test.go`; verify web đủ 4 lệnh.
- Phase 4: chia lượt chỉ cắt khi hết slot toàn cục và có truyện chờ của user khác đủ điều kiện chạy (tránh cắt lặp), kèm test âm; hàm đo đĩa truyền từ `server.go` vào `books`.
- Phase 5: `pnpm install` trong worktree; tương phản ≥ 4.5:1.
- Phase 7: URL repo đúng; assert không lồng phần tử tương tác.
- Phase 9: AC đo được.
- Phase 11: sửa số dòng.
- Phase 12: so checksum `types.gen.ts`; ngoại lệ grep "invite" cho mục nâng cấp/rollback.
- plan.md: trạng thái đã validate, đóng câu hỏi mở, cập nhật AC toàn đợt (invite, chia lượt), bảng file dùng chung, ghi chú worktree.

### Whole-Plan Consistency Sweep
- Files reread: plan.md, phase-01 … phase-12
- Decision deltas checked: 9
- Reconciled stale references: 14 (lệnh verify ở 13 file; câu captcha ở plan.md và phase 1)
- Unresolved contradictions: 0

<!-- slug: web-ux-onboarding-providers -->
