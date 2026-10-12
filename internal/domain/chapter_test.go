package domain

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

func TestWordCount_Vietnamese(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"Hắn bước ra khỏi Thanh Vân Tông.", 7},
		{"  Trời   vừa\nhửng sáng,\t\"anh\" đi—rồi về…  ", 8},
		{"tay-chân rã rời", 4},              // gạch nối tách âm tiết
		{"trả 3.000 đồng, lãi 2,5 phần", 6}, // dấu chấm/phẩy giữa hai chữ số không tách
		{"năm 2024. Rồi 5,mười", 5},         // dấu câu không nằm giữa hai chữ số thì tách
		{"Ha\u0300nh lang", 2},              // dấu tổ hợp (NFD) thuộc về từ
		{"# Chương 1: Gió nổi", 4},
	}
	for _, c := range cases {
		if got := WordCount(c.text); got != c.want {
			t.Errorf("WordCount(%q) = %d, muốn %d", c.text, got, c.want)
		}
	}
}

func TestWordCount_ChineseKeepsRuneCount(t *testing.T) {
	lang.Set(lang.ZH)
	t.Cleanup(func() { lang.Set(lang.VI) })
	if got := WordCount("他迈步向前。"); got != 6 {
		t.Errorf("zh phải đếm rune, có %d", got)
	}
	if got := WordCount("Hắn đi"); got != 6 {
		t.Errorf("zh phải đếm rune kể cả chữ Latin, có %d", got)
	}
}

func TestHookTypeCounts(t *testing.T) {
	got := HookTypeCounts([]string{"mystery", "", "crisis", "mystery"})
	if got["mystery"] != 2 || got["crisis"] != 1 || len(got) != 2 {
		t.Errorf("HookTypeCounts = %v", got)
	}
}
