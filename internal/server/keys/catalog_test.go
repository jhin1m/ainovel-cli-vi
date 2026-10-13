package keys

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/entry/worker"
	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
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
			pr, ok := c.PricingFor(p.Name, m.ID)
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
	if _, ok := c.PricingFor("openai", "gpt"); ok {
		t.Fatal("PricingFor không được khớp mờ theo chuỗi con")
	}
}

func TestCatalogZaiAndRecommended(t *testing.T) {
	c := NewCatalog(models.NewModelRegistry())
	byName := map[string]api.ProviderInfo{}
	for _, p := range c.Providers() {
		byName[p.Name] = p
		rec := 0
		for _, m := range p.Models {
			if m.Recommended {
				rec++
			}
		}
		if rec > 1 {
			t.Errorf("%s có %d model đề xuất, tối đa 1", p.Name, rec)
		}
		if id, ok := recommendedModels[p.Name]; ok && rec == 0 {
			t.Errorf("%s: model đề xuất %s không hiện trong danh sách", p.Name, id)
		}
	}
	coding, ok := byName["zai-coding"]
	if !ok || !coding.Subscription || coding.Label != "Z.ai Coding Plan" || len(coding.Models) == 0 {
		t.Fatalf("zai-coding sai: %+v", coding)
	}
	if z := byName["zai"]; z.Subscription || len(z.Models) == 0 {
		t.Fatalf("zai sai: %+v", z)
	}
	if g := byName["glm"]; g.Label != "GLM (bigmodel.cn)" || g.Subscription {
		t.Fatalf("glm sai: %+v", g)
	}
	for name, p := range byName {
		if name != "zai-coding" && p.Subscription {
			t.Errorf("%s không phải thuê bao", name)
		}
	}
}

func TestPricingForZaiOverride(t *testing.T) {
	c := NewCatalog(models.NewModelRegistry())
	glm, ok := c.PricingFor("glm", "glm-4.6")
	if !ok {
		t.Fatal("glm-4.6 phải có giá")
	}
	zai, ok := c.PricingFor("zai", "glm-4.6")
	if !ok || zai.InputCostPer1M != 0.6 || zai.OutputCostPer1M != 2.2 || zai.CacheReadCostPer1M != 0.11 {
		t.Fatalf("giá zai phải theo bảng Z.ai: %+v", zai)
	}
	// Chỉ thay giá: Provider/ID vẫn là mục registry để worker resolve khớp.
	if zai.Provider != glm.Provider || zai.ID != glm.ID || zai.ContextWindow != glm.ContextWindow {
		t.Fatalf("override không được đổi mục registry: zai=%+v glm=%+v", zai, glm)
	}
	// Mọi model trong override phải nằm trong danh sách chọn lọc của provider đó.
	for prov, ms := range priceOverrides {
		for id := range ms {
			if !c.Allowed(prov, id) {
				t.Errorf("override %s/%s không nằm trong danh sách chọn được", prov, id)
			}
		}
	}
}

// zai-coding hiện tên model thật mà gói chạy; mã model gửi đi và provider zai giữ nguyên.
func TestCatalogZaiCodingDisplayNames(t *testing.T) {
	c := NewCatalog(models.NewModelRegistry())
	names := map[string]map[string]string{}
	for _, p := range c.Providers() {
		names[p.Name] = map[string]string{}
		for _, m := range p.Models {
			names[p.Name][m.ID] = m.Name
		}
	}
	want := map[string]string{"glm-4.7": "GLM-5.3-Flash", "glm-5.1": "GLM-5.3"}
	for id, name := range want {
		if got := names["zai-coding"][id]; got != name {
			t.Errorf("zai-coding %s: tên = %q, muốn %q", id, got, name)
		}
		if got := names["zai"][id]; got == "" || got == name {
			t.Errorf("zai %s phải giữ tên gốc, got %q", id, got)
		}
	}
	pr, ok := c.PricingFor("zai-coding", "glm-4.7")
	if !ok || pr.ID != "glm-4.7" {
		t.Fatalf("mã model gửi đi phải giữ nguyên: %+v", pr)
	}
}
