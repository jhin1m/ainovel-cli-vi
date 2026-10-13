package worker

import "github.com/voocel/ainovel-cli/internal/bootstrap"

// Provider là một nhà cung cấp LLM được phép dùng qua bản web.
type Provider struct {
	Name    string
	Label   string
	BaseURL string // hằng số biên dịch sẵn từ preset, không bao giờ lấy từ input
}

// allowedProviders là allowlist biên dịch sẵn. Không có ollama/bedrock/custom:
// các provider đó cần base_url/type tuỳ ý, mở đường cho SSRF từ phía người dùng.
var allowedProviders = []string{"openrouter", "anthropic", "gemini", "openai", "deepseek", "qwen", "glm", "grok"}

// Providers trả về danh sách provider được phép, theo đúng thứ tự hiển thị.
func Providers() []Provider {
	presets := make(map[string]bootstrap.ProviderPreset)
	for _, p := range bootstrap.ProviderPresets() {
		presets[p.Name] = p
	}
	out := make([]Provider, 0, len(allowedProviders))
	for _, name := range allowedProviders {
		p, ok := presets[name]
		if !ok {
			continue
		}
		out = append(out, Provider{Name: p.Name, Label: p.Label, BaseURL: p.BaseURL})
	}
	return out
}

// LookupProvider trả về provider trong allowlist theo tên.
func LookupProvider(name string) (Provider, bool) {
	for _, p := range Providers() {
		if p.Name == name {
			return p, true
		}
	}
	return Provider{}, false
}
