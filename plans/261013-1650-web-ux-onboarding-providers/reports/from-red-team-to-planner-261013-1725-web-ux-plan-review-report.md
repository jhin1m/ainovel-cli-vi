# Red team: plan đợt web UX + onboarding + providers

- Ngày: 2026-10-13
- Plan: `plans/261013-1650-web-ux-onboarding-providers/plan.md` (11 phase lúc review, sau review là 12)
- Reviewer (4, chạy song song, mỗi người phải trích `file:line`): Security Adversary, Failure Mode Analyst, Assumption Destroyer, Scope & Complexity Critic
- Phát hiện thô: 37. Sau gộp trùng còn 15 (cap 15). Tất cả đều có bằng chứng `file:line`, nên không cái nào bị loại ở bước lọc bằng chứng.
- Kết quả: 4 Critical, 9 High, 2 Medium. Chấp nhận 15 (5 cái chấp nhận một phần). Các đề xuất con bị bác được liệt kê riêng ở cuối.
- Không đảo ngược quyết định nào trong `plans/reports/briefs/brief-plan-web-ux-round.md`. Đăng ký vẫn tự do, thêm key vẫn không cần mật khẩu, không làm captcha.

## Bằng chứng do controller tự kiểm lại

- `internal/server/jobs/proc.go:304-311,322-327`: mọi `ExitError` bị thay bằng `msgRunFailed`. `errMsg` chỉ ghi log (`proc.go:340`).
- `internal/llmretry/retry.go:36-49`: vòng retry không có trần, chỉ dừng theo context.
- `internal/agents/build.go:45`: `subagentMaxRetries = 7`. Đây là tầng retry chính khi viết chương, nằm trong agentcore.
- `internal/server/keys/repo.go:115-116`: `DecryptForBook` là "lối duy nhất lấy key rõ".
- `internal/server/keys/handlers.go:89-93`: nơi duy nhất throttle khi tạo key là `VerifyUserPassword`.
- `internal/server/auth/ratelimit.go:27-34,63-70`: limiter tính theo phút. Hàm sweep xoá entry nhàn rỗi quá 10 phút, nên không làm được giới hạn theo ngày nếu giữ nguyên.
- `internal/server/auth/password.go:34-36`: `hashSlots` (4) đã giới hạn số argon2 chạy đồng thời.
- `internal/server/config.go:135-137`: `MaxJobs` 4, `MaxJobsPerUser` 1, `MaxBooksPerUser` 20. Cả ba đều tính theo tài khoản hoặc toàn cục.
- `internal/models/registry.go:127-131`: `MergeModels` không ghi đè giá bằng 0. Lookup không có prefix thì trả entry đầu tiên khớp.
- `internal/entry/worker/config.go:58-66`: `applyPricing` bắt Provider/ID khớp.
- `internal/server/jobs/fakeworker/main.go:437-440`: fakeworker kiểm budget trước mỗi chương. Với budget 0 thì dừng ngay ở chương 1.
- `internal/server/db/migrations/0002_auth.sql:46-48`: phần Down chạy `DROP TABLE invite_codes;`, không có `IF EXISTS`.
- `internal/server/httpx/json.go:83`: `DisallowUnknownFields`.

## Phát hiện và phân xử

### 1. Nút "Kiểm tra key" thành oracle thử key bị đánh cắp — CRITICAL
**Reviewer:** Security, Failure Mode, Scope, Assumption
**Vị trí:** Phase 3 (cũ), endpoint `POST /api/keys/test`; Phase 5 (cũ), `add-key-form`
**Lỗ hổng:** Endpoint nhận key thô `{provider, api_key}` mà server chưa từng lưu, và chỉ giới hạn theo user (6/phút). Đăng ký thì tự do: limiter IP 30/phút (`auth/handlers.go:47,212-216`), tức khoảng 43k tài khoản/IP/ngày. Mỗi tài khoản mới lại có hạn mức kiểm key mới. Request ra provider đi từ IP của server, cũng là IP mà worker dùng.
**Kịch bản:** Kẻ gian có danh sách key bị lộ. Chúng lặp: đăng ký, kiểm 6 key, đổi tài khoản. Kết quả `ok`/`code` cho biết key nào sống. Mã 1113 còn cho biết "key sống nhưng sai gói". Provider thấy mẫu thử key hàng loạt thì chặn IP server, và worker của mọi user bị lỗi theo.
**Phân xử:** Chấp nhận. Đây đúng là điều user yêu cầu xét.
- Endpoint chỉ nhận `{key_id}`, là key đã lưu của chính user.
- Giới hạn kiểm key: theo user 3/phút và 20/ngày, theo IP bucket 60/ngày, toàn cục tối đa 4 request đang chạy (không chờ, hết chỗ trả 429 `busy`).
- Tạo key cũng bị throttle và có trần (phát hiện 6). Nhờ vậy vòng "lưu rồi kiểm" không lách được giới hạn.
- Response chỉ có mã đóng (phát hiện 8).
- Rule rate limit trên Cloudflare cho `/api/keys*` là bước deploy bắt buộc.
**Bác một phần:** Reviewer Scope đề nghị giữ chế độ key thô và bỏ `key_id`. Bác, vì user chọn chỉ kiểm key đã lưu. Lập luận "lưu key không bị throttle nên key_id cũng không an toàn hơn" đã được xử lý bằng phát hiện 6.
**Áp vào:** Phase 4 (mới), 6, 8, 12

### 2. Server vứt message lỗi gốc của worker, lỗi 1113 không bao giờ tới user — CRITICAL
**Reviewer:** Failure Mode, Scope, Assumption
**Vị trí:** Phase 3, câu "Supervisor/books lưu nguyên message vào `last_error`"
**Lỗ hổng:** `proc.outcome()` ghi `msgRunFailed` cho mọi `ExitError` (`proc.go:310,327`). `proc.go` không có trong danh sách file của phase 3. Phase 11 (cũ là 10) lại phải dò chuỗi tiếng Việt trong `last_error` để biết khi nào là "key bị từ chối".
**Kịch bản:** Truyện bị 1113 nhưng UI chỉ hiện "Lượt chạy gặp lỗi. Hãy bấm Thử lại". User bấm thử lại và vòng lặp lỗi lặp lại.
**Phân xử:** Chấp nhận. Thêm `proc.go` vào phase 3. Khi `errCode == provider_rejected` thì `LastError = errMsg` (worker dựng message từ bảng cố định, tối đa 300 rune) và `StopReason = provider_rejected`. Các mã khác vẫn ghi message chung. Phase 11 phát hiện lỗi bằng `book.stop_reason`, không dò chuỗi.
**Áp vào:** Phase 3, 11

### 3. Hết slot job: 429 retry vô hạn, truyện thuê bao chạy không giới hạn — CRITICAL
**Reviewer:** Failure Mode, Scope
**Vị trí:** Phase 3 ("các 429 khác giữ retry"); Phase 2 (truyện thuê bao tắt ngân sách); plan.md "Giới hạn tài nguyên"
**Lỗ hổng:**
- `llmretry.Generate` không có trần (`retry.go:36-49`). Arbiter lúc khởi động chạy qua nó (`host.go:411-414`).
- Key hết quota cũng trả 429 retryable (litellm `errors.go:309-331`). Worker vì vậy giữ slot mãi.
- Truyện thuê bao không còn ngân sách USD, mà ngân sách là thứ duy nhất chặn một lượt chạy dài.
- `MaxJobs = 4` là trần toàn cục và lịch chạy là FIFO (`jobs/db.go:33`).
**Kịch bản:** 4 tài khoản miễn phí, mỗi tài khoản có key hết quota hoặc truyện thuê bao chế độ auto, chiếm cả 4 slot nhiều giờ hoặc nhiều ngày. Mọi user khác kẹt ở `queued`. Restart server thì `recoverAll` đưa chúng vào hàng đợi lại.
**Phân xử:** Chấp nhận.
- (a) `llmretry` có trần: 10 lần hoặc 10 phút, sau đó trả lỗi. Áp dụng cho cả CLI.
- (b) Chia lượt theo thời gian: `AINOVEL_MAX_RUN_MINUTES` (mặc định 60). Hết hạn mà có truyện của user khác đang chờ thì gửi `shutdown`. Truyện về `queued` với `pending=resume` và `updated_at=now()`, tức xuống cuối hàng. Áp cho mọi truyện. Đây không phải ngân sách USD, nên không đảo quyết định "thuê bao không áp ngân sách".
**Áp vào:** Phase 3 (a), Phase 4 (b), Phase 12 (docs)

### 4. Thiết kế giá cho thuê bao hỏng và thừa — CRITICAL
**Reviewer:** Assumption, Scope
**Vị trí:** Phase 2: `subscriptionModels`, `PricingFor(provider, model)`, nới `applyPricing`
**Lỗ hổng:**
- Registry đã có `glm-4.6/4.7/5/5.1` với giá > 0 và context 202752 (`models_generated.go:71-77`).
- Entry `Provider: zai-coding` làm `applyPricing` báo "pricing không khớp" (`config.go:65`).
- Entry `Provider: glm` với giá 0 thì không ghi đè được giá (`registry.go:131`). Engine vẫn tính chi phí theo giá glm (`host/usage.go:712`), tức truyện thuê bao hiện chi phí USD giả. `estimateChapters` (phase 8) sau đó dùng số giả này.
- `fakeworker` dừng ngay khi budget là 0 (`fakeworker/main.go:437-440`).
**Phân xử:** Chấp nhận.
- Bỏ `subscriptionModels`. Giữ nguyên chữ ký `PricingFor(model)` và cách kiểm giá trong `applyPricing`.
- Server gửi giá registry như cũ.
- Worker chỉ cho phép `budget_usd == 0` khi provider là thuê bao, và `mapSnapshot` đặt `TotalCostUSD`/`BudgetLimitUSD` về 0 cho truyện thuê bao.
- Bắt buộc sửa `fakeworker`: bỏ kiểm budget khi bằng 0.
- `estimateChapters` loại truyện thuê bao.
**Áp vào:** Phase 2, 8

### 5. Phase 3 nhắm sai tầng retry và sai đường thoát của worker — HIGH
**Reviewer:** Assumption, Failure Mode, Scope
**Lỗ hổng:**
- `imp/call.go` chỉ thuộc pipeline import truyện.
- `llmretry` chỉ được gọi qua `llmcontract.Execute`.
- Khi viết chương, retry nằm trong vòng lặp agentcore (7 lần, `agents/build.go:45`) và dựa vào `LiteLLMError.Retryable`.
- Lỗi giữa lượt chạy kết thúc qua `stop(StopError)` (`run.go:363,535-546`), không qua `fail()` (`run.go:309-311`).
- litellm chỉ đọc `error.code` khi là chuỗi (`errors.go:137,165`).
**Phân xử:** Chấp nhận.
- Sửa ở một chỗ: lớp model của bootstrap (`createModelFromConfig` `models.go:361` / `failoverModel` `models.go:408`). Lỗi fatal được bọc thành kiểu có `Retryable() == false` và phải nằm ngoài cùng chuỗi lỗi. Có test chứng minh vòng agentcore chỉ gọi 1 lần.
- Hàm phân loại `ClassifyProviderError` trả mã đóng, dùng chung cho worker và phase 4.
- Observer gắn kind `provider_rejected` (`host/observer.go:23`). `pump()` lưu kind này. `stop()` gửi `MsgError` trước khi exit.
- Test fixture có cả `code` dạng số. Fallback dò `\b1113\b` trong message chỉ áp cho provider `Type: glm`.
- Không tạo package mới.
**Áp vào:** Phase 3 (viết lại)

### 6. Bỏ re-auth làm mất throttle duy nhất của `POST /api/keys`, không có trần số key — HIGH
**Reviewer:** Security, Failure Mode, Scope
**Lỗ hổng:**
- `userLimiter` chỉ nằm trong `VerifyUserPassword` (`auth/handlers.go:118-121`), và `Repo.Create` không đếm số key (`keys/repo.go:64-81`).
- Phase 1 ghi sai "không đổi limiter".
- Phase 1 còn gợi ý "bỏ dependency auth", điều không làm được vì mọi route dùng `h.auth.RequireUser` (`keys/handlers.go:40-43`).
- Rủi ro "phiên bị chiếm" chỉ xét chuyện đọc key, bỏ sót chuyện kẻ gian thêm key của chính chúng rồi `PATCH api_key_id` truyện của nạn nhân (`books/handlers.go:250-268`) để hứng prompt và chương.
**Phân xử:** Chấp nhận.
- Tạo key bị giới hạn 30/ngày/user và trần 20 key/user (409 `key_limit`).
- Giữ dependency `auth`.
- Sửa lại câu rủi ro.
- Trang truyện hiện "Key đang dùng: nhãn …last4" để user nhận ra key lạ. Không làm banner hay thu hồi key (YAGNI).
**Áp vào:** Phase 1, 11

### 7. Đường giải mã cho `key_id` không tồn tại, dễ thành IDOR — HIGH
**Reviewer:** Security, Assumption
**Lỗ hổng:** Plan viết "giải mã bằng đường hiện có", nhưng chỉ có `DecryptForBook` (`repo.go:115-133`). Hàm mới viết vội có thể tra theo `id` rồi dựng AAD từ `user_id` của chính row đó. Khi đó giải mã thành công cho key của người khác.
**Phân xử:** Chấp nhận. Thêm `Repo.DecryptOwned(ctx, sessionUserID, keyID)` với `WHERE user_id = $1 AND id = $2` và AAD lấy từ user của phiên. Sửa comment "lối duy nhất". Test: `key_id` của user khác phải trả 404.
**Áp vào:** Phase 4

### 8. Không giới hạn egress đồng thời; trả nguyên văn lỗi provider — HIGH
**Reviewer:** Security
**Lỗ hổng:**
- Burst của limiter bằng perMinute (`ratelimit.go:31`). Mỗi lần kiểm key giữ goroutine và kết nối tối đa 20s.
- `LiteLLMError.Message` là body thô của provider (`litellm errors.go:120-160`). Body này có thể lộ org id, last4 hay số dư (vd. "can only afford N tokens"), tức oracle còn xếp được key theo số dư.
**Phân xử:** Chấp nhận.
- Semaphore toàn cục 4, không chờ.
- Context 20s. Gọi model trực tiếp một lần, không qua `llmretry` hay agentcore. Test khẳng định provider giả chỉ nhận 1 request.
- Response `{ok, code ∈ {ok, auth_failed, plan_mismatch, rate_limited, quota, network, unknown}, message do server viết, model}`. Không chuyển tiếp văn bản của provider.
**Bác một phần:** Không làm `http.Transport` riêng, vì semaphore 4 cộng timeout là đủ (YAGNI).
**Áp vào:** Phase 4, 6

### 9. Đăng ký mở: giới hạn hiện có tính theo tài khoản; không dọn tài khoản, không chặn khi đĩa đầy — HIGH
**Reviewer:** Failure Mode, Security, Scope
**Lỗ hổng:**
- Signup dùng chung limiter 30/phút/IP với login.
- Không xác minh email. Lỗi 409 `email_taken` trở thành công cụ dò email (`auth/handlers.go:262-264`).
- Chỉ có dọn session (`session.go:104-106`).
- `disk_low` chỉ được báo qua `/healthz` (`server.go:278-294`), không chặn gì.
- Limiter đầy thì từ chối mọi khoá mới (`ratelimit.go:36-55`).
- Rule Cloudflare chỉ là khuyến nghị (`docs/web-server.md:120`).
**Phân xử:** Chấp nhận.
- Limiter riêng cho signup: 10/ngày/IP bucket (dùng `ipKey` /64 hiện có), cộng thêm limiter 30/phút cũ.
- `POST /api/books` trả 507 `disk_full` khi `diskFree < lowDiskBytes`.
- Dọn định kỳ user không có key, không có truyện, không có session và tạo quá 30 ngày (một câu SQL trong vòng dọn hiện có).
- Rule Cloudflare cho `/api/auth/signup` và `/api/keys*` là bước deploy bắt buộc.
- Docs ghi rõ: limiter nằm trong bộ nhớ, mất khi restart; rủi ro chiếm email trước và dò email.
**Bác một phần:**
- Semaphore cho argon2: đã có `hashSlots` (`password.go:36`).
- Limiter kiểu LRU: Cloudflare là lớp chặn chính, chỉ ghi vào docs.
- Race giữa đếm và insert khi tạo truyện: lệch tối đa vài truyện, chấp nhận.
**Áp vào:** Phase 1, 4, 12

### 10. Migration 0004/0005 và rollback trên prod — HIGH
**Reviewer:** Failure Mode, Scope, Assumption
**Lỗ hổng:**
- Migration tự chạy khi `serve` khởi động (`db/db.go:44-62`). goose bỏ qua version cao hơn mức binary biết.
- Binary cũ chạy trên DB mới sẽ lỗi: signup chạy `UPDATE invite_codes` và trả 500 (`auth/handlers.go:249`); truyện thuê bao bị từ chối ở bước init và chuyển `failed` (`protocol.go:118-120`).
- 0002 có phần Down mà không có `IF EXISTS`. Nếu 0004 không có Down dựng lại bảng thì không down được qua 0002.
- Down của 0005 khôi phục CHECK `> 0` sẽ hỏng khi đã có truyện thuê bao.
- grep verify của phase 1 không loại trừ 0004, nên không bao giờ ra rỗng.
- Tab còn chạy SPA cũ gửi `invite_code`/`current_password` thì `DisallowUnknownFields` trả 400.
**Phân xử:** Chấp nhận.
- 0004: Up `DROP TABLE IF EXISTS`, Down dựng lại bảng y như 0002.
- 0005: `DROP CONSTRAINT IF EXISTS books_budget_usd_check, ADD CONSTRAINT books_budget_usd_check CHECK (budget_usd >= 0)`. Down dùng `CHECK (budget_usd > 0) NOT VALID`. Test đọc `pg_constraint`.
- Chạy `deploy/backup.sh` trước khi deploy. Docs ghi: rollback là binary cũ cộng restore DB, mất dữ liệu sau thời điểm nâng cấp; chạy binary cũ trên DB mới thì hỏng signup và truyện thuê bao.
- Sửa grep loại trừ 0004.
- Docs ghi "tải lại trang sau khi nâng cấp".
**Bác một phần:** Không giữ field `invite_code` làm placeholder, vì mâu thuẫn tiêu chí "không còn chuỗi invite trong API".
**Áp vào:** Phase 1, 2, 12

### 11. Lời khuyên cho 1113 ("chọn đúng provider") không làm được với truyện đã tạo — HIGH
**Reviewer:** Failure Mode, Scope, Assumption
**Lỗ hổng:** Provider của truyện cố định từ lúc tạo (`books/handlers.go:204-208`), và `PATCH` báo `provider_mismatch` nếu đổi (`:255-258`). Banner ở phase 11 chỉ cho chọn key cùng provider.
**Phân xử:** Chấp nhận.
- Message 1113 nói rõ "thêm key dưới đúng provider rồi tạo truyện mới".
- Với `stop_reason = provider_rejected`, phase 11 hiện hành động "Tạo truyện mới với provider khác" thay cho chọn key cùng provider.
- Phase 6 tự kiểm key ngay sau khi lưu, để bắt lỗi chọn sai provider trước khi có truyện.
- Cho phép `PATCH` đổi giữa `zai` và `zai-coding` là quyết định sản phẩm, nên đưa vào câu hỏi mở.
**Áp vào:** Phase 3, 6, 11

### 12. `make gen` sinh field bool bắt buộc, làm vỡ typecheck web và làn song song — HIGH
**Reviewer:** Scope
**Lỗ hổng:** tygo sinh `subscription: boolean` và `recommended: boolean` là bắt buộc. Hai test web dựng literal `ProviderInfo`/`ModelOption` (`new-book-logic.test.ts:5-8`, `new-book-form.test.tsx:17-20`). Phase 5, chạy song song trong cùng working tree, sẽ thấy typecheck đỏ.
**Phân xử:** Chấp nhận. Phase 2 sở hữu và sửa 2 fixture đó. Chỉ chạy phase 5 song song nếu dùng `git worktree` riêng, nếu không thì chạy tuần tự.
**Áp vào:** Phase 2, plan.md

### 13. Bộ đếm "(n)" trên tiêu đề tab không chạy được như mô tả — HIGH
**Reviewer:** Scope
**Lỗ hổng:** TanStack Query v5 dừng `refetchInterval` khi tab ẩn (`refetchIntervalInBackground` mặc định false). `refetchOnWindowFocus` cũng tắt (`main.tsx:19`). Vì vậy n luôn bằng 0. Hơn nữa `queries.ts` không thuộc phase 5.
**Phân xử:** Chấp nhận.
- n = số truyện cần xử lý (`waiting_review`, `failed`, `budget_exhausted`).
- Selector `needsAttention(books)` dùng chung với dải "Cần bạn xử lý" ở phase 7.
- `useBooks` đặt `refetchIntervalInBackground: true`.
- `queries.ts` thêm vào danh sách file của phase 5.
**Áp vào:** Phase 5, 7, plan.md

### 14. Giá của `zai` (trả theo token) lấy nhầm từ entry `glm` (bigmodel.cn) — MEDIUM
**Reviewer:** Failure Mode, Assumption
**Lỗ hổng:** `Catalog.resolve` chỉ tra theo model id (`catalog.go:96-112`). Giá Z.ai quốc tế có thể khác giá bigmodel. Nếu giá thật cao hơn thì ngân sách USD dừng muộn.
**Phân xử:** Chấp nhận. Phase 2 kiểm giá Z.ai, ghi nguồn và ngày. Nếu khác registry thì thêm `priceOverrides["zai"]` và lúc đó mới đổi thành `PricingFor(provider, model)`. Không kiểm được giá thì `zai` chỉ hiện các model có giá đã kiểm.
**Áp vào:** Phase 2

### 15. Phase 8 (form tạo truyện) lấn sang P2; nhãn bậc giá vô nghĩa với provider ít model — MEDIUM
**Reviewer:** Scope
**Lỗ hổng:** Phase 8 làm luôn P2 2.2, trong khi brief ghi "P2 để sau". Chia tertile giá với deepseek (1 model) hay grok (1 model) thì không mang nghĩa gì (`keys/models.go:22-25`).
**Phân xử:** Chấp nhận một phần. Bỏ mục P2 2.2. Bậc giá chỉ hiện khi provider có từ 3 model có giá trở lên.
**Bác:**
- Tách phase 8 thành 2: sau khi cắt, khối lượng vừa một phiên, và đã có điều kiện tách component khi vượt 200 dòng.
- Đưa P0 (phase 11) lên trước reader: cả đợt phát hành cùng lúc nên không có lợi ích.
**Áp vào:** Phase 8

## Captcha và xác minh email

Bằng chứng trên đủ mạnh để cần chặn oracle và giới hạn theo IP/ngày. Nó không đủ mạnh để bắt buộc captcha, vì các giới hạn đã áp (key_id, trần, IP/ngày, rule Cloudflare) đã đóng các đường lạm dụng chính. Giữ quyết định không làm captcha. Các lựa chọn được ghi vào câu hỏi mở của plan:
- (A) Giữ nguyên như hiện tại (khuyến nghị đợt này).
- (B) Thêm Cloudflare Turnstile cho `/api/auth/signup` khi log cho thấy signup hàng loạt, vd. vượt 50 tài khoản/ngày hoặc limiter 10/ngày/IP bị chạm thường xuyên.
- (C) Xác minh email. Đợt này không làm vì chưa có hạ tầng gửi mail.

## Đề xuất con bị bác

| Đề xuất | Lý do |
|---|---|
| Giữ kiểm key thô, bỏ `key_id` (Scope) | User chọn chỉ kiểm key đã lưu. Vòng lưu-rồi-kiểm đã bị chặn bằng trần và throttle khi tạo key. |
| Không đặt giới hạn theo ngày (Scope) | Mục tiêu của user yêu cầu giới hạn theo ngày. Làm bằng `NewLimiterPer`, chi phí nhỏ. |
| Dọn tài khoản là YAGNI (Scope) | User yêu cầu xét dọn tài khoản. Chỉ thêm một câu SQL trong vòng dọn sẵn có. |
| Semaphore argon2 (Failure Mode) | Đã có `hashSlots` ở `password.go:36`. |
| Limiter kiểu LRU khi đầy (Security) | Rule Cloudflare là lớp chính, docs ghi giới hạn của limiter. |
| `http.Transport` riêng cho kiểm key (Security) | Semaphore 4 và context 20s là đủ. |
| Placeholder `invite_code` cho tab SPA cũ (Assumption) | Mâu thuẫn tiêu chí không còn chuỗi invite. Docs hướng dẫn tải lại trang. |
| Banner "có key mới" và thu hồi key khi đổi mật khẩu (Security) | YAGNI. Đã hiện key đang dùng ở trang truyện. |
| Tách phase 8; đổi thứ tự P0 lên trước reader (Scope) | Xem phát hiện 15. |
| Sửa race giữa đếm và insert khi tạo truyện (Failure Mode) | Lệch tối đa vài truyện, chấp nhận. |

## Câu hỏi còn mở

1. Có cho phép `PATCH` đổi key giữa `zai` và `zai-coding`, kèm quy tắc ngân sách (chuyển sang thuê bao thì budget 0, rời thuê bao thì bắt nhập budget)? Mặc định đợt này: không.
2. Giá trị mặc định của `AINOVEL_MAX_RUN_MINUTES` (60) và các giới hạn số (signup 10/ngày/IP, tạo key 30/ngày, kiểm key 3/phút + 20/ngày/user + 60/ngày/IP, tối đa 20 key) có hợp với tải thực tế không? Tất cả đều chỉnh được qua hằng số hoặc env.
3. Captcha hoặc xác minh email: xem mục phía trên.
