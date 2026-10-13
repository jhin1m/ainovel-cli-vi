package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/db"
	"github.com/voocel/ainovel-cli/internal/server/db/dbtest"
	"github.com/voocel/ainovel-cli/internal/server/jobs"
	"github.com/voocel/ainovel-cli/internal/server/keys"
)

// Luồng truyện end-to-end qua HTTP, chạy worker giả (jobs/fakeworker) thay cho LLM.
// Cần AINOVEL_TEST_DATABASE_URL; thiếu biến này thì các test bị bỏ qua.

// fakeWorkerBin là đường dẫn binary worker giả, build một lần trong TestMain.
var fakeWorkerBin string

func TestMain(m *testing.M) {
	if os.Getenv(dbtest.EnvURL) == "" {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "ainovel-fakeworker-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tạo thư mục build worker giả:", err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "fakeworker")
	if out, err := exec.Command("go", "build", "-o", bin, "./jobs/fakeworker").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build worker giả thất bại: %v\n%s", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	fakeWorkerBin = bin
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

const (
	testModel   = "openai/gpt-4.1-mini" // có trong danh sách chọn lọc của openrouter và có giá
	flowTimeout = 30 * time.Second
	slowPrompt  = "Truyện chậm về một con tàu lạc bến"
)

// fastPrompt thêm marker để worker giả viết mỗi chương ~50ms.
func fastPrompt(s string) string { return s + " [fake:fast]" }

// newBookEnv dựng server thật với worker giả và bật scheduler (newTestEnv không làm việc đó).
func newBookEnv(t *testing.T, mutate func(*Config)) *testEnv {
	t.Helper()
	env := newTestEnv(t, func(c *Config) {
		c.DevWorkerBin = fakeWorkerBin
		if mutate != nil {
			mutate(c)
		}
	})
	if err := env.srv.startBackground(); err != nil {
		t.Fatal(err)
	}
	return env
}

// login đăng ký một user mới bằng invite riêng và trả về client đã có cookie phiên.
func (e *testEnv) login(email string) *client {
	e.t.Helper()
	c := e.newClient()
	c.signup(e.invite(1), email, testPassword)
	return c
}

// newClientAt tạo client trỏ tới base khác (dùng khi server được dựng lại).
func newClientAt(t *testing.T, base string) *client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: base, hc: &http.Client{Jar: jar}}
}

func (c *client) addKey(provider string) api.APIKey {
	c.t.Helper()
	r := c.do("POST", "/api/keys", map[string]string{
		"provider": provider, "label": "thử", "api_key": testLLMKey, "current_password": testPassword,
	})
	var k api.APIKey
	r.decode(c.t, &k)
	if r.status != http.StatusCreated {
		c.t.Fatalf("thêm key %s: %d %s", provider, r.status, r.body)
	}
	return k
}

func (c *client) createBook(prompt, keyID, model string, budget float64, mode string) api.Book {
	c.t.Helper()
	r := c.do("POST", "/api/books", api.CreateBookRequest{
		Prompt: prompt, APIKeyID: keyID, Model: model, BudgetUSD: budget, AdvanceMode: mode,
	})
	var b api.Book
	r.decode(c.t, &b)
	if r.status != http.StatusCreated {
		c.t.Fatalf("tạo truyện: %d %s", r.status, r.body)
	}
	return b
}

func (c *client) getBook(id string) api.Book {
	c.t.Helper()
	r := c.do("GET", "/api/books/"+id, nil)
	var b api.Book
	r.decode(c.t, &b)
	if r.status != http.StatusOK {
		c.t.Fatalf("lấy truyện %s: %d %s", id, r.status, r.body)
	}
	return b
}

func (c *client) listBooks() []api.Book {
	c.t.Helper()
	var list []api.Book
	r := c.do("GET", "/api/books", nil)
	r.decode(c.t, &list)
	if r.status != http.StatusOK {
		c.t.Fatalf("danh sách truyện: %d %s", r.status, r.body)
	}
	return list
}

// waitBook poll GET tới khi cond đúng; quá hạn thì dừng test kèm trạng thái cuối.
func (c *client) waitBook(id, desc string, cond func(api.Book) bool) api.Book {
	c.t.Helper()
	deadline := time.Now().Add(flowTimeout)
	for {
		b := c.getBook(id)
		if cond(b) {
			return b
		}
		if time.Now().After(deadline) {
			c.t.Fatalf("hết thời gian chờ %s; trạng thái cuối %s (stop=%q, lỗi=%q)", desc, b.Status, b.StopReason, b.LastError)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (c *client) waitStatus(id string, st api.BookStatus) api.Book {
	c.t.Helper()
	return c.waitBook(id, "trạng thái "+string(st), func(b api.Book) bool { return b.Status == st })
}

// waitWriting chờ worker chạy xong phần khởi tạo và bắt đầu viết chương.
func (c *client) waitWriting(id string) api.Book {
	c.t.Helper()
	return c.waitBook(id, "đang viết chương", func(b api.Book) bool {
		return b.Status == api.StatusRunning && b.Snapshot != nil && b.Snapshot.RuntimeState == "running"
	})
}

func (c *client) chapterCount(id string) int {
	c.t.Helper()
	var list []api.ChapterInfo
	r := c.do("GET", "/api/books/"+id+"/chapters", nil)
	r.decode(c.t, &list)
	if r.status != http.StatusOK {
		c.t.Fatalf("danh sách chương: %d %s", r.status, r.body)
	}
	return len(list)
}

// bookDirOf trả về thư mục truyện của user đang đăng nhập (suy từ DATA_DIR như server).
func (e *testEnv) bookDirOf(c *client, bookID string) string {
	e.t.Helper()
	var me api.User
	c.do("GET", "/api/me", nil).decode(e.t, &me)
	dir, err := jobs.BookDir(e.srv.cfg.DataDir, me.ID, bookID)
	if err != nil {
		e.t.Fatal(err)
	}
	return dir
}

// eventually poll điều kiện tới khi đúng hoặc hết hạn.
func eventually(t *testing.T, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(flowTimeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("hết thời gian chờ: %s", desc)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func statusIs(st api.BookStatus) func(api.Book) bool {
	return func(b api.Book) bool { return b.Status == st }
}

// ── SSE ──

type sseFrame struct {
	Type string
	Data string
}

// openSSE mở một kết nối SSE; goroutine đọc đẩy frame vào kênh. Kết nối đóng khi test kết thúc.
func (c *client) openSSE(path string) (*http.Response, <-chan sseFrame) {
	c.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c.t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	frames := make(chan sseFrame, 1024)
	go func() {
		defer close(frames)
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		var f sseFrame
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if f.Type != "" {
					select {
					case frames <- f:
					case <-ctx.Done():
						return
					}
				}
				f = sseFrame{}
			case strings.HasPrefix(line, "event: "):
				f.Type = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				f.Data = strings.TrimPrefix(line, "data: ")
			case strings.HasPrefix(line, "retry: "):
				f = sseFrame{Type: "retry", Data: strings.TrimPrefix(line, "retry: ")}
			}
		}
	}()
	return resp, frames
}

func nextFrame(t *testing.T, frames <-chan sseFrame) sseFrame {
	t.Helper()
	select {
	case f, ok := <-frames:
		if !ok {
			t.Fatal("kết nối SSE đã đóng sớm")
		}
		return f
	case <-time.After(flowTimeout):
		t.Fatal("hết thời gian chờ frame SSE")
	}
	return sseFrame{}
}

// untilFrame đọc tới khi gặp frame đúng kiểu và thoả match (nil = bất kỳ).
func untilFrame(t *testing.T, frames <-chan sseFrame, typ string, match func(string) bool) sseFrame {
	t.Helper()
	for {
		f := nextFrame(t, frames)
		if f.Type == typ && (match == nil || match(f.Data)) {
			return f
		}
	}
}

func statusFrameIs(st api.BookStatus) func(string) bool {
	return func(data string) bool {
		var sc api.StatusChange
		return json.Unmarshal([]byte(data), &sc) == nil && sc.Status == st
	}
}

// expectHead kiểm thứ tự frame khi kết nối: retry → snapshot → status.
func expectHead(t *testing.T, frames <-chan sseFrame, st api.BookStatus) {
	t.Helper()
	if f := nextFrame(t, frames); f.Type != "retry" || f.Data != "3000" {
		t.Fatalf("frame đầu phải retry 3000: %+v", f)
	}
	if f := nextFrame(t, frames); f.Type != api.SSESnapshot {
		t.Fatalf("frame thứ hai phải snapshot: %+v", f)
	}
	if f := nextFrame(t, frames); f.Type != api.SSEStatus || !statusFrameIs(st)(f.Data) {
		t.Fatalf("frame thứ ba phải status %s: %+v", st, f)
	}
}

// ── server chạy thật để kiểm drain/khôi phục ──

// serveTest chạy Serve trên listener tạm. Trả về địa chỉ gốc và hàm tắt (chờ Serve trả về).
func serveTest(t *testing.T, s *Server) (string, func() error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	var once sync.Once
	var serveErr error
	stop := func() error {
		once.Do(func() {
			cancel()
			select {
			case serveErr = <-done:
			case <-time.After(90 * time.Second):
				serveErr = errors.New("server không tắt sau 90s")
			}
		})
		return serveErr
	}
	t.Cleanup(func() { _ = stop() })
	return "http://" + ln.Addr().String(), stop
}

// dbBookStatus đọc status thô trong DB (sau khi server đã tắt, dùng pool riêng).
func dbBookStatus(t *testing.T, url, id string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var st string
	if err := pool.QueryRow(ctx, `SELECT status FROM books WHERE id::text = $1`, id).Scan(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

// ── các test ──

func TestBookReviewFlow(t *testing.T) {
	logs := captureLogs(t)
	env := newBookEnv(t, nil)
	c := env.login("an@example.com")
	key := c.addKey("openrouter")

	b := c.createBook(fastPrompt("Dòng sông"), key.ID, testModel, 5, api.AdvanceReview)
	if b.Status != api.StatusQueued || b.PendingText == "" {
		t.Fatalf("vừa tạo phải queued kèm việc chờ: %+v", b)
	}
	c.waitStatus(b.ID, api.StatusWaitingReview)
	if n := c.chapterCount(b.ID); n != 1 {
		t.Fatalf("sau lượt đầu phải có 1 chương, được %d", n)
	}

	var ch api.ChapterText
	r := c.do("GET", "/api/books/"+b.ID+"/chapters/1", nil)
	r.decode(t, &ch)
	// Chỉ có chương 1: không có chương trước và chương sau.
	if r.status != 200 || ch.Number != 1 || !strings.Contains(ch.Content, "Minh bước chậm rãi") || ch.Prev != 0 || ch.Next != 0 {
		t.Fatalf("đọc chương 1: %d %+v", r.status, ch)
	}
	if r := c.do("GET", "/api/books/"+b.ID+"/chapters/2", nil); r.status != http.StatusNotFound || r.errCode() != "chapter_not_found" {
		t.Fatalf("chương chưa viết phải 404: %d %s", r.status, r.body)
	}
	if r := c.do("GET", "/api/books/"+b.ID+"/chapters/abc", nil); r.status != http.StatusNotFound {
		t.Fatalf("chỉ số chương không hợp lệ phải 404: %d", r.status)
	}

	var ol api.Outline
	if r := c.do("GET", "/api/books/"+b.ID+"/outline", nil); r.status != 200 {
		t.Fatalf("outline: %d %s", r.status, r.body)
	} else {
		r.decode(t, &ol)
	}
	total := 0
	for _, v := range ol.Volumes {
		total += len(v.Chapters)
	}
	if ol.Title == "" || total != 3 {
		t.Fatalf("outline phải có tiêu đề và 3 chương: %+v", ol)
	}

	// Steer khi đang dừng: xếp hàng rồi chạy, viết thêm đúng một chương.
	if r := c.do("POST", "/api/books/"+b.ID+"/steer", api.SteerRequest{Text: "   "}); r.status != 400 || r.errCode() != "invalid_text" {
		t.Fatalf("steer rỗng phải 400: %d %s", r.status, r.body)
	}
	r = c.do("POST", "/api/books/"+b.ID+"/steer", api.SteerRequest{Text: "Cho Lan nói nhiều hơn"})
	var cr api.CommandResult
	r.decode(t, &cr)
	if r.status != http.StatusOK || !cr.Queued || cr.Book.Status != api.StatusQueued {
		t.Fatalf("steer khi dừng phải 200 queued: %d %s", r.status, r.body)
	}
	c.waitStatus(b.ID, api.StatusWaitingReview)
	if n := c.chapterCount(b.ID); n != 2 {
		t.Fatalf("steer phải viết thêm chương 2, có %d chương", n)
	}

	// Chương tiếp: chương 3 là chương cuối nên truyện hoàn thành.
	if r := c.do("POST", "/api/books/"+b.ID+"/next", nil); r.status != 200 {
		t.Fatalf("next: %d %s", r.status, r.body)
	}
	c.waitStatus(b.ID, api.StatusCompleted)
	if n := c.chapterCount(b.ID); n != 3 {
		t.Fatalf("truyện xong phải có 3 chương, có %d", n)
	}
	if r := c.do("POST", "/api/books/"+b.ID+"/next", nil); r.status != http.StatusConflict || r.errCode() != "not_waiting_review" {
		t.Fatalf("next khi đã hoàn thành phải 409: %d %s", r.status, r.body)
	}
	if strings.Contains(logs.String(), testLLMKey) {
		t.Fatal("log chứa API key")
	}
}

func TestBookSteerAndPauseWhileRunning(t *testing.T) {
	logs := captureLogs(t)
	env := newBookEnv(t, nil)
	c := env.login("an@example.com")
	key := c.addKey("openrouter")

	// Không có marker fast: mỗi chương ~1.2s, đủ thời gian để can thiệp khi đang chạy.
	b := c.createBook(slowPrompt, key.ID, testModel, 5, api.AdvanceAuto)
	c.waitWriting(b.ID)

	r := c.do("POST", "/api/books/"+b.ID+"/steer", api.SteerRequest{Text: "Thêm chi tiết về con sông"})
	if r.status != http.StatusOK && r.status != http.StatusAccepted {
		t.Fatalf("steer khi đang chạy phải 200 hoặc 202: %d %s", r.status, r.body)
	}
	var cr api.CommandResult
	r.decode(t, &cr)
	if cr.Queued {
		t.Fatalf("steer khi đang chạy không được báo queued: %s", r.body)
	}
	if r := c.do("PATCH", "/api/books/"+b.ID, api.UpdateBookRequest{BudgetUSD: ptrFloat(9)}); r.status != http.StatusConflict || r.errCode() != "book_running" {
		t.Fatalf("đổi ngân sách khi đang chạy phải 409 book_running: %d %s", r.status, r.body)
	}

	r = c.do("POST", "/api/books/"+b.ID+"/pause", nil)
	if r.status != http.StatusOK && r.status != http.StatusAccepted {
		t.Fatalf("pause khi đang chạy phải 200 hoặc 202: %d %s", r.status, r.body)
	}
	pb := c.waitStatus(b.ID, api.StatusPaused)
	if pb.StopReason != "user" {
		t.Fatalf("pause phải có stop_reason user: %q", pb.StopReason)
	}
	if r := c.do("POST", "/api/books/"+b.ID+"/pause", nil); r.status != http.StatusConflict || r.errCode() != "not_running" {
		t.Fatalf("pause khi đã dừng phải 409: %d %s", r.status, r.body)
	}

	if r := c.do("POST", "/api/books/"+b.ID+"/resume", nil); r.status != 200 {
		t.Fatalf("resume: %d %s", r.status, r.body)
	}
	c.waitWriting(b.ID)
	c.waitStatus(b.ID, api.StatusCompleted)
	if n := c.chapterCount(b.ID); n != 3 {
		t.Fatalf("sau khi tiếp tục phải đủ 3 chương, có %d", n)
	}
	if strings.Contains(logs.String(), testLLMKey) {
		t.Fatal("log chứa API key")
	}
}

func TestBookBudget(t *testing.T) {
	logs := captureLogs(t)
	env := newBookEnv(t, nil)
	c := env.login("an@example.com")
	key := c.addKey("openrouter")

	// Mỗi chương 0.40; ngân sách 0.50 chỉ đủ một chương.
	b := c.createBook(fastPrompt("Ngân sách eo hẹp"), key.ID, testModel, 0.5, api.AdvanceAuto)
	c.waitStatus(b.ID, api.StatusBudgetExhausted)
	if r := c.do("POST", "/api/books/"+b.ID+"/resume", nil); r.status != http.StatusConflict || r.errCode() != "budget_exhausted" {
		t.Fatalf("resume khi hết ngân sách phải 409: %d %s", r.status, r.body)
	}

	spent := c.getBook(b.ID).Snapshot.TotalCostUSD
	if r := c.do("PATCH", "/api/books/"+b.ID, api.UpdateBookRequest{BudgetUSD: ptrFloat(spent)}); r.status != 400 || r.errCode() != "budget_too_low" {
		t.Fatalf("ngân sách bằng số đã dùng phải 400: %d %s", r.status, r.body)
	}
	if r := c.do("PATCH", "/api/books/"+b.ID, api.UpdateBookRequest{BudgetUSD: ptrFloat(51)}); r.status != 400 || r.errCode() != "invalid_budget" {
		t.Fatalf("ngân sách vượt trần phải 400: %d %s", r.status, r.body)
	}
	if r := c.do("PATCH", "/api/books/"+b.ID, api.UpdateBookRequest{BudgetUSD: ptrFloat(0)}); r.status != 400 {
		t.Fatalf("ngân sách 0 phải 400: %d", r.status)
	}

	r := c.do("PATCH", "/api/books/"+b.ID, api.UpdateBookRequest{BudgetUSD: ptrFloat(2)})
	var up api.Book
	r.decode(t, &up)
	if r.status != 200 || up.Status != api.StatusPaused || up.BudgetUSD != 2 {
		t.Fatalf("nâng ngân sách phải chuyển paused: %d %s", r.status, r.body)
	}
	if r := c.do("POST", "/api/books/"+b.ID+"/resume", nil); r.status != 200 {
		t.Fatalf("resume sau khi nâng ngân sách: %d %s", r.status, r.body)
	}
	c.waitStatus(b.ID, api.StatusCompleted)
	if n := c.chapterCount(b.ID); n != 3 {
		t.Fatalf("sau khi nâng ngân sách phải đủ 3 chương, có %d", n)
	}
	if strings.Contains(logs.String(), testLLMKey) {
		t.Fatal("log chứa API key")
	}
}

func TestBookValidationAndLimits(t *testing.T) {
	env := newBookEnv(t, func(c *Config) { c.Limits.MaxBooksPerUser = 1 })
	c := env.login("an@example.com")
	or := c.addKey("openrouter")
	an := c.addKey("anthropic")

	// valid là yêu cầu hợp lệ; mỗi case chỉ đổi đúng một trường.
	valid := api.CreateBookRequest{Prompt: "x", APIKeyID: or.ID, Model: testModel, BudgetUSD: 5, AdvanceMode: api.AdvanceReview}
	with := func(f func(*api.CreateBookRequest)) api.CreateBookRequest {
		r := valid
		f(&r)
		return r
	}

	cases := []struct {
		name   string
		req    api.CreateBookRequest
		status int
		code   string
	}{
		{"prompt rỗng", with(func(r *api.CreateBookRequest) { r.Prompt = "   " }), 400, "invalid_prompt"},
		{"prompt quá 4000 ký tự", with(func(r *api.CreateBookRequest) { r.Prompt = strings.Repeat("ả", 4001) }), 400, "invalid_prompt"},
		{"chế độ viết lạ", with(func(r *api.CreateBookRequest) { r.AdvanceMode = "turbo" }), 400, "invalid_advance_mode"},
		{"ngân sách bằng 0", with(func(r *api.CreateBookRequest) { r.BudgetUSD = 0 }), 400, "invalid_budget"},
		{"ngân sách vượt trần", with(func(r *api.CreateBookRequest) { r.BudgetUSD = 51 }), 400, "invalid_budget"},
		{"model ngoài danh sách", with(func(r *api.CreateBookRequest) { r.Model = "openai/gpt-4o" }), 422, "model_not_allowed"},
		{"model của provider khác key", with(func(r *api.CreateBookRequest) { r.APIKeyID = an.ID }), 422, "model_not_allowed"},
		{"key không tồn tại", with(func(r *api.CreateBookRequest) { r.APIKeyID = "00000000-0000-4000-8000-000000000000" }), 404, "key_not_found"},
		{"key sai định dạng", with(func(r *api.CreateBookRequest) { r.APIKeyID = "khong-phai-uuid" }), 404, "key_not_found"},
	}
	for _, tc := range cases {
		r := c.do("POST", "/api/books", tc.req)
		if r.status != tc.status || r.errCode() != tc.code {
			t.Fatalf("%s: phải %d %s, nhận %d %s", tc.name, tc.status, tc.code, r.status, r.body)
		}
	}

	b := c.createBook(fastPrompt("Giới hạn một truyện"), or.ID, testModel, 5, api.AdvanceReview)
	if r := c.do("POST", "/api/books", valid); r.status != http.StatusConflict || r.errCode() != "book_limit" {
		t.Fatalf("vượt MaxBooksPerUser phải 409: %d %s", r.status, r.body)
	}
	c.waitStatus(b.ID, api.StatusWaitingReview)
}

func ptrFloat(v float64) *float64 { return &v }

func TestBookExport(t *testing.T) {
	env := newBookEnv(t, nil)
	c := env.login("an@example.com")
	key := c.addKey("openrouter")
	b := c.createBook(fastPrompt("Chuyến tàu đêm"), key.ID, testModel, 5, api.AdvanceAuto)
	c.waitStatus(b.ID, api.StatusCompleted)

	if r := c.do("GET", "/api/books/"+b.ID+"/export?format=pdf", nil); r.status != 400 || r.errCode() != "invalid_format" {
		t.Fatalf("định dạng lạ phải 400: %d %s", r.status, r.body)
	}

	txt := c.do("GET", "/api/books/"+b.ID+"/export?format=txt", nil)
	if txt.status != 200 {
		t.Fatalf("export txt: %d %s", txt.status, txt.body)
	}
	if cd := txt.header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, "filename") {
		t.Fatalf("Content-Disposition không đúng: %q", cd)
	}
	if !strings.HasPrefix(txt.header.Get("Content-Type"), "text/plain") {
		t.Fatalf("Content-Type txt: %q", txt.header.Get("Content-Type"))
	}
	if !strings.Contains(string(txt.body), "Minh bước chậm rãi") {
		t.Fatal("file txt thiếu nội dung chương")
	}

	exportsDir := filepath.Join(env.bookDirOf(c, b.ID), "exports")
	txtPath := filepath.Join(exportsDir, "3.txt")
	fi1, err := os.Stat(txtPath)
	if err != nil {
		t.Fatalf("cache txt chưa được ghi: %v", err)
	}
	// Lần hai phải dùng lại cache: file không bị ghi lại.
	again := c.do("GET", "/api/books/"+b.ID+"/export?format=txt", nil)
	fi2, err := os.Stat(txtPath)
	if again.status != 200 || err != nil || !fi2.ModTime().Equal(fi1.ModTime()) || string(again.body) != string(txt.body) {
		t.Fatalf("export lần hai phải dùng cache: %d, mtime %v -> %v", again.status, fi1.ModTime(), fi2.ModTime())
	}

	epub := c.do("GET", "/api/books/"+b.ID+"/export?format=epub", nil)
	if epub.status != 200 || !strings.HasPrefix(string(epub.body), "PK") {
		t.Fatalf("export epub không phải zip: %d", epub.status)
	}
	if ct := epub.header.Get("Content-Type"); ct != "application/epub+zip" {
		t.Fatalf("Content-Type epub: %q", ct)
	}
	entries, err := os.ReadDir(exportsDir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "3.epub,3.txt" {
		t.Fatalf("thư mục xuất phải chỉ có file cache và không còn file tạm: %v", names)
	}
}

func TestBookExportBeforeAnyChapter(t *testing.T) {
	env := newBookEnv(t, nil)
	c := env.login("an@example.com")
	key := c.addKey("openrouter")
	// Chương đầu của truyện chậm cần hơn một giây, nên lúc này chưa có chương nào hoàn thành.
	b := c.createBook(slowPrompt, key.ID, testModel, 5, api.AdvanceReview)
	if r := c.do("GET", "/api/books/"+b.ID+"/export?format=epub", nil); r.status != http.StatusConflict || r.errCode() != "nothing_to_export" {
		t.Fatalf("export khi chưa có chương phải 409 nothing_to_export: %d %s", r.status, r.body)
	}
	c.waitStatus(b.ID, api.StatusWaitingReview)
}

func TestBookAuthorization(t *testing.T) {
	env := newBookEnv(t, nil)
	a := env.login("an@example.com")
	bUser := env.login("binh@example.com")
	key := a.addKey("openrouter")
	bk := a.createBook(fastPrompt("Bí mật của An"), key.ID, testModel, 5, api.AdvanceReview)
	a.waitStatus(bk.ID, api.StatusWaitingReview)
	base := "/api/books/" + bk.ID

	routes := []struct {
		method, path string
		body         any
	}{
		{"GET", base, nil},
		{"PATCH", base, api.UpdateBookRequest{BudgetUSD: ptrFloat(9)}},
		{"DELETE", base, nil},
		{"POST", base + "/pause", nil},
		{"POST", base + "/resume", nil},
		{"POST", base + "/steer", api.SteerRequest{Text: "xen vào"}},
		{"POST", base + "/next", nil},
		{"GET", base + "/chapters", nil},
		{"GET", base + "/chapters/1", nil},
		{"GET", base + "/outline", nil},
		{"GET", base + "/events", nil},
		{"GET", base + "/export?format=txt", nil},
	}
	for _, rt := range routes {
		if r := bUser.do(rt.method, rt.path, rt.body); r.status != http.StatusNotFound {
			t.Fatalf("%s %s của người khác phải 404: %d %s", rt.method, rt.path, r.status, r.body)
		}
	}
	if r := bUser.do("POST", "/api/books", api.CreateBookRequest{Prompt: "x", APIKeyID: key.ID, Model: testModel, BudgetUSD: 1, AdvanceMode: api.AdvanceAuto}); r.status != http.StatusNotFound || r.errCode() != "key_not_found" {
		t.Fatalf("tạo truyện với key của người khác phải 404: %d %s", r.status, r.body)
	}
	if list := bUser.listBooks(); len(list) != 0 {
		t.Fatalf("B không được thấy truyện nào, danh sách có %d truyện", len(list))
	}

	// Truyện của A không bị ảnh hưởng bởi các lệnh của B.
	if got := a.getBook(bk.ID); got.Status != api.StatusWaitingReview || got.BudgetUSD != 5 {
		t.Fatalf("truyện của A bị thay đổi: %+v", got)
	}
}

func TestBookDelete(t *testing.T) {
	env := newBookEnv(t, nil)
	c := env.login("an@example.com")
	key := c.addKey("openrouter")

	// Truyện đang chạy: phải dừng worker rồi mới xoá thư mục.
	slow := c.createBook(slowPrompt, key.ID, testModel, 5, api.AdvanceAuto)
	c.waitWriting(slow.ID)
	slowDir := env.bookDirOf(c, slow.ID)

	// Key đang được truyện đang chạy dùng → 409.
	if r := c.do("DELETE", "/api/keys/"+key.ID, nil); r.status != http.StatusConflict || r.errCode() != "key_in_use" {
		t.Fatalf("xoá key đang dùng phải 409: %d %s", r.status, r.body)
	}

	if r := c.do("DELETE", "/api/books/"+slow.ID, nil); r.status != http.StatusAccepted {
		t.Fatalf("xoá truyện đang chạy phải 202: %d %s", r.status, r.body)
	}
	for _, bk := range c.listBooks() {
		if bk.ID == slow.ID {
			t.Fatal("truyện đang xoá vẫn nằm trong danh sách")
		}
	}
	eventually(t, "thư mục truyện đang chạy được xoá", func() bool {
		_, err := os.Stat(slowDir)
		return os.IsNotExist(err)
	})
	eventually(t, "worker của truyện đã thoát", func() bool { return env.srv.sup.RunningCount() == 0 })
	if r := c.do("GET", "/api/books/"+slow.ID, nil); r.status != http.StatusNotFound {
		t.Fatalf("truyện đã xoá phải 404: %d", r.status)
	}

	// Sau khi xoá, key không còn bị khoá.
	if r := c.do("DELETE", "/api/keys/"+key.ID, nil); r.status != http.StatusNoContent {
		t.Fatalf("key không còn truyện dùng phải xoá được: %d %s", r.status, r.body)
	}
}

func TestBookEventsStream(t *testing.T) {
	logs := captureLogs(t)
	env := newBookEnv(t, nil)
	c := env.login("an@example.com")
	key := c.addKey("openrouter")
	b := c.createBook(fastPrompt("Dòng sông lặng"), key.ID, testModel, 5, api.AdvanceReview)
	c.waitStatus(b.ID, api.StatusWaitingReview)
	path := "/api/books/" + b.ID + "/events"

	resp, frames := c.openSSE(path)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type SSE: %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("Cache-Control SSE: %q", cc)
	}
	if xa := resp.Header.Get("X-Accel-Buffering"); xa != "no" {
		t.Fatalf("X-Accel-Buffering: %q", xa)
	}
	if ce := resp.Header.Get("Content-Encoding"); ce != "" {
		t.Fatalf("SSE không được nén: %q", ce)
	}
	expectHead(t, frames, api.StatusWaitingReview)
	// Timeline đã xong: sau trạng thái là các event trong ring.
	ev := untilFrame(t, frames, api.SSEEvent, nil)
	if !strings.Contains(ev.Data, `"id"`) {
		t.Fatalf("event thiếu id: %s", ev.Data)
	}

	// Kết nối thứ hai của cùng truyện cũng nhận được cập nhật trực tiếp.
	_, frames2 := c.openSSE(path)
	expectHead(t, frames2, api.StatusWaitingReview)

	if r := c.do("POST", "/api/books/"+b.ID+"/next", nil); r.status != 200 {
		t.Fatalf("next: %d %s", r.status, r.body)
	}
	for _, fr := range []<-chan sseFrame{frames, frames2} {
		untilFrame(t, fr, api.SSEStatus, statusFrameIs(api.StatusQueued))
		untilFrame(t, fr, api.SSEStatus, statusFrameIs(api.StatusWaitingReview))
	}
	c.waitStatus(b.ID, api.StatusWaitingReview)
	if strings.Contains(logs.String(), testLLMKey) {
		t.Fatal("log chứa API key")
	}
}

func TestBookEventsLimitPerUser(t *testing.T) {
	env := newBookEnv(t, func(c *Config) { c.Limits.MaxSSEPerUser = 1 })
	c := env.login("an@example.com")
	key := c.addKey("openrouter")
	b := c.createBook(fastPrompt("Giới hạn kết nối"), key.ID, testModel, 5, api.AdvanceReview)
	c.waitStatus(b.ID, api.StatusWaitingReview)
	path := "/api/books/" + b.ID + "/events"

	_, frames := c.openSSE(path)
	expectHead(t, frames, api.StatusWaitingReview)
	r := c.do("GET", path, nil)
	if r.status != http.StatusTooManyRequests || r.errCode() != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("kết nối thứ hai của cùng user phải 429: %d %s", r.status, r.body)
	}
}

func TestBookRestartRecovery(t *testing.T) {
	logs := captureLogs(t)
	cfg := testConfig(t)
	cfg.Dev = true
	cfg.DevWorkerBin = fakeWorkerBin
	cfg.DatabaseURL = dbtest.NewURL(t)
	opts := func() Options {
		return Options{Version: "test", Dist: testDist(), Catalog: keys.NewCatalog(models.NewModelRegistry())}
	}

	// Lần chạy đầu: truyện đang viết khi server tắt có kiểm soát.
	s1, err := New(context.Background(), cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	base1, stop1 := serveTest(t, s1)
	c := newClientAt(t, base1)
	env := &testEnv{t: t, srv: s1}
	c.signup(env.invite(1), "an@example.com", testPassword)
	key := c.addKey("openrouter")
	b := c.createBook(slowPrompt, key.ID, testModel, 5, api.AdvanceAuto)
	c.waitBook(b.ID, "đã viết xong chương đầu", func(bk api.Book) bool {
		return bk.Snapshot != nil && bk.Snapshot.CompletedCount >= 1
	})
	if err := stop1(); err != nil {
		t.Fatalf("tắt server lần 1: %v", err)
	}
	if st := dbBookStatus(t, cfg.DatabaseURL, b.ID); st != string(api.StatusInterrupted) {
		t.Fatalf("tắt có kiểm soát phải để truyện interrupted, DB có %q", st)
	}

	// Lần khởi động thứ hai trên cùng DB: truyện tự chạy tiếp tới hết.
	s2, err := New(context.Background(), cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	base2, stop2 := serveTest(t, s2)
	c.base = base2
	c.waitWriting(b.ID)
	c.waitStatus(b.ID, api.StatusCompleted)
	if n := c.chapterCount(b.ID); n != 3 {
		t.Fatalf("sau khi khôi phục phải đủ 3 chương, có %d", n)
	}
	if err := stop2(); err != nil {
		t.Fatalf("tắt server lần 2: %v", err)
	}
	if strings.Contains(logs.String(), testLLMKey) {
		t.Fatal("log chứa API key")
	}
}
