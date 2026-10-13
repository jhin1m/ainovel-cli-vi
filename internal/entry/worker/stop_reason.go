package worker

import (
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// budgetExhausted báo chi phí đã chạm trần ngân sách đang bật.
func budgetExhausted(s host.UISnapshot) bool {
	return s.BudgetLimitUSD > 0 && s.TotalCostUSD >= s.BudgetLimitUSD
}

// stopReason suy ra vì sao engine dừng, từ snapshot cuối và việc người dùng có
// bấm dừng hay không. Thứ tự ưu tiên: hoàn thành > hết ngân sách > người dùng >
// ý định tạm dừng của can thiệp > chờ duyệt > lỗi.
//
// Điều kiện chờ duyệt khớp đúng điều kiện TUI dùng (panels_sidebar.go):
// review && !IsRunning && Phase=="writing", cộng thêm không có AdvanceHold.
func stopReason(s host.UISnapshot, userAborted bool) string {
	switch {
	case s.Phase == "complete":
		return api.StopComplete
	case budgetExhausted(s):
		return api.StopBudget
	case userAborted:
		return api.StopUser
	case s.HasAdvanceHold:
		return api.StopHold
	case s.AdvanceMode == api.AdvanceReview && !s.IsRunning && s.Phase == "writing":
		return api.StopReviewWait
	default:
		return api.StopError
	}
}
