package keys

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/secret"
)

// ErrNotFound: key không tồn tại hoặc không thuộc user.
var ErrNotFound = errors.New("không tìm thấy API key")

// Repo lưu API key đã mã hoá. Không có hàm nào trả key rõ ra ngoài trừ đường
// giải mã cho worker (theo truyện, kiểm chủ sở hữu bằng FK kép).
type Repo struct {
	pool *pgxpool.Pool
	box  *secret.Box
}

// NewRepo tạo repo.
func NewRepo(pool *pgxpool.Pool, box *secret.Box) *Repo {
	return &Repo{pool: pool, box: box}
}

// List trả về key của user, mới nhất trước.
func (r *Repo) List(ctx context.Context, userID string) ([]api.APIKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, provider, label, last4, created_at FROM api_keys
		WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	out := []api.APIKey{}
	for rows.Next() {
		var k api.APIKey
		if err := rows.Scan(&k.ID, &k.Provider, &k.Label, &k.Last4, &k.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Get trả về một key của user (không có key rõ).
func (r *Repo) Get(ctx context.Context, userID, id string) (api.APIKey, error) {
	var k api.APIKey
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, provider, label, last4, created_at FROM api_keys
		WHERE user_id = $1 AND id::text = $2`, userID, id).Scan(&k.ID, &k.Provider, &k.Label, &k.Last4, &k.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.APIKey{}, ErrNotFound
	}
	return k, err
}

// Create mã hoá và lưu key. ID sinh trước để AAD gắn đúng user|id.
func (r *Repo) Create(ctx context.Context, userID, provider, label, apiKey string) (api.APIKey, error) {
	id, err := newUUID()
	if err != nil {
		return api.APIKey{}, err
	}
	ct, nonce, err := r.box.Seal([]byte(apiKey), secret.KeyAAD(userID, id))
	if err != nil {
		return api.APIKey{}, err
	}
	k := api.APIKey{ID: id, Provider: provider, Label: label, Last4: last4(apiKey)}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO api_keys (id, user_id, provider, label, ciphertext, nonce, key_version, last4)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING created_at`,
		id, userID, provider, label, ct, nonce, secret.KeyVersion, k.Last4).Scan(&k.CreatedAt)
	if err != nil {
		return api.APIKey{}, err
	}
	return k, nil
}

// Delete xoá key của user; key của người khác coi như không tồn tại.
func (r *Repo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM api_keys WHERE user_id = $1 AND id::text = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func last4(key string) string {
	r := []rune(key)
	if len(r) <= 4 {
		return ""
	}
	return string(r[len(r)-4:])
}

// newUUID sinh UUID v4.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
