package auth

import (
	"context"
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

func TestInviteCodeNormalization(t *testing.T) {
	if string(HashInviteCode("abcd-efgh")) != string(HashInviteCode(" ABCD EFGH ")) {
		t.Fatal("mã mời phải chuẩn hoá hoa thường/gạch/khoảng trắng")
	}
}
