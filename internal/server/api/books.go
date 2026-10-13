package api

import "time"

// BookStatus là trạng thái của truyện trong DB (books.status).
type BookStatus string

const (
	StatusQueued          BookStatus = "queued"
	StatusRunning         BookStatus = "running"
	StatusWaitingReview   BookStatus = "waiting_review"
	StatusPaused          BookStatus = "paused"
	StatusBudgetExhausted BookStatus = "budget_exhausted"
	StatusCompleted       BookStatus = "completed"
	StatusFailed          BookStatus = "failed"
	StatusInterrupted     BookStatus = "interrupted"
	StatusDeleting        BookStatus = "deleting"
)

// Book là một truyện của người dùng.
type Book struct {
	ID         string     `json:"id"`
	Prompt     string     `json:"prompt"`
	Status     BookStatus `json:"status"`
	StopReason string     `json:"stop_reason"`
	Provider   string     `json:"provider"`
	Model      string     `json:"model"`
	APIKeyID   string     `json:"api_key_id"` // rỗng nếu key đã bị xoá
	BudgetUSD  float64    `json:"budget_usd"`
	// Subscription: provider là gói thuê bao (suy ra từ provider, không lưu DB);
	// budget_usd luôn 0 và chi phí USD không áp dụng.
	Subscription bool          `json:"subscription"`
	AdvanceMode  string        `json:"advance_mode"`
	PendingText  string        `json:"pending_text"` // can thiệp đang chờ lượt chạy kế tiếp
	LastError    string        `json:"last_error"`
	Snapshot     *BookSnapshot `json:"snapshot"` // nil khi engine chưa chạy lần nào
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// CreateBookRequest: POST /api/books.
type CreateBookRequest struct {
	Prompt      string  `json:"prompt"`
	APIKeyID    string  `json:"api_key_id"`
	Model       string  `json:"model"`
	BudgetUSD   float64 `json:"budget_usd"`
	AdvanceMode string  `json:"advance_mode"`
}

// UpdateBookRequest: PATCH /api/books/{id}. Field nil = giữ nguyên.
type UpdateBookRequest struct {
	BudgetUSD   *float64 `json:"budget_usd,omitempty"`
	AdvanceMode *string  `json:"advance_mode,omitempty"`
	APIKeyID    *string  `json:"api_key_id,omitempty"`
}

// SteerRequest: POST /api/books/{id}/steer.
type SteerRequest struct {
	Text string `json:"text"`
}

// CommandResult là kết quả lệnh gửi cho truyện. Queued=true: lệnh sẽ chạy ở lượt
// worker kế tiếp; Pending=true: đã gửi nhưng chưa có xác nhận (kết quả đến qua SSE).
type CommandResult struct {
	Book    Book `json:"book"`
	Queued  bool `json:"queued"`
	Pending bool `json:"pending"`
}

// ChapterInfo là một chương đã chốt.
type ChapterInfo struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	WordCount int    `json:"word_count"`
}

// ChapterText: GET /api/books/{id}/chapters/{n}. Content là markdown do LLM sinh
// — dữ liệu không tin cậy, client không được render HTML thô.
type ChapterText struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Prev    int    `json:"prev"` // 0 = không có
	Next    int    `json:"next"`
}

// Outline: GET /api/books/{id}/outline.
type Outline struct {
	Title      string             `json:"title"`
	Synopsis   string             `json:"synopsis"`
	Volumes    []OutlineVolume    `json:"volumes"`
	Characters []OutlineCharacter `json:"characters"`
}

// OutlineVolume gom chương theo quyển/cung; truyện không phân tầng có một quyển rỗng tiêu đề.
type OutlineVolume struct {
	Title    string           `json:"title"`
	Chapters []OutlineChapter `json:"chapters"`
}

type OutlineChapter struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	CoreEvent string `json:"core_event"`
	Done      bool   `json:"done"`
}

type OutlineCharacter struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Description string `json:"description"`
}

// Kiểu message SSE trên /api/books/{id}/events (trường event: của SSE).
const (
	SSESnapshot = "snapshot"
	SSEEvent    = "event"
	SSEStream   = "stream"
	SSEStatus   = "status"
)

// StreamDelta là message SSE "stream": văn bản đang sinh. Clear=true: bắt đầu lượt mới.
type StreamDelta struct {
	Delta string `json:"delta,omitempty"`
	Clear bool   `json:"clear,omitempty"`
}

// StatusChange là message SSE "status": trạng thái truyện vừa đổi.
type StatusChange struct {
	Status     BookStatus `json:"status"`
	StopReason string     `json:"stop_reason"`
	LastError  string     `json:"last_error"`
}
