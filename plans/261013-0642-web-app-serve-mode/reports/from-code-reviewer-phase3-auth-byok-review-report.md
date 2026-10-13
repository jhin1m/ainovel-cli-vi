# Code review: phase 3 auth + BYOK keys (commit 6da0b8e)

Score: **7/10**. The crypto, sessions, cookies, SQL and the invite transaction are correct. The weak spot is the rate limiter. It keeps attacker-controlled keys with no size limit, it still trusts the forgeable CF header from phase 2, and it treats each IPv6 /128 as a separate client. Success criterion 4 ("brute force gets 429; nobody can lock another account") holds only for IPv4 clients behind a correctly configured proxy.

Verification: `go build ./...` and `go vet ./internal/server/...` pass. `go test -race ./internal/server/...` (excluding books/jobs) passes against PG17, and the integration tests ran rather than being skipped. I also ran a manual test with the real binary on a scratch DB (since dropped).

## Acceptance criteria
| Criterion | Status |
|---|---|
| Auth flow; invite that is used up or expired is rejected | PASS (tests plus a manual run) |
| Keys stored only as ciphertext; not in logs or responses | PASS (`TestAuthAndKeysFlow`; my server log grep found 0 password hits) |
| No path to use or decrypt another user's key | PASS for list/delete; `DecryptForBook` deferred to phase 4 by decision |
| Brute force gets 429; other accounts cannot be locked | PARTIAL (see H1, H2, M1, M2) |

## High

**H1. The login limiter keeps attacker-sized keys, so memory can be exhausted.** `auth/handlers.go:282-284`, `auth/ratelimit.go:36-47`
- The key is `ip + "|" + email`, and `email` has no length cap. Only `MaxBytes` (1 MiB) bounds it.
- Each request leaves an entry of up to about 1 MB in the map. Entries are swept only every hour, or when there are more than 100k entries.
- Measured: 150 logins with a 900 KB email from 5 XFF IPs grew RSS from 184 MB to 388 MB. It was still 388 MB after 130 s, so the memory is retained.
- One IP at 30/min can pin about 1.8 GB per sweep window. A few IPs can OOM the server.
- Fix: reject `len(email) > 254` with `errBadLogin` before calling `loginLimiter.Allow`. Also, or instead, use `sha256(ip|email)` as the key so every entry has a fixed size.

**H2. Phase-2 finding H1 is still open, and phase 3 now depends on it.** `httpx/clientip.go:18-22`
- `CF-Connecting-IP` is trusted from any trusted peer.
- Scenario: nginx/Caddy in front with `AINOVEL_TRUSTED_PROXIES=127.0.0.1`, or the origin is reachable without going through Cloudflare. The attacker sends a random `CF-Connecting-IP` on each request.
- Result: `ipLimiter` and `loginLimiter` are both bypassed, which allows unlimited guessing against one account and makes H1 easier.
- Fix: as recommended in the phase-2 report, read exactly one configured header.

## Medium

**M1. IPv6 is bucketed per /128.** `auth/handlers.go:166,284`
- Any client with a /64 has 2^64 rate-limit keys. Per-account brute force is then limited only by argon2 throughput.
- With only 4 hash slots (`auth/password.go:36`), the same traffic queues legitimate logins until the 30 s timeout. That is a login DoS for everyone.
- Fresh keys also push the map past 100k entries. After that, every insert runs an O(n) `sweepLocked` under the global mutex.
- Fix: key IPv6 by `/64` (`netip.PrefixFrom(a, 64).Masked()`), and put a hard cap on the number of entries.

**M2. A missing `AINOVEL_TRUSTED_PROXIES` setting silently breaks per-IP limiting.**
- The default listen address `127.0.0.1:8080` assumes a reverse proxy. If the operator leaves the variable unset, every client appears as 127.0.0.1.
- Result: one 30/min budget is shared by every signup and login. Anyone can lock out any account with 5 wrong passwords per minute, which breaks criterion 4.
- Fix: in `LoadConfig`, add a `Warnings` entry when `!Dev && len(TrustedProxies)==0`, or log once when an XFF header arrives from an untrusted loopback peer.

**M3. Password change is not atomic.** `auth/handlers.go:366-373`
- `UPDATE users` and `DeleteOthers` run as separate statements.
- Scenario: a DB error or timeout between them leaves the password changed while the attacker's session is still alive. The client gets a 500, and retrying with the old password returns 403.
- Fix: run both statements in `pgx.BeginFunc`. Optionally, also issue a new token for the current session.

## Low
- **L1.** Signup runs argon2 (`handlers.go:229`) before checking the invite (`:236`). Anyone without an invite can use up hash slots. Fix: add a cheap `SELECT 1 FROM invite_codes ...` pre-check and keep the authoritative `UPDATE` inside the transaction.
- **L2.** Signup with a valid invite returns `409 email_taken`, and the rollback does not use up the invite. One invite holder can enumerate emails at 30/min per IP. This is acceptable for invite-only signup; note it before opening public signup.
- **L3.** `keys/repo.go:33-47` `List` has no `defer rows.Close()`, so the early return on a `Scan` error leaks a pool connection. Fix: use `pgx.CollectRows`.
- **L4.** Prices refresh every 48 h, not 24 h. `keys/catalog.go:27-41`: the tick at T+24h finds the cache at age 24h−δ (`models/pricing.go:104`), reuses it, and skips the fetch.
- **L5.** `go mod tidy -diff` fails because `golang.org/x/crypto` and `golang.org/x/time` are imported directly but marked `// indirect`.
- **L6.** `id::text = $2` (`repo.go:55,85`) is safe and is parameterized, but an uppercase UUID returns 404. It is fine at current scale; prefer parsing the UUID and using `id = $2`.
- **L7.** `cmd.go` now loads config before dispatching subcommands. With config missing, `serve bogus` exits 1 with a config error, where phase 2 exited 2 with "unsupported subcommand". This is a minor behaviour change.
- **L8.** Missing tests:
  - no concurrent invite test (I checked manually: 12 parallel signups on a 1-use invite gave 1×201, 11×403, `used_count=1`);
  - "unpriced model hidden" is tested only with an empty registry, not with an entry that has price 0;
  - no limits test for email length or IPv6.

## Checked and found correct
- **Timing:** for an unknown email, the dummy hash uses the same argon2 parameters. A password over 128 characters is rejected before the user lookup, so the result does not depend on whether the email exists. The only extra cost is a one-time lazy `dummyHash`.
- **Sessions:** login deletes the session from the old cookie and issues a fresh 32-byte token. The DB stores only sha256.
- **Cookie (prod):** `__Host-`, HttpOnly, Secure, SameSite=Lax, Path=/, no Domain (tested). Logout and login are also protected against CSRF by `CrossOriginProtection`.
- **SQL:** both the idle check (`last_seen_at > now-7d`) and the absolute expiry (`expires_at > now`) use the Go clock consistently. `citext` is created in 0001. `PurgeExpired` matches the `Lookup` predicates.
- **Invite race:** the row lock on the `UPDATE ... WHERE used_count < max_uses` statement is re-checked under READ COMMITTED, so a last-use invite gives exactly one success (verified empirically). A rollback on `email_taken` does not use up the invite.
- **AES-GCM:** a fresh 12-byte random nonce on every seal, and AAD = `user_id|key_id`. The ID is generated before sealing and is lowercase canonical, which matches `id::text`. `Open` normalizes all errors.
- **No plaintext leaks:** JSON decode errors drop user-supplied values; `APIKey` has no plaintext field; responses are `no-store`.
- **argon2 parameter bounds** are enforced on verify (m ≤ 256 MiB, t ≤ 10).

## Phase-4 handoff notes
- `DecryptForBook` must build the AAD from `b.user_id::text || '|' || b.api_key_id::text`, the same canonical text form used at `Create`.
- `handleDelete` runs Get → inUse → Delete, which is a TOCTOU race with starting a book. Use FK `ON DELETE RESTRICT` and map 23503 to 409, or make the DELETE conditional.
- `Catalog.PricingFor(model)` ignores the provider. Check `Allowed(provider, model)` before using it.

## Unresolved questions
- Is IPv6 ingress expected? If Cloudflare is in front, every client IP is the CF-provided one, so M1 depends on H2 being fixed.
