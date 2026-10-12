package tools

import (
	"strings"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/lang"
)

// premiseHeadingAliases ánh xạ tiêu đề premise (tiếng Trung lẫn tiếng Việt) về khoá
// canonical tiếng Trung. Khoá canonical chỉ dùng nội bộ; ra ngoài đổi sang nhãn theo
// ngôn ngữ hiện tại qua premiseHeadingLabel.
var premiseHeadingAliases = map[string]string{
	"题材定位":    "题材定位",    //check-han:ok
	"题材和基调":   "题材和基调",   //check-han:ok
	"核心冲突":    "核心冲突",    //check-han:ok
	"主角目标":    "主角目标",    //check-han:ok
	"结局方向":    "终局方向",    //check-han:ok
	"终局方向":    "终局方向",    //check-han:ok
	"写作禁区":    "写作禁区",    //check-han:ok
	"差异化卖点":   "差异化卖点",   //check-han:ok
	"差异化钩子":   "差异化钩子",   //check-han:ok
	"核心兑现承诺":  "核心兑现承诺",  //check-han:ok
	"故事引擎":    "故事引擎",    //check-han:ok
	"关系/成长主线": "关系/成长主线", //check-han:ok
	"升级路径":    "升级路径",    //check-han:ok
	"中段转折":    "中段转折",    //check-han:ok
	"中期转向":    "中段转折",    //check-han:ok
	"终局命题":    "终局命题",    //check-han:ok
	"短篇适配性":   "短篇适配性",   //check-han:ok
	"本作为什么适合短篇/单卷收束": "短篇适配性", //check-han:ok

	// Tiêu đề tiếng Việt, khớp nguyên văn với assets/prompts/vi/architect-*.md.
	// So khớp không phân biệt hoa thường (xem canonicalPremiseHeading).
	"thể loại và giọng điệu":     "题材和基调",   //check-han:ok
	"định vị thể loại":           "题材定位",    //check-han:ok
	"xung đột cốt lõi":           "核心冲突",    //check-han:ok
	"mục tiêu nhân vật chính":    "主角目标",    //check-han:ok
	"hướng kết cục":              "终局方向",    //check-han:ok
	"vùng cấm khi viết":          "写作禁区",    //check-han:ok
	"điểm bán khác biệt":         "差异化卖点",   //check-han:ok
	"móc câu khác biệt":          "差异化钩子",   //check-han:ok
	"lời hứa cốt lõi":            "核心兑现承诺",  //check-han:ok
	"động cơ câu chuyện":         "故事引擎",    //check-han:ok
	"tuyến quan hệ/trưởng thành": "关系/成长主线", //check-han:ok
	"lộ trình thăng cấp":         "升级路径",    //check-han:ok
	"chuyển hướng giữa truyện":   "中段转折",    //check-han:ok
	"bước ngoặt giữa truyện":     "中段转折",    //check-han:ok
	"câu hỏi kết cục":            "终局命题",    //check-han:ok
	"độ phù hợp truyện ngắn":     "短篇适配性",   //check-han:ok
	"vì sao truyện này phù hợp với dạng truyện ngắn/khép lại trong một quyển": "短篇适配性", //check-han:ok
}

// premiseHeadingVI là nhãn tiếng Việt của từng khoá canonical, dùng khi language=vi.
var premiseHeadingVI = map[string]string{
	"题材和基调":   "Thể loại và giọng điệu",     //check-han:ok
	"题材定位":    "Định vị thể loại",           //check-han:ok
	"核心冲突":    "Xung đột cốt lõi",           //check-han:ok
	"主角目标":    "Mục tiêu nhân vật chính",    //check-han:ok
	"终局方向":    "Hướng kết cục",              //check-han:ok
	"写作禁区":    "Vùng cấm khi viết",          //check-han:ok
	"差异化卖点":   "Điểm bán khác biệt",         //check-han:ok
	"差异化钩子":   "Móc câu khác biệt",          //check-han:ok
	"核心兑现承诺":  "Lời hứa cốt lõi",            //check-han:ok
	"故事引擎":    "Động cơ câu chuyện",         //check-han:ok
	"关系/成长主线": "Tuyến quan hệ/trưởng thành", //check-han:ok
	"升级路径":    "Lộ trình thăng cấp",         //check-han:ok
	"中段转折":    "Chuyển hướng giữa truyện",   //check-han:ok
	"终局命题":    "Câu hỏi kết cục",            //check-han:ok
	"短篇适配性":   "Độ phù hợp truyện ngắn",     //check-han:ok
}

// premiseHeadingLabel đổi khoá canonical sang nhãn hiển thị theo ngôn ngữ hiện tại.
func premiseHeadingLabel(canonical string) string {
	if lang.IsVI() {
		if label, ok := premiseHeadingVI[canonical]; ok {
			return label
		}
	}
	return canonical
}

func parsePremiseSections(premise string) map[string]string {
	lines := strings.Split(premise, "\n")
	sections := make(map[string]string)
	var current string
	var body []string

	flush := func() {
		if current == "" {
			return
		}
		text := strings.TrimSpace(strings.Join(body, "\n"))
		if text != "" {
			sections[premiseHeadingLabel(current)] = text
		}
		body = body[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if heading, ok := canonicalPremiseHeading(trimmed); ok {
			flush()
			current = heading
			continue
		}
		if current != "" {
			body = append(body, line)
		}
	}
	flush()
	return sections
}

func canonicalPremiseHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if title == "" {
		return "", false
	}
	if canonical, ok := premiseHeadingAliases[title]; ok {
		return canonical, true
	}
	if canonical, ok := premiseHeadingAliases[strings.ToLower(title)]; ok {
		return canonical, true
	}
	// Model hay chép cả phần chú thích sau tên tiêu đề, vd. "Định vị thể loại (độc giả
	// mục tiêu, ...)" hoặc "Móc câu khác biệt: điểm độc đáo ..." — bỏ phần đó rồi thử lại.
	if cut := strings.IndexAny(title, "(（:："); cut > 0 {
		short := strings.ToLower(strings.TrimSpace(title[:cut]))
		if canonical, ok := premiseHeadingAliases[short]; ok {
			return canonical, true
		}
	}
	return "", false
}

func premiseStructure(premise string, tier domain.PlanningTier) map[string]any {
	sections := parsePremiseSections(premise)
	required := requiredPremiseHeadings(tier)
	found := make([]string, 0, len(required))
	var missing []string
	for _, canonical := range required {
		heading := premiseHeadingLabel(canonical)
		if _, ok := sections[heading]; ok {
			found = append(found, heading)
			continue
		}
		missing = append(missing, heading)
	}

	structure := map[string]any{
		"template_ready": len(missing) == 0,
		"found":          found,
		"missing":        missing,
	}
	if len(sections) > 0 {
		structure["section_count"] = len(sections)
	}
	return structure
}

// requiredPremiseHeadings trả về khoá canonical của các tiêu đề bắt buộc theo tier.
func requiredPremiseHeadings(tier domain.PlanningTier) []string {
	common := []string{
		"题材和基调",  //check-han:ok
		"题材定位",   //check-han:ok
		"核心冲突",   //check-han:ok
		"主角目标",   //check-han:ok
		"终局方向",   //check-han:ok
		"写作禁区",   //check-han:ok
		"差异化卖点",  //check-han:ok
		"差异化钩子",  //check-han:ok
		"核心兑现承诺", //check-han:ok
	}

	switch tier {
	case domain.PlanningTierLong:
		return append(common,
			"故事引擎",    //check-han:ok
			"关系/成长主线", //check-han:ok
			"升级路径",    //check-han:ok
			"中段转折",    //check-han:ok
			"终局命题",    //check-han:ok
		)
	case domain.PlanningTierMid:
		return append(common,
			"故事引擎", //check-han:ok
			"中段转折", //check-han:ok
		)
	case domain.PlanningTierShort:
		return append(common,
			"短篇适配性", //check-han:ok
		)
	default:
		return common
	}
}
