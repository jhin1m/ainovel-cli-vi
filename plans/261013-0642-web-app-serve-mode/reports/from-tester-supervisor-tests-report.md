# Báo cáo test supervisor (internal/server/jobs)

## Kết quả
- `go test -race -count=1 ./internal/server/jobs/...`: xanh
- `-race -count=3`: xanh (jobs 34s, fakeworker 11s)
- `-race -cpu=1`: xanh
- `go vet ./internal/server/jobs/...`, `gofmt -l`, `go build ./...`: sạch

## Test thêm: `internal/server/jobs/supervisor_test.go` (18 test)
- Ánh xạ stop_reason → status: `review_wait`, `budget`, `complete`, `error` (kèm reset/tăng `resume_failures`), `user` qua `Command(abort)`.
- Bảng `outcome()` cho mọi stop_reason, exit error, shutdown, không có exit, lock_busy.
- Hello lệch version → `interrupted`, worker bị kill.
- `[fake:hang]` → `StopAndWait` mất ≥ 2×grace (leo thang SIGKILL).
- `[fake:crash]` → `failed` + đuôi stderr; key không có trong `last_error` và `worker-stderr.log`.
- `[fake:lockbusy]` → `queued`, chạy sau backoff, không tính `resume_failures`.
- Giới hạn: global, per-user, `waiting_review` không giữ slot, không spawn chồng cho một truyện.
- Khôi phục: `running` → queued (+1 failures), `interrupted` → queued giữ `pending_action`/`pending_text`, crash lặp → failed.
- Drain: `BeforeShutdown` + `Shutdown` → interrupted; supervisor mới → resume.
- Env worker là đúng 4 biến (`workerCmd`).
- Snapshot ghi vào `books.snapshot`; hub phát snapshot/event/status; client mới nhận ring.
- Abort gửi trước khi ready được ghi nhớ và gửi sau init.

## Bug tìm được và đã sửa
1. **Spawn lỗi không đóng `p.done`** (`supervisor.go:229`, `:310`, `:351`; `proc.go:77`). Đường `MkdirAll`/pipe/`cmd.Start` lỗi return mà không đóng `done`. Khi đó `terminate` (StopAndWait/Shutdown) chờ mãi, rồi `signalGroup(nil)` panic. Sửa: `markDone()` idempotent, `defer` ở `run`. Test: `TestSpawnFailureReleasesProc` (đã fail trước khi sửa).
2. **`signalGroup(p.cmd)` nil-unsafe và đọc `p.cmd` không đồng bộ** (`proc.go:81-88`, `:177`, `:183`, `supervisor.go:277`, `:519`). `run` ghi `p.cmd` ngoài lock; `Shutdown`/`terminate` đọc. Sửa: `p.signal()` đọc dưới `wmu` và bỏ qua khi `cmd == nil`.
3. **Abort có thể bị mất** (`proc.go:115-121`). Sau khi `abortLater.Store(true)`, kiểm lại `ready`; nếu `run` đã ready thì gửi abort ngay. Đã kiểm chứng bằng lập luận thứ tự (ready đóng trước khi `run` đọc cờ). Test: `TestAbortBeforeReadyIsRemembered` chỉ kiểm hợp đồng, không tái hiện được interleaving (không deterministic).
4. **Claim chạy sau drain** (`supervisor.go:58-60`, `:127-128`, `:485-486`). `schedule` có thể claim → `running` sau khi `BeforeShutdown` đã drain, khi đó restart coi là crash và tăng `resume_failures`. Sửa: `schedMu` tuần tự hoá. Chưa có test xác định (cửa sổ rất hẹp).
5. **Test flaky sẵn có `TestHubSlowSubscriberDropped`** (`hub_test.go`, vòng publish). Publisher không nhường CPU nên với `-cpu=1` subscriber nhanh bị đầy buffer và ngắt. Sửa: `time.Sleep(1ms)` mỗi 16 message. Hub không đổi.

## Còn lại / rủi ro
- `hub.go` không sửa.
- Spec vs code: `recoverAll` không đổi `waiting_review` thành `interrupted` như spec (dòng 1 bảng khôi phục). Code giữ nguyên có chủ đích (đã ghi trong comment); cần xác nhận.
- `interrupted` giữ `pending_action='start'` cho truyện bị drain khi đang start; spec ghi `resume`. Fakeworker chạy được vì `createStory` idempotent, worker thật chưa kiểm.
- `StopAndWait` trên worker treo làm truyện thành `failed` ("Worker dừng bất thường"). Chấp nhận khi status đã là `deleting`; cần handler xác nhận.
- Tổng thời gian jobs ~13s mỗi lượt race, dùng worker thật (không mock).
- File `supervisor_test.go` bị thay đổi trên đĩa bởi tiến trình khác trong lúc làm việc; nội dung hiện tại được giữ nguyên, không revert.

## Chưa giải quyết
- Có nên coi `waiting_review` là trạng thái nghỉ (code) hay phải khôi phục như spec?
- `pending_action` giữ `start` sau drain có chấp nhận được không?

Status: DONE_WITH_CONCERNS
Summary: 18 test supervisor mới cùng 4 bug sửa trong jobs (spawn lỗi treo shutdown, race p.cmd, mất abort, claim sau drain) và sửa flake hub test; `-race` xanh count=3 và cpu=1, vet/gofmt/build sạch.
Concerns/Blockers: spec vs code về `waiting_review` và `pending_action` khi khôi phục; race claim/drain chưa có test xác định.
