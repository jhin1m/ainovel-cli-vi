package exp

import (
	"strings"
	"testing"
)

// Writer 常把整行加粗当作章标题写进正文首行；导出器已统一渲染标题，
// 不剥掉就会出现两行标题。
func TestStripChapterTitleHeaderBoldLine(t *testing.T) {
	cases := []struct{ in, title, want string }{
		{"**Chương 4: Bí Mật Về Linh Thảo**\n\nNửa canh giờ trước.", "Bí Mật Về Linh Thảo", "Nửa canh giờ trước."},
		{"**Bí Mật Về Linh Thảo**\n\nNửa canh giờ trước.", "Bí Mật Về Linh Thảo", "Nửa canh giờ trước."},
		// 以加粗开头的正文句子不是标题，必须保留。
		{"**Không ai** ngờ rằng đêm ấy cả thôn thức trắng.", "Đêm Trắng", "**Không ai** ngờ rằng đêm ấy cả thôn thức trắng."},
		{"**Một câu in đậm trọn dòng.**\nTiếp.", "Đêm Trắng", "**Một câu in đậm trọn dòng.**\nTiếp."},
	}
	for _, c := range cases {
		if got := stripChapterTitleHeader(c.in, c.title); got != c.want {
			t.Errorf("stripChapterTitleHeader(%q, %q) = %q, mong đợi %q", c.in, c.title, got, c.want)
		}
	}
}

// 标题里自带的章号来自模型记忆（规划期命名为 Chương 19..23，落盘时实为 1..5），
// 导出器再拼一次真实章号就成了 "Chương 2: Chương 20: …"。
func TestRenderTXTDropsChapterNumberFromTitle(t *testing.T) {
	got := renderTXT("", []int{2, 3},
		chapterTitleIndex{2: "Chương 20: Gặp Linh Vân", 3: "Chương 21"},
		nil, map[int]string{2: "Văn chương hai.", 3: "Văn chương ba."})
	if !strings.Contains(got, "Chương 2: Gặp Linh Vân\n") {
		t.Fatalf("tiêu đề phải theo số chương thật:\n%s", got)
	}
	if strings.Contains(got, "Chương 20") || strings.Contains(got, "Chương 21") {
		t.Fatalf("số chương sai trong tiêu đề không được lọt ra:\n%s", got)
	}
	if !strings.Contains(got, "Chương 3\n") {
		t.Fatalf("tiêu đề chỉ có số chương thì rơi về dạng chỉ số:\n%s", got)
	}
}

func TestDisplayTitleKeepsPlainTitles(t *testing.T) {
	idx := chapterTitleIndex{1: "  Chương cuối của đời  ", 2: "第 20 章 灵草秘密", 3: "Chapter 7. Dawn"}
	for ch, want := range map[int]string{1: "Chương cuối của đời", 2: "灵草秘密", 3: "Dawn"} {
		if got := idx.display(ch); got != want {
			t.Errorf("display(%d) = %q, mong đợi %q", ch, got, want)
		}
	}
}
