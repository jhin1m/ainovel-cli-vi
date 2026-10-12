package ctxpack

import (
	"strings"
	"testing"

	"github.com/voocel/agentcore"
	corecontext "github.com/voocel/agentcore/context"

	"github.com/voocel/ainovel-cli/internal/lang"
)

func setLang(t *testing.T, c lang.Code) {
	t.Helper()
	prev := lang.Current()
	lang.Set(c)
	t.Cleanup(func() { lang.Set(prev) })
}

func TestEstimateTextTokens_VIUsesBytesOverThree(t *testing.T) {
	setLang(t, lang.VI)
	text := strings.Repeat("Hắn bước vào đại điện, ánh mắt lạnh lẽo quét qua đám người. ", 20)
	got := estimateTextTokens(text)
	want := (len(text) + 2) / 3
	if got != want {
		t.Fatalf("vi estimate = %d, want byte/3 = %d", got, want)
	}
	if base := corecontext.EstimateTokens(agentcore.UserMsg(text)); got <= base {
		t.Fatalf("vi estimate %d should exceed agentcore byte/4 estimate %d", got, base)
	}
	if bytesPerToken() != 3 {
		t.Fatalf("vi bytesPerToken = %d, want 3", bytesPerToken())
	}
}

func TestEstimateTextTokens_ZHKeepsAgentcore(t *testing.T) {
	setLang(t, lang.ZH)
	text := strings.Repeat("他走进大殿，目光冷冷扫过众人。", 20) //check-han:ok dữ liệu test zh
	if got, base := estimateTextTokens(text), corecontext.EstimateTokens(agentcore.UserMsg(text)); got != base {
		t.Fatalf("zh estimate = %d, want agentcore %d", got, base)
	}
	if bytesPerToken() != 4 {
		t.Fatalf("zh bytesPerToken = %d, want 4", bytesPerToken())
	}
}
