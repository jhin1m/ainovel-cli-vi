package worker

import "github.com/voocel/ainovel-cli/internal/bootstrap"

// Provider là một nhà cung cấp LLM được phép dùng qua bản web.
type Provider struct {
	Name    string
	Label   string
	Type    string // giao thức litellm; rỗng = dùng chính Name
	BaseURL string // hằng số biên dịch sẵn, không bao giờ lấy từ input
	// Subscription: gói thuê bao trả theo lượt dùng, không tính tiền theo token.
	// Truyện của provider này không có ngân sách USD và báo chi phí bằng 0.
	Subscription bool
}

// allowedProviders là allowlist biên dịch sẵn, theo thứ tự hiển thị. Không có
// ollama/bedrock/custom: các provider đó cần base_url/type tuỳ ý, mở đường cho
// SSRF từ phía người dùng.
var allowedProviders = []string{"openrouter", "anthropic", "gemini", "openai", "deepseek", "qwen", "zai-coding", "zai", "glm", "grok"}

// webProviders là provider chỉ có ở bản web (không nằm trong preset CLI). Z.ai
// dùng chung giao thức với GLM nên đi qua litellm "glm" với BaseURL cố định.
var webProviders = map[string]Provider{
	"zai-coding": {Name: "zai-coding", Label: "Z.ai Coding Plan", Type: "glm", BaseURL: "https://api.z.ai/api/coding/paas/v4", Subscription: true},
	"zai":        {Name: "zai", Label: "Z.ai", Type: "glm", BaseURL: "https://api.z.ai/api/paas/v4"},
}

// webLabels đổi nhãn preset cho bản web, nơi hai endpoint GLM đứng cạnh nhau.
var webLabels = map[string]string{"glm": "GLM (bigmodel.cn)"}

// Providers trả về danh sách provider được phép, theo đúng thứ tự hiển thị.
func Providers() []Provider {
	presets := make(map[string]bootstrap.ProviderPreset)
	for _, p := range bootstrap.ProviderPresets() {
		presets[p.Name] = p
	}
	out := make([]Provider, 0, len(allowedProviders))
	for _, name := range allowedProviders {
		if p, ok := webProviders[name]; ok {
			out = append(out, p)
			continue
		}
		p, ok := presets[name]
		if !ok {
			continue
		}
		label := p.Label
		if l, ok := webLabels[name]; ok {
			label = l
		}
		out = append(out, Provider{Name: p.Name, Label: label, BaseURL: p.BaseURL})
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

// IsSubscription báo provider có phải gói thuê bao (không ngân sách USD) không.
func IsSubscription(name string) bool {
	p, ok := LookupProvider(name)
	return ok && p.Subscription
}
