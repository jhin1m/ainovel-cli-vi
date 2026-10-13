// Package admin chứa thao tác quản trị chạy từ dòng lệnh (không có API web).
package admin

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/auth"
)

// CreateInvite tạo một mã mời dùng được uses lần, hết hạn sau ttl (0 = không hạn).
// Chỉ lưu sha256 của mã; mã rõ trả về một lần để in ra cho quản trị viên.
func CreateInvite(ctx context.Context, pool *pgxpool.Pool, uses int, ttl time.Duration) (string, error) {
	if uses <= 0 {
		return "", fmt.Errorf("số lượt dùng phải > 0")
	}
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	raw := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]) // 16 ký tự
	code := raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16]
	var expires *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expires = &t
	}
	_, err := pool.Exec(ctx, `INSERT INTO invite_codes (code_hash, max_uses, expires_at) VALUES ($1, $2, $3)`,
		auth.HashInviteCode(code), uses, expires)
	if err != nil {
		return "", err
	}
	return code, nil
}

// ParseTTL nhận dạng "7d", "12h", "30m" hoặc "0" (không hết hạn).
func ParseTTL(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("thời hạn %q không hợp lệ", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("thời hạn %q không hợp lệ (vd. 7d, 12h)", s)
	}
	return d, nil
}
