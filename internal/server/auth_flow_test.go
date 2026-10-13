package server

import (
	"context"
	"fmt"
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

	a := env.newClient()
	if r := a.do("GET", "/api/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("chưa đăng nhập phải 401: %d", r.status)
	}
	a.signup("an@example.com", testPassword)
	var me api.User
	r := a.do("GET", "/api/me", nil)
	r.decode(t, &me)
	if r.status != 200 || me.Email != "an@example.com" || me.ID == "" {
		t.Fatalf("/api/me: %d %+v", r.status, me)
	}

	// Email trùng (khác hoa thường) bị từ chối.
	if r := env.newClient().do("POST", "/api/auth/signup", map[string]string{
		"email": "AN@example.com", "password": testPassword, "display_name": "x",
	}); r.status != http.StatusConflict {
		t.Fatalf("email trùng phải 409: %d %s", r.status, r.body)
	}

	// Thêm key không cần nhập lại mật khẩu → 201, response chỉ có last4.
	if r := a.do("POST", "/api/keys", map[string]string{"provider": "ollama", "api_key": testLLMKey}); r.status != http.StatusBadRequest {
		t.Fatalf("provider ngoài allowlist phải 400: %d", r.status)
	}
	r = a.do("POST", "/api/keys", map[string]string{"provider": "openrouter", "label": "chính", "api_key": testLLMKey})
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
	b.signup("binh@example.com", testPassword)
	if r := b.do("DELETE", "/api/keys/"+key.ID, nil); r.status != http.StatusNotFound {
		t.Fatalf("B xoá key của A phải 404: %d", r.status)
	}
	if r := b.do("GET", "/api/keys", nil); strings.Contains(string(r.body), key.ID) {
		t.Fatal("B thấy key của A")
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

	for _, secret := range []string{testPassword, testNewPassword, testLLMKey} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("log chứa bí mật %q", secret[:6])
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	env := newTestEnv(t, nil)
	c := env.newClient()
	c.signup("an@example.com", testPassword)
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
	victim.signup("an@example.com", testPassword)
	attacker := env.newClient()
	attacker.ip = "203.0.113.9"
	for i := 0; i < 6; i++ {
		attacker.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": "sai-sai-sai-sai"})
	}
	if r := victim.do("POST", "/api/auth/login", map[string]string{"email": "an@example.com", "password": testPassword}); r.status != 200 {
		t.Fatalf("nạn nhân ở IP khác phải đăng nhập được: %d %s", r.status, r.body)
	}
}

// Đăng ký tự do nhưng mỗi IP chỉ 10 lần/ngày; IP khác không bị ảnh hưởng.
func TestSignupDailyLimitPerIP(t *testing.T) {
	env := newTestEnv(t, func(c *Config) {
		cfg, _ := parseCIDRs("127.0.0.1/32,::1/128")
		c.TrustedProxies = cfg
	})
	signup := func(ip, email string) response {
		c := env.newClient()
		c.ip = ip
		return c.do("POST", "/api/auth/signup", map[string]string{"email": email, "password": testPassword, "display_name": "A"})
	}
	for i := 0; i < 10; i++ {
		if r := signup("198.51.100.7", fmt.Sprintf("u%d@example.com", i)); r.status != http.StatusCreated {
			t.Fatalf("signup %d phải 201: %d %s", i, r.status, r.body)
		}
	}
	r := signup("198.51.100.7", "u10@example.com")
	if r.status != http.StatusTooManyRequests || r.errCode() != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("signup thứ 11 cùng IP phải 429 kèm Retry-After: %d %s", r.status, r.body)
	}
	if r := signup("203.0.113.9", "khac@example.com"); r.status != http.StatusCreated {
		t.Fatalf("IP khác vẫn đăng ký được: %d %s", r.status, r.body)
	}
}

// Trần 20 key mỗi user và 30 lần tạo key mỗi ngày.
func TestKeyCreateLimits(t *testing.T) {
	env := newTestEnv(t, nil)
	c := env.newClient()
	c.signup("an@example.com", testPassword)
	create := func() response {
		return c.do("POST", "/api/keys", map[string]string{"provider": "openrouter", "api_key": testLLMKey})
	}
	var ids []string
	for i := 0; i < 20; i++ {
		r := create()
		var k api.APIKey
		r.decode(t, &k)
		if r.status != http.StatusCreated {
			t.Fatalf("key %d phải 201: %d %s", i, r.status, r.body)
		}
		ids = append(ids, k.ID)
	}
	if r := create(); r.status != http.StatusConflict || r.errCode() != "key_limit" {
		t.Fatalf("key thứ 21 phải 409 key_limit: %d %s", r.status, r.body)
	}
	// Đã tiêu 21 lượt trong ngày: xoá bớt key thì còn đúng 9 lượt nữa.
	for _, id := range ids[:10] {
		if r := c.do("DELETE", "/api/keys/"+id, nil); r.status != http.StatusNoContent {
			t.Fatalf("xoá key: %d", r.status)
		}
	}
	for i := 0; i < 9; i++ {
		if r := create(); r.status != http.StatusCreated {
			t.Fatalf("lượt %d trong ngày phải 201: %d %s", 22+i, r.status, r.body)
		}
	}
	if r := create(); r.status != http.StatusTooManyRequests || r.errCode() != "rate_limited" {
		t.Fatalf("lần tạo thứ 31 trong ngày phải 429: %d %s", r.status, r.body)
	}
	// User khác không bị ảnh hưởng.
	other := env.newClient()
	other.signup("binh@example.com", testPassword)
	if r := other.do("POST", "/api/keys", map[string]string{"provider": "openrouter", "api_key": testLLMKey}); r.status != http.StatusCreated {
		t.Fatalf("user khác phải tạo được key: %d %s", r.status, r.body)
	}
}

// Vòng dọn xoá tài khoản thường quá 30 ngày không có key, truyện hay session;
// tài khoản có bất kỳ thứ nào trong đó, tài khoản mới và admin được giữ.
func TestCleanupAbandonedAccounts(t *testing.T) {
	env := newTestEnv(t, nil)
	ctx := context.Background()
	pool := env.srv.pool
	seq := 0
	newUser := func(age time.Duration, admin bool) string {
		t.Helper()
		seq++
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (email, password_hash, is_admin, created_at)
			VALUES ($1, 'x', $2, now() - $3::interval) RETURNING id::text`,
			fmt.Sprintf("bo-hoang-%d@example.com", seq), admin, age.String()).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	old := abandonedAccountAge + time.Hour
	empty := newUser(old, false)
	fresh := newUser(abandonedAccountAge-time.Hour, false)
	admin := newUser(old, true)
	withKey := newUser(old, false)
	if _, err := pool.Exec(ctx, `INSERT INTO api_keys (user_id, provider, ciphertext, nonce, last4) VALUES ($1::uuid, 'openrouter', 'c', 'n', 'test')`, withKey); err != nil {
		t.Fatal(err)
	}
	withBook := newUser(old, false)
	if _, err := pool.Exec(ctx, `
		INSERT INTO books (user_id, prompt, status, provider, model, budget_usd, advance_mode)
		VALUES ($1::uuid, 'x', 'paused', 'openrouter', 'm', 1, 'auto')`, withBook); err != nil {
		t.Fatal(err)
	}
	withSession := newUser(old, false)
	if _, err := pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2::uuid, now() + interval '1 day')`, []byte("tok-"+withSession), withSession); err != nil {
		t.Fatal(err)
	}
	// Chỉ còn session đã hết hạn: session bị dọn trước nên tài khoản thành bỏ hoang.
	expiredSession := newUser(old, false)
	if _, err := pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2::uuid, now() - interval '1 day')`, []byte("tok-"+expiredSession), expiredSession); err != nil {
		t.Fatal(err)
	}
	logs := captureLogs(t)

	env.srv.cleanup(ctx)

	exists := func(id string) bool {
		var ok bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1::uuid)`, id).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for name, id := range map[string]string{"rỗng quá 30 ngày": empty, "chỉ có session hết hạn": expiredSession} {
		if exists(id) {
			t.Fatalf("tài khoản %s phải bị xoá", name)
		}
	}
	for name, id := range map[string]string{"mới": fresh, "admin": admin, "có key": withKey, "có truyện": withBook, "có session": withSession} {
		if !exists(id) {
			t.Fatalf("tài khoản %s không được xoá", name)
		}
	}
	if !strings.Contains(logs.String(), "count=2") {
		t.Fatalf("phải log số tài khoản bị xoá: %s", logs.String())
	}
}

func TestProdCookieAttributes(t *testing.T) {
	env := newTestEnv(t, func(c *Config) { c.Dev = false })
	r := env.newClient().do("POST", "/api/auth/signup", map[string]string{"email": "a@b.vn", "password": testPassword, "display_name": "A"})
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
	c.signup("a@b.vn", testPassword)
	if _, err := env.srv.pool.Exec(context.Background(), `UPDATE sessions SET last_seen_at = now() - $1::interval`, (auth.SessionIdle + time.Hour).String()); err != nil {
		t.Fatal(err)
	}
	if r := c.do("GET", "/api/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("session quá hạn idle phải 401: %d", r.status)
	}
}
