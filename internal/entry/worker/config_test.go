package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

func TestBuildConfigIsLockedDown(t *testing.T) {
	// HOME có config độc hại: buildConfig không được đọc tới nó.
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	evil := `{"provider":"custom","providers":{"custom":{"type":"openai","base_url":"http://169.254.169.254"}},"notify":{"command":"touch /tmp/pwned"}}`
	if err := os.WriteFile(filepath.Join(home, ".ainovel", "config.json"), []byte(evil), 0o600); err != nil {
		t.Fatal(err)
	}

	msg := testInit("/data/b1", api.ActionStart)
	cfg, err := buildConfig(msg, "/data/b1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OutputDir != "/data/b1" || cfg.Provider != "openrouter" || cfg.ModelName != "test/fake-model" {
		t.Fatalf("config sai: %+v", cfg)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("chỉ được có đúng một provider: %v", cfg.Providers)
	}
	pc := cfg.Providers["openrouter"]
	if pc.APIKey != testAPIKey || pc.BaseURL != "https://openrouter.ai/api/v1" || pc.Type != "" || pc.Extra != nil || pc.ExtraBody != nil {
		t.Fatalf("provider config không an toàn: %+v", pc)
	}
	if len(cfg.Roles) != 0 {
		t.Fatalf("roles phải rỗng: %v", cfg.Roles)
	}
	if cfg.Notify.IsEnabled() || cfg.Notify.Command != "" {
		t.Fatalf("notify phải tắt: %+v", cfg.Notify)
	}
	if !cfg.DisableUpdateCheck || !cfg.Budget.HardStop || cfg.Budget.BookUSD != 5 {
		t.Fatalf("budget/update check sai: %+v", cfg)
	}
}

func TestBuildConfigProviderBaseURLFromPreset(t *testing.T) {
	msg := testInit("/b", api.ActionStart)
	msg.LLM.Provider = "anthropic"
	cfg, err := buildConfig(msg, "/b")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers["anthropic"].BaseURL != "" {
		t.Fatalf("anthropic không có base_url preset: %+v", cfg.Providers["anthropic"])
	}
}

func TestApplyPricing(t *testing.T) {
	reg := models.NewModelRegistry()
	msg := testInit("/b", api.ActionStart)
	if err := applyPricing(reg, msg); err != nil {
		t.Fatal(err)
	}
	entry, ok := reg.Resolve(msg.LLM.Model)
	if !ok || entry.InputCostPer1M != 1 || entry.OutputCostPer1M != 2 {
		t.Fatalf("giá chưa được nạp: %+v", entry)
	}

	msg.LLM.Model = "khong-ton-tai/xyz-unpriced"
	msg.Pricing = &api.ModelPricing{Provider: "x", ID: "khac", InputCostPer1M: 1, OutputCostPer1M: 1}
	if err := applyPricing(models.NewModelRegistry(), msg); err == nil {
		t.Fatal("model không có giá phải bị từ chối")
	}
}

func TestProvidersAllowlist(t *testing.T) {
	got := Providers()
	if len(got) != len(allowedProviders) {
		t.Fatalf("thiếu provider trong preset: %+v", got)
	}
	for _, name := range []string{"ollama", "bedrock", "custom"} {
		if _, ok := LookupProvider(name); ok {
			t.Fatalf("%s không được nằm trong allowlist", name)
		}
	}
}

func TestApplyPricingRejectsMismatchedEntry(t *testing.T) {
	reg := models.NewModelRegistry()
	msg := testInit("/b", api.ActionStart)
	msg.LLM.Model = "anthropic/claude-sonnet-4"
	// Giá gửi kèm là của model khác: không được chấp nhận dù registry có giá sẵn cho model đã chọn.
	msg.Pricing = &api.ModelPricing{Provider: "openai", ID: "gpt-x-khac", InputCostPer1M: 1, OutputCostPer1M: 1}
	if err := applyPricing(reg, msg); err == nil {
		t.Fatal("pricing lệch model phải bị từ chối")
	}
}
