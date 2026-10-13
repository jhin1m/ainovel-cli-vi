# Code review: Phase 6 (PWA, deploy, docs) and the `serve --help` fix

Commits reviewed: 2ac8c9f (PWA, deploy/, web.yml, docs, README, SYNC) and e40b7c8 (`serve --help`).
I tested both in a detached worktree at 2ac8c9f with `GOTOOLCHAIN=go1.25.5`. HEAD has since moved to 427ea1e. Among the files in scope, only SYNC.md changed after 2ac8c9f (one line). `exportTimeout` and `stopGrace` are unchanged at HEAD.

## What I checked

| Check | Result |
|---|---|
| `make gen` then `git diff` | clean |
| `pnpm -C web install --frozen-lockfile` / typecheck / lint / test / build (pnpm 12.11.2) | all green, 53 tests |
| `go build` with the built SPA, then `serve --help` under `env -i` | exit 0, prints usage |
| gofmt and `go vet` on server, worker, cmd | clean |
| `go test -race ./internal/server/... ./internal/entry/worker/...` against Postgres 17 | all pass |
| `caddy adapt` (Caddy v2.11.7, docker `caddy:2`) | adapts; only a "not formatted" warning |
| Caddy path matcher `not path /api/books/*/events`, tested at runtime | matches `/api/books/<id>/events` only, as intended |
| shellcheck `backup.sh` | 0 findings |
| actionlint `web.yml` | 0 findings |
| `systemd-analyze verify` on systemd 252 (Debian 12) | rc=0, `%d` resolves |
| `systemd-analyze security` | 3.2 "OK" |
| `ci.yml`, `.goreleaser.yml`, `Dockerfile` vs `vi` | unchanged |

## Critical

None.

## High

### H1. Docs "Full (strict) + let Caddy get its own certificate" cannot work with the documented firewall

- **Where:** `docs/web-server.md:102-104`, `deploy/Caddyfile:24`
- **Problem:** the Cloudflare proxy is on (orange cloud), so the TLS-ALPN-01 challenge cannot reach Caddy. The docs open only 443 to Cloudflare's ranges, so the HTTP-01 challenge (port 80) is blocked too.
- **Result:** Caddy never gets a certificate. Full (strict) then makes Cloudflare return 525/526 on the first deploy. Certificate renewal hits the same wall later.
- **The other option is not wired either:** the "Origin Certificate" path in the docs needs a `tls <cert> <key>` directive. The Caddyfile has none, and the docs don't say where to put the files.
- **Impact:** "docs are enough for someone else to set up the server" is not met.
- **Fix, pick one:**
  - (a) Use an Origin Certificate. Add `tls /etc/caddy/cf-origin.pem /etc/caddy/cf-origin.key` to the site block, plus the steps to create and install the cert. This is recommended because only 443 stays open.
  - (b) Keep ACME. Also open port 80 to Cloudflare's ranges, and state that renewals need it too.

### H2. One worker OOM restarts the whole service, the opposite of what the docs promise

- **Where:** `deploy/ainovel-serve.service`, which has no `OOMPolicy=`. The comment at lines 41-43 and `docs/web-server.md:210` promise the opposite.
- **Problem:** systemd's `OOMPolicy=` defaults to `DefaultOOMPolicy=stop`. I checked this against the systemd v252 man page source: "If set to stop the event is logged but the service is terminated cleanly by the service manager".
- **Scenario:**
  1. `MemoryMax=3G` is hit. Or global OOM kills the worker, which happens by design because of `oom_score_adj=500`.
  2. systemd sees `oom_kill` in the unit's cgroup and stops the whole unit.
  3. The server drains, and every running book for every user is interrupted.
  4. The result is `oom-kill`, not success. `service.c` restarts on `!IN_SET(result, SUCCESS, SKIP_CONDITION)`, so `Restart=on-failure` starts it again.
  5. Recovery resumes the same heavy book, which can OOM again. The cycle repeats every few minutes and never trips the start-limit.
- **Fix:** add `OOMPolicy=continue` to `[Service]`. Optionally add `OOMScoreAdjust=-500` for the server; the worker raises its own value to +500, which an unprivileged process is allowed to do.
- **Also:** update the comment and the troubleshooting row in the docs.

## Medium

### M1. SYNC.md go.mod conflict recipe silently bumps the Go version or fails

- **Where:** `SYNC.md:90-91`
- **Problem:** `git checkout --theirs go.mod go.sum && go mod tidy` drops the pinned server dependencies. `tidy` then re-resolves them at `@latest`.
- **Verified:** today that resolves to goose v3.29.0, x/crypto v0.58.0 and x/time v0.16.0, and all three have `go 1.26.0` in their go.mod.
  - With `GOTOOLCHAIN=auto` (the local Go 1.27) the `go` line is bumped to 1.26. The `golang:1.25` Docker image sets `GOTOOLCHAIN=local`, so that build then fails.
  - With a local 1.25.5 toolchain, `tidy` errors out.
- **Fix:** after `--theirs`, pin explicitly before running tidy:
  ```
  go get github.com/jackc/pgx/v5@v5.11.0 github.com/pressly/goose/v3@v3.27.0 golang.org/x/crypto@v0.55.0 golang.org/x/time@v0.15.0
  go mod tidy
  ```
  Then run `git diff go.mod` to confirm the `go` line did not change. Alternatively, merge the require blocks by hand.

### M2. Caddy gives up on the backend after 60s, but the server lets an export run for 120s

- **Where:** `deploy/Caddyfile:47` (`response_header_timeout 60s`) vs `internal/server/books/handlers.go:27` (`exportTimeout = 2 * time.Minute`)
- **Problem:** `handleExport` builds the whole file before it writes any header.
- **Scenario:** a slow export (big EPUB, slow disk) runs past 60s. Caddy returns 502 while the server keeps working for up to 2 minutes and holds one of the `AINOVEL_MAX_EXPORTS` slots. Cloudflare's limit is 100s, so 120s can never succeed through the proxy anyway.
- **Fix:** keep the export deadline under every proxy limit. For example, set `exportTimeout` to about 50s, or raise `response_header_timeout` to 95s and lower `exportTimeout` below 90s.
- **Note:** `export.go` and `handlers.go` are being edited by another agent, so coordinate.

### M3. Backup uses local tar instead of restic, which differs from the spec without sign-off

- **Where:** `deploy/backup.sh:2,42`; spec: "pg_dump -Fc + restic cho /var/lib/ainovel/users"
- **Problem:** backups stay on the same host and are unencrypted (umask 077 only). Pushing them off the machine is left to the operator, so a lost VPS loses the backups too.
- **What the dump holds:** password hashes and API keys. The keys are encrypted with the master key, which is stored separately, so that is acceptable.
- **Decision for the user:** accept tar plus manual offsite copy, or add an optional restic step (e.g. when `RESTIC_REPOSITORY` is set).

## Low

- **L1. Postgres version.** `docs/web-server.md:31` requires Postgres 17, but `apt install postgresql` (line 50) installs 15 on Debian 12 and 16 on Ubuntu 24.04. The migrations only need PG13+ (`gen_random_uuid`, citext). Either say "PG 15+ (CI tests 17)" or add the PGDG install step. CI only covers 17.
- **L2. Restore leaks the DB password to `ps`.** At `docs/web-server.md:166`, `pg_restore -d "$(sudo cat /etc/ainovel/database-url)"` puts the URL with its password in argv, where other users can read it. `backup.sh` already documents this risk; restore should too, or connect as postgres instead: `sudo -u postgres pg_restore --no-owner --role=ainovel -d ainovel ...`.
- **L3. Backup fails before the first user exists.** `deploy/backup.sh:24` exits 1 when `users/` doesn't exist yet (fresh install), so cron sends error mail until someone signs up. Consider skipping the tar with a message instead.
- **L4. Open PWA tabs may not see new versions.** The SW only checks for updates on full navigation, and the SPA rarely does one. A long-open PWA may not see a new version until it is relaunched. Optionally call `r.update()` hourly via `onRegisteredSW` in `web/src/lib/pwa-update.ts`.
- **L5. Subcommand help still needs config.** e40b7c8 only handles a single help argument. `serve invite --help` and `serve invite create --help` still require config or print "--help thiếu giá trị".
- **L6. Plan status ahead of evidence.** The phase-06 frontmatter says `status: completed` and plan.md says "Completed", while 3 of 5 success criteria are still unchecked (staging, PWA install, CI run). Recommend "In review" or keeping the manual items visible. I did not edit the plan.
- **L7. Restart loop on config errors.** With `Restart=on-failure` and `RestartSec=3`, a config error makes the service restart every 3s indefinitely: 4 starts in 10s never trips the default start limit. Consider `StartLimitIntervalSec=60` with `StartLimitBurst=5` in `[Unit]`.

## Verified OK (no action)

- **(b) Shutdown timing.** The server's `stopCtx` is 60s: HTTP shutdown up to 32s, then worker terminate at 10s grace + 10s, then SIGKILL fallback. That fits inside `TimeoutStopSec=90`. `KillMode=mixed` sends SIGTERM to the server only and SIGKILL to the cgroup at the end. Workers use `Setpgid` and are still in the unit cgroup.
- **(b) Hardening does not block the worker.** fork/exec works (no `SystemCallFilter`). `RestrictNamespaces` does not affect `Setpgid`. The worker's `HOME` is under `DataDir`, so `ProtectHome` is fine. Writing `/proc/self/oom_score_adj` is not covered by `ProtectKernelTunables`. `StateDirectory` together with `ReadWritePaths` is consistent.
- **(b) Credentials.** `LoadCredential` files are 0400 and owned by the service user, so `readSecretFile` gives no warning.
- **(c) Client IP.** Caddy has `trusted_proxies` = Cloudflare ranges and `client_ip_headers CF-Connecting-IP`, then `header_up X-Forwarded-For {client_ip}` (overwrite). The server has `AINOVEL_TRUSTED_PROXIES=127.0.0.1/32` and `AINOVEL_CLIENT_IP_HEADER=X-Forwarded-For`. `fromForwardedFor` takes the single rightmost value. The Cloudflare ranges match the current published list.
- **(c) SSE and headers.** SSE runs with `flush_interval -1` and no encode. The server also sets `no-transform` and `X-Accel-Buffering`. Caddy sets only HSTS and removes `Server`. The app sets CSP, XFO, nosniff and Referrer-Policy itself, so nothing is duplicated or conflicting.
- **(d) backup.sh.** It uses `set -euo pipefail` and `umask 077`, prints no secrets, cleans up with a `trap`, and validates `KEEP`. Rotation globs only match `db-*`/`users-*`. pipefail is safe because the glob always matches the files that were just moved in.
- **(e) web.yml.** `pnpm/action-setup@v4` uses `package_json_file: web/package.json`, which reads `packageManager: pnpm@12.11.2`. The deploy report still says `version:` is pinned, but the committed file no longer does. The Postgres 17 service is healthy and `AINOVEL_TEST_DATABASE_URL` matches `dbtest.EnvURL`. Not yet proven: action-setup@v4 with pnpm 12's lockfile containing `packageManagerDependencies`, since only a real CI run will show it.
- **(f) PWA.** I checked the built `dist/sw.js`. It only precaches build assets plus the manifest, and calls `NavigationRoute(index.html, {denylist:[/^\/api\//,/^\/healthz/]})`. It has no `runtimeCaching`, so `/api` and SSE never touch the SW. Export downloads are navigations under `/api/`, so they are excluded from the fallback. The update flow is `registerType: 'prompt'` with `injectRegister: false`, so there is no inline script and the CSP stays intact. The update toast offers Cập nhật / Để sau. Workbox handles the case where the `load` event has already fired (`readyState` check). The manifest has `lang` vi, `standalone`, 192/512 icons and maskable. Icon URLs appear twice in the precache with the same revision; this is harmless.
- **(g) Docs and README.** The env var table matches `config.go` exactly, including dev-only variables and defaults (budget 50). The invite command is correct. The upgrade order is stop → replace binary → start. The README section is present.

## Recommended actions, in order

1. Add `OOMPolicy=continue` (H2) and fix the docs troubleshooting row.
2. Fix the TLS path in the Caddyfile and docs (H1).
3. Rewrite the SYNC.md go.mod recipe to pin versions (M1).
4. Align the export and proxy timeouts (M2), together with the agent editing export.
5. Get the user's decision on tar vs restic (M3).
6. Low items as convenient. Run web.yml in real CI and do the staging checks, which are still manual.

## Unresolved questions

- Will the user accept local tar backups instead of restic?
- Which TLS mode is intended for production: Origin Certificate or ACME?

Status: DONE_WITH_CONCERNS
Summary: The artifacts are syntactically valid and verified with real tools: caddy adapt, systemd-analyze, shellcheck, actionlint, the web gate and Go race tests all pass. Two deploy-level defects remain: TLS cannot be issued under the documented firewall, and the default OOMPolicy restarts the whole service when one worker OOMs.
Concerns: H1/H2 must be fixed before the staging deploy. M1 can break builds after the next upstream merge. CI web.yml and the PWA on real devices have not been run yet.
