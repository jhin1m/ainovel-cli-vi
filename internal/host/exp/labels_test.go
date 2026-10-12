package exp

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/lang"
)

func TestExportLabelsFollowLanguage(t *testing.T) {
	t.Cleanup(func() { lang.Set(lang.VI) })

	render := func() (string, string, string) {
		txt := renderTXT("Sách", []int{1}, chapterTitleIndex{1: "Mở đầu"}, nil, map[int]string{1: "Nội dung."})
		nav := renderNavXHTML(true, []int{1}, chapterTitleIndex{1: "Mở đầu"})
		opf := renderOPF(domain.BookMetadata{Title: "Sách"}, true, []int{1})
		return txt, nav, opf
	}

	lang.Set(lang.VI)
	txt, nav, opf := render()
	if !strings.Contains(txt, "Chương 1: Mở đầu") || strings.Contains(txt, "《") {
		t.Errorf("TXT vi sai nhãn:\n%s", txt)
	}
	if !strings.Contains(nav, "Mục lục") || !strings.Contains(nav, ">Bìa<") || !strings.Contains(opf, "<dc:language>vi</dc:language>") {
		t.Errorf("EPUB vi sai nhãn:\n%s\n%s", nav, opf)
	}

	lang.Set(lang.ZH)
	txt, nav, opf = render()
	if !strings.Contains(txt, labels().chapterHeading(1, "Mở đầu", true)) || !strings.HasPrefix(txt, zhExportLabels.nameOpen) {
		t.Errorf("TXT zh phải dùng nhãn gốc:\n%s", txt)
	}
	if !strings.Contains(nav, zhExportLabels.toc) || !strings.Contains(opf, "<dc:language>zh-CN</dc:language>") || strings.Contains(nav, "Mục lục") {
		t.Errorf("EPUB zh phải dùng nhãn gốc:\n%s\n%s", nav, opf)
	}
}
