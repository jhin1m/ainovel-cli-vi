package keys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voocel/agentcore"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/entry/worker"
	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/auth"
	"github.com/voocel/ainovel-cli/internal/server/db/dbtest"
	"github.com/voocel/ainovel-cli/internal/server/httpx"
	"github.com/voocel/ainovel-cli/internal/server/secret"
)

const (
	checkTestPassword = "mat-khau-kiem-key-123"
	checkTestKey      = "zai-THIS-IS-A-SECRET-KEY-0123456789"
	providerOKBody    = `{"id":"stub","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
)

// fakeProvider là endpoint OpenAI-compatible giả: đếm request, ghi lại key nhận
// được và trả status/body cấu hình sẵn.
type fakeProvider struct {
	srv     *httptest.Server
	hits    atomic.Int32
	mu      sync.Mutex
	status  int
	body    string
	gotAuth string
	gotMax  int           // max_tokens (hoặc max_completion_tokens) của request cuối
	block   chan struct{} // khác nil: chặn mọi request tới khi đóng
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	p := &fakeProvider{status: http.StatusOK, body: providerOKBody}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.hits.Add(1)
		var req struct {
			MaxTokens           int `json:"max_tokens"`
			MaxCompletionTokens int `json:"max_completion_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		p.mu.Lock()
		status, body, block := p.status, p.body, p.block
		p.gotAuth = r.Header.Get("Authorization")
		p.gotMax = max(req.MaxTokens, req.MaxCompletionTokens)
		p.mu.Unlock()
		if block != nil {
			select {
			case <-block:
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *fakeProvider) reply(status int, body string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status, p.body = status, body
}

func (p *fakeProvider) auth() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gotAuth
}

// modelAt dựng model bằng đúng factory của bootstrap như worker, chỉ đổi BaseURL
// sang provider giả. Đây là đường inject duy nhất, chỉ có trong test.
func modelAt(baseURL string) modelFactory {
	return func(provider, model, apiKey string) (agentcore.ChatModel, error) {
		wp, ok := worker.LookupProvider(provider)
		if !ok {
			return nil, fmt.Errorf("provider %q không được phép", provider)
		}
		cfg := bootstrap.Config{
			Provider: provider, ModelName: model, Language: "vi",
			Providers: map[string]bootstrap.ProviderConfig{
				provider: {APIKey: apiKey, Type: wp.Type, BaseURL: baseURL},
			},
		}
		cfg.FillDefaults()
		set, err := bootstrap.NewModelSet(cfg)
		if err != nil {
			return nil, err
		}
		return set.Default, nil
	}
}

type checkEnv struct {
	t        *testing.T
	ts       *httptest.Server
	h        *Handlers
	provider *fakeProvider
	seq      atomic.Int32
}

// newCheckEnv dựng auth + keys trên Postgres test, model trỏ vào provider giả.
func newCheckEnv(t *testing.T) *checkEnv {
	t.Helper()
	pool := dbtest.New(t)
	box, err := secret.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(pool, true)
	h := NewHandlers(NewRepo(pool, box), NewCatalog(models.NewModelRegistry()), authSvc, nil)
	p := newFakeProvider(t)
	h.check.newModel = modelAt(p.srv.URL)
	h.check.timeout = 2 * time.Second
	mux := http.NewServeMux()
	authSvc.Register(mux)
	h.Register(mux)
	ts := httptest.NewServer(httpx.Chain(mux, httpx.RequestContext(nil, "")))
	t.Cleanup(ts.Close)
	return &checkEnv{t: t, ts: ts, h: h, provider: p}
}

type checkClient struct {
	env *checkEnv
	hc  *http.Client
}

type checkResp struct {
	status int
	header http.Header
	body   []byte
}

func (r checkResp) errCode() string {
	var e api.ErrorResponse
	_ = json.Unmarshal(r.body, &e)
	return e.Error.Code
}

func (c *checkClient) do(method, path string, body any) checkResp {
	t := c.env.t
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, c.env.ts.URL+path, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return checkResp{status: resp.StatusCode, header: resp.Header, body: data}
}

// user đăng ký một user mới (cookie phiên trong jar).
func (e *checkEnv) user() *checkClient {
	e.t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &checkClient{env: e, hc: &http.Client{Jar: jar}}
	email := fmt.Sprintf("kiemkey%d@example.com", e.seq.Add(1))
	if r := c.do("POST", "/api/auth/signup", map[string]string{"email": email, "password": checkTestPassword, "display_name": "kiem"}); r.status != http.StatusCreated {
		e.t.Fatalf("signup: %d %s", r.status, r.body)
	}
	return c
}

func (c *checkClient) addKey(provider string) string {
	c.env.t.Helper()
	r := c.do("POST", "/api/keys", map[string]string{"provider": provider, "api_key": checkTestKey})
	var k api.APIKey
	if r.status != http.StatusCreated || json.Unmarshal(r.body, &k) != nil {
		c.env.t.Fatalf("thêm key: %d %s", r.status, r.body)
	}
	return k.ID
}

func (c *checkClient) test(keyID string) checkResp {
	c.env.t.Helper()
	return c.do("POST", "/api/keys/test", api.TestKeyRequest{KeyID: keyID})
}

func (c *checkClient) testOK(keyID string) api.TestKeyResult {
	c.env.t.Helper()
	r := c.test(keyID)
	var res api.TestKeyResult
	if r.status != http.StatusOK || json.Unmarshal(r.body, &res) != nil {
		c.env.t.Fatalf("kiểm key phải 200: %d %s", r.status, r.body)
	}
	return res
}

func TestKeyCheckResults(t *testing.T) {
	env := newCheckEnv(t)
	cases := []struct {
		name     string
		status   int
		body     string
		wantKind string
		wantCode string
	}{
		{"ok", http.StatusOK, providerOKBody, api.KeyCheckOK, ""},
		{"401", http.StatusUnauthorized, `{"error":{"message":"invalid api key org-SECRET42 last4 6789"}}`, api.KeyCheckAuthFailed, ""},
		{"1113", http.StatusTooManyRequests, `{"error":{"code":"1113","message":"Insufficient balance for org-SECRET42, balance 0.00"}}`, api.KeyCheckPlanMismatch, "1113"},
		{"429 thường", http.StatusTooManyRequests, `{"error":{"message":"rate limited org-SECRET42"}}`, api.KeyCheckRateLimited, ""},
		{"403", http.StatusForbidden, `{"error":{"message":"forbidden org-SECRET42"}}`, api.KeyCheckAuthFailed, ""},
		{"402 hết số dư", http.StatusPaymentRequired, `{"error":{"message":"Insufficient balance org-SECRET42"}}`, api.KeyCheckQuota, ""},
		// Lỗi tham số chứng tỏ provider đã nhận key: không được báo key hỏng.
		{"400 max_tokens", http.StatusBadRequest, `{"error":{"message":"max_tokens must be at least 1024 when thinking is enabled org-SECRET42"}}`, api.KeyCheckOK, ""},
		{"422 tham số", http.StatusUnprocessableEntity, `{"error":{"message":"unsupported parameter org-SECRET42"}}`, api.KeyCheckOK, ""},
		// Gemini trả 400 cho key sai.
		{"400 key sai", http.StatusBadRequest, `{"error":{"message":"API key not valid. Please pass a valid API key. org-SECRET42"}}`, api.KeyCheckAuthFailed, ""},
		// Anthropic và Qwen báo hết tiền bằng 400.
		{"400 hết credit", http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the API org-SECRET42"}}`, api.KeyCheckQuota, ""},
		{"400 nợ cước", http.StatusBadRequest, `{"code":"Arrearage","message":"Access denied, please make sure your account is in good standing. org-SECRET42"}`, api.KeyCheckQuota, ""},
	}
	var c *checkClient
	for i, tc := range cases {
		// Mỗi user kiểm tối đa checksPerMinute lần/phút, và mỗi IP chỉ đăng ký được
		// 10 lần/ngày: đổi user sau mỗi checksPerMinute ca.
		if i%checksPerMinute == 0 {
			c = env.user()
		}
		t.Run(tc.name, func(t *testing.T) {
			id := c.addKey("zai-coding")
			env.provider.reply(tc.status, tc.body)
			before := env.provider.hits.Load()
			r := c.test(id)
			var res api.TestKeyResult
			if r.status != http.StatusOK || json.Unmarshal(r.body, &res) != nil {
				t.Fatalf("kiểm key phải 200: %d %s", r.status, r.body)
			}
			if res.Kind != tc.wantKind || res.Code != tc.wantCode || res.OK != (tc.wantKind == api.KeyCheckOK) {
				t.Fatalf("kết quả = %+v, muốn kind=%s code=%s", res, tc.wantKind, tc.wantCode)
			}
			if res.Model != recommendedModels["zai-coding"] || res.Message != checkMessages[tc.wantKind] {
				t.Fatalf("model/message sai: %+v", res)
			}
			if got := env.provider.hits.Load() - before; got != 1 {
				t.Fatalf("mỗi lần kiểm phải gọi provider đúng 1 lần, got %d", got)
			}
			for _, leak := range []string{"org-SECRET42", "balance", "6789", checkTestKey, "Insufficient"} {
				if strings.Contains(string(r.body), leak) {
					t.Fatalf("response lộ %q: %s", leak, r.body)
				}
			}
			if env.provider.auth() != "Bearer "+checkTestKey {
				t.Fatalf("provider phải nhận đúng key đã lưu, got %q", env.provider.auth())
			}
			env.provider.mu.Lock()
			gotMax := env.provider.gotMax
			env.provider.mu.Unlock()
			if gotMax != checkMaxTokens {
				t.Fatalf("request kiểm phải gửi max_tokens %d, got %d", checkMaxTokens, gotMax)
			}
		})
	}
}

// Model có thinking (gemini-2.5-flash) có thể dùng hết max_tokens trước khi ra
// chữ: provider trả 200 nhưng không có nội dung. Key đã được nhận nên phải là ok.
func TestKeyCheckTruncatedReplyIsOK(t *testing.T) {
	env := newCheckEnv(t)
	env.provider.reply(http.StatusOK, `{"candidates":[{"finishReason":"MAX_TOKENS","content":{"role":"model"}}],"usageMetadata":{"promptTokenCount":1,"thoughtsTokenCount":32,"totalTokenCount":33}}`)
	c := env.user()
	res := c.testOK(c.addKey("gemini"))
	if res.Kind != api.KeyCheckOK || !res.OK {
		t.Fatalf("200 bị cắt vì max_tokens phải là key hợp lệ: %+v", res)
	}
	if got := env.provider.hits.Load(); got != 1 {
		t.Fatalf("phải gọi provider đúng 1 lần, got %d", got)
	}
}

func TestKeyCheckTimeoutIsNetwork(t *testing.T) {
	env := newCheckEnv(t)
	env.h.check.timeout = 200 * time.Millisecond
	block := make(chan struct{})
	defer close(block)
	env.provider.mu.Lock()
	env.provider.block = block
	env.provider.mu.Unlock()
	c := env.user()
	res := c.testOK(c.addKey("zai-coding"))
	if res.Kind != api.KeyCheckNetwork || res.OK {
		t.Fatalf("quá thời gian phải là network: %+v", res)
	}
	if got := env.provider.hits.Load(); got != 1 {
		t.Fatalf("quá thời gian không được gọi lại, got %d", got)
	}
}

func TestKeyCheckOwnershipAndInput(t *testing.T) {
	env := newCheckEnv(t)
	owner := env.user()
	id := owner.addKey("zai-coding")
	other := env.user()
	if r := other.test(id); r.status != http.StatusNotFound || r.errCode() != "key_not_found" {
		t.Fatalf("key của người khác phải 404 key_not_found: %d %s", r.status, r.body)
	}
	if r := other.do("POST", "/api/keys/test", map[string]string{"provider": "zai-coding", "api_key": checkTestKey}); r.status != http.StatusBadRequest {
		t.Fatalf("gửi key thô phải 400: %d %s", r.status, r.body)
	}
	if r := other.do("POST", "/api/keys/test", map[string]string{"key_id": id, "api_key": checkTestKey}); r.status != http.StatusBadRequest {
		t.Fatalf("body có api_key phải 400: %d %s", r.status, r.body)
	}
	if got := env.provider.hits.Load(); got != 0 {
		t.Fatalf("request bị từ chối không được gọi provider, got %d", got)
	}
	anon := &checkClient{env: env, hc: &http.Client{}}
	if r := anon.test(id); r.status != http.StatusUnauthorized {
		t.Fatalf("chưa đăng nhập phải 401: %d", r.status)
	}
	if res := owner.testOK(id); !res.OK {
		t.Fatalf("chủ key phải kiểm được: %+v", res)
	}
}

func TestKeyCheckUserLimits(t *testing.T) {
	env := newCheckEnv(t)
	c := env.user()
	id := c.addKey("zai-coding")
	for i := 0; i < checksPerMinute; i++ {
		c.testOK(id)
	}
	r := c.test(id)
	if r.status != http.StatusTooManyRequests || r.errCode() != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("lần thứ 4 trong một phút phải 429 kèm Retry-After: %d %s", r.status, r.body)
	}
	if got := env.provider.hits.Load(); got != checksPerMinute {
		t.Fatalf("lần bị chặn không được gọi provider: %d lần gọi", got)
	}

	// Giới hạn theo ngày (hạ xuống 2 để test): lượt phút còn nhưng ngày đã hết.
	env.h.check.userDay = auth.NewLimiterPer(2, 24*time.Hour)
	d := env.user()
	did := d.addKey("zai-coding")
	d.testOK(did)
	d.testOK(did)
	if r := d.test(did); r.status != http.StatusTooManyRequests || r.errCode() != "rate_limited" {
		t.Fatalf("quá giới hạn ngày của user phải 429: %d %s", r.status, r.body)
	}
}

func TestKeyCheckIPLimit(t *testing.T) {
	env := newCheckEnv(t)
	env.h.check.ipDay = auth.NewLimiterPer(2, 24*time.Hour)
	a, b := env.user(), env.user()
	aid, bid := a.addKey("zai-coding"), b.addKey("zai-coding")
	a.testOK(aid)
	a.testOK(aid)
	// User khác cùng IP vẫn bị chặn: giới hạn theo IP, không theo tài khoản.
	if r := b.test(bid); r.status != http.StatusTooManyRequests || r.errCode() != "rate_limited" {
		t.Fatalf("quá giới hạn ngày của IP phải 429: %d %s", r.status, r.body)
	}
}

func TestKeyCheckConcurrencyLimit(t *testing.T) {
	env := newCheckEnv(t)
	block := make(chan struct{})
	env.provider.mu.Lock()
	env.provider.block = block
	env.provider.mu.Unlock()

	var wg sync.WaitGroup
	results := make(chan int, maxChecksInFlight)
	for i := 0; i < maxChecksInFlight; i++ {
		c := env.user()
		id := c.addKey("zai-coding")
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- c.test(id).status
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for env.provider.hits.Load() < maxChecksInFlight {
		if time.Now().After(deadline) {
			t.Fatalf("chỉ %d lượt kiểm tới được provider", env.provider.hits.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	c := env.user()
	r := c.test(c.addKey("zai-coding"))
	if r.status != http.StatusTooManyRequests || r.errCode() != "busy" || r.header.Get("Retry-After") == "" {
		t.Fatalf("lượt thứ %d đồng thời phải 429 busy: %d %s", maxChecksInFlight+1, r.status, r.body)
	}
	close(block)
	wg.Wait()
	close(results)
	for st := range results {
		if st != http.StatusOK {
			t.Fatalf("lượt đang chạy phải xong 200, got %d", st)
		}
	}
	// Chỗ đã được nhả: kiểm tiếp được.
	if res := c.testOK(c.addKey("zai-coding")); !res.OK {
		t.Fatalf("sau khi nhả chỗ phải kiểm được: %+v", res)
	}
}

// Mọi nhóm lỗi provider của bootstrap đều có message do server viết.
func TestKeyCheckMessagesCoverProviderKinds(t *testing.T) {
	for _, kind := range []string{
		bootstrap.ProviderErrAuthFailed, bootstrap.ProviderErrPlanMismatch, bootstrap.ProviderErrRateLimited,
		bootstrap.ProviderErrQuota, bootstrap.ProviderErrNetwork, bootstrap.ProviderErrUnknown, api.KeyCheckOK,
	} {
		if res := checkResult(kind, "", "m"); res.Kind != kind || res.Message == "" {
			t.Fatalf("thiếu message cho %q: %+v", kind, res)
		}
	}
	if res := checkResult("lạ", "", "m"); res.Kind != api.KeyCheckUnknown {
		t.Fatalf("kind lạ phải thành unknown: %+v", res)
	}
}

func TestProbeModelPicksRecommended(t *testing.T) {
	for _, p := range worker.Providers() {
		m := probeModel(p.Name)
		if m == "" {
			t.Fatalf("provider %s không có model để kiểm", p.Name)
		}
		if rec := recommendedModels[p.Name]; rec != "" && m != rec {
			t.Fatalf("provider %s: muốn model đề xuất %s, got %s", p.Name, rec, m)
		}
	}
}
