package keys

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/entry/worker"
	"github.com/voocel/ainovel-cli/internal/models"
)

func TestCatalogEveryProviderHasPricedModels(t *testing.T) {
	c := NewCatalog(models.NewModelRegistry())
	got := c.Providers()
	if len(got) != len(worker.Providers()) {
		t.Fatalf("thiếu provider: %d", len(got))
	}
	for _, p := range got {
		if len(p.Models) == 0 {
			t.Errorf("provider %s không có model nào có giá trong registry", p.Name)
		}
		for _, m := range p.Models {
			if !(m.InputCostPer1M > 0) || !(m.OutputCostPer1M > 0) {
				t.Errorf("%s/%s không có giá", p.Name, m.ID)
			}
			if !c.Allowed(p.Name, m.ID) {
				t.Errorf("%s/%s hiển thị nhưng không Allowed", p.Name, m.ID)
			}
			pr, ok := c.PricingFor(m.ID)
			if !ok || pr.InputCostPer1M != m.InputCostPer1M {
				t.Errorf("PricingFor(%s) lệch: %+v", m.ID, pr)
			}
		}
	}
}

func TestCatalogHidesUnpricedAndUnknown(t *testing.T) {
	reg := models.NewModelRegistry()
	// Registry rỗng (zero value): model không có mục giá phải bị ẩn.
	empty := NewCatalog(&models.ModelRegistry{})
	for _, p := range empty.Providers() {
		if len(p.Models) != 0 {
			t.Fatalf("registry rỗng mà %s vẫn có model: %+v", p.Name, p.Models)
		}
	}
	c := NewCatalog(reg)
	if c.Allowed("openrouter", "khong/co-model-nay") || c.Allowed("ollama", "llama3") || c.Allowed("anthropic", "gpt-5") {
		t.Fatal("model ngoài danh sách hoặc provider lạ phải bị từ chối")
	}
	if _, ok := c.PricingFor("gpt"); ok {
		t.Fatal("PricingFor không được khớp mờ theo chuỗi con")
	}
}
