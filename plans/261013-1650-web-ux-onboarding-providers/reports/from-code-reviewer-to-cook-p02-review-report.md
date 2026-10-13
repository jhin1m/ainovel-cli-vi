# Phase 2 review: Z.ai providers + subscription mode

**Score: 8/10.** The implementation is correct for the forward path. One High finding: rolling back with `goose Down` leaves the old binary unable to start. Phase-1 changes in the working tree were not reviewed.

## Scope
- Files: worker `providers.go`, `config.go`, `protocol.go`, `mapping.go`, `run.go` (+ tests); keys `models.go`, `catalog.go` (+ test); api `account.go`, `books.go`; jobs `supervisor.go`, `fakeworker/main.go` (+ test fake); books `handlers.go`, `repo.go`; migration `0005`; `db_test.go`; `books_flow_test.go`; web `types.gen.ts` + 2 test fixtures.
- Re-ran `go test ./internal/entry/worker/... ./internal/server/keys/...`: both pass. I tested the NOT VALID behaviour by hand against the test Postgres (see H1).

## Critical
None.

## High

**H1. Migration 0005 Down: after rollback, the old binary cannot boot if any subscription book is interrupted, queued or running.** `internal/server/db/migrations/0005_books_budget_subscription.sql:8-13`
- Postgres still checks a `NOT VALID` CHECK on every **UPDATE**, not just on INSERT. I confirmed this on the test DB: with `CHECK (budget_usd > 0) NOT VALID` in place, `UPDATE t SET status='queued'` on a row with budget 0 fails with `violates check constraint`.
- Why this is new evidence: the plan picked `NOT VALID` so that Down itself would not fail. It did not account for what the old binary does with existing rows after Down.
- What breaks on the old binary:
  - Graceful shutdown (`jobs/db.go:121`, drain) sets every running book to `interrupted`.
  - On startup, `recoverAll` (`jobs/db.go:134-154`) runs one transaction that does `UPDATE ... SET status='queued' WHERE status='interrupted'`. A single subscription book with budget 0 makes the whole transaction fail.
  - `Supervisor.Start` returns that error (`supervisor.go:95`), so `startBackground` fails (`server.go:258`) and the server does not start.
- Second failure mode: a queued subscription book makes `claim` (`jobs/db.go:52`) fail. `schedule` then returns the error (`supervisor.go:159-165`). Queued books are ordered by `updated_at`, so the oldest such book blocks every queued book behind it, for all users, on every tick.
- After Down, all other UPDATEs on these rows also return 500: delete (`status='deleting'`), pause, resume and patch.
- The comment on line 8 ("ràng buộc chỉ áp cho dòng ghi mới", i.e. "the constraint only applies to newly written rows") is wrong. Line 9 ("Binary cũ không đọc được", i.e. "the old binary cannot read them") is also wrong: the old binary can read these rows; it cannot update them.
- Fix options (lead to choose; this touches a plan decision):
  1. Down normalises the rows before adding the constraint, which also removes the need for `NOT VALID`:
     `UPDATE books SET budget_usd = 0.01 WHERE budget_usd = 0;` then `ADD CONSTRAINT ... CHECK (budget_usd > 0)` (validated). The old worker rejects `zai-coding` at init, so these books just end up `failed` and can be deleted. Up should then reset the value: `UPDATE books SET budget_usd = 0 WHERE provider = 'zai-coding';`. This is safe because `sendInit` already forces 0 and PATCH already returns 400.
  2. Keep `NOT VALID`, but in Down first run `UPDATE books SET status='failed', last_error='...' WHERE budget_usd = 0 AND status IN ('queued','running','interrupted')`. The phase-12 rollback docs must then say that these books can only be deleted with SQL (`DELETE`), not via the UI.
- Either way, fix the comment, and add a db_test step that runs an `UPDATE ... status` on the budget-0 row after Down.

## Medium

**M1. The recommended model for the Coding Plan uses 3x the quota.** `internal/server/keys/models.go:38,46`
- The comment says that because the plan is not billed per token, the strongest model is recommended. That reasoning is wrong.
- docs.z.ai/devpack/overview (fetched 2026-10-13) meters plan credits per token, with multipliers: GLM-5.3 is 6.9 in / 24 out, GLM-5.3-Flash is 2.3 in / 8 out.
- `glm-5.1` is routed to GLM-5.3, so it uses about 3x the quota of `glm-4.7` (routed to 5.3-Flash). Novel writing is output-heavy, so this matters.
- The plan's rule for `recommended` is "a cheap model that writes well". That points to `glm-4.7` for `zai-coding`. This is a product call; at minimum, correct the comment.

## Low

- **L1. Error ordering changed in create** (`books/handlers.go:177-195`). A request with an invalid budget and a missing or other user's key now gets 404 `key_not_found` instead of 400 `invalid_budget`. The cheap check also now runs after a DB lookup. The spec requires this (budget validity depends on the key's provider) and no existing test asserts the old order. Accepted, noted for the API changelog in phase 12.
- **L2. `IsSubscription` rebuilds the full provider list on every call** (`LookupProvider` → `Providers()` → `ProviderPresets()`). It is now called per scanned row (`books/repo.go:53`) and per snapshot (`worker/run.go:530`). This is cheap at current sizes. Optional: compute it once in `session` init.
- **L3. The UI shows "$0 / $0" for subscription books** (`book-card.tsx:51-55`, `book-screen.tsx:302`), and the budget dialog is still reachable, which leads to a 400 `budget_not_applicable`. This is expected until phases 8 and 9, but don't ship phase 2 alone to prod.
- **L4. `plan.md` is stale.** Lines 47 and 182 still say "giữ chữ ký `PricingFor(model)`" ("keep the `PricingFor(model)` signature"). The phase file allows changing the signature when `zai` needs its own pricing, and it does. Update plan.md so later phases don't revert it.
- **L5.** The `model_not_allowed` message ("cần có bảng giá để giới hạn ngân sách", i.e. "needs a price table to limit the budget") is misleading for subscription providers. Cosmetic.

## Explicit checks
- **(a) Success criteria.**
  - `/api/providers` returns zai-coding (subscription, has models), zai, and glm with label "GLM (bigmodel.cn)". Covered by `TestCatalogZaiAndRecommended` and `TestBookSubscription`. Met.
  - Allowlist is constant. Met.
  - Subscription book runs to completed with cost 0, limit 0, tokens kept, not stopped by budget. Met.
  - Comments cite docs sources and dates. Met.
  - Down runs with budget-0 rows. Met literally, but see H1.
  - Tests and web typecheck: green per lead, and the unit tests I re-ran pass.
- **(b) Non-subscription regressions.** None found.
  - `validBudget` is unchanged for non-subscription providers.
  - The PATCH budget path is unchanged apart from the guard placed before it.
  - `budget_exhausted` → `paused` CASE is unchanged.
  - `fakeworker` gates on `BudgetUSD > 0` only.
  - `budgetExhausted` (`stop_reason.go:10`) uses the raw engine snapshot, with limit 0 when budget is disabled.
- **(c) Contract changes.** Intentional and additive: `PricingFor(provider, model)` has one production caller plus the test fake; the new JSON fields are bools that are always present; fixtures are updated.
- **(d) No user input reaches BaseURL or Type.** Verified.
  - The key-create handler checks `LookupProvider`.
  - BaseURL and Type come only from the unexported `webProviders` map, read by value.
  - Book provider = key provider, and PATCH blocks `provider_mismatch`.
- **(e) `applyPricing` with the zai price override still resolves correctly.**
  - The override keeps the registry Provider/ID (`glm`/`glm-4.x`).
  - `MergeModels` overwrites prices in place on the process `DefaultRegistry`, so the engine's `resolveCost` (`host/usage.go:712`) uses zai prices.
  - The Provider/ID equality check still passes.
- **(f) Places that read engine cost directly.**
  - `TotalCostUSD` in `internal/entry/worker` appears only in `mapping.go` and `stop_reason.go` (the latter is correct to use the raw value).
  - The host budget sentinel is nil when the budget is disabled, so no "$" events are emitted.
  - `runEndBody` only goes to the notifier, which is disabled in the worker.
  - Export includes only `meta/book.json` and the outlines.
- **(g) Migration.**
  - Up is correct: it drops the default constraint name and validates `>= 0`, and the test checks `pg_constraint`.
  - Down: see H1.

## Recommended actions
1. Fix H1: pick option 1 or 2, fix the migration comment, and extend `TestMigration0005...` with an UPDATE after Down.
2. Decide M1 (`glm-4.7` vs `glm-5.1` as the recommended zai-coding model) and correct the comment.
3. Update the PricingFor wording in plan.md (L4).

## Unresolved questions
- H1 changes the Down strategy the plan chose. Does the lead accept option 1 (data normalisation) or option 2 (fail the rows + document SQL delete)?

Status: DONE_WITH_CONCERNS
Summary: Phase 2 meets its success criteria and causes no regressions for non-subscription books. The migration 0005 Down leaves `NOT VALID` rows that fail every later UPDATE, so after a rollback the old binary's `recoverAll` fails and the server does not start (High, verified on Postgres).
