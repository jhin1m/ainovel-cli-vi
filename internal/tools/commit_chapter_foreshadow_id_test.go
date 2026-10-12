package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

// foreshadowIDStore 建一份账本里已有第 1 章种下 id 的 Store。
func foreshadowIDStore(t *testing.T, id string) *store.Store {
	t.Helper()
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(5); err != nil {
		t.Fatal(err)
	}
	if err := s.World.SaveForeshadowLedger([]domain.ForeshadowEntry{
		{ID: id, Description: "Ánh mắt vô hình soi xét từ xa", PlantedAt: 1, Status: "planted"},
	}); err != nil {
		t.Fatal(err)
	}
	return s
}

func plantArgs(chapter int, id string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"chapter": chapter, "title": "Chương mới", "summary": "Gieo phục bút mới",
		"characters": []string{"Lê An"}, "key_events": []string{"Nhặt được ngọc bội"},
		"foreshadow_updates": []map[string]any{{"id": id, "action": "plant", "description": "Ngọc bội khắc tên lạ"}},
	})
	return b
}

// 模型把每条新伏笔都命名为 "1" 之类的同一个 id：第二条起撞上旧条目，新伏笔连同描述被静默丢弃。
// 必须在创建 PendingCommit 之前报错，让 Writer 换一个真正的 id。
func TestCommitChapterRejectsForeshadowIDTakenByEarlierChapter(t *testing.T) {
	for _, id := range []string{"1", "f_jade"} {
		s := foreshadowIDStore(t, id)
		_, err := newTestCommitChapterTool(s).Execute(context.Background(), plantArgs(2, id))
		if err == nil || !strings.Contains(err.Error(), "chương 1") {
			t.Fatalf("id %q đã bị chương 1 chiếm, plant ở chương 2 phải bị bác, nhận: %v", id, err)
		}
		if pending, err := s.Signals.LoadPendingCommit(); err != nil || pending != nil {
			t.Fatalf("tham số sai không được tạo pending commit: pending=%+v err=%v", pending, err)
		}
	}
}

func TestCommitChapterRejectsPlaceholderForeshadowID(t *testing.T) {
	for _, id := range []string{"new", " TBD ", "n/a", "-", "auto"} {
		s := foreshadowIDStore(t, "f_other")
		_, err := newTestCommitChapterTool(s).Execute(context.Background(), plantArgs(1, id))
		if err == nil || !strings.Contains(err.Error(), "giữ chỗ") {
			t.Fatalf("id giữ chỗ %q phải bị bác kèm lý do, nhận: %v", id, err)
		}
	}
}
