package rules

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/lang"
)

// 重复检测（language=vi）：模型陷入循环时会把整段/整句逐字重写，或把上一章照抄成新章。
//
// 阈值来自实测：一本 22 章的书里 19 章两项都是 0%，出事的三章分别是自重复 43.5%
// （同一段落出现 5 次）、跨章 32.2% 与 12.9%。信号是二元的，取 10% 给刻意重复的
// 副歌/咒诀留足余地。度量单位是词（domain.WordCount），与章长同口径。
const (
	selfDupThreshold  = 0.10
	crossDupThreshold = 0.10
	// paraMinWords：短段落天然会重复（"Hắn gật đầu."），只有成段文字逐字重现才算事故。
	paraMinWords = 12
	// sentenceMinWords：单句长到这个程度还逐字重现两次，几乎只可能是生成循环。
	sentenceMinWords = 15
)

// normalizeSpan 统一 Unicode 形式与空白，避免换行/多空格差异掩盖逐字重复。
func normalizeSpan(s string) string {
	return strings.Join(strings.Fields(norm.NFC.String(s)), " ")
}

// dupParagraphs 切出够长、值得比对的正文段落，跳过标题行。
func dupParagraphs(text string) []string {
	var out []string
	for p := range strings.SplitSeq(text, "\n\n") {
		p = normalizeSpan(p)
		if strings.HasPrefix(p, "#") {
			continue
		}
		if domain.WordCount(p) >= paraMinWords {
			out = append(out, p)
		}
	}
	return out
}

// appendSelfDuplication 报告本章内部的逐字重复：
//   - self_duplication（error）：重复段落占全章词数 ≥10%；
//   - repeated_sentence（warning）：未达段落阈值时，仍有长句逐字出现 ≥2 次。
func appendSelfDuplication(vs []Violation, text string) []Violation {
	paras := dupParagraphs(text)
	count := make(map[string]int, len(paras))
	total := 0
	for _, p := range paras {
		count[p]++
		total += domain.WordCount(p)
	}
	dup, worst, worstN := 0, "", 0
	for _, p := range paras {
		if n := count[p]; n > 1 && n > worstN {
			worst, worstN = p, n
		}
	}
	for p, n := range count {
		if n > 1 {
			dup += domain.WordCount(p) * (n - 1)
		}
	}
	if total > 0 && float64(dup)/float64(total) >= selfDupThreshold {
		return append(vs, Violation{
			Rule:     "self_duplication",
			Target:   fmt.Sprintf("×%d %s", worstN, truncateRunes(worst, 60)),
			Limit:    fmt.Sprintf("<%.0f%%", selfDupThreshold*100),
			Actual:   fmt.Sprintf("%.0f%%", 100*float64(dup)/float64(total)),
			Severity: SeverityError,
		})
	}
	return appendRepeatedSentences(vs, text)
}

// sentenceEndRe 按句末标点与换行切句；引号/破折号留在句内，比对前统一去掉首尾。
var sentenceEndRe = regexp.MustCompile(`[.!?…]+|\n`)

func appendRepeatedSentences(vs []Violation, text string) []Violation {
	count := make(map[string]int)
	var order []string
	for _, s := range sentenceEndRe.Split(norm.NFC.String(text), -1) {
		s = strings.Trim(normalizeSpan(s), " \"'“”‘’«»—–-")
		if domain.WordCount(s) < sentenceMinWords {
			continue
		}
		if count[s] == 0 {
			order = append(order, s)
		}
		count[s]++
	}
	repeated, extra, sample := 0, 0, ""
	for _, s := range order {
		if n := count[s]; n > 1 {
			repeated++
			extra += n - 1
			if sample == "" {
				sample = fmt.Sprintf("×%d %s", n, truncateRunes(s, 60))
			}
		}
	}
	if repeated == 0 {
		return vs
	}
	return append(vs, Violation{
		Rule:     "repeated_sentence",
		Target:   sample,
		Actual:   extra,
		Severity: SeverityWarning,
	})
}

// CheckAgainstPrevious 检出"新章其实是前几章的副本"（仅 language=vi）。
//
// Writer 有 read_chapter，会先读上一章再把它抄成续写。self_duplication 看不见这类事故：
// 它只在单章内统计，照抄的那一章内部完全干净，因此必须单独与上文比对。previous 为空则跳过。
func CheckAgainstPrevious(text string, previous []string) []Violation {
	if !lang.IsVI() || len(previous) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	for _, prev := range previous {
		for _, p := range dupParagraphs(prev) {
			seen[p] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	dup, total, sample := 0, 0, ""
	done := make(map[string]struct{})
	for _, p := range dupParagraphs(text) {
		if _, ok := done[p]; ok {
			continue
		}
		done[p] = struct{}{}
		n := domain.WordCount(p)
		total += n
		if _, ok := seen[p]; ok {
			dup += n
			if sample == "" {
				sample = p
			}
		}
	}
	if total == 0 || float64(dup)/float64(total) < crossDupThreshold {
		return nil
	}
	return []Violation{{
		Rule:     "copied_previous_chapter",
		Target:   truncateRunes(sample, 60),
		Limit:    fmt.Sprintf("<%.0f%%", crossDupThreshold*100),
		Actual:   fmt.Sprintf("%.0f%%", 100*float64(dup)/float64(total)),
		Severity: SeverityError,
	}}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
