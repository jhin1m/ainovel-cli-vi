// Package userrules 是用户规则归一化的服务层：把各来源的自然语言规则经 LLM 结构化调用
// 归一化成候选结构化字段，再由 rules.BuildSnapshot 确定性合并成本书快照。
//
// 分层职责：
//   - rules 包：纯数据 + 确定性合并（Snapshot / Candidate / BuildSnapshot / SystemDefaults）
//   - 本包：LLM 归一化 + 编排 + 落盘（依赖 agentcore + store + rules）
//
// 归一化是增强路径，不是主创作的前置条件：任何来源失败都降级为 raw preferences，主创作必须继续。
package userrules

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/voocel/agentcore"
	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/internal/llmcontract"
	"github.com/voocel/ainovel-cli/internal/rules"
)

// normalizeMaxTokens 单次归一化的输出上限（思考 token 与 JSON 输出共享这一预算）。
// 归一化 JSON 本身很小（通常 <1k），这里留大头是给"无法关闭思考的推理模型"的思考预算——
// 留窄了思考会挤占 JSON 导致截断、解析失败。max_tokens 是上限不是计费量，调大不增成本。
const normalizeMaxTokens = 8192

// normalizeContract 紧邻边界 DTO：全字段 required、fatigue_words 用对象数组
// （strict 模式禁止动态 key 的 map），两种模式共用同一 DTO 约定。
var normalizeContract = llmcontract.Contract{
	Name:        "userrules_normalize",
	Description: "Chuẩn hoá quy tắc viết bằng ngôn ngữ tự nhiên của người dùng thành các trường có cấu trúc",
	Schema: schema.Object(
		schema.Property("structured", schema.Object(
			schema.Property("genre", schema.String("Thể loại; không có thì để chuỗi rỗng")).Required(),
			schema.Property("forbidden_chars", schema.Array("Các ký tự cấm xuất hiện", schema.String("Ký tự"))).Required(),
			schema.Property("forbidden_phrases", schema.Array("Các cụm từ cấm xuất hiện (so khớp nguyên văn chính xác)", schema.String("Cụm từ"))).Required(),
			schema.Property("fatigue_words", schema.Array("Từ gây mỏi và giới hạn số lần xuất hiện mỗi chương", schema.Object(
				schema.Property("word", schema.String("Từ gây mỏi")).Required(),
				schema.Property("max_per_chapter", schema.Int("Giới hạn số lần xuất hiện mỗi chương (số nguyên dương)")).Required(),
			))).Required(),
		)).Required(),
		schema.Property("preferences", schema.String("Sở thích về văn phong/nhân vật/thẩm mỹ bằng ngôn ngữ tự nhiên; không có thì để chuỗi rỗng")).Required(),
		schema.Property("uncertain", schema.Array("Các mục cố ý không đưa vào structured kèm lý do", schema.String("Mục"))).Required(),
	),
}

// Normalizer 把单个来源的自然语言规则归一化成 rules.Candidate。
type Normalizer struct {
	model agentcore.ChatModel
}

// NewNormalizer 用一个 ChatModel 构造归一化器。归一化是一次性启动工具，
// 应传入能力较强的模型（如 ModelSet 的默认模型），不必跟随写作的弱模型。
//
// 归一化不覆盖 thinking：显式 off 本身也是只有部分模型支持的推理参数，
// 普通 chat 模型会拒绝它。沿用 provider/model 默认，由 normalizeMaxTokens
// 为不可关闭思考的模型预留输出预算。
func NewNormalizer(model agentcore.ChatModel) *Normalizer {
	return &Normalizer{model: model}
}

// Normalize 归一化一个来源。失败返回 error（含真实原因），由调用方决定降级
// （Service.normalizeOrDegrade 落 degraded 候选）——技术错误不再伪装成正常结果，
// 终止错误（鉴权/权限等）不重试。
func (n *Normalizer) Normalize(ctx context.Context, source, text string) (rules.Candidate, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return rules.Candidate{Source: source}, nil
	}
	if n == nil || n.model == nil {
		return rules.Candidate{}, fmt.Errorf("chưa cấu hình model chuẩn hoá")
	}

	out, err := llmcontract.Execute(ctx, n.model, llmcontract.Request[normalizerOutput]{
		Contract:     normalizeContract,
		SystemPrompt: normalizerSystemPrompt,
		Payload:      text,
		Options:      []agentcore.CallOption{agentcore.WithMaxTokens(normalizeMaxTokens)},
		Validate: func(out *normalizerOutput) error {
			_, err := out.toCandidate(source)
			return err
		},
		Agent: "rules",
		Hooks: llmcontract.Hooks{
			Resolved: func(res llmcontract.Resolution) {
				slog.Debug("chọn giao thức chuẩn hoá quy tắc", "module", "rules", "source", source,
					"contract", normalizeContract.Name, "structured_mode", res.Mode,
					"capability_source", res.Source, "provider", res.Provider, "model", res.Model,
					"schema_fingerprint", normalizeContract.Fingerprint())
			},
			Correction: func(ev llmcontract.Correction) {
				slog.Warn("tự sửa đầu ra chuẩn hoá quy tắc", "module", "rules", "source", source,
					"attempt", ev.Attempt, "layer", ev.Layer, "structured_mode", ev.Mode, "err", ev.Err)
			},
		},
	})
	if err != nil {
		return rules.Candidate{}, fmt.Errorf("chuẩn hoá thất bại: %w", err)
	}
	return out.toCandidate(source)
}

// degraded 构造一个降级候选：归一化失败时把原文当作风格偏好，不提炼任何机械规则。
// uncertain 标注来源（便于回显"哪些来源未能解析"），但不含技术错误细节——技术错误只进日志。
func degraded(source, text string) rules.Candidate {
	return rules.Candidate{
		Source:      source,
		Preferences: text,
		Uncertain:   []string{source + ": chuẩn hoá thất bại, đã giữ nguyên văn làm sở thích văn phong (không rút ra quy tắc kiểm tra máy móc)"},
		Degraded:    true,
	}
}

// normalizerOutput 是归一化器约定的边界 DTO（两种模式共用）：uncertain 固定
// 字符串数组，fatigue_words 固定对象数组——形态由契约钉死，不再多形态猜测。
type normalizerOutput struct {
	Structured  normalizerStructured `json:"structured"`
	Preferences string               `json:"preferences"`
	Uncertain   []string             `json:"uncertain"`
}

type normalizerStructured struct {
	Genre            string             `json:"genre"`
	ForbiddenChars   []string           `json:"forbidden_chars"`
	ForbiddenPhrases []string           `json:"forbidden_phrases"`
	FatigueWords     []fatigueWordEntry `json:"fatigue_words"`
}

type fatigueWordEntry struct {
	Word          string `json:"word"`
	MaxPerChapter int    `json:"max_per_chapter"`
}

// toCandidate 校验边界 DTO 并转成领域候选：fatigue 条目须词非空、上限为正整数
// （校验错误可反馈给模型修正），领域侧仍是 map[string]int。
func (o normalizerOutput) toCandidate(source string) (rules.Candidate, error) {
	var fatigue map[string]int
	for _, e := range o.Structured.FatigueWords {
		word := strings.TrimSpace(e.Word)
		if word == "" {
			return rules.Candidate{}, fmt.Errorf("fatigue_words có mục từ rỗng")
		}
		if e.MaxPerChapter < 1 {
			return rules.Candidate{}, fmt.Errorf("fatigue_words[%q].max_per_chapter phải là số nguyên dương, nhận được %d", word, e.MaxPerChapter)
		}
		if fatigue == nil {
			fatigue = make(map[string]int, len(o.Structured.FatigueWords))
		}
		fatigue[word] = e.MaxPerChapter
	}
	return rules.Candidate{
		Source: source,
		Structured: rules.Structured{
			Genre:            strings.TrimSpace(o.Structured.Genre),
			ForbiddenChars:   nonEmpty(o.Structured.ForbiddenChars),
			ForbiddenPhrases: nonEmpty(o.Structured.ForbiddenPhrases),
			FatigueWords:     fatigue,
		},
		Preferences: strings.TrimSpace(o.Preferences),
		Uncertain:   nonEmpty(o.Uncertain),
	}, nil
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normalizerSystemPrompt 只描述归一化语义，输出结构由 normalizeContract 单点维护。
// 已用 10 条真实例子（含阈值发明陷阱）验证保守提升成立（10/10）。
const normalizerSystemPrompt = `Bạn là «bộ chuẩn hoá quy tắc» của hệ thống viết tiểu thuyết bằng AI. Bạn đọc các quy tắc viết dài hạn (ngôn ngữ tự nhiên) từ MỘT nguồn của người dùng, đưa những quy tắc rõ ràng và kiểm tra máy móc được vào structured, phần còn lại xếp vào preferences hoặc uncertain.

[Nâng cấp thận trọng — quan trọng nhất]
- Chỉ ghi vào structured khi người dùng nói rõ ràng, không mơ hồ.
- forbidden_chars/forbidden_phrases là mức error: chỉ nâng cấp khi có lệnh cấm rõ ràng kiểu «không được xuất hiện X / cấm dùng X / đừng viết X».
- fatigue_words: chỉ nâng cấp khi có đồng thời «từ cụ thể» và «ngưỡng số lần cụ thể»; «hạn chế dùng X / đừng dùng X hoài» mà không có con số thì đưa vào preferences, tuyệt đối không tự bịa ngưỡng.
- Mong muốn về số chữ/độ dài («mỗi chương 3000 chữ», «ngắn lại chút») luôn đưa vào preferences: độ dài chương là chuyện nhịp kể, được nắm bắt tự nhiên khi sáng tác, không kiểm tra máy móc.
- Những gì không kiểm tra máy móc được, không có ngưỡng rõ ràng hoặc phụ thuộc ngữ cảnh đều đưa vào preferences.
- Nguyên tắc: thà bỏ sót khỏi structured còn hơn nâng cấp sai (nâng sai sẽ báo lỗi nhầm ở mọi chương).
- Từ và cụm từ trong structured phải giữ nguyên văn đúng như người dùng viết (đúng ngôn ngữ, đúng dấu), vì chúng được so khớp nguyên văn với chính văn; không dịch, không đổi cách viết.

preferences dùng một đoạn văn ngôn ngữ tự nhiên dễ đọc để giữ lại các sở thích về văn phong, nhân vật và thẩm mỹ.
uncertain giải thích các mục bạn cố ý không đưa vào structured và lý do.`
