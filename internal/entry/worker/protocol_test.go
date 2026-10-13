package worker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeInitValid(t *testing.T) {
	msg, err := decodeInit(mustJSON(t, testInit("/data/b1", api.ActionStart)))
	if err != nil {
		t.Fatal(err)
	}
	if msg.LLM.Provider != "openrouter" || msg.BudgetUSD != 5 {
		t.Fatalf("giải mã sai: %+v", msg)
	}
}

func TestDecodeInitRejects(t *testing.T) {
	base := func() map[string]any {
		var m map[string]any
		_ = json.Unmarshal(mustJSON(t, testInit("/data/b1", api.ActionStart)), &m)
		return m
	}
	cases := map[string]func(m map[string]any){
		"base_url trong llm":       func(m map[string]any) { m["llm"].(map[string]any)["base_url"] = "http://169.254.169.254" },
		"notify ở cấp trên":        func(m map[string]any) { m["notify"] = map[string]any{"command": "rm -rf /"} },
		"roles ở cấp trên":         func(m map[string]any) { m["roles"] = map[string]any{} },
		"providers ở cấp trên":     func(m map[string]any) { m["providers"] = map[string]any{} },
		"protocol lệch":            func(m map[string]any) { m["protocol"] = 2 },
		"action lạ":                func(m map[string]any) { m["action"] = "continue" },
		"start thiếu prompt":       func(m map[string]any) { m["prompt"] = " " },
		"advance_mode lạ":          func(m map[string]any) { m["advance_mode"] = "manual" },
		"provider ngoài allowlist": func(m map[string]any) { m["llm"].(map[string]any)["provider"] = "custom" },
		"thiếu api_key":            func(m map[string]any) { m["llm"].(map[string]any)["api_key"] = "" },
		"ngân sách 0":              func(m map[string]any) { m["budget_usd"] = 0 },
		"thiếu pricing":            func(m map[string]any) { delete(m, "pricing") },
		"pricing giá 0":            func(m map[string]any) { m["pricing"].(map[string]any)["output_cost_per_1m"] = 0 },
		"style có đường dẫn":       func(m map[string]any) { m["style"] = "../../etc" },
		"reasoning lạ":             func(m map[string]any) { m["llm"].(map[string]any)["reasoning_effort"] = "ultra" },
		"type khác init":           func(m map[string]any) { m["type"] = "steer" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			m := base()
			mut(m)
			_, err := decodeInit(mustJSON(t, m))
			if err == nil {
				t.Fatal("phải bị từ chối")
			}
			if strings.Contains(err.Error(), testAPIKey) {
				t.Fatalf("lỗi chứa API key: %v", err)
			}
		})
	}
}

func TestDecodeInitMalformed(t *testing.T) {
	for _, line := range []string{`{`, `[]`, `{"protocol":1}`, `{"type":"init"} {"x":1}`, `{"type":"init","api_key":"` + testAPIKey + `"}`} {
		_, err := decodeInit([]byte(line))
		if err == nil {
			t.Fatalf("%q phải lỗi", line)
		}
		if strings.Contains(err.Error(), testAPIKey) {
			t.Fatalf("lỗi chứa API key: %v", err)
		}
	}
}

func TestDecodeCommand(t *testing.T) {
	ok := []string{
		`{"type":"steer","id":"1","text":"thêm"}`,
		`{"type":"set_advance_mode","id":"2","mode":"auto"}`,
		`{"type":"abort","id":"3"}`,
		`{"type":"shutdown","id":"4"}`,
	}
	for _, line := range ok {
		if _, err := decodeCommand([]byte(line)); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}
	bad := []string{
		`{"type":"init","id":"1"}`,
		`{"type":"steer","id":"1"}`,
		`{"type":"steer","text":"x"}`,
		`{"type":"set_advance_mode","id":"1","mode":"x"}`,
		`{"type":"abort","id":"1","extra":true}`,
		`not json`,
	}
	for _, line := range bad {
		if _, err := decodeCommand([]byte(line)); err == nil {
			t.Fatalf("%s phải lỗi", line)
		}
	}
}

// subscriptionInit là init của truyện Z.ai Coding Plan: budget 0, mục giá lấy
// đúng từ registry thật như server gửi (entry glm, giá > 0).
func subscriptionInit(t *testing.T) api.WorkerInit {
	t.Helper()
	e, ok := models.NewModelRegistry().Resolve("glm-4.6")
	if !ok {
		t.Fatal("registry thiếu glm-4.6")
	}
	msg := testInit("/data/b1", api.ActionStart)
	msg.LLM.Provider, msg.LLM.Model = "zai-coding", "glm-4.6"
	msg.BudgetUSD = 0
	msg.Pricing = &api.ModelPricing{
		Provider: e.Provider, ID: e.ID, Name: e.Name, ContextWindow: e.ContextWindow, MaxTokens: e.MaxTokens,
		InputCostPer1M: e.InputCostPer1M, OutputCostPer1M: e.OutputCostPer1M,
	}
	return msg
}

func TestDecodeInitSubscription(t *testing.T) {
	msg, err := decodeInit(mustJSON(t, subscriptionInit(t)))
	if err != nil {
		t.Fatalf("init thuê bao budget 0 phải hợp lệ: %v", err)
	}
	// Worker dùng registry thật: mục giá phải resolve đúng theo tên model.
	if err := applyPricing(models.NewModelRegistry(), msg); err != nil {
		t.Fatalf("applyPricing thuê bao: %v", err)
	}

	withBudget := subscriptionInit(t)
	withBudget.BudgetUSD = 5
	if _, err := decodeInit(mustJSON(t, withBudget)); err == nil {
		t.Fatal("thuê bao gửi budget > 0 phải bị từ chối")
	}

	// Provider trả theo token (zai) vẫn đòi budget > 0 như cũ.
	payg := subscriptionInit(t)
	payg.LLM.Provider = "zai"
	if _, err := decodeInit(mustJSON(t, payg)); err == nil {
		t.Fatal("provider thường budget 0 phải bị từ chối")
	}
	// Thuê bao vẫn cần mục giá > 0 để init.
	noPrice := subscriptionInit(t)
	noPrice.Pricing.OutputCostPer1M = 0
	if _, err := decodeInit(mustJSON(t, noPrice)); err == nil {
		t.Fatal("thuê bao thiếu giá phải bị từ chối")
	}
}
