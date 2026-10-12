package rules

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// appendVietnameseLints 是 language=vi 的语言混杂与生成事故检查。
// 正文先做 NFC 归一化：组合附标形式（e + U+0300）会让 ASCII 判定把 "thè" 误当 "the"。
func appendVietnameseLints(vs []Violation, text string) []Violation {
	text = norm.NFC.String(text)
	vs = appendCJKLeak(vs, text)
	vs = appendEnglishResidue(vs, text)
	vs = appendTechnicalResidue(vs, text)
	vs = appendBrokenWords(vs, text)
	vs = appendSelfDuplication(vs, text)
	return vs
}

// cjkLeakRe 覆盖汉字、假名、谚文以及 CJK 全角标点：越南语正文里出现任何一个都是缺陷。
// 标点也要收——实测漏进越南语正文的不只是「根系之力」，还有孤立的全角句号。
var cjkLeakRe = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}\x{3000}-\x{303f}\x{ff01}-\x{ff5e}]+`)

func appendCJKLeak(vs []Violation, text string) []Violation {
	matches := cjkLeakRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return vs
	}
	return append(vs, Violation{
		Rule:     "cjk_leak",
		Target:   strings.Join(distinctExamples(matches, 3, false), ", "),
		Actual:   len(matches),
		Severity: SeverityError,
	})
}

// asciiTokens 切出正文的词元；只有纯 ASCII 词元才可能是英语或代码残留，越南语词几乎都带非 ASCII 字母。
func asciiTokens(text string) []string {
	var out []string
	start := -1
	ascii := true
	flush := func(end int) {
		if start >= 0 && ascii {
			out = append(out, text[start:end])
		}
		start, ascii = -1, true
	}
	for i, r := range text {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || r == '_' {
			if start < 0 {
				start = i
			}
			if r > unicode.MaxASCII {
				ascii = false
			}
			continue
		}
		flush(i)
	}
	flush(len(text))
	return out
}

// appendEnglishResidue 报告越南语正文里夹带的英语词（error）。
//
// 越南语也用拉丁字母，不能按字符判定，只能按词判定，并严防误报：an/anh/con/ban/hai/to/me
// 等不带附标的越南语词都合法，radio/taxi/video 等外来语也合法。因此只认精选英语词表
// （englishWords）——虚词与高频副词，不会作为越南语词或常见外来语出现。实测漏出的形态
// 是破折号后紧跟虚词：「chuyển động—not nhanh chóng」「pháp sư—they đang theo dõi」。
// 内容词（pattern/flow）刻意不收：误报代价高，且 "the flow" 已被 the 命中。
//
// 只认全小写或首字母大写（句首 "The"）；全大写视为缩写放过。
func appendEnglishResidue(vs []Violation, text string) []Violation {
	var hits []string
	for _, tok := range asciiTokens(text) {
		low := strings.ToLower(tok)
		if _, ok := englishWords[low]; !ok {
			continue
		}
		if tok == low || tok[1:] == low[1:] {
			hits = append(hits, tok)
		}
	}
	if len(hits) == 0 {
		return vs
	}
	return append(vs, Violation{
		Rule:     "english_residue",
		Target:   strings.Join(distinctExamples(hits, 4, true), ", "),
		Actual:   len(hits),
		Severity: SeverityError,
	})
}

// englishWords 只收不会与无附标越南语词冲突的英语词。刻意排除：
// a/an/to/in/do/no/so/me/be/he/my/can/may/than/ten/long/man/son/go/hit。
var englishWords = toSet(`the and but for from with they this that these those just was were have has had
when which while into over not are is of or will would could should been being what where there their
them then your you she his her him its our we it by as if yes all some very about after before again
never always maybe really still only even because however although though without something nothing
everything suddenly perhaps already until upon through against between under around here now how why
who one out up off like`)

func toSet(words string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, w := range strings.Fields(words) {
		out[w] = struct{}{}
	}
	return out
}

// technicalRe 匹配混进正文的机制/代码残留：URL、camelCase 字段名（至少两个小写字母打头，
// 放过 iPhone/eBay 一类品牌名）、snake_case、文件路径扩展名、反引号与模板占位符。
var technicalRe = regexp.MustCompile("https?://\\S+|www\\.\\S+|\\b[a-z]{2,}[A-Z][A-Za-z]*\\b|\\b[A-Za-z]+_[A-Za-z0-9_]+\\b|\\b[\\w/-]+\\.(?:md|json|go|txt|yaml|yml)\\b|`+|\\{\\{|\\}\\}")

func appendTechnicalResidue(vs []Violation, text string) []Violation {
	matches := technicalRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return vs
	}
	return append(vs, Violation{
		Rule:     "technical_residue",
		Target:   strings.Join(distinctExamples(matches, 3, false), ", "),
		Actual:   len(matches),
		Severity: SeverityWarning,
	})
}

// brokenWordRe 匹配被段落分隔切断的单词：一段以字母结尾（无句末标点），空行后又以小写字母开头。
// 越南语正文实测：「Thanh Vân Tông, Ng」+ 空行 +「ọc Lâm dừng bước」——人名 Ngọc 被劈成两半。
var brokenWordRe = regexp.MustCompile(`\p{L}\n[ \t]*\n[ \t]*\p{Ll}`)

func appendBrokenWords(vs []Violation, text string) []Violation {
	matches := brokenWordRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return vs
	}
	return append(vs, Violation{
		Rule:     "broken_word",
		Target:   strings.Join(strings.Fields(matches[0]), "⏎"),
		Actual:   len(matches),
		Severity: SeverityWarning,
	})
}
