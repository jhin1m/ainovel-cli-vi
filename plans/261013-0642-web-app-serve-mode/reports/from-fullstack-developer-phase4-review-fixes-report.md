# Sửa finding review supervisor/Books API

Nhánh feat/web-app. Không commit. Không đụng internal/host, internal/entry/worker, web/**, api/ (không đổi field nên không cần `make gen`).

## Đã sửa

| Mục | Thay đổi | File |
|---|---|---|
| H1 | Nhánh claim lỗi/`!ok` gọi `p.markDone()` trước `release` | jobs/supervisor.go |
| M1 | `outcome()` chỉ đặt `ClearPending=true` cho lượt sạch; crash, ExitError, stop_reason lạ giữ `pending_action/text`. Fake worker: `resume` trên thư mục chưa có progress trả `nothing_to_resume` như worker thật | jobs/proc.go, jobs/fakeworker/main.go |
| M2 | Khi drain: chỉ bỏ qua kết quả nếu lượt bị chính server cắt (`stopping`, không có exit, hoặc `ExitShutdown`). Worker tự dừng vẫn ghi kết quả và xoá pending (`finishResult.AfterDrain` cho phép ghi đè `interrupted`). Lock_busy khi drain: không requeue | jobs/supervisor.go, jobs/db.go |
| M3 | `command()` hết hạn chờ `ready` trả `ErrNotReady` (mới). `handleSteer` trả 503 `worker_starting` + `Retry-After: 5`, message tiếng Việt, không còn 202. `ackTimeout` thành biến để test rút ngắn | jobs/proc.go, books/handlers.go |
| M4 | Khoá cache `exports/<N>-<chữ ký>.<ext>`; chữ ký = sha256 của size+mtime từng `chapters/NN.md` đã chốt + `meta/book.json`, `outline.json`, `layered_outline.json`. Bản cache cũ cùng định dạng bị dọn | books/export.go |
| L3 | `last_error` gửi client là thông báo chung tiếng Việt (`msgWorkerCrashed`/`msgRunFailed`); chi tiết (exit error, err_code, 200 dòng stderr) chỉ vào log server (`failureDetail`) và `worker-stderr.log` | jobs/proc.go, jobs/supervisor.go |
| L8 | `scanBook` chỉ trả `pending_text` khi `pending_action='steer'` (start: là prompt) | books/repo.go |
| L2 | `pumpStderr` đếm byte, ngừng ghi khi đạt 5MB (kèm một dòng đánh dấu); ring 200 dòng trong bộ nhớ vẫn giữ | jobs/proc.go |
| L6 | `Exporter` theo dõi export đang chạy theo thư mục; `finishDelete` gọi `WaitDir` trước `RemoveAll`; `MkdirAll` -> `Mkdir` nên không tạo lại thư mục đã mất | books/export.go, books/handlers.go |

## Test thêm/sửa
- `TestStopAndWaitDuringFailedClaim`: giữ khoá dòng để claim chờ, đổi sang deleting, StopAndWait phải trả về. Đã xác nhận FAIL khi bỏ `markDone`.
- `TestNaturalStopDuringDrainIsRecorded` (M2), `TestCommandBeforeReadyReturnsNotReady` (M3), `TestPumpStderrCapsLogSize` (L2).
- `TestCrashWithoutExitFailsWithStderrTail`: last_error chung, stderr chỉ trong log, pending=start còn, `claim` sau retry vẫn trả action start kèm prompt (M1).
- `TestOutcomeMapsExitToStatus`, `TestStopReasonsReachBookStatus`: cập nhật theo last_error chung và ClearPending theo lượt sạch/lỗi.
- `TestExportCacheInvalidatedByRewrite` (M4), `TestExporterWaitDirBlocksUntilExportEnds` (L6); test export/flow cũ cập nhật tên file cache; flow kiểm `pending_text` rỗng khi mới tạo (L8).
- Handler 503 của M3 chưa có test HTTP riêng (cần dựng worker chưa sẵn sàng qua server thật); phần gây ra nó (`ErrNotReady`) có unit test.

## Giữ nguyên theo quyết định đã chốt
- `waiting_review` giữ nguyên khi khởi động; DELETE 202 xoá nền; PATCH budget khi queued/running -> 409 `book_running`.
- L7 `resume_failures`: giữ đúng bảng spec (tăng khi lượt lỗi, đặt lại 0 khi lượt sạch; recoverAll vẫn +1 khi crash). Hệ quả đã biết: lượt lỗi worker cũng cộng vào ngưỡng "gián đoạn nhiều lần khi máy chủ sập". Cần chốt lại nếu muốn tách cột.

## Low bỏ qua (không làm)
L1 (run.go ngoài phạm vi), L4 (PATCH budget race), L5 (đếm MAX_BOOKS_PER_USER không khoá), L9 (`id::text` không dùng PK index), L10 (Hub không gỡ), L11 (DELETE/drain đúng lúc spawn vẫn gửi init).

## Ghi chú
- Lượt lỗi giữ pending nên retry `steer` có thể áp lại góp ý; an toàn vì `mergeSteer` khử trùng và `doIntervention` lưu `PendingSteer` trước (theo review).
- Chưa smoke với worker thật + key thật.

## Gate
`gofmt -l .` rỗng; `go vet ./...` sạch; `go test ./...` PASS; `go test -race -count=1 ./internal/server/... ./internal/entry/worker/...` (có `AINOVEL_TEST_DATABASE_URL`) PASS. Không chạm DB ainovel_e2e hay cổng 28431.

Status: DONE_WITH_CONCERNS
Summary: H1, M1-M4 và Low (a)-(d) đã sửa kèm test; mọi gate xanh.
Concerns/Blockers: L7 giữ theo spec (lỗi worker cộng vào resume_failures); 503 của M3 chưa có test HTTP; chưa smoke worker thật.
