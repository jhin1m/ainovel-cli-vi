// Package books là REST API cho truyện: tạo, điều khiển, đọc chương, dàn ý,
// xuất file và SSE realtime. Mọi truy vấn lọc theo user; truyện người khác = 404.
package books

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/api"
)

// ErrNotFound: truyện không tồn tại, không thuộc user, hoặc đang bị xoá.
var ErrNotFound = errors.New("không tìm thấy truyện")

// bookRow là book kèm các field nội bộ không trả cho client.
type bookRow struct {
	api.Book
	UserID        string
	PendingAction string
}

// Repo truy vấn bảng books.
type Repo struct {
	pool *pgxpool.Pool
}

// NewRepo tạo repo.
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const bookCols = `id::text, user_id::text, prompt, status, stop_reason, provider, model,
	COALESCE(api_key_id::text, ''), budget_usd, advance_mode, COALESCE(pending_action, ''),
	COALESCE(pending_text, ''), last_error, snapshot, created_at, updated_at`

func scanBook(row pgx.Row) (bookRow, error) {
	var b bookRow
	var status string
	var snap []byte
	err := row.Scan(&b.ID, &b.UserID, &b.Prompt, &status, &b.StopReason, &b.Provider, &b.Model,
		&b.APIKeyID, &b.BudgetUSD, &b.AdvanceMode, &b.PendingAction, &b.PendingText, &b.LastError,
		&snap, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return bookRow{}, ErrNotFound
	}
	if err != nil {
		return bookRow{}, err
	}
	b.Status = api.BookStatus(status)
	if len(snap) > 0 {
		var s api.BookSnapshot
		if json.Unmarshal(snap, &s) == nil {
			b.Snapshot = &s
		}
	}
	return b, nil
}

// List trả về truyện của user (trừ truyện đang xoá), mới cập nhật trước.
func (r *Repo) List(ctx context.Context, userID string) ([]api.Book, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+bookCols+` FROM books
		WHERE user_id = $1 AND status <> 'deleting' ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b.Book)
	}
	return out, rows.Err()
}

// Get trả về truyện của user.
func (r *Repo) Get(ctx context.Context, userID, id string) (bookRow, error) {
	return scanBook(r.pool.QueryRow(ctx, `SELECT `+bookCols+` FROM books
		WHERE id::text = $2 AND user_id = $1 AND status <> 'deleting'`, userID, id))
}

// Count đếm truyện của user (kể cả đang xoá, vì thư mục còn trên đĩa).
func (r *Repo) Count(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM books WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}

// newBook là dữ liệu tạo truyện.
type newBook struct {
	UserID, APIKeyID, Prompt, Provider, Model, AdvanceMode string
	BudgetUSD                                              float64
}

// Create tạo truyện ở trạng thái queued với việc chờ làm là start.
func (r *Repo) Create(ctx context.Context, n newBook) (bookRow, error) {
	return scanBook(r.pool.QueryRow(ctx, `
		INSERT INTO books (user_id, api_key_id, prompt, status, provider, model, budget_usd, advance_mode, pending_action, pending_text)
		VALUES ($1, $2, $3, 'queued', $4, $5, $6, $7, 'start', $3)
		RETURNING `+bookCols, n.UserID, n.APIKeyID, n.Prompt, n.Provider, n.Model, n.BudgetUSD, n.AdvanceMode))
}

// update chạy một UPDATE có điều kiện và trả về book mới; ErrNotFound khi không
// dòng nào khớp (sai chủ, hoặc trạng thái đã đổi).
func (r *Repo) update(ctx context.Context, sql string, args ...any) (bookRow, error) {
	return scanBook(r.pool.QueryRow(ctx, sql+` RETURNING `+bookCols, args...))
}

// KeyInUse báo key đang được truyện chạy hoặc chờ chạy dùng.
func (r *Repo) KeyInUse(ctx context.Context, userID, keyID string) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM books
		WHERE user_id = $1 AND api_key_id::text = $2 AND status IN ('queued', 'running')`, userID, keyID).Scan(&n)
	return n > 0, err
}

// deletingBooks trả về truyện đang dở việc xoá (để làm tiếp sau restart).
func (r *Repo) deletingBooks(ctx context.Context) ([][2]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, user_id::text FROM books WHERE status = 'deleting'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][2]string
	for rows.Next() {
		var id, uid string
		if err := rows.Scan(&id, &uid); err != nil {
			return nil, err
		}
		out = append(out, [2]string{id, uid})
	}
	return out, rows.Err()
}

func (r *Repo) hardDelete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM books WHERE id::text = $1 AND status = 'deleting'`, id)
	return err
}
