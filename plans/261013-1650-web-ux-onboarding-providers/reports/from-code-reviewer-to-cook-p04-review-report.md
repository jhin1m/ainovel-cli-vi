# Review phase 4: kiểm key + chống lạm dụng

**Điểm: 7/10.** Phần kiểm key, chặn đĩa, dọn tài khoản và config đạt spec, test tốt. Phần chia lượt chạy đúng ba điều kiện như spec, nhưng có 2 lỗi High: slot nhường ra có thể quay về chính user bị cắt, và lệnh pause của user có thể bị lượt cắt ghi đè. Cả hai đã được chứng minh bằng probe.

## Phạm vi và kiểm chứng
- File: keys/check.go(+test), keys/handlers.go, keys/repo.go, api/account.go, auth/handlers.go (IPKey, PurgeAbandonedUsers), worker/config.go (NewProbeModel), jobs/{supervisor,proc,db}.go(+test), server/{config,server}.go, books/handlers.go, testclient/books_flow/auth_flow tests.
- `go build ./...` và `go vet` pass. `go test -race` cho các test Slice/Outcome/KeyCheck/ProbeModel/Cleanup/DiskLow/config trên DB 55439 cũng pass.
- Probe chạy trên bản sao trong scratchpad (cây thật không bị sửa): (1) e2e supervisor cho thấy slot vừa cắt được trao cho truyện khác của cùng user; (2) unit `outcome()` với các trường hợp sliced + StopUser và sliced + exit nil; (3) `NewProbeModel` dựng được model cho đủ 10 provider.

## High

### H1. Slot vừa cắt có thể về tay chính user bị cắt, nên chia lượt không đạt mục đích
- `internal/server/jobs/supervisor.go:205-236` (`sliceVictimLocked`, `hasRunnableOtherLocked`)
- Kịch bản (mặc định MaxJobsPerUser=1): A có A1 đang chạy và A2 đang xếp hàng (bị chặn vì A chạm trần), còn B1 của user B xếp hàng sau A2. Khi A1 quá MaxRun, điều kiện "có truyện user khác chạy được" thỏa vì có B1, nên A1 bị cắt. A1 nhả slot thì `perUser[A]` về 0. Vòng `schedule()` duyệt theo `updated_at` và claim A2 trước B1. B phải chờ thêm MaxRun nữa. Nếu A xếp sẵn k truyện trước B thì B chờ khoảng k×60 phút, đúng kiểu lạm dụng mà tính năng này nhắm tới. Probe e2e (MaxJobs=1, MaxRun=1s) cho thấy a2 chạy (`running→queued`) trước khi b1 được chạy, với 2 lần cắt.
- Test hiện có không bắt được lỗi này vì chỉ có một truyện của A.
- Đề xuất: trong `sliceVictimLocked`, giả lập trạng thái sau khi victim nhả slot (`perUser[victim.userID]-1`). Lấy truyện đầu tiên trong `queued` theo thứ tự mà khi đó chạy được. Chỉ cắt nếu truyện này thuộc user khác victim. Viết lại thành hàm thuần dùng chung `blockedLocked` với perUser điều chỉnh. Thêm test: A1 đang chạy, A2 và B1 xếp hàng, B1 phải được chạy ngay sau lần cắt đầu tiên.

### H2. Lệnh pause của user bị lượt cắt ghi đè, truyện tự chạy lại và tốn quota
- `internal/server/jobs/proc.go:314-321` (nhánh `ExitShutdown` + `sliced`), `supervisor.go:205-224`, `books/handlers.go` handlePause
- Biến thể A (abort đến trước): user bấm Tạm dừng → `MsgAbort` → worker đang dừng engine (thoát sau `doneSeen` hoặc 2 tick idle). Trong lúc đó vòng lập lịch (tick 2s hoặc Kick) chọn proc này làm victim, vì `p.stopping` chưa bật (abort không đặt cờ). Worker nhận shutdown và thoát với `ExitShutdown, StopReason=user` (run.go:615-618). `outcome()` thấy `sliced`, bỏ qua StopReason và trả `queued + resume`. Probe: `user abort + slice -> status=queued resume=true`. Truyện người dùng đã dừng lại tự chạy tiếp, tốn tiền hoặc quota của họ.
- Biến thể B (cắt đến trước): trong cửa sổ tối đa khoảng 20s, `Command` trả `ErrNotRunning` vì `stopping` đã bật. Pause rơi xuống `UPDATE … WHERE status='queued'` và trả 409 "Truyện không đang chạy". Sau đó truyện vẫn về queued và tự chạy.
- Đề xuất: (1) `outcome()` chỉ đi nhánh sliced khi `p.exit.StopReason != api.StopUser`. (2) Thêm cờ `userAborted` trên proc, đặt trong `Supervisor.Command` khi `cmd.Type == MsgAbort`, và để `sliceVictimLocked` bỏ qua proc có cờ này. (3) Với biến thể B: khi pause gặp proc đang sliced, gọi một hàm kiểu `sup.CancelSlice(bookID)` để chuyển `sliced` về false (outcome sẽ ra `paused`) rồi trả 202 pending. Thêm unit test cho `outcome()` với sliced + StopUser.

## Medium

### M1. Lượt cắt mà worker bị SIGKILL bị tính là lỗi và truyện dừng ở failed
- `internal/server/jobs/proc.go:305-311`; `supervisor.go:239-243`
- Kịch bản: `closeEngine` của worker chờ tối đa `closeGrace = 20s` (run.go:70), bằng đúng tổng hai bước `stopGrace` 10s + 10s trước khi gửi SIGKILL. Nếu engine đóng chậm (lời gọi executor đang ghi store), process bị SIGKILL và `p.exit == nil`. Kết quả: `failed`, `msgWorkerCrashed`, `resume_failures+1`. Lượt cắt do server chủ động nhưng user thấy "Worker dừng bất thường" và phải tự bấm Thử lại. Trước đây đường drain và xoá bỏ qua outcome nên không gặp; chia lượt là đường đầu tiên ghi outcome sau một lần terminate do server khởi xướng.
- Đề xuất: trong `outcome()`, nếu `sliced && p.exit == nil` thì trả `queued + ResumeNext`, `Failed=false`, và log warn kèm stderr tail.

### M2. ResumeNext ghi đè `pending_action='start'` và xoá prompt, khác bất biến của worker
- `internal/server/jobs/db.go:102-103`
- Worker cố ý chạy lại `start` khi lượt bị gián đoạn (run.go:443-447: "chạy lại vẫn với action start"). Drain cũng giữ pending. Chia lượt thì đổi sang `resume` và đặt `pending_text = NULL`.
- Kịch bản: với `AINOVEL_MAX_RUN_MINUTES` < 3, lượt start bị cắt trong `PrepareUserRules` (timeout 3 phút, host.go:99), trước khi `Progress.Init` chạy. Lần resume sau, `resumeLabel` trả "" nên worker báo `nothing_to_resume` và outcome ghi `completed`. Truyện rỗng bị đánh dấu hoàn thành và prompt bị mất. Với mặc định 60 phút thì gần như không xảy ra, nhưng đây là mất dữ liệu phụ thuộc cấu hình.
- Đề xuất: `pending_action = CASE WHEN $9 THEN CASE WHEN pending_action = 'start' THEN 'start' ELSE 'resume' END …`, và chỉ xoá `pending_text` khi action không phải start. Cách khác: đặt sàn MaxRun ≥ 5 phút.

## Low
- **L1.** `internal/server/config.go:152-159`: `AINOVEL_MAX_RUN_MINUTES` không có trần. `time.Duration(n)*time.Minute` (server.go, chỗ dựng `jobCfg`) tràn int64 khi n > ~1.5e8: kết quả có thể âm (lặng lẽ tắt chia lượt) hoặc quay vòng thành số dương nhỏ (cắt liên tục). Đề xuất trần 10080 (7 ngày) và báo lỗi khi vượt.
- **L2.** `internal/server/keys/check.go:152-159`: lượt bị 429 `busy` vẫn tiêu quota phút/ngày của user và quota ngày của IP. Code đúng thứ tự spec nên chỉ ghi nhận. Nếu muốn, có thể hoàn lượt (thêm `Limiter.Undo`) hoặc kiểm semaphore trước bước IP.
- **L3.** `internal/server/auth/handlers.go` (`PurgeAbandonedUsers`): chỉ xoá dòng DB. Nếu user từng có truyện rồi xoá hết (thư mục `HomeDir` vẫn còn), thư mục `data/…/<user_id>` bị bỏ lại trên đĩa. Rủi ro nhỏ về dung lượng, đề xuất `os.RemoveAll` theo danh sách id trả từ `DELETE … RETURNING id`.

## Đối chiếu yêu cầu và acceptance criteria
| Criterion | Kết quả |
|---|---|
| Chỉ nhận `key_id` của chính user; key người khác → 404; có `api_key` → 400 | Đạt. `DecryptOwned` dùng `WHERE user_id=$1 AND id::text=$2`, AAD dựng từ user của phiên. Có test. |
| Có limiter user phút/ngày, IP/ngày, semaphore 4; mỗi loại có test; nối vào sweep | Đạt. `keysH.SweepLimiters()` được gọi trong `cleanup()`. |
| Response chỉ có kind/code đóng và message do server viết | Đạt. `checkResult` ép kind lạ về unknown, code đi qua allowlist. Log chỉ có user/provider/kind/code. Test kiểm `org-SECRET42`, key và `balance` không xuất hiện trong response. |
| Gọi provider 1 lần, timeout 20s, không qua llmretry | Đạt. Chuỗi gọi là Swappable → rejectingModel → adapter, không có lớp retry (litellm v1.8.10 không tự retry). Timeout 20s < requestTimeout 30s. |
| Không SSRF | Đạt. BaseURL lấy từ `LookupProvider`; `newModel` chỉ đổi được trong test cùng package. Provider đọc từ DB và đã được allowlist lúc tạo key. |
| Chia lượt + test âm từng điều kiện | Đạt về hình thức. Mục đích chưa đạt (H1), còn race với pause (H2) và đường SIGKILL (M1). |
| `schedule()` sau refactor | Giữ nguyên hành vi: thứ tự draining → hết slot → blocked (live/perUser/backoff) như cũ, đặt `started` trước khi vào `s.procs` dưới `s.mu`. |
| SQL `finish` với `$9` | Đúng: đủ 9 tham số, `resume_failures` về 0 với lượt sạch. Chỉ còn M2. |
| Drain hoặc Shutdown khi proc đang sliced | An toàn: `cutByDrain` (vì `stopping`) giữ interrupted. Hai lần `terminate` chạy song song không gây hại; goroutine `slice` thoát theo `p.done`. |
| Xoá truyện đang bị cắt | An toàn: status `deleting` khiến `finish` (`WHERE status='running'`) không có dòng nào khớp. |
| Đĩa < 1 GiB → 507 | Đạt. `ok=false` hoặc hàm nil thì không chặn. Có test. |
| Dọn tài khoản rỗng > 30 ngày | Đạt, có test âm cho key/truyện/session/mới/admin. Bỏ qua admin là lệch spec có chủ ý, chấp nhận được. Mọi FK tới users đều `ON DELETE CASCADE` nên DELETE không bị chặn. |
| Public contract | API mới `POST /api/keys/test`, env `AINOVEL_MAX_RUN_MINUTES`, `Options.DiskFree` và `books.Deps.DiskFree` đều là phần thêm (additive). `types.gen.ts` đã sinh lại. Docs env để phase 12. |

## Race và deadlock
- `p.started` được ghi dưới `s.mu` trước khi proc vào map, và chỉ được đọc dưới `s.mu`, nên không race. `sliced`, `stopping` và `ready` dùng atomic hoặc channel.
- `slice()` chạy sau `s.mu.Unlock()`. Nếu proc vừa thoát xong thì `terminate` gặp `done` đã đóng và trả về ngay, không bị kẹt.
- `-race` pass trên các test liên quan.

## Thứ tự sửa đề xuất
1. H2: sửa outcome bỏ qua StopUser và skip proc đã abort. Ít dòng, chặn được việc ghi đè ý định của user.
2. H1: chọn victim theo "truyện đầu tiên chạy được sau khi nhả slot thuộc user khác", kèm test A1/A2/B1.
3. M1, M2: sửa trong `outcome()` và câu SQL `finish`.
4. L1: thêm trần cho env.

## Câu hỏi mở
- Với max_tokens=8 trên model có thinking (gemini-2.5-flash, gpt-5-mini), provider thật có trả 200 với nội dung rỗng hay trả 400? Nếu trả 400, key hợp lệ sẽ bị báo `unknown`. Chưa kiểm được vì không có mạng hay key thật; nên thử tay một lần trước khi deploy.

Status: DONE_WITH_CONCERNS
Summary: Phần kiểm key, chặn đĩa và dọn tài khoản đạt spec, không thấy lộ key hay body của provider, không IDOR, không SSRF. Chia lượt còn 2 lỗi High đã chứng minh bằng probe (slot về lại cùng user; pause bị ghi đè thành queued) và 2 lỗi Medium (SIGKILL bị tính là failed; mất prompt start khi MaxRun nhỏ).
