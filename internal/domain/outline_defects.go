package domain

import (
	"fmt"
	"strings"
)

// 同一钩子/核心事件重复到此次数即判为大纲空转。钩子重复 2 次可能是有意的两段式，
// 3 次起读者被同一个悬念挂三章而无人兑现；核心事件重复 2 次即说明两章写的是同一件事。
const (
	maxHookRepeat      = 3
	maxCoreEventRepeat = 2
)

// StalledOutline 检出"原地打转"的大纲：章节标题各异，但钩子或核心事件是同一句
// 复制多份。Writer 会忠实执行——每章重述上章、再添一点，读起来像改写而非续写。
// Writer 与 Editor 都只看单章，单章本身自洽，因此必须在大纲落盘处按整批比对。
//
// 返回空串表示通过；否则为可直接回给规划者的诊断。
func StalledOutline(entries []OutlineEntry) string {
	hooks := map[string]int{}
	events := map[string]int{}
	for _, e := range entries {
		if h := strings.TrimSpace(e.Hook); h != "" {
			hooks[h]++
		}
		if c := strings.TrimSpace(e.CoreEvent); c != "" {
			events[c]++
		}
	}
	if worst, n := mostRepeated(hooks); n >= maxHookRepeat {
		return fmt.Sprintf("Dàn ý giậm chân tại chỗ: cùng một hook lặp ở %d/%d chương — %q. "+
			"Hook của mỗi chương phải là hệ quả mới sinh ra trong chương đó và được chương sau đáp lại; hãy viết lại từng chương",
			n, len(entries), truncateRunes(worst, 40))
	}
	if worst, n := mostRepeated(events); n >= maxCoreEventRepeat {
		return fmt.Sprintf("Dàn ý giậm chân tại chỗ: cùng một core_event lặp ở %d chương — %q. "+
			"Mỗi chương phải xảy ra chuyện khác và làm thay đổi tình thế; hãy viết lại từng chương",
			n, truncateRunes(worst, 40))
	}
	return ""
}

func mostRepeated(m map[string]int) (string, int) {
	var key string
	best := 0
	for k, n := range m {
		// 并列时取字典序最小者，保证诊断文本稳定。
		if n > best || (n == best && k < key) {
			key, best = k, n
		}
	}
	return key, best
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// SkeletonArcs 列出尚未展开的骨架弧，用于完本前置校验。
//
// 扁平大纲由 FlattenOutline 从已展开的弧派生，骨架弧贡献 0 章，对"大纲内还有
// 未写章节"的比对完全隐形：整弧未展开时写完已展开章节即可宣告完本。
func SkeletonArcs(volumes []VolumeOutline) []string {
	var out []string
	for vi := range volumes {
		for ai := range volumes[vi].Arcs {
			if a := &volumes[vi].Arcs[ai]; !a.IsExpanded() {
				out = append(out, fmt.Sprintf("quyển %d cung %d \"%s\"", volumes[vi].Index, a.Index, a.Title))
			}
		}
	}
	return out
}
