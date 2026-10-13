package books

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/voocel/ainovel-cli/internal/host/exp"
	"github.com/voocel/ainovel-cli/internal/store"
)

var (
	// ErrExportBusy: hết chỗ xuất đồng thời (toàn cục hoặc của người dùng này).
	ErrExportBusy = errors.New("hệ thống đang bận xuất file, vui lòng thử lại sau")
	// ErrNothingToExport: truyện chưa có chương nào hoàn thành.
	ErrNothingToExport = errors.New("chưa có chương nào hoàn thành để xuất")
)

// Exporter xuất truyện ra txt/epub, giới hạn số lượt chạy đồng thời.
type Exporter struct {
	sem chan struct{}

	mu    sync.Mutex
	users map[string]struct{}
}

// NewExporter tạo Exporter cho phép tối đa maxConcurrent lượt xuất cùng lúc
// và mỗi người dùng một lượt.
func NewExporter(maxConcurrent int) *Exporter {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Exporter{sem: make(chan struct{}, maxConcurrent), users: make(map[string]struct{})}
}

// acquire giữ chỗ không chặn; trả hàm giải phóng.
func (e *Exporter) acquire(userID string) (func(), bool) {
	e.mu.Lock()
	if _, busy := e.users[userID]; busy {
		e.mu.Unlock()
		return nil, false
	}
	select {
	case e.sem <- struct{}{}:
	default:
		e.mu.Unlock()
		return nil, false
	}
	e.users[userID] = struct{}{}
	e.mu.Unlock()
	return func() {
		e.mu.Lock()
		delete(e.users, userID)
		e.mu.Unlock()
		<-e.sem
	}, true
}

// Export trả về đường dẫn file xuất và tên tải về. Kết quả được cache theo số chương
// đã hoàn thành tại <dir>/exports/<N>.<ext>.
func (e *Exporter) Export(ctx context.Context, userID, dir, format string) (path, downloadName string, err error) {
	var f exp.Format
	switch format {
	case "txt":
		f = exp.FormatTXT
	case "epub":
		f = exp.FormatEPUB
	default:
		return "", "", fmt.Errorf("định dạng xuất không được hỗ trợ: %q", format)
	}
	release, ok := e.acquire(userID)
	if !ok {
		return "", "", ErrExportBusy
	}
	defer release()

	st := store.NewStore(dir)
	p, err := st.Progress.Load()
	if err != nil {
		return "", "", fmt.Errorf("đọc tiến độ truyện: %w", err)
	}
	if p == nil || len(p.CompletedChapters) == 0 {
		return "", "", ErrNothingToExport
	}
	title := ""
	if book, err := st.Book.Load(); err == nil && book != nil {
		title = book.Title
	}
	downloadName = exportFileName(title, format)

	exportsDir := filepath.Join(dir, "exports")
	final := filepath.Join(exportsDir, fmt.Sprintf("%d.%s", len(p.CompletedChapters), format))
	if fi, statErr := os.Stat(final); statErr == nil && fi.Mode().IsRegular() {
		return final, downloadName, nil
	}
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		return "", "", fmt.Errorf("tạo thư mục xuất: %w", err)
	}
	// Tên tạm kết thúc bằng đuôi đúng định dạng; exp.Run ghi thẳng vào đó.
	tmp, err := os.CreateTemp(exportsDir, "export-*."+format)
	if err != nil {
		return "", "", fmt.Errorf("tạo file tạm: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath) // no-op sau khi rename thành công

	if _, err := exp.Run(ctx, exp.Deps{Store: st}, exp.Options{Format: f, OutPath: tmpPath, Overwrite: true}); err != nil {
		return "", "", fmt.Errorf("xuất truyện: %w", err)
	}
	if err := os.Rename(tmpPath, final); err != nil {
		return "", "", fmt.Errorf("lưu file xuất: %w", err)
	}
	return final, downloadName, nil
}

// exportFileName làm sạch tên sách thành tên file an toàn; rỗng → "truyen".
func exportFileName(title, format string) string {
	var b strings.Builder
	for _, r := range title {
		switch {
		case unicode.IsControl(r), strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	name := strings.Join(strings.Fields(b.String()), " ")
	name = strings.Trim(name, ". ")
	if r := []rune(name); len(r) > 100 {
		name = strings.TrimSpace(string(r[:100]))
	}
	if name == "" {
		name = "truyen"
	}
	return name + "." + format
}
