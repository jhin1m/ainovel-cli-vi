package keys

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/litellm"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/entry/worker"
	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/auth"
	"github.com/voocel/ainovel-cli/internal/server/httpx"
)

// Giới hạn kiểm key. Mỗi lần kiểm tốn quota của user và một lượt gọi ra ngoài
// từ server, nên chặn theo user, theo IP và số lượt đang chạy toàn server.
const (
	checksPerMinute   = 3
	checksPerDayUser  = 20
	checksPerDayIP    = 60
	maxChecksInFlight = 4
	checkTimeout      = 20 * time.Second
	// checkMaxTokens: đủ nhỏ để rẻ nhưng không quá nhỏ — model có thinking có thể
	// từ chối max_tokens quá thấp.
	checkMaxTokens     = 32
	checkBusyRetryWait = 5 * time.Second
)

// modelFactory dựng model gọi provider. Mặc định là worker.NewProbeModel (cùng
// allowlist và BaseURL cố định với worker); test thay bằng model trỏ vào server giả.
type modelFactory func(provider, model, apiKey string) (agentcore.ChatModel, error)

// keyChecker gom limiter, semaphore và cách dựng model cho POST /api/keys/test.
type keyChecker struct {
	userMinute *auth.Limiter
	userDay    *auth.Limiter
	ipDay      *auth.Limiter
	slots      chan struct{}
	newModel   modelFactory
	timeout    time.Duration
}

func newKeyChecker() *keyChecker {
	return &keyChecker{
		userMinute: auth.NewLimiterPer(checksPerMinute, time.Minute),
		userDay:    auth.NewLimiterPer(checksPerDayUser, 24*time.Hour),
		ipDay:      auth.NewLimiterPer(checksPerDayIP, 24*time.Hour),
		slots:      make(chan struct{}, maxChecksInFlight),
		newModel:   worker.NewProbeModel,
		timeout:    checkTimeout,
	}
}

func (c *keyChecker) sweep() {
	c.userMinute.Sweep()
	c.userDay.Sweep()
	c.ipDay.Sweep()
}

// allow áp giới hạn theo đúng thứ tự: user/phút, user/ngày, IP/ngày.
func (c *keyChecker) allow(r *http.Request, userID string) error {
	if ok, wait := c.userMinute.Allow(userID); !ok {
		return httpx.TooManyRequests(wait)
	}
	if ok, wait := c.userDay.Allow(userID); !ok {
		return httpx.TooManyToday(wait)
	}
	if ok, wait := c.ipDay.Allow(auth.IPKey(r)); !ok {
		return httpx.TooManyToday(wait)
	}
	return nil
}

// acquire giữ một chỗ trong semaphore toàn cục, không chờ. false = đã đủ chỗ.
func (c *keyChecker) acquire() bool {
	select {
	case c.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (c *keyChecker) release() { <-c.slots }

// errCheckBusy: đã có đủ lượt kiểm đang chạy toàn server.
var errCheckBusy = &httpx.Error{
	Status: http.StatusTooManyRequests, Code: "busy",
	Message: "Máy chủ đang kiểm nhiều key cùng lúc, thử lại sau ít giây", RetryAfter: checkBusyRetryWait,
}

// probe gọi provider đúng một lần (không qua llmretry hay vòng agentcore) và
// quy kết quả về nhóm đóng. Văn bản lỗi của provider không bao giờ ra khỏi hàm.
func (c *keyChecker) probe(ctx context.Context, provider, model, apiKey string) (api.TestKeyResult, error) {
	m, err := c.newModel(provider, model, apiKey)
	if err != nil {
		return api.TestKeyResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_, err = m.Generate(ctx, []agentcore.Message{agentcore.UserMsg("ping")}, nil, agentcore.WithMaxTokens(checkMaxTokens))
	if err == nil {
		return checkResult(api.KeyCheckOK, "", model), nil
	}
	kind, code := bootstrap.ClassifyProviderError(err)
	if kind == bootstrap.ProviderErrUnknown && errors.Is(err, context.DeadlineExceeded) {
		kind = bootstrap.ProviderErrNetwork
	}
	if kind == bootstrap.ProviderErrUnknown {
		kind = reclassifyUnknown(err, kind)
	}
	return checkResult(kind, code, model), nil
}

// Các pattern đọc lời báo lỗi của provider (chỉ để phân loại, không đưa ra ngoài).
var (
	// keyErrorPattern: lỗi nói về key/xác thực. Vài provider (Gemini) trả 400
	// thay vì 401 cho key sai.
	keyErrorPattern = regexp.MustCompile(`(?i)api[ _-]?key|unauthori[sz]ed|authenticat|credential`)
	// quotaErrorPattern: hết tiền/hạn mức. Anthropic ("credit balance is too low")
	// và DashScope/Qwen ("Arrearage") trả 400 thay vì 402.
	quotaErrorPattern = regexp.MustCompile(`(?i)credit balance|insufficient[ _-]?(balance|credit|quota|funds)|arrearage|billing|quota|余额`)
	// truncatedPattern: provider trả 200 nhưng hết max_tokens trước khi có nội dung
	// (thinking ăn hết), adapter biến thành lỗi không có HTTP status.
	truncatedPattern = regexp.MustCompile(`(?i)finish_reason=(max_tokens|length)`)
)

// reclassifyUnknown xếp lại lỗi chưa phân loại được nhưng chứng tỏ provider đã
// nhận key: response 200 bị cắt vì max_tokens, hoặc 400/422 về tham số (vd.
// max_tokens quá nhỏ với model có thinking — provider chỉ kiểm tham số sau khi
// xác thực). 400/422 nói về key hay về số dư thì báo đúng nhóm đó.
func reclassifyUnknown(err error, kind string) string {
	var le *litellm.LiteLLMError
	if !errors.As(err, &le) {
		return kind
	}
	switch {
	case le.StatusCode == 0 && truncatedPattern.MatchString(le.Message):
		return api.KeyCheckOK
	case le.StatusCode != http.StatusBadRequest && le.StatusCode != http.StatusUnprocessableEntity:
		return kind
	case keyErrorPattern.MatchString(le.Message):
		return api.KeyCheckAuthFailed
	case quotaErrorPattern.MatchString(le.Message):
		return api.KeyCheckQuota
	}
	return api.KeyCheckOK
}

// checkMessages: câu trả lời cho user theo nhóm kết quả, do server viết.
var checkMessages = map[string]string{
	api.KeyCheckOK:           "Key dùng được.",
	api.KeyCheckAuthFailed:   "Nhà cung cấp không nhận key này (sai key, key bị thu hồi hoặc hết hạn). Hãy kiểm tra lại key.",
	api.KeyCheckPlanMismatch: "Nhà cung cấp từ chối key với gói hoặc endpoint này: key có thể thuộc gói khác, gói đã hết hạn mức, hoặc model không có trong gói. Với Z.ai, key gói Coding Plan phải lưu dưới \"Z.ai Coding Plan\", key trả theo token lưu dưới \"Z.ai\".",
	api.KeyCheckRateLimited:  "Nhà cung cấp đang giới hạn tốc độ, chưa kiểm được key. Thử lại sau ít phút.",
	api.KeyCheckQuota:        "Tài khoản ở nhà cung cấp đã hết số dư hoặc hết hạn mức.",
	api.KeyCheckNetwork:      "Không kết nối được tới nhà cung cấp hoặc quá thời gian chờ. Thử lại sau.",
	api.KeyCheckUnknown:      "Nhà cung cấp trả về lỗi không xác định. Thử lại sau.",
}

func checkResult(kind, code, model string) api.TestKeyResult {
	msg, ok := checkMessages[kind]
	if !ok {
		kind, msg = api.KeyCheckUnknown, checkMessages[api.KeyCheckUnknown]
	}
	return api.TestKeyResult{OK: kind == api.KeyCheckOK, Kind: kind, Code: code, Message: msg, Model: model}
}

// probeModel chọn model dùng để kiểm: model "Đề xuất" của provider, không có thì
// model đầu tiên trong danh sách chọn lọc.
func probeModel(provider string) string {
	if m := recommendedModels[provider]; m != "" {
		return m
	}
	if list := curatedModels[provider]; len(list) > 0 {
		return list[0]
	}
	return ""
}

func (h *Handlers) handleTest(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var req api.TestKeyRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if err := h.check.allow(r, u.ID); err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if !h.check.acquire() {
		httpx.WriteErr(w, r, errCheckBusy)
		return
	}
	defer h.check.release()

	provider, key, err := h.repo.DecryptOwned(r.Context(), u.ID, req.KeyID)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "key_not_found", "Không tìm thấy API key")
		return
	}
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	defer clear(key)
	model := probeModel(provider)
	if model == "" {
		httpx.WriteErr(w, r, errors.New("provider không có model để kiểm: "+provider))
		return
	}
	res, err := h.check.probe(r.Context(), provider, model, string(key))
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	// Chỉ log nhóm và mã đóng: không có key, không có body của provider.
	httpx.LoggerFrom(r.Context()).Info("kiểm API key", slog.String("user_id", u.ID), slog.String("provider", provider),
		slog.String("kind", res.Kind), slog.String("code", res.Code))
	httpx.WriteJSON(w, http.StatusOK, res)
}
