package api

// BookSnapshot là trạng thái tổng hợp của engine mà UI cần. Không chứa dàn ý,
// nhân vật hay tóm tắt (nặng) — các phần đó phục vụ qua /outline.
type BookSnapshot struct {
	BookTitle         string          `json:"book_title"`
	Provider          string          `json:"provider"`
	ModelName         string          `json:"model_name"`
	Phase             string          `json:"phase"`
	Flow              string          `json:"flow"`
	RuntimeState      string          `json:"runtime_state"`
	IsRunning         bool            `json:"is_running"`
	StatusLabel       string          `json:"status_label"`
	CurrentChapter    int             `json:"current_chapter"`
	TotalChapters     int             `json:"total_chapters"`
	CompletedCount    int             `json:"completed_count"`
	TotalWordCount    int             `json:"total_word_count"`
	InProgressChapter int             `json:"in_progress_chapter"`
	AdvanceMode       string          `json:"advance_mode"`
	HasAdvanceHold    bool            `json:"has_advance_hold"`
	AdvanceHoldReason string          `json:"advance_hold_reason"`
	PendingSteer      string          `json:"pending_steer"`
	RecoveryLabel     string          `json:"recovery_label"`
	Agents            []AgentSnapshot `json:"agents"`
	TotalCostUSD      float64         `json:"total_cost_usd"`
	BudgetLimitUSD    float64         `json:"budget_limit_usd"`
	TotalInputTokens  int             `json:"total_input_tokens"`
	TotalOutputTokens int             `json:"total_output_tokens"`
}

// AgentSnapshot là trạng thái rút gọn của một agent (architect/writer/editor).
type AgentSnapshot struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Summary string `json:"summary"`
	Tool    string `json:"tool"`
}
