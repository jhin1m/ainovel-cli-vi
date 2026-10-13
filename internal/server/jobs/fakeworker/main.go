// Command fakeworker giả lập process `ainovel-cli worker` để test supervisor của
// server và e2e trên trình duyệt mà không cần LLM. Nó nói đúng giao thức JSON Lines
// (api.WorkerMessage), dùng package store thật nên reader/export của server chạy
// được trên kết quả.
//
// Cách gọi: fakeworker worker --version V (cwd = thư mục truyện).
//
// Marker trong init.prompt:
//
//	[fake:fast]     mỗi chương ~50ms
//	[fake:slow]     mỗi chương ~6s
//	[fake:crash]    in vài dòng stderr rồi thoát mã 3, không có message exit
//	[fake:hang]     bỏ qua shutdown, SIGTERM, EOF stdin (chỉ SIGKILL mới dừng)
//	[fake:lockbusy] lần chạy đầu báo lock_busy rồi thoát 1, các lần sau chạy bình thường
//	[fake:error]    viết một chương rồi dừng với stop_reason error
package main

import (
	"bufio"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/store"
)

const (
	totalChapters  = 3
	chapterCostUSD = 0.40
	streamChunks   = 12
	costFile       = "meta/fake_cost.json"
	lockBusyFile   = ".fake_lockbusy_count"
)

var chapterTitles = [totalChapters]string{
	"Khởi đầu bên bờ sông",
	"Bức thư không người gửi",
	"Ngã rẽ cuối cùng",
}

// encoder là lối ghi stdout duy nhất; mỗi message một dòng JSON, có khoá mutex.
type encoder struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func newEncoder(w io.Writer) *encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &encoder{enc: enc}
}

func (e *encoder) send(msg api.WorkerMessage) {
	e.mu.Lock()
	defer e.mu.Unlock()
	_ = e.enc.Encode(msg)
}

// runner giữ trạng thái một lượt chạy.
type runner struct {
	out     *encoder
	st      *store.Store
	dir     string
	init    api.WorkerInit
	version string

	runID string
	evSeq atomic.Int64
	fast  bool
	slow  bool
	cost  float64

	ctx    context.Context // huỷ khi shutdown
	cancel context.CancelFunc
	abort  atomic.Bool
	mu     sync.Mutex // bảo vệ mode
	mode   string
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "worker" {
		fmt.Fprintln(os.Stderr, "usage: fakeworker worker [--version V]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("worker", flag.ExitOnError)
	version := fs.String("version", "fake", "phiên bản báo trong hello")
	_ = fs.Parse(os.Args[2:])

	// Cài handler tín hiệu trước hello để không có cửa sổ bị SIGTERM giết sớm;
	// marker hang sẽ nuốt tín hiệu, còn lại coi như shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var hanging atomic.Bool
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		for range sigs {
			if !hanging.Load() {
				cancel()
			}
		}
	}()

	out := newEncoder(os.Stdout)
	out.send(api.WorkerMessage{Type: api.MsgHello, Protocol: api.WorkerProtocol, Version: *version})

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var init api.WorkerInit
	if !scanner.Scan() {
		fail(out, api.ErrCodeProtocol, "stdin đóng trước khi nhận init")
	}
	if err := json.Unmarshal(scanner.Bytes(), &init); err != nil || init.Type != api.MsgInit {
		fail(out, api.ErrCodeProtocol, "message đầu tiên phải là init hợp lệ")
	}
	cwd, err := resolve(".")
	if err != nil {
		fail(out, api.ErrCodeConfigInvalid, "không xác định được thư mục làm việc")
	}
	bookDir, err := resolve(init.BookDir)
	if init.Protocol != api.WorkerProtocol || err != nil || init.BookDir == "" || bookDir != cwd {
		fail(out, api.ErrCodeConfigInvalid, "init không hợp lệ: protocol hoặc book_dir lệch")
	}

	r := &runner{
		out: out, dir: cwd, init: init, version: *version,
		runID: randomID(), ctx: ctx, cancel: cancel,
		fast: strings.Contains(init.Prompt, "[fake:fast]"),
		slow: strings.Contains(init.Prompt, "[fake:slow]"),
		mode: init.AdvanceMode,
	}
	if r.mode == "" {
		r.mode = api.AdvanceReview
	}

	switch {
	case strings.Contains(init.Prompt, "[fake:crash]"):
		fmt.Fprintln(os.Stderr, "fakeworker: giả lập sự cố")
		fmt.Fprintln(os.Stderr, "panic: fake crash (không phải panic thật)")
		fmt.Fprintln(os.Stderr, "goroutine 1 [running]:")
		os.Exit(3)
	case strings.Contains(init.Prompt, "[fake:hang]"):
		hanging.Store(true)
		hang()
	case strings.Contains(init.Prompt, "[fake:lockbusy]"):
		if r.lockBusy() {
			out.send(api.WorkerMessage{Type: api.MsgError, Code: api.ErrCodeLockBusy, Message: "thư mục truyện đang bị khoá (giả lập)"})
			out.send(api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitError, StopReason: api.StopError, Error: "lock_busy"})
			os.Exit(1)
		}
	}

	r.st = store.NewStore(cwd)
	if err := r.st.Init(); err != nil {
		out.send(api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitError, StopReason: api.StopError, Error: err.Error()})
		os.Exit(1)
	}
	r.cost = r.loadCost()

	go r.readCommands(scanner)
	out.send(r.exitFor(r.run()))
}

// fail báo lỗi khởi tạo rồi thoát mã 1.
func fail(out *encoder, code, msg string) {
	out.send(api.WorkerMessage{Type: api.MsgError, Code: code, Message: msg})
	out.send(api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitError, StopReason: api.StopError, Error: msg})
	os.Exit(1)
}

// hang chặn vô hạn; chỉ SIGKILL kết thúc được process (tín hiệu mềm đã bị nuốt).
func hang() {
	for {
		time.Sleep(time.Hour)
	}
}

func resolve(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func randomID() string {
	b := make([]byte, 4)
	if _, err := crand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}

// lockBusy ghi nhận một lần thử và trả true nếu đây là lần đầu.
func (r *runner) lockBusy() bool {
	path := filepath.Join(r.dir, lockBusyFile)
	if data, err := os.ReadFile(path); err == nil {
		if n, _ := strconv.Atoi(strings.TrimSpace(string(data))); n >= 1 {
			return false
		}
	}
	_ = os.WriteFile(path, []byte("1"), 0o644)
	return true
}

// ── lệnh runtime ──

func (r *runner) readCommands(sc *bufio.Scanner) {
	for sc.Scan() {
		var cmd api.WorkerCommand
		if err := json.Unmarshal(sc.Bytes(), &cmd); err != nil {
			continue
		}
		switch cmd.Type {
		case api.MsgAbort:
			r.abort.Store(true)
			r.ack(cmd.ID, true, "")
		case api.MsgShutdown:
			r.ack(cmd.ID, true, "")
			r.cancel()
		case api.MsgSteer:
			r.event("USER", "", "[Tiếp tục] "+cmd.Text, "", false)
			r.ack(cmd.ID, true, "")
		case api.MsgSetAdvanceMode:
			if cmd.Mode != api.AdvanceAuto && cmd.Mode != api.AdvanceReview {
				r.ack(cmd.ID, false, "mode không hợp lệ")
				continue
			}
			r.mu.Lock()
			r.mode = cmd.Mode
			r.mu.Unlock()
			r.ack(cmd.ID, true, "")
		default:
			r.ack(cmd.ID, false, "lệnh không hỗ trợ: "+cmd.Type)
		}
	}
	// EOF stdin: như shutdown.
	r.cancel()
}

func (r *runner) ack(id string, ok bool, msg string) {
	r.out.send(api.WorkerMessage{Type: api.MsgAck, ID: id, OK: ok, Message: msg})
}

func (r *runner) advanceMode() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mode
}

// ── event / snapshot ──

func (r *runner) nextEventID() string {
	return fmt.Sprintf("%s:n%d", r.runID, r.evSeq.Add(1))
}

// event phát một event đã kết thúc (hoặc bắt đầu nếu !finished) và trả id.
func (r *runner) event(category, agent, summary, detail string, finished bool) string {
	id := r.nextEventID()
	r.emitEvent(api.Event{ID: id, Category: category, Agent: agent, Summary: summary, Detail: detail, Level: "info", Finished: finished || !isCall(category)})
	return id
}

func isCall(category string) bool { return category == "MODEL" || category == "TOOL" }

func (r *runner) emitEvent(ev api.Event) {
	ev.Time = time.Now()
	r.out.send(api.WorkerMessage{Type: api.MsgEvent, Event: &ev})
}

// call phát cặp event start/finish cùng id cho lời gọi MODEL/TOOL.
func (r *runner) call(category, agent, summary string, work func() bool) bool {
	id := r.nextEventID()
	start := time.Now()
	r.emitEvent(api.Event{ID: id, Category: category, Agent: agent, Summary: summary, Level: "info"})
	ok := work()
	r.emitEvent(api.Event{ID: id, Category: category, Agent: agent, Summary: summary, Level: "info",
		DurationMS: time.Since(start).Milliseconds(), Finished: true, Failed: !ok})
	return ok
}

func (r *runner) snapshot(runtimeState string, running bool) {
	snap := api.BookSnapshot{
		Provider: r.init.LLM.Provider, ModelName: r.init.LLM.Model,
		Phase: "init", Flow: "writing", RuntimeState: runtimeState, IsRunning: running,
		StatusLabel: runtimeState, TotalChapters: totalChapters,
		AdvanceMode: r.advanceMode(), TotalCostUSD: r.cost, BudgetLimitUSD: r.init.BudgetUSD,
		Agents: []api.AgentSnapshot{},
	}
	if book, err := r.st.Book.Load(); err == nil && book != nil {
		snap.BookTitle = book.Title
	}
	if p, err := r.st.Progress.Load(); err == nil && p != nil {
		snap.Phase = string(p.Phase)
		snap.CompletedCount = len(p.CompletedChapters)
		snap.TotalWordCount = p.TotalWordCount
		snap.CurrentChapter = p.CurrentChapter
		if p.TotalChapters > 0 {
			snap.TotalChapters = p.TotalChapters
		}
		if p.Flow != "" {
			snap.Flow = string(p.Flow)
		}
	}
	if running {
		snap.Agents = []api.AgentSnapshot{{Name: "writer", State: "running", Summary: "Đang viết chương", Tool: "draft_chapter"}}
	}
	r.out.send(api.WorkerMessage{Type: api.MsgSnapshot, Snapshot: &snap})
}

// ── chi phí giả ──

func (r *runner) loadCost() float64 {
	data, err := os.ReadFile(filepath.Join(r.dir, costFile))
	if err != nil {
		return 0
	}
	var c struct {
		Total float64 `json:"total_cost_usd"`
	}
	if json.Unmarshal(data, &c) != nil {
		return 0
	}
	return c.Total
}

func (r *runner) saveCost() error {
	data, _ := json.Marshal(map[string]float64{"total_cost_usd": r.cost})
	return os.WriteFile(filepath.Join(r.dir, costFile), data, 0o644)
}

// ── luồng chính ──

// outcome là kết quả một lượt chạy để dựng message exit.
type outcome struct {
	shutdown bool
	stop     string
	errText  string
}

func (r *runner) exitFor(o outcome) api.WorkerMessage {
	if o.shutdown {
		return api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitShutdown}
	}
	return api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitStopped, StopReason: o.stop, Error: o.errText}
}

func (r *runner) fail(err error) outcome {
	return outcome{stop: api.StopError, errText: err.Error()}
}

func (r *runner) run() outcome {
	switch r.init.Action {
	case api.ActionStart:
		if err := r.createStory(); err != nil {
			return r.fail(err)
		}
	case api.ActionResume, api.ActionAdvance, api.ActionSteer:
	default:
		return r.fail(fmt.Errorf("action không hỗ trợ: %q", r.init.Action))
	}

	progress, err := r.st.Progress.Load()
	if err != nil {
		return r.fail(err)
	}
	if progress == nil {
		// Worker thật: resume trên thư mục chưa có tiến độ báo nothing_to_resume.
		if r.init.Action == api.ActionResume {
			return outcome{stop: api.StopNothingToResume}
		}
		return r.fail(errors.New("truyện chưa được khởi tạo"))
	}
	done := progress.Phase == domain.PhaseComplete

	switch r.init.Action {
	case api.ActionResume:
		if done {
			r.snapshot("idle", false)
			return outcome{stop: api.StopNothingToResume}
		}
	case api.ActionAdvance:
		if done {
			r.snapshot("idle", false)
			return outcome{stop: api.StopComplete}
		}
	case api.ActionSteer:
		r.event("USER", "", "[Tiếp tục] "+r.init.Text, "", true)
		if done {
			r.snapshot("idle", false)
			return outcome{stop: api.StopComplete}
		}
	}

	return r.writeLoop(r.init.Action == api.ActionAdvance)
}

// stopped trả outcome nếu có yêu cầu dừng (shutdown ưu tiên hơn abort).
func (r *runner) stopped() (outcome, bool) {
	if r.ctx.Err() != nil {
		return outcome{shutdown: true}, true
	}
	if r.abort.Load() {
		return outcome{stop: api.StopUser}, true
	}
	return outcome{}, false
}

func (r *runner) writeLoop(oneShot bool) outcome {
	for {
		if o, ok := r.stopped(); ok {
			r.snapshot("stopped", false)
			return o
		}
		progress, err := r.st.Progress.Load()
		if err != nil || progress == nil {
			return r.fail(fmt.Errorf("đọc progress: %v", err))
		}
		next := len(progress.CompletedChapters) + 1
		if next > totalChapters {
			return r.finishBook()
		}
		// Budget 0 = gói thuê bao: không áp ngân sách (khớp budgetExhausted của worker thật).
		if r.init.BudgetUSD > 0 && r.cost+chapterCostUSD > r.init.BudgetUSD+1e-9 {
			r.snapshot("stopped", false)
			return outcome{stop: api.StopBudget}
		}

		if completed, o := r.writeChapter(next); !completed {
			r.snapshot("stopped", false)
			return o
		}
		if strings.Contains(r.init.Prompt, "[fake:error]") {
			r.snapshot("stopped", false)
			return outcome{stop: api.StopError, errText: "lỗi giả lập"}
		}
		if next >= totalChapters {
			return r.finishBook()
		}
		if oneShot || r.advanceMode() == api.AdvanceReview {
			r.snapshot("waiting", false)
			return outcome{stop: api.StopReviewWait}
		}
	}
}

func (r *runner) finishBook() outcome {
	if err := r.st.Progress.MarkComplete(); err != nil {
		return r.fail(err)
	}
	r.event("SYSTEM", "", "Truyện đã hoàn thành", "", true)
	r.snapshot("idle", false)
	return outcome{stop: api.StopComplete}
}

// createStory tạo metadata, dàn ý và progress nếu chưa có.
func (r *runner) createStory() error {
	if p, err := r.st.Progress.Load(); err != nil {
		return err
	} else if p != nil {
		return nil // đã có truyện: start lại thì coi như tiếp tục
	}
	var err error
	ok := r.call("MODEL", "architect", "Lập dàn ý", func() bool {
		if !r.sleep(r.step(300 * time.Millisecond)) {
			return false
		}
		title := storyTitle(r.init.Prompt)
		entries := make([]domain.OutlineEntry, totalChapters)
		for i := range entries {
			entries[i] = domain.OutlineEntry{
				Chapter: i + 1, Title: chapterTitles[i],
				CoreEvent: "Sự kiện chính của chương " + strconv.Itoa(i+1),
				Hook:      "Một bí mật vừa hé lộ", Scenes: []string{"Mở đầu", "Cao trào"},
			}
		}
		synopsis := "Truyện giả lập do fakeworker sinh ra để kiểm thử. Yêu cầu gốc: " + cleanPrompt(r.init.Prompt)
		if err = r.st.Book.Save(domain.BookMetadata{Title: title, Synopsis: synopsis}); err != nil {
			return false
		}
		if err = r.st.Outline.SavePremise("# Tiền đề\n\n" + synopsis); err != nil {
			return false
		}
		if err = r.st.Outline.SaveOutline(entries); err != nil {
			return false
		}
		if err = r.st.Progress.Init(totalChapters); err != nil {
			return false
		}
		err = r.st.Progress.UpdatePhase(domain.PhaseWriting)
		return err == nil
	})
	if err != nil {
		return err
	}
	if !ok {
		if o, stop := r.stopped(); stop && o.shutdown {
			return errors.New("bị dừng khi lập dàn ý")
		}
		return errors.New("bị huỷ khi lập dàn ý")
	}
	return nil
}

// writeChapter stream rồi lưu chương n. Trả false cùng outcome nếu bị dừng/lỗi
// trước khi lưu; chương dở dang không được ghi.
func (r *runner) writeChapter(n int) (bool, outcome) {
	r.out.send(api.WorkerMessage{Type: api.MsgStream, Clear: true})
	r.snapshot("running", true)

	paras := chapterText(n)
	var sb strings.Builder
	interrupted := false
	r.call("MODEL", "writer", fmt.Sprintf("Viết chương %d", n), func() bool {
		for i := 0; i < streamChunks; i++ {
			if !r.sleep(r.step(100 * time.Millisecond)) {
				interrupted = true
				return false
			}
			chunk := paras[i%len(paras)] + " "
			if i%4 == 3 {
				chunk += "\n\n"
			}
			sb.WriteString(chunk)
			r.out.send(api.WorkerMessage{Type: api.MsgStream, Delta: chunk})
		}
		return true
	})
	if interrupted {
		o, _ := r.stopped()
		return false, o
	}

	text := strings.TrimSpace(sb.String()) + "\n"
	var saveErr error
	r.call("TOOL", "writer", fmt.Sprintf("commit_chapter %d", n), func() bool {
		saveErr = r.persist(n, text)
		return saveErr == nil
	})
	if saveErr != nil {
		return false, r.fail(saveErr)
	}
	if r.init.BudgetUSD > 0 { // thuê bao: worker thật báo chi phí USD bằng 0
		r.cost += chapterCostUSD
	}
	if err := r.saveCost(); err != nil {
		return false, r.fail(err)
	}
	r.snapshot("running", true)
	return true, outcome{}
}

func (r *runner) persist(n int, text string) error {
	if err := r.st.Progress.StartChapter(n); err != nil {
		return err
	}
	if err := r.st.Drafts.SaveFinalChapter(n, text); err != nil {
		return err
	}
	title := chapterTitles[n-1]
	if err := r.st.Summaries.SaveSummary(domain.ChapterSummary{
		Chapter: n, Title: title, Summary: "Tóm tắt giả lập của chương " + strconv.Itoa(n),
		Characters: []string{"Minh", "Lan"}, KeyEvents: []string{"Sự kiện " + strconv.Itoa(n)},
	}); err != nil {
		return err
	}
	return r.st.Progress.MarkChapterComplete(n, domain.WordCount(text), "", "")
}

// step co (nhanh) hoặc giãn (chậm) thời gian chờ.
func (r *runner) step(d time.Duration) time.Duration {
	switch {
	case r.fast:
		return d / 25
	case r.slow:
		return d * 5
	}
	return d
}

// sleep ngủ d, trả false nếu bị shutdown hoặc abort cắt ngang.
func (r *runner) sleep(d time.Duration) bool {
	if r.abort.Load() || r.ctx.Err() != nil {
		return false
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-r.ctx.Done():
		return false
	case <-t.C:
		return !r.abort.Load()
	}
}

// ── nội dung giả ──

func cleanPrompt(prompt string) string {
	for _, m := range []string{"[fake:fast]", "[fake:slow]", "[fake:crash]", "[fake:hang]", "[fake:lockbusy]", "[fake:error]"} {
		prompt = strings.ReplaceAll(prompt, m, "")
	}
	return strings.Join(strings.Fields(prompt), " ")
}

func storyTitle(prompt string) string {
	words := strings.Fields(cleanPrompt(prompt))
	if len(words) > 5 {
		words = words[:5]
	}
	if len(words) == 0 {
		return "Truyện thử"
	}
	return "Truyện thử: " + strings.Join(words, " ")
}

func chapterText(n int) []string {
	return []string{
		fmt.Sprintf("Chương %d bắt đầu khi trời vừa sáng, sương còn đọng trên mặt cỏ.", n),
		"Minh bước chậm rãi trên con đường đất, trong đầu vẫn vang lên lời dặn của người thầy cũ.",
		"Lan đứng chờ ở đầu cầu, đôi mắt đỏ hoe nhưng giọng nói vẫn bình tĩnh đến lạ.",
		"Họ trò chuyện rất lâu về những điều chưa kịp nói, về con sông và về những mùa đã qua.",
		"Gió thổi qua rặng tre, mang theo mùi khói bếp từ xóm nhỏ phía bên kia.",
		"Mỗi lời nói ra đều nặng như đá, vậy mà không ai muốn là người im lặng trước.",
	}
}
