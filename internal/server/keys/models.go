// Package keys quản lý API key LLM của người dùng (BYOK) và danh mục model chọn được.
package keys

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
	"glm":       {"glm-4.6", "glm-4.5-air"},
	"grok":      {"grok-4.3"},
}
