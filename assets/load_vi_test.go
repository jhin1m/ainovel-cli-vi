package assets

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// TestVILanguageComplete: bộ tài nguyên vi phải phủ đủ từng file của bản gốc.
func TestVILanguageComplete(t *testing.T) {
	// Tự đối chiếu từng file gốc với bản vi trong đúng FS nhúng chứa bản dịch.
	for _, p := range baseAssetPaths() {
		lp := localPath(lang.VI, p)
		if _, err := fs.Stat(fsFor(lp), lp); err != nil {
			t.Errorf("thiếu bản dịch assets/%s", lp)
		}
	}
	// CheckLanguage hiện tra voice.md (vi) trong voiceFS thay vì promptsFS nên luôn báo
	// thiếu prompts/vi/voice.md dù file có thật; chấp nhận đúng lỗi đó, mọi file khác phải đủ.
	// Khi loader sửa (fsFor(lp)), nhánh này tự hết và CheckLanguage phải trả nil.
	if err := CheckLanguage(lang.VI); err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "thiếu 1 tài nguyên") || !strings.Contains(msg, "assets/prompts/vi/voice.md") {
			t.Fatal(err)
		}
		t.Logf("CheckLanguage báo nhầm voice.md: %v", err)
	}
}

// placeholderRe khớp placeholder {{...}}; identRe khớp định danh snake_case (tên tool, tên trường, giá trị enum).
var (
	placeholderRe = regexp.MustCompile(`\{\{[^}]+\}\}`)
	identRe       = regexp.MustCompile(`[a-z][a-z0-9]*(?:_[a-z0-9]+)+`)
)

// protectedTokens trích các ký hiệu bản dịch bắt buộc giữ nguyên.
func protectedTokens(s string) []string {
	set := map[string]bool{}
	for _, m := range placeholderRe.FindAllString(s, -1) {
		set[m] = true
	}
	for _, m := range identRe.FindAllString(s, -1) {
		set[m] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestVIPromptsKeepProtectedTokens so từng prompt: placeholder và định danh snake_case
// có trong bản Trung phải có đủ trong bản Việt (LLM dựa vào chúng để gọi tool, điền trường).
func TestVIPromptsKeepProtectedTokens(t *testing.T) {
	paths := []string{"voice.md"}
	entries, err := promptsFS.ReadDir("prompts")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			paths = append(paths, "prompts/"+e.Name())
		}
	}
	for _, p := range paths {
		zh := mustRead(fsFor(p), p)
		lp := localPath(lang.VI, p)
		vi := mustRead(fsFor(lp), lp)
		for _, tok := range protectedTokens(zh) {
			if !strings.Contains(vi, tok) {
				t.Errorf("%s: bản vi thiếu %q", lp, tok)
			}
		}
	}
}

func TestVIInterventionPromptsKeepScopeContract(t *testing.T) {
	prompts := loadPromptsFor(lang.VI)
	for _, phrase := range []string{"ngữ cảnh không đồng nghĩa với quyền sửa đổi", "phạm vi tối thiểu đủ dùng", "phạm vi phân tích không đồng nghĩa với phạm vi sửa đổi"} {
		if !strings.Contains(prompts.ArbiterIntervention, phrase) {
			t.Fatalf("Arbiter can thiệp (vi) thiếu hợp đồng phạm vi %q", phrase)
		}
	}
	for _, phrase := range []string{"Can thiệp gốc của người dùng", "phạm vi phân tích không đồng nghĩa với phạm vi sửa đổi", "tập chương tối thiểu đủ dùng"} {
		if !strings.Contains(prompts.Editor, phrase) {
			t.Fatalf("Editor (vi) thiếu hợp đồng phạm vi %q", phrase)
		}
	}
}

func TestVIStructuredArbiterPromptsContainOnlySemantics(t *testing.T) {
	prompts := loadPromptsFor(lang.VI)
	for name, prompt := range map[string]string{
		"plan_start": prompts.ArbiterPlanStart,
		"failure":    prompts.ArbiterFailure,
	} {
		for _, duplicate := range []string{"```json", "Markdown", "xuất một đối tượng JSON", "trả về một đối tượng JSON"} {
			if strings.Contains(prompt, duplicate) {
				t.Fatalf("%s (vi) vẫn tự quy định định dạng đầu ra %q", name, duplicate)
			}
		}
	}
}

// TestVIWriterPromptAssembly: writer vi cũng đi qua BuildWriterPrompt, placeholder được thay
// và phần giao thức không bị văn phong ghi đè làm hỏng.
func TestVIWriterPromptAssembly(t *testing.T) {
	b := Load("default", LoadOptions{Language: lang.VI})
	if strings.Count(mustRead(promptsFS, "prompts/vi/writer.md"), voicePlaceholder) != 1 {
		t.Fatal("writer vi phải chứa đúng một placeholder {{VOICE}}")
	}
	got := BuildWriterPrompt(b.Prompts.Writer, b.Voice, b.Styles["default"])
	if strings.Contains(got, voicePlaceholder) {
		t.Fatal("placeholder phải được thay thế")
	}
	if !strings.Contains(got, "## Giao thức thực thi") || !strings.Contains(got, strings.TrimSpace(b.Voice)) {
		t.Fatal("writer vi lắp ráp thiếu giao thức hoặc văn phong")
	}
}

// TestVIAssetsHaveNoHan: tài nguyên vi không được sót chữ Hán (dễ kéo model viết ra tiếng Trung).
func TestVIAssetsHaveNoHan(t *testing.T) {
	for _, root := range []struct {
		fsys fs.FS
		dir  string
	}{{promptsFS, "prompts/vi"}, {referencesFS, "references/vi"}, {stylesFS, "styles/vi"}} {
		err := fs.WalkDir(root.fsys, root.dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(root.fsys, p)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				if strings.IndexFunc(line, func(r rune) bool { return unicode.Is(unicode.Han, r) }) >= 0 {
					t.Errorf("%s:%d còn chữ Hán: %s", p, i+1, line)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
