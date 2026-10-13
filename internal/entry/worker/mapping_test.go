package worker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/host"
)

// Golden: đổi wire format là đổi hợp đồng với server và SPA — phải sửa có chủ đích.
func TestMapEventGolden(t *testing.T) {
	m := eventMapper{runID: "r1"}
	at := time.Date(2026, 10, 13, 7, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	running := m.mapEvent(host.Event{ID: "e7", Time: at, Category: "MODEL", Agent: "writer", Summary: "gọi model", Level: "info", Depth: 1})
	done := m.mapEvent(host.Event{ID: "e7", Time: at, FinishedAt: at.Add(2 * time.Second), Category: "MODEL", Agent: "writer",
		Summary: "gọi model", Detail: "chi tiết", Kind: "stream_idle", Level: "error", Depth: 1, Duration: 2 * time.Second, Failed: true})
	system := m.mapEvent(host.Event{Time: at, Category: "SYSTEM", Summary: "Bắt đầu", Level: "info"})

	cases := []struct {
		got  any
		want string
	}{
		{running, `{"id":"r1:e7","time":"2026-10-13T00:00:00Z","category":"MODEL","agent":"writer","summary":"gọi model","level":"info","depth":1,"duration_ms":0,"failed":false,"finished":false}`},
		{done, `{"id":"r1:e7","time":"2026-10-13T00:00:00Z","category":"MODEL","agent":"writer","summary":"gọi model","detail":"chi tiết","kind":"stream_idle","level":"error","depth":1,"duration_ms":2000,"failed":true,"finished":true}`},
		{system, `{"id":"r1:n1","time":"2026-10-13T00:00:00Z","category":"SYSTEM","summary":"Bắt đầu","level":"info","depth":0,"duration_ms":0,"failed":false,"finished":true}`},
	}
	for _, tc := range cases {
		b, _ := json.Marshal(tc.got)
		if string(b) != tc.want {
			t.Fatalf("event JSON lệch golden\n got: %s\nwant: %s", b, tc.want)
		}
	}
}

func TestMapSnapshotGolden(t *testing.T) {
	snap := mapSnapshot(host.UISnapshot{
		Provider: "openrouter", BookTitle: "Kiếm Đạo", ModelName: "m", RuntimeState: "running", IsRunning: true,
		StatusLabel: "Đang viết", Phase: "writing", Flow: "writing", CurrentChapter: 3, TotalChapters: 30, CompletedCount: 2,
		TotalWordCount: 9000, InProgressChapter: 3, AdvanceMode: "review", HasAdvanceHold: true, AdvanceHoldReason: "lý do",
		PendingSteer: "x", RecoveryLabel: "Khôi phục", TotalCostUSD: 0.5, BudgetLimitUSD: 5,
		TotalInputTokens: 100, TotalOutputTokens: 50,
		Agents:   []host.AgentSnapshot{{Name: "writer", State: "working", Summary: "viết", Tool: "draft_chapter", Turn: 3}},
		Outline:  []host.OutlineSnapshot{{Chapter: 1, Title: "không được lộ"}},
		Premise:  "nặng",
		Synopsis: "nặng",
	}, false)
	b, _ := json.Marshal(snap)
	want := `{"book_title":"Kiếm Đạo","provider":"openrouter","model_name":"m","phase":"writing","flow":"writing","runtime_state":"running","is_running":true,"status_label":"Đang viết","current_chapter":3,"total_chapters":30,"completed_count":2,"total_word_count":9000,"in_progress_chapter":3,"advance_mode":"review","has_advance_hold":true,"advance_hold_reason":"lý do","pending_steer":"x","recovery_label":"Khôi phục","agents":[{"name":"writer","state":"working","summary":"viết","tool":"draft_chapter"}],"total_cost_usd":0.5,"budget_limit_usd":5,"total_input_tokens":100,"total_output_tokens":50}`
	if string(b) != want {
		t.Fatalf("snapshot JSON lệch golden\n got: %s\nwant: %s", b, want)
	}
}

func TestMapSnapshotSubscriptionZeroesCost(t *testing.T) {
	snap := mapSnapshot(host.UISnapshot{
		Provider: "zai-coding", ModelName: "glm-4.6", TotalCostUSD: 1.25, BudgetLimitUSD: 3,
		TotalInputTokens: 1000, TotalOutputTokens: 400,
	}, true)
	if snap.TotalCostUSD != 0 || snap.BudgetLimitUSD != 0 {
		t.Fatalf("thuê bao phải báo chi phí và trần ngân sách bằng 0: %+v", snap)
	}
	if snap.TotalInputTokens != 1000 || snap.TotalOutputTokens != 400 {
		t.Fatalf("token phải giữ nguyên: %+v", snap)
	}
}
