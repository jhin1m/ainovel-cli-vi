package worker

import (
	"encoding/json"
	"strings"
	"testing"

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
