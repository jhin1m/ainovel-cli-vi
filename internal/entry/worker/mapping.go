package worker

import (
	"fmt"

	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// eventMapper gắn tiền tố lượt chạy vào ID event: ID của host ("e1", "e2"…) đếm
// lại từ đầu mỗi process, còn hub của server giữ timeline qua nhiều lượt worker.
// Event không có ID (SYSTEM/ERROR…) được cấp ID riêng để client upsert đồng nhất.
type eventMapper struct {
	runID string
	seq   uint64
}

func (m *eventMapper) mapEvent(ev host.Event) api.Event {
	id := ev.ID
	if id == "" {
		m.seq++
		id = fmt.Sprintf("n%d", m.seq)
	}
	return api.Event{
		ID:         m.runID + ":" + id,
		Time:       ev.Time.UTC(),
		Category:   ev.Category,
		Agent:      ev.Agent,
		Summary:    ev.Summary,
		Detail:     ev.Detail,
		Kind:       ev.Kind,
		Level:      ev.Level,
		Depth:      ev.Depth,
		DurationMS: ev.Duration.Milliseconds(),
		Failed:     ev.Failed,
		Finished:   !ev.Running(),
	}
}

// mapSnapshot chuyển snapshot của engine sang DTO. Engine tính chi phí theo giá
// registry kể cả với gói thuê bao, nên subscription=true đặt chi phí và trần
// ngân sách về 0 (token giữ nguyên) để server không lưu chi phí USD giả.
func mapSnapshot(s host.UISnapshot, subscription bool) api.BookSnapshot {
	agents := make([]api.AgentSnapshot, 0, len(s.Agents))
	for _, a := range s.Agents {
		agents = append(agents, api.AgentSnapshot{Name: a.Name, State: a.State, Summary: a.Summary, Tool: a.Tool})
	}
	cost, limit := s.TotalCostUSD, s.BudgetLimitUSD
	if subscription {
		cost, limit = 0, 0
	}
	return api.BookSnapshot{
		BookTitle:         s.BookTitle,
		Provider:          s.Provider,
		ModelName:         s.ModelName,
		Phase:             s.Phase,
		Flow:              s.Flow,
		RuntimeState:      s.RuntimeState,
		IsRunning:         s.IsRunning,
		StatusLabel:       s.StatusLabel,
		CurrentChapter:    s.CurrentChapter,
		TotalChapters:     s.TotalChapters,
		CompletedCount:    s.CompletedCount,
		TotalWordCount:    s.TotalWordCount,
		InProgressChapter: s.InProgressChapter,
		AdvanceMode:       s.AdvanceMode,
		HasAdvanceHold:    s.HasAdvanceHold,
		AdvanceHoldReason: s.AdvanceHoldReason,
		PendingSteer:      s.PendingSteer,
		RecoveryLabel:     s.RecoveryLabel,
		Agents:            agents,
		TotalCostUSD:      cost,
		BudgetLimitUSD:    limit,
		TotalInputTokens:  s.TotalInputTokens,
		TotalOutputTokens: s.TotalOutputTokens,
	}
}
