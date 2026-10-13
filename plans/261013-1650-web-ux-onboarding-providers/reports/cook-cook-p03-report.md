# Cook phase 3: lỗi gốc provider + chặn retry vô ích

Ngày: 2026-10-13 | Phase: [phase-03](../phase-03-backend-provider-errors-and-retry.md) | Review: [from-code-reviewer-to-cook-p03-review-report.md](./from-code-reviewer-to-cook-p03-review-report.md) (8/10, đã sửa 2 Medium + 1 Low)

## Đã làm
- **`internal/bootstrap/provider_errors.go` (mới):**
  - `ClassifyProviderError` trả một trong các nhóm `auth_failed`/`plan_mismatch`/`rate_limited`/`quota`/`network`/`unknown`. Mã gốc chỉ lấy khi nằm trong allowlist (hiện là `1113`).
  - Nhận 1113 qua `Code == "1113"`. Nếu không có Code thì dò `\b1113\b` trong message, và chỉ dò khi provider là `glm`.
  - `ProviderRejectedError` có `Retryable()=false`, có `Unwrap`, và `Error()` bắt đầu bằng tiền tố `provider_rejected:`.
  - Thêm `ParseProviderRejected` (đọc lại từ chuỗi đã ép) và `ProviderRejection`. Phase 4 dùng lại các hàm này.
- **`bootstrap/models.go`:** `createModelFromConfig` trả `rejectingModel` (nhúng `*llm.LiteLLMAdapter`), bọc lỗi 1113 ở cả `Generate` và `GenerateStream`. Các interface tuỳ chọn vẫn còn.
- **`llmretry`:**
  - Thêm `Limits` và `SetLimits` (lưu `atomic.Pointer` cấp process). Giá trị mặc định là không giới hạn, nên CLI/TUI không đổi.
  - Comment đầu package đã cập nhật.
  - Worker gọi `SetLimits({10, 10m})` trong `Run()`.
- **host:**
  - `errorKind` ưu tiên trả `provider_rejected`.
  - `engine.handleWorkerError`: gặp `provider_rejected` thì tạm dừng ngay, không giao lại và không hỏi Arbiter.
  - `imp/call.go` `modelErrDetail` có thêm "mã <code>".
- **worker `run.go`:**
  - `pump` giữ riêng lời giải thích khi `ev.Kind == provider_rejected`.
  - `stop()`: với `StopError` (và với review_wait/hold/pending_steer khi có rejection) thì gửi `MsgError{provider_rejected}`.
  - `fail()` phân loại lỗi theo cùng cách.
  - Message tiếng Việt lấy từ bảng cố định, ≤300 rune, không chứa body của provider.
- **server:**
  - Thêm `api.StopProviderRejected` và `api.ErrCodeProviderRejected`.
  - `proc.outcome()` → `applyProviderRejected`: đặt `stop_reason = provider_rejected` và `last_error` lấy từ message của worker (≤300 rune). Mã lỗi khác vẫn dùng message chung.
  - Đã chạy `make gen`; phase này chỉ thêm 2 hằng số.
- **Bổ sung:** `keys/models.go` thêm `displayNames` cho `zai-coding` (glm-4.7 → "GLM-5.3-Flash", glm-5.1 → "GLM-5.3"). Mã model gửi đi không đổi; provider `zai` giữ tên gốc.

## Tests mới
- **bootstrap:**
  - Bảng phân loại lỗi (12 ca).
  - Contract của `ProviderRejectedError`.
  - Đếm request qua httptest với model thật: 1113 qua agentcore loop (MaxRetries 7) và qua llmretry đều chỉ có **1** request. Stream trả về ProviderRejectedError.
  - Failover: provider từ chối nhận 1 request, fallback nhận 1. 429 không có code vẫn retry (2 request).
  - Interface tuỳ chọn còn nguyên. Huỷ giữa stream vẫn nhận `context.Canceled`.
- **llmretry:** không trần thì retry tới khi ctx kết thúc (>11 lần); trần 10 lần thì dừng ở 11 lần gọi; trần 10 phút dùng clock giả thì dừng ở 3 lần gọi; lỗi không retry được chỉ gọi 1 lần.
- **host:**
  - `TestErrorKindProviderRejected`.
  - `TestEngine_ProviderRejectedStopsWithoutRetry`: writer 1 lần, Arbiter 0 lần. Bỏ nhánh mới trong engine thì test đỏ, writer bị gọi 4 lần.
- **worker:**
  - Lỗi giữa lượt (đường `stop`), lỗi lúc khởi động (đường `fail`), chế độ review bị ép thành error.
  - Lỗi thường vẫn mang mã `engine`.
  - Độ dài message và trần retry.
- **jobs:** `TestOutcomeProviderRejected` gồm 7 ca: hai nhánh, cắt độ dài, đúng 300 rune, message rỗng, mã engine giữ message chung.
- **keys:** `TestCatalogZaiCodingDisplayNames`.

## Verify (đều pass)
- `go build ./... && go vet ./internal/...`, gofmt sạch.
- `go test -count=1 ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/... ./internal/entry/worker/... ./internal/server/...` với `AINOVEL_TEST_DATABASE_URL` (container ainovel-testpg). Đã chạy `-v` để kiểm: 0 SKIP ở server/db/jobs.
- `-race` cho bootstrap, llmretry, worker, jobs.
- `make gen`: chạy lại không ra diff.
- `pnpm -C web typecheck && lint && test (63/63) && build`.

## Lệch so với phase file
- Có sửa `internal/host/engine.go`, file này không nằm trong danh sách của phase. Đây là tầng retry thứ tư mà phase chưa liệt kê: engine giao lại worker một lần rồi hỏi Arbiter, và cả hai đều gọi lại cùng provider. Không chặn ở đây thì không đạt tiêu chí "1113 không retry ở mọi tầng".
- Failover vẫn được chuyển sang provider dự phòng khác. Provider đã từ chối không bị gọi lại. Trên web không có fallback nên không ảnh hưởng.

## Chưa làm / lưu ý
- Chưa kiểm thủ công với key Z.ai Coding Plan thật. Việc này để phase 12.
- Chưa nhận ra body có code số lồng trong error (`{"error":{"code":1113}}`), vì litellm bỏ code. Docs z.ai trả code dạng chuỗi. Đã ghi chú trong code.
- Chưa có test chứng minh `Run()` gọi `SetLimits`, vì `Run()` dùng engine thật.
- Body provider vẫn có trong `Event.Detail` và summary tạm dừng gửi qua event stream. Hành vi này đã có từ trước cho mọi lỗi; `last_error` thì sạch.
- Phase 12 cần ghi vào docs: trần llmretry chỉ áp cho worker serve; thêm `stop_reason = provider_rejected`.
- Không commit (theo ràng buộc). Không chạy docs-manager vì docs thuộc phase 12. Không viết journal.

## Câu hỏi còn mở
- Có nên chặn cả failover khi gặp 1113 không? Hiện vẫn cho chuyển sang provider dự phòng khác; chỉ ảnh hưởng CLI có cấu hình fallbacks.

Status: DONE
