package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/agentcore/llm"
	"github.com/voocel/ainovel-cli/internal/llmretry"
	"github.com/voocel/litellm"
)

func TestClassifyProviderError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind string
		wantCode string
	}{
		{"1113 code chuỗi", litellm.NewHTTPError("glm", 429, `{"error":{"code":"1113","message":"Insufficient balance or no resource package. Please recharge."}}`), ProviderErrPlanMismatch, "1113"},
		{"1113 code chuỗi qua fmt wrap", fmt.Errorf("writer: %w", litellm.NewHTTPError("glm", 429, `{"error":{"code":"1113","message":"x"}}`)), ProviderErrPlanMismatch, "1113"},
		{"1113 code số, chỉ có trong message (glm)", litellm.NewHTTPError("glm", 429, `{"code":1113,"msg":"Insufficient balance"}`), ProviderErrPlanMismatch, "1113"},
		{"1113 trong message nhưng không phải glm", litellm.NewHTTPError("openai", 429, `{"code":1113,"msg":"Insufficient balance"}`), ProviderErrRateLimited, ""},
		{"11130 không khớp nguyên từ", litellm.NewHTTPError("glm", 429, `{"code":11130,"msg":"busy"}`), ProviderErrRateLimited, ""},
		{"401", litellm.NewHTTPError("glm", 401, `{"error":{"code":"1000","message":"invalid api key"}}`), ProviderErrAuthFailed, ""},
		{"429 không có code", litellm.NewHTTPError("glm", 429, `{"error":{"message":"rate limited"}}`), ProviderErrRateLimited, ""},
		{"402", litellm.NewHTTPError("openai", 402, `{"error":{"message":"payment required"}}`), ProviderErrQuota, ""},
		{"lỗi mạng", litellm.NewNetworkError("glm", "dial tcp: connection refused", errors.New("refused")), ProviderErrNetwork, ""},
		{"timeout", litellm.NewTimeoutError("glm", "request timeout"), ProviderErrNetwork, ""},
		{"lỗi lạ", errors.New("boom"), ProviderErrUnknown, ""},
		{"đã bọc", &ProviderRejectedError{Kind: ProviderErrPlanMismatch, Code: "1113", Err: errors.New("x")}, ProviderErrPlanMismatch, "1113"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, code := ClassifyProviderError(tt.err)
			if kind != tt.wantKind || code != tt.wantCode {
				t.Fatalf("ClassifyProviderError = (%q, %q), want (%q, %q)", kind, code, tt.wantKind, tt.wantCode)
			}
		})
	}
	if kind, code := ClassifyProviderError(nil); kind != "" || code != "" {
		t.Fatalf("nil phải trả rỗng, got (%q, %q)", kind, code)
	}
}

func TestProviderRejectedErrorContract(t *testing.T) {
	inner := litellm.NewHTTPError("glm", 429, `{"error":{"code":"1113","message":"Insufficient balance"}}`)
	if !inner.Retryable {
		t.Fatal("tiền đề: litellm coi 429 là retryable")
	}
	err := rejectIfFatal("zai-coding", "glm-4.7", fmt.Errorf("adapter: %w", inner))

	var retryable agentcore.RetryableError
	if !errors.As(err, &retryable) || retryable.Retryable() {
		t.Fatalf("RetryableError đầu tiên trong chuỗi phải báo false: %v", err)
	}
	var le *litellm.LiteLLMError
	if !errors.As(err, &le) || le.Code != "1113" {
		t.Fatalf("Unwrap phải giữ lỗi gốc: %v", err)
	}
	if !strings.HasPrefix(err.Error(), "provider_rejected:") {
		t.Fatalf("Error() phải bắt đầu bằng tiền tố ổn định: %q", err.Error())
	}
	if code, ok := ParseProviderRejected("writer lỗi: " + err.Error()); !ok || code != "1113" {
		t.Fatalf("ParseProviderRejected = (%q, %v)", code, ok)
	}
	if code, ok := ProviderRejection(errors.New(err.Error()), ""); !ok || code != "1113" {
		t.Fatalf("ProviderRejection từ chuỗi đã ép = (%q, %v)", code, ok)
	}
	if again := rejectIfFatal("zai-coding", "glm-4.7", err); again != err {
		t.Fatal("không được bọc hai lần")
	}
	if _, ok := ParseProviderRejected("rate limited [rate_limit, HTTP 429, glm]"); ok {
		t.Fatal("chuỗi thường không phải provider_rejected")
	}
	if code, ok := ParseProviderRejected("provider_rejected: plan_mismatch code=9999 provider=x model=y: z"); !ok || code != "" {
		t.Fatalf("mã ngoài allowlist phải bị bỏ, got (%q, %v)", code, ok)
	}

	plain := litellm.NewHTTPError("glm", 429, `{"error":{"message":"rate limited"}}`)
	if got := rejectIfFatal("zai", "glm-4.7", plain); got != error(plain) {
		t.Fatalf("429 thường phải giữ nguyên: %v", got)
	}
}

// providerStub là endpoint OpenAI-compatible giả, đếm số request nhận được.
func providerStub(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

const (
	body1113 = `{"error":{"code":"1113","message":"Insufficient balance or no resource package. Please recharge."}}`
	bodyOK   = `{"id":"stub","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
)

func glmModel(t *testing.T, providerKey, baseURL string) agentcore.ChatModel {
	t.Helper()
	m, err := createModelFromConfig(providerKey, "glm-4.7", ProviderConfig{
		Type:    "glm",
		APIKey:  "stub-key",
		BaseURL: baseURL,
	}, map[string]agentcore.ChatModel{})
	if err != nil {
		t.Fatalf("createModelFromConfig: %v", err)
	}
	return m
}

func runAgentOnce(t *testing.T, model agentcore.ChatModel, maxRetries int) {
	t.Helper()
	agent := agentcore.NewAgent(agentcore.WithModel(model), agentcore.WithMaxRetries(maxRetries))
	if err := agent.Prompt(context.Background(), "hi"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	done := make(chan struct{})
	go func() {
		agent.WaitForIdle()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("agent không dừng")
	}
}

func TestModel1113NotRetriedByAgentLoop(t *testing.T) {
	srv, hits := providerStub(t, http.StatusTooManyRequests, body1113)
	runAgentOnce(t, glmModel(t, "zai-coding", srv.URL), 7)
	if got := hits.Load(); got != 1 {
		t.Fatalf("1113 qua vòng agentcore chỉ được gọi 1 lần, got %d", got)
	}
}

func TestModelPlain429StillRetriedByAgentLoop(t *testing.T) {
	srv, hits := providerStub(t, http.StatusTooManyRequests, `{"error":{"message":"rate limited"}}`)
	runAgentOnce(t, glmModel(t, "zai", srv.URL), 1)
	if got := hits.Load(); got != 2 {
		t.Fatalf("429 không có code vẫn phải retry (1 lần retry = 2 lần gọi), got %d", got)
	}
}

func TestModel1113NotRetriedByLLMRetry(t *testing.T) {
	srv, hits := providerStub(t, http.StatusTooManyRequests, body1113)
	_, err := llmretry.Generate(context.Background(), glmModel(t, "zai-coding", srv.URL), llmretry.Config{}, []agentcore.Message{agentcore.UserMsg("hi")})
	var rejected *ProviderRejectedError
	if !errors.As(err, &rejected) || rejected.Code != "1113" || rejected.Provider != "zai-coding" || rejected.Model != "glm-4.7" {
		t.Fatalf("phải trả ProviderRejectedError đủ thông tin: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("1113 qua llmretry chỉ được gọi 1 lần, got %d", got)
	}
}

func TestModel1113StreamErrorIsRejected(t *testing.T) {
	srv, hits := providerStub(t, http.StatusTooManyRequests, body1113)
	m := glmModel(t, "zai-coding", srv.URL)
	var gotErr error
	ch, err := m.GenerateStream(context.Background(), []agentcore.Message{agentcore.UserMsg("hi")}, nil)
	if err != nil {
		gotErr = err
	} else {
		for ev := range ch {
			if ev.Type == agentcore.StreamEventError {
				gotErr = ev.Err
			}
		}
	}
	var rejected *ProviderRejectedError
	if !errors.As(gotErr, &rejected) {
		t.Fatalf("lỗi stream phải là ProviderRejectedError: %v", gotErr)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("got %d request", got)
	}
}

// Failover chuyển sang provider dự phòng khác (không phải gọi lại provider đã từ
// chối); provider bị từ chối chỉ nhận đúng 1 request.
func TestModel1113FailoverDoesNotRecallRejectedProvider(t *testing.T) {
	rejectSrv, rejectHits := providerStub(t, http.StatusTooManyRequests, body1113)
	okSrv, okHits := providerStub(t, http.StatusOK, bodyOK)
	fallback := glmModel(t, "zai", okSrv.URL)
	set := &ModelSet{fallbacks: map[string][]modelTarget{
		"writer": {{provider: "zai", name: "glm-4.7", model: fallback}},
	}}
	fm := &failoverModel{
		role:    "writer",
		primary: NewSwappableModel("zai-coding", "glm-4.7", glmModel(t, "zai-coding", rejectSrv.URL), nil),
		set:     set,
	}
	if _, err := fm.Generate(context.Background(), []agentcore.Message{agentcore.UserMsg("hi")}, nil); err != nil {
		t.Fatalf("fallback phải trả lời được: %v", err)
	}
	if got := rejectHits.Load(); got != 1 {
		t.Fatalf("provider từ chối chỉ được gọi 1 lần, got %d", got)
	}
	if got := okHits.Load(); got != 1 {
		t.Fatalf("fallback được gọi %d lần, want 1", got)
	}
}

// Bọc model không được làm mất các interface tuỳ chọn mà ModelSet/llmcontract dựa vào.
func TestRejectingModelKeepsOptionalInterfaces(t *testing.T) {
	m := glmModel(t, "zai-coding", "http://127.0.0.1:1")
	if ModelName(m) != "glm-4.7" {
		t.Fatalf("ModelName = %q", ModelName(m))
	}
	if _, ok := m.(interface{ ProviderName() string }); !ok {
		t.Fatal("mất ProviderName")
	}
	if _, ok := m.(llm.CapabilityProvider); !ok {
		t.Fatal("mất Capabilities")
	}
	if _, ok := m.(interface{ Info() llm.ModelInfo }); !ok {
		t.Fatal("mất Info")
	}
}

// Huỷ giữa stream: event lỗi cuối (context.Canceled) phải luôn tới bên đọc, nếu
// không agentcore hiểu nhầm là stream đứt và báo "thử lại" giả.
func TestRejectingModelStreamKeepsCancelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"id\":\"s\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"a\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	m := glmModel(t, "zai", srv.URL)
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := m.GenerateStream(ctx, []agentcore.Message{agentcore.UserMsg("hi")}, nil)
		if err != nil {
			cancel()
			t.Fatalf("GenerateStream: %v", err)
		}
		var last agentcore.StreamEvent
		first := true
		for ev := range ch {
			if first {
				cancel()
				first = false
			}
			last = ev
		}
		cancel()
		if last.Type != agentcore.StreamEventError || !errors.Is(last.Err, context.Canceled) {
			t.Fatalf("lần %d: event cuối = %+v, muốn lỗi context.Canceled", i, last)
		}
	}
}
