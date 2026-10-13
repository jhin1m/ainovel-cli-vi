package books

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

func TestExportTXTCached(t *testing.T) {
	dir := readerFixture(t)
	e := NewExporter(2)
	p1, name, err := e.Export(context.Background(), "u1", dir, "txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p1, "/exports/2-") || !strings.HasSuffix(p1, ".txt") {
		t.Fatalf("đường dẫn cache sai: %s", p1)
	}
	if name != "Truyện Thử Nghiệm.txt" {
		t.Fatalf("tên tải về sai: %q", name)
	}
	data, err := os.ReadFile(p1)
	if err != nil || !strings.Contains(string(data), "Nội dung chương 2.") {
		t.Fatalf("nội dung xuất sai: %v", err)
	}
	fi1, _ := os.Stat(p1)
	time.Sleep(20 * time.Millisecond)
	p2, _, err := e.Export(context.Background(), "u1", dir, "txt")
	if err != nil || p2 != p1 {
		t.Fatalf("lần hai: %s %v", p2, err)
	}
	fi2, _ := os.Stat(p2)
	if !fi2.ModTime().Equal(fi1.ModTime()) {
		t.Fatal("cache không được dùng lại (mtime đổi)")
	}
	entries, _ := os.ReadDir(dir + "/exports")
	if len(entries) != 1 {
		t.Fatalf("còn file tạm: %v", entries)
	}
}

func TestExportEPUB(t *testing.T) {
	dir := readerFixture(t)
	p, name, err := NewExporter(1).Export(context.Background(), "u", dir, "epub")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) < 4 || string(data[:2]) != "PK" {
		t.Fatalf("epub không phải zip: %v", err)
	}
	if !strings.HasSuffix(name, ".epub") {
		t.Fatalf("tên: %s", name)
	}
}

func TestExportBadFormat(t *testing.T) {
	if _, _, err := NewExporter(1).Export(context.Background(), "u", readerFixture(t), "pdf"); err == nil {
		t.Fatal("muốn lỗi định dạng")
	}
}

func TestExportNothingToExport(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(1); err != nil {
		t.Fatal(err)
	}
	if err := s.Book.Save(domain.BookMetadata{Title: "Rỗng", Synopsis: "x"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := NewExporter(1).Export(context.Background(), "u", dir, "txt")
	if !errors.Is(err, ErrNothingToExport) {
		t.Fatalf("muốn ErrNothingToExport, được %v", err)
	}
}

func TestExportBusy(t *testing.T) {
	dir := readerFixture(t)
	e := NewExporter(1)

	// Slot của chính người dùng đang bị giữ.
	rel, ok := e.acquire("u1")
	if !ok {
		t.Fatal("acquire đầu phải thành công")
	}
	if _, _, err := e.Export(context.Background(), "u1", dir, "txt"); !errors.Is(err, ErrExportBusy) {
		t.Fatalf("slot người dùng: %v", err)
	}
	// Semaphore toàn cục đã đầy, người dùng khác cũng bị từ chối.
	if _, _, err := e.Export(context.Background(), "u2", dir, "txt"); !errors.Is(err, ErrExportBusy) {
		t.Fatalf("slot toàn cục: %v", err)
	}
	rel()
	if _, _, err := e.Export(context.Background(), "u2", dir, "txt"); err != nil {
		t.Fatalf("sau khi nhả: %v", err)
	}
}

func TestExportFileName(t *testing.T) {
	cases := map[string]string{
		"":         "truyen.txt",
		" ../..  ": "truyen.txt",
		"a/b\\c:d": "a b c d.txt",
		"Đại Mộng": "Đại Mộng.txt",
		"x\ny":     "x y.txt",
	}
	for in, want := range cases {
		if got := exportFileName(in, "txt"); got != want {
			t.Errorf("exportFileName(%q) = %q, muốn %q", in, got, want)
		}
	}
}

// Viết lại một chương (số chương không đổi) phải làm cache export mất hiệu lực.
func TestExportCacheInvalidatedByRewrite(t *testing.T) {
	dir := readerFixture(t)
	e := NewExporter(2)
	p1, _, err := e.Export(context.Background(), "u1", dir, "txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.NewStore(dir).Drafts.SaveFinalChapter(2, "Bản viết lại hoàn toàn khác, dài hơn trước nhiều."); err != nil {
		t.Fatal(err)
	}
	p2, _, err := e.Export(context.Background(), "u1", dir, "txt")
	if err != nil {
		t.Fatal(err)
	}
	if p2 == p1 {
		t.Fatal("cache cũ vẫn được dùng sau khi viết lại chương")
	}
	data, _ := os.ReadFile(p2)
	if !strings.Contains(string(data), "Bản viết lại hoàn toàn khác") || strings.Contains(string(data), "Nội dung chương 2.") {
		t.Fatalf("export không phản ánh chương đã viết lại: %q", data)
	}
	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Fatalf("cache cũ phải bị dọn: %v", err)
	}
}

// WaitDir chờ lượt xuất đang chạy trên thư mục trước khi cho xoá.
func TestExporterWaitDirBlocksUntilExportEnds(t *testing.T) {
	e := NewExporter(1)
	e.enter("/d")
	done := make(chan struct{})
	go func() {
		e.WaitDir("/d")
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("WaitDir trả về khi export còn chạy")
	case <-time.After(50 * time.Millisecond):
	}
	e.leave("/d")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitDir không trả về sau khi export kết thúc")
	}
	e.WaitDir("/khac") // thư mục không có export: không chờ
}
