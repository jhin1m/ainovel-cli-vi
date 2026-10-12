package tools

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/lang"
)

// withTestLang đặt ngôn ngữ cho một test rồi trả lại tiếng Việt (mặc định) khi xong.
func withTestLang(t *testing.T, c lang.Code) {
	t.Helper()
	lang.Set(c)
	t.Cleanup(func() { lang.Set(lang.VI) })
}

func TestParsePremiseSections(t *testing.T) {
	withTestLang(t, lang.ZH)
	premise := `# Premise

## 题材和基调
东方玄幻，冷硬成长。

## 题材定位
东方玄幻升级流，面向追求爽点和关系推进的读者。

## 核心冲突
主角必须在宗门规则与个人良知之间做选择。

## 中期转向
旧有修炼路线失效，必须转向禁术体系。
`

	sections := parsePremiseSections(premise)
	if sections["题材和基调"] == "" {
		t.Fatalf("expected 题材和基调 section, got %+v", sections)
	}
	if sections["题材定位"] == "" {
		t.Fatalf("expected 题材定位 section, got %+v", sections)
	}
	if sections["核心冲突"] == "" {
		t.Fatalf("expected 核心冲突 section, got %+v", sections)
	}
	if sections["中段转折"] == "" {
		t.Fatalf("expected 中期转向 alias normalized to 中段转折, got %+v", sections)
	}
}

func TestPremiseStructure(t *testing.T) {
	withTestLang(t, lang.ZH)
	premise := `## 题材和基调
升级流，偏冷硬。

## 题材定位
升级流

## 核心冲突
冲突

## 主角目标
目标

## 终局方向
终局

## 写作禁区
禁区

## 差异化卖点
卖点

## 差异化钩子
钩子

## 核心兑现承诺
兑现

## 故事引擎
引擎

## 中段转折
转折
`

	structure := premiseStructure(premise, domain.PlanningTierMid)
	if ready, _ := structure["template_ready"].(bool); !ready {
		t.Fatalf("expected template_ready, got %+v", structure)
	}
	missing, _ := structure["missing"].([]string)
	if len(missing) != 0 {
		t.Fatalf("expected no missing headings, got %+v", missing)
	}
}

func TestPremiseStructureShortAcceptsLegacyHeadingAlias(t *testing.T) {
	withTestLang(t, lang.ZH)
	premise := `## 题材和基调
单卷高压营救。

## 题材定位
短篇高密度冒险。

## 核心冲突
主角必须在一夜内救出人质。

## 主角目标
救出人质并活着离开。

## 结局方向
完成任务但付出代价。

## 写作禁区
不扩展成长期连载。

## 差异化卖点
时限压力与连续反转。

## 差异化钩子
每次选择都缩短救援时间。

## 核心兑现承诺
紧迫感、抉择与反转。

## 本作为什么适合短篇/单卷收束
核心矛盾和人物弧线都能在单次任务中完成。
`

	structure := premiseStructure(premise, domain.PlanningTierShort)
	if ready, _ := structure["template_ready"].(bool); !ready {
		t.Fatalf("expected short template_ready, got %+v", structure)
	}
}

func TestParsePremiseSectionsVietnamese(t *testing.T) {
	withTestLang(t, lang.VI)
	premise := `# Tiền đề câu chuyện

## Thể loại và giọng điệu
Huyền huyễn phương Đông, trưởng thành lạnh lùng.

## Định vị thể loại (độc giả mục tiêu, điểm hấp dẫn cốt lõi)
Thăng cấp, nhắm tới độc giả thích nhịp nhanh.

## xung đột cốt lõi
Nhân vật chính phải chọn giữa môn quy và lương tâm.

## Bước ngoặt giữa truyện
Con đường tu luyện cũ mất tác dụng.
`

	sections := parsePremiseSections(premise)
	for _, heading := range []string{"Thể loại và giọng điệu", "Định vị thể loại", "Xung đột cốt lõi", "Chuyển hướng giữa truyện"} {
		if sections[heading] == "" {
			t.Fatalf("thiếu mục %q, có %+v", heading, sections)
		}
	}
}

func TestPremiseStructureVietnameseLong(t *testing.T) {
	withTestLang(t, lang.VI)
	premise := `# Tiền đề câu chuyện

## Thể loại và giọng điệu
a
## Định vị thể loại
a
## Xung đột cốt lõi
a
## Mục tiêu nhân vật chính
a
## Hướng kết cục (hướng mang tính chủ đề)
a
## Vùng cấm khi viết
a
## Điểm bán khác biệt
a
## Móc câu khác biệt: điểm độc đáo nhất
a
## Lời hứa cốt lõi
a
## Động cơ câu chuyện
a
## Tuyến quan hệ/trưởng thành
a
## Lộ trình thăng cấp
a
## Chuyển hướng giữa truyện
a
`

	structure := premiseStructure(premise, domain.PlanningTierLong)
	if ready, _ := structure["template_ready"].(bool); ready {
		t.Fatalf("thiếu Câu hỏi kết cục nhưng template_ready=true: %+v", structure)
	}
	missing, _ := structure["missing"].([]string)
	if len(missing) != 1 || missing[0] != "Câu hỏi kết cục" {
		t.Fatalf("missing phải là tiêu đề tiếng Việt [Câu hỏi kết cục], có %+v", missing)
	}

	structure = premiseStructure(premise+"## Câu hỏi kết cục\na\n", domain.PlanningTierLong)
	if ready, _ := structure["template_ready"].(bool); !ready {
		t.Fatalf("premise tiếng Việt đủ mục phải template_ready, có %+v", structure)
	}
	found, _ := structure["found"].([]string)
	if len(found) != 14 || found[0] != "Thể loại và giọng điệu" {
		t.Fatalf("found phải gồm 14 tiêu đề tiếng Việt, có %+v", found)
	}
}

func TestPremiseStructureVietnameseShort(t *testing.T) {
	withTestLang(t, lang.VI)
	premise := `## Thể loại và giọng điệu
a
## Định vị thể loại
a
## Xung đột cốt lõi
a
## Mục tiêu nhân vật chính
a
## Hướng kết cục
a
## Vùng cấm khi viết
a
## Điểm bán khác biệt
a
## Móc câu khác biệt
a
## Lời hứa cốt lõi
a
## Vì sao truyện này phù hợp với dạng truyện ngắn/khép lại trong một quyển
a
`
	structure := premiseStructure(premise, domain.PlanningTierShort)
	if ready, _ := structure["template_ready"].(bool); !ready {
		t.Fatalf("premise ngắn tiếng Việt phải template_ready, có %+v", structure)
	}
}
