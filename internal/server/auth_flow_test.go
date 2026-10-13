package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/auth"
)

const (
	testPassword    = "mat-khau-rat-bi-mat-123"
	testNewPassword = "mat-khau-moi-cung-bi-mat-456"
	testLLMKey      = "sk-or-v1-THIS-IS-A-SECRET-KEY-0123456789"
)

func TestAuthAndKeysFlow(t *testing.T) {
	logs := captureLogs(t)
	env := newTestEnv(t, nil)
	invite := env.invite(2)

	a := env.newClient()
	if r := a.do("GET", "/api/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("chưa đăng nhập phải 401: %d", r.status)
	}
	a.signup(invite, "an@example.com", testPassword)
	var me api.User
	r := a.do("GET", "/api/me", nil)
	r.decode(t, &me)
	if r.status != 200 || me.Email != "an@example.com" || me.ID == "" {
		t.Fatalf("/api/me: %d %+v", r.status, me)
	}

	// Email trùng (khác hoa thường) bị từ chối.
	if r := env.newClient().do("POST", "/api/auth/signup", map[string]string{
		"email": "AN@example.com", "password": testPassword, "display_name": "x", "invite_code": invite,
	}); r.status != http.StatusConflict {
		t.Fatalf("email trùng phải 409: %d %s", r.status, r.body)
	}

	// Thêm key: sai mật khẩu → 403; đúng → 201, response chỉ có last4.
	if r := a.do("POST", "/api/keys", map[string]string{"provider": "openrouter", "label": "chính", "api_key": testLLMKey, "current_password": "sai-mat-khau"}); r.status != http.StatusForbidden {
		t.Fatalf("thêm key với mật khẩu sai phải 403: %d %s", r.status, r.body)
	}
	if r := a.do("POST", "/api/keys", map[string]string{"provider": "ollama", "api_key": testLLMKey, "current_password": testPassword}); r.status != http.StatusBadRequest {
		t.Fatalf("provider ngoài allowlist phải 400: %d", r.status)
	}
	r = a.do("POST", "/api/keys", map[string]string{"provider": "openrouter", "label": "chính", "api_key": testLLMKey, "current_password": testPassword})
	var key api.APIKey
	r.decode(t, &key)
	if r.status != http.StatusCreated || key.Last4 != "6789" || strings.Contains(string(r.body), testLLMKey) {
		t.Fatalf("tạo key: %d %s", r.status, r.body)
	}
	r = a.do("GET", "/api/keys", nil)
	if r.status != 200 || strings.Contains(string(r.body), testLLMKey) || !strings.Contains(string(r.body), key.ID) {
		t.Fatalf("list key: %d %s", r.status, r.body)
	}

	// Trong DB chỉ có ciphertext.
	var ct []byte
	if err := env.srv.pool.QueryRow(context.Background(), `SELECT ciphertext FROM api_keys WHERE id = $1`, key.ID).Scan(&ct); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ct), testLLMKey) || strings.Contains(string(ct), "THIS-IS-A-SECRET") {
		t.Fatal("key lưu dạng rõ trong DB")
	}

	// /api/providers chỉ có model có giá.
	var providers []api.ProviderInfo
	r = a.do("GET", "/api/providers", nil)
	r.decode(t, &providers)
	if r.status != 200 || len(providers) == 0 || len(providers[0].Models) == 0 {
		t.Fatalf("providers: %d %s", r.status, r.body)
	}

	// User B không thấy, không xoá được key của A.
	b := env.newClient()
	b.signup(invite, "binh@example.com", testPassword)
	if r := b.do("DELETE", "/api/keys/"+key.ID, nil); r.status != http.StatusNotFound {
		t.Fatalf("B xoá key của A phải 404: %d", r.status)
	}
	if r := b.do("GET", "/api/keys", nil); strings.Contains(string(r.body), key.ID) {
		t.Fatal("B thấy key của A")
	}
	// Invite hết lượt (2 lượt đã dùng).
	if r := env.newClient().do("POST", "/api/auth/signup", map[string]string{
		"email": "chi@example.com", "password": testPassword, "display_name": "Chi", "invite_code": invite,
	}); r.status != http.StatusForbidden || r.errCode() != "invite_invalid" {
		t.Fatalf("invite hết lượt phải 403: %d %s", r.status, r.body)
	}

	// Phiên thứ hai của A để kiểm đổi mật khẩu huỷ phiên khác.
	a2 := env.newClient()
	if r := a2.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": testPassword}); r.status != 200 {
		t.Fatalf("login: %d %s", r.status, r.body)
	}
	if r := a.do("POST", "/api/me/password", map[string]string{"current_password": "sai-mat-khau-cu", "new_password": testNewPassword}); r.status != http.StatusForbidden {
		t.Fatalf("sai mật khẩu cũ phải 403: %d", r.status)
	}
	if r := a.do("POST", "/api/me/password", map[string]string{"current_password": testPassword, "new_password": testNewPassword}); r.status != http.StatusNoContent {
		t.Fatalf("đổi mật khẩu: %d %s", r.status, r.body)
	}
	if r := a.do("GET", "/api/me", nil); r.status != 200 {
		t.Fatalf("phiên hiện tại phải còn: %d", r.status)
	}
	if r := a2.do("GET", "/api/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("phiên khác phải bị huỷ: %d", r.status)
	}
	if r := a2.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": testPassword}); r.status != http.StatusUnauthorized {
		t.Fatalf("mật khẩu cũ không còn dùng được: %d", r.status)
	}

	// Xoá key của chính mình; đăng xuất → 401.
	if r := a.do("DELETE", "/api/keys/"+key.ID, nil); r.status != http.StatusNoContent {
		t.Fatalf("xoá key: %d", r.status)
	}
	if r := a.do("POST", "/api/auth/logout", nil); r.status != http.StatusNoContent {
		t.Fatalf("logout: %d", r.status)
	}
	if r := a.do("GET", "/api/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("sau logout phải 401: %d", r.status)
	}

	for _, secret := range []string{testPassword, testNewPassword, testLLMKey, invite} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("log chứa bí mật %q", secret[:6])
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	env := newTestEnv(t, nil)
	c := env.newClient()
	c.signup(env.invite(1), "an@example.com", testPassword)
	attacker := env.newClient()
	for i := 0; i < 5; i++ {
		if r := attacker.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": "doan-mo-sai-" + string(rune('a'+i))}); r.status != http.StatusUnauthorized || r.errCode() != "invalid_credentials" {
			t.Fatalf("lần %d: %d %s", i, r.status, r.body)
		}
	}
	r := attacker.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": testPassword})
	if r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") == "" {
		t.Fatalf("lần 6 phải 429 kèm Retry-After: %d %v", r.status, r.header)
	}
	// Cùng IP, email khác vẫn đăng nhập được (không khoá chéo).
	if r := attacker.do("POST", "/api/auth/login", map[string]string{"email": "khac@example.com", "password": "x"}); r.status != http.StatusUnauthorized {
		t.Fatalf("email khác cùng IP phải chỉ 401: %d", r.status)
	}
}

// Kẻ tấn công ở IP khác không khoá được tài khoản của nạn nhân.
func TestLoginRateLimitPerIPWhenBehindProxy(t *testing.T) {
	env := newTestEnv(t, func(c *Config) {
		cfg, _ := parseCIDRs("127.0.0.1/32,::1/128")
		c.TrustedProxies = cfg
	})
	victim := env.newClient()
	victim.ip = "198.51.100.7"
	victim.signup(env.invite(1), "an@example.com", testPassword)
	attacker := env.newClient()
	attacker.ip = "203.0.113.9"
	for i := 0; i < 6; i++ {
		attacker.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": "sai-sai-sai-sai"})
	}
	if r := victim.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": testPassword}); r.status != 200 {
		t.Fatalf("nạn nhân ở IP khác phải đăng nhập được: %d %s", r.status, r.body)
	}
}

func TestExpiredInviteRejected(t *testing.T) {
	env := newTestEnv(t, nil)
	code := env.invite(5)
	if _, err := env.srv.pool.Exec(context.Background(), `UPDATE invite_codes SET expires_at = now() - interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	r := env.newClient().do("POST", "/api/auth/signup", map[string]string{"email": "a@b.vn", "password": testPassword, "display_name": "A", "invite_code": code})
	if r.status != http.StatusForbidden {
		t.Fatalf("invite hết hạn phải 403: %d", r.status)
	}
}

func TestProdCookieAttributes(t *testing.T) {
	env := newTestEnv(t, func(c *Config) { c.Dev = false })
	r := env.newClient().do("POST", "/api/auth/signup", map[string]string{"email": "a@b.vn", "password": testPassword, "display_name": "A", "invite_code": env.invite(1)})
	if r.status != http.StatusCreated {
		t.Fatalf("signup: %d %s", r.status, r.body)
	}
	sc := r.header.Get("Set-Cookie")
	for _, want := range []string{"__Host-ainovel_session=", "Path=/", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(sc, want) {
			t.Fatalf("cookie thiếu %q: %s", want, sc)
		}
	}
	if strings.Contains(sc, "Domain=") {
		t.Fatalf("cookie __Host- không được có Domain: %s", sc)
	}
}

func TestSessionIdleExpiry(t *testing.T) {
	env := newTestEnv(t, nil)
	c := env.newClient()
	c.signup(env.invite(1), "a@b.vn", testPassword)
	if _, err := env.srv.pool.Exec(context.Background(), `UPDATE sessions SET last_seen_at = now() - $1::interval`, (auth.SessionIdle + time.Hour).String()); err != nil {
		t.Fatal(err)
	}
	if r := c.do("GET", "/api/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("session quá hạn idle phải 401: %d", r.status)
	}
}
