package domain

import (
	"strings"
	"testing"
)

func TestStalledOutlineAcceptsHealthyOutline(t *testing.T) {
	entries := []OutlineEntry{
		{Chapter: 1, CoreEvent: "Mất mùa, phải nộp giống", Hook: "Cây mai rung lên trong đêm"},
		{Chapter: 2, CoreEvent: "Gặp người lạ ngoài chợ", Hook: "Người lạ biết tên mẹ hắn"},
		{Chapter: 3, CoreEvent: "Bị đuổi khỏi thôn", Hook: "Người lạ biết tên mẹ hắn"},
	}
	if got := StalledOutline(entries); got != "" {
		t.Fatalf("dàn ý lành mạnh (hook lặp 2 lần là chấp nhận được) không được báo lỗi, nhận %q", got)
	}
}

func TestStalledOutlineCatchesRepeatedCoreEvent(t *testing.T) {
	entries := []OutlineEntry{
		{Chapter: 1, CoreEvent: "Giả Mộc luyện kiếm"},
		{Chapter: 2, CoreEvent: "Giả Mộc luyện kiếm"},
	}
	if got := StalledOutline(entries); !strings.Contains(got, "core_event") {
		t.Fatalf("phải bắt được core_event lặp, nhận %q", got)
	}
}

func TestSkeletonArcsSilentWhenAllExpanded(t *testing.T) {
	volumes := []VolumeOutline{{Index: 1, Arcs: []ArcOutline{
		{Index: 1, Chapters: []OutlineEntry{{Chapter: 1}}},
	}}}
	if got := SkeletonArcs(volumes); len(got) != 0 {
		t.Fatalf("mọi cung đã triển khai thì không được báo, nhận %v", got)
	}
}
