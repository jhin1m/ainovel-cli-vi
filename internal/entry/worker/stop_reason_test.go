package worker

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

func TestStopReason(t *testing.T) {
	review := func(mut func(*host.UISnapshot)) host.UISnapshot {
		s := host.UISnapshot{AdvanceMode: "review", Phase: "writing", BudgetLimitUSD: 5, TotalCostUSD: 1}
		if mut != nil {
			mut(&s)
		}
		return s
	}
	cases := []struct {
		name         string
		snap         host.UISnapshot
		aborted      bool
		holdConsumed bool
		want         string
	}{
		{"chờ duyệt", review(nil), false, false, api.StopReviewWait},
		{"chờ duyệt sau dàn ý (chưa có chương)", review(func(s *host.UISnapshot) { s.CompletedCount = 0 }), false, false, api.StopReviewWait},
		{"review nhưng đang chạy", review(func(s *host.UISnapshot) { s.IsRunning = true }), false, false, api.StopError},
		{"review ở giai đoạn dàn ý", review(func(s *host.UISnapshot) { s.Phase = "outline" }), false, false, api.StopError},
		{"review dừng giữa chương (lỗi)", review(func(s *host.UISnapshot) { s.InProgressChapter = 3 }), false, false, api.StopError},
		{"review dừng còn hàng viết lại", review(func(s *host.UISnapshot) { s.PendingRewrites = []int{2} }), false, false, api.StopError},
		{"review dừng còn giấy phép (lỗi gate)", review(func(s *host.UISnapshot) { s.AdvancePermitChapter = 4 }), false, false, api.StopError},
		{"còn can thiệp treo", review(func(s *host.UISnapshot) { s.PendingSteer = "x" }), false, false, api.StopPendingSteer},
		{"hold bị gate tiêu thụ ở auto", host.UISnapshot{AdvanceMode: "auto", Phase: "writing"}, false, true, api.StopHold},
		{"hold bị tiêu thụ ở review", review(nil), false, true, api.StopHold},
		{"hold còn nguyên khi dừng", review(func(s *host.UISnapshot) { s.HasAdvanceHold = true }), false, false, api.StopHold},
		{"hết ngân sách", review(func(s *host.UISnapshot) { s.TotalCostUSD = 5 }), false, false, api.StopBudget},
		{"vượt ngân sách dù user dừng", review(func(s *host.UISnapshot) { s.TotalCostUSD = 6 }), true, false, api.StopBudget},
		{"ngân sách tắt", host.UISnapshot{AdvanceMode: "auto", Phase: "writing", TotalCostUSD: 9}, false, false, api.StopError},
		{"hoàn thành", review(func(s *host.UISnapshot) { s.Phase = "complete"; s.TotalCostUSD = 5 }), false, true, api.StopComplete},
		{"user dừng ở review", review(nil), true, false, api.StopUser},
		{"user dừng ở auto", host.UISnapshot{AdvanceMode: "auto", Phase: "writing"}, true, false, api.StopUser},
		{"auto dừng không rõ", host.UISnapshot{AdvanceMode: "auto", Phase: "writing"}, false, false, api.StopError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stopReason(tc.snap, tc.aborted, tc.holdConsumed); got != tc.want {
				t.Fatalf("stopReason = %q, muốn %q", got, tc.want)
			}
		})
	}
}
