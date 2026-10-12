package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

// 分层模式下扁平大纲只含已展开的弧，骨架弧对"大纲内还有未写章节"的比对完全
// 隐形：第 1 卷还有整弧没展开，写完已展开的章节后 complete_book 仍被放行。
func TestSaveFoundationCompleteBookRejectsSkeletonArcs(t *testing.T) {
	s := completeBookSetup(t)
	if err := s.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
		Title: "Quyển 1",
		Arcs: []domain.ArcOutline{
			{Title: "Cung mở đầu", Goal: "Nhập môn", Chapters: []domain.OutlineEntry{
				{Title: "Một", CoreEvent: "Rời làng"},
				{Title: "Hai", CoreEvent: "Gặp sư phụ"},
			}},
			{Title: "Thảm hoạ linh căn", Goal: "Mất linh căn", EstimatedChapters: 13},
		},
	}}); err != nil {
		t.Fatalf("SaveLayeredOutline: %v", err)
	}
	if err := s.Progress.SetLayered(true); err != nil {
		t.Fatalf("SetLayered: %v", err)
	}
	for ch := 1; ch <= 2; ch++ {
		if err := s.Progress.MarkChapterComplete(ch, 3000, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}
	tool := NewSaveFoundationTool(s)
	args, _ := json.Marshal(map[string]any{
		"type": "complete_book", "content": map[string]any{}, "reason": "đã viết hết",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("còn cung khung chưa triển khai thì complete_book phải bị chặn")
	}
	if !strings.Contains(err.Error(), "Thảm hoạ linh căn") || !strings.Contains(err.Error(), "final") {
		t.Fatalf("thông báo phải nêu cung khung và chỉ lối final, nhận: %v", err)
	}
	if p, _ := s.Progress.Load(); p.Phase != domain.PhaseWriting {
		t.Fatalf("phase phải giữ writing, nhận %s", p.Phase)
	}
}

// Dàn ý "giậm chân": tiêu đề khác nhau nhưng nhiều chương chung một móc câu,
// Writer viết trung thành nên chương sau chỉ kể lại chương trước. Writer và
// Editor đều chỉ thấy từng chương nên không phát hiện; phải chặn lúc lưu dàn ý.
func TestSaveFoundationRejectsStalledOutline(t *testing.T) {
	newStore := func(t *testing.T) *store.Store {
		s := store.NewStore(t.TempDir())
		if err := s.Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		if err := s.Progress.Init(0); err != nil {
			t.Fatalf("InitProgress: %v", err)
		}
		return s
	}
	chapters := func(n int) []map[string]any {
		out := make([]map[string]any, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, map[string]any{
				"title":      fmt.Sprintf("Chương %d", i),
				"core_event": fmt.Sprintf("Biến cố thứ %d", i),
				"hook":       "Một tu sĩ trẻ xuất hiện với lời mời gia nhập tông phái",
			})
		}
		return out
	}

	t.Run("outline", func(t *testing.T) {
		tool := NewSaveFoundationTool(newStore(t))
		args, _ := json.Marshal(map[string]any{"type": "outline", "content": chapters(4)})
		if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "hook") {
			t.Fatalf("outline lặp hook phải bị bác, nhận: %v", err)
		}
	})
	t.Run("layered_outline", func(t *testing.T) {
		tool := NewSaveFoundationTool(newStore(t))
		args, _ := json.Marshal(map[string]any{
			"type": "layered_outline", "scale": "long",
			"content": []map[string]any{{
				"title": "Quyển 1", "theme": "Khởi đầu",
				"arcs": []map[string]any{{"title": "Cung 1", "goal": "Mục tiêu", "chapters": chapters(4)}},
			}},
		})
		if _, err := tool.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "hook") {
			t.Fatalf("layered_outline lặp hook phải bị bác, nhận: %v", err)
		}
	})
}
