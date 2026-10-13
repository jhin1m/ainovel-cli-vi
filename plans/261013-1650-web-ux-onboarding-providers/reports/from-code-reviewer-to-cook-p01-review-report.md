# Code review: phase 1, open signup and key creation without password

## Scope
- Files: 19 modified, 1 deleted (`internal/server/admin/invite.go`), 1 new (`internal/server/db/migrations/0004_drop_invite_codes.sql`). About 316 lines added, 294 removed.
- Focus: uncommitted diff on `feat/web-app`, checked against `phase-01-backend-open-signup-and-key-auth.md` (Requirements and Success Criteria).

## Verification (all run in this review)
| Check | Result |
|---|---|
| `go build ./... && go vet ./internal/server/...` | pass |
| `go test -count=1 ./internal/server/...` with `AINOVEL_TEST_DATABASE_URL` set | pass. The new tests ran and were not skipped (`-v` shows PASS for `TestMigration0004DropInviteCodesUpDown`, `TestSignupDailyLimitPerIP`, `TestKeyCreateLimits`, `TestCommandRejectsSubcommands`) |
| `go test -race ./internal/server/auth/ ./internal/server/keys/` | pass |
| `pnpm -C web typecheck && lint && test` | pass (63/63) |
| Grep for "invite" / "mã mời" in `internal/server web/src`, minus the allowed exceptions | empty |

## Acceptance criteria
| Criterion | Status | Evidence |
|---|---|---|
| No "invite" left in `internal/server` or `web/src` outside the allowed files | Met | grep is empty. `docs/web-server.md:23,152-160,211` and `docs/architecture.md:464` still mention it, but the plan defers docs to phase 12 |
| `serve invite create` reports an unsupported subcommand | Met | `cmd.go:39-43` exits 2 before `LoadConfig`. The test uses `admin create` because of the grep rule, and the code path is the same generic one |
| 0004 Up/Down/Up | Met | Down is byte-identical to `0002_auth.sql:22-28`. The test checks `to_regclass` and the goose version at 4, 3, 4 |
| Adding a key needs no password; changing the password still does | Met | `keys/handlers.go:103-111`. The flow test still checks that change-password with the wrong password returns 403 (`auth_flow_test.go:90`) |
| 30 key creations/day/user, cap of 20 keys, 10 signups/day/IP, with tests | Met | See the limiter and cap notes below |
| Verify commands pass | Met | `pnpm build` and `make gen` were not run, as instructed |

Contract changes are only the intended ones. `invite_code` and `current_password` are removed (old SPA tabs will get 400 for unknown fields, as the plan accepts). `invite_required`/`invite_invalid` are gone. There is a new 409 `key_limit`, and both new limiters return 429 `rate_limited`. `Repo.Create` gained a `maxKeys` parameter and has only one caller (`keys/handlers.go:107`).

## Critical
None.

## High
None.

## Medium

### M1. A full daily limiter map blocks every new IP from signing up for up to 24h
- Where: `internal/server/auth/ratelimit.go:60-66` together with `idle = max(10m, per)` (`:42`), as used by `signupLimiter` (`auth/handlers.go:52`).
- How it fails:
  - When a key is new and `len(entries) >= 100_000`, `Allow` sweeps only entries idle longer than `l.idle`, then returns `false`.
  - With the minute limiters, a flood heals itself in about 10 minutes. With the 24h signup limiter, nothing can be swept for 24h after the last request from each key.
  - An attacker with about 100k distinct IPv6 /64s (a couple of /48s, or a rotating proxy pool) sends valid-looking signup payloads. `Allow` runs before the argon2 slot wait, so entries are created at network speed and are not throttled by `hashSlots` (4).
  - After that, signups from every IP that is not already tracked get 429 for up to a day. The hourly sweep does not help.
- This is new exposure: the overflow policy predates this diff, but the 24h retention turns a 10-minute blip into a day-long outage. Cloudflare rules (phase 12) reduce the risk but are not in the code.
- Fix (pick one):
  - (a) Give the limiter an overflow policy. For long-window limiters, evict an arbitrary entry when full (Go map iteration order is random), which resets only one key's counter. Or fail open, since `ipLimiter` 30/min and Cloudflare still apply.
  - (b) Keep a separate, smaller cap for the daily limiter and log a warning on overflow so operators notice.
  - Add a unit test that fills the map and checks that a new key is still allowed (or whatever policy is chosen).

### M2. The daily 429 message says "Thao tác quá nhanh, thử lại sau" and is misleading
- Where: `httpx/json.go:42`, reused by `auth/handlers.go:228` and `keys/handlers.go:104`.
- How it fails: a user who hits 10 signups/day or 30 key creations/day reads "action too fast, try again later" and keeps retrying, while `Retry-After` is about 2.4h (signup) or 48 min (keys). Retries also re-hit the limiter (see L1).
- Fix: return a dedicated message, for example `httpx.NewError(429, "rate_limited", "Đã vượt giới hạn trong ngày, thử lại sau vài giờ")` with `RetryAfter: wait`, keeping the code `rate_limited` so the contract stays the same. This could be deferred to the phase 6 key-dialog rework, but signup has no later phase planned.

## Low

### L1. Quota is spent on requests that do not succeed
- Signup (`auth/handlers.go:227`): the daily token is spent before the `email_taken` check and before the hash or insert, so a 409 or 500 still costs a slot.
- Key creation (`keys/handlers.go:103`): the token is spent before the cap check, so a user at 20 keys who retries burns their daily quota. The test codifies this deliberately ("Đã tiêu 21 lượt").
- This is acceptable for anti-abuse. Note it as intended, or use `Reserve` and cancel on 409.

### L2. The cap lock is stronger than needed
- Where: `keys/repo.go:80` uses `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`.
- `FOR UPDATE` conflicts with the `FOR KEY SHARE` lock that Postgres FK checks take, so the same user's session inserts (login elsewhere) and book inserts briefly wait behind it. The transaction is short (count plus insert), so the impact is tiny.
- Fix: `FOR NO KEY UPDATE` serializes concurrent `Repo.Create` calls just as well and does not block FK checks.
- The race itself is handled correctly. Under READ COMMITTED, the `count(*)` statement runs after the lock is acquired and sees rows committed by the previous holder. This beats the plan's `CountByUser` count-then-insert, which would race.

### L3. There is no concurrent test for the cap
- `TestKeyCreateLimits` is sequential, so a future regression (for example someone dropping the lock) would not be caught.
- Suggest a test that fires about 25 parallel `POST /api/keys` for a user at 18 keys and asserts exactly 2 return 201.

### L4. "20" is hard-coded in three places
- Where: `keys/handlers.go:24`, the message string at `keys/handlers.go:109`, and `web/src/lib/messages.ts` `keys.limit`.
- The web special case in `add-key-dialog.tsx` is redundant, because `errorMessage()` already shows the server message (`web/src/api/client.ts:125-127`). It was added because the plan asked for a string.
- Fix: build the server message with `fmt.Sprintf(..., maxKeysPerUser)`. Either drop `msg.keys.limit` or keep it, knowing the two copies can drift.

### L5. The token bucket allows more than "10 per day" in the first 24h
- Burst is n, and the bucket refills at n/per, so a fresh key can make about 2n requests in the first rolling 24h window (10 at once, then 1 every 2.4h).
- That is fine for anti-abuse. The comments ("10 lần/ngày") should say it is a token bucket so nobody later assumes a fixed window.

### L6. The migration test covers only 4→3→4
- 0002's Down drops `invite_codes` without `IF EXISTS`, and 0004 exists precisely so that a full `DownTo(0)` works. That full path is not exercised.
- Adding `provider.DownTo(ctx, 0)` followed by `UpTo(ctx, 4)` would prove the "goose down past 0002" claim in the 0004 comment.

## Edge cases checked, no issue
- **Sweep logic for the per-day limiter.** An entry idle for at least `per` has refilled fully (rate × per = n), so dropping it loses no state. `TestLimiterPerDay` checks that it survives 11 minutes and is gone after 25h. The `max` builtin is fine under go 1.25.5.
- **Concurrency.** The limiter is mutex-guarded. `s.keysH` is set in `routes()` (`server.go:148`) before `startBackground()` (`:207`) starts the sweep goroutine, so there is a happens-before edge. The race tests pass.
- **Signup transaction removal.** A single INSERT does not need a transaction. The 23505→409 mapping is kept, and session creation was already outside the transaction.
- **CLI help.** `serve -h extra` now exits 2 instead of 0. That matches the spec ("help only for exactly one arg"). No script or Makefile calls `serve invite`; the only references are in docs, which phase 12 covers.
- **Callers.** The books flow `addKey`, the restart-recovery test, and `newBookEnv` were updated. No test env signs up more than 10 users from 127.0.0.1, because the full suite passes.
- **Rollback hazard.** An old binary on a database that has 0004 returns 500 on signup. The plan documents this already (Risk/Rollback).

## Plan follow-ups (for the lead; plan files were not edited)
- Every phase 1 success criterion is met in code. The phase file still shows `status: pending` with unchecked boxes.
- Phase 12 must update `docs/web-server.md` (lines 23, 152-160, 211) and `docs/architecture.md:464`, which still mention `admin` and `serve invite create`.
- Phase 12 should also note: "Fail-closed limiter overflow makes the Cloudflare signup rule mandatory, not optional", unless M1 is fixed here.

## Unresolved questions
1. CGNAT: Vietnamese mobile carriers share IPv4 addresses widely, so 10 signups/day/IP could block real users behind one carrier NAT. That number is a user/product decision; confirm it was chosen with this in mind.
2. M1 needs a policy choice: fail open, evict one entry, or keep failing closed and rely on Cloudflare.

## Score: 8.5/10
The implementation is correct and tightly scoped. It chose the transaction plus lock over the plan's racy count-then-insert, and its tests prove the limits. Points are off for the 24h fail-closed overflow (M1) and the misleading daily 429 message (M2).

Status: DONE_WITH_CONCERNS
Summary: Phase 1 meets all acceptance criteria. Build, vet, Go tests with Postgres, the race tests and web checks all pass, and the contract changes are only the intended ones. The main concerns are that the 24h signup limiter fails closed for every new IP for a day once its 100k-entry map is full (Medium), and that the daily 429 message is misleading (Medium).
