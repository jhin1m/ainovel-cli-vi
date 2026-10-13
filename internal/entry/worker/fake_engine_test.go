package worker

import (
	"sync"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/host"
)

// fakeEngine mô phỏng host.Host: ghi lại lời gọi, cho test điều khiển lúc nào
// engine chạy/dừng và cho phép chặn các lời gọi lâu (arbiter/LLM).
type fakeEngine struct {
	mu      sync.Mutex
	calls   []string
	snap    host.UISnapshot
	running bool
	closed  bool

	events chan host.Event
	stream chan string
	done   chan struct{}

	// startBlock/steerBlock != nil: lời gọi tương ứng chờ tới khi kênh đóng.
	startBlock chan struct{}
	steerBlock chan struct{}

	resumeLabel string
	startErr    error
	// continueStarts=false: Continue không khởi động engine (vd. chỉ trả lời).
	continueStarts bool
	continueText   string
	closeCalled    chan struct{}
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		events:         make(chan host.Event, 100),
		stream:         make(chan string, 256),
		done:           make(chan struct{}, 4),
		continueStarts: true,
		resumeLabel:    "Khôi phục",
		closeCalled:    make(chan struct{}),
	}
}

func (f *fakeEngine) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

func (f *fakeEngine) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeEngine) setRunning() {
	f.mu.Lock()
	f.running = true
	f.snap.RuntimeState = "running"
	f.mu.Unlock()
}

// finish dừng engine như runEnded: cập nhật snapshot rồi báo Done.
func (f *fakeEngine) finish(mut func(*host.UISnapshot)) {
	f.mu.Lock()
	f.running = false
	f.snap.RuntimeState = "idle"
	if mut != nil {
		mut(&f.snap)
	}
	if !f.closed {
		select {
		case f.done <- struct{}{}:
		default:
		}
	}
	f.mu.Unlock()
}

func (f *fakeEngine) emit(ev host.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.events <- ev
	}
}

func (f *fakeEngine) emitDelta(d string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.stream <- d
	}
}

func (f *fakeEngine) PrepareUserRules(string) error { f.record("PrepareUserRules"); return nil }

func (f *fakeEngine) StartPrepared(string) error {
	f.record("StartPrepared")
	if f.startBlock != nil {
		<-f.startBlock
	}
	if f.startErr != nil {
		return f.startErr
	}
	f.setRunning()
	return nil
}

func (f *fakeEngine) Resume() (string, error) {
	f.record("Resume")
	if f.resumeLabel != "" {
		f.setRunning()
	}
	return f.resumeLabel, nil
}

func (f *fakeEngine) Continue(text string) error {
	f.record("Continue")
	f.mu.Lock()
	f.continueText = text
	f.mu.Unlock()
	if f.continueStarts {
		f.setRunning()
	}
	return nil
}

func (f *fakeEngine) Steer(string) error {
	f.record("Steer")
	if f.steerBlock != nil {
		<-f.steerBlock
	}
	return nil
}

func (f *fakeEngine) SetAdvanceMode(mode domain.ChapterAdvanceMode) error {
	f.record("SetAdvanceMode:" + string(mode))
	f.mu.Lock()
	f.snap.AdvanceMode = string(mode)
	f.mu.Unlock()
	return nil
}

func (f *fakeEngine) AdvanceOneChapter() error {
	f.record("AdvanceOneChapter")
	f.setRunning()
	return nil
}

func (f *fakeEngine) Abort() bool {
	f.record("Abort")
	f.mu.Lock()
	running := f.running
	f.mu.Unlock()
	if !running {
		return false
	}
	f.finish(func(s *host.UISnapshot) { s.RuntimeState = "paused" })
	return true
}

func (f *fakeEngine) Close() {
	f.record("Close")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.closed = true
	f.running = false
	close(f.events)
	close(f.stream)
	close(f.done)
	close(f.closeCalled)
}

func (f *fakeEngine) Events() <-chan host.Event { return f.events }
func (f *fakeEngine) Stream() <-chan string     { return f.stream }
func (f *fakeEngine) Done() <-chan struct{}     { return f.done }
func (f *fakeEngine) Snapshot() host.UISnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.snap
	s.IsRunning = f.running
	return s
}
