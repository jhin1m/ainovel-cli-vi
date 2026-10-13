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
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/entry/worker"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// KeySource giải mã API key cho đúng truyện (FK kép: chỉ key của chủ truyện).
type KeySource interface {
	DecryptForBook(ctx context.Context, bookID string) (provider string, key []byte, err error)
}

// PricingSource cho giá model gửi kèm init.
type PricingSource interface {
	PricingFor(provider, model string) (api.ModelPricing, bool)
}

// Config cấu hình supervisor.
type Config struct {
	DataDir        string
	Version        string   // worker phải chào đúng phiên bản này
	MaxJobs        int      // worker sống đồng thời toàn server
	MaxJobsPerUser int      // worker sống đồng thời mỗi user
	WorkerBin      string   // rỗng = chính binary đang chạy
	WorkerArgs     []string // tham số sau "worker" (chỉ dùng cho worker giả ở dev/test)
	Language       string
	StopGrace      time.Duration // mỗi bước dừng worker chờ bao lâu; 0 = 10s
	// MaxRun: lượt chạy lâu hơn mức này phải nhường slot khi server hết slot và có
	// truyện của user khác đang chờ chạy được ngay; 0 = tắt.
	MaxRun time.Duration
	// SliceGrace: lượt bị cắt được viết nốt chương đang dở trong chừng này rồi mới
	// nhường slot; quá hạn thì dừng hẳn. 0 = dừng hẳn ngay (bỏ chương dở).
	SliceGrace time.Duration
}

// Supervisor giữ map bookID → process đang sống và chạy vòng lập lịch.
type Supervisor struct {
	cfg     Config
	db      jobDB
	keys    KeySource
	pricing PricingSource
	hubs    *Hubs

	mu       sync.Mutex
	procs    map[string]*proc
	perUser  map[string]int
	backoff  map[string]backoffState
	draining bool

	// schedMu tuần tự hoá các lượt lập lịch (claim) với BeforeShutdown: claim chạy sau
	// drain sẽ để lại truyện ở running, bị tính là crash ở lần khởi động sau.
	schedMu sync.Mutex

	wake chan struct{}
	wg   sync.WaitGroup
	idN  atomic.Uint64
}

type backoffState struct {
	attempts int
	until    time.Time
}

// lockBusyBackoff: worker chưa lấy được khoá thư mục — thử lại sau 5s/15s/45s.
var lockBusyBackoff = []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}

// New tạo supervisor.
func New(cfg Config, pool *pgxpool.Pool, keys KeySource, pricing PricingSource, hubs *Hubs) *Supervisor {
	if cfg.StopGrace <= 0 {
		cfg.StopGrace = stopGrace
	}
	return &Supervisor{
		cfg: cfg, db: jobDB{pool: pool}, keys: keys, pricing: pricing, hubs: hubs,
		procs:   make(map[string]*proc),
		perUser: make(map[string]int),
		backoff: make(map[string]backoffState),
		wake:    make(chan struct{}, 1),
	}
}

// Hubs trả về hub realtime.
func (s *Supervisor) Hubs() *Hubs { return s.hubs }

// Start khôi phục trạng thái sau restart rồi chạy vòng lập lịch tới khi ctx huỷ.
func (s *Supervisor) Start(ctx context.Context) error {
	if err := s.db.recoverAll(ctx); err != nil {
		return fmt.Errorf("khôi phục truyện sau khởi động: %w", err)
	}
	go s.loop(ctx)
	s.Kick()
	return nil
}

// Kick đánh thức vòng lập lịch (có truyện mới xếp hàng hoặc slot vừa trống).
func (s *Supervisor) Kick() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Supervisor) loop(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-t.C:
		}
		if err := s.schedule(ctx); err != nil && ctx.Err() == nil {
			logger().Warn("lập lịch worker thất bại", "err", err)
		}
	}
}

func (s *Supervisor) schedule(ctx context.Context) error {
	s.schedMu.Lock()
	defer s.schedMu.Unlock()
	s.mu.Lock()
	if s.draining {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	queued, err := s.db.queued(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, q := range queued {
		s.mu.Lock()
		if s.draining {
			s.mu.Unlock()
			return nil
		}
		if len(s.procs) >= s.cfg.MaxJobs {
			victim := s.sliceVictimLocked(queued, now)
			s.mu.Unlock()
			if victim != nil {
				return s.slice(ctx, victim)
			}
			return nil
		}
		if s.blockedLocked(q.ID, q.UserID, now) {
			s.mu.Unlock()
			continue
		}
		// Giữ chỗ trước khi claim để vòng sau không spawn chồng.
		p := newProc(q.ID, q.UserID, s.hubs.Get(q.ID))
		p.started = now
		s.procs[q.ID] = p
		s.perUser[q.UserID]++
		s.mu.Unlock()

		row, ok, err := s.db.claim(ctx, q.ID)
		if err != nil || !ok {
			// Đóng done để StopAndWait đang chờ proc giữ chỗ này không bị treo.
			p.markDone()
			s.release(p)
			if err != nil {
				return err
			}
			continue
		}
		s.publishStatus(q.ID, api.StatusChange{Status: api.StatusRunning})
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.run(p, row)
		}()
	}
	return nil
}

// queuedBook là một truyện đang chờ chạy.
type queuedBook = struct{ ID, UserID string }

// blockedLocked báo truyện đang chờ chưa chạy ngay được: đã có process, user đã
// chạm trần worker, hoặc đang backoff. Lập lịch và chia lượt dùng chung điều kiện
// này để không lệch nhau. Gọi khi đang giữ s.mu.
func (s *Supervisor) blockedLocked(bookID, userID string, now time.Time) bool {
	_, live := s.procs[bookID]
	return live || s.perUser[userID] >= s.cfg.MaxJobsPerUser || now.Before(s.backoff[bookID].until)
}

// sliceVictimLocked chọn worker phải nhường slot, nil nếu không cần cắt. Chỉ cắt
// khi đủ cả ba điều kiện: server đã hết slot (phía gọi kiểm), lượt chạy quá
// MaxRun, và có truyện của user khác chờ chạy được ngay. Mỗi lúc chỉ cắt một
// worker: worker đang bị cắt còn giữ slot tới khi thoát hẳn, cắt thêm là thừa.
// Gọi khi đang giữ s.mu.
func (s *Supervisor) sliceVictimLocked(queued []queuedBook, now time.Time) *proc {
	if s.cfg.MaxRun <= 0 {
		return nil
	}
	var victim *proc
	for _, p := range s.procs {
		if p.sliced.Load() {
			return nil
		}
		if p.stopping.Load() || now.Sub(p.started) < s.cfg.MaxRun || !p.isReady() {
			continue
		}
		if victim != nil && !p.started.Before(victim.started) {
			continue
		}
		if s.hasRunnableOtherLocked(queued, p.userID, now) {
			victim = p
		}
	}
	return victim
}

// hasRunnableOtherLocked: có truyện chờ của user khác userID chạy được ngay.
func (s *Supervisor) hasRunnableOtherLocked(queued []queuedBook, userID string, now time.Time) bool {
	for _, q := range queued {
		if q.UserID != userID && !s.blockedLocked(q.ID, q.UserID, now) {
			return true
		}
	}
	return false
}

// IsSliced báo truyện đang có worker bị cắt lượt, chưa thoát hẳn.
func (s *Supervisor) IsSliced(bookID string) bool {
	p := s.live(bookID)
	return p != nil && p.sliced.Load()
}

// slice yêu cầu worker dừng để nhường slot. Trước đó đẩy mọi truyện đang chờ
// của cùng user xuống cuối hàng: nếu không, user vừa được nhả trần sẽ lấy lại
// chính slot đó bằng truyện khác của mình. outcome() đưa truyện về cuối hàng
// đợi, không tính là lỗi.
func (s *Supervisor) slice(ctx context.Context, p *proc) error {
	if err := s.db.deprioritizeQueued(ctx, p.userID); err != nil {
		return err
	}
	p.sliced.Store(true)
	logProc(p).Info("lượt chạy quá thời gian, nhường slot cho truyện đang chờ", "max_run", s.cfg.MaxRun, "grace", s.cfg.SliceGrace)
	go s.sliceAtChapterBoundary(p)
	return nil
}

// sliceAtChapterBoundary để worker viết xong chương đang dở rồi mới nhả slot: bỏ
// chương giữa chừng là bắt user trả token hai lần. Quá SliceGrace mà chưa tới
// ranh giới chương, hoặc worker không nhận lệnh, thì dừng hẳn như cũ.
func (s *Supervisor) sliceAtChapterBoundary(p *proc) {
	if s.cfg.SliceGrace > 0 && p.stopAtChapterBoundary(s.nextID()) {
		t := time.NewTimer(s.cfg.SliceGrace)
		defer t.Stop()
		select {
		case <-p.done:
			return
		case <-t.C:
			logProc(p).Info("chưa tới ranh giới chương sau thời gian ân hạn, dừng hẳn worker", "grace", s.cfg.SliceGrace)
		}
	}
	p.terminate(s.cfg.StopGrace)
}

// release bỏ process khỏi bảng theo dõi và đánh thức lập lịch.
func (s *Supervisor) release(p *proc) {
	s.mu.Lock()
	if s.procs[p.bookID] == p {
		delete(s.procs, p.bookID)
		s.perUser[p.userID]--
		if s.perUser[p.userID] <= 0 {
			delete(s.perUser, p.userID)
		}
	}
	s.mu.Unlock()
	s.Kick()
}

func (s *Supervisor) publishStatus(bookID string, st api.StatusChange) {
	s.hubs.Get(bookID).PublishStatus(st)
}

// PublishStatus cho handler báo đổi trạng thái (xếp hàng, tạm dừng khi chưa chạy…).
func (s *Supervisor) PublishStatus(bookID string, st api.StatusChange) { s.publishStatus(bookID, st) }

func (s *Supervisor) workerCmd(bookDir, home string) (*exec.Cmd, error) {
	bin := s.cfg.WorkerBin
	if bin == "" {
		bin = defaultWorkerBin
	}
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		bin = exe
	}
	cmd := exec.Command(bin, append([]string{"worker"}, s.cfg.WorkerArgs...)...)
	cmd.Dir = bookDir
	// Chỉ đúng bốn biến: không secret nào của server lọt sang worker.
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/bin:/usr/bin:/bin"
	}
	tz := os.Getenv("TZ")
	if tz == "" {
		tz = "UTC"
	}
	cmd.Env = []string{"HOME=" + home, "PATH=" + path, "LANG=C.UTF-8", "TZ=" + tz}
	setProcAttr(cmd)
	return cmd, nil
}

// run chạy trọn một lượt worker cho truyện và ghi kết quả.
func (s *Supervisor) run(p *proc, row jobRow) {
	defer s.release(p)
	// Đóng done trên mọi đường thoát, kể cả spawn lỗi: terminate/Shutdown chờ nó.
	defer p.markDone()
	log := logProc(p)
	ctx := context.Background()
	p.mmu.Lock()
	p.mode = row.AdvanceMode
	p.mmu.Unlock()

	fail := func(msg string, err error) {
		if err != nil {
			log.Error(msg, "err", err)
		}
		s.finish(ctx, p, finishResult{Status: api.StatusFailed, StopReason: api.StopError, LastError: msg, ClearPending: false, Failed: true})
	}

	bookDir, err := BookDir(s.cfg.DataDir, row.UserID, row.ID)
	if err != nil {
		fail("Đường dẫn truyện không hợp lệ", err)
		return
	}
	home, _ := HomeDir(s.cfg.DataDir, row.UserID)
	for _, d := range []string{bookDir, home} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			fail("Không tạo được thư mục truyện", err)
			return
		}
	}
	cmd, err := s.workerCmd(bookDir, home)
	if err != nil {
		fail("Không tìm thấy chương trình worker", err)
		return
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fail("Không khởi động được worker", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fail("Không khởi động được worker", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fail("Không khởi động được worker", err)
		return
	}
	if err := cmd.Start(); err != nil {
		fail("Không khởi động được worker", err)
		return
	}
	p.wmu.Lock()
	p.cmd = cmd
	p.stdin = stdin
	p.wmu.Unlock()

	// Luôn đọc stdout tới EOF: worker chặn khi ống đầy và khi đó không thấy được
	// lệnh shutdown hay SIGTERM.
	lines := make(chan []byte, 256)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64<<10), stdoutBufBytes)
		for sc.Scan() {
			lines <- append([]byte(nil), sc.Bytes()...)
		}
		_, _ = io.Copy(io.Discard, stdout)
	}()
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		p.pumpStderr(stderr, bookDir)
	}()

	handshakeErr := readHello(lines, s.cfg.Version)
	if handshakeErr == nil {
		handshakeErr = s.sendInit(ctx, p, row, bookDir)
	}
	if handshakeErr != nil {
		log.Error("bắt tay worker thất bại", "err", handshakeErr)
		signalGroup(cmd, true)
		for range lines {
		}
		<-stderrDone
		_ = cmd.Wait()
		p.markDone()
		if errors.Is(handshakeErr, errInitRejected) {
			fail(handshakeErr.Error(), nil)
			return
		}
		// Lệch phiên bản / không chào: giữ truyện để lần khởi động sau chạy lại.
		_, _ = s.db.pool.Exec(ctx, `UPDATE books SET status = 'interrupted', updated_at = now() WHERE id = $1 AND status = 'running'`, p.bookID)
		s.publishStatus(p.bookID, api.StatusChange{Status: api.StatusInterrupted})
		return
	}
	p.markReady()
	if p.abortLater.Load() {
		_ = p.writeLine(api.WorkerCommand{Type: api.MsgAbort, ID: s.nextID()})
	}

	var lastSave time.Time
	saveSnap := func(snap *api.BookSnapshot, changed bool) {
		if !changed && time.Since(lastSave) < snapshotSaveGap {
			return
		}
		lastSave = time.Now()
		if b := marshalSnapshot(snap); b != nil {
			if err := s.db.saveSnapshot(ctx, p.bookID, b); err != nil {
				log.Warn("lưu snapshot thất bại", "err", err)
			}
		}
	}
	for line := range lines {
		var msg api.WorkerMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Warn("worker ghi dòng không phải JSON")
			continue
		}
		p.handle(msg, saveSnap)
	}
	// Wait chỉ được gọi khi đã đọc xong cả stdout lẫn stderr (yêu cầu của os/exec).
	<-stderrDone
	waitErr := cmd.Wait()
	p.wmu.Lock()
	p.stdin = nil
	p.wmu.Unlock()
	p.markDone()
	if waitErr != nil && p.exit == nil {
		log.Warn("worker thoát bất thường", "err", waitErr)
	}

	s.mu.Lock()
	draining := s.draining
	s.mu.Unlock()
	// Worker tự dừng (có exit message không phải do server yêu cầu shutdown) thì kết
	// quả vẫn hợp lệ dù server đang drain; chỉ lượt bị chính drain cắt ngang mới giữ
	// interrupted và pending để chạy lại sau khởi động.
	cutByDrain := p.stopping.Load() || p.exit == nil || p.exit.Reason == api.ExitShutdown
	if draining && cutByDrain {
		// drain đã đặt interrupted; chỉ lưu snapshot cuối.
		if b := marshalSnapshot(p.lastSnap); b != nil {
			_ = s.db.saveSnapshot(ctx, p.bookID, b)
		}
		return
	}
	res, requeue := p.outcome()
	res.AfterDrain = draining
	if res.Failed {
		log.Warn("lượt chạy worker thất bại", p.failureDetail()...)
	}
	if requeue {
		if draining {
			return
		}
		s.mu.Lock()
		b := s.backoff[p.bookID]
		delay := lockBusyBackoff[min(b.attempts, len(lockBusyBackoff)-1)]
		s.backoff[p.bookID] = backoffState{attempts: b.attempts + 1, until: time.Now().Add(delay)}
		s.mu.Unlock()
		if err := s.db.requeue(ctx, p.bookID); err != nil {
			log.Error("xếp hàng lại thất bại", "err", err)
		}
		s.publishStatus(p.bookID, api.StatusChange{Status: api.StatusQueued})
		return
	}
	s.mu.Lock()
	delete(s.backoff, p.bookID)
	s.mu.Unlock()
	s.finish(ctx, p, res)
}

// errInitRejected: không dựng được init (key bị xoá, model mất giá) — lỗi của truyện,
// không phải của worker.
var errInitRejected = errors.New("init bị từ chối")

type initError struct{ msg string }

func (e initError) Error() string        { return e.msg }
func (e initError) Is(target error) bool { return target == errInitRejected }

func (s *Supervisor) sendInit(ctx context.Context, p *proc, row jobRow, bookDir string) error {
	provider, key, err := s.keys.DecryptForBook(ctx, row.ID)
	if err != nil {
		return initError{"API key của truyện không còn dùng được (đã bị xoá?). Hãy chọn key khác rồi thử lại."}
	}
	defer clear(key)
	pricing, ok := s.pricing.PricingFor(provider, row.Model)
	if !ok {
		return initError{"Model của truyện hiện không có bảng giá nên không thể chạy có giới hạn ngân sách."}
	}
	// Gói thuê bao không có ngân sách USD: worker từ chối init có budget khác 0.
	budget := row.BudgetUSD
	if worker.IsSubscription(provider) {
		budget = 0
	}
	init := api.WorkerInit{
		Type: api.MsgInit, Protocol: api.WorkerProtocol, Action: row.Action,
		BookDir: bookDir, AdvanceMode: row.AdvanceMode,
		LLM:       api.WorkerLLM{Provider: provider, Model: row.Model, APIKey: string(key)},
		Pricing:   &pricing,
		Language:  s.cfg.Language,
		BudgetUSD: budget,
	}
	switch row.Action {
	case api.ActionStart:
		init.Prompt = row.Text
	case api.ActionSteer:
		init.Text = row.Text
	}
	b, err := json.Marshal(init)
	init.LLM.APIKey = ""
	if err != nil {
		return err
	}
	b = append(b, '\n')
	defer clear(b)
	p.wmu.Lock()
	_, err = p.stdin.Write(b)
	p.wmu.Unlock()
	if err != nil {
		return fmt.Errorf("gửi init: %w", err)
	}
	return nil
}

func (s *Supervisor) finish(ctx context.Context, p *proc, res finishResult) {
	ok, err := s.db.finish(ctx, p.bookID, res)
	if err != nil {
		logProc(p).Error("ghi kết quả lượt chạy thất bại", "err", err)
		return
	}
	if ok {
		s.publishStatus(p.bookID, api.StatusChange{Status: res.Status, StopReason: res.StopReason, LastError: res.LastError})
	}
}

func (s *Supervisor) nextID() string { return fmt.Sprintf("s%d", s.idN.Add(1)) }

func (s *Supervisor) live(bookID string) *proc {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.procs[bookID]
}

// IsRunning báo truyện có worker đang sống.
func (s *Supervisor) IsRunning(bookID string) bool { return s.live(bookID) != nil }

// Command gửi lệnh runtime tới worker đang sống. ack nil kèm err nil: đã gửi nhưng
// chưa có xác nhận trong 5s.
func (s *Supervisor) Command(ctx context.Context, bookID string, cmd api.WorkerCommand) (*api.WorkerMessage, error) {
	p := s.live(bookID)
	if p == nil || p.stopping.Load() {
		return nil, ErrNotRunning
	}
	cmd.ID = s.nextID()
	if cmd.Type == api.MsgSetAdvanceMode {
		return p.setAdvanceMode(ctx, cmd)
	}
	return p.command(ctx, cmd)
}

// StopAndWait dừng worker của truyện (nếu có) và chờ process thoát hẳn — chỉ khi
// đó mới được xoá thư mục hay spawn lượt mới.
func (s *Supervisor) StopAndWait(bookID string) {
	if p := s.live(bookID); p != nil {
		p.terminate(s.cfg.StopGrace)
	}
	// run() có thể vẫn đang ghi kết quả sau Wait: chờ nó nhả slot.
	for s.live(bookID) != nil {
		time.Sleep(20 * time.Millisecond)
	}
}

// BeforeShutdown: đánh dấu truyện đang chạy là interrupted và ngừng lập lịch.
func (s *Supervisor) BeforeShutdown(ctx context.Context) {
	s.mu.Lock()
	s.draining = true
	s.mu.Unlock()
	// Chờ lượt lập lịch đang claim (nếu có) xong rồi mới drain.
	s.schedMu.Lock()
	defer s.schedMu.Unlock()
	if err := s.db.drain(ctx); err != nil {
		logger().Error("đánh dấu truyện gián đoạn thất bại", "err", err)
	}
}

// Shutdown dừng mọi worker song song (shutdown → SIGTERM → SIGKILL) và chờ xong.
func (s *Supervisor) Shutdown(ctx context.Context) {
	s.mu.Lock()
	s.draining = true
	procs := make([]*proc, 0, len(s.procs))
	for _, p := range s.procs {
		procs = append(procs, p)
	}
	s.mu.Unlock()
	var wg sync.WaitGroup
	for _, p := range procs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.terminate(s.cfg.StopGrace)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		for _, p := range procs {
			p.signal(true)
		}
		<-done
	}
}

// RunningCount trả về số worker đang sống (cho test và healthz).
func (s *Supervisor) RunningCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.procs)
}

func logger() *slog.Logger { return slog.Default().With("module", "jobs") }
