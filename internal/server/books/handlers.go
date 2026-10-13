package books

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/auth"
	"github.com/voocel/ainovel-cli/internal/server/httpx"
	"github.com/voocel/ainovel-cli/internal/server/jobs"
	"github.com/voocel/ainovel-cli/internal/server/keys"
)

// Giới hạn độ dài input (ký tự).
const (
	maxPromptRunes = 4000
	maxSteerRunes  = 2000
	exportTimeout  = 2 * time.Minute
)

// Limits là các giới hạn liên quan tới truyện.
type Limits struct {
	MaxBooksPerUser  int
	MaxBookBudgetUSD float64
	MaxSSEPerUser    int
	MaxSSETotal      int
}

// Service gom handler truyện.
type Service struct {
	repo     *Repo
	keys     *keys.Repo
	catalog  *keys.Catalog
	auth     *auth.Service
	sup      *jobs.Supervisor
	exporter *Exporter
	dataDir  string
	limits   Limits
	closing  <-chan struct{}

	sseMu      sync.Mutex
	ssePerUser map[string]int
	sseTotal   int
}

// Deps là phụ thuộc của Service.
type Deps struct {
	Repo     *Repo
	Keys     *keys.Repo
	Catalog  *keys.Catalog
	Auth     *auth.Service
	Sup      *jobs.Supervisor
	Exporter *Exporter
	DataDir  string
	Limits   Limits
	Closing  <-chan struct{} // đóng khi server tắt: SSE tự kết thúc
}

// NewService tạo service.
func NewService(d Deps) *Service {
	return &Service{
		repo: d.Repo, keys: d.Keys, catalog: d.Catalog, auth: d.Auth, sup: d.Sup,
		exporter: d.Exporter, dataDir: d.DataDir, limits: d.Limits, closing: d.Closing,
		ssePerUser: make(map[string]int),
	}
}

// Register gắn route. Route JSON thường vào apiMux (có CSRF + timeout); SSE và
// tải file vào rootMux (chỉ GET, tự quản deadline).
func (s *Service) Register(apiMux, rootMux *http.ServeMux) {
	need := s.auth.RequireUser
	h := func(fn http.HandlerFunc) http.Handler { return need(fn) }
	apiMux.Handle("GET /api/books", h(s.handleList))
	apiMux.Handle("POST /api/books", h(s.handleCreate))
	apiMux.Handle("GET /api/books/{id}", h(s.handleGet))
	apiMux.Handle("PATCH /api/books/{id}", h(s.handlePatch))
	apiMux.Handle("DELETE /api/books/{id}", h(s.handleDelete))
	apiMux.Handle("POST /api/books/{id}/pause", h(s.handlePause))
	apiMux.Handle("POST /api/books/{id}/resume", h(s.handleResume))
	apiMux.Handle("POST /api/books/{id}/steer", h(s.handleSteer))
	apiMux.Handle("POST /api/books/{id}/next", h(s.handleNext))
	apiMux.Handle("GET /api/books/{id}/chapters", h(s.handleChapters))
	apiMux.Handle("GET /api/books/{id}/chapters/{n}", h(s.handleChapter))
	apiMux.Handle("GET /api/books/{id}/outline", h(s.handleOutline))
	rootMux.Handle("GET /api/books/{id}/events", h(s.handleEvents))
	rootMux.Handle("GET /api/books/{id}/export", h(s.handleExport))
}

// KeyInUse dùng cho keys.Handlers: không cho xoá key đang được truyện chạy/chờ chạy dùng.
func (s *Service) KeyInUse(r *http.Request, userID, keyID string) (bool, error) {
	return s.repo.KeyInUse(r.Context(), userID, keyID)
}

func errNotFound() error {
	return httpx.NewError(http.StatusNotFound, "not_found", "Không tìm thấy truyện")
}

func conflict(code, msg string) error { return httpx.NewError(http.StatusConflict, code, msg) }

func badRequest(code, msg string) error { return httpx.NewError(http.StatusBadRequest, code, msg) }

// load lấy truyện của user đang đăng nhập theo {id}.
func (s *Service) load(r *http.Request) (auth.User, bookRow, error) {
	u, _ := auth.UserFrom(r.Context())
	b, err := s.repo.Get(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		return u, b, errNotFound()
	}
	return u, b, err
}

func (s *Service) bookDir(b bookRow) (string, error) {
	return jobs.BookDir(s.dataDir, b.UserID, b.ID)
}

func spent(b bookRow) float64 {
	if b.Snapshot == nil {
		return 0
	}
	return b.Snapshot.TotalCostUSD
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	list, err := s.repo.List(r.Context(), u.ID)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	_, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b.Book)
}

func (s *Service) validBudget(v float64) error {
	if !(v > 0) || v > s.limits.MaxBookBudgetUSD {
		return badRequest("invalid_budget", "Ngân sách phải lớn hơn 0 và không quá $"+strconv.FormatFloat(s.limits.MaxBookBudgetUSD, 'f', -1, 64))
	}
	return nil
}

func validMode(m string) bool { return m == api.AdvanceAuto || m == api.AdvanceReview }

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var req api.CreateBookRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" || utf8.RuneCountInString(prompt) > maxPromptRunes {
		httpx.WriteErr(w, r, badRequest("invalid_prompt", "Ý tưởng truyện phải có từ 1 đến 4000 ký tự"))
		return
	}
	if !validMode(req.AdvanceMode) {
		httpx.WriteErr(w, r, badRequest("invalid_advance_mode", "Chế độ viết không hợp lệ"))
		return
	}
	if err := s.validBudget(req.BudgetUSD); err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	// Key phải là của chính user (404 nếu không); FK kép trong DB chặn thêm một lớp.
	key, err := s.keys.Get(r.Context(), u.ID, req.APIKeyID)
	if errors.Is(err, keys.ErrNotFound) {
		httpx.WriteErr(w, r, httpx.NewError(http.StatusNotFound, "key_not_found", "Không tìm thấy API key"))
		return
	}
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if !s.catalog.Allowed(key.Provider, req.Model) {
		httpx.WriteErr(w, r, httpx.NewError(http.StatusUnprocessableEntity, "model_not_allowed", "Model này không dùng được với API key đã chọn (cần có bảng giá để giới hạn ngân sách)"))
		return
	}
	n, err := s.repo.Count(r.Context(), u.ID)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if n >= s.limits.MaxBooksPerUser {
		httpx.WriteErr(w, r, conflict("book_limit", "Bạn đã có tối đa "+strconv.Itoa(s.limits.MaxBooksPerUser)+" truyện, hãy xoá bớt truyện cũ"))
		return
	}
	b, err := s.repo.Create(r.Context(), newBook{
		UserID: u.ID, APIKeyID: key.ID, Prompt: prompt, Provider: key.Provider,
		Model: req.Model, AdvanceMode: req.AdvanceMode, BudgetUSD: req.BudgetUSD,
	})
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	s.sup.Kick()
	httpx.WriteJSON(w, http.StatusCreated, b.Book)
}

func (s *Service) handlePatch(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	var req api.UpdateBookRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	budget, mode, keyID := b.BudgetUSD, b.AdvanceMode, b.APIKeyID
	if req.BudgetUSD != nil {
		// Worker chỉ nhận ngân sách lúc khởi động: đổi khi đang chạy sẽ không có hiệu lực.
		if b.Status == api.StatusQueued || b.Status == api.StatusRunning {
			httpx.WriteErr(w, r, conflict("book_running", "Hãy tạm dừng truyện trước khi đổi ngân sách"))
			return
		}
		if err := s.validBudget(*req.BudgetUSD); err != nil {
			httpx.WriteErr(w, r, err)
			return
		}
		if *req.BudgetUSD <= spent(b) {
			httpx.WriteErr(w, r, badRequest("budget_too_low", "Ngân sách mới phải lớn hơn số đã dùng ($"+strconv.FormatFloat(spent(b), 'f', 2, 64)+")"))
			return
		}
		budget = *req.BudgetUSD
	}
	if req.AdvanceMode != nil {
		if !validMode(*req.AdvanceMode) {
			httpx.WriteErr(w, r, badRequest("invalid_advance_mode", "Chế độ viết không hợp lệ"))
			return
		}
		mode = *req.AdvanceMode
	}
	if req.APIKeyID != nil {
		key, err := s.keys.Get(r.Context(), u.ID, *req.APIKeyID)
		if errors.Is(err, keys.ErrNotFound) {
			httpx.WriteErr(w, r, httpx.NewError(http.StatusNotFound, "key_not_found", "Không tìm thấy API key"))
			return
		}
		if err != nil {
			httpx.WriteErr(w, r, err)
			return
		}
		if key.Provider != b.Provider {
			httpx.WriteErr(w, r, badRequest("provider_mismatch", "Key mới phải cùng nhà cung cấp với model của truyện"))
			return
		}
		keyID = key.ID
	}
	// Nâng ngân sách cho truyện đã hết ngân sách: chuyển sang tạm dừng để tiếp tục được.
	updated, err := s.repo.update(r.Context(), `
		UPDATE books SET budget_usd = $3::float8, advance_mode = $4, api_key_id = NULLIF($5, '')::uuid, updated_at = now(),
			status = CASE WHEN status = 'budget_exhausted' AND $3::float8 > $6::float8 THEN 'paused' ELSE status END
		WHERE id::text = $2 AND user_id = $1 AND status <> 'deleting'`,
		u.ID, b.ID, budget, mode, keyID, spent(b))
	if err != nil {
		httpx.WriteErr(w, r, mapNotFound(err))
		return
	}
	if updated.Status != b.Status {
		s.sup.PublishStatus(b.ID, api.StatusChange{Status: updated.Status})
	}
	if mode != b.AdvanceMode && s.sup.IsRunning(b.ID) {
		_, _ = s.sup.Command(r.Context(), b.ID, api.WorkerCommand{Type: api.MsgSetAdvanceMode, Mode: mode})
	}
	httpx.WriteJSON(w, http.StatusOK, updated.Book)
}

func mapNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return errNotFound()
	}
	return err
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if _, err := s.repo.update(r.Context(), `UPDATE books SET status = 'deleting', updated_at = now()
		WHERE id::text = $2 AND user_id = $1 AND status <> 'deleting'`, u.ID, b.ID); err != nil {
		httpx.WriteErr(w, r, mapNotFound(err))
		return
	}
	// Dừng worker có thể mất tới ~30s: làm ở nền, client thấy truyện biến khỏi danh sách ngay.
	go s.finishDelete(b.ID, b.UserID)
	w.WriteHeader(http.StatusAccepted)
}

// finishDelete: dừng worker → chờ Wait xong → xoá thư mục → xoá dòng.
func (s *Service) finishDelete(bookID, userID string) {
	log := slog.Default().With("module", "books", "book_id", bookID)
	s.sup.StopAndWait(bookID)
	dir, err := jobs.BookDir(s.dataDir, userID, bookID)
	if err != nil {
		log.Error("đường dẫn truyện không hợp lệ khi xoá", "err", err)
		return
	}
	s.exporter.WaitDir(dir)
	if err := os.RemoveAll(dir); err != nil {
		log.Error("xoá thư mục truyện thất bại", "err", err)
		return
	}
	if err := s.repo.hardDelete(context.Background(), bookID); err != nil {
		log.Error("xoá dòng truyện thất bại", "err", err)
		return
	}
	s.sup.Hubs().Remove(bookID)
}

// ResumeDeletes làm tiếp các lần xoá dở dang trước khi server tắt.
func (s *Service) ResumeDeletes(ctx context.Context) {
	list, err := s.repo.deletingBooks(ctx)
	if err != nil {
		slog.Error("đọc truyện đang xoá thất bại", "err", err)
		return
	}
	for _, b := range list {
		go s.finishDelete(b[0], b[1])
	}
}

func (s *Service) writeResult(w http.ResponseWriter, r *http.Request, bookID, userID string, queued, pending bool) {
	b, err := s.repo.Get(r.Context(), userID, bookID)
	if err != nil {
		httpx.WriteErr(w, r, mapNotFound(err))
		return
	}
	status := http.StatusOK
	if pending {
		status = http.StatusAccepted
	}
	httpx.WriteJSON(w, status, api.CommandResult{Book: b.Book, Queued: queued, Pending: pending})
}

// queue chuyển truyện sang queued với việc cần làm; chỉ khi đang ở một trong các trạng thái cho phép.
func (s *Service) queue(r *http.Request, userID, bookID, action, text string, from ...api.BookStatus) (bookRow, error) {
	states := make([]string, len(from))
	for i, st := range from {
		states[i] = string(st)
	}
	b, err := s.repo.update(r.Context(), `
		UPDATE books SET status = 'queued', updated_at = now(),
			pending_action = CASE WHEN $3 = '' THEN COALESCE(pending_action, 'resume') ELSE $3 END,
			pending_text = CASE
				WHEN $3 = 'steer' AND pending_action = 'steer' AND COALESCE(pending_text, '') <> '' THEN pending_text || E'\n\n' || $4
				WHEN $3 = '' THEN pending_text
				ELSE NULLIF($4, '') END
		WHERE id::text = $2 AND user_id = $1 AND status = ANY($5)`,
		userID, bookID, action, text, states)
	if err == nil {
		s.sup.PublishStatus(bookID, api.StatusChange{Status: api.StatusQueued})
		s.sup.Kick()
	}
	return b, err
}

func (s *Service) handlePause(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if s.sup.IsRunning(b.ID) {
		ack, err := s.sup.Command(r.Context(), b.ID, api.WorkerCommand{Type: api.MsgAbort})
		if err == nil {
			s.writeResult(w, r, b.ID, u.ID, false, ack == nil)
			return
		}
	}
	// Chưa có worker: huỷ lượt đang xếp hàng.
	if _, err := s.repo.update(r.Context(), `UPDATE books SET status = 'paused', updated_at = now()
		WHERE id::text = $2 AND user_id = $1 AND status = 'queued'`, u.ID, b.ID); err != nil {
		httpx.WriteErr(w, r, conflictIfNotFound(err, "not_running", "Truyện không đang chạy"))
		return
	}
	s.sup.PublishStatus(b.ID, api.StatusChange{Status: api.StatusPaused})
	s.writeResult(w, r, b.ID, u.ID, false, false)
}

func conflictIfNotFound(err error, code, msg string) error {
	if errors.Is(err, ErrNotFound) {
		return conflict(code, msg)
	}
	return err
}

func (s *Service) handleResume(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if b.Status == api.StatusBudgetExhausted {
		httpx.WriteErr(w, r, conflict("budget_exhausted", "Truyện đã dùng hết ngân sách, hãy tăng ngân sách trước"))
		return
	}
	if _, err := s.queue(r, u.ID, b.ID, "", "", api.StatusPaused, api.StatusFailed, api.StatusInterrupted); err != nil {
		httpx.WriteErr(w, r, conflictIfNotFound(err, "cannot_resume", "Truyện không ở trạng thái tiếp tục được"))
		return
	}
	s.writeResult(w, r, b.ID, u.ID, true, false)
}

func (s *Service) handleNext(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if _, err := s.queue(r, u.ID, b.ID, api.ActionAdvance, "", api.StatusWaitingReview); err != nil {
		httpx.WriteErr(w, r, conflictIfNotFound(err, "not_waiting_review", "Truyện không ở trạng thái chờ duyệt"))
		return
	}
	s.writeResult(w, r, b.ID, u.ID, true, false)
}

func (s *Service) handleSteer(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	var req api.SteerRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" || utf8.RuneCountInString(text) > maxSteerRunes {
		httpx.WriteErr(w, r, badRequest("invalid_text", "Góp ý phải có từ 1 đến 2000 ký tự"))
		return
	}
	if s.sup.IsRunning(b.ID) {
		ack, err := s.sup.Command(r.Context(), b.ID, api.WorkerCommand{Type: api.MsgSteer, Text: text})
		if err == nil {
			if ack != nil && ack.Error != "" {
				httpx.WriteErr(w, r, conflict("steer_failed", ack.Error))
				return
			}
			s.writeResult(w, r, b.ID, u.ID, false, ack == nil)
			return
		}
		if errors.Is(err, jobs.ErrNotReady) {
			httpx.WriteErr(w, r, &httpx.Error{Status: http.StatusServiceUnavailable, Code: "worker_starting",
				Message: "Truyện đang khởi động, chưa nhận được góp ý. Hãy thử lại sau vài giây.", RetryAfter: 5 * time.Second})
			return
		}
		// Worker vừa thoát: rơi xuống nhánh xếp hàng.
		if b, err = s.repo.Get(r.Context(), u.ID, b.ID); err != nil {
			httpx.WriteErr(w, r, mapNotFound(err))
			return
		}
	}
	switch {
	case b.Snapshot == nil || b.Snapshot.Phase == "":
		httpx.WriteErr(w, r, conflict("not_started", "Truyện chưa khởi động xong, hãy chờ lượt chạy đầu tiên rồi góp ý"))
		return
	case b.Status == api.StatusBudgetExhausted:
		httpx.WriteErr(w, r, conflict("budget_exhausted", "Truyện đã dùng hết ngân sách, hãy tăng ngân sách trước"))
		return
	case b.Status == api.StatusQueued && b.PendingAction != api.ActionSteer:
		httpx.WriteErr(w, r, conflict("queued", "Truyện đang chờ chạy, hãy góp ý khi truyện bắt đầu chạy"))
		return
	}
	if _, err := s.queue(r, u.ID, b.ID, api.ActionSteer, text,
		api.StatusPaused, api.StatusWaitingReview, api.StatusCompleted, api.StatusFailed, api.StatusInterrupted, api.StatusQueued); err != nil {
		httpx.WriteErr(w, r, conflictIfNotFound(err, "cannot_steer", "Truyện vừa đổi trạng thái, hãy thử lại"))
		return
	}
	s.writeResult(w, r, b.ID, u.ID, true, false)
}

func (s *Service) handleChapters(w http.ResponseWriter, r *http.Request) {
	_, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	dir, err := s.bookDir(b)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	list, err := ListChapters(dir)
	if err != nil {
		httpx.WriteErr(w, r, readConflict(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// readConflict: đọc store lỗi (thường do worker đang ghi) → 409 cho client thử lại.
func readConflict(err error) error {
	slog.Warn("đọc dữ liệu truyện thất bại", "err", err)
	return conflict("read_failed", "Dữ liệu truyện đang được cập nhật, hãy thử lại")
}

func (s *Service) handleChapter(w http.ResponseWriter, r *http.Request) {
	_, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n <= 0 {
		httpx.WriteErr(w, r, httpx.NewError(http.StatusNotFound, "chapter_not_found", "Không tìm thấy chương"))
		return
	}
	dir, err := s.bookDir(b)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	ch, err := ReadChapter(dir, n)
	if errors.Is(err, ErrChapterNotFound) {
		httpx.WriteErr(w, r, httpx.NewError(http.StatusNotFound, "chapter_not_found", "Không tìm thấy chương"))
		return
	}
	if err != nil {
		httpx.WriteErr(w, r, readConflict(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ch)
}

func (s *Service) handleOutline(w http.ResponseWriter, r *http.Request) {
	_, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	dir, err := s.bookDir(b)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	o, err := ReadOutline(dir)
	if err != nil {
		httpx.WriteErr(w, r, readConflict(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (s *Service) handleExport(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	format := r.URL.Query().Get("format")
	if format != "txt" && format != "epub" {
		httpx.WriteErr(w, r, badRequest("invalid_format", "Định dạng xuất phải là txt hoặc epub"))
		return
	}
	dir, err := s.bookDir(b)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), exportTimeout)
	defer cancel()
	path, name, err := s.exporter.Export(ctx, u.ID, dir, format)
	switch {
	case errors.Is(err, ErrExportBusy):
		httpx.WriteErr(w, r, httpx.TooManyRequests(10*time.Second))
		return
	case errors.Is(err, ErrNothingToExport):
		httpx.WriteErr(w, r, conflict("nothing_to_export", "Chưa có chương nào hoàn thành để xuất"))
		return
	case err != nil:
		httpx.WriteErr(w, r, err)
		return
	}
	ctype := "text/plain; charset=utf-8"
	if format == "epub" {
		ctype = "application/epub+zip"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}
