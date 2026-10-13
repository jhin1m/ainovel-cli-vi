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
		name    string
		snap    host.UISnapshot
		aborted bool
		want    string
	}{
		{"chờ duyệt", review(nil), false, api.StopReviewWait},
		{"chờ duyệt sau dàn ý (chưa có chương)", review(func(s *host.UISnapshot) { s.CompletedCount = 0 }), false, api.StopReviewWait},
		{"review nhưng đang chạy", review(func(s *host.UISnapshot) { s.IsRunning = true }), false, api.StopError},
		{"review ở giai đoạn dàn ý", review(func(s *host.UISnapshot) { s.Phase = "outline" }), false, api.StopError},
		{"review có hold", review(func(s *host.UISnapshot) { s.HasAdvanceHold = true }), false, api.StopHold},
		{"hold ở auto", host.UISnapshot{AdvanceMode: "auto", Phase: "writing", HasAdvanceHold: true}, false, api.StopHold},
		{"hết ngân sách", review(func(s *host.UISnapshot) { s.TotalCostUSD = 5 }), false, api.StopBudget},
		{"vượt ngân sách dù user dừng", review(func(s *host.UISnapshot) { s.TotalCostUSD = 6 }), true, api.StopBudget},
		{"ngân sách tắt", host.UISnapshot{AdvanceMode: "auto", Phase: "writing", TotalCostUSD: 9}, false, api.StopError},
		{"hoàn thành", review(func(s *host.UISnapshot) { s.Phase = "complete"; s.TotalCostUSD = 5 }), false, api.StopComplete},
		{"user dừng ở review", review(nil), true, api.StopUser},
		{"user dừng ở auto", host.UISnapshot{AdvanceMode: "auto", Phase: "writing"}, true, api.StopUser},
		{"auto dừng không rõ", host.UISnapshot{AdvanceMode: "auto", Phase: "writing"}, false, api.StopError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stopReason(tc.snap, tc.aborted); got != tc.want {
				t.Fatalf("stopReason = %q, muốn %q", got, tc.want)
			}
		})
	}
}
