# Cook phase 4b: cắt lượt ở ranh giới chương, sửa test flaky, kiểm key không báo nhầm

Ngày: 2026-10-13 | Phase: [phase-04b](../phase-04b-slice-at-chapter-boundary.md) | Review: [from-code-reviewer-to-cook-p04b-review-report.md](./from-code-reviewer-to-cook-p04b-review-report.md).

Review lần đầu chấm 7/10. Đã sửa cả 2 High (H1, H2), 3 Medium (M1, M2, M3) và một Low (L2). Mỗi chỗ sửa đều có test.

## Đã làm

### 1. Cắt lượt ở ranh giới chương (`internal/server/jobs`)
- `Config.SliceGrace`. Khi `slice()` chọn được lượt cần cắt, nó chạy goroutine `sliceAtChapterBoundary`:
  - Gửi `set_advance_mode=review`. Lệnh luôn được gửi, kể cả khi user đã chọn review (sửa M1), vì lệnh đổi mode trước đó có thể chưa tới worker.
  - Chờ `p.done` tối đa `SliceGrace`. Quá hạn, hoặc worker trả ack lỗi, thì `terminate` như phase 4. `SliceGrace = 0` cũng `terminate` ngay.
- `proc.mode` là chế độ user chọn, lấy từ `row.AdvanceMode` lúc chạy.
  - `Supervisor.Command(set_advance_mode)` gọi `setAdvanceMode`. Nếu đang cắt lượt thì chỉ ghi nhận rồi trả ack ok: worker phải giữ review, và lượt sau áp lại mode từ init.
  - Hàm chờ worker sẵn sàng trước, rồi mới lấy `mmu` (sửa M2). Phần chờ này tách thành `waitReady`, dùng chung với `command`.
- `outcome()`: lượt bị cắt dừng ở `review_wait` hoặc `pending_steer` (L2) mà user muốn auto thì truyện về `queued`, xoá pending (lượt sau resume) và không tính lỗi. Snapshot được lưu với `advance_mode=auto`. User muốn review thì giữ kết quả thật (`waiting_review`).
- Pause trong lúc cắt: lệnh abort vẫn tới worker (vì `stopping` chưa bật), nên truyện thành `paused`, đúng như cũ.
- fakeworker có thêm marker `[fake:slow]`: mỗi chương khoảng 6s.

### 2. Config
- Thêm `AINOVEL_SLICE_GRACE_MINUTES`: mặc định 20, nhận 0..10080, 0 nghĩa là cắt ngay.
- Thêm helper `envMinutes`, dùng chung với `AINOVEL_MAX_RUN_MINUTES`.
- `server.go` truyền `SliceGrace` sang supervisor.

### 3. Test flaky "directory not empty"
- `models.RefreshPricing(ctx, …)` chạy đồng bộ: fetch theo ctx, và không ghi cache khi ctx đã huỷ.
- `StartPricingRefresh` chỉ còn gọi `go RefreshPricing(context.Background(), …)`. CLI (`host.go`) giữ nguyên hành vi.
- `keys.Catalog.StartRefresh` đổi thành `RefreshLoop`, chặn tới khi ctx huỷ. Đã kiểm, không còn chỗ nào gọi tên cũ.
- Server chạy vòng này trong `bgWG`. `afterHTTP` gọi `bgCancel` rồi `bgWG.Wait()`, nên không còn ghi vào `DataDir/cache` sau khi tắt.

### 4. Kiểm key (`keys/check.go`)
- `checkMaxTokens` tăng từ 8 lên 32.
- `reclassifyUnknown` chỉ chạy khi `ClassifyProviderError` trả `unknown`:
  - Response 200 nhưng bị cắt vì max_tokens (`finish_reason=MAX_TOKENS/length`, ví dụ thinking của Gemini ăn hết token) → `ok` (H2).
  - Lỗi 400/422 nhắc tới key → `auth_failed`. Gemini trả 400 khi key sai.
  - Lỗi 400/422 nhắc tới số dư hoặc hạn mức → `quota` (H1). Anthropic báo "credit balance is too low", Qwen báo "Arrearage".
  - Lỗi 400/422 khác → `ok`: provider từ chối tham số, nghĩa là đã nhận key.

## Tests mới
- **jobs**:
  - Unit: `TestOutcomeSlicedAtChapterBoundary` (auto → queued, review → waiting_review, pending_steer → queued), `TestSetAdvanceModeWhileSlicing`, `TestStopAtChapterBoundaryAlwaysSendsReview`, `TestSetAdvanceModeWaitsReadyWithoutLock`.
  - Integration với fakeworker: `TestSliceWaitsForChapterBoundary` (mỗi chương bắt đầu viết đúng 1 lần, không có `waiting_review`, lượt sau chạy auto tới hết), `TestSliceGraceExpiredTerminates`, `TestPauseWhileSlicingKeepsPaused`.
  - Thử đột biến: đặt `SliceGrace = 0` thì `TestSliceWaitsForChapterBoundary` đỏ (chương 2 bị viết 2 lần).
- **server**: các ca env mới trong `TestLoadConfigErrors` và `TestLoadConfigMaxRunMinutes`; giá trị mặc định kiểm trong test config mặc định.
- **models** (`pricing_test.go`, file mới): ctx đã huỷ thì không ghi gì; cache còn hạn thì nạp mà không cần mạng.
- **keys**:
  - `TestKeyCheckResults` có thêm các ca: 403, 402, 400 lỗi max_tokens, 422, 400 key sai, 400 hết credit, 400 Arrearage. Test kiểm thêm `max_tokens` gửi đi là 32.
  - Thêm `TestKeyCheckTruncatedReplyIsOK`: provider gemini trả 200 với `MAX_TOKENS`.
  - Test giờ dùng lại một user cho mỗi 3 ca, vì giới hạn đăng ký 10/ngày/IP của server. Giới hạn không bị nới.

## Verify (DB `ainovel-testpg`, `AINOVEL_TEST_DATABASE_URL` như phase file)
- `go vet ./internal/...`, biên dịch toàn repo, gofmt: sạch.
- `go test ./internal/server/... ./internal/models/... -count=1`: pass, test DB chạy thật, không skip.
- `-race` trên các test Slice/Outcome/SetAdvance/Pause/StopAt: pass.
- `go test ./internal/server/` chạy lặp 10 lần: 10/10, chạy hai đợt (trước và sau các bản sửa của review).
- `make gen && pnpm -C web typecheck && pnpm -C web test`: pass (63/63). Phase này không đổi package `api` nên `types.gen.ts` không đổi; diff hiện có của file đó là từ phase 1–4.

## Lệch so với phase file
- Truyện vốn ở chế độ review bị cắt thì giữ `waiting_review`, không "xử lý như cắt". Nếu đưa về hàng đợi, lượt sau chỉ spawn rồi lại dừng ở cổng chương.
- Có sửa ngoài danh sách file: `server.go` (để truyền SliceGrace và chờ vòng refresh), `keys/catalog.go` (đổi API refresh), `jobs/fakeworker`.
- Mục docs ghi trong `plan.md` (mục "Mục docs phase 12 phải ghi"). Tiêu chí toàn đợt về giữ slot sửa thành MAX_RUN + SLICE_GRACE.

## Còn lại / rủi ro
- Low từ review, chưa xử lý:
  - L3: trong lúc ân hạn, truyện chờ có thể đã được slot khác nhận, nhưng lượt bị cắt vẫn nhường slot.
  - L4: event log của truyện auto bị cắt sẽ hiện câu "chuyển sang nghiệm thu từng chương".
  - L5: key hợp lệ nhưng không dùng được (vùng không hỗ trợ, model không tồn tại) vẫn được báo `ok`.
- Phân loại 400/422 dựa trên câu chữ lỗi của provider. Cần thử bằng key thật ở phase 12: Anthropic hết credit, Gemini 2.5 Flash với 32 token.
- Không commit (theo ràng buộc). Không chạy docs-manager vì docs thuộc phase 12. Không viết journal.

## Câu hỏi còn mở
- Không.

Status: DONE
