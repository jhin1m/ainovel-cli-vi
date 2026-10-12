package rules

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

func TestCountPhrase_VietnameseWordBoundaryAndCase(t *testing.T) {
	cases := []struct {
		text, phrase string
		want         int
	}{
		{"Tuy nhiên hắn vẫn đi. Hắn tuy nhiên chẳng nói.", "tuy nhiên", 2},
		{"Hắn chạy nhanh, trời lạnh, anh cũng đi.", "anh", 1}, // không khớp trong "nhanh"/"lạnh"
		{"Hắn không khỏi bật cười, không khỏi lắc đầu.", "không khỏi", 2},
		{"Nhưng một lúc sau như một cơn gió.", "như một", 1},              // "Nhưng một" không tính
		{"Đồng tử co rút lại, đồng tử co rút thêm.", "đồng tử co rút", 2}, // chữ hoa đầu câu
		{"Ha\u0300nh lang dài.", "hành lang", 1},                          // NFD vẫn khớp NFC
		{"他不禁笑了，又不禁皱眉。", "不禁", 2},                                         // chữ Hán không có ranh giới từ
	}
	for _, c := range cases {
		if got := countPhrase(c.text, c.phrase); got != c.want {
			t.Errorf("countPhrase(%q, %q) = %d, muốn %d", c.text, c.phrase, got, c.want)
		}
	}
}

func TestCountPhrase_ZHKeepsSubstringCount(t *testing.T) {
	useLang(t, lang.ZH)
	if got := countPhrase("Hắn chạy nhanh", "anh"); got != 1 {
		t.Errorf("zh phải giữ đếm chuỗi con, có %d", got)
	}
}

// Mốc mặc định tiếng Việt chạy qua Check với văn Việt thật.
func TestCheck_VietnameseDefaults(t *testing.T) {
	text := "Hắn hít sâu một hơi. Khóe môi khẽ nhếch, hắn không khỏi bật cười, rồi lại không khỏi lắc đầu. " +
		"Ở một mức độ nào đó, hắn đã thắng."
	vs := Check(text, SystemDefaults().Structured)
	if v := findViolation(vs, "forbidden_phrases", "ở một mức độ nào đó"); v == nil || v.Severity != SeverityError {
		t.Errorf("mong đợi forbidden_phrases: %+v", vs)
	}
	if v := findViolation(vs, "fatigue_words", "không khỏi"); v == nil || v.Actual != 2 {
		t.Errorf("mong đợi fatigue không khỏi ×2: %+v", vs)
	}
	if v := findViolation(vs, "fatigue_words", "hít sâu một hơi"); v != nil {
		t.Errorf("1 lần chưa vượt ngưỡng 1: %+v", v)
	}

	clean := "Hắn nhìn mặt hồ rất lâu. Gió lạnh thổi qua, lá tre xào xạc. Hắn quay người bước đi, không ngoảnh lại."
	if vs := Check(clean, SystemDefaults().Structured); len(vs) != 0 {
		t.Errorf("văn sạch không được báo: %+v", vs)
	}
}
