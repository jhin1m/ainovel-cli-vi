package guard

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

func withLang(t *testing.T, c lang.Code) {
	t.Helper()
	prev := lang.Current()
	lang.Set(c)
	t.Cleanup(func() { lang.Set(prev) })
}

func TestSummaryTaskMarkers_VI(t *testing.T) {
	withLang(t, lang.VI)
	cases := []struct {
		task     string
		vol, arc bool
	}{
		{"Tạo tóm tắt quyển 2", true, false},
		{"Viết Tóm tắt cung truyện 1 của quyển 3", false, true},
		{"Tạo tóm tắt cung 4, ảnh chụp nhân vật và quy tắc viết", false, true},
		{"Duyệt cung 1 quyển 5 (scope=arc)", false, false},
		{"bất kỳ (save_volume_summary)", true, false},
		{"bất kỳ (save_arc_summary)", false, true},
		// Cụm tiếng Trung không được nhận khi language=vi.
		{"生成第 1 卷卷摘要", false, false},
	}
	for _, c := range cases {
		if got := isVolumeSummaryTask(c.task); got != c.vol {
			t.Errorf("isVolumeSummaryTask(%q) = %v, want %v", c.task, got, c.vol)
		}
		if got := isArcSummaryTask(c.task); got != c.arc {
			t.Errorf("isArcSummaryTask(%q) = %v, want %v", c.task, got, c.arc)
		}
	}
}

func TestSummaryTaskMarkers_ZH(t *testing.T) {
	withLang(t, lang.ZH)
	if !isVolumeSummaryTask("生成第 1 卷卷摘要") {
		t.Error("zh: 卷摘要 phải được nhận là task tóm tắt quyển")
	}
	if !isArcSummaryTask("生成第 1 卷第 2 弧摘要") {
		t.Error("zh: 弧摘要 phải được nhận là task tóm tắt cung")
	}
	if isArcSummaryTask("Tạo tóm tắt cung 2") {
		t.Error("zh: cụm tiếng Việt không dùng khi language=zh")
	}
}
