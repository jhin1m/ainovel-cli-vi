// Package server là chế độ `ainovel-cli serve`: HTTP API + SSE + SPA nhúng,
// Postgres cho tài khoản/key/truyện, mỗi lượt chạy truyện là một process worker.
package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/auth"
	"github.com/voocel/ainovel-cli/internal/server/books"
	"github.com/voocel/ainovel-cli/internal/server/db"
	"github.com/voocel/ainovel-cli/internal/server/httpx"
	"github.com/voocel/ainovel-cli/internal/server/jobs"
	"github.com/voocel/ainovel-cli/internal/server/keys"
	"github.com/voocel/ainovel-cli/internal/server/secret"
	"github.com/voocel/ainovel-cli/web"
)

// Kích thước body tối đa của request JSON.
const maxBodyBytes = 1 << 20

// requestTimeout là thời gian tối đa cho một route JSON thường.
const requestTimeout = 30 * time.Second

// Server giữ các phụ thuộc dùng chung và hai mux: root (healthz, SPA, route stream
// tự quản deadline) và api (route JSON thường, có timeout).
type Server struct {
	cfg     Config
	version string
	pool    *pgxpool.Pool
	dist    fs.FS

	root *http.ServeMux
	api  *http.ServeMux

	auth    *auth.Service
	keyRepo *keys.Repo
	catalog *keys.Catalog
	sup     *jobs.Supervisor
	books   *books.Service

	// bgCtx sống suốt đời server; huỷ khi tắt để dừng các việc nền.
	bgCtx    context.Context
	bgCancel context.CancelFunc

	// closing đóng khi bắt đầu tắt: SSE và việc chạy dài lắng nghe để tự kết thúc.
	closing     chan struct{}
	closingOnce sync.Once

	// beforeShutdown chạy trước khi dừng HTTP (vd. đánh dấu job interrupted);
	// afterHTTP chạy sau khi HTTP đã dừng (vd. tắt các worker).
	beforeShutdown []func(context.Context)
	afterHTTP      []func(context.Context)
}

// Options cho New.
type Options struct {
	Version string
	Dist    fs.FS         // nil = SPA nhúng trong binary
	Catalog *keys.Catalog // nil = registry giá mặc định
}

// New mở Postgres, chạy migration và dựng toàn bộ route.
func New(ctx context.Context, cfg Config, opts Options) (*Server, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("tạo AINOVEL_DATA_DIR: %w", err)
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	s, err := newServer(cfg, pool, opts)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func newServer(cfg Config, pool *pgxpool.Pool, opts Options) (*Server, error) {
	dist := opts.Dist
	if dist == nil {
		sub, err := fs.Sub(web.Dist, "dist")
		if err != nil {
			return nil, err
		}
		dist = sub
	}
	box, err := secret.New(cfg.MasterKey)
	if err != nil {
		return nil, err
	}
	catalog := opts.Catalog
	if catalog == nil {
		catalog = keys.NewCatalog(models.DefaultRegistry())
	}
	s := &Server{
		cfg:     cfg,
		version: opts.Version,
		pool:    pool,
		dist:    dist,
		root:    http.NewServeMux(),
		api:     http.NewServeMux(),
		closing: make(chan struct{}),
		auth:    auth.NewService(pool, cfg.Dev),
		keyRepo: keys.NewRepo(pool, box),
		catalog: catalog,
	}
	s.bgCtx, s.bgCancel = context.WithCancel(context.Background())
	jobCfg := jobs.Config{
		DataDir: cfg.DataDir, Version: opts.Version, Language: "vi",
		MaxJobs: cfg.Limits.MaxJobs, MaxJobsPerUser: cfg.Limits.MaxJobsPerUser,
	}
	if cfg.Dev && cfg.DevWorkerBin != "" {
		jobCfg.WorkerBin = cfg.DevWorkerBin
		jobCfg.WorkerArgs = []string{"--version", opts.Version}
	}
	s.sup = jobs.New(jobCfg, pool, s.keyRepo, catalog, jobs.NewHubs())
	s.books = books.NewService(books.Deps{
		Repo: books.NewRepo(pool), Keys: s.keyRepo, Catalog: catalog, Auth: s.auth, Sup: s.sup,
		Exporter: books.NewExporter(cfg.Limits.MaxExports), DataDir: cfg.DataDir, Closing: s.closing,
		Limits: books.Limits{
			MaxBooksPerUser: cfg.Limits.MaxBooksPerUser, MaxBookBudgetUSD: cfg.Limits.MaxBookBudgetUSD,
			MaxSSEPerUser: cfg.Limits.MaxSSEPerUser, MaxSSETotal: cfg.Limits.MaxSSETotal,
		},
	})
	// Tắt hai bước: đánh dấu truyện đang chạy là interrupted trước khi dừng HTTP,
	// dừng worker sau khi HTTP đã dừng, rồi mới huỷ việc nền và đóng pool.
	s.beforeShutdown = append(s.beforeShutdown, s.sup.BeforeShutdown)
	s.afterHTTP = append(s.afterHTTP, s.sup.Shutdown, func(context.Context) { s.bgCancel() })
	if err := s.routes(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) routes() error {
	crossOrigin, err := httpx.CrossOrigin(s.cfg.PublicURL)
	if err != nil {
		return fmt.Errorf("AINOVEL_PUBLIC_URL: %w", err)
	}
	// Route JSON thường: CSRF → bắt buộc JSON → giới hạn body → timeout.
	// Recover đặt cả trong lớp timeout: TimeoutHandler chạy handler ở goroutine
	// riêng và ném lại panic, stack trace bắt ở lớp ngoài là của TimeoutHandler.
	s.root.Handle("/api/", httpx.Chain(s.api, crossOrigin, httpx.RequireJSON, httpx.MaxBytes(maxBodyBytes), httpx.Timeout(requestTimeout), httpx.Recover))
	s.auth.Register(s.api)
	keys.NewHandlers(s.keyRepo, s.catalog, s.auth, s.books.KeyInUse).Register(s.api)
	s.books.Register(s.api, s.root)
	s.api.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy API này")
	})
	s.root.HandleFunc("GET /healthz", s.handleHealth)
	s.root.Handle("/", spaHandler(s.dist))
	return nil
}

// Handler trả về handler gốc đã bọc middleware chung.
func (s *Server) Handler() http.Handler {
	return httpx.Chain(s.root,
		httpx.RequestContext(s.cfg.TrustedProxies, s.cfg.ClientIPHeader),
		httpx.AccessLog,
		httpx.Recover,
		httpx.SecurityHeaders,
	)
}

// Closing trả về kênh đóng khi server bắt đầu tắt.
func (s *Server) Closing() <-chan struct{} { return s.closing }

// Run lắng nghe tới khi ctx bị huỷ rồi tắt theo hai bước: beforeShutdown →
// dừng HTTP → afterHTTP → đóng pool.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

// Serve giống Run nhưng dùng listener có sẵn (test).
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}
	if err := s.startBackground(); err != nil {
		s.shutdownRest(context.Background())
		return err
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	slog.Info("server đang lắng nghe", "addr", ln.Addr().String(), "version", s.version)

	select {
	case err := <-errCh:
		s.shutdownRest(context.Background())
		return err
	case <-ctx.Done():
	}

	slog.Info("đang tắt server")
	stopCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, fn := range s.beforeShutdown {
		fn(stopCtx)
	}
	s.closingOnce.Do(func() { close(s.closing) })
	// Chờ request đang chạy tối đa requestTimeout (SSE đã tự thoát nhờ closing),
	// quá hạn thì đóng cứng để context của các request còn lại bị huỷ.
	httpCtx, cancelHTTP := context.WithTimeout(stopCtx, requestTimeout+2*time.Second)
	defer cancelHTTP()
	if err := srv.Shutdown(httpCtx); err != nil {
		slog.Warn("tắt HTTP quá hạn, đóng cứng các kết nối còn lại", "err", err)
		_ = srv.Close()
	}
	s.shutdownRest(stopCtx)
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) shutdownRest(ctx context.Context) {
	s.closingOnce.Do(func() { close(s.closing) })
	for _, fn := range s.afterHTTP {
		fn(ctx)
	}
	if s.pool != nil {
		s.pool.Close()
	}
}

// startBackground chạy các việc nền: nạp/làm mới bảng giá model, dọn session hết
// hạn và rate limiter.
func (s *Server) startBackground() error {
	s.catalog.StartRefresh(s.bgCtx, filepath.Join(s.cfg.DataDir, "cache"))
	if err := s.sup.Start(s.bgCtx); err != nil {
		return err
	}
	s.books.ResumeDeletes(s.bgCtx)
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-s.bgCtx.Done():
				return
			case <-t.C:
				if err := s.auth.Sessions().PurgeExpired(s.bgCtx); err != nil {
					slog.Warn("dọn session hết hạn thất bại", "err", err)
				}
				s.auth.SweepLimiters()
			}
		}
	}()
	return nil
}

// lowDiskBytes: dưới mức này /healthz báo disk_low để cảnh báo trước khi đầy.
const lowDiskBytes = 1 << 30

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status, code := "ok", http.StatusOK
	dbStatus := "ok"
	if s.pool == nil || s.pool.Ping(ctx) != nil {
		dbStatus, status, code = "error", "error", http.StatusServiceUnavailable
	}
	free, ok := diskFree(s.cfg.DataDir)
	body := map[string]any{"status": status, "db": dbStatus, "version": s.version}
	if ok {
		body["disk_free_bytes"] = free
		body["disk_low"] = free < lowDiskBytes
	}
	httpx.WriteJSON(w, code, body)
}
