//check-han:ignore-file
// Dữ liệu nhận diện văn phong chỉ có nghĩa khi language=zh: mẫu câu, hư từ và
// dấu câu tiếng Trung.

package stylestat

import (
	"regexp"
	"strings"
)

// zhProfile 通用 AI 文风句式模式（中文）。计数是近似（正则不做语法分析），
// 用途是本书自身的纵向基线对比，绝对精度不重要。
var zhProfile = &profile{
	patterns: []patternDef{
		{"矫正句『不是…(而)是…』", regexp.MustCompile(`不是[^。！？\n]{1,24}?[，、]?(?:而)?是`)},
		{"计时量词『X息/X瞬』", regexp.MustCompile(`[一两二三四五六七八九十几数半][息瞬]`)},
		{"明喻『像一/仿佛/如同/宛如』", regexp.MustCompile(`像一|仿佛|如同|宛如`)},
		{"沉默节拍『沉默了/没有说话/没有回头』", regexp.MustCompile(`沉默了|没有说话|没有回头`)},
		{"神态模板『眼中闪过/嘴角勾起/咬了咬唇』", regexp.MustCompile(`眼[中底]闪过|目光一凝|瞳孔一缩|眼眶微红|嘴角[微轻一]?[勾扬翘]|咬了咬唇|不可置信`)},
		{"躯体反应『心头一紧/身子一颤/倒吸凉气』", regexp.MustCompile(`心头一[紧沉颤]|身子一[颤震僵]|倒吸(?:了)?一口凉气`)},
		{"思维标记『心想/意识到/感到/觉得』", regexp.MustCompile(`心想|意识到|感到|觉得`)},
		{"抽象套话『一种说不出的/的意义在于』", regexp.MustCompile(`一种说不出的|说不清[的道]|的意义在于|真正的[^。！？\n]{1,10}是`)},
	},
	sentenceSplit: regexp.MustCompile(`[。！？\n]+`),
	openingTime:   regexp.MustCompile(`夜|清晨|黎明|天亮|醒来|晨光|一整夜`),
	titlePrefix:   regexp.MustCompile(`^#{0,2}\s*第[零〇一二三四五六七八九十百千万\d]+章`),
	displayRunes:  40,
	minePhrases:   minePhrasesZH,
}

// minePhrases 在窗口内挖掘 3-6 字高频短语。
// 过滤：含标点/空白、首尾虚词、命中专有名词；去重：与已选短语互为子串的丢弃。
func minePhrasesZH(chapters []string, stopwords []string) []PhraseStat {
	text := strings.Join(chapters, "\n")
	runes := []rune(text)
	threshold := max(8, len(chapters)/2)

	counts := make(map[string]int)
	for size := 3; size <= 6; size++ {
		for i := 0; i+size <= len(runes); i++ {
			gram := runes[i : i+size]
			if !validGram(gram) {
				continue
			}
			counts[string(gram)]++
		}
	}

	stopGrams := stopwordBigrams(stopwords)
	reject := func(g string) bool { return hitStopword(g, stopGrams) }
	return pickPhrases(counts, threshold, reject, strings.Contains)
}

// gramEdgeStop 首尾为这些虚词/代词的 n-gram 不是文风短语，跳过。
const gramEdgeStop = "的了着是在和与就也都还又把被他她它我你这那"

func validGram(gram []rune) bool {
	for _, r := range gram {
		if r < 0x4E00 || r > 0x9FFF { // 仅纯汉字片段
			return false
		}
	}
	if strings.ContainsRune(gramEdgeStop, gram[0]) || strings.ContainsRune(gramEdgeStop, gram[len(gram)-1]) {
		return false
	}
	return true
}

// stopwordBigrams 把专有名词拆成 2 字片段：人名常以部分形式入文
// （"九渊负手"含"九渊"），按整名匹配会漏网。宁可过滤偏严——短语事实少一条
// 无碍，人名混进口头禅清单才是噪声。
func stopwordBigrams(stopwords []string) []string {
	var grams []string
	for _, w := range stopwords {
		runes := []rune(strings.TrimSpace(w))
		if len(runes) < 2 {
			continue
		}
		for i := 0; i+2 <= len(runes); i++ {
			grams = append(grams, string(runes[i:i+2]))
		}
	}
	return grams
}

func hitStopword(gram string, stopGrams []string) bool {
	for _, g := range stopGrams {
		if strings.Contains(gram, g) {
			return true
		}
	}
	return false
}
