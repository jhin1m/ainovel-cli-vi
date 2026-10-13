# Phase 04b: cắt lượt ở ranh giới chương + sửa test flaky + kiểm key không báo nhầm

Trạng thái: completed (2026-10-13) · Báo cáo: [cook](./reports/cook-cook-p04b-report.md), [review](./reports/from-code-reviewer-to-cook-p04b-review-report.md) · Phụ thuộc: phase 4 (đã xong) · Chạy trước phase 5.

## Bối cảnh
- Báo cáo phase 4: `reports/cook-cook-p04-report.md`, review: `reports/from-code-reviewer-to-cook-p04-review-report.md`.
- Phase 4 cắt lượt (`AINOVEL_MAX_RUN_MINUTES`) bằng `terminate` ngay → chương đang viết dở bị bỏ, user trả token 2 lần. User duyệt "chia lượt" để nhường slot, KHÔNG muốn mất tiền.
- Worker đã hỗ trợ `set_advance_mode` (`internal/server/api/worker.go:22,74`): `review` = dừng sau khi chương hiện tại commit.

## Yêu cầu
1. **Cắt ở ranh giới chương** (`internal/server/jobs/`):
   - Khi chọn được lượt cần cắt: gửi `set_advance_mode=review` thay vì terminate; đánh dấu `slicing` (giữ mode gốc để khôi phục).
   - Khi worker dừng ở ranh giới chương vì lý do do việc cắt gây ra: xử lý như lượt bị cắt hiện tại (về `queued`, giữ pending/resume, xuống cuối hàng, không tính lỗi). Truyện ở chế độ `auto` không được chuyển thành `awaiting_review` — khôi phục mode gốc cho lượt sau.
   - Truyện vốn ở chế độ review: không cần đổi mode; để chương hiện tại xong rồi xử lý như cắt.
   - Ân hạn: nếu sau `AINOVEL_SLICE_GRACE_MINUTES` (mặc định 20, 0 = cắt ngay như cũ) chưa tới ranh giới chương → terminate như phase 4.
   - User bấm tạm dừng trong lúc `slicing` → giữ `paused` (như phase 4 đã xử lý).
   - Test: chương đang viết hoàn thành đúng 1 lần trước khi nhường slot; ân hạn hết → terminate; mode auto được khôi phục; pause trong lúc slicing.
2. **Test flaky "directory not empty"**: goroutine làm mới bảng giá `internal/models/pricing.go:~74` ghi cache vào thư mục tạm không dừng theo context. Cho nó dừng theo context / có cách đợi xong (hoặc test inject đường dẫn cache) để `go test ./internal/server/...` chạy lặp 10 lần không fail. Không đổi hành vi CLI.
3. **Kiểm key không báo nhầm** (`POST /api/keys/test`, phase 4): model có thinking có thể lỗi với `max_tokens` rất nhỏ. Phân loại: 401/403 → key sai; 1113/hết số dư/quota → thông báo tương ứng; lỗi request khác chứng tỏ đã xác thực được (vd. 400 về max_tokens/tham số) → coi là key hợp lệ. Nâng `max_tokens` lên mức an toàn nhỏ (vd. 16–32) nếu cần. Test bằng provider giả.

## File
- Modify: `internal/server/jobs/*` (supervisor, proc, test), `internal/server/config.go` (env mới), `internal/models/pricing.go` (+test), file handler kiểm key của phase 4 (+test).

## Verify
```
AINOVEL_TEST_DATABASE_URL='postgres://postgres:ci@127.0.0.1:55439/postgres?sslmode=disable' go test ./internal/server/... ./internal/models/... -count=1
for i in $(seq 1 10); do AINOVEL_TEST_DATABASE_URL=... go test ./internal/server/ -count=1 || break; done
go vet ./internal/...
make gen && pnpm -C web typecheck && pnpm -C web test
```

## Tiêu chí
- [x] Lượt bị cắt không bỏ chương dở (test đếm số lần viết chương = 1).
- [x] Ân hạn hết thì vẫn cắt; env 0 = hành vi cũ.
- [x] `./internal/server/` chạy 10 lần liên tiếp pass.
- [x] Kiểm key: key hợp lệ không bị báo lỗi chỉ vì tham số; key sai → "key không hợp lệ".
- [x] Ghi mục docs cho phase 12 (env `AINOVEL_SLICE_GRACE_MINUTES`) vào plan.md.

## Ghi chú khi làm
- Truyện vốn ở chế độ review bị cắt: giữ `waiting_review` (kết quả thật), không xếp hàng lại; nếu xếp lại thì lượt sau chỉ spawn rồi dừng ngay ở cổng chương.
- Quyết định "auto hay review" dựa vào chế độ user chọn (DB, cập nhật qua PATCH). Lúc cắt luôn gửi `set_advance_mode=review`; PATCH đến trong lúc cắt chỉ được ghi nhận, lượt sau áp lại từ init.
- Ngoài danh sách file: `server.go` (truyền `SliceGrace`, chờ vòng làm mới bảng giá khi tắt), `keys/catalog.go` (`StartRefresh` → `RefreshLoop`), `jobs/fakeworker` (marker `[fake:slow]`).
