package worker

import (
	"fmt"
	"strings"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// buildConfig dựng bootstrap.Config từng field từ init. Không đọc file config nào
// trên đĩa; mọi field có thể dẫn tới mạng tuỳ ý (base_url, type, extra) hay thực
// thi lệnh (notify.command) đều cố định ở giá trị an toàn.
func buildConfig(msg api.WorkerInit, bookDir string) (bootstrap.Config, error) {
	provider, ok := LookupProvider(msg.LLM.Provider)
	if !ok {
		return bootstrap.Config{}, fmt.Errorf("provider %q không được phép", msg.LLM.Provider)
	}
	notifyOff := false
	cfg := bootstrap.Config{
		OutputDir:       bookDir,
		Provider:        provider.Name,
		ModelName:       msg.LLM.Model,
		ReasoningEffort: msg.LLM.ReasoningEffort,
		Providers: map[string]bootstrap.ProviderConfig{
			provider.Name: {APIKey: msg.LLM.APIKey, BaseURL: provider.BaseURL},
		},
		Roles:              nil,
		Style:              msg.Style,
		Language:           msg.Language,
		Budget:             bootstrap.BudgetConfig{BookUSD: msg.BudgetUSD, HardStop: true},
		Notify:             bootstrap.NotifyConfig{Enabled: &notifyOff},
		DisableUpdateCheck: true,
	}
	cfg.FillDefaults()
	if err := cfg.ValidateBase(); err != nil {
		return bootstrap.Config{}, err
	}
	return cfg, nil
}

// applyPricing nạp giá model từ server vào registry của process rồi kiểm model
// đã chọn có giá > 0. Không có giá thì chi phí luôn $0 và ngân sách vô hiệu.
func applyPricing(reg *models.ModelRegistry, msg api.WorkerInit) error {
	if p := msg.Pricing; p != nil {
		reg.MergeModels([]models.ModelEntry{{
			Provider:            p.Provider,
			ID:                  p.ID,
			Name:                p.Name,
			ContextWindow:       p.ContextWindow,
			MaxTokens:           p.MaxTokens,
			InputCostPer1M:      p.InputCostPer1M,
			OutputCostPer1M:     p.OutputCostPer1M,
			CacheReadCostPer1M:  p.CacheReadCostPer1M,
			CacheWriteCostPer1M: p.CacheWriteCostPer1M,
		}})
	}
	entry, ok := reg.Resolve(msg.LLM.Model)
	if !ok || !(entry.InputCostPer1M > 0) || !(entry.OutputCostPer1M > 0) {
		return fmt.Errorf("model %q chưa có giá trong registry", msg.LLM.Model)
	}
	// Resolve có so khớp mờ: bắt buộc trúng đúng mục giá server gửi, tránh tính
	// chi phí theo một model khác tình cờ chứa chuỗi tên.
	if p := msg.Pricing; p != nil && (!strings.EqualFold(entry.Provider, p.Provider) || !strings.EqualFold(entry.ID, p.ID)) {
		return fmt.Errorf("pricing không khớp model %q", msg.LLM.Model)
	}
	return nil
}
