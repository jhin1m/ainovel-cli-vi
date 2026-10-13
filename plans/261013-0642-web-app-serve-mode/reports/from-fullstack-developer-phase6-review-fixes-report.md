# Phase 6 review fixes

| Finding | Fix |
|---|---|
| H1 | `deploy/Caddyfile`: `tls /etc/caddy/cf-origin.pem /etc/caddy/cf-origin.key`, header comment updated. `docs/web-server.md` section 5: create Origin Cert, install (0644/0640 root:caddy), Full (strict), `caddy validate`, ufw example for 443 from CF ranges only, 525/526 troubleshooting row. |
| H2 | `ainovel-serve.service`: `OOMPolicy=continue`, `OOMScoreAdjust=-500`, comment fixed; docs + troubleshooting row. |
| L7 | `StartLimitIntervalSec=60`, `StartLimitBurst=5` in [Unit]. |
| M1 | `SYNC.md`: `go get` pinned (pgx 5.11.0, goose 3.27.0, x/crypto 0.55.0, x/time 0.15.0) before `go mod tidy`, check `go` line stays 1.25.x. |
| M2 | `exportTimeout` 2m -> 80s (`internal/server/books/handlers.go`); Caddy `response_header_timeout` 90s; docs note ordering 80 < 90 < 100. No test depended on the constant. |
| M3 | Kept tar. Docs: backup must be pushed off the VPS (rsync/rclone/restic paragraph). `backup.sh`: `chmod 0700` dir, `chmod 0600` files (umask 077 already present). |
| L1 | Docs: Postgres >= 13. |
| L2 | Restore uses `sudo -u postgres pg_restore -d ainovel`, no password in argv. |
| L3 | `backup.sh` skips tar with a message when `users/` is missing (tested with fake pg_dump: exit 0, dump 0600, dir 0700). |
| L5 | `wantsHelp` in `internal/server/cmd.go` (help, or -h/--help last arg incl. `invite`, `invite create`); test extended in `cmd_test.go`. |
| L4 | `schedulePeriodicUpdate` (60 min, `registration.update()`, errors swallowed) wired via `onRegisteredSW`; fake-timer test added. |
| L6 | Skipped as asked. |

## Gate
- gofmt clean, go vet clean, go test ./... pass.
- Race tests (server, worker) with Postgres 55432: pass.
- web typecheck, lint, build pass; vitest 63 tests pass.
- `bash -n backup.sh` ok; shellcheck (docker) 0 findings.
- `caddy adapt` (docker caddy:2) with a self-signed cert in place of the Origin Cert: adapts, only the "not formatted" warning.
- `systemd-analyze verify` (debian:12 docker): only complaint is missing `/usr/local/bin/ainovel-cli` (expected in container); no directive errors.
- ci.yml, .goreleaser.yml, Dockerfile untouched.

Status: DONE
Summary: All listed findings fixed except L6 (skipped as instructed); gate green.
Concerns: Real Origin Cert flow and OOMPolicy behavior not tested on a live host. `docs/journals/` is untracked but not created by this work. SYNC.md pins are literal version numbers and need updating if go.mod pins change.
