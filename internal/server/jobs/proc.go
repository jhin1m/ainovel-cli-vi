package jobs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voocel/ainovel-cli/internal/server/api"
)

const (
	helloTimeout     = 10 * time.Second
	ackTimeout       = 5 * time.Second
	stopGrace        = 10 * time.Second // shutdown → SIGTERM → SIGKILL, mỗi bước chờ chừng này
	stdoutBufBytes   = 4 << 20
	stderrRingLines  = 200
	stderrLogMax     = 5 << 20
	snapshotSaveGap  = 10 * time.Second
	lastErrorMaxSize = 8 << 10
)

// ErrNotRunning: truyện không có worker đang sống.
var ErrNotRunning = errors.New("truyện không có worker đang chạy")

// proc là một process worker đang sống của một truyện.
type proc struct {
	bookID, userID string
	cmd            *exec.Cmd
	hub            *Hub

	wmu   sync.Mutex // một writer stdin
	stdin io.WriteCloser

	ready      chan struct{} // đóng sau khi đã gửi init
	readyOnce  sync.Once
	abortLater atomic.Bool // lệnh abort đến trước khi init gửi xong
	stopping   atomic.Bool
	done       chan struct{} // đóng khi run kết thúc việc với process (kể cả spawn lỗi)
	doneOnce   sync.Once

	amu  sync.Mutex
	acks map[string]chan api.WorkerMessage

	// Chỉ goroutine đọc stdout ghi; đọc sau khi done đóng.
	exit     *api.WorkerMessage
	errCode  string
	errMsg   string
	lastSnap *api.BookSnapshot

	smu        sync.Mutex
	stderrRing []string
}

func newProc(bookID, userID string, hub *Hub) *proc {
	return &proc{
		bookID: bookID, userID: userID, hub: hub,
		ready: make(chan struct{}),
		done:  make(chan struct{}),
		acks:  make(map[string]chan api.WorkerMessage),
	}
}

func (p *proc) markReady() { p.readyOnce.Do(func() { close(p.ready) }) }

// markDone đóng done đúng một lần; run gọi cả ở đường thường lẫn đường spawn lỗi.
func (p *proc) markDone() { p.doneOnce.Do(func() { close(p.done) }) }

// signal gửi SIGTERM (hard=false) hoặc SIGKILL (hard=true) tới nhóm process của worker.
// Không làm gì nếu worker chưa được spawn (spawn lỗi).
func (p *proc) signal(hard bool) {
	p.wmu.Lock()
	cmd := p.cmd
	p.wmu.Unlock()
	if cmd != nil {
		signalGroup(cmd, hard)
	}
}

// writeLine ghi một message JSON ra stdin của worker.
func (p *proc) writeLine(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	p.wmu.Lock()
	defer p.wmu.Unlock()
	if p.stdin == nil {
		return ErrNotRunning
	}
	_, err = p.stdin.Write(b)
	clear(b)
	return err
}

// command gửi lệnh và chờ ack tối đa ackTimeout. ack nil = đã gửi nhưng chưa có
// xác nhận (kết quả sẽ đến qua SSE).
func (p *proc) command(ctx context.Context, cmd api.WorkerCommand) (*api.WorkerMessage, error) {
	if cmd.Type == api.MsgAbort {
		select {
		case <-p.ready:
		default:
			// Init chưa gửi xong: ghi nhớ, gửi ngay sau init (worker kiểm lại sau action).
			p.abortLater.Store(true)
			// run có thể vừa đánh dấu ready sau khi đã kiểm cờ: kiểm lại để abort không bị mất.
			select {
			case <-p.ready:
			default:
				return nil, nil
			}
		}
	}
	select {
	case <-p.ready:
	case <-p.done:
		return nil, ErrNotRunning
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(ackTimeout):
		return nil, nil
	}
	ch := make(chan api.WorkerMessage, 1)
	p.amu.Lock()
	p.acks[cmd.ID] = ch
	p.amu.Unlock()
	defer func() {
		p.amu.Lock()
		delete(p.acks, cmd.ID)
		p.amu.Unlock()
	}()
	if err := p.writeLine(cmd); err != nil {
		return nil, ErrNotRunning
	}
	select {
	case ack := <-ch:
		return &ack, nil
	case <-p.done:
		return nil, ErrNotRunning
	case <-ctx.Done():
		return nil, nil
	case <-time.After(ackTimeout):
		return nil, nil
	}
}

// terminate dừng worker theo thứ tự: lệnh shutdown → SIGTERM nhóm → SIGKILL nhóm,
// mỗi bước chờ grace. Trả về khi process đã thoát (Wait xong).
func (p *proc) terminate(grace time.Duration) {
	p.stopping.Store(true)
	select {
	case <-p.ready:
		_ = p.writeLine(api.WorkerCommand{Type: api.MsgShutdown, ID: "shutdown"})
	default:
		// Chưa gửi init: đóng stdin, worker coi EOF như shutdown.
		p.wmu.Lock()
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		p.wmu.Unlock()
	}
	select {
	case <-p.done:
		return
	case <-time.After(grace):
	}
	p.signal(false)
	select {
	case <-p.done:
		return
	case <-time.After(grace):
	}
	p.signal(true)
	<-p.done
}

func (p *proc) pushStderr(line string) {
	p.smu.Lock()
	defer p.smu.Unlock()
	p.stderrRing = append(p.stderrRing, line)
	if len(p.stderrRing) > stderrRingLines {
		p.stderrRing = p.stderrRing[len(p.stderrRing)-stderrRingLines:]
	}
}

func (p *proc) stderrTail() string {
	p.smu.Lock()
	defer p.smu.Unlock()
	s := strings.Join(p.stderrRing, "\n")
	if len(s) > lastErrorMaxSize {
		s = s[len(s)-lastErrorMaxSize:]
	}
	return s
}

// pumpStderr ghi stderr của worker vào logs/worker-stderr.log (cắt khi quá 5MB)
// và giữ 200 dòng cuối để báo lỗi khi worker chết bất thường.
func (p *proc) pumpStderr(r io.Reader, bookDir string) {
	var f *os.File
	logPath := filepath.Join(bookDir, "logs", "worker-stderr.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err == nil {
		flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
		if fi, err := os.Stat(logPath); err == nil && fi.Size() > stderrLogMax {
			flags |= os.O_TRUNC
		}
		f, _ = os.OpenFile(logPath, flags, 0o640)
	}
	if f != nil {
		defer f.Close()
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		p.pushStderr(line)
		if f != nil {
			_, _ = f.WriteString(line + "\n")
		}
	}
}

// handle xử lý một message từ stdout của worker.
func (p *proc) handle(msg api.WorkerMessage, saveSnap func(*api.BookSnapshot, bool)) {
	switch msg.Type {
	case api.MsgEvent:
		if msg.Event != nil {
			p.hub.PublishEvent(*msg.Event)
		}
	case api.MsgStream:
		p.hub.PublishStream(api.StreamDelta{Delta: msg.Delta, Clear: msg.Clear})
	case api.MsgSnapshot:
		if msg.Snapshot != nil {
			prev := p.lastSnap
			p.lastSnap = msg.Snapshot
			p.hub.SetSnapshot(*msg.Snapshot)
			changed := prev == nil || prev.Phase != msg.Snapshot.Phase ||
				prev.CompletedCount != msg.Snapshot.CompletedCount || prev.RuntimeState != msg.Snapshot.RuntimeState
			saveSnap(msg.Snapshot, changed)
		}
	case api.MsgAck:
		p.amu.Lock()
		ch := p.acks[msg.ID]
		p.amu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
	case api.MsgError:
		p.errCode, p.errMsg = msg.Code, msg.Message
	case api.MsgExit:
		m := msg
		p.exit = &m
	}
}

// outcome quy đổi cách worker kết thúc thành trạng thái truyện.
func (p *proc) outcome() (res finishResult, requeue bool) {
	res.Snapshot = marshalSnapshot(p.lastSnap)
	res.ClearPending = true
	if p.errCode == api.ErrCodeLockBusy {
		return finishResult{}, true
	}
	if p.exit == nil {
		res.Status = api.StatusFailed
		res.Failed = true
		res.StopReason = api.StopError
		res.LastError = "Worker dừng bất thường"
		if tail := p.stderrTail(); tail != "" {
			res.LastError += ":\n" + tail
		}
		return res, false
	}
	switch p.exit.Reason {
	case api.ExitShutdown:
		res.Status = api.StatusPaused
		res.StopReason = p.exit.StopReason
		return res, false
	case api.ExitError:
		res.Status = api.StatusFailed
		res.Failed = true
		res.StopReason = api.StopError
		res.LastError = firstNonEmpty(p.exit.Error, p.errMsg, "Worker báo lỗi")
		return res, false
	}
	res.StopReason = p.exit.StopReason
	switch p.exit.StopReason {
	case api.StopReviewWait:
		res.Status = api.StatusWaitingReview
	case api.StopBudget:
		res.Status = api.StatusBudgetExhausted
	case api.StopComplete, api.StopNothingToResume:
		res.Status = api.StatusCompleted
	case api.StopUser, api.StopHold, api.StopPendingSteer:
		res.Status = api.StatusPaused
	default:
		res.Status = api.StatusFailed
		res.Failed = true
		res.LastError = firstNonEmpty(p.exit.Error, p.errMsg, "Engine dừng vì lỗi")
	}
	return res, false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// readHello đọc dòng đầu tiên, phải là hello khớp giao thức và phiên bản.
func readHello(lines <-chan []byte, version string) error {
	select {
	case line, ok := <-lines:
		if !ok {
			return fmt.Errorf("worker thoát trước khi chào")
		}
		var m api.WorkerMessage
		if err := json.Unmarshal(line, &m); err != nil || m.Type != api.MsgHello {
			return fmt.Errorf("dòng đầu của worker không phải hello")
		}
		if m.Protocol != api.WorkerProtocol || m.Version != version {
			return fmt.Errorf("worker lệch phiên bản (protocol %d, version %q; cần %d, %q)", m.Protocol, m.Version, api.WorkerProtocol, version)
		}
		return nil
	case <-time.After(helloTimeout):
		return fmt.Errorf("worker không chào sau %v", helloTimeout)
	}
}

func logProc(p *proc) *slog.Logger {
	return slog.Default().With("module", "jobs", "book_id", p.bookID)
}
