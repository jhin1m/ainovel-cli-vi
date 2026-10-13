package keys

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/voocel/ainovel-cli/internal/entry/worker"
	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// Catalog trả lời "provider nào, model nào chọn được, giá bao nhiêu" từ allowlist
// của worker, danh sách chọn lọc và registry giá.
type Catalog struct {
	reg *models.ModelRegistry
}

// NewCatalog dùng registry cho trước (thường là models.DefaultRegistry()).
func NewCatalog(reg *models.ModelRegistry) *Catalog {
	return &Catalog{reg: reg}
}

// RefreshLoop nạp giá (cache trên đĩa 24h) ngay rồi kiểm lại mỗi 12h (nhịp ngắn
// hơn TTL để cache không sống tới gần 48h). Chặn tới khi ctx huỷ; khi trả về thì
// không còn lượt làm mới nào ghi vào cacheDir.
func (c *Catalog) RefreshLoop(ctx context.Context, cacheDir string) {
	models.RefreshPricing(ctx, c.reg, cacheDir)
	t := time.NewTicker(12 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			slog.Info("làm mới bảng giá model")
			models.RefreshPricing(ctx, c.reg, cacheDir)
		}
	}
}

// Providers trả về mọi provider được phép kèm model có giá.
func (c *Catalog) Providers() []api.ProviderInfo {
	out := []api.ProviderInfo{}
	for _, p := range worker.Providers() {
		info := api.ProviderInfo{Name: p.Name, Label: p.Label, Subscription: p.Subscription, Models: []api.ModelOption{}}
		for _, id := range curatedModels[p.Name] {
			pr, ok := c.PricingFor(p.Name, id)
			if !ok {
				continue
			}
			name := pr.Name
			if override, ok := displayNames[p.Name][id]; ok {
				name = override
			}
			if name == "" {
				name = id
			}
			info.Models = append(info.Models, api.ModelOption{
				ID: id, Name: name, ContextWindow: pr.ContextWindow,
				InputCostPer1M: pr.InputCostPer1M, OutputCostPer1M: pr.OutputCostPer1M,
				Recommended: recommendedModels[p.Name] == id,
			})
		}
		out = append(out, info)
	}
	return out
}

// Allowed báo provider/model có chọn được không (nằm trong danh sách và có giá).
func (c *Catalog) Allowed(provider, model string) bool {
	if _, ok := worker.LookupProvider(provider); !ok {
		return false
	}
	if !slices.Contains(curatedModels[provider], model) {
		return false
	}
	_, ok := c.resolve(model)
	return ok
}

// PricingFor trả về giá gửi kèm init cho worker. Cùng phép Resolve mà worker dùng
// để kiểm, nên mục giá luôn khớp đúng model. Provider có giá riêng (priceOverrides)
// thay phần giá, giữ Provider/ID của mục registry.
func (c *Catalog) PricingFor(provider, model string) (api.ModelPricing, bool) {
	e, ok := c.resolve(model)
	if !ok {
		return api.ModelPricing{}, false
	}
	pr := api.ModelPricing{
		Provider: e.Provider, ID: e.ID, Name: e.Name,
		ContextWindow: e.ContextWindow, MaxTokens: e.MaxTokens,
		InputCostPer1M: e.InputCostPer1M, OutputCostPer1M: e.OutputCostPer1M,
		CacheReadCostPer1M: e.CacheReadCostPer1M, CacheWriteCostPer1M: e.CacheWriteCostPer1M,
	}
	if o, ok := priceOverrides[provider][model]; ok {
		pr.InputCostPer1M, pr.OutputCostPer1M = o.InputCostPer1M, o.OutputCostPer1M
		pr.CacheReadCostPer1M, pr.CacheWriteCostPer1M = o.CacheReadCostPer1M, o.CacheWriteCostPer1M
	}
	return pr, true
}

// resolve tìm mục giá và bắt buộc trùng đúng mã model (Resolve của registry có
// so khớp chuỗi con, không được nhận nhầm model khác).
func (c *Catalog) resolve(model string) (models.ModelEntry, bool) {
	e, ok := c.reg.Resolve(model)
	if !ok || !(e.InputCostPer1M > 0) || !(e.OutputCostPer1M > 0) {
		return models.ModelEntry{}, false
	}
	id := model
	if i := strings.LastIndex(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	norm := func(s string) string { return strings.ReplaceAll(strings.ToLower(s), ".", "-") }
	if norm(e.ID) != norm(id) {
		return models.ModelEntry{}, false
	}
	return *e, true
}
