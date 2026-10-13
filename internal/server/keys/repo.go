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

// ErrKeyLimit: user đã có đủ số key tối đa.
var ErrKeyLimit = errors.New("đã đạt số API key tối đa")

// Repo lưu API key đã mã hoá. Chỉ hai hàm trả key rõ: DecryptForBook (cho worker)
// và DecryptOwned (cho kiểm key); cả hai đều buộc chủ sở hữu.
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
	defer rows.Close()
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

// Create mã hoá và lưu key, từ chối bằng ErrKeyLimit khi user đã có maxKeys key.
// Khoá dòng user trong transaction để hai request song song không cùng lọt trần.
// ID sinh trước để AAD gắn đúng user|id.
func (r *Repo) Create(ctx context.Context, userID, provider, label, apiKey string, maxKeys int) (api.APIKey, error) {
	id, err := newUUID()
	if err != nil {
		return api.APIKey{}, err
	}
	ct, nonce, err := r.box.Seal([]byte(apiKey), secret.KeyAAD(userID, id))
	if err != nil {
		return api.APIKey{}, err
	}
	k := api.APIKey{ID: id, Provider: provider, Label: label, Last4: last4(apiKey)}
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR NO KEY UPDATE`, userID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE user_id = $1`, userID).Scan(&n); err != nil {
			return err
		}
		if n >= maxKeys {
			return ErrKeyLimit
		}
		return tx.QueryRow(ctx, `
			INSERT INTO api_keys (id, user_id, provider, label, ciphertext, nonce, key_version, last4)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING created_at`,
			id, userID, provider, label, ct, nonce, secret.KeyVersion, k.Last4).Scan(&k.CreatedAt)
	})
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

// Có hai lối lấy key rõ, cả hai đều buộc chủ sở hữu: DecryptForBook (đi qua
// truyện) và DecryptOwned (theo user của phiên).

// DecryptForBook đi qua truyện và FK kép (key phải thuộc đúng chủ truyện),
// AAD = user_id|api_key_id. Trả về provider của key.
func (r *Repo) DecryptForBook(ctx context.Context, bookID string) (string, []byte, error) {
	var provider, userID, keyID string
	var ct, nonce []byte
	err := r.pool.QueryRow(ctx, `
		SELECT k.provider, k.ciphertext, k.nonce, b.user_id::text, k.id::text
		FROM books b JOIN api_keys k ON k.user_id = b.user_id AND k.id = b.api_key_id
		WHERE b.id::text = $1`, bookID).Scan(&provider, &ct, &nonce, &userID, &keyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	key, err := r.box.Open(ct, nonce, secret.KeyAAD(userID, keyID))
	if err != nil {
		return "", nil, err
	}
	return provider, key, nil
}

// DecryptOwned giải mã key keyID của chính userID (user của phiên). Key của
// người khác trả ErrNotFound như key không tồn tại. AAD dựng từ userID của phiên,
// nên dù truy vấn có sai thì bản mã của user khác cũng không mở được.
func (r *Repo) DecryptOwned(ctx context.Context, userID, keyID string) (string, []byte, error) {
	var provider string
	var ct, nonce []byte
	err := r.pool.QueryRow(ctx, `
		SELECT provider, ciphertext, nonce FROM api_keys
		WHERE user_id = $1 AND id::text = $2`, userID, keyID).Scan(&provider, &ct, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	key, err := r.box.Open(ct, nonce, secret.KeyAAD(userID, keyID))
	if err != nil {
		return "", nil, err
	}
	return provider, key, nil
}
