package store

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// useLang đặt ngôn ngữ cho một test và khôi phục giá trị cũ khi test kết thúc.
func useLang(t *testing.T, c lang.Code) {
	t.Helper()
	prev := lang.Current()
	lang.Set(c)
	t.Cleanup(func() { lang.Set(prev) })
}

func newDraftTestStore(t *testing.T, chapters ...string) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	for i, text := range chapters {
		if err := s.Drafts.SaveFinalChapter(i+1, text); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestExtractStyleAnchorsVietnameseBounds(t *testing.T) {
	useLang(t, lang.VI)
	short := "Trời mưa."
	good := "Anh đi ra bờ hồ và ngồi đó rất lâu, gió thổi nhẹ."
	long := strings.TrimSpace(strings.Repeat("Cô lặng lẽ nhìn theo bóng người khuất dần sau rặng tre già. ", 10))
	quoted := `"Đi." "Ở lại." "Thôi." Anh nói rồi cô đáp, hai người cứ thế mãi.`
	s := newDraftTestStore(t, strings.Join([]string{short, good, long, quoted}, "\n\n"))

	if n := len(strings.Fields(good)); n < 12 || n > 70 {
		t.Fatalf("đoạn mẫu phải nằm trong 12–70 từ, có %d", n)
	}
	anchors, err := s.Drafts.ExtractStyleAnchors(5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 1 || anchors[0] != good {
		t.Fatalf("chỉ đoạn vừa độ dài và ít thoại được chọn, nhận %q", anchors)
	}
}

func TestExtractStyleAnchorsChineseBoundsUnchanged(t *testing.T) {
	useLang(t, lang.ZH)
	good := strings.Repeat("夜色沉沉，", 12)     // 60 rune
	short := "风起。"                          // 3 rune
	long := strings.Repeat("长街尽头灯火通明，", 40) // 360 rune
	s := newDraftTestStore(t, strings.Join([]string{short, good, long}, "\n\n"))

	anchors, err := s.Drafts.ExtractStyleAnchors(5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 1 || anchors[0] != good {
		t.Fatalf("zh giữ ngưỡng 50–300 rune, nhận %q", anchors)
	}
}

func TestExtractDialogueVietnamese(t *testing.T) {
	useLang(t, lang.VI)
	text := strings.Join([]string{
		`Lâm Mặc nói: "Chúng ta phải rời khỏi đây ngay đêm nay."`,
		`Lâm Mặc gật đầu: ""`,
		"Lâm Mặc thì thầm: “Đừng quay đầu lại, cứ đi tiếp.”",
		`A Vân đáp: "Không được, trời tối quá."`,
	}, "\n")
	s := newDraftTestStore(t, text)

	samples, err := s.Drafts.ExtractDialogue("Lâm Mặc", nil, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`Lâm Mặc: "Chúng ta phải rời khỏi đây ngay đêm nay."`,
		"Lâm Mặc: “Đừng quay đầu lại, cứ đi tiếp.”",
	}
	if strings.Join(samples, "|") != strings.Join(want, "|") {
		t.Fatalf("lời thoại trích sai:\n got %q\nwant %q", samples, want)
	}
}

func TestExtractDialogueChineseKeepsRuneThreshold(t *testing.T) {
	useLang(t, lang.ZH)
	s := newDraftTestStore(t, "林墨道：\"好\"\n林墨又道：\"今晚必须离开这里\"")

	samples, err := s.Drafts.ExtractDialogue("林墨", nil, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0] != "林墨: \"今晚必须离开这里\"" {
		t.Fatalf("zh giữ ngưỡng >5 rune, nhận %q", samples)
	}
}
