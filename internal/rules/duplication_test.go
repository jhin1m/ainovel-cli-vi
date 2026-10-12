package rules

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

func viPara(seed string) string {
	// Câu đệm ngắn (<15 từ) để chỉ đoạn trùng mới tạo câu dài lặp lại.
	return seed + " Gió núi phả vào mặt lạnh buốt. Hắn siết chặt cây gậy gỗ mục trong tay."
}

// Model lặp vòng: cùng một đoạn xuất hiện 5 lần trong chương.
func TestSelfDuplication_CatchesRepeatedParagraph(t *testing.T) {
	rep := viPara("Họ đi xuống dốc núi theo con đường Thầy Mộc Lâu chọn.")
	body := strings.Join([]string{rep, viPara("Một."), rep, viPara("Hai."), rep, rep, rep}, "\n\n")
	v := findRule(Lint(body), "self_duplication")
	if v == nil || v.Severity != SeverityError {
		t.Fatalf("mong đợi self_duplication, nhận: %+v", Lint(body))
	}
	if !strings.Contains(v.Target, "×5") {
		t.Errorf("phải nêu số lần lặp, nhận: %v", v.Target)
	}
	if findRule(Lint(body), "repeated_sentence") != nil {
		t.Error("đã báo self_duplication thì không báo trùng repeated_sentence")
	}
}

// Khác biệt khoảng trắng/xuống dòng không che được lặp nguyên văn.
func TestSelfDuplication_NormalizesWhitespace(t *testing.T) {
	rep := viPara("Hắn đứng lặng trước cổng tông môn rất lâu.")
	alt := strings.Replace(rep, " ", "  ", 3)
	body := strings.Join([]string{rep, alt, viPara("Ba."), viPara("Bốn.")}, "\n\n")
	if findRule(Lint(body), "self_duplication") == nil {
		t.Fatalf("mong đợi self_duplication, nhận: %+v", Lint(body))
	}
}

func TestSelfDuplication_SilentOnCleanChapter(t *testing.T) {
	body := strings.Join([]string{viPara("Một."), viPara("Hai."), viPara("Ba."), viPara("Bốn.")}, "\n\n")
	vs := Lint(body)
	if findRule(vs, "self_duplication") != nil {
		t.Fatalf("không được báo trên văn sạch, nhận: %+v", vs)
	}
}

// Đoạn ngắn lặp lại là bình thường ("Hắn gật đầu.").
func TestSelfDuplication_IgnoresShortParagraphs(t *testing.T) {
	body := strings.Join([]string{"Hắn gật đầu.", viPara("Một."), "Hắn gật đầu.", viPara("Hai."), "Hắn gật đầu."}, "\n\n")
	vs := Lint(body)
	if findRule(vs, "self_duplication") != nil || findRule(vs, "repeated_sentence") != nil {
		t.Fatalf("đoạn ngắn lặp không được báo, nhận: %+v", vs)
	}
}

// Một câu dài lặp nguyên văn giữa các đoạn khác nhau (chưa tới ngưỡng đoạn).
func TestRepeatedSentence_CatchesLongSentenceLoop(t *testing.T) {
	loop := "Ánh trăng bạc rải xuống mặt hồ tĩnh lặng như một tấm gương khổng lồ phản chiếu cả bầu trời đêm."
	body := strings.Join([]string{
		viPara("Một.") + " " + loop,
		viPara("Hai."),
		viPara("Ba.") + " " + loop,
		viPara("Bốn."),
		viPara("Năm."),
		viPara("Sáu."),
	}, "\n\n")
	v := findRule(Lint(body), "repeated_sentence")
	if v == nil || v.Severity != SeverityWarning || v.Actual != 1 {
		t.Fatalf("mong đợi repeated_sentence ×1, nhận: %+v", Lint(body))
	}
}

func TestCheckAgainstPrevious_CatchesCopiedChapter(t *testing.T) {
	shared := []string{viPara("Hơi ẩm rừng buổi sáng hòa lẫn mùi đất."), viPara("Hắn mở mắt ra.")}
	prev := strings.Join(append(append([]string{}, shared...), viPara("Riêng của chương trước.")), "\n\n")
	cur := strings.Join(append(append([]string{}, shared...), viPara("Một câu mới.")), "\n\n")
	vs := CheckAgainstPrevious(cur, []string{prev})
	if len(vs) != 1 || vs[0].Rule != "copied_previous_chapter" || vs[0].Severity != SeverityError {
		t.Fatalf("mong đợi copied_previous_chapter, nhận: %+v", vs)
	}
}

func TestCheckAgainstPrevious_SilentOnFreshChapter(t *testing.T) {
	prev := strings.Join([]string{viPara("Một."), viPara("Hai.")}, "\n\n")
	cur := strings.Join([]string{viPara("Ba."), viPara("Bốn.")}, "\n\n")
	if vs := CheckAgainstPrevious(cur, []string{prev}); len(vs) != 0 {
		t.Fatalf("chương mới hoàn toàn không được báo, nhận: %+v", vs)
	}
	if vs := CheckAgainstPrevious(cur, nil); len(vs) != 0 {
		t.Fatalf("không có chương trước thì phải im, nhận: %+v", vs)
	}
}

// language=zh giữ nguyên hành vi upstream: không có luật trùng lặp.
func TestDuplicationRules_GatedToVietnamese(t *testing.T) {
	useLang(t, lang.ZH)
	rep := viPara("Lặp.")
	body := strings.Join([]string{rep, rep, rep}, "\n\n")
	if vs := CheckAgainstPrevious(body, []string{body}); len(vs) != 0 {
		t.Fatalf("zh không chạy copied_previous_chapter: %+v", vs)
	}
	if v := findRule(Lint(body), "self_duplication"); v != nil {
		t.Fatalf("zh không chạy self_duplication: %+v", v)
	}
}
