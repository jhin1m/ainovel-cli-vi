package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/httpx"
)

// Tên cookie session. Tiền tố __Host- buộc Secure, Path=/ và không có Domain —
// không dùng được trên http://localhost nên ở dev dùng tên thường.
const (
	cookieNameProd = "__Host-ainovel_session"
	cookieNameDev  = "ainovel_session"
)

const maxDisplayNameLen = 60

// Service gom các handler tài khoản.
type Service struct {
	pool     *pgxpool.Pool
	sessions *Sessions
	dev      bool

	ipLimiter    *Limiter // mọi request auth theo IP
	loginLimiter *Limiter // đăng nhập theo (IP, email)
	userLimiter  *Limiter // đổi mật khẩu / nhập lại mật khẩu theo user
}

// NewService tạo service auth. dev=true dùng cookie không Secure cho http://localhost.
func NewService(pool *pgxpool.Pool, dev bool) *Service {
	return &Service{
		pool:         pool,
		sessions:     NewSessions(pool),
		dev:          dev,
		ipLimiter:    NewLimiter(30),
		loginLimiter: NewLimiter(5),
		userLimiter:  NewLimiter(5),
	}
}

// Sessions trả về store session (dọn định kỳ).
func (s *Service) Sessions() *Sessions { return s.sessions }

// SweepLimiters dọn rate limiter; gọi định kỳ.
func (s *Service) SweepLimiters() {
	s.ipLimiter.Sweep()
	s.loginLimiter.Sweep()
	s.userLimiter.Sweep()
}

func (s *Service) cookieName() string {
	if s.dev {
		return cookieNameDev
	}
	return cookieNameProd
}

// Register gắn route tài khoản vào mux API.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/signup", s.handleSignup)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.Handle("GET /api/me", s.RequireUser(http.HandlerFunc(s.handleMe)))
	mux.Handle("POST /api/me/password", s.RequireUser(http.HandlerFunc(s.handleChangePassword)))
}

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxToken
)

// UserFrom trả về user đã đăng nhập (RequireUser đặt vào context).
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxUser).(User)
	return u, ok
}

// RequireUser chặn request chưa đăng nhập bằng 401 JSON.
func (s *Service) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.cookieName())
		if err != nil {
			httpx.WriteErr(w, r, httpx.ErrUnauthorized)
			return
		}
		u, err := s.sessions.Lookup(r.Context(), c.Value)
		if err != nil {
			if !errors.Is(err, ErrNoSession) {
				httpx.WriteErr(w, r, err)
				return
			}
			s.clearCookie(w)
			httpx.WriteErr(w, r, httpx.ErrUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, u)
		ctx = context.WithValue(ctx, ctxToken, c.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// VerifyUserPassword kiểm mật khẩu hiện tại của user (dùng khi thêm API key).
// Có rate limit theo user để phiên bị chiếm không dò được mật khẩu.
func (s *Service) VerifyUserPassword(ctx context.Context, userID, password string) error {
	if ok, wait := s.userLimiter.Allow(userID); !ok {
		return tooMany(wait)
	}
	var hash string
	if err := s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return err
	}
	ok, err := VerifyPassword(ctx, password, hash)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NewError(http.StatusForbidden, "wrong_password", "Mật khẩu hiện tại không đúng")
	}
	return nil
}

func (s *Service) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionAbsolute / time.Second),
		HttpOnly: true,
		Secure:   !s.dev,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !s.dev,
		SameSite: http.SameSiteLaxMode,
	})
}

func tooMany(wait time.Duration) error { return httpx.TooManyRequests(wait) }

// writeErr ghi lỗi chuẩn (gồm Retry-After cho 429).
func writeErr(w http.ResponseWriter, r *http.Request, err error) { httpx.WriteErr(w, r, err) }

func (s *Service) allowIP(r *http.Request) error {
	if ok, wait := s.ipLimiter.Allow(httpx.ClientIPFrom(r.Context()).String()); !ok {
		return tooMany(wait)
	}
	return nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	if len(email) > 254 {
		return "", httpx.NewError(http.StatusBadRequest, "invalid_email", "Email không hợp lệ")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email, "@") {
		return "", httpx.NewError(http.StatusBadRequest, "invalid_email", "Email không hợp lệ")
	}
	return strings.ToLower(email), nil
}

// NormalizeInviteCode chuẩn hoá code người dùng gõ (bỏ gạch, khoảng trắng, hoa thường).
func NormalizeInviteCode(code string) string {
	code = strings.ToUpper(code)
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, code)
}

// HashInviteCode băm code đã chuẩn hoá; DB chỉ giữ giá trị băm.
func HashInviteCode(code string) []byte {
	sum := sha256.Sum256([]byte(NormalizeInviteCode(code)))
	return sum[:]
}

func (s *Service) handleSignup(w http.ResponseWriter, r *http.Request) {
	if err := s.allowIP(r); err != nil {
		writeErr(w, r, err)
		return
	}
	var req api.SignupRequest
	if err := httpx.Decode(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := ValidatePassword(req.Password); err != nil {
		writeErr(w, r, httpx.NewError(http.StatusBadRequest, "weak_password", err.Error()))
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" || utf8.RuneCountInString(name) > maxDisplayNameLen {
		writeErr(w, r, httpx.NewError(http.StatusBadRequest, "invalid_display_name", "Tên hiển thị phải có từ 1 đến 60 ký tự"))
		return
	}
	if strings.TrimSpace(req.InviteCode) == "" {
		writeErr(w, r, httpx.NewError(http.StatusBadRequest, "invite_required", "Cần mã mời để đăng ký"))
		return
	}
	hash, err := HashPassword(r.Context(), req.Password)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	var u User
	err = pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `
			UPDATE invite_codes SET used_count = used_count + 1
			WHERE code_hash = $1 AND used_count < max_uses AND (expires_at IS NULL OR expires_at > now())`,
			HashInviteCode(req.InviteCode))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.NewError(http.StatusForbidden, "invite_invalid", "Mã mời không hợp lệ, đã hết lượt hoặc đã hết hạn")
		}
		err = tx.QueryRow(r.Context(), `
			INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)
			RETURNING id::text, email::text, display_name, is_admin, created_at`,
			email, hash, name).Scan(&u.ID, &u.Email, &u.DisplayName, &u.IsAdmin, &u.CreatedAt)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return httpx.NewError(http.StatusConflict, "email_taken", "Email này đã được đăng ký")
		}
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	token, err := s.sessions.Create(r.Context(), u.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	s.setCookie(w, token)
	httpx.WriteJSON(w, http.StatusCreated, u)
}

var errBadLogin = httpx.NewError(http.StatusUnauthorized, "invalid_credentials", "Email hoặc mật khẩu không đúng")

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := s.allowIP(r); err != nil {
		writeErr(w, r, err)
		return
	}
	var req api.LoginRequest
	if err := httpx.Decode(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	// Theo (IP, email): kẻ tấn công ở IP khác không khoá được tài khoản người khác.
	if ok, wait := s.loginLimiter.Allow(httpx.ClientIPFrom(r.Context()).String() + "|" + email); !ok {
		writeErr(w, r, tooMany(wait))
		return
	}
	if req.Password == "" || utf8.RuneCountInString(req.Password) > MaxPasswordLen {
		writeErr(w, r, errBadLogin)
		return
	}

	var u User
	var hash string
	err := s.pool.QueryRow(r.Context(), `
		SELECT id::text, email::text, display_name, is_admin, created_at, password_hash
		FROM users WHERE email = $1`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &u.IsAdmin, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		burnPasswordCheck(r.Context(), req.Password)
		writeErr(w, r, errBadLogin)
		return
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	ok, err := VerifyPassword(r.Context(), req.Password, hash)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if !ok {
		writeErr(w, r, errBadLogin)
		return
	}
	// Xoay session: bỏ session cũ trên trình duyệt này (nếu có) trước khi cấp mới.
	if c, err := r.Cookie(s.cookieName()); err == nil && c.Value != "" {
		_ = s.sessions.Delete(r.Context(), c.Value)
	}
	token, err := s.sessions.Create(r.Context(), u.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	s.setCookie(w, token)
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cookieName()); err == nil && c.Value != "" {
		if err := s.sessions.Delete(r.Context(), c.Value); err != nil {
			writeErr(w, r, err)
			return
		}
	}
	s.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (s *Service) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	token, _ := r.Context().Value(ctxToken).(string)
	var req api.ChangePasswordRequest
	if err := httpx.Decode(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		writeErr(w, r, httpx.NewError(http.StatusBadRequest, "weak_password", err.Error()))
		return
	}
	if err := s.VerifyUserPassword(r.Context(), u.ID, req.CurrentPassword); err != nil {
		writeErr(w, r, err)
		return
	}
	hash, err := HashPassword(r.Context(), req.NewPassword)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, u.ID, hash); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := s.sessions.DeleteOthers(r.Context(), u.ID, token); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
