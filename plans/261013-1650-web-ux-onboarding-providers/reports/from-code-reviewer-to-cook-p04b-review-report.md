# Review phase 04b: cắt lượt ở ranh giới chương, test flaky, kiểm key

**Điểm: 7/10.** Phần cắt lượt ở ranh giới chương chạy đúng trên đường chính, test có ý nghĩa thật: đổi `SliceGrace` thành 0 thì `TestSliceWaitsForChapterBoundary` đỏ (`map[1:1 2:2 3:1]`). Test flaky đã hết, 10/10 lần chạy pass. Còn hai chỗ chưa ổn. Thứ nhất, phần kiểm key vẫn báo nhầm theo cả hai chiều: một số key hết tiền bị báo "dùng được", còn key Gemini hợp lệ có thể bị báo "lỗi không xác định". Thứ hai, quyết định có gửi `review` lúc cắt hay không đang dựa vào chế độ user chọn, chứ không dựa vào chế độ worker đang chạy thật. Các lỗi này đã được chứng minh bằng probe.

## Phạm vi và cách kiểm
- File: `jobs/{proc,supervisor}.go`, `jobs/supervisor_test.go`, `jobs/fakeworker/main.go`, `server/{config,server}.go` (+`config_test.go`), `models/pricing.go` (+`pricing_test.go`), `keys/catalog.go`, `keys/check.go` (+`check_test.go`). Đọc thêm phía worker và engine để đối chiếu: `entry/worker/{run,stop_reason}.go`, `host/advance_gate.go`, `flow/advance.go`, `books/handlers.go`, litellm v1.8.10 (`errors.go`, `provider/gemini/response.go`).
- Mọi việc chạy trên bản sao trong scratchpad; cây thật không bị sửa.
  - `go build ./...` pass. `go vet ./internal/server/... ./internal/models/...` pass. gofmt sạch.
  - `go test -race` cho `jobs` (cả gói, chạy `-v` để chắc các test DB không bị skip), `keys`, `models`: pass.
  - `go test ./internal/server/`: 10/10 lần pass, mỗi lần khoảng 22s.
  - Mutation: đặt `SliceGrace = 0` trong `TestSliceWaitsForChapterBoundary` thì test fail đúng như mong đợi.
  - Probe: (1) `setAdvanceMode` giữ `mmu` làm `run()` bị chặn; (2) `p.mode` được ghi dù lệnh không tới worker; (3) đưa body lỗi kiểu thật của provider qua `keyChecker.probe` bằng factory bootstrap thật (`modelAt`).

## High

### H1. Kiểm key báo "Key dùng được" cho tài khoản hết tiền (Anthropic, Qwen)
- `internal/server/keys/check.go:128-139` (`classifyBadRequest`)
- Mọi lỗi 400/422 không khớp regex key đều bị xếp là `ok`. Có hai provider trả 400 khi hết tiền:
  - Anthropic: `400 invalid_request_error "Your credit balance is too low to access the Anthropic API…"`
  - DashScope/Qwen: `400 {"code":"Arrearage","message":"Access denied, please make sure your account is in good standing."}`
- Probe cho ra `anthropic hết credit (400) -> ok=true kind=ok` và `qwen nợ cước Arrearage (400) -> ok=true kind=ok`.
- Kịch bản: user hết credit Anthropic, bấm "Kiểm tra" và thấy "Key dùng được". Sau đó tạo truyện, lượt chạy đầu lỗi ngay. Kết quả này trái yêu cầu 3 của phase ("hết số dư/quota → thông báo tương ứng").
- Đề xuất: trước khi trả `ok` cho 400/422, so message với một pattern quota, khớp thì trả `api.KeyCheckQuota`. Ví dụ `(?i)credit balance|insufficient[_ ](balance|credit|quota|funds)|arrearage|billing|quota|余额`. Thêm hai ca này vào `TestKeyCheckResults`.

### H2. Key Gemini hợp lệ có thể bị báo "lỗi không xác định": đúng trường hợp phase muốn sửa
- `internal/server/keys/check.go:100-119` (`probe`)
- Model đề xuất của provider `gemini` là `gemini-2.5-flash`. Model này mặc định có thinking, và thinking ăn vào `maxOutputTokens=32`. Khi hết token, Gemini trả **HTTP 200** với `finishReason: MAX_TOKENS` và không có `parts`. litellm biến response đó thành lỗi `gemini: stream finished before content: finish_reason=MAX_TOKENS` (`StatusCode=0`, `Type=provider`), xem `provider/gemini/response.go:41-43`.
- `ClassifyProviderError` trả `unknown`. `classifyBadRequest` bỏ qua lỗi này vì status không phải 400/422. Kết quả cuối là `kind=unknown`. Probe xác nhận: `gemini thinking ăn hết max_tokens (200, không có parts) -> ok=false kind=unknown`.
- Provider đã trả 200 thì key chắc chắn đã được xác thực. Chưa kiểm với key thật xem "ping" có làm Gemini nghĩ quá 32 token không, nhưng đường code thì đã được chứng minh.
- Đề xuất: lỗi `LiteLLMError` có `StatusCode == 0`, thuộc loại không phải network/timeout và message chứa `finish_reason=` thì coi là `ok`. Một cách hẹp hơn là chỉ bắt `MAX_TOKENS`/`length`. Thêm test với provider `gemini` và server giả trả body trên. `modelAt` đã hỗ trợ sẵn, probe chạy được ngay.

## Medium

### M1. Có gửi `review` lúc cắt hay không lại dựa vào chế độ user chọn, nên worker có thể vẫn ở auto và bị cắt giữa chương sau ân hạn
- `internal/server/jobs/proc.go:108-116` (`setAdvanceMode` ghi `p.mode` trước khi gửi, kể cả khi gửi thất bại) và `proc.go:121-126` (`stopAtChapterBoundary` thấy `p.mode == review` thì không gửi gì).
- `p.mode` là ý của user, không phải chế độ worker đang chạy. Có hai đường làm hai giá trị này lệch nhau:
  1. PATCH đổi sang review nhưng lệnh không tới worker: `ErrNotReady` (lệnh chưa gửi), ack `busy`/`OK=false`, hoặc lỗi ghi. `p.mode` vẫn thành review. Probe: `Command err=worker chưa sẵn sàng nhận lệnh, p.mode="review"`, sau đó `stopAtChapterBoundary=true` mà không gửi lệnh nào.
  2. PATCH đổi sang review rơi đúng khe giữa `p.sliced.Store(true)` (`supervisor.go:254`) và lúc goroutine `sliceAtChapterBoundary` lấy `mmu`. Lệnh chỉ được ghi nhận, không gửi; goroutine thấy review nên cũng không gửi.
- Hậu quả: worker vẫn chạy auto suốt `SliceGrace` (20 phút). Trong thời gian đó nó giữ slot và viết thêm chương, rồi bị `terminate` giữa chương. Đây đúng là việc mất token mà phase này muốn tránh.
- Đề xuất, chọn một trong hai:
  - Lúc cắt luôn gửi `set_advance_mode=review`, bỏ lối tắt `p.mode == review`. Lệnh này idempotent; chi phí chỉ là một event "đã chuyển chế độ".
  - Giữ thêm trường `workerMode`, chỉ cập nhật khi ack `OK`, và quyết định dựa trên trường đó.
- Nên thêm unit test: `p.mode = review`, worker giả đang auto, cắt lượt thì phải có lệnh gửi đi.

### M2. `setAdvanceMode` giữ `mmu` trong lúc chờ worker sẵn sàng, chặn `run()` trước khi spawn
- `internal/server/jobs/proc.go:108-115` và `supervisor.go:333-335`
- `schedule()` đưa proc vào `s.procs` trước khi `claim`. Từ lúc đó `IsRunning` đã là true, nên PATCH đổi chế độ gọi `Command` → `setAdvanceMode`. Hàm này khoá `mmu` rồi `p.command` chờ `p.ready` tối đa `ackTimeout` (5s). Trong khi đó `run()` cần `mmu` để gán `p.mode`, và việc này nằm trước bước spawn worker. Hai bên chờ nhau cho tới khi hết timeout.
- Probe với `ackTimeout=800ms`: `run() bị chặn 780ms; Command err=worker chưa sẵn sàng nhận lệnh`.
- Hậu quả:
  - Lượt chạy khởi động trễ 5s.
  - Lệnh đổi chế độ thất bại, trong khi trước 04b nó được gửi sau khi worker sẵn sàng. Nếu `claim` đọc `advance_mode` trước UPDATE của PATCH thì lượt này chạy với chế độ cũ.
  - Đường lỗi `ErrNotReady` này cũng mở ra M1.
- Khe thời gian hẹp: khoảng một lần round-trip `claim` cộng thời gian khởi động goroutine.
- Đề xuất: trong `setAdvanceMode`, chờ `p.ready` (select ready/done/ctx/timeout) **trước** khi lấy `mmu`. `run()` gán `p.mode` trước `markReady()`, nên khi đó không còn tranh chấp.

### M3. Tiêu chí docs chưa làm
- Tiêu chí cuối của phase là "Ghi mục docs cho phase 12 (env `AINOVEL_SLICE_GRACE_MINUTES`) vào plan.md". `plan.md` và `phase-12-docs-and-final-verification.md:27` mới chỉ có `AINOVEL_MAX_RUN_MINUTES`.
- Nên ghi thêm hai điều:
  - Thời gian giữ slot tối đa giờ là `MAX_RUN + SLICE_GRACE` (mặc định 80 phút).
  - Acceptance tại `plan.md:128` ("không giữ slot quá `AINOVEL_MAX_RUN_MINUTES`") cần sửa cho khớp.

## Low
- **L1. Truyện review bị cắt giữ `waiting_review`, không về queued.** `proc.go:356-357` chỉ áp nhánh mới khi chế độ là auto. Spec viết "để chương hiện tại xong rồi xử lý như cắt". Tôi thấy lệch này hợp lý: truyện review vốn sẽ dừng chờ duyệt ở cổng chương, nếu xếp hàng lại thì lượt sau chỉ spawn rồi dừng ngay. Nên ghi lệch này vào báo cáo cook.
- **L2. Can thiệp của user trong lúc ân hạn có thể làm truyện thành paused.** User gửi steer trong thời gian ân hạn mà engine dừng ở cổng review khi `PendingSteer` còn treo: worker trả `pending_steer`, `outcome()` không đi nhánh cắt, truyện thành `paused` và user phải tự bấm tiếp. Nếu không bị cắt, truyện auto đã tự xử lý steer. Hiếm gặp; có thể coi `StopPendingSteer` cùng `sliced` và auto như ranh giới chương (về queued, giữ pending).
- **L3. Lượt cắt không còn cần thiết vẫn nhường slot.** Trong 20 phút ân hạn, truyện chờ có thể đã được slot khác nhận, nhưng lượt bị cắt vẫn dừng ở ranh giới chương và xếp hàng lại, tốn thêm một lần spawn. YAGNI; chỉ ghi nhận.
- **L4. UX nhỏ.** Truyện auto bị cắt sẽ hiện trong event log "Đã chuyển chế độ tiến chương sang nghiệm thu từng chương", rồi câu dừng của cổng có nhắc lệnh `/next` (lệnh TUI). Snapshot trong hub (bộ nhớ) vẫn mang `advance_mode=review` tới lượt sau; DB thì đã được `snapshotWithMode` sửa. UI không đọc `snapshot.advance_mode` (`book-screen.tsx:110` dùng `book.advance_mode`), nên không sai trạng thái nút.
- **L5. Key hợp lệ nhưng không dùng được vẫn báo "dùng được".** `classifyBadRequest` coi mọi 400 khác là `ok`, gồm cả "User location is not supported" (Gemini) và model không tồn tại. Đúng nghĩa "key hợp lệ" nhưng user vẫn không chạy được. Chấp nhận được; có thể bổ sung sau cùng chỗ sửa H1.

## Race và deadlock (yêu cầu b)
- `mmu` giữ trong `stopAtChapterBoundary` khi `p.command` chờ ack tối đa 5s: không deadlock với `outcome()`. `run()` gọi `markDone()` trước `outcome()`, nên `p.command` trả `ErrNotRunning` qua `<-p.done` và nhả `mmu`. Chỗ có vấn đề là M2, ở phía PATCH.
- Thứ tự `sliced.Store` và `setAdvanceMode`: `setAdvanceMode` đọc `sliced` dưới `mmu`. Nếu set auto trước khi cắt thì lệnh auto tới trước và goroutine gửi review sau, đúng. Nếu cắt trước thì lệnh chỉ được ghi nhận, và trường hợp user chọn review rơi vào M1.
- Goroutine `sliceAtChapterBoundary`:
  - Shutdown, drain, xoá truyện, `StopAndWait`: đều gọi `terminate`, `done` đóng, và goroutine thoát ở `<-p.done`. Nếu timer ân hạn kích cùng lúc thì có hai `terminate` song song, vô hại.
  - Goroutine không nằm trong `s.wg`, nhưng luôn thoát ngay sau `done`, nên không rò.
- Drain trong lúc ân hạn mà worker tự dừng ở ranh giới chương: `cutByDrain=false` → nhánh mới → `queued` với `AfterDrain` (khớp `interrupted`). `recoverAll` giữ nguyên `queued`, nên sau khi khởi động lại truyện chạy resume. Đúng.
- Pause trong lúc ân hạn: `stopping` chưa bật nên `MsgAbort` tới worker, worker trả `ExitStopped/StopUser` và `outcome()` đi đường thường ra `paused`. Đã có test.

## Hồi quy so với phase 4 (yêu cầu c)
- `SliceGrace=0`: `sliceAtChapterBoundary` gọi `terminate` ngay, giống hệt phase 4. Mutation xác nhận test phân biệt được hai chế độ.
- pause / drain / lock_busy / xoá truyện: không đổi. `lock_busy` vẫn được xét trước trong `outcome()`; `finish` không đè lên `deleting`.

## pending_action khi dừng ở ranh giới chương (yêu cầu d)
- `ClearPending=true` nên lượt sau chạy `resume`. Từng action:
  - **start**: chỉ có thể dừng ở cổng review sau khi plan xong. `StartsForwardChapter` yêu cầu `Phase == writing` và không có chương dở, nên Progress đã tồn tại và resume đúng. Lỗi mất prompt (M2 của phase 4) không xảy ra ở đường này.
  - **steer**: can thiệp đã được tiêu thụ trước khi tới `review_wait`. Nếu còn treo thì worker trả `pending_steer` chứ không phải `review_wait` (xem L2).
  - **advance**: chỉ có ở truyện review, nên không đi nhánh mới, trừ khi user đổi sang auto trong lúc cắt. Khi đó resume ở auto là đúng ý user.
- Chế độ trên đĩa (RunMeta = review) được sửa lại ở lượt sau: `run.go:279-283` áp `init.AdvanceMode` khi khác, đã có test `TestRunSkipsSetAdvanceModeWhenUnchanged` và test SetAdvanceMode:auto trong `run_test.go`.

## Hợp đồng công khai (yêu cầu e)
- `Catalog.StartRefresh` đổi thành `RefreshLoop`: grep cho thấy không còn caller cũ, chỉ `server.go:278` gọi hàm mới. `models.StartPricingRefresh` giữ nguyên hành vi cho CLI (`host/host.go:156`).
- Env `AINOVEL_SLICE_GRACE_MINUTES` là phần thêm, nhận giá trị trong [0, 10080]. Giá trị âm, không phải số hoặc vượt trần đều báo lỗi, có test.
- Test flaky đã hết: `shutdownRest` → `afterHTTP` (`bgCancel` + `bgWG.Wait`) chạy trước khi `TempDir` bị xoá (t.Cleanup chạy LIFO). `RefreshPricing` không ghi cache khi ctx đã huỷ.

## Đối chiếu tiêu chí
| Tiêu chí | Kết quả |
|---|---|
| Lượt bị cắt không bỏ chương dở (đếm số lần viết = 1) | Đạt trên đường chính, `TestSliceWaitsForChapterBoundary` đếm `chapterStarts`. Còn lỗ ở M1. |
| Ân hạn hết thì vẫn cắt; env 0 = hành vi cũ | Đạt (`TestSliceGraceExpiredTerminates`, mutation `SliceGrace=0`). |
| Mode auto được khôi phục, không thành `awaiting_review` | Đạt (nhánh `outcome` + `snapshotWithMode`; worker áp lại mode từ init). Assert `advance_mode` trong DB yếu vì DB chưa từng bị đổi; phần quan trọng nằm ở việc không có `waiting_review` và lượt sau chạy hết. |
| Pause trong lúc cắt giữ `paused` | Đạt (`TestPauseWhileSlicingKeepsPaused`). |
| `./internal/server/` 10 lần liên tiếp pass | Đạt: 10/10 trên bản sao. |
| Kiểm key: key hợp lệ không bị báo lỗi vì tham số | Đạt một phần: 400/422 về tham số → ok, nhưng Gemini 200 + MAX_TOKENS → unknown (H2). |
| Kiểm key: key sai → "key không hợp lệ" | Đạt: 401/403 → auth_failed; 400 nhắc tới key (Gemini, xAI) → auth_failed, đã thử bằng probe. |
| Hết số dư/quota → thông báo tương ứng | Chưa đạt với Anthropic/Qwen (H1). |
| Ghi mục docs cho phase 12 vào plan.md | Chưa làm (M3). |

## Thứ tự sửa đề xuất
1. H1 + H2: thêm pattern quota và nhận 200/MAX_TOKENS là ok trong `check.go`, kèm test với server giả.
2. M1: luôn gửi review khi cắt, hoặc thêm `workerMode`.
3. M2: chờ `ready` trước khi lấy `mmu`.
4. M3: ghi docs vào plan.md và sửa acceptance tại `plan.md:128`.
5. L1 ghi vào báo cáo cook; L2 tuỳ chọn.

## Câu hỏi mở
- Với prompt "ping" và `maxOutputTokens=32`, Gemini 2.5 Flash và GPT-5-mini thật có trả response rỗng hoặc lỗi không? Nên thử một lần bằng key thật. Sửa theo H2 thì kết quả đúng ở cả hai trường hợp.

Status: DONE_WITH_CONCERNS
Summary: Cắt lượt ở ranh giới chương, sửa test flaky và đường SliceGrace=0 đều đạt, có test và mutation chứng minh; `./internal/server/` pass 10/10. Kiểm key còn báo "dùng được" cho tài khoản Anthropic/Qwen hết tiền và báo "unknown" cho Gemini trả 200/MAX_TOKENS (High). Lúc cắt, việc có gửi review hay không còn dựa vào chế độ user chọn thay vì chế độ worker đang chạy, kèm việc giữ `mmu` trong lúc chờ worker sẵn sàng (Medium). Tiêu chí docs chưa làm.
