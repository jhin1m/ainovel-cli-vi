# Research: engine scout + stack cho `serve` (2026-10-13)

Tổng hợp từ 1 scout (đọc code fork) + 2 researcher (Go server stack, SPA/PWA). Bản tóm tắt; con số version cần kiểm lại lúc cài.

> **Đã bị red-team (2026-10-13) thay thế một phần — plan là nguồn chuẩn:** SSE bỏ seq/`Last-Event-ID`/replay (dùng snapshot + ring upsert theo id); SSE đặt write deadline mỗi lần ghi thay vì xoá; một master key (`AINOVEL_MASTER_KEY_FILE`) thay keyring nhiều version; `KillMode=mixed` thay `control-group`; secret đọc từ file, không qua env.

## 1. Engine (đã kiểm trong code)

- Entry: `cmd/ainovel-cli/main.go` → `runWithConfig` → `headless.Run` / `tui.Run`. Không có HTTP server nào sẵn.
- `host.New(cfg, bundle, opts...)` (`internal/host/host.go:102`) — giữ flock `.ainovel.lock` trên thư mục truyện.
- Điều khiển: `PrepareUserRules`+`StartPrepared` (tạo mới), `Resume()`, `Steer(text)` (đang chạy), `Continue(text)` (đã dừng/xong), `SetAdvanceMode(auto|review)`, `AdvanceOneChapter()` (= `/next`), `Abort()`, `Close()`.
- Output: `Events()` (cap 100), `Stream()` (cap 256, có `StreamClearSentinel`), `Done()`, `Snapshot() UISnapshot` (TUI poll 3s). Kênh single-consumer, đầy thì bỏ bản cũ.
- `UISnapshot` (`internal/host/events.go:50-122`): Phase, Flow, StatusLabel, CurrentChapter, TotalChapters, CompletedCount, TotalWordCount, PendingSteer, AdvanceMode, HasAdvanceHold, Agents, TotalCostUSD, tokens, BudgetLimitUSD, Outline, Characters.
- Store: `cfg.OutputDir` (`json:"-"`, chỉ set từ Go). Chương chốt `chapters/NN.md`, nháp `drafts/`, meta trong `meta/` (progress, run, usage, checkpoints, runtime/queue.jsonl).
- Export: `exp.Run(ctx, exp.Deps{Store}, opts)` chỉ cần Store, không cần dừng engine → server gọi trực tiếp với `store.NewStore(dir)`. **Có ghi file**: mặc định vào thư mục truyện, lỗi nếu đã tồn tại (`internal/host/exp/exporter.go:107-118`) → luôn đặt `OutPath` + `Overwrite`. Định dạng TXT, EPUB.
- Config: `bootstrap.Config{Provider, ModelName, Providers map[name]ProviderConfig{APIKey, BaseURL, Type}, Roles, Language, Style, Budget{BookUSD, WarnRatio, HardStop}, DisableUpdateCheck, OutputDir}` — dựng in-memory được, `ValidateBase` kiểm key.
- Provider preset (`internal/bootstrap/setup.go:52`): openrouter, anthropic, gemini, openai, deepseek, qwen, glm, grok, ollama, bedrock, custom.
- Không có xử lý SIGINT/SIGTERM ở đâu cả (grep `signal.Notify` rỗng).
- `SwitchModel`/`/config` ghi ra `EffectiveConfigPath()` (`internal/host/model_config.go:359`) → không expose qua web.
- `models.MergeModels` có mutex — ok.

### Trạng thái toàn cục của process (lý do chọn 1 process/truyện)

- `lang.Set` (global), `slog.SetDefault` qua `WithFileLog`, `modelreg.DefaultRegistry()` singleton + goroutine `StartPricingRefresh` mỗi `New`.
- `~/.ainovel` (`os.UserHomeDir`): config, pricing cache, user rules, update-check, last-error.log.
- → Nhiều Host trong một process: log lẫn, rules/config dùng chung, panic sập cả server. Subprocess + HOME riêng cô lập miễn phí.

## 2. Server Go (researcher)

- Router: stdlib `net/http` ServeMux (Go 1.22+ pattern). CSRF: `http.CrossOriginProtection` (Go 1.25) + bắt buộc `Content-Type: application/json` cho mutation. (Chưa fetch được doc — kiểm API lúc code.)
- DB: pgx v5 (`pgxpool`), migration goose v3 embed SQL, `pg_advisory_lock` khi migrate. (SQLite đơn giản hơn nếu chắc chỉ 1 node; giữ Postgres theo quyết định.)
- Mật khẩu: argon2id PHC string, tối thiểu OWASP m=19MiB t=2 p=1; semaphore ~4 hash đồng thời chống DoS bộ nhớ.
- Session: 32 byte random, DB lưu `sha256(token)`, cookie `__Host-` HttpOnly Secure SameSite=Lax, idle 7d / tuyệt đối 30d, xoay khi login.
- Rate limit: `x/time/rate` theo IP + email; `CF-Connecting-IP` chỉ tin khi origin khoá theo dải IP Cloudflare.
- Đăng ký mở → bắt buộc xác minh email; MVP dùng invite code thay SMTP.
- BYOK: AES-256-GCM, nonce 12B, AAD=`user_id|key_id`, master key có version (`AINOVEL_MASTER_KEYS`/file 0400). Không bao giờ trả key về client (chỉ last4). Chuyển key cho worker qua **stdin** (không qua env, không file). `RLIMIT_CORE=0`.
- SSE: header `text/event-stream`, `no-cache, no-transform`, `X-Accel-Buffering: no`; heartbeat 15–25s (Cloudflare ~100s idle → 524); `id:` = seq, replay theo `Last-Event-ID`; subscribe trước rồi replay, dedupe theo seq; hub per-job, kênh subscriber buffered, chậm thì cắt cho reconnect; không gzip SSE; xoá write deadline bằng `ResponseController`.
- Supervisor: `exec.Command(self,"worker")`, `Setpgid`, Linux `Pdeathsig`; dừng = SIGTERM nhóm → chờ 10–30s → SIGKILL; `bufio.Scanner` buffer 4MB; stderr vào ring/log; giới hạn global + per-user; khởi động lại: `running` → `interrupted` → resume, có trần retry.
- Deploy: 1 binary + systemd (hardening, `KillMode=control-group`, `TimeoutStopSec=60`) + Caddy/Cloudflare Full(strict) + Postgres apt. Backup `pg_dump` + restic thư mục truyện; **backup master key riêng**.

## 3. SPA/PWA (researcher, version từ `npm view` 2026-10-13)

- Vite 8.3 + React 19.3 + TanStack Router 1.170 (file-based, `autoCodeSplitting`) + Query 5.104 + Tailwind 4.3 (`@tailwindcss/vite`) + shadcn CLI 4.21 + pnpm 10. Không dùng TanStack Start (Go phục vụ static).
- TypeScript: npm `latest` là 7.0 — thử với router-plugin/typescript-eslint, lỗi thì ghim 5.9.
- Type Go→TS: `tygo`, commit `types.gen.ts`, CI kiểm diff.
- SSE: `EventSource` native (same-origin cookie, tự reconnect + Last-Event-ID). Event dạng append → buffer ref, flush rAF vào Query cache; trạng thái → `setQueryData`; token stream giữ ngoài Query (`useSyncExternalStore`).
- UI: `@tanstack/react-virtual` cho timeline; font Literata (đọc) + Be Vietnam Pro (UI) qua fontsource, subset vietnamese+latin; reader `max-width 65ch`, `line-height 1.75`.
- PWA: vite-plugin-pwa 2.0 (đọc migration note), `generateSW`, `navigateFallbackDenylist: [/^\/api\//]`, không cache `/api`. `sw.js`, manifest `no-cache`.
- Native sau này: PWA trước → Capacitor với `server.url` trỏ domain (giữ cookie same-origin). Bundle local cần Bearer token.
- i18n: YAGNI, một file `messages.ts`.
- Embed: `web/embed.go` `//go:embed all:dist`, SPA fallback chỉ cho path không có đuôi file; `/assets/*` immutable 1 năm; dev: Vite proxy `/api` → Go.

## Câu hỏi còn mở

- Đăng ký: invite code hay mở + xác minh email?
- Deploy: systemd hay Docker Compose (hạ tầng hiện có của user)?
- Có VPS/domain sẵn chưa? Cloudflare Free?
- Giới hạn mặc định: số job đồng thời toàn server, mỗi user, số truyện mỗi user.

## Nguồn

- https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
- https://community.cloudflare.com/t/100-second-proxy-read-timeout-524-gateway-error-increase/684447
- https://smartscope.blog/en/Infrastructure/sse-timeout-mitigation-cloudflare-alb/
- https://capacitorjs.com/docs/apis/cookies
- https://forum.ionicframework.com/t/cookies-blocked-after-capacitor-4-upgrade/225625
- https://github.com/antfu/vite-plugin-pwa/issues/218
