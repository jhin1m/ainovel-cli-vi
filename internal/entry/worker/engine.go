package worker

import (
	"errors"

	"github.com/voocel/ainovel-cli/assets"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/host"
)

// engine là phần của host.Host mà worker dùng tới. Giữ hẹp để test bằng fake và
// để upstream đổi chữ ký host thì lỗi lộ ra ngay lúc compile.
type engine interface {
	PrepareUserRules(rawPrompt string) error
	StartPrepared(rawRequirement string) error
	Resume() (string, error)
	Continue(text string) error
	Steer(text string) error
	SetAdvanceMode(mode domain.ChapterAdvanceMode) error
	AdvanceOneChapter() error
	Abort() bool
	Close()
	Events() <-chan host.Event
	Stream() <-chan string
	Done() <-chan struct{}
	Snapshot() host.UISnapshot
}

// engineFactory tạo engine cho một thư mục truyện; test thay bằng fake.
type engineFactory func(cfg bootstrap.Config, bundle assets.Bundle) (engine, error)

func newHostEngine(cfg bootstrap.Config, bundle assets.Bundle) (engine, error) {
	h, err := host.New(cfg, bundle, host.WithFileLog("worker.log", false))
	if err != nil {
		return nil, err
	}
	return h, nil
}

// isLockBusy báo lỗi do thư mục truyện đang bị process khác giữ khoá.
func isLockBusy(err error) bool {
	return errors.Is(err, host.ErrBookInUse)
}
