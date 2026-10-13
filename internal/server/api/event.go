package api

import "time"

// Event là một dòng trên timeline của truyện. Lời gọi MODEL/TOOL/DISPATCH/DECISION
// phát hai lần với cùng ID (bắt đầu: finished=false, kết thúc: finished=true);
// client và hub upsert theo ID. ID luôn khác rỗng và duy nhất giữa các lượt worker.
type Event struct {
	ID         string    `json:"id"`
	Time       time.Time `json:"time"`
	Category   string    `json:"category"`
	Agent      string    `json:"agent,omitempty"`
	Summary    string    `json:"summary"`
	Detail     string    `json:"detail,omitempty"`
	Kind       string    `json:"kind,omitempty"`
	Level      string    `json:"level,omitempty"`
	Depth      int       `json:"depth"`
	DurationMS int64     `json:"duration_ms"`
	Failed     bool      `json:"failed"`
	Finished   bool      `json:"finished"`
}
