-- Truyện của provider thuê bao (Z.ai Coding Plan) không có ngân sách USD và lưu
-- budget_usd = 0. Tên ràng buộc là tên mặc định Postgres đặt cho CHECK ở 0003.
-- +goose Up
ALTER TABLE books
    DROP CONSTRAINT IF EXISTS books_budget_usd_check,
    ADD CONSTRAINT books_budget_usd_check CHECK (budget_usd >= 0);
-- Hoàn lại giá trị mà Down đã đổi (chỉ provider thuê bao mới có budget 0).
UPDATE books SET budget_usd = 0 WHERE provider = 'zai-coding';

-- Postgres kiểm CHECK (kể cả NOT VALID) ở mọi UPDATE, nên dòng budget 0 phải đổi
-- sang giá trị dương trước khi siết lại; nếu không, binary cũ lỗi khi cập nhật
-- trạng thái truyện (recover lúc khởi động, claim hàng đợi). Binary cũ không có
-- provider zai-coding nên các truyện này chỉ dừng với lỗi init, không chạy được.
-- +goose Down
UPDATE books SET budget_usd = 0.01 WHERE budget_usd = 0;
ALTER TABLE books
    DROP CONSTRAINT IF EXISTS books_budget_usd_check,
    ADD CONSTRAINT books_budget_usd_check CHECK (budget_usd > 0);
