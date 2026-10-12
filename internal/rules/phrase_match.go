package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// countPhrase 统计 phrase 在 text 中出现的次数，供 forbidden_phrases / fatigue_words 使用。
//
// zh：字面子串计数（strings.Count），保持上游行为。
// vi：越南语按空格分音节，字面子串会误命中（用户疲劳词 "anh" 会命中 "nhanh"、"lạnh"），
// 且句首大写（"Tuy nhiên"）会漏计。因此：
//   - 两边先做 NFC 归一化（模型偶尔输出组合附标形式）并转小写；
//   - 只在整词边界计数：短语首/尾若是非汉字字母，则相邻字符不得是字母/附标/数字。
//     汉字没有词界，遇到汉字边缘不做边界要求，用户写的中文禁词在 vi 下仍可命中。
func countPhrase(text, phrase string) int {
	if !lang.IsVI() {
		return strings.Count(text, phrase)
	}
	t := strings.ToLower(norm.NFC.String(text))
	p := strings.ToLower(norm.NFC.String(phrase))
	if p == "" {
		return 0
	}
	first, _ := utf8.DecodeRuneInString(p)
	last, _ := utf8.DecodeLastRuneInString(p)
	needLeft, needRight := needsBoundary(first), needsBoundary(last)

	n := 0
	for i := 0; i < len(t); {
		j := strings.Index(t[i:], p)
		if j < 0 {
			break
		}
		start, end := i+j, i+j+len(p)
		ok := true
		if needLeft && start > 0 {
			prev, _ := utf8.DecodeLastRuneInString(t[:start])
			ok = !isWordPart(prev)
		}
		if ok && needRight && end < len(t) {
			next, _ := utf8.DecodeRuneInString(t[end:])
			ok = !isWordPart(next)
		}
		if ok {
			n++
			i = end
			continue
		}
		// 未命中边界：前进一个 rune 继续找，允许重叠位置上的合法匹配。
		_, size := utf8.DecodeRuneInString(t[start:])
		i = start + size
	}
	return n
}

// isWordPart 判定 rune 是否属于一个拉丁词的组成部分（字母、组合附标、数字）。
func isWordPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r)
}

// needsBoundary：短语边缘是非汉字的字母/数字时才需要整词边界。
func needsBoundary(r rune) bool {
	return isWordPart(r) && !unicode.Is(unicode.Han, r)
}
