package stylestat

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

func TestComputePatternsVI(t *testing.T) {
	useLang(t, lang.VI)
	body := "Hắn không phải tức giận, mà là sợ hãi. Im lặng một lúc sau vài hơi thở. Ánh đèn như một vì sao. " +
		"Trong mắt lóe lên vẻ hoảng loạn, tim thắt lại. Hắn cảm thấy một cảm giác khó tả.\nChính văn.\n"
	chapters := make([]string, 6)
	for i := range chapters {
		chapters[i] = chapterWith(body)
	}
	s := Compute(Input{Chapters: chapters})
	if s == nil {
		t.Fatal("expected stats")
	}
	if len(s.Patterns) != len(viProfile.patterns) {
		t.Fatalf("want %d pattern classes, got %d: %+v", len(viProfile.patterns), len(s.Patterns), s.Patterns)
	}
	for _, p := range s.Patterns {
		if p.Total != 6 || p.PerChapter != 1.0 {
			t.Errorf("%s: total=%d per_chapter=%v, want 6 / 1.0", p.Name, p.Total, p.PerChapter)
		}
	}
}

func TestComputeTopPhrasesVIFiltersNames(t *testing.T) {
	useLang(t, lang.VI)
	body := strings.Repeat("Mọi người nhìn về đỉnh núi Thanh Vân, Lục Cửu Uyên chắp tay đứng lặng.\nUyên gật đầu thật chậm rãi.\n", 3)
	chapters := make([]string, 10)
	for i := range chapters {
		chapters[i] = chapterWith(body)
	}
	s := Compute(Input{Chapters: chapters, Stopwords: []string{"Lục Cửu Uyên"}})
	if s == nil {
		t.Fatal("expected stats")
	}
	var hasPeak bool
	for _, p := range s.TopPhrases {
		if strings.Contains(p.Text, "đỉnh núi") {
			hasPeak = true
		}
		if strings.Contains(p.Text, "uyên") || strings.Contains(p.Text, "thanh vân") || strings.Contains(p.Text, "cửu") {
			t.Errorf("proper noun leaked into phrases: %+v", s.TopPhrases)
		}
	}
	if !hasPeak {
		t.Errorf("expected phrase with 'đỉnh núi', got %+v", s.TopPhrases)
	}
}

func TestComputeRepeatedSentencesVI(t *testing.T) {
	useLang(t, lang.VI)
	motto := "Cả đời này chưa từng đi xa, mong cậu thay tôi ngắm nhìn núi sông nơi xa."
	chapters := make([]string, 6)
	for i := range chapters {
		body := "Chính văn bình thường, không lặp gì.\n"
		if i%2 == 0 {
			body += motto + "\n"
		}
		chapters[i] = chapterWith(body)
	}
	s := Compute(Input{Chapters: chapters})
	if s == nil {
		t.Fatal("expected stats")
	}
	for _, r := range s.RepeatedSentences {
		if strings.HasPrefix(r.Text, "Cả đời này chưa từng đi xa") {
			if r.Chapters != 3 || r.Count != 3 {
				t.Errorf("repeated sentence: %+v", r)
			}
			if r.Text != strings.TrimSuffix(motto, ".") {
				t.Errorf("Latin sentence should not be truncated at 40 runes: %q", r.Text)
			}
			return
		}
	}
	t.Fatalf("expected repeated motto, got %+v", s.RepeatedSentences)
}

func TestComputeEndingOpeningTitlesVI(t *testing.T) {
	useLang(t, lang.VI)
	short := chapterWith("Cả đêm không ngủ.\nChính văn rất dài.\nAnh ấy đi rồi.")
	long := chapterWith("Chuyện xảy ra ban ngày.\nChính văn.\n" +
		"Đây là một câu kết rất rất dài, dài hơn hẳn ba mươi âm tiết, được viết ra chỉ để kiểm tra xem trung vị độ dài dòng cuối có được tính theo số từ hay không, và nó phải vượt ngưỡng.")
	s := Compute(Input{
		Chapters: []string{short, short, short, long, long},
		Titles:   []string{"Chương 1: Gió nổi", "Mây cuộn", "## Chương III - Sấm động", "Chương mới"},
	})
	if s == nil {
		t.Fatal("expected stats")
	}
	if s.Ending.ShortRatio != 0.6 {
		t.Errorf("short_ratio: got %v want 0.6", s.Ending.ShortRatio)
	}
	if s.OpeningTimeRate != 0.6 {
		t.Errorf("opening_time_rate: got %v want 0.6", s.OpeningTimeRate)
	}
	if s.TitleFormats == nil || s.TitleFormats.WithPrefix != 2 || s.TitleFormats.WithoutPrefix != 2 {
		t.Errorf("title formats: %+v", s.TitleFormats)
	}
}

func TestTrackerMatchesComputeVI(t *testing.T) {
	useLang(t, lang.VI)
	tracker := NewTracker()
	chapters := map[int]string{}
	for chapter := 1; chapter <= 6; chapter++ {
		chapters[chapter] = chapterWith("Sáng sớm, hắn không phải do dự, mà là sợ hãi.\n" +
			"Cả đời này chưa từng đi xa, mong cậu thay tôi ngắm nhìn núi sông nơi xa.\nCửa mở.")
		tracker.Upsert(chapter, chapters[chapter])
		assertTrackerMatchesCompute(t, tracker, chapters, []string{"Chương 1", "Mây cuộn"}, []string{"Lục Cửu Uyên"})
	}
	delete(chapters, 3)
	tracker.Remove(3)
	assertTrackerMatchesCompute(t, tracker, chapters, nil, nil)
}
