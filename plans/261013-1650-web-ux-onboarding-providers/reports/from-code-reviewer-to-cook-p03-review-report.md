# Review phase 3: lỗi gốc provider + chặn retry vô ích

Ngày: 2026-10-13 | Reviewer: code-reviewer (subagent không được ghi file; cook lưu lại) | **Điểm: 8/10**, không có Critical/High.

Đã kiểm: `go build`, `go vet`, `go test -race` (bootstrap, llmretry, worker, host, jobs, keys; DB 55439). Có viết test thăm dò trên bản sao trong scratchpad.

## Medium (đã sửa)
- **M1** `bootstrap/models.go` `rejectingModel.GenerateStream`: `select` giữa `out <- ev` và `ctx.Done()` làm rơi event lỗi cuối (context.Canceled) khoảng 96/200 lần huỷ. agentcore nhận `PartialStreamError` (retryable) nên phát "retry 1/7" giả. Đã sửa: bỏ `select`, chuyển nguyên từng event (giống adapter/failoverModel). Thêm test `TestRejectingModelStreamKeepsCancelError` (huỷ 30 lần).
- **M2** `worker/run.go` `stop()`: lý do provider từ chối chỉ được gửi khi reason là `StopError`. Engine tạm dừng ở chế độ review khi chưa có chương đang viết thì ra `review_wait`, `last_error` rỗng (tương tự `hold`, `pending_steer`). Đã sửa: có `rejected` mà reason thuộc {review_wait, hold, pending_steer} thì ép thành `StopError`; `complete`/`budget`/`user` giữ ưu tiên. Thêm test `TestRunProviderRejectedOverridesReviewWait`.

## Low
- **L1** Body `{"error":{"code":1113,...}}` (code số lồng trong error): litellm bỏ code và message không chứa mã nên không nhận ra. Docs z.ai trả code dạng chuỗi. Đã ghi giới hạn này trong comment `isPlanMismatch`, chưa xử lý.
- **L2** Chưa có test chứng minh `Run()` gọi `llmretry.SetLimits`; test chỉ kiểm giá trị `serveRetryLimits`. Chấp nhận, vì `Run()` dùng engine thật.
- **L3** Body provider vẫn tới client qua `Event.Detail` và summary tạm dừng. Đây là hành vi có sẵn cho mọi lỗi, không phải regression. `last_error` đã sạch.
- **L4** `TruncateRunes(msg, 299)` cắt cả message dài đúng 300 rune. Đã sửa: chỉ cắt khi > 300 rune, có test.

## Touchpoint không regression
- Interface tuỳ chọn vẫn còn nhờ nhúng `*llm.LiteLLMAdapter`; không nơi nào type-assert adapter.
- agentcore `isRetryable` thấy `ProviderRejectedError` trước; failover vẫn chuyển sang provider dự phòng.
- llmretry: `atomic.Pointer`; mặc định retry vô hạn.
- Engine: pause ngay, Arbiter 0 lần.
- Worker: đọc `rejected` sau khi pump xả xong, không race.
- `proc.outcome`: chỉ sửa 2 nhánh failed.
- API: chỉ thêm 2 hằng số.
- `displayNames`: chỉ đổi tên hiển thị.

Status: DONE_WITH_CONCERNS → sau khi sửa M1/M2/L4: DONE
