package stylestat

import (
	"regexp"
	"strings"
	"unicode"
)

// viProfile là bộ mẫu văn phong AI thường gặp trong văn tiếng Việt. Các regex đều
// không phân biệt hoa thường; ranh giới từ được bỏ qua vì \b của Go chỉ hiểu ASCII,
// nên mỗi mẫu dùng cụm nhiều âm tiết đủ đặc trưng để hạn chế khớp nhầm.
var viProfile = &profile{
	patterns: []patternDef{
		{"Câu đính chính «không phải… mà là…»", regexp.MustCompile(`(?i)không phải[^.!?…\n]{1,80}?[,;]?\s*mà(?:\s+là)?\s`)},
		{"Lượng từ thời gian «vài hơi thở/chốc lát/một nhịp»", regexp.MustCompile(`(?i)(?:một|hai|ba|vài|mấy|nửa|mươi)\s+(?:hơi thở|nhịp thở|nhịp|cái chớp mắt|chớp mắt|khắc)|chốc lát`)},
		{"So sánh «như một/tựa như/phảng phất/dường như»", regexp.MustCompile(`(?i)như một|tựa như|tựa hồ|phảng phất|dường như|hệt như`)},
		{"Nhịp im lặng «im lặng một lúc/không nói gì/không quay đầu»", regexp.MustCompile(`(?i)im lặng (?:một lúc|hồi lâu|một hồi|giây lát)|trầm mặc|không nói gì|không quay đầu|không ngoảnh lại`)},
		{"Khuôn biểu cảm «trong mắt lóe lên/khóe môi cong/cắn môi»", regexp.MustCompile(`(?i)(?:trong|đáy) mắt (?:thoáng )?(?:lóe|loé|lướt|hiện) lên|ánh mắt (?:chợt )?(?:ngưng|trầm) lại|đồng tử co (?:rút|lại)|vành mắt (?:hơi )?đỏ|khóe (?:môi|miệng) (?:khẽ |hơi )?(?:nhếch|cong|giương)|cắn (?:nhẹ )?môi|không thể tin (?:nổi|được)`)},
		{"Phản ứng cơ thể «tim thắt lại/người khẽ run/hít một hơi lạnh»", regexp.MustCompile(`(?i)(?:tim|lòng) (?:chợt )?(?:thắt|chùng|hẫng|trĩu) (?:lại|xuống)|(?:người|thân hình|cả người) (?:khẽ )?(?:run lên|run rẩy|chấn động|cứng đờ)|hít (?:vào )?một (?:hơi|ngụm) khí lạnh`)},
		{"Đánh dấu suy nghĩ «thầm nghĩ/nhận ra/cảm thấy»", regexp.MustCompile(`(?i)thầm nghĩ|nhận ra|ý thức được|cảm thấy`)},
		{"Sáo ngữ trừu tượng «một cảm giác khó tả/ý nghĩa nằm ở»", regexp.MustCompile(`(?i)một (?:cảm giác|thứ|nỗi) (?:khó tả|không tên|không nói nên lời)|không nói rõ được|ý nghĩa (?:nằm ở|của nó là)|điều thật sự[^.!?…\n]{1,40}? là`)},
	},
	sentenceSplit: regexp.MustCompile(`[.!?…\n]+`),
	openingTime:   regexp.MustCompile(`(?i)đêm|sáng sớm|bình minh|rạng sáng|tờ mờ sáng|trời sáng|tỉnh dậy|tỉnh giấc|thức dậy|nắng sớm|ánh ban mai`),
	titlePrefix:   regexp.MustCompile(`(?i)^#{0,2}\s*chương\s+(?:\d+|[ivxlcdm]+)(?:[^\p{L}\d]|$)`),
	displayRunes:  120,
	minePhrases:   minePhrasesVI,
}

// viEdgeStop là hư từ/đại từ: n-gram mở đầu hoặc kết thúc bằng chúng không phải
// cụm từ văn phong mà chỉ là mảnh ngữ pháp, nên bỏ qua.
var viEdgeStop = toSet(strings.Fields(`và của là thì mà đã đang sẽ bị được cho với những các một này kia đó ấy
	hắn nàng anh cô nó ta tôi y gã họ chúng mình cũng lại rồi vẫn không chẳng có trong ở ra vào lên xuống
	như để từ đến tới khi nhưng hay hoặc nên vì do rằng thế vậy đây đấy chỉ còn rất quá lắm đều`))

func toSet(words []string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

// viClauseSplit tách văn bản thành các mệnh đề: n-gram không được vượt qua dấu câu.
var viClauseSplit = regexp.MustCompile(`[.,;:!?…"“”‘’'()\[\]«»\-—–\n]+`)

// minePhrasesVI khai thác cụm 3–6 âm tiết tần suất cao trong cửa sổ.
// Lọc: chứa chữ số/ký tự lạ, đầu hoặc cuối là hư từ, chứa danh từ riêng (âm tiết
// viết hoa không đứng đầu mệnh đề) hoặc trùng tên nhân vật; khử trùng: cụm này là
// cụm con của cụm đã chọn thì bỏ.
func minePhrasesVI(chapters []string, stopwords []string) []PhraseStat {
	threshold := max(8, len(chapters)/2)
	counts := make(map[string]int)
	for _, text := range chapters {
		for line := range strings.SplitSeq(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue // tiêu đề Markdown không phải chính văn
			}
			for _, clause := range viClauseSplit.Split(line, -1) {
				countClauseGrams(strings.Fields(clause), counts)
			}
		}
	}

	nameGrams, nameHeads := viNameFilters(stopwords)
	reject := func(g string) bool {
		padded := " " + g + " "
		for _, ng := range nameGrams {
			if strings.Contains(padded, ng) {
				return true
			}
		}
		first, _, _ := strings.Cut(g, " ")
		_, isName := nameHeads[first]
		return isName
	}
	containsWords := func(a, b string) bool { return strings.Contains(" "+a+" ", " "+b+" ") }
	return pickPhrases(counts, threshold, reject, containsWords)
}

func countClauseGrams(tokens []string, counts map[string]int) {
	for size := 3; size <= 6; size++ {
		for i := 0; i+size <= len(tokens); i++ {
			gram := tokens[i : i+size]
			if !validGramVI(gram, i == 0) {
				continue
			}
			counts[strings.ToLower(strings.Join(gram, " "))]++
		}
	}
}

// validGramVI: mọi âm tiết chỉ gồm chữ cái; âm tiết viết hoa chỉ được phép ở đầu
// mệnh đề (chữ hoa đầu câu), ở vị trí khác coi là danh từ riêng; hai đầu không là hư từ.
func validGramVI(gram []string, clauseStart bool) bool {
	for i, tok := range gram {
		for _, r := range tok {
			if !unicode.IsLetter(r) {
				return false
			}
		}
		if unicode.IsUpper([]rune(tok)[0]) && (i > 0 || !clauseStart) {
			return false
		}
	}
	if _, ok := viEdgeStop[strings.ToLower(gram[0])]; ok {
		return false
	}
	if _, ok := viEdgeStop[strings.ToLower(gram[len(gram)-1])]; ok {
		return false
	}
	return true
}

// viNameFilters trả về các cặp âm tiết liền kề của tên riêng (dạng " a b " để so
// theo ranh giới từ) và tập âm tiết tên dùng chặn cụm mở đầu bằng tên. Tên thường
// xuất hiện từng phần ("Cửu Uyên chắp tay"), nên lọc theo cặp âm tiết thay vì cả tên —
// thà lọc hơi chặt còn hơn để tên nhân vật lẫn vào danh sách câu cửa miệng.
func viNameFilters(stopwords []string) (grams []string, heads map[string]struct{}) {
	heads = make(map[string]struct{})
	for _, w := range stopwords {
		toks := strings.Fields(strings.ToLower(w))
		for _, t := range toks {
			heads[t] = struct{}{}
		}
		for i := 0; i+2 <= len(toks); i++ {
			grams = append(grams, " "+toks[i]+" "+toks[i+1]+" ")
		}
	}
	return grams, heads
}
