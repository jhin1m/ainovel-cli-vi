package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/assets"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

const testAPIKey = "sk-test-SECRET-0123456789abcdef"

// harness chạy run() trên goroutine riêng, nối stdin/stdout qua pipe.
type harness struct {
	t       *testing.T
	stdin   *io.PipeWriter
	msgs    chan api.WorkerMessage
	result  chan error
	cancel  context.CancelFunc
	rawMu   sync.Mutex
	raw     strings.Builder
	eng     *fakeEngine
	bookDir string
}

func testDeps(t *testing.T, eng *fakeEngine, bookDir string) deps {
	t.Helper()
	return deps{
		version:       "test",
		newEngine:     func(bootstrap.Config, assets.Bundle) (engine, error) { return eng, nil },
		registry:      models.NewModelRegistry(),
		getwd:         func() (string, error) { return bookDir, nil },
		prepare:       func(bootstrap.Config) (assets.Bundle, error) { return assets.Bundle{}, nil },
		pollInterval:  20 * time.Millisecond,
		snapshotEvery: 100 * time.Millisecond,
		closeGrace:    2 * time.Second,
	}
}

// startHarness chạy worker trên một thư mục truyện tạm; mutate (nếu có) chỉnh deps.
func startHarness(t *testing.T, eng *fakeEngine, mutate func(*deps)) *harness {
	t.Helper()
	bookDir := t.TempDir()
	d := testDeps(t, eng, bookDir)
	if mutate != nil {
		mutate(&d)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	h := &harness{t: t, stdin: inW, msgs: make(chan api.WorkerMessage, 1000), result: make(chan error, 1),
		cancel: cancel, eng: eng, bookDir: bookDir}
	go func() {
		err := run(ctx, inR, outW, d)
		outW.Close()
		h.result <- err
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			h.rawMu.Lock()
			h.raw.Write(sc.Bytes())
			h.raw.WriteByte('\n')
			h.rawMu.Unlock()
			var m api.WorkerMessage
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("stdout không phải JSON: %q", sc.Text())
				continue
			}
			h.msgs <- m
		}
		close(h.msgs)
	}()
	t.Cleanup(func() {
		cancel()
		inW.Close()
		h.rawMu.Lock()
		raw := h.raw.String()
		h.rawMu.Unlock()
		if strings.Contains(raw, testAPIKey) {
			t.Errorf("API key xuất hiện trên stdout")
		}
	})
	return h
}

func (h *harness) send(v any) {
	h.t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.stdin.Write(append(b, '\n')); err != nil {
		h.t.Fatalf("ghi stdin: %v", err)
	}
}

// expect đọc tới khi gặp message type mong muốn (bỏ qua message khác).
func (h *harness) expect(typ string, timeout time.Duration) api.WorkerMessage {
	h.t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-h.msgs:
			if !ok {
				h.t.Fatalf("stdout đóng trước khi nhận %q", typ)
			}
			if m.Type == typ {
				return m
			}
		case <-deadline:
			h.t.Fatalf("hết giờ chờ message %q", typ)
		}
	}
}

func (h *harness) wait(timeout time.Duration) error {
	h.t.Helper()
	select {
	case err := <-h.result:
		return err
	case <-time.After(timeout):
		h.t.Fatalf("run không trả về sau %v", timeout)
		return nil
	}
}

func testInit(bookDir, action string) api.WorkerInit {
	return api.WorkerInit{
		Type: api.MsgInit, Protocol: api.WorkerProtocol, Action: action,
		Prompt: "Truyện tu tiên về một thư sinh", Text: "thêm nhân vật phản diện",
		BookDir: bookDir, AdvanceMode: api.AdvanceReview,
		LLM:       api.WorkerLLM{Provider: "openrouter", Model: "test/fake-model", APIKey: testAPIKey},
		Pricing:   &api.ModelPricing{Provider: "test", ID: "fake-model", InputCostPer1M: 1, OutputCostPer1M: 2},
		Language:  "vi",
		BudgetUSD: 5,
	}
}

func waitCall(t *testing.T, eng *fakeEngine, name string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if slices.Contains(eng.Calls(), name) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("không thấy lời gọi %s; calls=%v", name, eng.Calls())
}

func TestRunStartUntilReviewWait(t *testing.T) {
	eng := newFakeEngine()
	h := startHarness(t, eng, nil)
	hello := h.expect(api.MsgHello, time.Second)
	if hello.Protocol != api.WorkerProtocol || hello.Version != "test" {
		t.Fatalf("hello sai: %+v", hello)
	}
	h.send(testInit(h.bookDir, api.ActionStart))
	waitCall(t, eng, "StartPrepared")

	eng.emit(host.Event{ID: "e1", Time: time.Now(), Category: "TOOL", Agent: "writer", Summary: "draft_chapter"})
	eng.emit(host.Event{ID: "e1", Time: time.Now(), FinishedAt: time.Now(), Category: "TOOL", Agent: "writer", Summary: "draft_chapter", Duration: 1500 * time.Millisecond})
	eng.emit(host.Event{Time: time.Now(), Category: "SYSTEM", Summary: "Bắt đầu sáng tác"})
	eng.emitDelta("Chương một")
	eng.emitDelta(host.StreamClearSentinel)

	// event và stream đi hai kênh khác nhau nên có thể đan xen: gom đủ rồi kiểm.
	var evs []*api.Event
	var streams []api.WorkerMessage
	deadline := time.After(2 * time.Second)
	for len(evs) < 3 || len(streams) < 2 {
		select {
		case m := <-h.msgs:
			switch m.Type {
			case api.MsgEvent:
				evs = append(evs, m.Event)
			case api.MsgStream:
				streams = append(streams, m)
			}
		case <-deadline:
			t.Fatalf("thiếu message: %d event, %d stream", len(evs), len(streams))
		}
	}
	first, second, third := evs[0], evs[1], evs[2]
	if first.ID != second.ID || first.Finished || !second.Finished || second.DurationMS != 1500 {
		t.Fatalf("start/finish phải cùng id: %+v / %+v", first, second)
	}
	if third.ID == "" || third.ID == first.ID || !strings.Contains(first.ID, ":e1") {
		t.Fatalf("id event sai: %q %q", first.ID, third.ID)
	}
	if streams[0].Delta != "Chương một" || !streams[1].Clear {
		t.Fatalf("stream sai: %+v", streams)
	}

	eng.finish(func(s *host.UISnapshot) { s.Phase = "writing"; s.CompletedCount = 1 })
	exit := h.expect(api.MsgExit, 2*time.Second)
	if exit.Reason != api.ExitStopped || exit.StopReason != api.StopReviewWait {
		t.Fatalf("exit sai: %+v", exit)
	}
	if err := h.wait(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	calls := eng.Calls()
	want := []string{"SetAdvanceMode:review", "Resume", "PrepareUserRules", "StartPrepared"}
	if !slices.Equal(calls[:4], want) {
		t.Fatalf("thứ tự lời gọi sai: %v", calls)
	}
	if !slices.Contains(calls, "Close") {
		t.Fatalf("thiếu Close: %v", calls)
	}
}

func TestRunShutdownWhileSteerHangs(t *testing.T) {
	eng := newFakeEngine()
	eng.steerBlock = make(chan struct{}) // không bao giờ mở: giả lập LLM treo
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionStart))
	waitCall(t, eng, "StartPrepared")
	h.send(api.WorkerCommand{Type: api.MsgSteer, ID: "c1", Text: "đổi giọng văn"})
	waitCall(t, eng, "Steer")

	began := time.Now()
	h.send(api.WorkerCommand{Type: api.MsgShutdown, ID: "c2"})
	exit := h.expect(api.MsgExit, time.Second)
	if exit.Reason != api.ExitShutdown {
		t.Fatalf("exit sai: %+v", exit)
	}
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Fatalf("shutdown mất %v", elapsed)
	}
}

func TestRunAbortDuringStart(t *testing.T) {
	eng := newFakeEngine()
	eng.startBlock = make(chan struct{})
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionStart))
	waitCall(t, eng, "StartPrepared")

	h.send(api.WorkerCommand{Type: api.MsgAbort, ID: "a1"})
	if ack := h.expect(api.MsgAck, time.Second); ack.ID != "a1" || !ack.OK {
		t.Fatalf("ack sai: %+v", ack)
	}
	close(eng.startBlock) // StartPrepared trả về, engine bắt đầu chạy
	exit := h.expect(api.MsgExit, 2*time.Second)
	if exit.StopReason != api.StopUser {
		t.Fatalf("abort trong lúc start phải dừng với user: %+v", exit)
	}
	if n := strings.Count(strings.Join(eng.Calls(), ","), "Abort"); n < 2 {
		t.Fatalf("Abort phải được gọi lại sau StartPrepared, calls=%v", eng.Calls())
	}
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestRunStdinEOFClosesEngine(t *testing.T) {
	eng := newFakeEngine()
	eng.resumeLabel = "Khôi phục"
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionResume))
	waitCall(t, eng, "Resume")
	h.stdin.Close()
	if exit := h.expect(api.MsgExit, time.Second); exit.Reason != api.ExitShutdown {
		t.Fatalf("exit sai: %+v", exit)
	}
	select {
	case <-eng.closeCalled:
	case <-time.After(time.Second):
		t.Fatal("EOF stdin phải gọi Close")
	}
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestRunSteerActionMergesPendingAndContinues(t *testing.T) {
	for _, phase := range []string{"writing", "complete"} {
		t.Run(phase, func(t *testing.T) {
			eng := newFakeEngine()
			eng.snap.Phase = phase
			eng.snap.PendingSteer = "can thiệp cũ"
			h := startHarness(t, eng, nil)
			h.send(testInit(h.bookDir, api.ActionSteer))
			waitCall(t, eng, "Continue")
			eng.mu.Lock()
			got := eng.continueText
			eng.mu.Unlock()
			if got != "can thiệp cũ\n\nthêm nhân vật phản diện" {
				t.Fatalf("Continue nhận %q", got)
			}
			if slices.Contains(eng.Calls(), "Resume") || slices.Contains(eng.Calls(), "Steer") {
				t.Fatalf("action steer không được đi nhánh Resume/Steer: %v", eng.Calls())
			}
			eng.finish(nil)
			h.expect(api.MsgExit, 2*time.Second)
			if err := h.wait(time.Second); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRunStopsWhenActionDoesNotStartEngine(t *testing.T) {
	eng := newFakeEngine()
	eng.continueStarts = false
	eng.snap.Phase = "complete"
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionSteer))
	exit := h.expect(api.MsgExit, 2*time.Second)
	if exit.StopReason != api.StopComplete {
		t.Fatalf("exit sai: %+v", exit)
	}
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestRunResumeNothingToResume(t *testing.T) {
	eng := newFakeEngine()
	eng.resumeLabel = ""
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionResume))
	if exit := h.expect(api.MsgExit, time.Second); exit.StopReason != api.StopNothingToResume {
		t.Fatalf("exit sai: %+v", exit)
	}
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestRunStartOverExistingProgressResumes(t *testing.T) {
	eng := newFakeEngine()
	eng.resumeLabel = "Khôi phục"
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionStart))
	waitCall(t, eng, "Resume")
	eng.finish(func(s *host.UISnapshot) { s.Phase = "writing"; s.CompletedCount = 2 })
	h.expect(api.MsgExit, 2*time.Second)
	_ = h.wait(time.Second)
	for _, c := range eng.Calls() {
		if c == "StartPrepared" || c == "PrepareUserRules" {
			t.Fatalf("thư mục đã có tiến độ không được tạo lại truyện; calls=%v", eng.Calls())
		}
	}
}

func TestRunAdvance(t *testing.T) {
	eng := newFakeEngine()
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionAdvance))
	waitCall(t, eng, "AdvanceOneChapter")
	eng.finish(func(s *host.UISnapshot) { s.Phase = "writing"; s.CompletedCount = 2 })
	exit := h.expect(api.MsgExit, 2*time.Second)
	if exit.StopReason != api.StopReviewWait {
		t.Fatalf("exit sai: %+v", exit)
	}
	_ = h.wait(time.Second)
}

func TestRunBudgetRefusedAtStart(t *testing.T) {
	eng := newFakeEngine()
	eng.startErr = errors.New("Truyện này đã tiêu $5.00, chạm giới hạn ngân sách $5.00")
	eng.snap.TotalCostUSD = 5
	eng.snap.BudgetLimitUSD = 5
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionStart))
	if m := h.expect(api.MsgError, time.Second); m.Code != api.ErrCodeBudgetExhausted {
		t.Fatalf("error sai: %+v", m)
	}
	if exit := h.expect(api.MsgExit, time.Second); exit.StopReason != api.StopBudget {
		t.Fatalf("exit sai: %+v", exit)
	}
	if err := h.wait(time.Second); err != nil {
		t.Fatalf("hết ngân sách không phải lỗi process: %v", err)
	}
}

func TestRunEngineErrorReportsLastError(t *testing.T) {
	eng := newFakeEngine()
	eng.resumeLabel = "Khôi phục"
	eng.snap.AdvanceMode = "auto"
	h := startHarness(t, eng, nil)
	init := testInit(h.bookDir, api.ActionResume)
	init.AdvanceMode = api.AdvanceAuto
	h.send(init)
	waitCall(t, eng, "Resume")
	eng.emit(host.Event{Time: time.Now(), Category: "ERROR", Level: "error", Summary: "mô hình trả lỗi 500"})
	h.expect(api.MsgEvent, time.Second)
	eng.finish(func(s *host.UISnapshot) { s.Phase = "writing" })
	exit := h.expect(api.MsgExit, 2*time.Second)
	if exit.StopReason != api.StopError || exit.Error != "mô hình trả lỗi 500" {
		t.Fatalf("exit sai: %+v", exit)
	}
	_ = h.wait(time.Second)
}

func TestRunRuntimeCommandsAck(t *testing.T) {
	eng := newFakeEngine()
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionStart))
	waitCall(t, eng, "StartPrepared")
	h.send(api.WorkerCommand{Type: api.MsgSetAdvanceMode, ID: "m1", Mode: api.AdvanceAuto})
	if ack := h.expect(api.MsgAck, time.Second); ack.ID != "m1" || !ack.OK {
		t.Fatalf("ack sai: %+v", ack)
	}
	h.send(map[string]string{"type": "bogus", "id": "x"})
	if m := h.expect(api.MsgError, time.Second); m.Code != api.ErrCodeProtocol {
		t.Fatalf("type lạ phải trả error protocol: %+v", m)
	}
	h.send(api.WorkerCommand{Type: api.MsgSteer, ID: "s1", Text: "tăng kịch tính"})
	if ack := h.expect(api.MsgAck, time.Second); ack.ID != "s1" || !ack.OK {
		t.Fatalf("ack steer sai: %+v", ack)
	}
	if !slices.Contains(eng.Calls(), "SetAdvanceMode:auto") {
		t.Fatalf("thiếu SetAdvanceMode:auto: %v", eng.Calls())
	}
	eng.finish(func(s *host.UISnapshot) { s.Phase = "complete" })
	if exit := h.expect(api.MsgExit, 2*time.Second); exit.StopReason != api.StopComplete {
		t.Fatalf("exit sai: %+v", exit)
	}
	_ = h.wait(time.Second)
}

func TestRunInitFailures(t *testing.T) {
	cases := []struct {
		name    string
		initMut func(*api.WorkerInit)
		depsMut func(*deps)
		code    string
	}{
		{"cwd khác book_dir", nil, func(d *deps) {
			d.getwd = func() (string, error) { return "/", nil }
		}, api.ErrCodeConfigInvalid},
		{"khoá thư mục bận", nil, func(d *deps) {
			d.newEngine = func(bootstrap.Config, assets.Bundle) (engine, error) {
				return nil, fmt.Errorf("x: %w", host.ErrBookInUse)
			}
		}, api.ErrCodeLockBusy},
		{"model không có giá", func(m *api.WorkerInit) {
			m.LLM.Model = "khong-ton-tai/model-xyz"
			m.Pricing = &api.ModelPricing{Provider: "khac", ID: "model-khac", InputCostPer1M: 1, OutputCostPer1M: 1}
		}, nil, api.ErrCodeModelUnpriced},
		{"provider ngoài allowlist", func(m *api.WorkerInit) { m.LLM.Provider = "ollama" }, nil, api.ErrCodeProtocol},
		{"book_dir tương đối", func(m *api.WorkerInit) { m.BookDir = "books/b1" }, nil, api.ErrCodeConfigInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng := newFakeEngine()
			h := startHarness(t, eng, tc.depsMut)
			init := testInit(h.bookDir, api.ActionStart)
			if tc.initMut != nil {
				tc.initMut(&init)
			}
			h.send(init)
			if m := h.expect(api.MsgError, time.Second); m.Code != tc.code {
				t.Fatalf("code = %q, muốn %q (%s)", m.Code, tc.code, m.Message)
			}
			exit := h.expect(api.MsgExit, time.Second)
			if exit.Reason != api.ExitError {
				t.Fatalf("exit sai: %+v", exit)
			}
			if err := h.wait(time.Second); err == nil {
				t.Fatal("run phải trả lỗi")
			}
			if len(eng.Calls()) != 0 {
				t.Fatalf("không được chạm engine: %v", eng.Calls())
			}
		})
	}
}

func TestRunSkipsSetAdvanceModeWhenUnchanged(t *testing.T) {
	eng := newFakeEngine()
	eng.resumeLabel = "Khôi phục"
	eng.snap.AdvanceMode = api.AdvanceReview
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionResume))
	waitCall(t, eng, "Resume")
	if slices.ContainsFunc(eng.Calls(), func(c string) bool { return strings.HasPrefix(c, "SetAdvanceMode") }) {
		t.Fatalf("không được ghi lại advance mode không đổi: %v", eng.Calls())
	}
	eng.finish(func(s *host.UISnapshot) { s.Phase = "writing" })
	h.expect(api.MsgExit, 2*time.Second)
	_ = h.wait(time.Second)
}

func TestRunCommandAfterInitWaitsForAction(t *testing.T) {
	eng := newFakeEngine()
	eng.startBlock = make(chan struct{})
	h := startHarness(t, eng, nil)
	// init và steer đi cùng một lần ghi: steer không được chạy trước StartPrepared.
	initLine := mustJSON(t, testInit(h.bookDir, api.ActionStart))
	steerLine := mustJSON(t, api.WorkerCommand{Type: api.MsgSteer, ID: "s1", Text: "x"})
	if _, err := h.stdin.Write(append(append(append(initLine, '\n'), steerLine...), '\n')); err != nil {
		t.Fatal(err)
	}
	waitCall(t, eng, "StartPrepared")
	time.Sleep(100 * time.Millisecond)
	if slices.Contains(eng.Calls(), "Steer") {
		t.Fatalf("Steer chạy trước khi action xong: %v", eng.Calls())
	}
	close(eng.startBlock)
	if ack := h.expect(api.MsgAck, time.Second); ack.ID != "s1" {
		t.Fatalf("ack sai: %+v", ack)
	}
	calls := eng.Calls()
	if slices.Index(calls, "StartPrepared") > slices.Index(calls, "Steer") {
		t.Fatalf("thứ tự sai: %v", calls)
	}
	eng.finish(func(s *host.UISnapshot) { s.Phase = "complete" })
	h.expect(api.MsgExit, 2*time.Second)
	_ = h.wait(time.Second)
}

func TestRunReportsConsumedHold(t *testing.T) {
	eng := newFakeEngine()
	eng.resumeLabel = "Khôi phục"
	h := startHarness(t, eng, nil)
	init := testInit(h.bookDir, api.ActionResume)
	init.AdvanceMode = api.AdvanceAuto
	h.send(init)
	waitCall(t, eng, "Resume")
	eng.mu.Lock()
	eng.snap.HasAdvanceHold = true // phán quyết can thiệp đặt hold "dừng sau chương 5"
	eng.mu.Unlock()
	time.Sleep(100 * time.Millisecond) // vài nhịp poll để worker thấy hold
	// Gate tiêu thụ hold rồi mới dừng: snapshot cuối không còn hold.
	eng.finish(func(s *host.UISnapshot) { s.HasAdvanceHold = false; s.Phase = "writing" })
	if exit := h.expect(api.MsgExit, 2*time.Second); exit.StopReason != api.StopHold {
		t.Fatalf("exit sai: %+v", exit)
	}
	_ = h.wait(time.Second)
}

func TestRunShutdownWaitsForInFlightAction(t *testing.T) {
	eng := newFakeEngine()
	eng.startBlock = make(chan struct{}) // không mở: chỉ Close mới giải phóng
	h := startHarness(t, eng, nil)
	h.send(testInit(h.bookDir, api.ActionStart))
	waitCall(t, eng, "StartPrepared")
	h.send(api.WorkerCommand{Type: api.MsgShutdown, ID: "x"})
	if exit := h.expect(api.MsgExit, time.Second); exit.Reason != api.ExitShutdown {
		t.Fatalf("exit sai: %+v", exit)
	}
	if err := h.wait(time.Second); err != nil {
		t.Fatal(err)
	}
}
