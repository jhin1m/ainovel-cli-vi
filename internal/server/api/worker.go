// Package api chứa wire types dùng chung giữa server web, process worker và SPA.
// Package chỉ có struct + json tag, không phụ thuộc package nội bộ nào khác để
// tygo sinh được type TypeScript và để thay đổi engine không lan sang giao thức.
package api

// WorkerProtocol là phiên bản giao thức JSON Lines giữa server và worker.
// Tăng số này mỗi khi đổi ngữ nghĩa message; server từ chối worker lệch phiên bản.
const WorkerProtocol = 1

// Action của message init: worker chạy đúng một lượt cho action này rồi thoát.
const (
	ActionStart   = "start"
	ActionResume  = "resume"
	ActionSteer   = "steer"
	ActionAdvance = "advance"
)

// Kiểu message server → worker (stdin).
const (
	MsgInit           = "init"
	MsgSteer          = "steer"
	MsgSetAdvanceMode = "set_advance_mode"
	MsgAbort          = "abort"
	MsgShutdown       = "shutdown"
)

// Kiểu message worker → server (stdout).
const (
	MsgHello    = "hello"
	MsgEvent    = "event"
	MsgStream   = "stream"
	MsgSnapshot = "snapshot"
	MsgAck      = "ack"
	MsgError    = "error"
	MsgExit     = "exit"
)

// Lý do process worker thoát (exit.reason).
const (
	ExitStopped  = "stopped"
	ExitShutdown = "shutdown"
	ExitError    = "error"
)

// Lý do engine dừng (exit.stop_reason). Server map sang books.status.
const (
	StopReviewWait      = "review_wait"
	StopBudget          = "budget"
	StopComplete        = "complete"
	StopHold            = "hold"
	StopUser            = "user"
	StopNothingToResume = "nothing_to_resume"
	StopPendingSteer    = "pending_steer" // còn can thiệp chưa xử lý: server phải resume, không advance
	StopError           = "error"
)

// Mã lỗi trong message error của worker.
const (
	ErrCodeProtocol        = "protocol"
	ErrCodeConfigInvalid   = "config_invalid"
	ErrCodeLockBusy        = "lock_busy"
	ErrCodeBudgetExhausted = "budget_exhausted"
	ErrCodeModelUnpriced   = "model_unpriced"
	ErrCodeEngine          = "engine"
	ErrCodeBusy            = "busy"
)

// AdvanceMode của truyện: auto chạy liên tục, review dừng sau mỗi chương chờ duyệt.
const (
	AdvanceAuto   = "auto"
	AdvanceReview = "review"
)

// WorkerInit là message đầu tiên server gửi qua stdin. Chứa API key rõ nên
// không bao giờ được log hay ghi xuống đĩa.
type WorkerInit struct {
	Type        string        `json:"type"`
	Protocol    int           `json:"protocol"`
	Action      string        `json:"action"`
	Prompt      string        `json:"prompt,omitempty"`
	Text        string        `json:"text,omitempty"`
	BookDir     string        `json:"book_dir"`
	AdvanceMode string        `json:"advance_mode"`
	LLM         WorkerLLM     `json:"llm"`
	Pricing     *ModelPricing `json:"pricing"`
	Language    string        `json:"language,omitempty"`
	Style       string        `json:"style,omitempty"`
	BudgetUSD   float64       `json:"budget_usd"`
}

// WorkerLLM là lựa chọn model của truyện. Không có base_url/type/extra: provider
// chỉ lấy từ allowlist biên dịch sẵn trong worker.
type WorkerLLM struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	APIKey          string `json:"api_key"`
}

// ModelPricing là giá của model đã chọn (USD / 1M token), server lấy từ registry
// của mình để ngân sách trong worker không bao giờ tính ra $0.
type ModelPricing struct {
	Provider            string  `json:"provider"`
	ID                  string  `json:"id"`
	Name                string  `json:"name,omitempty"`
	ContextWindow       int     `json:"context_window,omitempty"`
	MaxTokens           int     `json:"max_tokens,omitempty"`
	InputCostPer1M      float64 `json:"input_cost_per_1m"`
	OutputCostPer1M     float64 `json:"output_cost_per_1m"`
	CacheReadCostPer1M  float64 `json:"cache_read_cost_per_1m,omitempty"`
	CacheWriteCostPer1M float64 `json:"cache_write_cost_per_1m,omitempty"`
}

// WorkerCommand là lệnh runtime server gửi trong lúc worker đang chạy.
type WorkerCommand struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Text string `json:"text,omitempty"`
	Mode string `json:"mode,omitempty"`
}

// WorkerMessage là một dòng JSON worker ghi ra stdout. Field nào dùng tuỳ Type.
type WorkerMessage struct {
	Type string `json:"type"`

	// hello
	Protocol int    `json:"protocol,omitempty"`
	Version  string `json:"version,omitempty"`

	// event
	Event *Event `json:"event,omitempty"`

	// stream
	Delta string `json:"delta,omitempty"`
	Clear bool   `json:"clear,omitempty"`

	// snapshot
	Snapshot *BookSnapshot `json:"snapshot,omitempty"`

	// ack (ok = error rỗng)
	ID string `json:"id,omitempty"`
	OK bool   `json:"ok,omitempty"`

	// error
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`

	// exit
	Reason     string `json:"reason,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	Error      string `json:"error,omitempty"`
}
