// Package worker là chế độ `ainovel-cli worker`: chạy đúng một lượt engine cho
// một truyện, nhận lệnh JSON Lines qua stdin, phát JSON Lines qua stdout, rồi
// thoát mỗi khi engine dừng (chờ duyệt, hết ngân sách, người dùng dừng, xong, lỗi).
package worker

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voocel/ainovel-cli/assets"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/entry/startup"
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/lang"
	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/rules"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// defaultMemoryLimit là GOMEMLIMIT mềm khi môi trường không đặt sẵn.
const defaultMemoryLimit = 1 << 30

// deps gom các phụ thuộc có thể thay trong test.
type deps struct {
	version       string
	newEngine     engineFactory
	registry      *models.ModelRegistry
	getwd         func() (string, error)
	prepare       func(cfg bootstrap.Config) (assets.Bundle, error)
	pollInterval  time.Duration // chu kỳ kiểm snapshot / điều kiện dừng
	snapshotEvery time.Duration // snapshot định kỳ dù không đổi
	closeGrace    time.Duration // thời gian tối đa chờ Close khi shutdown
}

// Run chạy worker với engine thật. version phải khớp phiên bản của server.
func Run(ctx context.Context, in io.Reader, out io.Writer, version string) error {
	lowerOOMPriority()
	if debug.SetMemoryLimit(-1) == math.MaxInt64 {
		debug.SetMemoryLimit(defaultMemoryLimit)
	}
	return run(ctx, in, out, deps{
		version:       version,
		newEngine:     newHostEngine,
		registry:      models.DefaultRegistry(),
		getwd:         os.Getwd,
		prepare:       prepareRuntime,
		pollInterval:  3 * time.Second, // Snapshot() đọc nhiều file store: giữ nhịp như TUI
		snapshotEvery: 9 * time.Second,
		closeGrace:    20 * time.Second,
	})
}

// prepareRuntime đặt ngôn ngữ toàn cục và nạp tài nguyên prompt/văn phong như
// entry TUI/headless, nhưng không đọc config trên đĩa.
func prepareRuntime(cfg bootstrap.Config) (assets.Bundle, error) {
	code, err := lang.Parse(cfg.Language)
	if err != nil {
		return assets.Bundle{}, err
	}
	lang.Set(code)
	if err := assets.CheckLanguage(code); err != nil {
		return assets.Bundle{}, err
	}
	rules.EnsureHomeRulesDir()
	opts := assets.DefaultLoadOptions(cfg.OutputDir)
	opts.Language = code
	return assets.Load(cfg.Style, opts), nil
}

// writer là lối ghi stdout duy nhất; mỗi message một dòng JSON. Sau message exit
// mọi lần ghi bị bỏ qua để server không nhận rác sau khi đã chốt trạng thái.
type writer struct {
	mu     sync.Mutex
	enc    *json.Encoder
	closed bool
}

func newWriter(out io.Writer) *writer {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return &writer{enc: enc}
}

func (w *writer) send(msg api.WorkerMessage) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	_ = w.enc.Encode(msg)
}

// finish ghi message exit rồi khoá writer.
func (w *writer) finish(msg api.WorkerMessage) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	_ = w.enc.Encode(msg)
	w.closed = true
}

// job là một việc chạy tuần tự trên executor: action ban đầu hoặc lệnh runtime.
type job struct {
	action bool
	cmd    api.WorkerCommand
}

type actionResult struct {
	nothingToResume bool
	err             error
}

type session struct {
	d      deps
	eng    engine
	w      *writer
	init   api.WorkerInit
	mapper eventMapper

	aborted  atomic.Bool  // người dùng đã bấm dừng (dính, kiểm lại sau action)
	stopping atomic.Bool  // đang đóng engine: job còn trong hàng đợi bị bỏ
	inFlight atomic.Int32 // số job đã nhận nhưng chưa chạy xong
	lastErr  atomic.Value // summary của event lỗi/cảnh báo gần nhất kể từ khi action xong (string)
	sawHold  bool         // trong lượt chạy từng thấy AdvanceHold (chỉ loop đọc/ghi)

	jobs     chan job
	jobDone  chan struct{}
	actionCh chan actionResult

	shutdownOnce sync.Once
	shutdownCh   chan struct{}
	pumpDone     chan struct{} // đóng khi pump đã xả hết kênh của engine
}

func run(ctx context.Context, in io.Reader, out io.Writer, d deps) error {
	w := newWriter(out)
	w.send(api.WorkerMessage{Type: api.MsgHello, Protocol: api.WorkerProtocol, Version: d.version})

	lines := make(chan []byte)
	scanErr := make(chan error, 1)
	go scanLines(in, lines, scanErr)

	var initLine []byte
	select {
	case line, ok := <-lines:
		if !ok {
			return failBeforeEngine(w, api.ErrCodeProtocol, errors.New("stdin đóng trước khi nhận init"))
		}
		initLine = line
	case <-ctx.Done():
		w.finish(api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitShutdown})
		return nil
	}

	msg, err := decodeInit(initLine)
	clear(initLine) // dòng init chứa API key rõ
	if err != nil {
		return failBeforeEngine(w, api.ErrCodeProtocol, err)
	}
	bookDir, err := checkBookDir(msg.BookDir, d.getwd)
	if err != nil {
		return failBeforeEngine(w, api.ErrCodeConfigInvalid, err)
	}
	cfg, err := buildConfig(msg, bookDir)
	msg.LLM.APIKey = "" // từ đây key chỉ còn trong cfg của engine
	if err != nil {
		return failBeforeEngine(w, api.ErrCodeConfigInvalid, err)
	}
	if err := applyPricing(d.registry, msg); err != nil {
		return failBeforeEngine(w, api.ErrCodeModelUnpriced, err)
	}
	bundle, err := d.prepare(cfg)
	if err != nil {
		return failBeforeEngine(w, api.ErrCodeConfigInvalid, err)
	}
	eng, err := d.newEngine(cfg, bundle)
	if err != nil {
		code := api.ErrCodeEngine
		if isLockBusy(err) {
			code = api.ErrCodeLockBusy
		}
		return failBeforeEngine(w, code, err)
	}

	s := &session{
		d:          d,
		eng:        eng,
		w:          w,
		init:       msg,
		mapper:     eventMapper{runID: newRunID()},
		jobs:       make(chan job, 16),
		jobDone:    make(chan struct{}, 1),
		actionCh:   make(chan actionResult, 1),
		shutdownCh: make(chan struct{}),
	}
	s.lastErr.Store("")
	return s.loop(ctx, lines, scanErr)
}

// scanLines đọc stdin theo dòng; không bao giờ chờ engine.
func scanLines(in io.Reader, lines chan<- []byte, scanErr chan<- error) {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)
	for sc.Scan() {
		line := append([]byte(nil), sc.Bytes()...)
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		lines <- line
	}
	scanErr <- sc.Err()
	close(lines)
}

func failBeforeEngine(w *writer, code string, err error) error {
	w.send(api.WorkerMessage{Type: api.MsgError, Code: code, Message: err.Error()})
	w.finish(api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitError, StopReason: api.StopError, Error: err.Error()})
	return err
}

// checkBookDir bắt buộc cwd trùng thư mục truyện: rules dự án và EffectiveConfigPath
// đọc theo cwd, cwd khác nghĩa là có thể đọc nhầm cấu hình của truyện/user khác.
func checkBookDir(bookDir string, getwd func() (string, error)) (string, error) {
	if !filepath.IsAbs(bookDir) {
		return "", fmt.Errorf("book_dir phải là đường dẫn tuyệt đối")
	}
	want, err := filepath.EvalSymlinks(filepath.Clean(bookDir))
	if err != nil {
		return "", fmt.Errorf("book_dir không truy cập được: %w", err)
	}
	wd, err := getwd()
	if err != nil {
		return "", fmt.Errorf("không đọc được thư mục hiện tại: %w", err)
	}
	got, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return "", fmt.Errorf("không đọc được thư mục hiện tại: %w", err)
	}
	if got != want {
		return "", fmt.Errorf("thư mục hiện tại khác book_dir")
	}
	return want, nil
}

func newRunID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (s *session) loop(ctx context.Context, lines <-chan []byte, scanErr <-chan error) error {
	// advance_mode không thuộc bootstrap.Config mà nằm trong RunMeta: áp trước mọi
	// action. Chỉ ghi khi khác để mỗi lượt spawn không sinh event thừa.
	if snap := s.eng.Snapshot(); snap.AdvanceMode != s.init.AdvanceMode {
		if err := s.eng.SetAdvanceMode(domain.ChapterAdvanceMode(s.init.AdvanceMode)); err != nil {
			return s.fail(err)
		}
	}
	s.sendSnapshot(s.eng.Snapshot())

	pumpDone := make(chan struct{})
	s.pumpDone = pumpDone
	go func() {
		defer close(pumpDone)
		s.pump()
	}()
	// Action phải vào hàng đợi trước khi luồng đọc chạy: lệnh runtime gửi ngay sau
	// init không được chạy trước StartPrepared/Resume.
	s.inFlight.Add(1)
	s.jobs <- job{action: true}
	go s.dispatch(lines, scanErr)
	go s.execute()

	ticker := time.NewTicker(s.d.pollInterval)
	defer ticker.Stop()

	done := s.eng.Done()
	actionOK := false
	doneSeen := false
	jobFinished := false
	idleTicks := 0
	lastSent := time.Now()
	var lastKey string

	for {
		tick := false
		select {
		case <-ctx.Done():
			return s.shutdown()
		case <-s.shutdownCh:
			return s.shutdown()
		case res := <-s.actionCh:
			if res.err != nil {
				return s.fail(res.err)
			}
			if res.nothingToResume {
				return s.stop(s.eng.Snapshot(), api.StopNothingToResume)
			}
			actionOK = true
			s.lastErr.Store("") // lỗi trong lúc khởi động đã được action xử lý
			// Abort chỉ có tác dụng khi engine đang chạy: lệnh abort đến lúc action
			// còn chạy (Start/Resume chưa trả về) được thực thi lại ở đây.
			if s.aborted.Load() {
				s.eng.Abort()
			}
		case _, ok := <-done:
			if !ok {
				done = nil
				continue
			}
			doneSeen = true
		case <-s.jobDone:
			jobFinished = true
		case <-ticker.C:
			tick = true
		}

		if !tick && !doneSeen && !jobFinished {
			continue
		}
		jobFinished = false
		snap := s.eng.Snapshot()
		if snap.HasAdvanceHold {
			s.sawHold = true
		}
		if tick {
			key := fmt.Sprintf("%s|%s|%s|%d|%t", snap.Phase, snap.Flow, snap.RuntimeState, snap.CompletedCount, snap.IsRunning)
			if key != lastKey || time.Since(lastSent) >= s.d.snapshotEvery {
				s.sendSnapshot(snap)
				lastKey, lastSent = key, time.Now()
			}
		}
		if !actionOK || s.inFlight.Load() > 0 {
			continue
		}
		if snap.IsRunning {
			doneSeen, idleTicks = false, 0
			continue
		}
		if tick {
			idleTicks++
		}
		// Done là tín hiệu chính; hai nhịp liền không chạy là lưới an toàn cho
		// lệnh không khởi động engine (vd. can thiệp chỉ trả lời câu hỏi).
		if doneSeen || idleTicks >= 2 {
			return s.stop(snap, stopReason(snap, s.aborted.Load(), s.sawHold && !snap.HasAdvanceHold))
		}
	}
}

// dispatch phân loại lệnh trên luồng đọc: abort/shutdown xử lý ngay, lệnh khác
// vào executor. Không gọi gì có thể chặn lâu ở đây.
func (s *session) dispatch(lines <-chan []byte, scanErr <-chan error) {
	for line := range lines {
		cmd, err := decodeCommand(line)
		if err != nil {
			s.w.send(api.WorkerMessage{Type: api.MsgError, Code: api.ErrCodeProtocol, Message: err.Error()})
			continue
		}
		switch cmd.Type {
		case api.MsgAbort:
			s.aborted.Store(true)
			s.eng.Abort()
			s.w.send(api.WorkerMessage{Type: api.MsgAck, ID: cmd.ID, OK: true})
		case api.MsgShutdown:
			s.w.send(api.WorkerMessage{Type: api.MsgAck, ID: cmd.ID, OK: true})
			s.requestShutdown()
		default:
			s.inFlight.Add(1)
			select {
			case s.jobs <- job{cmd: cmd}:
			default:
				s.inFlight.Add(-1)
				s.w.send(api.WorkerMessage{Type: api.MsgAck, ID: cmd.ID, Code: api.ErrCodeBusy, Error: "worker đang bận, thử lại sau"})
			}
		}
	}
	if err := <-scanErr; err != nil {
		s.w.send(api.WorkerMessage{Type: api.MsgError, Code: api.ErrCodeProtocol, Message: "đọc stdin thất bại: " + err.Error()})
	}
	// stdin EOF: server đã mất hoặc đóng ống — dừng như shutdown.
	s.requestShutdown()
}

func (s *session) requestShutdown() {
	s.shutdownOnce.Do(func() { close(s.shutdownCh) })
}

// execute chạy job tuần tự trên goroutine riêng để luồng đọc không bị chặn.
func (s *session) execute() {
	for j := range s.jobs {
		if s.stopping.Load() {
			// Engine đang đóng: không chạm store nữa, lượt sau sẽ làm lại.
			if !j.action {
				s.w.send(api.WorkerMessage{Type: api.MsgAck, ID: j.cmd.ID, Code: api.ErrCodeBusy, Error: "worker đang dừng"})
			}
			s.inFlight.Add(-1)
			continue
		}
		if j.action {
			nothing, err := s.runAction()
			s.actionCh <- actionResult{nothingToResume: nothing, err: err}
		} else {
			s.runCommand(j.cmd)
		}
		s.inFlight.Add(-1)
		select {
		case s.jobDone <- struct{}{}:
		default:
		}
	}
}

func (s *session) runAction() (bool, error) {
	switch s.init.Action {
	case api.ActionStart:
		prompt, err := startup.PrepareQuick(s.init.Prompt)
		if err != nil {
			return false, err
		}
		if err := s.eng.PrepareUserRules(prompt); err != nil {
			return false, err
		}
		return false, s.eng.StartPrepared(prompt)
	case api.ActionResume:
		label, err := s.eng.Resume()
		if err != nil {
			return false, err
		}
		return label == "", nil
	case api.ActionSteer:
		// Truyện đang dừng: giống TUI khi engine không chạy, đi qua Continue
		// (phán quyết can thiệp rồi chạy tiếp). Gộp can thiệp còn treo từ lượt
		// trước vì Continue ghi đè PendingSteer.
		text := mergeSteer(s.eng.Snapshot().PendingSteer, s.init.Text)
		return false, s.eng.Continue(text)
	case api.ActionAdvance:
		return false, s.eng.AdvanceOneChapter()
	}
	return false, fmt.Errorf("action %q không hợp lệ", s.init.Action)
}

func mergeSteer(pending, text string) string {
	pending, text = strings.TrimSpace(pending), strings.TrimSpace(text)
	if pending == "" || pending == text {
		return text
	}
	return pending + "\n\n" + text
}

func (s *session) runCommand(cmd api.WorkerCommand) {
	var err error
	switch cmd.Type {
	case api.MsgSteer:
		err = s.eng.Steer(cmd.Text)
		// Phán quyết reopen/dispatch có thể khởi động lại engine: giữ ý người dùng đã bấm dừng.
		if s.aborted.Load() {
			s.eng.Abort()
		}
	case api.MsgSetAdvanceMode:
		err = s.eng.SetAdvanceMode(domain.ChapterAdvanceMode(cmd.Mode))
	default:
		err = fmt.Errorf("lệnh %q không hợp lệ", cmd.Type)
	}
	ack := api.WorkerMessage{Type: api.MsgAck, ID: cmd.ID, OK: err == nil}
	if err != nil {
		ack.Error = err.Error()
	}
	s.w.send(ack)
}

// pump chuyển event và stream của engine ra stdout cho tới khi engine đóng kênh.
func (s *session) pump() {
	events, stream := s.eng.Events(), s.eng.Stream()
	for events != nil || stream != nil {
		select {
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if strings.TrimSpace(ev.Summary) == "" {
				continue
			}
			// Engine tự dừng (bế tắc, lỗi phán quyết…) phát SYSTEM mức warn ngay trước
			// Done: giữ lại làm lời giải thích cho exit.error.
			if ev.Category == "ERROR" || ev.Level == "error" || (ev.Category == "SYSTEM" && ev.Level == "warn") {
				s.lastErr.Store(ev.Summary)
			}
			e := s.mapper.mapEvent(ev)
			s.w.send(api.WorkerMessage{Type: api.MsgEvent, Event: &e})
		case delta, ok := <-stream:
			if !ok {
				stream = nil
				continue
			}
			switch {
			case delta == host.StreamClearSentinel:
				s.w.send(api.WorkerMessage{Type: api.MsgStream, Clear: true})
			case delta != "":
				s.w.send(api.WorkerMessage{Type: api.MsgStream, Delta: delta})
			}
		}
	}
}

func (s *session) sendSnapshot(snap host.UISnapshot) {
	dto := mapSnapshot(snap)
	s.w.send(api.WorkerMessage{Type: api.MsgSnapshot, Snapshot: &dto})
}

// stop: engine đã dừng → snapshot cuối → Close (xả hết event còn trong kênh) → exit.
func (s *session) stop(snap host.UISnapshot, reason string) error {
	s.sendSnapshot(snap)
	exit := api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitStopped, StopReason: reason}
	s.closeEngine()
	if reason == api.StopError {
		exit.Error, _ = s.lastErr.Load().(string)
		if exit.Error == "" {
			exit.Error = "Engine dừng không rõ lý do"
		}
	}
	s.w.finish(exit)
	return nil
}

// fail: action hoặc khởi tạo lỗi. Hết ngân sách được báo riêng để server cho nâng.
func (s *session) fail(err error) error {
	snap := s.eng.Snapshot()
	s.sendSnapshot(snap)
	s.closeEngine()
	if budgetExhausted(snap) {
		s.w.send(api.WorkerMessage{Type: api.MsgError, Code: api.ErrCodeBudgetExhausted, Message: err.Error()})
		s.w.finish(api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitStopped, StopReason: api.StopBudget})
		return nil
	}
	s.w.send(api.WorkerMessage{Type: api.MsgError, Code: api.ErrCodeEngine, Message: err.Error()})
	s.w.finish(api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitError, StopReason: api.StopError, Error: err.Error()})
	return err
}

// shutdown: server yêu cầu dừng (hoặc SIGTERM / stdin EOF). Close huỷ runCtx nên
// cắt cả lời gọi LLM đang treo; không chờ quá closeGrace.
func (s *session) shutdown() error {
	s.sendSnapshot(s.eng.Snapshot())
	s.closeEngine()
	exit := api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitShutdown}
	if s.aborted.Load() {
		exit.StopReason = api.StopUser
	}
	s.w.finish(exit)
	return nil
}

// closeEngine chạy Close ở goroutine riêng rồi chờ pump xả xong kênh event/stream,
// tối đa closeGrace. Quá hạn thì vẫn trả về: process sắp thoát, OS nhả khoá thư mục.
func (s *session) closeEngine() {
	s.stopping.Store(true)
	closed := make(chan struct{})
	go func() {
		s.eng.Close()
		close(closed)
	}()
	timeout := time.After(s.d.closeGrace)
	select {
	case <-closed:
	case <-timeout:
		return
	}
	// Close không chờ lời gọi đồng bộ đang chạy trên executor (StartPrepared/Resume
	// sau khi arbiter bị huỷ vẫn ghi store): chờ nó xong trước khi báo exit, để
	// server chỉ spawn lượt kế tiếp khi không còn ai ghi vào thư mục truyện.
	for s.inFlight.Load() > 0 {
		select {
		case <-s.jobDone:
		case <-time.After(20 * time.Millisecond):
		case <-timeout:
			return
		}
	}
	if s.pumpDone == nil {
		return
	}
	select {
	case <-s.pumpDone:
	case <-timeout:
	}
}
