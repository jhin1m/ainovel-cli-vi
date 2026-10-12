package assets

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

func TestLocalPath(t *testing.T) {
	cases := []struct {
		code lang.Code
		in   string
		want string
	}{
		{lang.ZH, "prompts/writer.md", "prompts/writer.md"},
		{lang.VI, "prompts/writer.md", "prompts/vi/writer.md"},
		{lang.VI, "references/genres/fantasy/arc-templates.md", "references/vi/genres/fantasy/arc-templates.md"},
		{lang.VI, "styles/default.md", "styles/vi/default.md"},
		{lang.VI, "voice.md", "prompts/vi/voice.md"},
	}
	for _, c := range cases {
		if got := localPath(c.code, c.in); got != c.want {
			t.Errorf("localPath(%s, %s) = %s; muốn %s", c.code, c.in, got, c.want)
		}
	}
}

func TestBaseAssetPathsSkipsTranslations(t *testing.T) {
	paths := baseAssetPaths()
	if len(paths) < 20 {
		t.Fatalf("quá ít tài nguyên gốc: %d", len(paths))
	}
	for _, p := range paths {
		if strings.Contains(p, "/vi/") {
			t.Fatalf("danh sách gốc lẫn bản dịch: %s", p)
		}
	}
}

func TestCheckLanguage(t *testing.T) {
	if err := CheckLanguage(lang.ZH); err != nil {
		t.Fatalf("zh không được báo thiếu: %v", err)
	}
	err := CheckLanguage(lang.VI)
	if err != nil {
		// Chưa dịch đủ: lỗi phải chỉ rõ file thiếu.
		if !strings.Contains(err.Error(), "assets/") || !strings.Contains(err.Error(), "/vi/") {
			t.Fatalf("lỗi thiếu tài nguyên không rõ ràng: %v", err)
		}
		return
	}
	b := Load("fantasy", LoadOptions{Language: lang.VI})
	if b.Language != lang.VI || b.Voice == "" || b.Prompts.Writer == "" || b.References.ChapterGuide == "" {
		t.Fatal("bộ tài nguyên vi nạp thiếu")
	}
	if b.Voice == mustRead(voiceFS, "voice.md") {
		t.Fatal("language=vi vẫn nạp voice.md tiếng Trung")
	}
	if !strings.Contains(b.Prompts.Writer, "Hồ sơ phỏng tác") {
		t.Fatal("writer vi phải dùng chỉ dẫn phỏng tác tiếng Việt")
	}
	if _, ok := b.Styles["default"]; !ok {
		t.Fatal("bộ phong cách vi phải có default")
	}
}

func TestLoadDefaultLanguageIsZH(t *testing.T) {
	b := Load("default", LoadOptions{})
	if b.Language != lang.ZH {
		t.Fatalf("LoadOptions rỗng phải giữ zh, có %s", b.Language)
	}
}
