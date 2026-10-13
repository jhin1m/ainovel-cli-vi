-- +goose Up
-- citext: email không phân biệt hoa thường. Là trusted extension từ Postgres 13,
-- chủ database tự tạo được, không cần superuser.
CREATE EXTENSION IF NOT EXISTS citext;

-- +goose Down
DROP EXTENSION IF EXISTS citext;
