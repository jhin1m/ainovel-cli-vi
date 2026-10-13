// Package llmretry 是直接模型调用共用的请求层重试内核：仅重试模型适配器
// 明确标记为 retryable 的错误，遵守 Retry-After/指数退避，并经 ToolProgress
// 把进度送入既有工作台观察链。账户、鉴权、权限等终止错误会立即返回；
// retryable 错误默认持续重试，生命周期只由 context 控制（CLI/TUI）。
// 进程可经 SetLimits 设定重试次数与总时长上限，到顶后返回最后一次错误：
// web serve 的 worker 用它避免 key 长期 429 时一直占着 job 槽位。
package llmretry

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/voocel/agentcore"
)

const maxRetryDelay = 60 * time.Second

// baseRetryDelay 与 now 仅供测试替换。
var (
	baseRetryDelay = time.Second
	now            = time.Now
)

// Limits 是进程级的重试上限；零值字段表示不限。
type Limits struct {
	MaxRetries int           // 最多重试次数（不含首次调用）
	MaxElapsed time.Duration // 自首次调用起的总时长
}

var limits atomic.Pointer[Limits]

// SetLimits 设定此后所有 Generate 调用的重试上限。零值 Limits 恢复不限。
func SetLimits(l Limits) {
	limits.Store(&l)
}

func currentLimits() Limits {
	if l := limits.Load(); l != nil {
		return *l
	}
	return Limits{}
}

// Generator 是请求重试所需的最小模型接口。
type Generator interface {
	Generate(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (*agentcore.LLMResponse, error)
}

// Event 描述一次即将发生的请求重试。
type Event struct {
	Attempt int
	Delay   time.Duration
	Err     error
}

// Config 配置重试的可观测信息，不改变重试语义。
type Config struct {
	Agent   string
	OnRetry func(Event)
}

// Generate 调用 model.Generate。retryable 错误退避后重试，直到成功、context
// 结束或触及 SetLimits 的上限（返回最后一次错误）；非 retryable 错误立即返回。
func Generate(ctx context.Context, model Generator, cfg Config, messages []agentcore.Message, opts ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	lim := currentLimits()
	start := now()
	for retry := 1; ; retry++ {
		resp, err := model.Generate(ctx, messages, nil, opts...)
		if err == nil {
			return resp, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || !isRetryable(err) {
			return nil, err
		}

		delay := retryDelay(err, retry-1)
		if lim.MaxRetries > 0 && retry > lim.MaxRetries {
			return nil, err
		}
		if lim.MaxElapsed > 0 && now().Sub(start)+delay >= lim.MaxElapsed {
			return nil, err
		}
		event := Event{Attempt: retry, Delay: delay, Err: err}
		if cfg.OnRetry != nil {
			cfg.OnRetry(event)
		}
		meta, _ := json.Marshal(struct {
			DelayMS int64 `json:"retry_delay_ms"`
		}{DelayMS: delay.Milliseconds()})
		agentcore.ReportToolProgress(ctx, agentcore.ProgressPayload{
			Kind:    agentcore.ProgressRetry,
			Agent:   cfg.Agent,
			Attempt: retry,
			Message: err.Error(),
			Meta:    meta,
		})
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func isRetryable(err error) bool {
	var retryable agentcore.RetryableError
	return errors.As(err, &retryable) && retryable.Retryable()
}

func retryDelay(err error, attempt int) time.Duration {
	var hinter agentcore.RetryHinter
	if errors.As(err, &hinter) {
		if delay := hinter.RetryAfter(); delay > 0 {
			if delay > maxRetryDelay {
				return maxRetryDelay
			}
			return delay
		}
	}
	delay := baseRetryDelay
	for i := 0; i < attempt && delay < maxRetryDelay; i++ {
		delay *= 2
	}
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}
