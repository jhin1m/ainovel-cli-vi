package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/api"
)

// Thời hạn session: hết hạn khi không dùng 7 ngày, hoặc tuyệt đối sau 30 ngày.
const (
	SessionIdle     = 7 * 24 * time.Hour
	SessionAbsolute = 30 * 24 * time.Hour
	// lastSeenEvery: cập nhật last_seen_at tối đa một lần mỗi phút.
	lastSeenEvery = time.Minute
)

// ErrNoSession: token không tồn tại hoặc đã hết hạn.
var ErrNoSession = errors.New("session không hợp lệ")

// User là người dùng đã đăng nhập (wire type dùng chung với SPA).
type User = api.User

// Sessions lưu session trong Postgres.
type Sessions struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewSessions tạo store session.
func NewSessions(pool *pgxpool.Pool) *Sessions {
	return &Sessions{pool: pool, now: time.Now}
}

// hashToken băm token; DB chỉ giữ giá trị băm.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Create tạo session mới cho user, trả về token gửi cho client.
func (s *Sessions) Create(ctx context.Context, userID string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	now := s.now()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, last_seen_at, expires_at) VALUES ($1, $2, $3, $3, $4)`,
		hashToken(token), userID, now, now.Add(SessionAbsolute))
	if err != nil {
		return "", err
	}
	return token, nil
}

// Lookup trả về user của token còn hiệu lực, và gia hạn last_seen_at (thưa).
func (s *Sessions) Lookup(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNoSession
	}
	now := s.now()
	h := hashToken(token)
	var u User
	var lastSeen time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email::text, u.display_name, u.is_admin, u.created_at, s.last_seen_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > $2 AND s.last_seen_at > $3`,
		h, now, now.Add(-SessionIdle)).Scan(&u.ID, &u.Email, &u.DisplayName, &u.IsAdmin, &u.CreatedAt, &lastSeen)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, err
	}
	if now.Sub(lastSeen) >= lastSeenEvery {
		_, _ = s.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = $2 WHERE token_hash = $1`, h, now)
	}
	return u, nil
}

// Delete xoá một session (đăng xuất).
func (s *Sessions) Delete(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

// DeleteOthers xoá mọi session của user trừ token đang dùng (sau khi đổi mật khẩu).
func (s *Sessions) DeleteOthers(ctx context.Context, userID, keepToken string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`, userID, hashToken(keepToken))
	return err
}

// PurgeExpired dọn session đã hết hạn; gọi định kỳ.
func (s *Sessions) PurgeExpired(ctx context.Context) error {
	now := s.now()
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= $1 OR last_seen_at <= $2`, now, now.Add(-SessionIdle))
	return err
}
