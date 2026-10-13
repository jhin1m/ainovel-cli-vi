// Package keys quản lý API key LLM của người dùng (BYOK) và danh mục model chọn được.
package keys

import "github.com/voocel/ainovel-cli/internal/server/api"

// curatedModels là danh sách model chọn lọc theo provider, viết đúng mã model mà
// API của provider đó nhận. Model chỉ hiện ra cho người dùng khi registry có giá
// đầu vào và đầu ra > 0 — không có giá thì ngân sách không có tác dụng.
var curatedModels = map[string][]string{
	"openrouter": {
		"google/gemini-2.5-flash",
		"google/gemini-2.5-pro",
		"anthropic/claude-sonnet-4.5",
		"anthropic/claude-haiku-4.5",
		"openai/gpt-5-mini",
		"openai/gpt-4.1-mini",
		"deepseek/deepseek-v3.2",
		"z-ai/glm-4.6",
		"qwen/qwen3-max",
	},
	"anthropic": {"claude-sonnet-4-5", "claude-haiku-4-5", "claude-opus-4-1"},
	"gemini":    {"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.5-flash-lite"},
	"openai":    {"gpt-5-mini", "gpt-5", "gpt-4.1-mini", "gpt-4.1"},
	"deepseek":  {"deepseek-chat"},
	"qwen":      {"qwen-plus", "qwen3-max"},
	// Z.ai (kiểm 2026-10-13): context 200K, output tối đa 128K cho glm-4.7/5.1
	// (https://docs.z.ai/guides/llm/glm-4.7, https://docs.z.ai/guides/llm/glm-5.1).
	// Coding Plan chỉ nhận GLM-5.3/GLM-5.3-Flash; gọi glm-5.1 được chuyển sang
	// GLM-5.3, glm-4.7 sang GLM-5.3-Flash (https://docs.z.ai/devpack/overview).
	// Registry chưa có glm-5.3 nên dùng hai mã cũ, mã nào cũng được gói nhận.
	"zai-coding": {"glm-4.7", "glm-5.1"},
	"zai":        {"glm-4.7", "glm-5.1", "glm-4.6", "glm-4.5-air"},
	"glm":        {"glm-4.6", "glm-4.5-air"},
	"grok":       {"grok-4.3"},
}

// recommendedModels là model "Đề xuất" của mỗi provider: rẻ mà viết văn tốt. Gói
// thuê bao vẫn trừ hạn mức theo token và model lớn tốn hạn mức hơn nhiều, nên
// zai-coding cũng chọn model rẻ (glm-4.7 → GLM-5.3-Flash).
var recommendedModels = map[string]string{
	"openrouter": "google/gemini-2.5-flash",
	"anthropic":  "claude-sonnet-4-5",
	"gemini":     "gemini-2.5-flash",
	"openai":     "gpt-5-mini",
	"deepseek":   "deepseek-chat",
	"qwen":       "qwen-plus",
	"zai-coding": "glm-4.7",
	"zai":        "glm-4.7",
	"glm":        "glm-4.6",
	"grok":       "grok-4.3",
}

// displayNames là tên hiển thị riêng theo provider khi model thật chạy phía
// provider khác với mã model gửi đi. Chỉ đổi tên hiển thị; mã model giữ nguyên.
//
// zai-coding: gói chuyển glm-4.7 → GLM-5.3-Flash, glm-5.1 → GLM-5.3
// (https://docs.z.ai/devpack/overview). Provider zai (API thường) giữ tên gốc.
var displayNames = map[string]map[string]string{
	"zai-coding": {
		"glm-4.7": "GLM-5.3-Flash",
		"glm-5.1": "GLM-5.3",
	},
}

// priceOverrides là giá USD / 1M token riêng của một provider khi khác giá
// registry (registry lấy giá bigmodel/OpenRouter). Chỉ ghi đè các field giá;
// Provider/ID vẫn là của mục registry để worker resolve đúng model.
//
// zai: bảng giá Z.ai, kiểm 2026-10-13 (https://docs.z.ai/guides/overview/pricing).
var priceOverrides = map[string]map[string]api.ModelPricing{
	"zai": {
		"glm-4.7":     {InputCostPer1M: 0.6, OutputCostPer1M: 2.2, CacheReadCostPer1M: 0.11},
		"glm-5.1":     {InputCostPer1M: 1.4, OutputCostPer1M: 4.4, CacheReadCostPer1M: 0.26},
		"glm-4.6":     {InputCostPer1M: 0.6, OutputCostPer1M: 2.2, CacheReadCostPer1M: 0.11},
		"glm-4.5-air": {InputCostPer1M: 0.2, OutputCostPer1M: 1.1, CacheReadCostPer1M: 0.03},
	},
}
