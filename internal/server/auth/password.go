// Package auth xử lý tài khoản: mật khẩu argon2id, session phía server, đăng ký
// bằng invite code, rate limit đăng nhập.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Tham số argon2id: mức tối thiểu OWASP (m=19 MiB, t=2, p=1).
const (
	argonMemoryKiB = 19 * 1024
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16
)

// Độ dài mật khẩu cho phép (tính theo ký tự).
const (
	MinPasswordLen = 10
	MaxPasswordLen = 128
)

// hashSlots giới hạn số lần băm đồng thời: mỗi lần tốn ~19 MiB, không giới hạn
// thì một loạt request đăng nhập có thể ăn hết RAM.
var hashSlots = make(chan struct{}, 4)

var errBadHash = errors.New("chuỗi băm mật khẩu không hợp lệ")

// ValidatePassword kiểm tra độ dài mật khẩu mới.
func ValidatePassword(pw string) error {
	n := utf8.RuneCountInString(pw)
	if n < MinPasswordLen || n > MaxPasswordLen {
		return fmt.Errorf("Mật khẩu phải dài từ %d đến %d ký tự", MinPasswordLen, MaxPasswordLen)
	}
	return nil
}

func derive(ctx context.Context, pw string, salt []byte, mem, iters uint32, threads uint8, keyLen uint32) ([]byte, error) {
	select {
	case hashSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-hashSlots }()
	return argon2.IDKey([]byte(pw), salt, iters, mem, threads, keyLen), nil
}

// HashPassword băm mật khẩu thành chuỗi PHC:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>.
func HashPassword(ctx context.Context, pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := derive(ctx, pw, salt, argonMemoryKiB, argonTime, argonThreads, argonKeyLen)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonThreads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword so mật khẩu với chuỗi PHC trong thời gian hằng.
func VerifyPassword(ctx context.Context, pw, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	var mem, iters uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iters, &threads); err != nil || mem == 0 || iters == 0 || threads == 0 {
		return false, errBadHash
	}
	// Chặn tham số phi lý trong DB (vd. bị sửa tay) làm cạn bộ nhớ.
	if mem > 256*1024 || iters > 10 {
		return false, errBadHash
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errBadHash
	}
	got, err := derive(ctx, pw, salt, mem, iters, threads, uint32(len(want)))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash dùng khi email không tồn tại: vẫn chạy một lần băm để thời gian phản
// hồi không tiết lộ email nào đã đăng ký. Tính lười để CLI/TUI không tốn công.
var dummyHash = sync.OnceValue(func() string {
	h, err := HashPassword(context.Background(), "ainovel-dummy-password")
	if err != nil {
		panic(err)
	}
	return h
})

// burnPasswordCheck chạy một lần băm giả.
func burnPasswordCheck(ctx context.Context, pw string) {
	_, _ = VerifyPassword(ctx, pw, dummyHash())
}
