package jobs

import (
	"context"
	"encoding/json"
	"fmt"
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

func (fakePricing) PricingFor(model string) (api.ModelPricing, bool) {
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
		{"error_with_message", exit(api.ExitStopped, api.StopError, "lỗi engine"), "", api.StatusFailed, true, false, "lỗi engine"},
		{"unknown_stop_reason", exit(api.ExitStopped, "khong_ro", ""), "", api.StatusFailed, true, false, "Engine dừng vì lỗi"},
		{"exit_error", exit(api.ExitError, "", "boom"), "", api.StatusFailed, true, false, "boom"},
		{"exit_shutdown", exit(api.ExitShutdown, "", ""), "", api.StatusPaused, false, false, ""},
		{"no_exit_message", nil, "", api.StatusFailed, true, false, "Worker dừng bất thường"},
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
			if !res.ClearPending {
				t.Fatal("ClearPending phải true sau mọi lượt worker kết thúc")
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
	if b.StopReason != api.StopError || b.LastError != "lỗi giả lập" || b.ResumeFailures != 1 {
		t.Fatalf("error: %+v", b)
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
	if !containsAll(b.LastError, "giả lập sự cố", "panic: fake crash", "goroutine 1") {
		t.Fatalf("last_error thiếu đuôi stderr: %q", b.LastError)
	}
	logPath := filepath.Join(mustBookDir(t, sup, uid, id), "logs", "worker-stderr.log")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("đọc worker-stderr.log: %v", err)
	}
	if !containsAll(string(logData), "giả lập sự cố", "goroutine 1") {
		t.Fatalf("worker-stderr.log thiếu nội dung stderr: %q", logData)
	}
	if containsAll(b.LastError, testSecret) || containsAll(string(logData), testSecret) {
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
