package rules

import (
	"strings"
	"testing"
)

// cleanVietnamese 是一段真实风格的越南语正文：包含大量不带附标的合法越南语词
// （anh, con, ban, hai, to, me, an, lam, sao, tin, long, man, son, can, may, go…），
// 外国人名、缩写、度量单位与数字，用来压住误报。
const cleanVietnamese = `# Chương 3: Gió nổi

Trời vừa hửng sáng, Lâm Trần đã ra khỏi con hẻm nhỏ sau chợ. Anh mang theo hai cái bao to, trong bao là me chua và ít gạo mà mẹ gửi lên. Ban mai lạnh, sương còn đọng trên mái ngói.

"Anh định đi đâu sớm vậy?" Tiểu Vân hỏi, tay vẫn cầm cây chổi tre.

Lâm Trần không đáp ngay. Anh nhìn về phía con sông, nơi mấy chiếc ghe đang neo, rồi mới nói: "Lên huyện. Nghe nói ông John bên tiệm thuốc cần người khuân hàng, trả 3.000 đồng một ngày."

Hắn đi bộ chừng 5 km thì tới đầu làng bên. Long Hổ Sơn hiện ra sau màn sương, xanh thẫm như một bức tranh thủy mặc. Hai bên đường, lúa đã ngả vàng, gió thổi qua làm cả cánh đồng gợn sóng.

Anh chợt nhớ lời thầy dặn: tâm phải tĩnh, chân phải vững, tay-chân không được rối. Anh hít một hơi, siết lại quai bao rồi bước tiếp. Ở trạm xe, một người đàn ông mặc áo NATO cũ đang ngồi hút thuốc, chiếc radio bên cạnh rè rè phát tin thời tiết. Con mèo nằm im, ai gọi cũng không ra, may mà trời chưa mưa, gió cũng nhẹ dần.`

func TestVietnameseLints_SilentOnCleanProse(t *testing.T) {
	if vs := Lint(cleanVietnamese); len(vs) != 0 {
		t.Fatalf("văn Việt sạch không được báo lỗi, nhận: %+v", vs)
	}
}

func TestCJKLeak_CatchesHanKanaHangulAndPunctuation(t *testing.T) {
	cases := map[string]string{
		"han":    "Lữ Cầm buộc phải dùng根系之力 để ổn định làng quê trước cơn lũ.",
		"punct":  "Hắn muốn tự mình chứng kiến。Gió thổi qua luống thuốc.",
		"kana":   "Cô gái khẽ nói ありがとう rồi quay đi.",
		"hangul": "Trên tấm biển có dòng chữ 안녕 đã mờ.",
	}
	for name, text := range cases {
		v := findRule(Lint(text), "cjk_leak")
		if v == nil || v.Severity != SeverityError {
			t.Errorf("%s: mong đợi cjk_leak error, nhận %+v", name, Lint(text))
		}
	}
	v := findRule(Lint(cases["han"]), "cjk_leak")
	if v.Target != "根系之力" || v.Actual != 1 {
		t.Errorf("ví dụ/số lần sai: %+v", v)
	}
}

// Rò rỉ thật đã thấy: hư từ tiếng Anh ngay sau gạch ngang.
func TestEnglishResidue_CatchesFunctionWords(t *testing.T) {
	text := "Lá cây bắt đầu chuyển động—not nhanh chóng mà nhẹ nhàng nghiêng mình. " +
		"Tông môn, pháp sư—they đang theo dõi ngươi từ xa. The gió vẫn thổi."
	v := findRule(Lint(text), "english_residue")
	if v == nil {
		t.Fatal("mong đợi english_residue")
	}
	if v.Severity != SeverityError || v.Actual != 3 {
		t.Errorf("mong đợi error ×3, nhận %+v", v)
	}
	if !strings.Contains(v.Target, "not") || !strings.Contains(v.Target, "they") || !strings.Contains(v.Target, "the") {
		t.Errorf("ví dụ phải gồm not/they/the: %q", v.Target)
	}
}

// Từ nội dung/ngoại lai (pattern, flow, radio) không nằm trong danh sách → không báo, tránh dương tính giả.
func TestEnglishResidue_IgnoresContentWordsAndLoanwords(t *testing.T) {
	text := "Hắn nhận ra một pattern lạ trong dòng chảy, cái flow ấy cứ lặp lại như tiếng radio cũ."
	if v := findRule(Lint(text), "english_residue"); v != nil {
		t.Fatalf("không được báo từ nội dung/ngoại lai: %+v", v)
	}
}

// Chữ hoa đầu (tên riêng nước ngoài) và viết tắt toàn hoa không bị báo.
func TestEnglishResidue_IgnoresNamesAndAcronyms(t *testing.T) {
	text := "Ông Smith đưa cho hắn tấm thẻ của FBI rồi lái chiếc Ford đi mất."
	if v := findRule(Lint(text), "english_residue"); v != nil {
		t.Fatalf("không được báo tên riêng/viết tắt: %+v", v)
	}
}

// Dạng tổ hợp (NFD) của chữ có dấu không được bị coi là từ ASCII.
func TestEnglishResidue_NFDInputNoFalsePositive(t *testing.T) {
	text := "Ha\u0306\u0301n the\u0300m mo\u0301n a\u0306n a\u0301y la\u0306\u0301m."
	if v := findRule(Lint(text), "english_residue"); v != nil {
		t.Fatalf("NFD không được báo: %+v", v)
	}
}

func TestTechnicalResidue(t *testing.T) {
	text := "Hắn mở cuộn trục, thấy dòng chapterPlan và hook_type, cùng địa chỉ https://example.com và file drafts/01.md."
	v := findRule(Lint(text), "technical_residue")
	if v == nil || v.Actual != 4 || v.Severity != SeverityWarning {
		t.Fatalf("mong đợi technical_residue ×4, nhận %+v", v)
	}
	clean := "Hắn cầm chiếc iPhone, mở YouTube xem tin tức rồi gọi cho eBay."
	if v := findRule(Lint(clean), "technical_residue"); v != nil {
		t.Fatalf("tên thương hiệu không được báo: %+v", v)
	}
}

// Tên riêng bị cắt đôi qua dấu xuống đoạn.
func TestBrokenWord(t *testing.T) {
	text := "Hắn bước ra khỏi Thanh Vân Tông, Ng\n\nọc Lâm dừng bước.\n\nTrước mặt là vực sâu."
	if v := findRule(Lint(text), "broken_word"); v == nil || v.Actual != 1 {
		t.Fatalf("mong đợi broken_word, nhận %+v", Lint(text))
	}
	normal := "Hắn cúi xuống bên luống thuốc.\n\nGió thổi qua, lá lay động nhẹ.\n\nNgọc Lâm dừng bước."
	if v := findRule(Lint(normal), "broken_word"); v != nil {
		t.Fatalf("văn bình thường không được báo: %+v", v)
	}
}
