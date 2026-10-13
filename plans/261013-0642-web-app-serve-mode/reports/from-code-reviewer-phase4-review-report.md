# Review phase 4: a214cc8 + b570fad

Ngày: 2026-10-13. Nhánh feat/web-app. Reviewer: code-reviewer. Không sửa code, không commit.

## Phạm vi
- a214cc8: `internal/entry/worker/run.go` (start gọi `Resume()` trước), test worker.
- b570fad: `internal/server/{books,jobs}/*`, `jobs/fakeworker`, migration `0003_books.sql`, `server.go`, `config.go`, `keys/repo.go` (+`DecryptForBook`, +`defer rows.Close()`).
- `internal/host`, `internal/server/{auth,httpx}` không bị sửa (đã kiểm `git show --stat`).

## Kiểm chứng đã chạy
- Worktree tách ở b570fad: `go test -race -count=1 ./internal/server/... ./internal/entry/worker/...` → PASS hết.
- `go vet` sạch; `GOTOOLCHAIN=go1.25.5 go build ./...` OK; `GOOS=windows go build ./...` OK.
- Viết test tạm trong worktree (đã xoá) để tái hiện H1 → FAIL đúng như dự đoán.

## High

### H1. `StopAndWait` treo vĩnh viễn khi xoá đúng lúc scheduler giữ chỗ mà claim thất bại
- `jobs/supervisor.go:152-165`: `schedule` đưa `p` vào `s.procs` rồi mới `claim`. Nếu claim trả `!ok` hoặc lỗi thì chỉ gọi `s.release(p)`, **không** gọi `p.markDone()`.
- `jobs/proc.go:159-184`: `terminate` chờ grace, gửi tín hiệu (no-op vì `cmd == nil`), rồi `<-p.done` không có lối thoát.
- Kịch bản: scheduler đọc `queued()` có truyện X → user DELETE X (status=deleting) → scheduler giữ chỗ `p` cho X → `finishDelete` gọi `live(X)` và nhận `p` → claim thất bại (`status='queued'` không khớp) → release. Goroutine `finishDelete` bị treo mãi. Thư mục không được xoá, dòng nằm ở `deleting` tới lần restart sau và vẫn bị tính vào `MAX_BOOKS_PER_USER`. Goroutine bị rò.
- Đã tái hiện: dựng supervisor, đưa proc vào map, gọi `StopAndWait` song song rồi `release(p)` → sau 2s vẫn không trả về.
- `Shutdown` hiện được `schedMu` trong `BeforeShutdown` che (luôn chạy trước). Nếu gọi `Shutdown` một mình thì cũng treo.
- Sửa: gọi `p.markDone()` trước `s.release(p)` ở nhánh claim lỗi hoặc `!ok` (supervisor.go:159-160). Thêm test như trên.

## Medium

### M1. Lượt đầu chết trước khi engine tạo tiến độ → "Thử lại" biến truyện thành `completed` rỗng
- `jobs/proc.go:271`: `ClearPending = true` cho **mọi** kết cục, kể cả `exit == nil` (crash, :275) và `ExitError` (:290, gồm `failBeforeEngine` của worker: config, model chưa có giá, `newEngine` lỗi không phải lock_busy).
- Truyện mới (`pending_action=start`) gặp các lỗi trên → `failed`, `pending_action=NULL`. User bấm resume → `handlers.go:415` → `COALESCE(pending_action,'resume')` → worker thật `run.go:447-452`: `Resume()` trên thư mục rỗng trả label `""` → `nothing_to_resume` → `proc.go:303` → `completed`. Kết quả: truyện 0 chương ở trạng thái completed. Steer bị chặn (snapshot rỗng → 409 `not_started`), không có lối chạy lại, chỉ còn cách xoá.
- Fake worker che lỗi này: `fakeworker/main.go:382-384` trả **lỗi** khi progress nil, còn worker thật trả `nothing_to_resume`.
- Sửa (chọn một):
  - (a) `outcome()` không xoá pending khi `Failed=true`. Steer lặp lại an toàn vì `mergeSteer` khử trùng và `doIntervention` lưu `PendingSteer` trước.
  - (b) `handleResume` dùng `start` khi `snapshot == nil || snapshot.Phase == ""`.

  Nên sửa thêm fake worker cho khớp với worker thật.

### M2. Worker tự thoát trong lúc drain → `pending_action` không bị xoá → sau restart `advance` viết thêm một chương user chưa duyệt
- `jobs/supervisor.go:356-365`: hễ `draining` là bỏ qua `finish` (không đổi status, không xoá pending), kể cả khi worker đã thoát **tự nhiên** với `review_wait`/`complete`.
- Kịch bản: user bấm `/next` (pending=advance) → worker nộp xong chương N, engine dừng → trong khoảng drain (từ `BeforeShutdown`, qua HTTP drain tối đa `requestTimeout+2s`, cộng chu kỳ poll 3s của worker) worker báo `review_wait` hoặc nhận `shutdown` → run bỏ qua → truyện vẫn `interrupted` với `pending_action='advance'` → restart → `AdvanceOneChapter` cấp phép `progress.NextChapter()` = N+1 (`internal/host/host.go:870-874`) → viết chương N+1 mà user chưa bấm duyệt, tốn tiền của user. Với `steer` thì can thiệp bị áp lần hai.
- Cùng hậu quả khi server sập ngay sau khi worker thoát mà chưa kịp `finish`, nhưng cửa sổ đó chỉ vài ms.
- Sửa: khi `draining` nhưng `p.exit != nil && p.exit.Reason != ExitShutdown` (worker tự dừng), vẫn ghi kết quả với `WHERE status IN ('running','interrupted')`. Tối thiểu cũng phải xoá pending. Spec "draining thì không đổi status" nhắm vào lượt bị drain cắt ngang, không phải lượt đã tự xong.

### M3. Steer trả 202 nhưng lệnh không bao giờ được gửi
- `jobs/proc.go:124-132`: khi worker chưa `ready` sau 5s (hello chậm, tối đa `helloTimeout` 10s), `command` trả `(nil, nil)` mà **chưa ghi lệnh**. `handlers.go:452-459` hiểu thành "đã gửi, chờ ack" → 202 `pending=true`. Văn bản steer của user bị mất và SSE sẽ không bao giờ báo gì.
- Áp dụng cho `steer` và `set_advance_mode` (lệnh sau ít hại vì DB đã cập nhật).
- Sửa: hết hạn chờ `ready` thì trả `ErrNotRunning` (handler rơi xuống nhánh xếp hàng hoặc 409), hoặc chờ `ready` cho tới `done` hay `ctx`.

### M4. Cache export theo `CompletedCount` trả file cũ sau khi viết lại hoặc trau chuốt chương
- `books/export.go:97-100`: khoá cache chỉ là số chương đã chốt. Rewrite, polishing hay reopen theo steer đổi nội dung nhưng không đổi số chương → `<N>.txt|epub` cũ được trả mãi, tới khi có chương mới.
- Spec ghi "cache theo CompletedCount" nên đây là lỗ hổng ở spec, không phải sai triển khai.
- Sửa: thêm vào khoá mtime lớn nhất của các file chương (hoặc của progress/summary), hoặc xoá `exports/` khi worker báo `CompletedCount`/`Flow` đổi.

## Low

- **L1 (a214cc8)** `run.go:436`: start chạy lại trên truyện đã `complete` (pending `start` còn sót do M2 hoặc crash ở lượt đầu chạy auto) → `resumeLabel` trả `""` vì `Phase==complete` → `StartPrepared` → `refuseNewBookOverExisting` chặn → `failed` với thông báo khó hiểu. Không mất dữ liệu. Sửa: ở nhánh start, nếu progress đã có (kể cả complete) thì báo `nothing_to_resume`.
- **L2** `proc.go:208-229`: log stderr chỉ bị cắt lúc spawn (kiểm `> 5MB` khi mở). Trong một lượt chạy dài (auto viết cả truyện) thì không có giới hạn. Sửa: đếm byte đã ghi, quá 5MB thì truncate/rotate.
- **L3** `proc.go:279-282` + `api.Book.LastError`: 8KB stderr thô (stack panic, đường dẫn DATA_DIR, đường dẫn build) trả thẳng cho client qua REST và SSE. Spec yêu cầu ghi vào `last_error`, nhưng nên lưu đủ trong log hoặc DB và chỉ trả client một dòng tóm tắt.
- **L4** `handlers.go:229-271`: 409 `book_running` kiểm trên dữ liệu đọc trước. UPDATE không có điều kiện status nên resume + claim chen giữa vẫn đổi được ngân sách khi đang chạy. Sửa: thêm `AND (NOT $budgetChanged OR status NOT IN ('queued','running'))`.
- **L5** `handlers.go:194-203`: `Count` rồi `Create` không có khoá → các POST đồng thời vượt được `MAX_BOOKS_PER_USER`. Sửa: `INSERT ... SELECT ... WHERE (SELECT count(*) ...) < $max`, hoặc advisory lock theo user.
- **L6** `export.go:101` và `handlers.go:317`: export đang chạy khi DELETE `RemoveAll` → `MkdirAll(exportsDir)` tạo lại `<book>/exports` sau khi dòng đã bị xoá → thư mục mồ côi.
- **L7** `jobs/db.go:98` + `proc.go:277,292,309`: `resume_failures` tăng cả khi worker lỗi (`Failed=true`), khớp bảng spec nhưng **trái** với quyết định "chỉ tăng khi server sập". Hậu quả: 2 lần worker lỗi, user thử lại, rồi server sập 1 lần → `failed` với thông báo "gián đoạn nhiều lần khi máy chủ gặp sự cố". Cần chốt lại: tách cột hoặc không tăng ở `outcome`.
- **L8** `repo.go:34-36` + `api/books.go:31` + UI `steer-box.tsx:35-37`: với `pending_action='start'`, `pending_text` chính là prompt → UI hiện "góp ý đang chờ: <prompt>" cho truyện mới xếp hàng. Sửa: chỉ trả `pending_text` khi `pending_action='steer'`.
- **L9** `repo.go:83,140,270,299,366`: `id::text = $n` không dùng được PK index. Hiện chấp nhận được vì đã lọc theo `user_id`. Nên validate UUID ở handler rồi so `id = $n`.
- **L10** `jobs/hub.go:30-39`: Hub chỉ bị gỡ khi xoá truyện. Bộ nhớ tăng theo số truyện đã chạm từ lúc khởi động. Event nhỏ nên rủi ro thấp.
- **L11** `supervisor.go:299-323`: DELETE hoặc drain đúng lúc đang spawn → `terminate` thấy chưa `ready` và `stdin` còn nil → không gửi gì; `run` vẫn gửi init → worker chạy (gọi LLM có trả phí) tới khi SIGTERM ở giây thứ 10. Sửa: sau `markReady`, nếu `p.stopping` thì gửi `shutdown` ngay (hoặc bỏ qua init).

## Đối chiếu acceptance
- Giới hạn global/per-user, waiting_review không giữ slot, `/next`, budget, restart resume, env 4 biến, cô lập user (12 route trả 404, key của người khác trả 404), export 429: có test, PASS.
- Chưa có bằng chứng:
  - smoke với worker thật + key thật (bước 7). Toàn bộ e2e dùng fake worker, mà fake worker lệch hành vi worker thật (M1);
  - SSE client không đọc bị cắt ≤10s (tester ghi "chưa phủ").
- Bảo mật:
  - key chỉ đi qua stdin, init được `clear`;
  - `DecryptForBook` JOIN theo FK kép;
  - đường dẫn suy từ UUID đã regex;
  - reader nhận `n` kiểu int;
  - tên file export đã làm sạch;
  - không thấy key trong log, env hay last_error.

## Không phải lỗi (đã kiểm)
- `waiting_review` giữ nguyên khi khởi động, DELETE 202 xoá nền, PATCH budget 409 khi chạy, fake worker chỉ ở dev (`config.go` chặn khi không phải dev): theo quyết định đã chốt.
- `ON DELETE SET NULL (api_key_id)` cần PG15+: docs chốt PG17.
- SSE qua `AccessLog`: `statusRecorder.Unwrap` có, nên Flush/SetWriteDeadline hoạt động.

## Câu hỏi mở
- L7: `resume_failures` có tính lỗi worker không?
- M4: có chấp nhận export cũ sau rewrite cho MVP không?

Status: DONE_WITH_CONCERNS
Summary: Test/vet/build sạch, không regression ở host/auth/httpx. 1 lỗi High đã tái hiện (StopAndWait treo khi claim thất bại), 4 Medium về state machine (completed rỗng sau lỗi lượt đầu, drain làm advance chạy lặp, steer 202 bị mất, cache export cũ) cùng 11 Low.
Concerns/Blockers: Nên sửa H1, M1, M2 trước khi merge; chưa có smoke với worker thật.
