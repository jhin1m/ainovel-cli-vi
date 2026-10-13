package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/db/dbtest"
)

const (
	testVersion = "fake-v"
	testSecret  = "sk-test-secret"
)

// fakeBin là worker giả, build một lần cho cả package.
var fakeBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jobs-fakeworker-")
	if err != nil {
		panic(err)
	}
	fakeBin = filepath.Join(dir, "fakeworker")
	if out, err := exec.Command("go", "build", "-o", fakeBin, "./fakeworker").CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build fakeworker: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// fakeKeys cấp key giả. Mỗi lần gọi trả bản sao mới vì supervisor xoá key sau khi gửi init.
type fakeKeys struct{}

func (fakeKeys) DecryptForBook(context.Context, string) (string, []byte, error) {
	return "openrouter", []byte(testSecret), nil
}

type fakePricing struct{}

func (fakePricing) PricingFor(_, model string) (api.ModelPricing, bool) {
	return api.ModelPricing{Provider: "openrouter", ID: model, InputCostPer1M: 1, OutputCostPer1M: 2}, true
}

var seqN atomic.Int64

// newUser tạo một user và một API key giả, trả về id user và id key.
func newUser(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	var uid, kid string
	email := fmt.Sprintf("tester%d@test.local", seqN.Add(1))
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id::text`, email).Scan(&uid); err != nil {
		t.Fatalf("tạo user: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, provider, ciphertext, nonce, last4)
		VALUES ($1::uuid, 'openrouter', $2, $3, 'test') RETURNING id::text`,
		uid, []byte("c"), []byte("n")).Scan(&kid); err != nil {
		t.Fatalf("tạo api key: %v", err)
	}
	return uid, kid
}

// bookFix mô tả một dòng books cần chèn. Trường rỗng nhận giá trị mặc định.
type bookFix struct {
	status         api.BookStatus // mặc định queued
	action         string         // pending_action; rỗng = NULL
	text           string         // prompt và pending_text
	advance        string         // mặc định auto
	budget         float64        // mặc định 10
	resumeFailures int
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func insertBook(t *testing.T, pool *pgxpool.Pool, uid, kid string, f bookFix) string {
	t.Helper()
	if f.status == "" {
		f.status = api.StatusQueued
	}
	if f.advance == "" {
		f.advance = api.AdvanceAuto
	}
	if f.budget == 0 {
		f.budget = 10
	}
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO books (user_id, api_key_id, prompt, status, provider, model, budget_usd, advance_mode,
			pending_action, pending_text, resume_failures)
		VALUES ($1::uuid, $2::uuid, $3, $4, 'openrouter', 'test-model', $5, $6, $7, $8, $9)
		RETURNING id::text`,
		uid, kid, f.text, string(f.status), f.budget, f.advance, nullable(f.action), nullable(f.text), f.resumeFailures).Scan(&id)
	if err != nil {
		t.Fatalf("chèn truyện: %v", err)
	}
	return id
}

// bookRow là các cột supervisor ghi, đọc thẳng từ DB.
type bookRow struct {
	Status                     api.BookStatus
	StopReason, LastError      string
	ResumeFailures             int
	PendingAction, PendingText *string
	HasSnapshot                bool
}

func loadBook(pool *pgxpool.Pool, id string) (bookRow, error) {
	var b bookRow
	err := pool.QueryRow(context.Background(), `
		SELECT status, stop_reason, last_error, resume_failures, pending_action, pending_text, snapshot IS NOT NULL
		FROM books WHERE id = $1::uuid`, id).
		Scan(&b.Status, &b.StopReason, &b.LastError, &b.ResumeFailures, &b.PendingAction, &b.PendingText, &b.HasSnapshot)
	return b, err
}

func mustBook(t *testing.T, pool *pgxpool.Pool, id string) bookRow {
	t.Helper()
	b, err := loadBook(pool, id)
	if err != nil {
		t.Fatalf("đọc truyện %s: %v", id, err)
	}
	return b
}

// waitBook chờ truyện đạt đúng status rồi trả về dòng DB.
func waitBook(t *testing.T, pool *pgxpool.Pool, id string, want api.BookStatus) bookRow {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		b, err := loadBook(pool, id)
		if err == nil && b.Status == want {
			return b
		}
		if time.Now().After(deadline) {
			t.Fatalf("hết thời gian chờ status %q; hiện %q (stop=%q, last_error=%q, err=%v)", want, b.Status, b.StopReason, b.LastError, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("hết thời gian chờ: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// recorder ghi lại mọi message hub phát cho một subscription.
type recorder struct {
	mu   sync.Mutex
	msgs []Message
}

func record(t *testing.T, sub *Subscription) *recorder {
	t.Helper()
	r := &recorder{}
	t.Cleanup(sub.Close)
	go func() {
		for {
			select {
			case m := <-sub.C():
				r.mu.Lock()
				r.msgs = append(r.msgs, m)
				r.mu.Unlock()
			case <-sub.Dropped():
				return
			}
		}
	}()
	return r
}

func (r *recorder) count(typ string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, m := range r.msgs {
		if m.Type == typ {
			n++
		}
	}
	return n
}

// statuses trả về các status đã phát, theo thứ tự.
func (r *recorder) statuses() []api.BookStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.BookStatus
	for _, m := range r.msgs {
		if m.Type != api.SSEStatus {
			continue
		}
		var sc api.StatusChange
		if json.Unmarshal(m.Data, &sc) == nil {
			out = append(out, sc.Status)
		}
	}
	return out
}

// newSupervisor dựng supervisor với worker giả. Shutdown được gọi khi test xong.
func newSupervisor(t *testing.T, pool *pgxpool.Pool, mut func(*Config)) *Supervisor {
	t.Helper()
	cfg := Config{
		DataDir: t.TempDir(), Version: testVersion,
		MaxJobs: 8, MaxJobsPerUser: 8,
		WorkerBin: fakeBin, WorkerArgs: []string{"--version", testVersion},
		Language: "zh-CN", StopGrace: 300 * time.Millisecond,
	}
	if mut != nil {
		mut(&cfg)
	}
	sup := New(cfg, pool, fakeKeys{}, fakePricing{}, NewHubs())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		sup.Shutdown(ctx)
	})
	return sup
}

// start chạy Start rồi huỷ vòng lập lịch khi test xong (trước khi Shutdown chạy).
func start(t *testing.T, sup *Supervisor) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := sup.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

// waitReady chờ worker của truyện đã chào và nhận init xong.
func waitReady(t *testing.T, sup *Supervisor, id string) *proc {
	t.Helper()
	var p *proc
	waitFor(t, "worker sẵn sàng "+id, func() bool {
		p = sup.live(id)
		if p == nil {
			return false
		}
		select {
		case <-p.ready:
			return true
		default:
			return false
		}
	})
	return p
}

func abortBook(t *testing.T, sup *Supervisor, id string) {
	t.Helper()
	ack, err := sup.Command(context.Background(), id, api.WorkerCommand{Type: api.MsgAbort})
	if err != nil || ack == nil || !ack.OK {
		t.Fatalf("abort %s: ack=%+v err=%v", id, ack, err)
	}
}

// assertQueuedWhileFull kiểm tra trong suốt d: truyện vẫn queued và số worker sống không vượt limit.
func assertQueuedWhileFull(t *testing.T, sup *Supervisor, pool *pgxpool.Pool, id string, limit int, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b := mustBook(t, pool, id); b.Status != api.StatusQueued {
			t.Fatalf("truyện %s = %s, muốn queued khi slot đầy", id, b.Status)
		}
		if n := sup.RunningCount(); n > limit {
			t.Fatalf("RunningCount = %d, vượt limit %d", n, limit)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Worker chạy chậm (~3.6s): không có marker, chế độ auto. Dùng khi cần giữ process sống.
const slowPrompt = "truyện chậm"

func TestWorkerCmdEnvIsAllowlist(t *testing.T) {
	t.Setenv("TZ", "Asia/Ho_Chi_Minh")
	t.Setenv("AINOVEL_SECRET_TEST", "khong-duoc-lo")
	dataDir := t.TempDir()
	s := New(Config{DataDir: dataDir, WorkerBin: fakeBin, WorkerArgs: []string{"--version", testVersion}},
		nil, fakeKeys{}, fakePricing{}, NewHubs())
	bookDir := filepath.Join(dataDir, "users", "u", "books", "b")
	home := filepath.Join(dataDir, "users", "u", "home")
	cmd, err := s.workerCmd(bookDir, home)
	if err != nil {
		t.Fatalf("workerCmd: %v", err)
	}
	want := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "TZ=Asia/Ho_Chi_Minh"}
	if !slices.Equal(cmd.Env, want) {
		t.Fatalf("env = %q, muốn đúng %q", cmd.Env, want)
	}
	if cmd.Dir != bookDir {
		t.Fatalf("Dir = %q, muốn %q", cmd.Dir, bookDir)
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "worker" {
		t.Fatalf("Args = %q, muốn bắt đầu bằng worker", cmd.Args)
	}
}

// Ánh xạ exit của worker sang status truyện, không cần process thật.
func TestOutcomeMapsExitToStatus(t *testing.T) {
	exit := func(reason, stop, errText string) *api.WorkerMessage {
		return &api.WorkerMessage{Type: api.MsgExit, Reason: reason, StopReason: stop, Error: errText}
	}
	cases := []struct {
		name      string
		exit      *api.WorkerMessage
		errCode   string
		status    api.BookStatus
		failed    bool
		requeue   bool
		lastError string
	}{
		{"review_wait", exit(api.ExitStopped, api.StopReviewWait, ""), "", api.StatusWaitingReview, false, false, ""},
		{"budget", exit(api.ExitStopped, api.StopBudget, ""), "", api.StatusBudgetExhausted, false, false, ""},
		{"complete", exit(api.ExitStopped, api.StopComplete, ""), "", api.StatusCompleted, false, false, ""},
		{"nothing_to_resume", exit(api.ExitStopped, api.StopNothingToResume, ""), "", api.StatusCompleted, false, false, ""},
		{"user", exit(api.ExitStopped, api.StopUser, ""), "", api.StatusPaused, false, false, ""},
		{"hold", exit(api.ExitStopped, api.StopHold, ""), "", api.StatusPaused, false, false, ""},
		{"pending_steer", exit(api.ExitStopped, api.StopPendingSteer, ""), "", api.StatusPaused, false, false, ""},
		{"error_with_message", exit(api.ExitStopped, api.StopError, "lỗi engine"), "", api.StatusFailed, true, false, msgRunFailed},
		{"unknown_stop_reason", exit(api.ExitStopped, "khong_ro", ""), "", api.StatusFailed, true, false, msgRunFailed},
		{"exit_error", exit(api.ExitError, "", "boom"), "", api.StatusFailed, true, false, msgRunFailed},
		{"exit_shutdown", exit(api.ExitShutdown, "", ""), "", api.StatusPaused, false, false, ""},
		{"no_exit_message", nil, "", api.StatusFailed, true, false, msgWorkerCrashed},
		{"lock_busy_requeues", exit(api.ExitError, "", "lock_busy"), api.ErrCodeLockBusy, "", false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newProc("b", "u", nil)
			p.exit = tc.exit
			p.errCode = tc.errCode
			res, requeue := p.outcome()
			if requeue != tc.requeue {
				t.Fatalf("requeue = %v, muốn %v", requeue, tc.requeue)
			}
			if requeue {
				return
			}
			if res.Status != tc.status || res.Failed != tc.failed || res.LastError != tc.lastError {
				t.Fatalf("outcome = {status:%s failed:%v last_error:%q}, muốn {status:%s failed:%v last_error:%q}",
					res.Status, res.Failed, res.LastError, tc.status, tc.failed, tc.lastError)
			}
			if res.ClearPending == tc.failed {
				t.Fatalf("ClearPending = %v với failed = %v: chỉ lượt sạch mới xoá việc đang chờ", res.ClearPending, tc.failed)
			}
			if strings.Contains(res.LastError, "boom") || strings.Contains(res.LastError, "lỗi engine") {
				t.Fatalf("last_error lộ chi tiết lỗi: %q", res.LastError)
			}
		})
	}
}

// Các stop_reason của worker giả đi tới đúng status; lượt lỗi tăng resume_failures,
// lượt sạch đặt lại về 0.
func TestStopReasonsReachBookStatus(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	review := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] duyệt", advance: api.AdvanceReview, resumeFailures: 2})
	budget := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] ngân sách", budget: 0.5})
	done := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] hoàn thành"})
	broken := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast][fake:error] lỗi"})
	sup := newSupervisor(t, pool, nil)
	start(t, sup)

	b := waitBook(t, pool, review, api.StatusWaitingReview)
	if b.StopReason != api.StopReviewWait || b.ResumeFailures != 0 || b.LastError != "" || b.PendingAction != nil || !b.HasSnapshot {
		t.Fatalf("review: %+v", b)
	}

	b = waitBook(t, pool, budget, api.StatusBudgetExhausted)
	if b.StopReason != api.StopBudget || b.PendingAction != nil {
		t.Fatalf("budget: %+v", b)
	}

	b = waitBook(t, pool, done, api.StatusCompleted)
	if b.StopReason != api.StopComplete || b.PendingAction != nil || !b.HasSnapshot {
		t.Fatalf("complete: %+v", b)
	}

	b = waitBook(t, pool, broken, api.StatusFailed)
	if b.StopReason != api.StopError || b.LastError != msgRunFailed || b.ResumeFailures != 1 {
		t.Fatalf("error: %+v", b)
	}
	if b.PendingAction == nil || *b.PendingAction != api.ActionStart || b.PendingText == nil {
		t.Fatalf("lượt lỗi phải giữ việc đang chờ để thử lại: %+v", b)
	}
}

func TestAbortPausesRunningWorker(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: slowPrompt})
	sup := newSupervisor(t, pool, nil)
	rec := record(t, sup.Hubs().Get(id).Subscribe())
	start(t, sup)
	// Chờ tới khi đang viết chương: abort trước đó sẽ làm lỗi ở bước lập dàn ý.
	waitFor(t, "đang viết chương", func() bool { return rec.count(api.SSEStream) > 0 })
	abortBook(t, sup, id)

	b := waitBook(t, pool, id, api.StatusPaused)
	if b.StopReason != api.StopUser || b.ResumeFailures != 0 {
		t.Fatalf("paused: %+v", b)
	}
	waitFor(t, "worker thoát", func() bool { return !sup.IsRunning(id) })
}

// Worker chào sai phiên bản: bị kill, truyện giữ interrupted, không tính là lỗi.
func TestHelloVersionMismatchInterrupts(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] lệch phiên bản"})
	sup := newSupervisor(t, pool, func(c *Config) { c.Version = "phien-ban-khac" })
	start(t, sup)

	b := waitBook(t, pool, id, api.StatusInterrupted)
	if b.ResumeFailures != 0 || b.PendingAction == nil || *b.PendingAction != api.ActionStart {
		t.Fatalf("interrupted: %+v", b)
	}
	waitFor(t, "worker bị kill", func() bool { return sup.RunningCount() == 0 })
}

// Worker bỏ qua shutdown và SIGTERM: chỉ SIGKILL mới dừng được. Hai bước chờ
// grace nên StopAndWait phải mất ít nhất 2*grace.
func TestStopAndWaitEscalatesToSigkill(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:hang] treo"})
	sup := newSupervisor(t, pool, nil)
	start(t, sup)
	waitReady(t, sup, id)

	began := time.Now()
	sup.StopAndWait(id)
	elapsed := time.Since(began)

	if min := 2 * sup.cfg.StopGrace; elapsed < min {
		t.Fatalf("StopAndWait trả về sau %v, nhanh hơn 2*grace=%v: chưa leo thang đủ", elapsed, min)
	}
	if elapsed > 2*sup.cfg.StopGrace+3*time.Second {
		t.Fatalf("StopAndWait mất %v, quá lâu", elapsed)
	}
	if sup.IsRunning(id) {
		t.Fatal("worker vẫn sống sau StopAndWait")
	}
	b := mustBook(t, pool, id)
	if b.Status != api.StatusFailed || b.LastError == "" {
		t.Fatalf("worker bị giết không có exit: %+v", b)
	}
}

// Crash không có message exit: failed kèm đuôi stderr; key không lọt ra log hay DB.
func TestCrashWithoutExitFailsWithStderrTail(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:crash] sập"})
	sup := newSupervisor(t, pool, nil)
	start(t, sup)

	b := waitBook(t, pool, id, api.StatusFailed)
	if b.StopReason != api.StopError || b.ResumeFailures != 1 {
		t.Fatalf("crash: %+v", b)
	}
	if b.LastError != msgWorkerCrashed {
		t.Fatalf("last_error phải là thông báo chung, được %q", b.LastError)
	}
	if b.PendingAction == nil || *b.PendingAction != api.ActionStart {
		t.Fatalf("crash trước khi engine chạy phải giữ action start: %+v", b)
	}
	// Thử lại: claim phải vẫn trả action start (không rơi về resume trên thư mục rỗng).
	if _, err := pool.Exec(context.Background(), `UPDATE books SET status = 'queued' WHERE id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if row, ok, err := sup.db.claim(context.Background(), id); err != nil || !ok || row.Action != api.ActionStart || row.Text != "[fake:crash] sập" {
		t.Fatalf("claim sau crash = %+v ok=%v err=%v, muốn action start kèm prompt", row, ok, err)
	}
	logPath := filepath.Join(mustBookDir(t, sup, uid, id), "logs", "worker-stderr.log")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("đọc worker-stderr.log: %v", err)
	}
	if !containsAll(string(logData), "giả lập sự cố", "goroutine 1") {
		t.Fatalf("worker-stderr.log thiếu nội dung stderr: %q", logData)
	}
	if containsAll(b.LastError, "goroutine", "panic", ".go") || containsAll(b.LastError, testSecret) || containsAll(string(logData), testSecret) {
		t.Fatal("API key lọt vào last_error hoặc worker-stderr.log")
	}
}

// lock_busy: xếp hàng lại, chờ backoff rồi chạy; lỗi khoá không tính vào resume_failures.
func TestLockBusyRequeuesThenRuns(t *testing.T) {
	old := lockBusyBackoff
	lockBusyBackoff = []time.Duration{200 * time.Millisecond}
	t.Cleanup(func() { lockBusyBackoff = old })

	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:lockbusy] [fake:fast] thử lại"})
	sup := newSupervisor(t, pool, nil)
	rec := record(t, sup.Hubs().Get(id).Subscribe())
	start(t, sup)

	b := waitBook(t, pool, id, api.StatusCompleted)
	if b.ResumeFailures != 0 || b.LastError != "" {
		t.Fatalf("sau lock_busy: %+v", b)
	}
	want := []api.BookStatus{api.StatusQueued, api.StatusRunning, api.StatusCompleted}
	if !isSubsequence(rec.statuses(), want) {
		t.Fatalf("chuỗi status %v, muốn có con %v theo thứ tự", rec.statuses(), want)
	}
	if _, err := os.Stat(filepath.Join(mustBookDir(t, sup, uid, id), ".fake_lockbusy_count")); err != nil {
		t.Fatalf("lần thử đầu không chạy: %v", err)
	}
}

func TestMaxJobsGlobalLimit(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	slow := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: slowPrompt})
	waiting := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] đợi slot"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 1 })
	start(t, sup)

	waitBook(t, pool, slow, api.StatusRunning)
	assertQueuedWhileFull(t, sup, pool, waiting, 1, 800*time.Millisecond)
	abortBook(t, sup, slow)
	waitBook(t, pool, slow, api.StatusPaused)
	waitBook(t, pool, waiting, api.StatusCompleted)
}

func TestMaxJobsPerUserLimit(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	slowA := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: slowPrompt})
	waitingA := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: "[fake:fast] cùng user"})
	fastB := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] user khác"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 4; c.MaxJobsPerUser = 1 })
	start(t, sup)

	waitBook(t, pool, slowA, api.StatusRunning)
	// User khác không bị chặn bởi slot của user A.
	waitBook(t, pool, fastB, api.StatusCompleted)
	assertQueuedWhileFull(t, sup, pool, waitingA, 1, 800*time.Millisecond)
	abortBook(t, sup, slowA)
	waitBook(t, pool, slowA, api.StatusPaused)
	waitBook(t, pool, waitingA, api.StatusCompleted)
}

// Truyện chờ duyệt không giữ slot: truyện sau chạy xong khi truyện trước vẫn waiting_review.
func TestWaitingReviewReleasesSlot(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	review := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] duyệt", advance: api.AdvanceReview})
	next := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] truyện sau"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 1 })
	start(t, sup)

	waitBook(t, pool, review, api.StatusWaitingReview)
	waitBook(t, pool, next, api.StatusCompleted)
	waitFor(t, "không còn worker", func() bool { return sup.RunningCount() == 0 })
}

// Lập lịch lặp lại khi worker đang sống không được spawn thêm process cho cùng truyện.
func TestNeverTwoProcessesForOneBook(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: slowPrompt})
	sup := newSupervisor(t, pool, nil)
	rec := record(t, sup.Hubs().Get(id).Subscribe())
	start(t, sup)
	waitReady(t, sup, id)
	// Chờ tới khi đang viết chương: lúc đó worker đã phát status running.
	waitFor(t, "đang viết chương", func() bool { return rec.count(api.SSEStream) > 0 })

	ctx := context.Background()
	for i := 0; i < 30; i++ {
		sup.Kick()
		if err := sup.schedule(ctx); err != nil {
			t.Fatalf("schedule: %v", err)
		}
	}
	time.Sleep(200 * time.Millisecond)
	if n := sup.RunningCount(); n != 1 {
		t.Fatalf("RunningCount = %d, muốn 1", n)
	}
	if n := runningPublishes(rec); n != 1 {
		t.Fatalf("phát status running %d lần, muốn 1 (spawn chồng)", n)
	}
	abortBook(t, sup, id)
	waitBook(t, pool, id, api.StatusPaused)
}

// Server khởi động: running (crash) → queued với resume_failures+1, interrupted → queued,
// giữ nguyên pending_action/pending_text; quá ngưỡng crash → failed; queued và waiting_review giữ nguyên.
func TestRecoverAllRequeuesRunningAndInterrupted(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	crashed := insertBook(t, pool, uid, kid, bookFix{status: api.StatusRunning})
	interrupted := insertBook(t, pool, uid, kid, bookFix{status: api.StatusInterrupted, action: api.ActionSteer, text: "giữ lệnh này"})
	queued := insertBook(t, pool, uid, kid, bookFix{status: api.StatusQueued, action: api.ActionAdvance})
	looping := insertBook(t, pool, uid, kid, bookFix{status: api.StatusRunning, resumeFailures: 2})
	review := insertBook(t, pool, uid, kid, bookFix{status: api.StatusWaitingReview})

	// MaxJobs = 0: không spawn worker, chỉ kiểm bước khôi phục.
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 0 })
	start(t, sup)

	b := mustBook(t, pool, crashed)
	if b.Status != api.StatusQueued || b.ResumeFailures != 1 || b.PendingAction == nil || *b.PendingAction != api.ActionResume {
		t.Fatalf("running → queued: %+v", b)
	}
	b = mustBook(t, pool, interrupted)
	if b.Status != api.StatusQueued || b.PendingAction == nil || *b.PendingAction != api.ActionSteer ||
		b.PendingText == nil || *b.PendingText != "giữ lệnh này" || b.ResumeFailures != 0 {
		t.Fatalf("interrupted → queued: %+v", b)
	}
	b = mustBook(t, pool, queued)
	if b.Status != api.StatusQueued || b.PendingAction == nil || *b.PendingAction != api.ActionAdvance {
		t.Fatalf("queued giữ nguyên: %+v", b)
	}
	b = mustBook(t, pool, looping)
	if b.Status != api.StatusFailed || b.LastError == "" {
		t.Fatalf("crash lặp lại phải failed: %+v", b)
	}
	if b = mustBook(t, pool, review); b.Status != api.StatusWaitingReview {
		t.Fatalf("waiting_review giữ nguyên: %+v", b)
	}
}

// Tắt server có kiểm soát: cả hai worker thành interrupted, không ghi status khác.
// Khởi động supervisor mới trên cùng DB: cả hai được chạy lại.
func TestDrainInterruptsThenRestartResumes(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	dataDir := t.TempDir()
	prompts := map[string]string{}
	for _, text := range []string{"truyện A chậm", "truyện B chậm"} {
		id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: text})
		prompts[id] = text
	}
	ids := make([]string, 0, len(prompts))
	for id := range prompts {
		ids = append(ids, id)
	}

	sup1 := newSupervisor(t, pool, func(c *Config) { c.DataDir = dataDir })
	start(t, sup1)
	for _, id := range ids {
		waitReady(t, sup1, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sup1.BeforeShutdown(ctx)
	sup1.Shutdown(ctx)
	if n := sup1.RunningCount(); n != 0 {
		t.Fatalf("sau Shutdown còn %d worker", n)
	}
	for _, id := range ids {
		b := mustBook(t, pool, id)
		if b.Status != api.StatusInterrupted {
			t.Fatalf("sau drain %s = %s, muốn interrupted", id, b.Status)
		}
		if b.PendingAction == nil || *b.PendingAction != api.ActionStart || b.PendingText == nil || *b.PendingText != prompts[id] {
			t.Fatalf("drain làm mất pending: %+v", b)
		}
	}

	sup2 := newSupervisor(t, pool, func(c *Config) { c.DataDir = dataDir })
	recs := make(map[string]*recorder, len(ids))
	for _, id := range ids {
		recs[id] = record(t, sup2.Hubs().Get(id).Subscribe())
	}
	start(t, sup2)
	for _, id := range ids {
		r := recs[id]
		waitFor(t, "truyện "+id+" được chạy lại", func() bool {
			return slices.Contains(r.statuses(), api.StatusRunning)
		})
	}
}

// Khi hoàn thành: snapshot ghi vào books.snapshot; hub phát snapshot, event, status.
func TestSnapshotAndHubEventsOnCompletion(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] snapshot"})
	sup := newSupervisor(t, pool, nil)
	rec := record(t, sup.Hubs().Get(id).Subscribe())
	start(t, sup)
	waitBook(t, pool, id, api.StatusCompleted)

	var raw string
	if err := pool.QueryRow(context.Background(), `SELECT snapshot::text FROM books WHERE id = $1::uuid`, id).Scan(&raw); err != nil {
		t.Fatalf("đọc snapshot: %v", err)
	}
	var snap api.BookSnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		t.Fatalf("snapshot không phải JSON hợp lệ: %v", err)
	}
	if snap.BookTitle == "" || snap.Provider != "openrouter" {
		t.Fatalf("snapshot thiếu tiêu đề/provider: %+v", snap)
	}
	// Ba chương, mỗi chương 0.40 USD (worker giả).
	if math.Abs(snap.TotalCostUSD-1.2) > 1e-6 {
		t.Fatalf("total_cost_usd = %v, muốn 1.2", snap.TotalCostUSD)
	}

	waitFor(t, "hub nhận status completed", func() bool { return slices.Contains(rec.statuses(), api.StatusCompleted) })
	if rec.count(api.SSEEvent) == 0 || rec.count(api.SSESnapshot) == 0 {
		t.Fatalf("hub thiếu event/snapshot: event=%d snapshot=%d", rec.count(api.SSEEvent), rec.count(api.SSESnapshot))
	}
	late := sup.Hubs().Get(id).Subscribe()
	t.Cleanup(late.Close)
	gotSnap, events := late.Initial()
	if gotSnap == nil || len(events) == 0 {
		t.Fatalf("client mới kết nối không nhận được snapshot/ring: snap=%v events=%d", gotSnap != nil, len(events))
	}
}

// Worker không khởi động được (binary thiếu): run phải đóng p.done và nhả slot,
// để StopAndWait/Shutdown không treo và không panic khi gọi signal trên proc chưa có process.
func TestSpawnFailureReleasesProc(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{status: api.StatusRunning, action: api.ActionStart, text: "khởi động lỗi"})
	sup := newSupervisor(t, pool, func(c *Config) {
		c.WorkerBin = filepath.Join(c.DataDir, "khong-ton-tai")
	})
	p := newProc(id, uid, sup.Hubs().Get(id))
	sup.mu.Lock()
	sup.procs[id] = p
	sup.perUser[uid]++
	sup.mu.Unlock()

	sup.run(p, jobRow{ID: id, UserID: uid, Action: api.ActionStart, Text: "khởi động lỗi",
		Model: "test-model", BudgetUSD: 10, AdvanceMode: api.AdvanceAuto})

	select {
	case <-p.done:
	default:
		t.Fatal("p.done chưa đóng khi worker không khởi động được: StopAndWait sẽ treo")
	}
	if sup.IsRunning(id) {
		t.Fatal("proc vẫn nằm trong bảng theo dõi")
	}
	p.signal(true)
	b := mustBook(t, pool, id)
	if b.Status != api.StatusFailed || b.LastError != "Không khởi động được worker" {
		t.Fatalf("spawn lỗi: %+v", b)
	}
}

// ── helper nhỏ ──

func mustBookDir(t *testing.T, sup *Supervisor, uid, id string) string {
	t.Helper()
	dir, err := BookDir(sup.cfg.DataDir, uid, id)
	if err != nil {
		t.Fatalf("BookDir: %v", err)
	}
	return dir
}

// containsAll kiểm tra s chứa mọi chuỗi con trong parts.
func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}

// isSubsequence: want xuất hiện theo đúng thứ tự trong got (không nhất thiết liền nhau).
func isSubsequence(got, want []api.BookStatus) bool {
	j := 0
	for _, g := range got {
		if j < len(want) && g == want[j] {
			j++
		}
	}
	return j == len(want)
}

func runningPublishes(r *recorder) int {
	n := 0
	for _, st := range r.statuses() {
		if st == api.StatusRunning {
			n++
		}
	}
	return n
}

// Abort gửi khi worker chưa sẵn sàng phải được ghi nhớ (không ghi ra stdin lúc đó) và
// không được mất; sau khi sẵn sàng, abort được ghi thẳng ra stdin.
func TestAbortBeforeReadyIsRemembered(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()

	p := newProc("b", "u", nil)
	p.stdin = pw
	ack, err := p.command(context.Background(), api.WorkerCommand{Type: api.MsgAbort, ID: "a1"})
	if ack != nil || err != nil {
		t.Fatalf("abort trước ready: ack=%v err=%v, muốn nil nil", ack, err)
	}
	if !p.abortLater.Load() {
		t.Fatal("abort trước ready không được ghi nhớ (abortLater)")
	}

	// Mô phỏng run: đánh dấu ready rồi gửi abort đang chờ, như trong run().
	p.markReady()
	if p.abortLater.Load() {
		if err := p.writeLine(api.WorkerCommand{Type: api.MsgAbort, ID: "a2"}); err != nil {
			t.Fatal(err)
		}
	}
	buf := make([]byte, 256)
	n, err := pr.Read(buf)
	if err != nil || !strings.Contains(string(buf[:n]), `"type":"abort"`) {
		t.Fatalf("worker không nhận được abort: %q err=%v", buf[:n], err)
	}
}

// Xoá đúng lúc scheduler giữ chỗ mà claim thất bại: StopAndWait không được treo.
func TestStopAndWaitDuringFailedClaim(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] xoá lúc claim"})
	sup := newSupervisor(t, pool, nil)
	ctx := context.Background()

	// Giữ khoá dòng để UPDATE của claim phải chờ; queued() là SELECT thường nên không bị chặn.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE books SET updated_at = now() WHERE id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	go func() { _ = sup.schedule(ctx) }()
	waitFor(t, "scheduler giữ chỗ", func() bool { return sup.live(id) != nil })

	stopped := make(chan struct{})
	go func() {
		sup.StopAndWait(id)
		close(stopped)
	}()
	// Truyện chuyển sang deleting trước khi claim chạy được: claim sẽ không khớp dòng nào.
	if _, err := tx.Exec(ctx, `UPDATE books SET status = 'deleting' WHERE id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("StopAndWait treo khi claim thất bại")
	}
	if sup.RunningCount() != 0 {
		t.Fatalf("còn %d proc giữ chỗ", sup.RunningCount())
	}
}

// Worker tự dừng (review_wait) trong lúc server đang drain: kết quả vẫn được ghi và
// pending bị xoá, không để lại interrupted kèm pending_action cũ.
func TestNaturalStopDuringDrainIsRecorded(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	id := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: slowPrompt, advance: api.AdvanceReview})
	sup := newSupervisor(t, pool, nil)
	start(t, sup)
	waitReady(t, sup, id)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sup.BeforeShutdown(ctx)
	if b := mustBook(t, pool, id); b.Status != api.StatusInterrupted {
		t.Fatalf("sau BeforeShutdown = %s, muốn interrupted", b.Status)
	}
	// Không gọi Shutdown: worker tự dừng khi xong chương đầu.
	b := waitBook(t, pool, id, api.StatusWaitingReview)
	if b.StopReason != api.StopReviewWait || b.PendingAction != nil || b.ResumeFailures != 0 {
		t.Fatalf("kết quả tự dừng khi drain: %+v", b)
	}
}

// Worker chưa sẵn sàng sau ackTimeout: lệnh chưa được gửi nên phải báo lỗi, không im lặng.
func TestCommandBeforeReadyReturnsNotReady(t *testing.T) {
	old := ackTimeout
	ackTimeout = 50 * time.Millisecond
	t.Cleanup(func() { ackTimeout = old })

	p := newProc("b", "u", nil)
	ack, err := p.command(context.Background(), api.WorkerCommand{Type: api.MsgSteer, ID: "1", Text: "x"})
	if err != ErrNotReady || ack != nil {
		t.Fatalf("command = (%+v, %v), muốn ErrNotReady", ack, err)
	}
}

// stderr của một lượt chạy dài không làm log phình quá giới hạn.
func TestPumpStderrCapsLogSize(t *testing.T) {
	dir := t.TempDir()
	p := newProc("b", "u", nil)
	line := strings.Repeat("x", 1023) + "\n"
	p.pumpStderr(strings.NewReader(strings.Repeat(line, 7<<10)), dir) // ~7MB

	fi, err := os.Stat(filepath.Join(dir, "logs", "worker-stderr.log"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() > stderrLogMax+512 {
		t.Fatalf("log %d byte, vượt giới hạn %d", fi.Size(), stderrLogMax)
	}
	if tail := p.stderrTail(); tail == "" {
		t.Fatal("đuôi stderr trong bộ nhớ vẫn phải được giữ")
	}
}

// Provider từ chối hẳn: giữ lời giải thích của worker (cắt ≤300 rune) và stop_reason riêng;
// mã lỗi khác vẫn dùng message chung, không lộ chi tiết nội bộ.
func TestOutcomeProviderRejected(t *testing.T) {
	const reason = "Z.ai từ chối yêu cầu (mã 1113): key không thuộc gói Coding Plan."
	exit := func(r, stop string) *api.WorkerMessage {
		return &api.WorkerMessage{Type: api.MsgExit, Reason: r, StopReason: stop, Error: "chi tiết nội bộ"}
	}
	cases := []struct {
		name       string
		exit       *api.WorkerMessage
		errCode    string
		errMsg     string
		stopReason string
		lastError  string
	}{
		{"giữa_lượt", exit(api.ExitStopped, api.StopError), api.ErrCodeProviderRejected, reason, api.StopProviderRejected, reason},
		{"lúc_khởi_động", exit(api.ExitError, ""), api.ErrCodeProviderRejected, reason, api.StopProviderRejected, reason},
		{"quá_dài_bị_cắt", exit(api.ExitError, ""), api.ErrCodeProviderRejected, strings.Repeat("ạ", 400), api.StopProviderRejected, strings.Repeat("ạ", 299) + "…"},
		{"đúng_300_rune_giữ_nguyên", exit(api.ExitError, ""), api.ErrCodeProviderRejected, strings.Repeat("ạ", 300), api.StopProviderRejected, strings.Repeat("ạ", 300)},
		{"message_rỗng", exit(api.ExitError, ""), api.ErrCodeProviderRejected, "  ", api.StopError, msgRunFailed},
		{"mã_engine_giữ_message_chung", exit(api.ExitError, ""), api.ErrCodeEngine, "open /data/books/x: permission denied", api.StopError, msgRunFailed},
		{"mã_engine_giữa_lượt", exit(api.ExitStopped, api.StopError), api.ErrCodeEngine, "stack trace", api.StopError, msgRunFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newProc("b", "u", nil)
			p.exit = tc.exit
			p.errCode, p.errMsg = tc.errCode, tc.errMsg
			res, requeue := p.outcome()
			if requeue {
				t.Fatal("không được requeue")
			}
			if res.Status != api.StatusFailed || !res.Failed || res.StopReason != tc.stopReason || res.LastError != tc.lastError {
				t.Fatalf("outcome = {status:%s failed:%v stop:%s last_error:%q}, muốn stop:%s last_error:%q",
					res.Status, res.Failed, res.StopReason, res.LastError, tc.stopReason, tc.lastError)
			}
			if n := len([]rune(res.LastError)); n > 300 {
				t.Fatalf("last_error dài %d rune", n)
			}
		})
	}
}

// Worker bị chia lượt cắt: về queued, giữ việc đang chờ (như drain), không tính
// lỗi — kể cả khi phải SIGKILL. Worker tự kết thúc hoặc user đã bấm tạm dừng thì
// giữ kết quả thật.
func TestOutcomeSliced(t *testing.T) {
	sliced := func(exit *api.WorkerMessage) finishResult {
		p := newProc("b", "u", nil)
		p.sliced.Store(true)
		p.exit = exit
		res, requeue := p.outcome()
		if requeue {
			t.Fatal("lượt bị cắt không đi đường requeue lock_busy")
		}
		return res
	}
	for name, exit := range map[string]*api.WorkerMessage{
		"shutdown": {Type: api.MsgExit, Reason: api.ExitShutdown},
		"sigkill":  nil,
	} {
		res := sliced(exit)
		if res.Status != api.StatusQueued || !res.ResumeNext || res.ClearPending || res.Failed || res.LastError != "" {
			t.Fatalf("%s: lượt bị cắt = %+v, muốn queued, giữ pending, không lỗi", name, res)
		}
	}
	if res := sliced(&api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitStopped, StopReason: api.StopComplete}); res.Status != api.StatusCompleted || res.ResumeNext {
		t.Fatalf("worker tự hoàn thành dù bị cắt phải giữ completed: %+v", res)
	}
	if res := sliced(&api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitShutdown, StopReason: api.StopUser}); res.Status != api.StatusPaused || res.ResumeNext {
		t.Fatalf("user đã bấm tạm dừng thì phải paused: %+v", res)
	}
}

// sliceVictimLocked chỉ chọn worker khi đủ điều kiện; trạng thái dựng tay, không cần DB.
func TestSliceVictimConditions(t *testing.T) {
	now := time.Now()
	newSup := func(maxJobs, perUser int) *Supervisor {
		return New(Config{MaxJobs: maxJobs, MaxJobsPerUser: perUser, MaxRun: time.Minute}, nil, nil, nil, NewHubs())
	}
	addProc := func(s *Supervisor, book, user string, age time.Duration) *proc {
		p := newProc(book, user, nil)
		p.started = now.Add(-age)
		p.markReady()
		s.procs[book] = p
		s.perUser[user]++
		return p
	}
	other := []queuedBook{{ID: "q1", UserID: "B"}}

	s := newSup(1, 1)
	old := addProc(s, "a1", "A", 2*time.Minute)
	if got := s.sliceVictimLocked(other, now); got != old {
		t.Fatalf("đủ điều kiện phải cắt a1, got %v", got)
	}
	if got := s.sliceVictimLocked([]queuedBook{{ID: "q2", UserID: "A"}}, now); got != nil {
		t.Fatal("truyện chờ cùng user không được làm cắt")
	}
	s.cfg.MaxRun = 0
	if got := s.sliceVictimLocked(other, now); got != nil {
		t.Fatal("MaxRun = 0 phải tắt chia lượt")
	}

	s = newSup(2, 1)
	addProc(s, "a1", "A", 30*time.Second)
	if got := s.sliceVictimLocked(other, now); got != nil {
		t.Fatal("lượt chưa quá MaxRun không được cắt")
	}

	// Chọn lượt chạy lâu nhất; user B đã chạm trần thì truyện chờ của B không tính.
	s = newSup(2, 1)
	addProc(s, "a1", "A", 2*time.Minute)
	b1 := addProc(s, "b1", "B", 3*time.Minute)
	if got := s.sliceVictimLocked([]queuedBook{{ID: "q1", UserID: "C"}}, now); got != b1 {
		t.Fatalf("phải cắt lượt lâu nhất b1, got %v", got)
	}
	if got := s.sliceVictimLocked(other, now); got != nil {
		t.Fatal("user của truyện chờ đã chạm MaxJobsPerUser thì không cắt")
	}
	s.backoff["q1"] = backoffState{until: now.Add(time.Minute)}
	if got := s.sliceVictimLocked([]queuedBook{{ID: "q1", UserID: "C"}}, now); got != nil {
		t.Fatal("truyện chờ đang backoff không được làm cắt")
	}
	delete(s.backoff, "q1")

	// Đang có một worker bị cắt chưa thoát: không cắt thêm.
	b1.sliced.Store(true)
	if got := s.sliceVictimLocked([]queuedBook{{ID: "q1", UserID: "C"}}, now); got != nil {
		t.Fatal("đang có worker bị cắt thì không cắt thêm")
	}
}

// slicePrompt: worker giả chạy chậm (~3.9s) để lượt chạy vượt MaxRun.
const slicePrompt = "truyện chậm chia lượt"

// modelChapters trả về số chương trong các event "Viết chương N" đã xong, theo
// thứ tự, kèm vị trí tương đối với status queued đầu tiên.
func modelChapters(t *testing.T, r *recorder) (before, after []int) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	queued := false
	for _, m := range r.msgs {
		switch m.Type {
		case api.SSEStatus:
			var sc api.StatusChange
			if json.Unmarshal(m.Data, &sc) == nil && sc.Status == api.StatusQueued {
				queued = true
			}
		case api.SSEEvent:
			var ev api.Event
			if json.Unmarshal(m.Data, &ev) != nil || ev.Category != "MODEL" || !ev.Finished || ev.Failed {
				continue
			}
			var n int
			if _, err := fmt.Sscanf(ev.Summary, "Viết chương %d", &n); err != nil {
				continue
			}
			if queued {
				after = append(after, n)
			} else {
				before = append(before, n)
			}
		}
	}
	return before, after
}

// Hết slot và có truyện của user khác chờ: lượt quá MaxRun nhường slot, về cuối
// hàng, không tính lỗi, rồi chạy tiếp đúng chương còn dở.
func TestSliceYieldsSlotToOtherUser(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	long := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: slicePrompt})
	waiting := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] user khác đang chờ"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 1; c.MaxRun = time.Second })
	rec := record(t, sup.Hubs().Get(long).Subscribe())
	start(t, sup)

	waitBook(t, pool, waiting, api.StatusCompleted)
	if b := mustBook(t, pool, long); b.Status == api.StatusCompleted {
		t.Fatalf("truyện chờ phải chạy xong trước khi truyện bị cắt hoàn thành")
	}
	b := waitBook(t, pool, long, api.StatusCompleted)
	if b.ResumeFailures != 0 || b.LastError != "" {
		t.Fatalf("bị cắt không được tính lỗi: %+v", b)
	}
	if !isSubsequence(rec.statuses(), []api.BookStatus{api.StatusRunning, api.StatusQueued, api.StatusRunning, api.StatusCompleted}) {
		t.Fatalf("chuỗi status = %v", rec.statuses())
	}
	before, after := modelChapters(t, rec)
	if len(after) == 0 || after[0] != len(before)+1 {
		t.Fatalf("lượt sau phải viết tiếp chương %d: trước %v, sau %v", len(before)+1, before, after)
	}
	if got := append(before, after...); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("mỗi chương phải viết xong đúng một lần, got %v", got)
	}
}

// assertNeverSliced chờ truyện hoàn thành và kiểm nó chưa từng bị đưa về queued.
func assertNeverSliced(t *testing.T, pool *pgxpool.Pool, rec *recorder, id string) {
	t.Helper()
	waitBook(t, pool, id, api.StatusCompleted)
	if slices.Contains(rec.statuses(), api.StatusQueued) {
		t.Fatalf("truyện %s không được bị cắt: %v", id, rec.statuses())
	}
}

// Truyện chờ là của cùng user: không cắt (cắt chỉ đổi thứ tự giữa các truyện của user đó).
func TestSliceSkipsSameUser(t *testing.T) {
	pool := dbtest.New(t)
	uid, kid := newUser(t, pool)
	long := insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: slicePrompt})
	insertBook(t, pool, uid, kid, bookFix{action: api.ActionStart, text: "[fake:fast] cùng user"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 1; c.MaxRun = 200 * time.Millisecond })
	rec := record(t, sup.Hubs().Get(long).Subscribe())
	start(t, sup)
	assertNeverSliced(t, pool, rec, long)
}

// User của truyện chờ đã chạm MaxJobsPerUser: cắt cũng không chạy được nó, nên không cắt.
func TestSliceSkipsUserAtCap(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	longA := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: slicePrompt})
	longB := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: slicePrompt})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 2; c.MaxJobsPerUser = 1; c.MaxRun = 200 * time.Millisecond })
	recA := record(t, sup.Hubs().Get(longA).Subscribe())
	start(t, sup)
	waitBook(t, pool, longA, api.StatusRunning)
	waitBook(t, pool, longB, api.StatusRunning)
	insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] B đã chạm trần"})
	sup.Kick()
	assertNeverSliced(t, pool, recA, longA)
}

// Còn slot trống: truyện chờ tự chạy, không cắt lượt đang quá MaxRun.
func TestSliceSkipsWhenSlotFree(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	long := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: slicePrompt})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 2; c.MaxRun = 200 * time.Millisecond })
	rec := record(t, sup.Hubs().Get(long).Subscribe())
	start(t, sup)
	waitBook(t, pool, long, api.StatusRunning)
	time.Sleep(300 * time.Millisecond) // lượt chạy đã quá MaxRun
	other := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] còn slot"})
	sup.Kick()
	waitBook(t, pool, other, api.StatusCompleted)
	assertNeverSliced(t, pool, rec, long)
}

// Khi cắt, truyện chờ khác của cùng user cũng xuống cuối hàng: slot nhả ra về
// user khác, không quay lại user vừa bị cắt.
func TestSliceDeprioritizesSameUserQueue(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	long := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: slicePrompt})
	nextA := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: "[fake:fast] truyện thứ hai của A"})
	waitingB := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] user B chờ sau"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 1; c.MaxJobsPerUser = 1; c.MaxRun = time.Second })
	start(t, sup)

	for _, id := range []string{long, nextA, waitingB} {
		waitBook(t, pool, id, api.StatusCompleted)
	}
	startedAt := func(id string) time.Time {
		var ts time.Time
		if err := pool.QueryRow(context.Background(), `SELECT started_at FROM books WHERE id = $1::uuid`, id).Scan(&ts); err != nil {
			t.Fatal(err)
		}
		return ts
	}
	if !startedAt(waitingB).Before(startedAt(nextA)) {
		t.Fatal("slot nhả ra phải về user B trước truyện thứ hai của user A")
	}
}

// Lượt bị cắt dừng ở ranh giới chương: về hàng đợi (không phải waiting_review)
// khi user muốn auto, xoá pending vì chương đã commit; snapshot giữ chế độ auto.
// User muốn review thì dừng chờ duyệt là kết quả thật.
func TestOutcomeSlicedAtChapterBoundary(t *testing.T) {
	boundaryWith := func(mode, stop string) finishResult {
		p := newProc("b", "u", nil)
		p.sliced.Store(true)
		p.mode = mode
		p.lastSnap = &api.BookSnapshot{AdvanceMode: api.AdvanceReview}
		p.exit = &api.WorkerMessage{Type: api.MsgExit, Reason: api.ExitStopped, StopReason: stop}
		res, requeue := p.outcome()
		if requeue {
			t.Fatal("dừng ở ranh giới chương không đi đường requeue lock_busy")
		}
		return res
	}
	boundary := func(mode string) finishResult { return boundaryWith(mode, api.StopReviewWait) }
	res := boundary(api.AdvanceAuto)
	if res.Status != api.StatusQueued || res.ResumeNext || !res.ClearPending || res.Failed || res.StopReason != "" || res.LastError != "" {
		t.Fatalf("auto bị cắt ở ranh giới chương = %+v, muốn queued, xoá pending, không lỗi", res)
	}
	var snap api.BookSnapshot
	if err := json.Unmarshal(res.Snapshot, &snap); err != nil || snap.AdvanceMode != api.AdvanceAuto {
		t.Fatalf("snapshot lưu lại phải mang chế độ auto của user: %s", res.Snapshot)
	}
	if res := boundary(api.AdvanceReview); res.Status != api.StatusWaitingReview {
		t.Fatalf("user muốn review thì phải waiting_review: %+v", res)
	}
	// Can thiệp gửi trong lúc ân hạn còn treo ở cổng chương: vẫn là dừng do cắt lượt.
	if res := boundaryWith(api.AdvanceAuto, api.StopPendingSteer); res.Status != api.StatusQueued || res.Failed {
		t.Fatalf("auto bị cắt còn can thiệp treo phải về queued: %+v", res)
	}
}

// ackWriter giả stdin của worker: ghi lại lệnh nhận được và trả ack ok.
type ackWriter struct {
	p   *proc
	got chan api.WorkerCommand
}

func (w ackWriter) Write(b []byte) (int, error) {
	var cmd api.WorkerCommand
	if err := json.Unmarshal(b, &cmd); err != nil {
		return 0, err
	}
	w.got <- cmd
	go w.p.handle(api.WorkerMessage{Type: api.MsgAck, ID: cmd.ID, OK: true}, nil)
	return len(b), nil
}

func (ackWriter) Close() error { return nil }

func newAckProc() (*proc, chan api.WorkerCommand) {
	p := newProc("b", "u", NewHubs().Get("b"))
	got := make(chan api.WorkerCommand, 4)
	p.stdin = ackWriter{p: p, got: got}
	return p, got
}

// Lúc cắt luôn gửi review, kể cả khi user đã chọn review: lệnh đổi chế độ trước đó
// có thể chưa tới worker (worker chưa sẵn sàng, ack lỗi), worker vẫn đang auto.
func TestStopAtChapterBoundaryAlwaysSendsReview(t *testing.T) {
	for _, mode := range []string{api.AdvanceAuto, api.AdvanceReview} {
		p, got := newAckProc()
		p.mode = mode
		p.markReady()
		if !p.stopAtChapterBoundary("s1") {
			t.Fatalf("mode %s: worker nhận lệnh thì phải trả true", mode)
		}
		select {
		case cmd := <-got:
			if cmd.Type != api.MsgSetAdvanceMode || cmd.Mode != api.AdvanceReview {
				t.Fatalf("mode %s: lệnh gửi sai %+v", mode, cmd)
			}
		default:
			t.Fatalf("mode %s: phải gửi set_advance_mode=review", mode)
		}
	}
}

// Đổi chế độ khi worker chưa sẵn sàng không được giữ mmu trong lúc chờ: run()
// cần mmu để gán mode trước khi spawn.
func TestSetAdvanceModeWaitsReadyWithoutLock(t *testing.T) {
	p, got := newAckProc()
	p.mode = api.AdvanceAuto
	type result struct {
		ack *api.WorkerMessage
		err error
	}
	done := make(chan result, 1)
	go func() {
		ack, err := p.setAdvanceMode(context.Background(), api.WorkerCommand{Type: api.MsgSetAdvanceMode, ID: "s1", Mode: api.AdvanceReview})
		done <- result{ack, err}
	}()
	time.Sleep(50 * time.Millisecond)
	locked := make(chan struct{})
	go func() {
		p.mmu.Lock()
		p.mmu.Unlock()
		close(locked)
	}()
	select {
	case <-locked:
	case <-time.After(time.Second):
		t.Fatal("setAdvanceMode giữ mmu trong lúc chờ worker sẵn sàng")
	}
	p.markReady()
	r := <-done
	if r.err != nil || r.ack == nil || !r.ack.OK {
		t.Fatalf("worker sẵn sàng thì lệnh phải tới: ack=%+v err=%v", r.ack, r.err)
	}
	if cmd := <-got; cmd.Mode != api.AdvanceReview || p.advanceMode() != api.AdvanceReview {
		t.Fatalf("lệnh/mode sai: %+v, mode=%q", cmd, p.advanceMode())
	}
}

// Đang cắt lượt: đổi chế độ viết chỉ được ghi nhận, không gửi worker (worker phải
// giữ review để dừng ở ranh giới chương). Không cắt thì gửi như thường.
func TestSetAdvanceModeWhileSlicing(t *testing.T) {
	s := New(Config{MaxJobs: 1, MaxJobsPerUser: 1}, nil, nil, nil, NewHubs())
	p := newProc("b", "u", nil)
	p.mode = api.AdvanceAuto
	p.markReady()
	s.procs["b"] = p
	s.perUser["u"] = 1

	// Chưa cắt: lệnh đi xuống worker (proc không có stdin nên ghi lỗi).
	if _, err := s.Command(context.Background(), "b", api.WorkerCommand{Type: api.MsgSetAdvanceMode, Mode: api.AdvanceReview}); err != ErrNotRunning {
		t.Fatalf("chưa cắt thì phải gửi worker, err=%v", err)
	}
	p.sliced.Store(true)
	ack, err := s.Command(context.Background(), "b", api.WorkerCommand{Type: api.MsgSetAdvanceMode, Mode: api.AdvanceAuto})
	if err != nil || ack == nil || !ack.OK {
		t.Fatalf("đang cắt thì chỉ ghi nhận và trả ack ok: ack=%+v err=%v", ack, err)
	}
	if p.advanceMode() != api.AdvanceAuto {
		t.Fatalf("chế độ user chọn phải được ghi nhận, got %q", p.advanceMode())
	}
}

// chapterStarts đếm số lần mỗi chương bắt đầu viết (event MODEL "Viết chương N"
// chưa xong), kể cả lần bị cắt ngang.
func chapterStarts(r *recorder) map[int]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[int]int{}
	for _, m := range r.msgs {
		var ev api.Event
		if m.Type != api.SSEEvent || json.Unmarshal(m.Data, &ev) != nil || ev.Category != "MODEL" || ev.Finished {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(ev.Summary, "Viết chương %d", &n); err == nil {
			out[n]++
		}
	}
	return out
}

func advanceModeOf(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var mode string
	if err := pool.QueryRow(context.Background(), `SELECT advance_mode FROM books WHERE id = $1::uuid`, id).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	return mode
}

// Có thời gian ân hạn: lượt bị cắt viết nốt chương đang dở rồi mới nhường slot.
// Không chương nào phải viết lại, truyện auto không rơi vào chờ duyệt và lượt sau
// chạy tiếp ở auto tới hết.
func TestSliceWaitsForChapterBoundary(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	long := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: slicePrompt})
	waiting := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] user khác đang chờ"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 1; c.MaxRun = time.Second; c.SliceGrace = time.Minute })
	rec := record(t, sup.Hubs().Get(long).Subscribe())
	start(t, sup)

	waitBook(t, pool, waiting, api.StatusCompleted)
	if b := mustBook(t, pool, long); b.Status == api.StatusCompleted {
		t.Fatalf("truyện chờ phải chạy xong trước khi truyện bị cắt hoàn thành")
	}
	b := waitBook(t, pool, long, api.StatusCompleted)
	if b.ResumeFailures != 0 || b.LastError != "" {
		t.Fatalf("bị cắt không được tính lỗi: %+v", b)
	}
	got := rec.statuses()
	if !isSubsequence(got, []api.BookStatus{api.StatusRunning, api.StatusQueued, api.StatusRunning, api.StatusCompleted}) {
		t.Fatalf("chuỗi status = %v", got)
	}
	if slices.Contains(got, api.StatusWaitingReview) {
		t.Fatalf("truyện auto bị cắt không được chuyển sang chờ duyệt: %v", got)
	}
	if starts := chapterStarts(rec); !maps.Equal(starts, map[int]int{1: 1, 2: 1, 3: 1}) {
		t.Fatalf("mỗi chương phải viết đúng một lần, không bỏ chương dở: %v", starts)
	}
	if mode := advanceModeOf(t, pool, long); mode != api.AdvanceAuto {
		t.Fatalf("chế độ của truyện phải giữ auto, got %q", mode)
	}
}

// Hết thời gian ân hạn mà chương chưa xong: dừng hẳn như khi không có ân hạn,
// chương dở được viết lại ở lượt sau.
func TestSliceGraceExpiredTerminates(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	long := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: "[fake:slow] chương rất dài"})
	waiting := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] user khác đang chờ"})
	sup := newSupervisor(t, pool, func(c *Config) {
		c.MaxJobs = 1
		c.MaxRun = 500 * time.Millisecond
		c.SliceGrace = 300 * time.Millisecond
	})
	rec := record(t, sup.Hubs().Get(long).Subscribe())
	start(t, sup)

	waitBook(t, pool, waiting, api.StatusCompleted)
	waitFor(t, "lượt sau viết lại chương bị cắt dở", func() bool { return chapterStarts(rec)[1] == 2 })
	if b := mustBook(t, pool, long); b.ResumeFailures != 0 || b.LastError != "" {
		t.Fatalf("bị cắt không được tính lỗi: %+v", b)
	}
	if !isSubsequence(rec.statuses(), []api.BookStatus{api.StatusRunning, api.StatusQueued, api.StatusRunning}) {
		t.Fatalf("chuỗi status = %v", rec.statuses())
	}
}

// User bấm tạm dừng trong lúc worker đang chờ tới ranh giới chương: giữ paused,
// không bị lượt cắt đưa về hàng đợi.
func TestPauseWhileSlicingKeepsPaused(t *testing.T) {
	pool := dbtest.New(t)
	uidA, kidA := newUser(t, pool)
	uidB, kidB := newUser(t, pool)
	long := insertBook(t, pool, uidA, kidA, bookFix{action: api.ActionStart, text: "[fake:slow] chương rất dài"})
	waiting := insertBook(t, pool, uidB, kidB, bookFix{action: api.ActionStart, text: "[fake:fast] user khác đang chờ"})
	sup := newSupervisor(t, pool, func(c *Config) { c.MaxJobs = 1; c.MaxRun = 500 * time.Millisecond; c.SliceGrace = time.Minute })
	rec := record(t, sup.Hubs().Get(long).Subscribe())
	start(t, sup)

	waitFor(t, "lượt chạy bị cắt", func() bool { return sup.IsSliced(long) })
	abortBook(t, sup, long)
	b := waitBook(t, pool, long, api.StatusPaused)
	if b.StopReason != api.StopUser {
		t.Fatalf("tạm dừng phải ghi stop_reason user: %+v", b)
	}
	waitBook(t, pool, waiting, api.StatusCompleted)
	if b := mustBook(t, pool, long); b.Status != api.StatusPaused {
		t.Fatalf("truyện user đã tạm dừng không được tự chạy lại: %s", b.Status)
	}
	if slices.Contains(rec.statuses(), api.StatusQueued) {
		t.Fatalf("tạm dừng trong lúc cắt không được về hàng đợi: %v", rec.statuses())
	}
}
