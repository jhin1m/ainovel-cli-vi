-- +goose Up
CREATE TABLE books (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id      uuid,
    prompt          text NOT NULL,
    status          text NOT NULL CHECK (status IN ('queued', 'running', 'waiting_review', 'paused',
                        'budget_exhausted', 'completed', 'failed', 'interrupted', 'deleting')),
    provider        text NOT NULL,
    model           text NOT NULL,
    budget_usd      double precision NOT NULL CHECK (budget_usd > 0),
    -- Ý định của người dùng; worker áp vào RunMeta mỗi lần spawn.
    advance_mode    text NOT NULL CHECK (advance_mode IN ('auto', 'review')),
    -- Việc cần làm ở lượt worker kế tiếp; sống sót qua restart server.
    pending_action  text CHECK (pending_action IN ('start', 'resume', 'steer', 'advance')),
    pending_text    text,
    stop_reason     text NOT NULL DEFAULT '',
    -- api.BookSnapshot mới nhất: nguồn duy nhất cho tiêu đề, số chương, chi phí.
    snapshot        jsonb,
    last_error      text NOT NULL DEFAULT '',
    resume_failures int NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    started_at      timestamptz,
    finished_at     timestamptz,
    -- FK kép: api_key_id phải là key của chính user sở hữu truyện. Xoá key chỉ
    -- bỏ trống api_key_id (Postgres 15+), user chọn key khác qua PATCH.
    FOREIGN KEY (user_id, api_key_id) REFERENCES api_keys (user_id, id) ON DELETE SET NULL (api_key_id)
);
CREATE INDEX books_user_updated_idx ON books (user_id, updated_at DESC);
CREATE INDEX books_status_idx ON books (status);

-- +goose Down
DROP TABLE books;
