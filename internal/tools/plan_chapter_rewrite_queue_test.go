package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

// rewriteQueueStore 建一份写完 1-3 章、返工队列为 queue 的 Store；
// InProgressChapter 指向队首，模拟 Engine 派发 writer 时的预标。
func rewriteQueueStore(t *testing.T, queue []int) *store.Store {
	t.Helper()
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Init(5); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	for ch := 1; ch <= 3; ch++ {
		if err := st.Progress.MarkChapterComplete(ch, 3000, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}
	p, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	p.Flow = domain.FlowRewriting
	p.PendingRewrites = queue
	p.InProgressChapter = queue[0]
	if err := st.Progress.Save(p); err != nil {
		t.Fatal(err)
	}
	return st
}

func rewritePlanArgs(chapter int, goal string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"chapter": chapter, "title": "Chương viết lại", "goal": goal,
		"conflict": "Áp lực từ tông môn", "hook": "Lá thư chưa mở", "emotion_arc": "Căng thẳng tới quyết tâm",
	})
	return b
}

// 返工队列里的已完成章节必须能落一份新的 chapter_contract：revise_outline 只能改
// 未发生的章节、写作期禁止全量覆盖大纲，plan_chapter 若也拒绝，就没有任何工具能
// 写下"这一章该重写成什么样"，Writer 只能拿着旧规划重写。
func TestPlanChapterAcceptsChapterQueuedForRewrite(t *testing.T) {
	st := rewriteQueueStore(t, []int{2})
	tool := NewPlanChapterTool(st)
	res, err := tool.Execute(context.Background(), rewritePlanArgs(2, "Để Lê An tự chọn rời tông môn thay vì bị đuổi"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out map[string]any
	_ = json.Unmarshal(res, &out)
	if out["skipped"] == true || out["planned"] != true {
		t.Fatalf("chương trong hàng chờ viết lại phải được lập kế hoạch, nhận %+v", out)
	}
	plan, err := st.Drafts.LoadChapterPlan(2)
	if err != nil || plan == nil || plan.Goal != "Để Lê An tự chọn rời tông môn thay vì bị đuổi" {
		t.Fatalf("kế hoạch viết lại phải được lưu, nhận %+v (err=%v)", plan, err)
	}
}

// 为队列后面的章写指令不能把 writer 正在写的章指针拽走、清空已完成场景。
func TestPlanChapterForLaterQueuedChapterKeepsInProgressPointer(t *testing.T) {
	st := rewriteQueueStore(t, []int{1, 3})
	tool := NewPlanChapterTool(st)
	if _, err := tool.Execute(context.Background(), rewritePlanArgs(3, "Đổi kết chương thành cuộc đối chất")); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	p, _ := st.Progress.Load()
	if p.InProgressChapter != 1 {
		t.Fatalf("InProgressChapter phải giữ ở 1, nhận %d", p.InProgressChapter)
	}
}

// 已完成且不在返工队列的章节仍旧拒绝重新规划。
func TestPlanChapterStillSkipsCompletedChapterOutsideQueue(t *testing.T) {
	st := rewriteQueueStore(t, []int{2})
	tool := NewPlanChapterTool(st)
	res, err := tool.Execute(context.Background(), rewritePlanArgs(1, "Không được ghi"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var out map[string]any
	_ = json.Unmarshal(res, &out)
	if out["skipped"] != true {
		t.Fatalf("chương đã chốt ngoài hàng đợi phải bị bỏ qua, nhận %+v", out)
	}
}
