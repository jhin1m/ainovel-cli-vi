package jobs

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/api"
)

// jobRow là những gì supervisor cần để chạy một lượt worker.
type jobRow struct {
	ID          string
	UserID      string
	Action      string
	Text        string
	Model       string
	BudgetUSD   float64
	AdvanceMode string
}

// jobDB gom mọi chuyển trạng thái của supervisor. Mỗi UPDATE có điều kiện trên
// status hiện tại để không giẫm lên thay đổi đồng thời của handler (vd. xoá truyện).
type jobDB struct {
	pool *pgxpool.Pool
}

// queuedIDs trả về truyện đang chờ chạy theo thứ tự đến trước.
func (d jobDB) queued(ctx context.Context) ([]struct{ ID, UserID string }, error) {
	rows, err := d.pool.Query(ctx, `SELECT id::text, user_id::text FROM books WHERE status = 'queued' ORDER BY updated_at LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ ID, UserID string }
	for rows.Next() {
		var r struct{ ID, UserID string }
		if err := rows.Scan(&r.ID, &r.UserID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// claim chuyển queued → running. false nếu truyện đã đổi trạng thái trước đó.
func (d jobDB) claim(ctx context.Context, id string) (jobRow, bool, error) {
	var r jobRow
	var action, text *string
	err := d.pool.QueryRow(ctx, `
		UPDATE books SET status = 'running', stop_reason = '', last_error = '', started_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'queued'
		RETURNING id::text, user_id::text, pending_action, pending_text, model, budget_usd, advance_mode`, id).
		Scan(&r.ID, &r.UserID, &action, &text, &r.Model, &r.BudgetUSD, &r.AdvanceMode)
	if errors.Is(err, pgx.ErrNoRows) {
		return jobRow{}, false, nil
	}
	if err != nil {
		return jobRow{}, false, err
	}
	r.Action = api.ActionResume
	if action != nil && *action != "" {
		r.Action = *action
	}
	if text != nil {
		r.Text = *text
	}
	return r, true, nil
}

// requeue: worker không lấy được khoá thư mục (lượt trước chưa nhả hẳn) — chờ chạy lại.
func (d jobDB) requeue(ctx context.Context, id string) error {
	_, err := d.pool.Exec(ctx, `UPDATE books SET status = 'queued', updated_at = now() WHERE id = $1 AND status = 'running'`, id)
	return err
}

// finishResult là kết quả một lượt worker.
type finishResult struct {
	Status       api.BookStatus
	StopReason   string
	LastError    string
	Snapshot     []byte // nil = giữ snapshot cũ
	ClearPending bool
	Failed       bool // tăng resume_failures; false = lượt sạch, đặt lại 0
	// AfterDrain: worker tự dừng trong lúc server đang tắt (drain đã đặt interrupted);
	// kết quả vẫn được ghi đè lên interrupted.
	AfterDrain bool
}

// finish ghi kết quả. Chỉ áp khi truyện còn running (hoặc interrupted nếu AfterDrain);
// không đè lên deleting.
func (d jobDB) finish(ctx context.Context, id string, r finishResult) (bool, error) {
	tag, err := d.pool.Exec(ctx, `
		UPDATE books SET
			status = $2, stop_reason = $3, last_error = $4,
			snapshot = COALESCE($5::jsonb, snapshot),
			pending_action = CASE WHEN $6 THEN NULL ELSE pending_action END,
			pending_text = CASE WHEN $6 THEN NULL ELSE pending_text END,
			resume_failures = CASE WHEN $7 THEN resume_failures + 1 ELSE 0 END,
			finished_at = now(), updated_at = now()
		WHERE id = $1 AND (status = 'running' OR ($8 AND status = 'interrupted'))`,
		id, string(r.Status), r.StopReason, r.LastError, nullJSON(r.Snapshot), r.ClearPending, r.Failed, r.AfterDrain)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// saveSnapshot lưu snapshot mới nhất (giới hạn tần suất ở phía gọi).
func (d jobDB) saveSnapshot(ctx context.Context, id string, snap []byte) error {
	_, err := d.pool.Exec(ctx, `UPDATE books SET snapshot = $2::jsonb WHERE id = $1`, id, snap)
	return err
}

// drain: server sắp tắt — mọi truyện đang chạy thành interrupted để lần khởi động
// sau tự chạy tiếp.
func (d jobDB) drain(ctx context.Context) error {
	_, err := d.pool.Exec(ctx, `UPDATE books SET status = 'interrupted', updated_at = now() WHERE status = 'running'`)
	return err
}

// maxCrashRecoveries: số lần tự khôi phục sau khi server sập (không drain êm) trước
// khi bỏ cuộc, tránh vòng lặp một truyện làm sập server mãi.
const maxCrashRecoveries = 3

// recoverAll chạy lúc khởi động:
//   - running (server sập, không kịp drain) → queued, tăng resume_failures; quá
//     ngưỡng → failed;
//   - interrupted (tắt có kiểm soát) → queued, giữ nguyên pending_action/pending_text;
//   - queued giữ nguyên; waiting_review là trạng thái nghỉ, không có process.
func (d jobDB) recoverAll(ctx context.Context) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE books SET status = 'failed', updated_at = now(),
				last_error = 'Truyện gặp lỗi hoặc bị gián đoạn nhiều lần liên tiếp, đã dừng tự khôi phục. Bấm "Thử lại" để chạy tiếp.'
			WHERE status = 'running' AND resume_failures + 1 >= $1`, maxCrashRecoveries); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE books SET status = 'queued', pending_action = COALESCE(pending_action, 'resume'),
				resume_failures = resume_failures + 1, updated_at = now()
			WHERE status = 'running'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE books SET status = 'queued', pending_action = COALESCE(pending_action, 'resume'), updated_at = now()
			WHERE status = 'interrupted'`)
		return err
	})
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

func marshalSnapshot(s *api.BookSnapshot) []byte {
	if s == nil {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	return b
}
