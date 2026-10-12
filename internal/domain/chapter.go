package domain

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// ReviewInterval 全局审阅间隔（每 N 章触发一次）。
const ReviewInterval = 5

// ShouldReview 根据已完成章节数判断是否需要全局审阅（短篇/中篇模式）。
func ShouldReview(completedCount int) (bool, string) {
	if completedCount > 0 && completedCount%ReviewInterval == 0 {
		return true, fmt.Sprintf("Đã hoàn thành %d chương, kích hoạt duyệt toàn cục", completedCount)
	}
	return false, ""
}

// ShouldArcReview 长篇模式下判断是否需要弧级/卷级评审。
func ShouldArcReview(isArcEnd, isVolumeEnd bool, volume, arc int) (bool, string) {
	if isVolumeEnd {
		return true, fmt.Sprintf("Kết thúc cung %d của quyển %d (hết quyển), kích hoạt duyệt cấp cung + cấp quyển", arc, volume)
	}
	if isArcEnd {
		return true, fmt.Sprintf("Kết thúc cung %d của quyển %d, kích hoạt duyệt cấp cung", arc, volume)
	}
	return false, ""
}

// WordCount 计算正文长度。
//
// 中文（language=zh）按 rune 计字，保持上游行为。
// 越南语（language=vi）按"词"计：一个词 = 一段连续的字母/组合附标/数字，
// 即以空白与标点分隔的音节。连字符拆开计数（"tay-chân" 计 2，越南语按音节计长），
// 数字内部夹在两个数字之间的 '.' 或 ',' 不断词（"3.000"、"2,5" 各计 1）。
func WordCount(content string) int {
	if !lang.IsVI() {
		return utf8.RuneCountInString(content)
	}
	return countWordsVI(content)
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r)
}

func countWordsVI(content string) int {
	runes := []rune(content)
	count := 0
	inWord := false
	for i, r := range runes {
		if isWordRune(r) {
			if !inWord {
				count++
				inWord = true
			}
			continue
		}
		if inWord && (r == '.' || r == ',') && i > 0 && i+1 < len(runes) &&
			unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
			continue
		}
		inWord = false
	}
	return count
}

// HookTypeCounts 统计 hook_history 中各钩子类型的出现次数（跳过空位）。
// 只给序列时模型会自己数且数错，凡是能算的事实都由代码算好再交给它。
func HookTypeCounts(history []string) map[string]int {
	out := make(map[string]int, len(history))
	for _, h := range history {
		if h != "" {
			out[h]++
		}
	}
	return out
}
