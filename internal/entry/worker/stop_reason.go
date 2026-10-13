package worker

import (
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// budgetExhausted báo chi phí đã chạm trần ngân sách đang bật.
func budgetExhausted(s host.UISnapshot) bool {
	return s.BudgetLimitUSD > 0 && s.TotalCostUSD >= s.BudgetLimitUSD
}

// stopReason suy ra vì sao engine dừng từ snapshot cuối và các tín hiệu worker
// quan sát được trong lượt chạy. Chỉ dùng tín hiệu cấu trúc (không so chuỗi
// thông báo) để không lệ thuộc ngôn ngữ hay câu chữ của engine.
//
//   - holdConsumed: trong lượt chạy đã thấy AdvanceHold, giờ không còn — gate
//     tiêu thụ hold rồi mới dừng (advance_gate.go), nên snapshot cuối không có hold.
//   - review_wait: điều kiện của TUI (review && !IsRunning && Phase=="writing")
//     cộng thêm: không còn chương viết dở, hàng viết lại, giấy phép chương hay can
//     thiệp treo — tức gate đang chờ cho phép chương kế tiếp, không phải dừng vì lỗi.
func stopReason(s host.UISnapshot, userAborted, holdConsumed bool) string {
	switch {
	case s.Phase == "complete":
		return api.StopComplete
	case budgetExhausted(s):
		return api.StopBudget
	case userAborted:
		return api.StopUser
	case holdConsumed || s.HasAdvanceHold:
		return api.StopHold
	case s.PendingSteer != "":
		return api.StopPendingSteer
	case s.AdvanceMode == api.AdvanceReview && !s.IsRunning && s.Phase == "writing" &&
		s.InProgressChapter == 0 && len(s.PendingRewrites) == 0 && s.AdvancePermitChapter == 0:
		return api.StopReviewWait
	default:
		return api.StopError
	}
}
