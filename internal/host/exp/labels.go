package exp

import (
	"fmt"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// exportLabels là nhãn hiển thị khi xuất TXT/EPUB, chọn theo ngôn ngữ sáng tác.
type exportLabels struct {
	chapterNum    string // "Chương %d"
	chapterTitled string // "Chương %d: %s" — zh dùng khoảng cách khác nhau cho TXT và EPUB
	chapterTXT    string
	volumeNum     string
	volumeTitled  string
	volumeTXT     string
	xmlLang       string
	cover, toc    string
	nameOpen      string // bao quanh tên sách ở đầu file TXT
	nameClose     string
}

var viExportLabels = exportLabels{
	chapterNum:    "Chương %d",
	chapterTitled: "Chương %d: %s",
	chapterTXT:    "Chương %d: %s",
	volumeNum:     "Quyển %d",
	volumeTitled:  "Quyển %d: %s",
	volumeTXT:     "Quyển %d: %s",
	xmlLang:       "vi",
	cover:         "Bìa",
	toc:           "Mục lục",
}

func labels() exportLabels {
	if lang.Current() == lang.ZH {
		return zhExportLabels
	}
	return viExportLabels
}

// chapterHeading trả về tiêu đề chương; txt=true dùng định dạng của file TXT.
func (l exportLabels) chapterHeading(ch int, title string, txt bool) string {
	if title == "" {
		return fmt.Sprintf(l.chapterNum, ch)
	}
	if txt {
		return fmt.Sprintf(l.chapterTXT, ch, title)
	}
	return fmt.Sprintf(l.chapterTitled, ch, title)
}

// volumeHeading trả về tiêu đề quyển; thiếu tên quyển thì chỉ có số.
func (l exportLabels) volumeHeading(idx int, title string, txt bool) string {
	if title == "" {
		return fmt.Sprintf(l.volumeNum, idx)
	}
	if txt {
		return fmt.Sprintf(l.volumeTXT, idx, title)
	}
	return fmt.Sprintf(l.volumeTitled, idx, title)
}
