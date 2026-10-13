package books

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

// readerFixture dựng thư mục truyện bằng API thật của store: chương 1 và 2 đã chốt,
// chương 3 chỉ có trong dàn ý.
func readerFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(3); err != nil {
		t.Fatal(err)
	}
	if err := s.Book.Save(domain.BookMetadata{Title: "Truyện: Thử/Nghiệm?", Synopsis: "Tóm tắt."}); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Mở đầu", CoreEvent: "Gặp gỡ"},
		{Chapter: 2, Title: "Dàn ý hai", CoreEvent: "Xung đột"},
		{Chapter: 3, Title: "Kết", CoreEvent: "Hoà giải"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Characters.Save([]domain.Character{{Name: "An", Role: "chính", Description: "Người kể"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []int{1, 2} {
		if err := s.Drafts.SaveFinalChapter(ch, fmt.Sprintf("Nội dung chương %d.", ch)); err != nil {
			t.Fatal(err)
		}
		if err := s.Progress.StartChapter(ch); err != nil {
			t.Fatal(err)
		}
		if err := s.Progress.MarkChapterComplete(ch, 100*ch, "cliff", "main"); err != nil {
			t.Fatal(err)
		}
	}
	// Chương 1 có tiêu đề trong summary; chương 2 phải rơi về tiêu đề dàn ý.
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "Tiêu đề tóm tắt", Summary: "x"}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestListChapters(t *testing.T) {
	dir := readerFixture(t)
	got, err := ListChapters(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Number != 1 || got[1].Number != 2 {
		t.Fatalf("danh sách sai: %+v", got)
	}
	if got[0].Title != "Tiêu đề tóm tắt" || got[1].Title != "Dàn ý hai" {
		t.Fatalf("tiêu đề sai: %+v", got)
	}
	if got[0].WordCount != 100 || got[1].WordCount != 200 {
		t.Fatalf("số từ sai: %+v", got)
	}
}

func TestListChaptersEmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := ListChapters(dir)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("muốn slice rỗng không nil, được %v %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("reader không được tạo file: %v", entries)
	}
}

func TestReadChapterPrevNext(t *testing.T) {
	dir := readerFixture(t)
	c1, err := ReadChapter(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Prev != 0 || c1.Next != 2 || c1.Content != "Nội dung chương 1." || c1.Title != "Tiêu đề tóm tắt" {
		t.Fatalf("chương 1 sai: %+v", c1)
	}
	c2, err := ReadChapter(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Prev != 1 || c2.Next != 0 {
		t.Fatalf("chương 2 sai: %+v", c2)
	}
}

func TestReadChapterNotCompleted(t *testing.T) {
	dir := readerFixture(t)
	for _, n := range []int{0, 3, 99} {
		if _, err := ReadChapter(dir, n); !errors.Is(err, ErrChapterNotFound) {
			t.Fatalf("chương %d: muốn ErrChapterNotFound, được %v", n, err)
		}
	}
}

func TestReadOutlineFlat(t *testing.T) {
	dir := readerFixture(t)
	o, err := ReadOutline(dir)
	if err != nil {
		t.Fatal(err)
	}
	if o.Title != "Truyện: Thử/Nghiệm?" || o.Synopsis != "Tóm tắt." {
		t.Fatalf("metadata sai: %+v", o)
	}
	if len(o.Volumes) != 1 || o.Volumes[0].Title != "" || len(o.Volumes[0].Chapters) != 3 {
		t.Fatalf("quyển sai: %+v", o.Volumes)
	}
	ch := o.Volumes[0].Chapters
	if !ch[0].Done || !ch[1].Done || ch[2].Done || ch[1].CoreEvent != "Xung đột" {
		t.Fatalf("chương sai: %+v", ch)
	}
	if len(o.Characters) != 1 || o.Characters[0].Name != "An" || o.Characters[0].Role != "chính" {
		t.Fatalf("nhân vật sai: %+v", o.Characters)
	}
}

func TestReadOutlineLayered(t *testing.T) {
	dir := readerFixture(t)
	s := store.NewStore(dir)
	if err := s.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
		Index: 1, Title: "Khởi đầu",
		Arcs: []domain.ArcOutline{
			{Index: 1, Title: "A", Chapters: []domain.OutlineEntry{{Chapter: 1, Title: "Mở đầu"}}},
			{Index: 2, Title: "B", Chapters: []domain.OutlineEntry{{Chapter: 2, Title: "Hai"}, {Chapter: 3, Title: "Ba"}}},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	o, err := ReadOutline(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Volumes) != 1 || o.Volumes[0].Title != "Quyển 1: Khởi đầu" || len(o.Volumes[0].Chapters) != 3 {
		t.Fatalf("quyển phân tầng sai: %+v", o.Volumes)
	}
}

func TestReadOutlineEmptyDir(t *testing.T) {
	o, err := ReadOutline(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if o.Title != "" || len(o.Volumes) != 0 || len(o.Characters) != 0 {
		t.Fatalf("muốn rỗng: %+v", o)
	}
}
