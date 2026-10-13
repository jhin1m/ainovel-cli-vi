package llmretry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/voocel/agentcore"
)

func TestRetryDelayCapsWithoutOverflow(t *testing.T) {
	if got := retryDelay(nil, 10_000); got != maxRetryDelay {
		t.Fatalf("retryDelay = %s, want %s", got, maxRetryDelay)
	}
	if got := retryDelay(nil, 3); got != 8*time.Second {
		t.Fatalf("retryDelay = %s, want 8s", got)
	}
}

type retryableErr struct{}

func (retryableErr) Error() string   { return "429 rate limited" }
func (retryableErr) Retryable() bool { return true }

type countingModel struct {
	calls  int
	onCall func(calls int)
	err    error
}

func (m *countingModel) Generate(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	m.calls++
	if m.onCall != nil {
		m.onCall(m.calls)
	}
	return nil, m.err
}

// fastRetries thu nhỏ backoff và đặt lại trần sau test để các test khác không bị ảnh hưởng.
func fastRetries(t *testing.T) {
	t.Helper()
	prevBase, prevNow := baseRetryDelay, now
	baseRetryDelay = time.Microsecond
	t.Cleanup(func() {
		baseRetryDelay, now = prevBase, prevNow
		SetLimits(Limits{})
	})
}

func TestGenerateWithoutLimitsRetriesUntilContextEnds(t *testing.T) {
	fastRetries(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &countingModel{err: retryableErr{}}
	m.onCall = func(calls int) {
		if calls == 15 {
			cancel()
		}
	}
	_, err := Generate(ctx, m, Config{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if m.calls != 15 {
		t.Fatalf("không có trần thì phải retry tới khi context kết thúc: calls = %d", m.calls)
	}
}

func TestGenerateStopsAtMaxRetries(t *testing.T) {
	fastRetries(t)
	SetLimits(Limits{MaxRetries: 10, MaxElapsed: 10 * time.Minute})
	m := &countingModel{err: retryableErr{}}
	_, err := Generate(context.Background(), m, Config{}, nil)
	if !errors.Is(err, retryableErr{}) {
		t.Fatalf("hết trần phải trả lỗi cuối, got %v", err)
	}
	if m.calls != 11 {
		t.Fatalf("10 lần retry = 11 lần gọi, got %d", m.calls)
	}
}

func TestGenerateStopsAtMaxElapsed(t *testing.T) {
	fastRetries(t)
	SetLimits(Limits{MaxRetries: 10, MaxElapsed: 10 * time.Minute})
	clock := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	now = func() time.Time { return clock }
	m := &countingModel{err: retryableErr{}}
	m.onCall = func(int) { clock = clock.Add(4 * time.Minute) }
	_, err := Generate(context.Background(), m, Config{}, nil)
	if !errors.Is(err, retryableErr{}) {
		t.Fatalf("hết thời gian phải trả lỗi cuối, got %v", err)
	}
	if m.calls != 3 {
		t.Fatalf("mỗi lần gọi tốn 4 phút, trần 10 phút → 3 lần gọi, got %d", m.calls)
	}
}

func TestGenerateNonRetryableReturnsImmediately(t *testing.T) {
	fastRetries(t)
	m := &countingModel{err: errors.New("401 unauthorized")}
	if _, err := Generate(context.Background(), m, Config{}, nil); err == nil {
		t.Fatal("phải trả lỗi")
	}
	if m.calls != 1 {
		t.Fatalf("lỗi không retry được chỉ gọi 1 lần, got %d", m.calls)
	}
}
