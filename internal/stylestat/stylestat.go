// Package stylestat 对已写正文做全书级风格统计，产出纯事实。
//
// 动机：弧内评审窗口（~10 章）对全书级模式固化天然失明——句式 tic 章均几十次、
// 章末形态同构、跨章复读，单章看每处都"正常"，只有全书统计能暴露。统计归代码
// （确定性、零幻觉），裁定归 LLM（editor 按数字判维度分，writer 据此自避免）。
// Compute 供离线评测一次性全量计算；运行时使用 Tracker 按章节增量维护。
package stylestat

import (
	"regexp"
	"sort"
	"strings"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/lang"
	"github.com/voocel/ainovel-cli/internal/utils"
)

// minChapters 少于此章数不出统计——样本太小，频率没有意义。
const minChapters = 5

// phraseWindow 动态短语挖掘只看最近 N 章：writer 需要避免的是"现在的口头禅"。
const phraseWindow = 20

// patternDef là một loại mẫu câu cố định (tic văn phong AI) cần đếm.
// Số đếm là gần đúng (regex không phân tích ngữ pháp), dùng để so với chính nền
// của cuốn sách theo thời gian; độ chính xác tuyệt đối không quan trọng.
type patternDef struct {
	name string
	re   *regexp.Regexp
}

// profile gom mọi dữ liệu và quy tắc phụ thuộc ngôn ngữ của thống kê văn phong:
// danh sách mẫu câu, cách tách câu, nhận diện mở đầu bằng thời gian, tiền tố tiêu đề
// chương và cách khai thác cụm từ tần suất cao.
type profile struct {
	patterns      []patternDef
	sentenceSplit *regexp.Regexp
	openingTime   *regexp.Regexp
	titlePrefix   *regexp.Regexp
	// displayRunes là số rune tối đa khi hiển thị câu lặp (văn Latin cần dài hơn chữ Hán).
	displayRunes int
	minePhrases  func(chapters, stopwords []string) []PhraseStat
}

// activeProfile chọn profile theo ngôn ngữ sáng tác của tiến trình.
func activeProfile() *profile {
	if lang.IsVI() {
		return viProfile
	}
	return zhProfile
}

// pickPhrases sắp xếp ứng viên theo tần suất (cùng tần suất thì ưu tiên dài hơn),
// lấy tối đa 8 cụm và bỏ cụm trùng lặp theo hàm contains.
func pickPhrases(counts map[string]int, threshold int, reject func(string) bool, contains func(a, b string) bool) []PhraseStat {
	type cand struct {
		text  string
		count int
	}
	var cands []cand
	for g, c := range counts {
		if c < threshold || reject(g) {
			continue
		}
		cands = append(cands, cand{g, c})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].count != cands[j].count {
			return cands[i].count > cands[j].count
		}
		// 同频取更长的（信息量更大），再按字典序稳定排序
		if len(cands[i].text) != len(cands[j].text) {
			return len(cands[i].text) > len(cands[j].text)
		}
		return cands[i].text < cands[j].text
	})

	var out []PhraseStat
	for _, c := range cands {
		if len(out) >= 8 {
			break
		}
		dup := false
		for _, picked := range out {
			if contains(picked.Text, c.text) || contains(c.text, picked.Text) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, PhraseStat{Text: c.text, Count: c.count})
		}
	}
	return out
}

// Input 统计输入。Chapters 按章号升序；Stopwords 为角色名等专有名词，
// 动态短语挖掘时跳过（出场人名天然高频，不是文风问题）。
type Input struct {
	Chapters  []string
	Titles    []string
	Stopwords []string
}

// Stats 全书风格统计结果。所有字段都是事实计数，不含任何裁定或指令。
type Stats struct {
	Chapters          int            `json:"chapters"`
	Patterns          []PatternStat  `json:"patterns,omitempty"`
	TopPhrases        []PhraseStat   `json:"top_phrases,omitempty"`
	RepeatedSentences []SentenceStat `json:"repeated_sentences,omitempty"`
	Ending            EndingStat     `json:"ending"`
	OpeningTimeRate   float64        `json:"opening_time_rate"`
	TitleFormats      *TitleStat     `json:"title_formats,omitempty"`
}

// PatternStat 固定句式模式类的全书计数（通用 AI 文风 tic）。
type PatternStat struct {
	Name       string  `json:"name"`
	Total      int     `json:"total"`
	PerChapter float64 `json:"per_chapter"`
}

// PhraseStat 最近 phraseWindow 章内挖掘出的高频短语。
type PhraseStat struct {
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// SentenceStat 跨章逐字重复的长句（复读交代的直接证据）。
type SentenceStat struct {
	Text     string `json:"text"`
	Chapters int    `json:"chapters"`
	Count    int    `json:"count"`
}

// EndingStat 章末行形态分布。短结尾本身合法，全书同构才是问题。
type EndingStat struct {
	ShortRatio  float64 `json:"short_ratio"`
	MedianRunes int     `json:"median_runes"`
}

// TitleStat 章节标题「第N章」前缀混用计数（混用=机制痕迹暴露在产物里）。
type TitleStat struct {
	WithPrefix    int `json:"with_prefix"`
	WithoutPrefix int `json:"without_prefix"`
}

// shortEndingRunes 末行不超过此字数计为"短结尾"（tiếng Việt: số từ/âm tiết, xem textLen）。
const shortEndingRunes = 30

// minSentenceLen là độ dài tối thiểu (chữ Hán hoặc âm tiết Việt) để một câu được tính
// vào thống kê câu lặp nguyên văn xuyên chương.
const minSentenceLen = 12

// textLen đo độ dài nội dung truyện theo ngôn ngữ hiện hành qua domain.WordCount
// (zh: số rune, vi: số từ tách theo khoảng trắng). Ngưỡng shortEndingRunes và
// minSentenceLen giữ nguyên mức số cho cả hai ngôn ngữ: một chữ Hán tương ứng xấp xỉ
// một âm tiết tiếng Việt, mà mỗi "từ" tách theo khoảng trắng chính là một âm tiết,
// nên câu 12 chữ Hán ≈ câu 12 âm tiết, dòng cuối 30 chữ ≈ 30 âm tiết.
func textLen(s string) int { return domain.WordCount(s) }

// Compute 计算全书风格统计；章数不足时返回 nil。
func Compute(in Input) *Stats {
	n := len(in.Chapters)
	if n < minChapters {
		return nil
	}
	all := strings.Join(in.Chapters, "\n")
	prof := activeProfile()

	s := &Stats{Chapters: n}
	for _, def := range prof.patterns {
		total := len(def.re.FindAllStringIndex(all, -1))
		if total == 0 {
			continue
		}
		s.Patterns = append(s.Patterns, PatternStat{
			Name:       def.name,
			Total:      total,
			PerChapter: round1(float64(total) / float64(n)),
		})
	}
	s.TopPhrases = prof.minePhrases(recentWindow(in.Chapters), in.Stopwords)
	s.RepeatedSentences = repeatedSentences(prof, in.Chapters)
	s.Ending = endingShape(in.Chapters)
	s.OpeningTimeRate = openingTimeRate(prof, in.Chapters)
	s.TitleFormats = titleFormats(prof, in.Titles)
	return s
}

func recentWindow(chapters []string) []string {
	if len(chapters) <= phraseWindow {
		return chapters
	}
	return chapters[len(chapters)-phraseWindow:]
}

// repeatedSentences 找跨 ≥3 章逐字重复的 ≥12 字句子，按次数取 top 5。
func repeatedSentences(prof *profile, chapters []string) []SentenceStat {
	type rec struct {
		count    int
		chapters map[int]struct{}
	}
	seen := make(map[string]*rec)
	for ci, text := range chapters {
		for sent, count := range chapterSentenceCounts(prof, text) {
			r := seen[sent]
			if r == nil {
				r = &rec{chapters: make(map[int]struct{})}
				seen[sent] = r
			}
			r.count += count
			r.chapters[ci] = struct{}{}
		}
	}

	var out []SentenceStat
	for sent, r := range seen {
		if len(r.chapters) < 3 {
			continue
		}
		out = append(out, SentenceStat{Text: utils.TruncateRunes(sent, prof.displayRunes), Chapters: len(r.chapters), Count: r.count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Text < out[j].Text
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// trimWrappedQuotes 剥掉包裹引号：同一句台词带/不带前引号不应算成两条。
func trimWrappedQuotes(sentence string) string {
	return strings.Trim(strings.TrimSpace(sentence), `"“”‘’「」『』`)
}

func endingShape(chapters []string) EndingStat {
	var lengths []int
	short := 0
	for _, text := range chapters {
		line := lastNonEmptyLine(text)
		if line == "" {
			continue
		}
		n := textLen(line)
		lengths = append(lengths, n)
		if n <= shortEndingRunes {
			short++
		}
	}
	if len(lengths) == 0 {
		return EndingStat{}
	}
	sort.Ints(lengths)
	return EndingStat{
		ShortRatio:  round2(float64(short) / float64(len(lengths))),
		MedianRunes: lengths[len(lengths)/2],
	}
}

func openingTimeRate(prof *profile, chapters []string) float64 {
	hit := 0
	for _, text := range chapters {
		if prof.openingTime.MatchString(firstParagraph(text)) {
			hit++
		}
	}
	return round2(float64(hit) / float64(len(chapters)))
}

func titleFormats(prof *profile, titles []string) *TitleStat {
	if len(titles) == 0 {
		return nil
	}
	t := &TitleStat{}
	for _, title := range titles {
		if strings.TrimSpace(title) == "" {
			continue
		}
		if prof.titlePrefix.MatchString(strings.TrimSpace(title)) {
			t.WithPrefix++
		} else {
			t.WithoutPrefix++
		}
	}
	// 只有混用才值得上报；统一格式不是事实意义上的问题
	if t.WithPrefix == 0 || t.WithoutPrefix == 0 {
		return nil
	}
	return t
}

func lastNonEmptyLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// firstParagraph 取第一个非空且非 Markdown 标题的行（章文件首行常是 # 标题）。
func firstParagraph(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
