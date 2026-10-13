package auth

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashVerify(t *testing.T) {
	ctx := context.Background()
	h, err := HashPassword(ctx, "mật-khẩu-dài-đủ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("định dạng PHC sai: %s", h)
	}
	if ok, err := VerifyPassword(ctx, "mật-khẩu-dài-đủ", h); err != nil || !ok {
		t.Fatalf("mật khẩu đúng phải qua: %v %v", ok, err)
	}
	if ok, _ := VerifyPassword(ctx, "mật-khẩu-sai!!", h); ok {
		t.Fatal("mật khẩu sai không được qua")
	}
	h2, _ := HashPassword(ctx, "mật-khẩu-dài-đủ")
	if h == h2 {
		t.Fatal("salt phải ngẫu nhiên")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	ctx := context.Background()
	bad := []string{
		"",
		"plain",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=999999999,t=2,p=1$c2FsdA$aGFzaA", // tham số phi lý: không được cạn RAM
		"$argon2id$v=19$m=19456,t=2,p=1$!!$aGFzaA",
	}
	for _, h := range bad {
		if ok, err := VerifyPassword(ctx, "x", h); ok || err == nil {
			t.Fatalf("%q phải lỗi", h)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if ValidatePassword("ngắn") == nil {
		t.Fatal("mật khẩu ngắn phải lỗi")
	}
	if ValidatePassword(strings.Repeat("a", 129)) == nil {
		t.Fatal("mật khẩu quá dài phải lỗi")
	}
	if err := ValidatePassword("đủ-mười-ký-tự"); err != nil {
		t.Fatal(err)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(5)
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow("ip|a@b.c"); !ok {
			t.Fatalf("lượt %d phải qua", i)
		}
	}
	ok, wait := l.Allow("ip|a@b.c")
	if ok || wait <= 0 {
		t.Fatalf("lượt 6 phải bị chặn kèm thời gian chờ: %v %v", ok, wait)
	}
	if ok, _ := l.Allow("ip|khac@b.c"); !ok {
		t.Fatal("khoá khác không bị ảnh hưởng")
	}
	now = now.Add(13 * time.Second) // 5/phút → 12s hồi một lượt
	if ok, _ := l.Allow("ip|a@b.c"); !ok {
		t.Fatal("sau khi hồi phải qua")
	}
	now = now.Add(time.Hour)
	l.Sweep()
	if len(l.entries) != 0 {
		t.Fatalf("Sweep không dọn: %d", len(l.entries))
	}
}

func TestLimiterPerDay(t *testing.T) {
	l := NewLimiterPer(10, 24*time.Hour)
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }
	for i := 0; i < 10; i++ {
		if ok, _ := l.Allow("ip"); !ok {
			t.Fatalf("lượt %d phải qua", i)
		}
	}
	if ok, wait := l.Allow("ip"); ok || wait < time.Hour {
		t.Fatalf("lượt 11 phải bị chặn, chờ hàng giờ: %v %v", ok, wait)
	}
	// Nhàn rỗi 11 phút: limiter theo phút sẽ dọn entry, limiter theo ngày thì không.
	now = now.Add(11 * time.Minute)
	l.Sweep()
	if len(l.entries) != 1 {
		t.Fatalf("entry theo ngày không được bị dọn sau 11 phút: %d", len(l.entries))
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Fatal("sau 11 phút vẫn phải bị chặn")
	}
	now = now.Add(25 * time.Hour)
	l.Sweep()
	if len(l.entries) != 0 {
		t.Fatalf("nhàn rỗi quá một ngày phải bị dọn: %d", len(l.entries))
	}
}

// Map đầy: limiter theo phút từ chối khoá mới; limiter theo ngày bỏ khoá cũ nhất
// để khoá mới vẫn qua, còn khoá vừa dùng giữ nguyên số lượt đã tiêu.
func TestLimiterFullMap(t *testing.T) {
	now := time.Unix(1000, 0)
	minute := NewLimiter(5)
	minute.now = func() time.Time { return now }
	minute.max = 2
	minute.Allow("a")
	minute.Allow("b")
	if ok, _ := minute.Allow("c"); ok {
		t.Fatal("limiter theo phút đầy map phải từ chối khoá mới")
	}

	day := NewLimiterPer(1, 24*time.Hour)
	day.now = func() time.Time { return now }
	day.max = 2
	day.Allow("cu")
	now = now.Add(time.Minute)
	day.Allow("moi")
	now = now.Add(time.Minute)
	if ok, _ := day.Allow("ip-moi"); !ok {
		t.Fatal("limiter theo ngày đầy map phải cho khoá mới qua")
	}
	if _, kept := day.entries[string(sha256Key("moi"))]; !kept {
		t.Fatal("khoá vừa dùng không được bị bỏ")
	}
	if ok, _ := day.Allow("moi"); ok {
		t.Fatal("khoá vừa dùng vẫn phải giữ số lượt đã tiêu")
	}
}

func sha256Key(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}
