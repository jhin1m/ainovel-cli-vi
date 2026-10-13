package keys

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/voocel/ainovel-cli/internal/entry/worker"
	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/auth"
	"github.com/voocel/ainovel-cli/internal/server/httpx"
)

const (
	maxLabelLen  = 60
	minAPIKeyLen = 8
	maxAPIKeyLen = 512

	// maxKeysPerUser và createsPerDay chặn lạm dụng khi thêm key không cần
	// nhập lại mật khẩu: mỗi key là một bản mã lưu lâu dài trong DB.
	maxKeysPerUser = 20
	createsPerDay  = 30
)

// InUseChecker báo key còn đang được một truyện đang chạy dùng (nối ở phase books).
type InUseChecker func(r *http.Request, userID, keyID string) (bool, error)

// Handlers phục vụ /api/keys và /api/providers.
type Handlers struct {
	repo    *Repo
	catalog *Catalog
	auth    *auth.Service
	inUse   InUseChecker

	createLimiter *auth.Limiter // tạo key theo user, cửa sổ một ngày
	check         *keyChecker   // POST /api/keys/test
}

// NewHandlers tạo handlers. inUse có thể nil.
func NewHandlers(repo *Repo, catalog *Catalog, authSvc *auth.Service, inUse InUseChecker) *Handlers {
	return &Handlers{
		repo: repo, catalog: catalog, auth: authSvc, inUse: inUse,
		createLimiter: auth.NewLimiterPer(createsPerDay, 24*time.Hour),
		check:         newKeyChecker(),
	}
}

// SweepLimiters dọn rate limiter; gọi định kỳ.
func (h *Handlers) SweepLimiters() {
	h.createLimiter.Sweep()
	h.check.sweep()
}

// Register gắn route vào mux API; mọi route yêu cầu đăng nhập.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/keys", h.auth.RequireUser(http.HandlerFunc(h.handleList)))
	mux.Handle("POST /api/keys", h.auth.RequireUser(http.HandlerFunc(h.handleCreate)))
	mux.Handle("POST /api/keys/test", h.auth.RequireUser(http.HandlerFunc(h.handleTest)))
	mux.Handle("DELETE /api/keys/{id}", h.auth.RequireUser(http.HandlerFunc(h.handleDelete)))
	mux.Handle("GET /api/providers", h.auth.RequireUser(http.HandlerFunc(h.handleProviders)))
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	list, err := h.repo.List(r.Context(), u.ID)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func validAPIKey(k string) bool {
	if n := utf8.RuneCountInString(k); n < minAPIKeyLen || n > maxAPIKeyLen {
		return false
	}
	for _, r := range k {
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func (h *Handlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var req api.CreateAPIKeyRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if _, ok := worker.LookupProvider(req.Provider); !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_provider", "Nhà cung cấp không được hỗ trợ")
		return
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	if !validAPIKey(req.APIKey) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_api_key", "API key không hợp lệ")
		return
	}
	label := strings.TrimSpace(req.Label)
	if utf8.RuneCountInString(label) > maxLabelLen {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_label", "Nhãn tối đa 60 ký tự")
		return
	}
	if ok, wait := h.createLimiter.Allow(u.ID); !ok {
		httpx.WriteErr(w, r, httpx.TooManyToday(wait))
		return
	}
	k, err := h.repo.Create(r.Context(), u.ID, req.Provider, label, req.APIKey, maxKeysPerUser)
	if errors.Is(err, ErrKeyLimit) {
		httpx.WriteError(w, http.StatusConflict, "key_limit", "Đã đạt tối đa 20 key. Xoá key không dùng rồi thử lại.")
		return
	}
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, k)
}

func (h *Handlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	id := r.PathValue("id")
	if _, err := h.repo.Get(r.Context(), u.ID, id); err != nil {
		h.writeRepoErr(w, r, err)
		return
	}
	if h.inUse != nil {
		busy, err := h.inUse(r, u.ID, id)
		if err != nil {
			httpx.WriteErr(w, r, err)
			return
		}
		if busy {
			httpx.WriteError(w, http.StatusConflict, "key_in_use", "Key đang được một truyện đang chạy sử dụng, hãy dừng truyện trước")
			return
		}
	}
	if err := h.repo.Delete(r.Context(), u.ID, id); err != nil {
		h.writeRepoErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleProviders(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.catalog.Providers())
}

func (h *Handlers) writeRepoErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy API key")
		return
	}
	httpx.WriteErr(w, r, err)
}
