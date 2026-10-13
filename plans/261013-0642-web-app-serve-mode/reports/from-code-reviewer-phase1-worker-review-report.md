# Code review — commit bc74c5c `ainovel-cli worker` (phase 1)

Scope: `internal/entry/worker/*`, `internal/server/api/*`, `cmd/ainovel-cli/main.go` (~2.3k LOC incl. tests).
Verification run: `go build ./...`, `go vet` (worker/api/cmd) clean, `gofmt -l` clean, `go test -race -count=10 ./internal/entry/worker/...` green, `go test ./...` green.

**Score: 6.5/10.** Concurrency skeleton (non-blocking reader, sticky abort, one writer, Close-before-exit) is solid and config lockdown is good. But `stop_reason` disagrees with host semantics in ways the table test can't catch (the test builds snapshots by hand), and there is one real ordering race at startup.

---

## High

### H1. `hold` is effectively unreachable. In auto mode a hold stop is reported as `error`
`stop_reason.go:27-28` checks `HasAdvanceHold`, but the gate **clears the hold before it pauses**: `advance_gate.go:86-98` runs `ClearAdvanceHold`, then `pauseNow`. By the time Done fires, `RunMeta.AdvanceHold == nil`, so the snapshot has `HasAdvanceHold=false`.
Failure scenario: in auto mode the user steers "dừng sau chương 5". The arbiter sets `hold{After:chapter, Target:5}`. After chapter 5 commits, the gate consumes the hold and pauses, then Done fires. `stopReason` falls through to `StopError`, so the worker sends `exit{stopped, stop_reason:"error", error:<stale lastErr or "Engine dừng không rõ lý do">}`. The server will mark the book failed after it stopped exactly as the user asked. In review mode the same stop shows up as `review_wait` (acceptable by luck).
`HasAdvanceHold=true` at stop only happens in rare paths (a hold applied while stopped with no restart). The test cases `"review có hold"` and `"hold ở auto"` assert a state the host never produces at that moment, so they pass without proving anything.
Fix: record why the engine paused at the source. The smallest change is a read-only host field: `abortWithEvent` already receives a reason, so tag the kind (user / budget / gate-hold / gate-review / gate-fail / engine-pause) and expose it in `UISnapshot`. The alternative is for the worker to read the last SYSTEM pause event before Done, which is fragile. At minimum, auto mode with `!aborted && !budget` should not default to `error`.

### H2. A runtime command can run before the init action
`run.go:282-286`: `go s.dispatch(...)` starts **before** `s.jobs <- job{action:true}`. `scanLines` may already be holding the next stdin line (for example a `steer` or `set_advance_mode` the server sends right after `init`). The dispatch goroutine can enqueue it first, so the executor runs `Steer`/`SetAdvanceMode` before `StartPrepared`/`Resume`/`Continue`.
Failure scenario: `init{action:start}` followed immediately by `steer`. `Steer` → `doIntervention` runs against a store with no StartPrompt/plan (`CollectInterventionFacts` error, or a decision made on empty facts). A `set_advance_mode` that lands first is less harmful, but the order is still undefined.
Fix: enqueue the action job before starting `dispatch`. `jobs` is buffered, so this cannot block.

## Medium

### M1. `review_wait` also swallows control errors and the engine's own pauses
`stop_reason.go:29-30` treats any non-running review/writing stop as waiting for review. That also covers:
- `gate.fail` (`advance_gate.go:215-217`), e.g. "giấy phép chương X không khớp", "PendingCommit xung đột";
- `engine.pauseWithNotify` (`engine.go:819-826`), the stalemate breaker.
Both pause through `abortWithEvent` with level `info`/`warn`, so `lastErr` never captures them either. The server will show "chờ duyệt" and offer `/next`, which hits the same fault. Same root cause and fix as H1 (the pause kind).

### M2. `review_wait` ignores `PendingSteer`
The TUI headline (`panels_sidebar.go:222-227`) checks `PendingSteer` **before** the review condition. The engine's exit defer puts a dispatched intervention back into `PendingSteer` (`engine.go:162-171`). If the worker reports `review_wait`, the server's next action is `advance`, and `AdvanceOneChapter` (`host.go:824-887`) never consumes `PendingSteer`. The user's intervention is silently postponed. Fix: return a distinct reason (e.g. `pending_steer`, to be answered with `resume`), or document that the server must route to `resume` when `snapshot.pending_steer != ""`.

### M3. `exit.error` can be stale or unrelated
`run.go:480-481` keeps the last ERROR event of the whole run, which may be a transient tool error from hours earlier that was retried successfully. `stop()` (`run.go:510-514`) attaches it to `stop_reason:error`. Reset `lastErr` when the engine (re)starts, or keep only errors after the last `IsRunning` transition.

### M4. A full `Snapshot()` every second per worker
`run.go:332` calls `eng.Snapshot()` on every 1s tick. `Snapshot` runs `fillDetails` (`host.go:1261+`), which loads the premise, outline, compass and layered outline, plus `LoadSummaryTitle` per outline chapter, and `resumeLabel`. That is O(chapters) file reads per worker per second, with N workers on one server, and most of it is discarded by `mapSnapshot`. Raise `pollInterval` (the spec says 3s), or add a cheap state probe (lifecycle + progress only) for change detection and build the full snapshot only when sending.

### M5. Shutdown can write to the book after the lock is released
`Host.Close` (`host.go:935-970`) waits for `asyncWG` and the engine, but **not** for synchronous host calls running on the executor:
- `StartPrepared` after its arbiter returns (`SetPlanStart`, `Decisions.Append`);
- `Resume` → `doIntervention` (synchronous, not `runAsync`);
- queued executor jobs after shutdown. `SetAdvanceMode` (`host.go:802`) has no `closing` check and writes RunMeta.
These writes can land after `bookLease.Close()`, or be cut mid-write by `os.Exit`, while the server may already have spawned the next worker. The window is small but real. Fix on the worker side: set a `stopping` flag before `closeEngine` and have `execute` drop queued jobs. Wait briefly (bounded by `closeGrace`) for the in-flight job before `Close`, or after it and before `finish`.

### M6. A stalled stdout blocks shutdown
`writer.send` (`run.go:99-106`) holds the mutex across a blocking `Encode`. If the server stops reading stdout, `loop` blocks inside `sendSnapshot` and never sees `ctx.Done()`/`shutdownCh`, and `dispatch` blocks on its ack, so SIGTERM is ignored. This is acceptable only if phase 4 always drains stdout and escalates to SIGKILL. Write that requirement into the phase-4 plan.

## Low

- **L1. `closeGrace` timeout means `exit` is sent while the lock is still held** (`run.go:556-561`). After a 20s Close timeout the server receives `exit`, respawns, and gets `lock_busy`. The server should treat process exit (`Wait`), not the `exit` message, as authoritative for releasing the lock.
- **L2. Pricing is not tied to the selected model** (`config.go:44-62`, `protocol.go:139-144`). Nothing checks `pricing.id == llm.model` or that providers match. `Resolve(model)` does fuzzy `Contains` matching, so a mismatched pricing entry can pass the check through some other baseline entry while the merged price is never used. Assert equality, or assert that `Resolve` returns the merged entry.
- **L3. Abort stickiness does not cover runtime restarts.** After `abort`, a later runtime `steer` whose decision is reopen/dispatch restarts the engine (`host.go:707-711`), and nothing re-aborts it. The usual path for these commands is fine; this is the edge case.
- **L4. `ErrCodeBusy` is declared but unused.** The busy ack (`run.go:381`) carries only `Error`. Either drop the constant or send `error{code:busy}`.
- **L5. Shared home-dir state across tenants.** `rules.EnsureHomeRulesDir`, `~/.ainovel/style`, `~/.ainovel/rules` and the pricing cache in `DefaultConfigDir()` all belong to the server's OS user, so they are global to every web user, and concurrent workers rewrite `README.txt` non-atomically. Not a leak today (admin-controlled), but phase 4 should give each worker its own `HOME` and a scrubbed env. The env scrub also stops inherited provider SDK env vars (base URL, key) from leaking into the worker.
- **L6. Test gaps.** The real-host test only covers `nothing_to_resume`: no LLM path and no provider-error path, and stderr is not asserted although the spec asks for it. `host.New` makes the test fetch OpenRouter pricing (`host.go:156`), a network dependency in CI. The fake `Close` does not model `asyncWG.Wait`, so "shutdown < 1s" proves only that the worker doesn't wait on the executor, not that the real `Close` returns quickly. The manual smoke (step 8) is still pending, yet the phase and plan are marked Completed.

## Checks requested

| Item | Result |
|---|---|
| (a) stop_reason table tested | Yes, but `hold` is unreachable as modeled (H1) and review_wait is over-broad (M1/M2) |
| (a) shutdown < 1s with a hanging steer | Met with the fake (test passes under `-race` ×10); real path depends on runCtx cancellation, smoke pending |
| (a) abort during StartPrepared | Met. Ordering is safe: the store happens before `Abort`, and the loop re-checks after `actionCh`. No lost-abort window |
| (a) stdin EOF → Close | Met (`dispatch` → `requestShutdown` → `shutdown` → `closeEngine`) |
| (a) key not leaked | stdout (all harness tests) and files are covered. `decodeInit` sanitizes errors, `ValidateBase` messages contain no values, the key never goes into env, `Notify` is off so there is no `sh -c` |
| (b) headless/TUI regression | None. The intercept sits before `parseCLIOptions`, positional args already `die` (`main.go:112-114`), and `go test ./...` is green |
| (c) races / send-on-closed / deadlock | No send on a closed channel (`jobs` is never closed; a closed `Done` is handled with `!ok`). `actionCh`/`jobDone` buffers prevent executor blocking. A stale Done is guarded by the `IsRunning` reset. Real issues: H2, M5, M6. Executor and dispatch goroutines leak after `run` returns, which only matters in tests |
| (d) config lockdown | Good: allowlist, preset BaseURL, `Roles` nil, notify off, `DisallowUnknownFields` applied to nested structs, trailing data rejected. Note that Go json matches keys case-insensitively; this is harmless because unknown keys are still rejected |
| (e) stopReason vs host | H1, M1, M2 |

## The `steer` → `Continue(merge)` deviation

**The deviation is sound, but the stated reason is not quite right.** If `Resume()` and `Steer()` both ran inside `runAction`, `inFlight` would keep the worker alive until `Steer` returned, so the "worker exits before Steer completes" risk would not occur. The real advantages are:
- It matches the TUI's stopped-state path (`model_update.go:321-322`, `Continue` when `!IsRunning`).
- One arbiter call decides over both texts, instead of `Resume` replaying the old `PendingSteer` and then a second call deciding on the new text.
- It is correct for `complete` books.

Accepted gaps compared with `Resume`:
- It skips `CheckConsistency` warnings and `ensureUserRules`.
- If the arbiter fails on the merged text, `clearPending` drops both texts (`host.go:676-683`). This matches TUI behaviour, where the user sees the failure event.
- The server must not offer `steer` for a book that has never started (empty store).

## Recommended actions (in order)
1. H2: enqueue the action before `go s.dispatch` (one-line move).
2. H1/M1: add a pause kind to the host snapshot and derive `hold` / `review_wait` / `error` from it. Replace the hand-built `hold` table cases with a real-host or gate-driven test.
3. M2: decide the `PendingSteer` contract (new stop reason, or a server rule) and document it in phase 4.
4. M5: stop executing queued jobs once shutdown begins, and bound-wait for the in-flight job.
5. M4: lower the snapshot polling cost.
6. M3, L1–L6 as you go. Carry M6 and L5 into the phase-4 supervisor spec.

## Unresolved questions
- Does phase 4 plan a per-worker `HOME` and a scrubbed env? If not, L5 becomes a cross-tenant config surface.
- Should `exit{reason:shutdown}` include a `stop_reason` beyond `user` (for example the last known state), so the server can restore the book status after SIGTERM?
