# Review: phase 2 server foundation (commit 1e1fe15)

Score: **7.5/10**. The foundation is solid and every acceptance criterion holds when tested directly. There are no Critical issues. One trust-boundary issue (CF-Connecting-IP) must be fixed before phase 3 rate limiting depends on `ClientIP`.

## Scope and verification performed
- Files: internal/server/{config,server,spa,cmd,diskfree_*}.go, httpx/*, db/* (+dbtest, 0001_init.sql), web/embed.go, .gitignore, cmd/ainovel-cli/main.go, go.mod.
- The commit was checked out alone in a detached worktree, without the uncommitted phase-3 files such as 0002_auth.sql.
  - `go test -race ./internal/server/...` passes with the DB under go1.25.5 (the toolchain the Dockerfile uses) and under the local go1.27.2.
  - `go vet ./...`, `gofmt -l` and `go mod tidy -diff` are all clean.
- End-to-end: the built binary started as two instances at once against an empty DB, using only secret files.
  - Both `/healthz` returned 200.
  - `goose_db_version` has exactly rows 0 and 1, so migration ran once.
  - `/` returned the placeholder with CSP and `no-cache`.
  - POST with `Sec-Fetch-Site: same-site` returned 403 JSON, and so did POST with `Origin: null`.
- Cross-compile (CGO_ENABLED=0): linux amd64/arm64/386/arm, darwin arm64, windows amd64/arm64, freebsd and dragonfly all succeed. openbsd, netbsd, solaris and illumos fail (see L1).

## Acceptance criteria
| Criterion | Result |
|---|---|
| serve starts with secret files; /healthz 200; migration runs exactly once with 2 concurrent instances | PASS (e2e + TestMigrateConcurrent with 4 goroutines) |
| Cross-origin POST blocked; same-origin JSON passes; HTML has CSP | PASS |
| `go build` works when web/dist only has .gitkeep | PASS (placeholder page) |
| Config: missing secret file errors, env DB URL rejected outside dev, perm > 0600 warns | PASS (config_test) |

## High

**H1. `CF-Connecting-IP` is trusted from any trusted proxy, so client IP can be forged when not behind Cloudflare.** `internal/server/httpx/clientip.go:18-22`
- Failure scenario: deploy nginx -> app with `AINOVEL_TRUSTED_PROXIES=127.0.0.1`. nginx forwards a client-supplied `CF-Connecting-IP` header unchanged. An attacker sends a random `CF-Connecting-IP` on each request, `ClientIP` returns that value, and per-IP rate limiting (login brute force in phase 3) is bypassed completely.
- This header also takes priority over XFF, so even a correct XFF walk does not help.
- The spec does say "read CF/XFF only when the peer is trusted", but that is not enough: the CF header is only valid when the edge in front is actually Cloudflare.
- Fix: add explicit config, e.g. `AINOVEL_CLIENT_IP_HEADER=x-forwarded-for|cf-connecting-ip` (default XFF). Read exactly one header and never fall back to the other. Add a test: trusted peer + forged CF header + default config -> uses XFF/peer.

## Medium

**M1. Panic stack traces are lost for every `/api/` route.** `server.go:108` + `httpx/middleware.go:195-199`
- `http.TimeoutHandler` recovers in its own goroutine and re-panics in the outer goroutine (net/http server.go:3859-3871). `Recover` therefore logs `debug.Stack()` from the TimeoutHandler goroutine, not from the handler.
- Verified with a temporary test: the stack does not contain the function that panicked.
- Failure scenario: a phase 3+ handler panics in production and the log has no clue where.
- Fix: also put `httpx.Recover` inside the api mux (as the last element of the `/api/` Chain, after `Timeout`), or wrap `s.api` with Recover before the Timeout.

**M2. The timeout 503 response is `text/plain`, which violates the unified JSON error contract.** `httpx/middleware.go:195-199`
- TimeoutHandler does not set Content-Type, so net/http sniffs it as `text/plain; charset=utf-8` (verified).
- Failure scenario: the SPA client branches on `content-type: application/json` to parse `{"error":...}`. On timeout it shows a generic error or a parse error instead of the Vietnamese message.
- Fix: in `Timeout`, set `w.Header().Set("Content-Type","application/json; charset=utf-8")` on the outer writer before calling `TimeoutHandler.ServeHTTP`. On the success path `maps.Copy` overrides it with the handler's own headers, and WriteJSON always sets CT.

**M3. Shutdown is not bounded as designed; the second signal is ignored.** `server.go:161-176`, `cmd.go:29-30`
- (a) `srv.Shutdown(15s)` is shorter than the `/api/` `requestTimeout` (30s). On deadline the code does not call `srv.Close()`, and `Shutdown` never cancels the context of requests in flight. The code then calls `pool.Close()`, which, per pgxpool/pool.go:468, "Blocks until all connections are returned". This call ignores `stopCtx` (60s). A handler holding a connection without honouring ctx (or a phase 4 SSE that misses `closing`) makes the process hang until systemd SIGKILLs it, and the afterHTTP hooks (stopping workers) may have run with an already expired ctx.
- (b) `signal.NotifyContext` keeps catching signals after the first one because `stop()` is only deferred. A second Ctrl+C or SIGTERM does not kill a hung process.
- Fix:
  - Use `BaseContext` with a cancelable root ctx. On `Shutdown` DeadlineExceeded, call `srv.Close()` and cancel the root ctx, then close the pool.
  - Call `stop()` right after `<-ctx.Done()` (or pass it in) so the second signal uses default behaviour.
  - Consider setting Shutdown >= requestTimeout, or making both depend on each other.

## Low

- **L1. Build regression on openbsd, netbsd, solaris and illumos.** `diskfree_unix.go:1`
  - `//go:build unix` covers them, but `syscall.Statfs`/`Statfs_t.Bavail` do not exist there; openbsd uses `F_bavail`. The parent commit bc74c5c built on all four; this commit fails.
  - Release targets (goreleaser: linux/darwin/windows) are not affected.
  - Fix: `//go:build linux || darwin || freebsd || dragonfly`, and change the `diskfree_other.go` tag to the negation.
- **L2. `/healthz` is public and unthrottled.** `server.go:192-207`
  - It returns the exact `version` (confirmed: `v0.0.0-...-1e1fe157e22c`) and `disk_free_bytes` to anyone.
  - Each call `Ping`s and holds one connection out of 20 (MaxConns).
  - Suggestion: return only `status` publicly, and keep details for localhost/trusted peers or the log.
- **L3. Invalid `AINOVEL_PUBLIC_URL` is caught late.** `config.go:55-59`
  - A value with a path or query (`https://x.com/app`) passes `LoadConfig`, but `AddTrustedOrigin` rejects it in `routes()` (`server.go:103`). This happens after the DB connection and migration have already run.
  - Fix: check `u.Path=="" && u.RawQuery=="" && u.Fragment==""` inside LoadConfig.
- **L4. A secret in env is silently ignored.** `config.go:65-76`
  - When both `AINOVEL_DATABASE_URL_FILE` and `AINOVEL_DATABASE_URL` are set outside dev, the env value is silently ignored. The secret stays in `/proc/<pid>/environ` and will be inherited by phase 4 worker children if env is not scrubbed.
  - Fix: fail (or at least warn) when the env var is set outside dev, regardless of whether the file is set.
- **L5. XFF walk falls back to the proxy IP on a bad entry.** `clientip.go:24-36`
  - When an XFF entry fails to parse (a proxy appending `ip:port`, or `unknown`), the walk `break`s and returns the peer, which is the proxy IP. Every user then shares one rate-limit bucket, so one person can lock everyone out.
  - Also, a trusted prefix written in `::ffff:a.b.c.d/x` form never matches, because the address has been `Unmap()`ed. Consider `p.Masked()` plus unmapping the prefix in `parseCIDRs`.
- **L6. Pre-existing `.gitignore` line `tasks` (line 44) is unanchored.**
  - `git check-ignore` confirms `web/src/tasks/List.tsx` and `internal/server/tasks/x.go` are ignored.
  - This commit did not introduce it, but phase 4/5 can easily lose code. Anchor it to `/tasks`.
  - The new rules themselves are correct, checked with `check-ignore`:
    - `web/dist/*` and `web/dist/assets/*` are ignored.
    - `web/dist/.gitkeep` is tracked.
    - root `dist/` and `other/dist/` are still ignored.
    - `web/src` is not ignored.
    - `.dockerignore` `dist` only matches the root, so the Docker build still has web/dist.
- **L7. SPA:** `spa.go:68` (`fs.ReadFile`) copies the whole file on every request, and there is no ETag/Last-Modified, so `no-cache` files never get a 304. SPA routes containing a dot (`/u/a.b`) return 404 (`spa.go:47`). Suggestion: precompute the ETag (sha256) once at startup and use `fs.Open` + `io.ReadSeeker`.
- **L8. `Recover` after headers are sent** (`middleware.go:105`) returns normally and produces a truncated response. It should `panic(http.ErrAbortHandler)` so the connection is aborted. `AccessLog` has no defer, so it loses the log line when a panic propagates.
- **L9. No body read deadline on the `/api/` chain.** There is no `ReadTimeout` (this is correct, for SSE). A slow-body client keeps the handler goroutine blocked reading the body after TimeoutHandler has already returned 503. Suggestion: call `http.NewResponseController(w).SetReadDeadline(now+30s)` in the `/api/` chain.

## Notes for phase 3/4 (no change needed now)
- `statusRecorder` already has `Unwrap`, so `http.NewResponseController` reaches Flush/SetWriteDeadline. SSE must use ResponseController; `w.(http.Flusher)` will fail through the 2 wrapper layers.
- Routes registered on the root mux (SSE/export) do **not** get CrossOriginProtection/RequireJSON/MaxBytes. They must be GET-only, or have the middleware attached manually.
- `RequireJSON` requires a Content-Type even for a DELETE with no body, so the SPA client must always send `Content-Type: application/json`.

## Points checked and found OK
- **CSRF semantics (Go 1.25.5 csrf.go):**
  - GET/HEAD/OPTIONS always pass.
  - `Sec-Fetch-Site` `same-origin` and `none` pass; `same-site` and `cross-site` get 403.
  - With no Sec-Fetch-Site but an Origin header, the request passes only if `Origin.Host == r.Host` or the origin is a trusted origin (PUBLIC_URL). `Origin: null` gets 403.
  - With neither header (curl), the request passes. That is acceptable because RequireJSON + SameSite cookies are the second layer.
- **DB URL does not leak:** the `ParseConfig` error is hidden, and the pgx connect error only contains user/database/host, not the password (test included).
- **SPA path traversal:** `path.Clean("/"+p)` + `fs.ValidPath` + read-only embed.FS. Cache headers for assets/index/sw/manifest match the spec.
- **goose advisory lock:**
  - Uses `pg_try_advisory_lock` with retries every 5s for up to 5 minutes, held on a dedicated `*sql.Conn`, and unlocks with a detached ctx.
  - The lock is scoped per database.
  - `stdlib.OpenDBFromPool` + `sqlDB.Close()` does **not** close the pgxpool: the connector has no Close, and healthz still returns 200 after Migrate in the test.
- **dbtest is safe to run in parallel:**
  - Random 48-bit DB names.
  - The advisory lock is per-database, so separate test DBs do not block each other.
  - 12 concurrent `CREATE DATABASE` commands in a stress test succeeded.
  - Cleanup runs LIFO (pool.Close before DROP ... WITH (FORCE)).
  - No `ainovel_test_*` databases were left over.
- **main.go:** `serve` is dispatched after `eval`/`worker` and before `parseCLIOptions`, following the same pattern. The only change is that `ainovel-cli serve` used to be the positional argument "serve", which is acceptable.
- **Pinned versions:** go.mod stays `go 1.25.5`; pgx v5.11.0, goose v3.27.0; tidy is clean.

## Unresolved questions
- Will production run behind Cloudflare, or only nginx/Caddy? This decides the default for H1.
- Should `/healthz` be public (uptime monitor) or local-only? This decides L2.
