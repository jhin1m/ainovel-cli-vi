package rules

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// useLang 临时切换进程语言，测试结束恢复默认 vi。
func useLang(t *testing.T, c lang.Code) {
	t.Helper()
	lang.Set(c)
	t.Cleanup(func() { lang.Set(lang.VI) })
}

func TestLint_CleanText(t *testing.T) {
	useLang(t, lang.ZH)
	if vs := Lint("# 第一章 风起\n他迈步向前。\n夜色渐深。"); len(vs) != 0 {
		t.Errorf("clean text should pass: %+v", vs)
	}
}

func TestLint_MarkdownResidue(t *testing.T) {
	useLang(t, lang.ZH)
	text := "# 第一章\n这是**重点**内容。\n## 小标题\n正文。"
	vs := Lint(text)
	bold := findViolation(vs, "markdown_residue", "**")
	if bold == nil || bold.Actual != 2 {
		t.Errorf("expected ** residue x2: %+v", vs)
	}
	heading := findViolation(vs, "markdown_residue", "#")
	if heading == nil || heading.Actual != 1 {
		t.Errorf("expected 1 heading beyond first line: %+v", vs)
	}
}

func TestLint_NonCJKFragments(t *testing.T) {
	useLang(t, lang.ZH)
	text := "# 第一章\n他发现了一个pattern，这个pattern像DNA一样规律。"
	vs := Lint(text)
	var v *Violation
	for i := range vs {
		if vs[i].Rule == "non_cjk_fragments" {
			v = &vs[i]
			break
		}
	}
	if v == nil {
		t.Fatalf("expected non_cjk violation: %+v", vs)
	}
	if v.Actual != 3 {
		t.Errorf("total count: got %v want 3", v.Actual)
	}
	if !strings.Contains(v.Target, "pattern") || !strings.Contains(v.Target, "DNA") {
		t.Errorf("examples should be distinct: %q", v.Target)
	}
	if v.Severity != SeverityWarning {
		t.Errorf("severity: %v", v.Severity)
	}
}

// non_cjk_fragments 只适用于中文正文：越南语正文每个词都是拉丁字母，不得命中。
func TestLint_NonCJKFragmentsGatedToZH(t *testing.T) {
	text := "# Chương 1\nHắn phát hiện một quy luật, giống như DNA vậy."
	for _, v := range Lint(text) {
		if v.Rule == "non_cjk_fragments" {
			t.Fatalf("vi 下不应出现 non_cjk_fragments: %+v", v)
		}
	}
	useLang(t, lang.ZH)
	if findRule(Lint(text), "non_cjk_fragments") == nil {
		t.Fatal("zh 下应保留 non_cjk_fragments")
	}
}

func TestLint_MarkdownResidueVietnamese(t *testing.T) {
	text := "# Chương 1\nĐây là **trọng điểm**.\n## Tiêu đề phụ\nChính văn."
	vs := Lint(text)
	if v := findViolation(vs, "markdown_residue", "**"); v == nil || v.Actual != 2 {
		t.Errorf("expected ** residue x2: %+v", vs)
	}
	if v := findViolation(vs, "markdown_residue", "#"); v == nil || v.Actual != 1 {
		t.Errorf("expected 1 heading: %+v", vs)
	}
}

func findRule(vs []Violation, rule string) *Violation {
	for i := range vs {
		if vs[i].Rule == rule {
			return &vs[i]
		}
	}
	return nil
}
