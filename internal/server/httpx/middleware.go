package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"mime"
	"net/http"
	"net/netip"
	"runtime/debug"
	"time"
)

// Middleware bọc một handler.
type Middleware func(http.Handler) http.Handler

// Chain áp middleware theo thứ tự: mws[0] là lớp ngoài cùng.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxLogger
	ctxClientIP
)

// RequestIDFrom trả về request id gắn vào context.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxRequestID).(string)
	return id
}

// LoggerFrom trả về logger có sẵn request id; thiếu thì dùng logger mặc định.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxLogger).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// ClientIPFrom trả về IP client đã được RequestContext xác định.
func ClientIPFrom(ctx context.Context) netip.Addr {
	a, _ := ctx.Value(ctxClientIP).(netip.Addr)
	return a
}

// RequestContext gắn request id (header X-Request-ID), IP client và logger vào context.
func RequestContext(trusted []netip.Prefix) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var b [8]byte
			_, _ = rand.Read(b[:])
			id := hex.EncodeToString(b[:])
			ip := ClientIP(r, trusted)
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), ctxRequestID, id)
			ctx = context.WithValue(ctx, ctxClientIP, ip)
			ctx = context.WithValue(ctx, ctxLogger, slog.Default().With("request_id", id))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// statusRecorder ghi lại status để log. Giữ Unwrap để ResponseController (SSE)
// vẫn chạm được Flush/SetWriteDeadline của writer gốc.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status, s.wrote = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status, s.wrote = http.StatusOK, true
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Recover bắt panic trong handler, trả 500 JSON thay vì cắt kết nối.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				LoggerFrom(r.Context()).Error("panic trong handler", "panic", v, "stack", string(debug.Stack()))
				if !rec.wrote {
					WriteError(rec, ErrInternal.Status, ErrInternal.Code, ErrInternal.Message)
				}
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

// AccessLog ghi một dòng log mỗi request (không ghi query string hay body).
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		LoggerFrom(r.Context()).Info("http",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(), "ip", ClientIPFrom(r.Context()).String())
	})
}

// SecurityHeaders đặt các header an toàn chung cho mọi response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// MaxBytes giới hạn kích thước body.
func MaxBytes(n int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > n {
				WriteError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Dữ liệu gửi lên quá lớn")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// RequireJSON bắt buộc Content-Type application/json cho mọi request thay đổi dữ
// liệu. Form HTML cross-site không đặt được header này nếu không qua preflight CORS,
// nên đây là lớp CSRF thứ hai bên cạnh CrossOriginProtection.
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutation(r.Method) {
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Yêu cầu phải gửi dạng JSON")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// CrossOrigin chặn request thay đổi dữ liệu đến từ origin khác (CSRF) bằng
// http.CrossOriginProtection; lỗi trả JSON thống nhất.
func CrossOrigin(trustedOrigins ...string) (Middleware, error) {
	cop := http.NewCrossOriginProtection()
	for _, o := range trustedOrigins {
		if o == "" {
			continue
		}
		if err := cop.AddTrustedOrigin(o); err != nil {
			return nil, err
		}
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusForbidden, "cross_origin", "Yêu cầu từ trang khác bị chặn")
	}))
	return cop.Handler, nil
}

// Timeout bọc handler JSON thường bằng http.TimeoutHandler. Không dùng cho SSE
// hay tải file (các route đó tự quản deadline).
func Timeout(d time.Duration) Middleware {
	body := `{"error":{"code":"timeout","message":"Máy chủ xử lý quá lâu, vui lòng thử lại"}}`
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, body)
	}
}
