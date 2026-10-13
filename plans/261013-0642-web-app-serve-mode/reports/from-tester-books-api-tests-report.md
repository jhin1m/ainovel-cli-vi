# Báo cáo test API truyện (phase 4) — tester

Ngày: 2026-10-13. Nhánh feat/web-app. Chưa commit.

## Kết quả
- `go test -race -count=1 ./internal/server/`: PASS (11 test mới + test cũ).
- `go test -race -count=2 ./internal/server/`: PASS (61s).
- `go test -race ./internal/server/books/`: PASS.
- `go vet ./internal/server/...`, `gofmt -l internal/server`: sạch.
- `go build ./...`: OK.
- Không có `AINOVEL_TEST_DATABASE_URL`: test book bị SKIP, không lỗi.
- Thời gian: ~31s mỗi lượt có race (test chậm nhất: restart ~5s, steer/pause ~5s).

## File
- Mới: `internal/server/books_flow_test.go` (TestMain build worker giả một lần, chỉ khi có DB env).
- Sửa: `internal/server/books/handlers.go` (1 bug, xem dưới).

## Bug đã sửa
- **PATCH /api/books/{id} luôn 500.** SQL dùng `$3` vừa cho `budget_usd` (double precision) vừa cho `$3 > $6`; Postgres báo `inconsistent types deduced for parameter $3 (42P08)`. Mọi PATCH chạm tới UPDATE đều lỗi. Sửa: `$3::float8` và `$6::float8`. Test `TestBookBudget` đã bắt được lỗi này trước khi sửa.

## Phủ theo spec
- Tạo truyện review → waiting_review; chương / outline; next; steer khi dừng → queued → chạy; steer khi đang chạy → 200/202; pause → paused; resume.
- Validation: prompt rỗng/quá dài, mode lạ, budget 0 và >trần, model ngoài danh sách (422), model khác provider key (422), key không có/sai định dạng (404), MaxBooksPerUser (409).
- Budget: budget_exhausted; resume bị 409; PATCH ≤ đã dùng → 400; PATCH cao hơn → paused; resume → completed.
- Export txt/epub: Content-Disposition, cache (mtime không đổi, thư mục không còn file tạm), định dạng sai 400, chưa có chương 409.
- Phân quyền: user B nhận 404 trên cả 12 route của truyện A (kể cả events và export); tạo truyện bằng key của A → 404.
- Xoá: 202, biến khỏi danh sách ngay, thư mục bị xoá, worker thoát; xoá key đang dùng → 409 key_in_use.
- SSE: header (text/event-stream, no-cache, X-Accel-Buffering, không nén); thứ tự retry → snapshot → status → event; status live (queued → waiting_review); kết nối thứ hai cùng nhận; vượt MaxSSEPerUser → 429.
- Khôi phục: Serve + huỷ ctx → interrupted trong DB; server mới trên cùng DB → truyện chạy tiếp tới completed.
- Log không chứa API key (kiểm ở các test chính).

## Chưa phủ
- Client không đọc bị cắt: `sseWriteDeadline` là const, nên bỏ qua theo hướng dẫn. Cần biến package hoặc test dài ~10s để kiểm.
- Lỗi crash worker (`[fake:crash]`), hang, lock_busy: thuộc supervisor, đã có ở jobs/fake worker test, không lặp lại ở đây.

## Concern (không sửa, ngoài phạm vi)
1. `jobs/proc.go` `command()`: lệnh abort gọi trước khi ready đặt `abortLater=true` sau khi kiểm `ready`. Nếu `run()` đóng `ready` và đọc `abortLater` giữa hai bước đó, abort bị mất: pause trả 202 nhưng truyện chạy tiếp. Cửa sổ hẹp. Gợi ý: sau khi Store, kiểm lại `ready` và gửi abort trực tiếp.
2. PATCH budget khi truyện đang chạy chỉ cập nhật DB; worker giữ budget của init. Spec không quy định gửi lệnh mới. Cần quyết định sản phẩm.
3. fakeworker: abort trong lúc dựng dàn ý (~300ms đầu) → stop_reason error → `failed`, không phải `paused`. Có thể là hành vi giả lập, không phải lỗi server.
4. Test khởi động scheduler gọi `StartRefresh` có thể tải bảng giá từ mạng (đã có tiền lệ ở TestServerHealthAndTwoStepShutdown).

## Câu hỏi chưa giải quyết
- Ngữ nghĩa PATCH budget trên truyện đang chạy (concern 2).
- Abort đến trước khi init xong: có muốn `pending` (202) hay phải đảm bảo dừng ngay (concern 1)?

Status: DONE_WITH_CONCERNS
Summary: Thêm 11 test end-to-end cho API truyện, SSE, khôi phục và phân quyền với worker giả; tất cả pass với -race và -count=2. Sửa 1 bug thật: PATCH truyện luôn 500 do lỗi kiểu tham số SQL.
Concerns/Blockers: race abort trước ready trong jobs/proc.go (không sửa, thuộc jobs); PATCH budget lúc đang chạy không tới worker; SSE client-không-đọc chưa test vì write deadline là const.
