package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/store"
)

const secretKey = "sk-super-secret-key-123"

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fakeworker-bin-")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "fakeworker")
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build fakeworker: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type proc struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  interface{ Write([]byte) (int, error) }
	closeI func()
	msgs   chan api.WorkerMessage
	stderr *bytes.Buffer
	raw    *bytes.Buffer
	done   chan error
}

func startProc(t *testing.T, dir string, init api.WorkerInit) *proc {
	t.Helper()
	cmd := exec.Command(binPath, "worker", "--version", "v-test")
	cmd.Dir = dir
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p := &proc{t: t, cmd: cmd, stdin: in, closeI: func() { _ = in.Close() },
		msgs: make(chan api.WorkerMessage, 1024), stderr: &bytes.Buffer{}, raw: &bytes.Buffer{}, done: make(chan error, 1)}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for sc.Scan() {
			p.raw.Write(sc.Bytes())
			var m api.WorkerMessage
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("dòng stdout không phải JSON: %q", sc.Text())
				continue
			}
			p.msgs <- m
		}
		close(p.msgs)
		p.done <- cmd.Wait()
	}()
	init.Type = api.MsgInit
	init.Protocol = api.WorkerProtocol
	init.BookDir = dir
	init.LLM.APIKey = secretKey
	p.send(init)
	return p
}

func (p *proc) send(v any) {
	b, _ := json.Marshal(v)
	if _, err := p.stdin.Write(append(b, '\n')); err != nil {
		p.t.Logf("ghi stdin: %v", err)
	}
}

// collect đọc tới message exit (hoặc hết giờ) và trả toàn bộ message.
func (p *proc) collect(timeout time.Duration) []api.WorkerMessage {
	p.t.Helper()
	var all []api.WorkerMessage
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-p.msgs:
			if !ok {
				return all
			}
			all = append(all, m)
			if m.Type == api.MsgExit {
				return all
			}
		case <-deadline:
			p.t.Fatalf("quá hạn %s chờ exit; đã nhận %d message", timeout, len(all))
		}
	}
}

func lastExit(t *testing.T, msgs []api.WorkerMessage) api.WorkerMessage {
	t.Helper()
	if len(msgs) == 0 || msgs[len(msgs)-1].Type != api.MsgExit {
		t.Fatalf("message cuối không phải exit: %+v", msgs)
	}
	return msgs[len(msgs)-1]
}

func count(msgs []api.WorkerMessage, typ string) int {
	n := 0
	for _, m := range msgs {
		if m.Type == typ {
			n++
		}
	}
	return n
}

func newDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func run(t *testing.T, dir string, init api.WorkerInit) []api.WorkerMessage {
	t.Helper()
	p := startProc(t, dir, init)
	msgs := p.collect(15 * time.Second)
	if err := <-p.done; err != nil {
		t.Fatalf("process thoát lỗi: %v", err)
	}
	return msgs
}

func completed(t *testing.T, dir string) int {
	t.Helper()
	p, err := store.NewStore(dir).Progress.Load()
	if err != nil || p == nil {
		t.Fatalf("load progress: %v", err)
	}
	return len(p.CompletedChapters)
}

func TestStartReviewWritesOneChapter(t *testing.T) {
	dir := newDir(t)
	msgs := run(t, dir, api.WorkerInit{Action: api.ActionStart, Prompt: "kiếm hiệp bên sông [fake:fast]", AdvanceMode: api.AdvanceReview, BudgetUSD: 10})
	if msgs[0].Type != api.MsgHello || msgs[0].Version != "v-test" || msgs[0].Protocol != 1 {
		t.Fatalf("hello sai: %+v", msgs[0])
	}
	if count(msgs, api.MsgEvent) == 0 || count(msgs, api.MsgStream) == 0 || count(msgs, api.MsgSnapshot) == 0 {
		t.Fatalf("thiếu event/stream/snapshot")
	}
	// Event MODEL/TOOL phải có cặp start/finish cùng id.
	started, finished := map[string]bool{}, map[string]bool{}
	for _, m := range msgs {
		if m.Event == nil {
			continue
		}
		if m.Event.Finished {
			finished[m.Event.ID] = true
		} else {
			started[m.Event.ID] = true
		}
	}
	for id := range started {
		if !finished[id] {
			t.Errorf("event %s không có bản finished", id)
		}
	}
	if x := lastExit(t, msgs); x.Reason != api.ExitStopped || x.StopReason != api.StopReviewWait {
		t.Fatalf("exit sai: %+v", x)
	}
	if _, err := os.Stat(filepath.Join(dir, "chapters", "01.md")); err != nil {
		t.Fatal(err)
	}
	if n := completed(t, dir); n != 1 {
		t.Fatalf("completed = %d, want 1", n)
	}
	book, err := store.NewStore(dir).Book.Load()
	if err != nil || book == nil || !strings.HasPrefix(book.Title, "Truyện thử: kiếm hiệp") {
		t.Fatalf("book = %+v err=%v", book, err)
	}
	snap := msgs[len(msgs)-2].Snapshot
	if snap == nil || snap.CompletedCount != 1 || snap.TotalChapters != 3 || snap.TotalCostUSD != 0.40 || snap.BudgetLimitUSD != 10 {
		t.Fatalf("snapshot cuối sai: %+v", snap)
	}
}

func TestAdvanceUntilComplete(t *testing.T) {
	dir := newDir(t)
	base := api.WorkerInit{Prompt: "[fake:fast] truyện", AdvanceMode: api.AdvanceReview, BudgetUSD: 10}
	steps := []struct {
		action, stop string
	}{
		{api.ActionStart, api.StopReviewWait},
		{api.ActionAdvance, api.StopReviewWait},
		{api.ActionAdvance, api.StopComplete},
		{api.ActionResume, api.StopNothingToResume},
	}
	for i, s := range steps {
		init := base
		init.Action = s.action
		if x := lastExit(t, run(t, dir, init)); x.StopReason != s.stop {
			t.Fatalf("bước %d (%s): stop_reason = %q, want %q", i, s.action, x.StopReason, s.stop)
		}
	}
	if n := completed(t, dir); n != 3 {
		t.Fatalf("completed = %d, want 3", n)
	}
	// Chế độ auto chạy một mạch.
	auto := newDir(t)
	x := lastExit(t, run(t, auto, api.WorkerInit{Action: api.ActionStart, Prompt: "[fake:fast]", AdvanceMode: api.AdvanceAuto, BudgetUSD: 10}))
	if x.StopReason != api.StopComplete || completed(t, auto) != 3 {
		t.Fatalf("auto: %+v", x)
	}
}

func TestShutdownDuringWriting(t *testing.T) {
	dir := newDir(t)
	p := startProc(t, dir, api.WorkerInit{Action: api.ActionStart, Prompt: "chậm", AdvanceMode: api.AdvanceAuto, BudgetUSD: 10})
	// Chờ tới khi đã stream để chắc chắn đang ghi.
	for m := range p.msgs {
		if m.Type == api.MsgStream && m.Delta != "" {
			break
		}
	}
	begin := time.Now()
	p.send(api.WorkerCommand{Type: api.MsgShutdown, ID: "c1"})
	msgs := p.collect(3 * time.Second)
	if err := <-p.done; err != nil {
		t.Fatal(err)
	}
	if d := time.Since(begin); d > time.Second {
		t.Fatalf("shutdown mất %s", d)
	}
	x := lastExit(t, msgs)
	if x.Reason != api.ExitShutdown || x.StopReason != "" {
		t.Fatalf("exit sai: %+v", x)
	}
	var acked bool
	for _, m := range msgs {
		acked = acked || (m.Type == api.MsgAck && m.ID == "c1" && m.OK)
	}
	if !acked {
		t.Fatal("thiếu ack cho shutdown")
	}
}

func TestAbortAndSteer(t *testing.T) {
	dir := newDir(t)
	p := startProc(t, dir, api.WorkerInit{Action: api.ActionStart, AdvanceMode: api.AdvanceAuto, BudgetUSD: 10})
	for m := range p.msgs {
		if m.Type == api.MsgStream && m.Delta != "" {
			break
		}
	}
	p.send(api.WorkerCommand{Type: api.MsgSteer, ID: "s1", Text: "đổi hướng"})
	p.send(api.WorkerCommand{Type: api.MsgAbort, ID: "a1"})
	msgs := p.collect(3 * time.Second)
	if x := lastExit(t, msgs); x.StopReason != api.StopUser {
		t.Fatalf("exit sai: %+v", x)
	}
	var sawUser bool
	for _, m := range msgs {
		if m.Event != nil && m.Event.Category == "USER" && strings.Contains(m.Event.Summary, "đổi hướng") {
			sawUser = true
		}
	}
	if !sawUser {
		t.Fatal("thiếu event USER")
	}
}

func TestBudgetStopsSecondChapter(t *testing.T) {
	dir := newDir(t)
	msgs := run(t, dir, api.WorkerInit{Action: api.ActionStart, Prompt: "[fake:fast]", AdvanceMode: api.AdvanceAuto, BudgetUSD: 0.5})
	if x := lastExit(t, msgs); x.Reason != api.ExitStopped || x.StopReason != api.StopBudget {
		t.Fatalf("exit sai: %+v", x)
	}
	if n := completed(t, dir); n != 1 {
		t.Fatalf("completed = %d, want 1", n)
	}
	data, err := os.ReadFile(filepath.Join(dir, "meta", "fake_cost.json"))
	if err != nil || !strings.Contains(string(data), "0.4") {
		t.Fatalf("fake_cost.json = %q err=%v", data, err)
	}
}

func TestScenarioMarkers(t *testing.T) {
	t.Run("crash", func(t *testing.T) {
		p := startProc(t, newDir(t), api.WorkerInit{Action: api.ActionStart, Prompt: "[fake:crash]", BudgetUSD: 10})
		msgs := p.collect(5 * time.Second)
		err := <-p.done
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != 3 {
			t.Fatalf("err = %v, want exit 3", err)
		}
		if count(msgs, api.MsgExit) != 0 || p.stderr.Len() == 0 {
			t.Fatalf("crash phải không có exit message và có stderr")
		}
	})
	t.Run("lockbusy", func(t *testing.T) {
		dir := newDir(t)
		init := api.WorkerInit{Action: api.ActionStart, Prompt: "[fake:lockbusy] [fake:fast]", AdvanceMode: api.AdvanceReview, BudgetUSD: 10}
		p := startProc(t, dir, init)
		msgs := p.collect(5 * time.Second)
		if err := <-p.done; err == nil {
			t.Fatal("lần đầu phải thoát mã 1")
		}
		var code string
		for _, m := range msgs {
			if m.Type == api.MsgError {
				code = m.Code
			}
		}
		if code != api.ErrCodeLockBusy || lastExit(t, msgs).Reason != api.ExitError {
			t.Fatalf("msgs = %+v", msgs)
		}
		if x := lastExit(t, run(t, dir, init)); x.StopReason != api.StopReviewWait {
			t.Fatalf("lần hai: %+v", x)
		}
	})
	t.Run("error", func(t *testing.T) {
		msgs := run(t, newDir(t), api.WorkerInit{Action: api.ActionStart, Prompt: "[fake:error] [fake:fast]", AdvanceMode: api.AdvanceAuto, BudgetUSD: 10})
		if x := lastExit(t, msgs); x.StopReason != api.StopError || x.Error != "lỗi giả lập" {
			t.Fatalf("exit sai: %+v", x)
		}
	})
	t.Run("hang", func(t *testing.T) {
		p := startProc(t, newDir(t), api.WorkerInit{Action: api.ActionStart, Prompt: "[fake:hang]", BudgetUSD: 10})
		if m := <-p.msgs; m.Type != api.MsgHello { // handler tín hiệu đã cài xong khi hello ra
			t.Fatalf("message đầu = %+v", m)
		}
		p.send(api.WorkerCommand{Type: api.MsgShutdown, ID: "x"})
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
		p.closeI()
		select {
		case <-p.done:
			t.Fatal("hang phải sống sót sau shutdown/SIGTERM/EOF")
		case <-time.After(700 * time.Millisecond):
		}
		_ = p.cmd.Process.Kill()
	})
}

func TestInitValidation(t *testing.T) {
	dir := newDir(t)
	p := startProc(t, dir, api.WorkerInit{Action: api.ActionStart, BudgetUSD: 1})
	// startProc ghi đè BookDir đúng; gửi lại bản sai bằng process khác.
	_ = p.cmd.Process.Kill()

	cmd := exec.Command(binPath, "worker")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(`{"type":"init","protocol":1,"action":"start","book_dir":"/nơi/khác"}` + "\n")
	out, _ := cmd.Output()
	if !strings.Contains(string(out), api.ErrCodeConfigInvalid) {
		t.Fatalf("stdout = %s", out)
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Fatalf("exit code = %d", cmd.ProcessState.ExitCode())
	}
}

func TestAPIKeyNeverLeaks(t *testing.T) {
	dir := newDir(t)
	p := startProc(t, dir, api.WorkerInit{Action: api.ActionStart, Prompt: "[fake:fast]", AdvanceMode: api.AdvanceAuto, BudgetUSD: 10})
	p.collect(10 * time.Second)
	<-p.done
	if strings.Contains(p.raw.String(), secretKey) || strings.Contains(p.stderr.String(), secretKey) {
		t.Fatal("api key lộ ra stdout/stderr")
	}
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, _ := os.ReadFile(path); bytes.Contains(b, []byte(secretKey)) {
				t.Errorf("api key bị ghi vào %s", path)
			}
		}
		return nil
	})
}
