-- Đăng ký tự do: bỏ bảng mã mời. Không có FK nào trỏ tới bảng này.
-- +goose Up
DROP TABLE IF EXISTS invite_codes;

-- Down dựng lại bảng y hệt 0002_auth.sql để goose down qua 0002 vẫn chạy được.
-- Dữ liệu mã mời cũ không lấy lại được.
-- +goose Down
CREATE TABLE invite_codes (
    code_hash  bytea PRIMARY KEY,
    max_uses   int NOT NULL CHECK (max_uses > 0),
    used_count int NOT NULL DEFAULT 0,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
