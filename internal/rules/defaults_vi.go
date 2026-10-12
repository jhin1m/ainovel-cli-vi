package rules

import "github.com/voocel/ainovel-cli/internal/lang"

// SystemDefaults 是代码内置的机械基线（最低优先级来源），不走 LLM 归一化。
// 按 language 选择基线：zh 用上游中文表（defaults_zh.go），vi 用越南语表。
func SystemDefaults() Candidate {
	s := systemDefaultsVI()
	if !lang.IsVI() {
		s = systemDefaultsZH()
	}
	return Candidate{Source: "system_defaults", Structured: s}
}

// systemDefaultsVI 是越南语写作的机械基线。
//
// 选词原则：只收 AI 译腔/网文腔里高频、可按字面匹配的固定串，不收普通虚词。
// 检查按整词、不分大小写匹配（见 countPhrase），所以 "như một" 不会命中 "nhưng một"。
//   - forbidden_phrases：几乎只出现在 AI 套话里的定长句式，出现即 error。
//   - fatigue_words：正常写作也会用、但模型会用到泛滥的词，按章阈值计 warning。
//     阈值对应中文表的同类词（不禁→không khỏi、仿佛→dường như、一丝→một tia…），
//     章长约 3000 词，与中文 3000 字同量级，故阈值沿用。
func systemDefaultsVI() Structured {
	return Structured{
		ForbiddenPhrases: []string{
			"ở một mức độ nào đó",
			"điều đáng chú ý là",
			"ngũ vị tạp trần",
			"trăm mối cảm xúc đan xen",
			"một cảm giác khó tả",
			"không thể diễn tả bằng lời",
		},
		FatigueWords: map[string]int{
			// 情绪/语气标签
			"không khỏi": 1, "bất giác": 1, "không ngờ": 2, "dường như": 2, "như thể": 2,
			"tựa như": 1, "ngoài ra": 1, "tuy nhiên": 2,
			// 量词式意象
			"một tia": 2, "một vệt": 2, "một làn": 2, "như một": 3,
			// 套路化的表情/生理反应
			"khóe môi khẽ nhếch": 1, "khóe miệng khẽ nhếch": 1, "một luồng khí lạnh": 1,
			"hít sâu một hơi": 1, "đồng tử co rút": 1, "ánh mắt lóe lên": 1,
			// 节拍词（对应 沉默了 / 没有说话 / X息）
			"trầm mặc": 2, "im lặng một lúc": 2, "không nói gì": 2,
			"vài hơi thở": 3, "mấy hơi thở": 2,
		},
	}
}

// homeRulesReadme 返回首次引导时写入 ~/.ainovel/rules/README.txt 的说明（按 language）。
// 刻意用 .txt 后缀而非 .md——扫描只认 .md，这份说明不会被当成规则归一化。
func homeRulesReadme() string {
	if lang.IsVI() {
		return homeRulesReadmeVI
	}
	return homeRulesReadmeZH
}

const homeRulesReadmeVI = `Đặt ở đây các sở thích viết dùng chung, áp dụng cho mọi cuốn truyện.

Tạo một file .md mới (vd. my-style.md), cứ viết yêu cầu bằng lời thường —
không cần định dạng, không cần YAML:

    # Nhân vật
    - Nhân vật chính Lâm Trần đừng viết thành thánh mẫu, ngoài lạnh trong nóng là được
    # Văn phong
    - Dùng cảm giác cơ thể (đốt ngón tay trắng bệch) thay cho nhãn cảm xúc (căng thẳng)
    - Lời thoại đừng quá sách vở, mỗi chương khoảng 3000 chữ
    - Không dùng kiểu giọng AI như "ở một mức độ nào đó"

Viết xong không cần lo định dạng: hệ thống sẽ dùng model chuẩn hoá các yêu cầu tự nhiên
này thành ràng buộc có cấu trúc (khoảng số chữ, từ cấm, ngưỡng từ mỏi…), tự tuân theo
khi viết và tự kiểm khi nộp chương.

Nhiều file .md được gộp theo thứ tự tên file; file ẩn bắt đầu bằng dấu chấm và file không
phải .md đều bị bỏ qua (nên README.txt này không bị coi là luật).

Mốc cơ bản cho các câu sáo AI và từ mỏi thường gặp đã có sẵn, dùng ngay được, không viết
gì cũng không sao.

Thứ tự ưu tiên (cao → thấp): ./.ainovel/rules/*.md (truyện này) > ~/.ainovel/rules/*.md (ở đây) > mặc định có sẵn
`
