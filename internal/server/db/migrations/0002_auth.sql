-- +goose Up
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    is_admin      boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Chỉ lưu sha256 của token: lộ bảng sessions không dùng được để đăng nhập.
CREATE TABLE sessions (
    token_hash   bytea PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE invite_codes (
    code_hash  bytea PRIMARY KEY,
    max_uses   int NOT NULL CHECK (max_uses > 0),
    used_count int NOT NULL DEFAULT 0,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- API key LLM của người dùng, mã hoá AES-256-GCM với AAD = user_id|id.
-- unique (user_id, id) để books tham chiếu bằng FK kép: không thể gắn key của người khác.
CREATE TABLE api_keys (
    id          uuid NOT NULL DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    text NOT NULL,
    label       text NOT NULL DEFAULT '',
    ciphertext  bytea NOT NULL,
    nonce       bytea NOT NULL,
    key_version int NOT NULL DEFAULT 1,
    last4       text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    UNIQUE (user_id, id)
);

-- +goose Down
DROP TABLE api_keys;
DROP TABLE invite_codes;
DROP TABLE sessions;
DROP TABLE users;
