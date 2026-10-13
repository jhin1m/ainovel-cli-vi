---
phase: 3
title: "Lỗi gốc provider + chặn retry vô ích"
status: completed
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 3: Lỗi gốc provider + chặn retry vô ích

## Overview
Lỗi fatal của provider (mã 1113 của z.ai) không bị retry ở bất kỳ tầng nào. User thấy được mã gốc cùng lời giải thích dễ hiểu trong `last_error` và `stop_reason`. `llmretry` có trần, để key hết quota không giữ slot job mãi. Endpoint "Kiểm tra key" được tách sang phase 4 và dùng lại hàm phân loại lỗi của phase này.

<!-- Red Team: phase được viết lại theo phát hiện 2, 3a, 5, 11 — 2026-10-13 -->

## Context
- Brainstorm mục 3. Theo docs z.ai, gọi sai endpoint hoặc sai gói thì nhận 429 code 1113 "insufficient balance". Retry không có tác dụng.
- Các tầng retry thật (đã kiểm khi red-team):
  - **Vòng agentcore** khi viết chương: `subagentMaxRetries = 7` (`internal/agents/build.go:45`). Quyết định retry dựa vào `providerError.Retryable()`, giá trị này đọc `LiteLLMError.Retryable` (`$(go env GOMODCACHE)/github.com/voocel/agentcore@v1.8.3/loop.go:512-514`, `llm/errors.go:71`). Đây là module bên thứ ba, không sửa được.
  - **`llmretry.Generate`**: chỉ dừng khi context kết thúc, không có trần (`internal/llmretry/retry.go:36-49`). Chỉ được gọi qua `llmcontract.Execute` (`internal/llmcontract/execute.go:108`), tức import, sim và arbiter. Arbiter chạy ngay lúc khởi động (`internal/host/host.go:411-414`).
  - **`failoverModel`** của bootstrap (`internal/bootstrap/models.go:408`). Trên web không có fallback vì `Roles` là nil.
  - litellm tự retry chỉ khi được bật, và dự án không bật (`litellm@v1.8.10/retry/retry.go:1-4`).
- litellm đã đánh dấu Auth/Quota/Validation là không retry (`litellm@v1.8.10/errors.go:329-336`), nên 401/403 đã không bị retry. Chỉ 1113 (HTTP 429, loại RateLimit, retryable) cần xử lý riêng. litellm chỉ đọc `error.code` khi nó là chuỗi (`errors.go:137,165-166`).
- `internal/host/imp/call.go` (`modelErrDetail`) chỉ thuộc pipeline import truyện, không nằm trên đường viết chương.
- Đường lỗi của worker:
  - `fail()` (`internal/entry/worker/run.go:309-311`) chỉ chạy khi action Start/Resume trả lỗi.
  - Lỗi giữa lượt chạy chỉ là chuỗi summary của event (`run.go:509-511`, `lastErr`). Lượt chạy kết thúc qua `stop(snap, StopError)` (`run.go:363,535-546`), và đường này không gửi `MsgError`.
  - Lỗi trong kết quả JSON của sub-agent có thể đã bị ép thành chuỗi. `host.errorKind(err, msg)` (`internal/host/observer.go:23`) đã có nhánh dự phòng theo chuỗi.
- Server: `proc.outcome()` ghi `msgRunFailed` cho mọi `ExitError` hoặc `StopError` (`internal/server/jobs/proc.go:304-311,322-327`). `errMsg` chỉ vào log (`proc.go:277,340`). `api.Book` có `stop_reason` dạng text tự do (`0003_books.sql:16`, không có CHECK).

## Requirements
- **Hàm phân loại dùng chung** (đặt trong `internal/bootstrap`, vì worker và `internal/server/keys` ở phase 4 đều import được):
  - `ClassifyProviderError(err error) (kind, code string)`. `kind` thuộc tập đóng: `auth_failed`, `plan_mismatch`, `rate_limited`, `quota`, `network`, `unknown`. `code` là mã gốc của provider, chỉ giữ khi nằm trong allowlist (hiện có `1113`).
  - 1113 nhận diện qua `LiteLLMError.Code == "1113"`. Nếu không có `Code` (code dạng số), dò `\b1113\b` trong `Message`, nhưng **chỉ** khi provider dùng `Type: glm`.
- **Chặn retry tại một chỗ:** lớp model của bootstrap (`createModelFromConfig` `models.go:361` hoặc `failoverModel.Generate/GenerateStream`) bọc lỗi `plan_mismatch` thành `*ProviderRejectedError{Kind, Code, Provider, Model, Err}`:
  - `Retryable() == false` và có `Unwrap()`.
  - Nằm ngoài cùng chuỗi lỗi, để agentcore và `llmretry` đều thấy là không retry.
  - `Error()` bắt đầu bằng tiền tố ổn định `provider_rejected:` để nhánh dự phòng theo chuỗi vẫn nhận ra.
- **`llmretry` có trần:** tối đa 10 lần retry hoặc 10 phút tính từ lần gọi đầu (cái nào đến trước), rồi trả lỗi cuối. Comment đầu package phải cập nhật theo. **User chốt: trần CHỈ bật trong worker của chế độ serve** (cấu hình truyền từ `internal/entry/worker`); CLI/TUI giữ retry vô hạn như cũ. Test cả hai chế độ.
- **Worker:**
  - `host.errorKind` trả `provider_rejected` khi `errors.As(*ProviderRejectedError)`, hoặc khi chuỗi có tiền tố `provider_rejected:`.
  - `pump()` lưu kind này và message cạnh `lastErr`.
  - `stop()` với `StopError` và kind `provider_rejected`: gửi `MsgError{Code: api.ErrCodeProviderRejected, Message}` trước khi exit.
  - `fail()` cũng phân loại như vậy.
  - Message tiếng Việt do worker dựng từ bảng cố định, tối đa 300 rune, có mã gốc. Ví dụ 1113: "Z.ai từ chối yêu cầu (mã 1113): key không thuộc gói Coding Plan, gói đã hết quota, hoặc model không có trong gói. Truyện này không đổi provider được: hãy thêm key dưới đúng provider (Z.ai Coding Plan cho key gói, Z.ai cho key trả theo token) rồi tạo truyện mới."
  - Không chép body thô của provider vào message.
- **Server `proc.outcome()`:** khi `p.errCode == api.ErrCodeProviderRejected` thì đặt `LastError = p.errMsg` (cắt ≤300 rune) và `StopReason = api.StopProviderRejected`. Mọi mã khác giữ message chung như cũ (vẫn không gửi chi tiết nội bộ cho client).
- **API:** thêm `api.ErrCodeProviderRejected = "provider_rejected"` (`internal/server/api/worker.go:59-65`) và `api.StopProviderRejected = "provider_rejected"` (cạnh `StopError` ở `worker.go:54`). Sau đó chạy `make gen`.
- `modelErrDetail` (`imp/call.go`) thêm `le.Code` khi có. Đây là sửa nhỏ cho đường import.

## Related Code Files
- Create: `internal/bootstrap/provider_errors.go` (+ `_test.go`)
- Modify: `internal/bootstrap/models.go` (bọc lỗi), `internal/llmretry/retry.go` + `retry_test.go`, `internal/host/observer.go` (+ test), `internal/entry/worker/run.go` (`pump`, `stop`, `fail`) + test (dùng `fake_engine_test.go`), `internal/server/jobs/proc.go` + test, `internal/server/api/worker.go`, `internal/host/imp/call.go`
- Generated: `web/src/api/types.gen.ts`

## Implementation Steps
1. Đọc `bootstrap/models.go:361-420`, agentcore `loop.go:500-530`, litellm `errors.go:100-170,300-340`, `host/observer.go`, `worker/run.go:300-560`, `jobs/proc.go:270-345`.
2. `provider_errors.go` + test. Dựng `LiteLLMError` bằng tay cho các ca: code chuỗi `"1113"`, code số (chỉ có trong message), 401, 429 không có code, lỗi mạng.
3. Bọc lỗi ở lớp model. Test với model giả trả 1113 chạy qua vòng agentcore và qua `llmretry`: model chỉ được gọi **1 lần**. Với 429 không có code thì vẫn retry.
4. Trần cho `llmretry`, kèm test (clock giả hoặc delay nhỏ).
5. Observer, `pump`/`stop`/`fail`. Test: fake engine phát event lỗi `provider_rejected` giữa lượt chạy, rồi kiểm worker gửi `MsgError{provider_rejected}` và exit Error.
6. `proc.outcome()` + test cho cả hai nhánh (provider_rejected thì giữ message, mã khác thì message chung).
7. `make gen`, chạy verify.

## Tests / Verify
```bash
go build ./... && go vet ./internal/...
go test ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/... ./internal/entry/worker/... ./internal/server/...
make gen
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```
Kiểm thủ công nếu user có key Z.ai Coding Plan thật: tạo truyện với provider `zai` bằng key gói. Kết quả mong đợi là truyện dừng ngay, `stop_reason = provider_rejected`, `last_error` có "mã 1113", và log worker chỉ có một lần gọi model.

## Success Criteria
- [x] 1113 không bị retry ở mọi tầng: vòng agentcore, `llmretry`, failover, và cả vòng "giao lại + hỏi Arbiter" của engine. Có test đếm số lần gọi.
- [x] 429 thật (rate limit) vẫn được retry, nhưng `llmretry` trong worker serve dừng sau tối đa 10 lần hoặc 10 phút; CLI vẫn retry vô hạn.
- [x] Lỗi 1113 giữa lượt chạy (đường `stop()`) và lúc khởi động (đường `fail()`) đều cho ra `stop_reason = provider_rejected` cùng `last_error` nêu nguyên nhân và mã gốc.
- [x] Các lỗi khác vẫn hiện message chung, không lộ chi tiết nội bộ.
- [x] Test mới pass, test cũ không phải nới điều kiện.

## Risk Assessment
- agentcore có thể bọc lỗi model bên trong `providerError` riêng, khiến `Retryable()` của lớp ngoài bị bỏ qua. Bước 3 phải chứng minh bằng test. Nếu test fail thì đổi sang trả lỗi đã đặt `LiteLLMError.Retryable = false` (sao chép rồi sửa) ngay tại lớp model.
- Trần của `llmretry` chỉ áp cho worker serve; CLI/TUI không đổi hành vi (user chốt). Ghi vào docs ở phase 12.
- Dò `1113` trong message có thể khớp nhầm. Đã giới hạn chỉ cho provider `Type: glm` và chỉ khớp nguyên từ.

## Bổ sung (orchestrator, sau phase 2)
- Tên hiển thị model của `zai-coding` phải phản ánh model thật mà gói chạy (docs https://docs.z.ai/devpack/overview: glm-5.1/5.2 → GLM-5.3, glm-4.7 → GLM-5.3-Flash). Trong catalog (`internal/server/keys/catalog.go`/`models.go`), override `Name` cho provider `zai-coding`: `glm-4.7` → "GLM-5.3-Flash", `glm-5.1` → "GLM-5.3". Mã model gửi đi giữ nguyên. Có test. Provider `zai` (API thường) giữ tên gốc.

## Kết quả (2026-10-13)
Đã xong, verify pass. Báo cáo: [cook](./reports/cook-cook-p03-report.md), [review](./reports/from-code-reviewer-to-cook-p03-review-report.md).

Lệch so với phase:
- Sửa thêm `internal/host/engine.go` (`handleWorkerError`): gặp `provider_rejected` thì tạm dừng ngay, không giao lại và không hỏi Arbiter. Thiếu nhánh này thì writer bị gọi 4 lần, không đạt tiêu chí "không retry ở mọi tầng".
- Bọc lỗi trong `createModelFromConfig` bằng `rejectingModel`, nhúng `*llm.LiteLLMAdapter`. Failover vẫn chuyển sang provider dự phòng khác; provider đã từ chối chỉ bị gọi 1 lần.
- Worker ép `review_wait`/`hold`/`pending_steer` thành `error` khi có lý do provider từ chối (theo review M2).
- Chưa nhận ra code số lồng trong `error` (`{"error":{"code":1113}}`), vì litellm bỏ code. Đã ghi trong comment.
