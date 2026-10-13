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
	"github.com/voocel/ainovel-cli/internal/utils"
)

const (
	helloTimeout     = 10 * time.Second
	stopGrace        = 10 * time.Second // shutdown → SIGTERM → SIGKILL, mỗi bước chờ chừng này
	stdoutBufBytes   = 4 << 20
	stderrRingLines  = 200
	stderrLogMax     = 5 << 20
	snapshotSaveGap  = 10 * time.Second
	lastErrorMaxSize = 8 << 10
)

// ackTimeout: thời gian chờ worker sẵn sàng và chờ ack (biến để test rút ngắn).
var ackTimeout = 5 * time.Second

// ErrNotRunning: truyện không có worker đang sống.
var ErrNotRunning = errors.New("truyện không có worker đang chạy")

// ErrNotReady: worker đang sống nhưng chưa sẵn sàng nhận lệnh (chưa chào/chưa nhận init).
var ErrNotReady = errors.New("worker chưa sẵn sàng nhận lệnh")

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
	sliced     atomic.Bool   // supervisor cắt lượt để nhường slot (chia lượt)
	started    time.Time     // lúc giữ slot; đặt trước khi proc vào s.procs
	done       chan struct{} // đóng khi run kết thúc việc với process (kể cả spawn lỗi)
	doneOnce   sync.Once

	amu  sync.Mutex
	acks map[string]chan api.WorkerMessage

	// mmu bảo vệ mode: chế độ viết user chọn (giá trị trong DB), không phải chế độ
	// worker đang chạy — lúc cắt lượt worker bị chuyển tạm sang review.
	mmu  sync.Mutex
	mode string

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

// isReady báo đã gửi init xong.
func (p *proc) isReady() bool {
	select {
	case <-p.ready:
		return true
	default:
		return false
	}
}

func (p *proc) advanceMode() string {
	p.mmu.Lock()
	defer p.mmu.Unlock()
	return p.mode
}

// setAdvanceMode ghi nhận chế độ user vừa chọn rồi gửi cho worker. Đang cắt lượt
// thì không gửi: worker phải giữ review để dừng ở ranh giới chương; DB đã lưu
// chế độ mới và lượt sau áp lại từ init.
func (p *proc) setAdvanceMode(ctx context.Context, cmd api.WorkerCommand) (*api.WorkerMessage, error) {
	// Chờ worker sẵn sàng trước khi khoá: run() cần mmu để gán mode trước khi spawn.
	if err := p.waitReady(ctx); err != nil {
		return nil, err
	}
	p.mmu.Lock()
	defer p.mmu.Unlock()
	p.mode = cmd.Mode
	if p.sliced.Load() {
		return &api.WorkerMessage{Type: api.MsgAck, ID: cmd.ID, OK: true}, nil
	}
	return p.command(ctx, cmd)
}

// stopAtChapterBoundary yêu cầu worker dừng sau khi chương đang viết commit, bằng
// cách chuyển sang review. Luôn gửi, kể cả khi user chọn review: mode là ý user,
// lệnh đổi mode trước đó có thể chưa tới worker. false = worker không nhận lệnh,
// phía gọi phải dừng hẳn.
func (p *proc) stopAtChapterBoundary(id string) bool {
	p.mmu.Lock()
	defer p.mmu.Unlock()
	ack, err := p.command(context.Background(), api.WorkerCommand{Type: api.MsgSetAdvanceMode, ID: id, Mode: api.AdvanceReview})
	// ack nil kèm err nil: đã gửi nhưng chưa xác nhận kịp; thời gian ân hạn vẫn chặn.
	return err == nil && (ack == nil || ack.OK)
}

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
// xác nhận (kết quả sẽ đến qua SSE). ErrNotReady: worker chưa sẵn sàng sau ackTimeout,
// lệnh CHƯA được gửi.
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
	if err := p.waitReady(ctx); err != nil {
		return nil, err
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

// waitReady chờ init gửi xong tối đa ackTimeout.
func (p *proc) waitReady(ctx context.Context) error {
	select {
	case <-p.ready:
		return nil
	case <-p.done:
		return ErrNotRunning
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(ackTimeout):
		return ErrNotReady
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

// pumpStderr ghi stderr của worker vào logs/worker-stderr.log (cắt khi mở nếu quá 5MB,
// và ngừng ghi khi đạt 5MB trong lúc chạy)
// và giữ 200 dòng cuối để báo lỗi khi worker chết bất thường.
func (p *proc) pumpStderr(r io.Reader, bookDir string) {
	var f *os.File
	var written int64
	logPath := filepath.Join(bookDir, "logs", "worker-stderr.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err == nil {
		flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
		if fi, err := os.Stat(logPath); err == nil {
			if fi.Size() > stderrLogMax {
				flags |= os.O_TRUNC
			} else {
				written = fi.Size()
			}
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
		if f != nil && written < stderrLogMax {
			n, _ := f.WriteString(line + "\n")
			written += int64(n)
			if written >= stderrLogMax {
				_, _ = f.WriteString("[đã đạt giới hạn kích thước log, bỏ qua phần còn lại của lượt chạy này]\n")
			}
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
	// Pending chỉ bị xoá khi lượt kết thúc sạch: lượt lỗi phải chạy lại đúng việc đang chờ.
	res.ClearPending = true
	if p.errCode == api.ErrCodeLockBusy {
		return finishResult{}, true
	}
	// Lượt bị cắt để nhường slot: về cuối hàng đợi (updated_at = now), giữ việc
	// đang chờ như khi drain (chạy lại start/steer nếu đã có, không thì resume),
	// không tính là lỗi. Kể cả khi worker đóng chậm và bị SIGKILL (không có exit).
	// Worker dừng vì user bấm tạm dừng (stop_reason user) thì giữ ý user.
	if p.sliced.Load() && (p.exit == nil || (p.exit.Reason == api.ExitShutdown && p.exit.StopReason != api.StopUser)) {
		res.Status = api.StatusQueued
		res.ClearPending = false
		res.ResumeNext = true
		return res, false
	}
	// Lượt bị cắt dừng đúng ranh giới chương: chương đã commit, việc đang chờ đã
	// xong nên xoá pending (lượt sau resume; can thiệp còn treo nằm trong store
	// của engine). Chỉ áp khi user muốn auto — dừng ở cổng chương là do cắt lượt
	// gây ra; user muốn review thì giữ kết quả thật.
	if p.sliced.Load() && p.exit != nil && p.exit.Reason == api.ExitStopped &&
		(p.exit.StopReason == api.StopReviewWait || p.exit.StopReason == api.StopPendingSteer) &&
		p.advanceMode() == api.AdvanceAuto {
		res.Status = api.StatusQueued
		res.Snapshot = marshalSnapshot(p.snapshotWithMode(api.AdvanceAuto))
		return res, false
	}
	if p.exit == nil {
		res.Status = api.StatusFailed
		res.Failed = true
		res.StopReason = api.StopError
		res.ClearPending = false
		res.LastError = msgWorkerCrashed
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
		res.ClearPending = false
		res.LastError = msgRunFailed
		p.applyProviderRejected(&res)
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
		res.ClearPending = false
		res.LastError = msgRunFailed
		p.applyProviderRejected(&res)
	}
	return res, false
}

// snapshotWithMode trả bản sao snapshot cuối với chế độ viết của user, để snapshot
// lưu lại không hiện review tạm thời do cắt lượt.
func (p *proc) snapshotWithMode(mode string) *api.BookSnapshot {
	if p.lastSnap == nil {
		return nil
	}
	snap := *p.lastSnap
	snap.AdvanceMode = mode
	return &snap
}

// applyProviderRejected: provider từ chối hẳn thì user cần biết nguyên nhân để
// đổi key/provider, nên giữ message của worker (dựng từ bảng cố định, không có
// chi tiết nội bộ). Mã lỗi khác vẫn dùng message chung.
func (p *proc) applyProviderRejected(res *finishResult) {
	if p.errCode != api.ErrCodeProviderRejected || strings.TrimSpace(p.errMsg) == "" {
		return
	}
	res.StopReason = api.StopProviderRejected
	res.LastError = p.errMsg
	if len([]rune(p.errMsg)) > maxProviderRejectedRunes {
		res.LastError = utils.TruncateRunes(p.errMsg, maxProviderRejectedRunes-1) // TruncateRunes thêm "…"
	}
}

// maxProviderRejectedRunes là độ dài tối đa của last_error khi provider từ chối.
const maxProviderRejectedRunes = 300

// Thông báo gửi client: chi tiết (stderr, đường dẫn) chỉ nằm trong log server.
const (
	msgWorkerCrashed = "Worker dừng bất thường. Hãy bấm \"Thử lại\"; nếu lỗi lặp lại, hãy liên hệ quản trị viên."
	msgRunFailed     = "Lượt chạy gặp lỗi. Hãy bấm \"Thử lại\"; nếu lỗi lặp lại, hãy liên hệ quản trị viên."
)

// failureDetail gom chi tiết lỗi của lượt chạy để ghi log server.
func (p *proc) failureDetail() []any {
	args := []any{"err_code", p.errCode, "err_msg", p.errMsg, "stderr_tail", p.stderrTail()}
	if p.exit != nil {
		args = append(args, "exit_reason", p.exit.Reason, "stop_reason", p.exit.StopReason, "exit_error", p.exit.Error)
	}
	return args
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
