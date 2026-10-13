# Code review: phase 11 (P0 book actions, web only)

Date: 2026-10-13. Reviewer: code-reviewer. Scope: the 19 web files listed by cook (other working-tree changes from earlier phases are out of scope).

## Score: 7.5/10

No critical or trust-boundary defects. All acceptance criteria are met, including the "Bổ sung" items. Gates pass: `typecheck`, `lint`, `test` (29 files, 251 tests) and `build` all green. What costs points: two dialogs lose their follow-up action if they are dismissed while a request is in flight, the live regions are oversized or inserted already filled, and two cross-layer UX gaps weaken the 0.1 and 0.2 flows that motivated this phase.

## Server facts verified
- `handlePatch` (internal/server/books/handlers.go:240-312): budget is refused when queued/running (L258); `budget_exhausted -> paused` happens only on a raise (L298); `provider_mismatch` (L289); key ownership is checked through `s.keys.Get(u.ID, ...)`.
- `handleResume` (L440-455): resume is allowed only from paused/failed/interrupted, so `RESUMABLE` is correct.
- `handleDelete` (L321-335): returns 202 and deletes in the background. `Repo.List` excludes `deleting` (repo.go:70).
- `stop_reason` is cleared only when the job is claimed as running (jobs/db.go:60). `queue()` does not clear it.
- `provider_rejected` uses only fixed messages: the 1113 message plus a fallback (worker/run.go:597-607). Both say "tạo truyện mới", so the banner's choice of primary action holds.

## Acceptance criteria
| Criterion | Status |
|---|---|
| Delete from book page, with confirm, then back to library | Met (book-screen.tsx:192-198, delete-book-dialog.tsx). Has a test. |
| Key missing: same-provider picker and continue | Met (status-banner.tsx:86-100, change-key-panel.tsx). Has a test. |
| provider_rejected: show cause and link to a new book with another provider | Met, with a caveat (see M4). Has a test. |
| Raise budget: one button restarts the book | Met (budget-dialog.tsx:88-97). Enter picks "Lưu và viết tiếp". Has a test. |
| "⋯" menu works by keyboard | Met (Enter/Space/↓/↑, Home/End, Esc returns focus, Tab closes). Has a test. |
| Danger button is not the Enter default | Met. There is no `<form>` and initial focus is on the panel. The test asserts it. |
| Panel always shows the key in use | Met (change-key-panel.tsx:78-83). |
| `keys.confirmBody` copy fixed | Met (messages.ts:456-457). |
| `book_limit` link | Met (new-book-form.tsx:116-123). Has a test. |
| Library `?action=budget/retry` and param cleared after use | Met (library-page.tsx:29-32,57; book-screen.tsx:153-163; route clears with `replace`). The router defaults `to` to `'.'` and inherits params (router-core router.ts:1960, 2011), so the cleanup navigate is correct. |
| Stepper hides "Chờ bạn duyệt" in auto mode | Met (progress-stepper.tsx:136). Has a test, including the waiting_review exception. |

## Critical
None.

## High
None.

## Medium

**M1. The delete dialog loses navigation and cache cleanup if it is dismissed while pending.** delete-book-dialog.tsx:21-27, 34
`del.mutate(undefined, { onSuccess })` uses per-call callbacks. While the request is pending, "Huỷ" stays enabled and Esc still closes the Dialog. If the user does either, `Body` unmounts, TanStack skips the per-call `onSuccess`, and the user stays on a book that is now `deleting`. The book then turns into a "Không tải được truyện" error once `finishDelete` removes the row. `ChangeKeyPanel` already documents this exact pitfall and uses `mutateAsync` to avoid it.
Fix:
```ts
const confirm = async () => {
  try { await del.mutateAsync() } catch { return }
  await navigate({ to: '/' })   // router.navigate keeps working after unmount
  forgetBook(qc, book.id)
}
```
Also disable "Huỷ" while `del.isPending`. Optionally add a `dismissible` prop to `Dialog` so Esc is ignored while pending.

**M2. The budget dialog has the same problem: dismissing during the PATCH drops the resume and the notice.** budget-dialog.tsx:54-69, 85
If the user clicks "Lưu và viết tiếp" and presses Esc or "Huỷ" before the PATCH returns, the budget is saved but resume never runs and nothing is announced. Fix: use the same `mutateAsync` sequence as `ChangeKeyPanel.apply`, and/or disable Cancel and Esc while `busy`.

**M3. The status banner live region is now atomic and wraps a whole form.** status-banner.tsx:30
`Box` is `<section role="status">`, and `role=status` implies `aria-atomic="true"`. The key-missing and rejected banners now contain `ChangeKeyPanel`: a select, buttons whose labels change ("Đang đổi key…"), the "Key đang dùng" line that fills in after `useKeys` resolves, and nested `role=alert`/`role=status`. Every one of those changes makes a screen reader re-read the entire banner. Fix: render `Box` as `<section aria-labelledby={titleId}>` (a region, not a live region). If title changes must be announced, put one small, always-mounted `role="status"` element next to it.

**M4. "Tạo truyện mới với provider khác" lands on a form that may preselect the same rejected key.** routes/_app/books/new.tsx:39-42, new-book-form.tsx:69
`NewBookForm` defaults to `keys.data[0]`. For a user with one Z.ai key, which is the common 1113 case, the primary call to action reopens the same provider and key. The fix is to pass the source provider and prefer the first key with a different provider:
```tsx
<NewBookForm initialIdea={source.data?.prompt} avoidProvider={source.data?.provider} ... />
// in form: const fallback = keys.data.find(k => k.provider !== avoidProvider) ?? keys.data[0]
```
This touches the form beyond the `book_limit` block, so the lead should decide. `initialIdea` already crossed that line for the same reason.

**M5 (cross-layer, phase forbids server edits; flag for lead). Deleting a book does not free a slot right away.** internal/server/books/repo.go:93-96
`Count` counts every row, including `status='deleting'`, and `finishDelete` can take about 30s. A user at `MaxBooksPerUser` follows the new flow (delete, then create) and hits `book_limit` again. The new link sends them to a library where the book is already gone. Fix it in a follow-up: add `AND status <> 'deleting'` to `Count`, or make the web form say "đang xoá, thử lại sau ít giây".

## Low

**L1. Duplicate notice in the rejected banner.** change-key-panel.tsx:60-63 and book-screen.tsx:222-230
After "Dùng key này" on a `provider_rejected` book, the status stays `failed` and `stop_reason` persists, so `RejectedBanner` stays mounted. Both the panel's own `role=status` and the page notice then show "Đã đổi key." and announce it twice. Fix: render the panel's local notice only when `onDone` is not provided, or drop `onDone` for the rejected banner.

**L2. Success notices are inserted already filled.** book-screen.tsx:227, change-key-panel.tsx:139
`<p role="status">` is conditionally mounted together with its text, and VoiceOver and some NVDA builds do not announce a live region that appears already populated. In addition, notices keyed to `status: 'queued'` vanish within about 1s, when the book goes running. Fix: keep an always-mounted `<div role="status">` and swap only its text. Clear the notice on a timer or on user action instead of on status equality.

**L3. `?action=retry` decides from possibly stale cached data.** book-screen.tsx:157-161
`statusAtOpen` is the first render's `book.status`. That can come from a cache entry up to gcTime old (`bookQuery` has no staleTime override). A stale `running` silently drops the retry, and a stale `interrupted` produces a `cannot_resume` error. Fix: run the effect only once `!bookQuery.isFetching`, or compare `dataUpdatedAt` against the mount time. `?action=budget` likewise opens even if the book is no longer `budget_exhausted`. That case is harmless because the server rejects a running book with a clear message.

**L4. StrictMode is not covered by tests.** `renderWithProviders` does not wrap `StrictMode`, but main.tsx does. The ref guard is correct under React 19 because refs survive the simulated remount. Add one test that renders `<StrictMode><BookScreen action="retry" …/></StrictMode>` and asserts a single POST, so the guard stays locked in.

**L5. Header "Tiếp tục" is still shown for `provider_rejected` books.** book-screen.tsx:208
It resumes with the same key and fails again at once with 1113. Consider hiding it when `stop_reason === 'provider_rejected'`, in line with the banner.

**L6. Changing the key while running/queued takes effect only on the next run.** book-screen.tsx:195
The server accepts the PATCH, but the worker reads the key only at startup, the same reason budget edits are refused while running. Either disable "Đổi key" while `RUNNING.has(status)` or add a hint saying the new key applies from the next run.

**L7. Wrong copy for `budget.saved` outside budget_exhausted/paused.** messages.ts:360
The text "Bấm Tiếp tục để AI viết tiếp" also appears for waiting_review (where the button is "Viết tiếp") and for completed (where there is no button). Pick the copy based on the returned status.

**L8. `RESUMABLE` lives in a UI component.** change-key-panel.tsx:12
`RESUMABLE` is a server-contract constant that `budget-dialog` and `book-screen` import from a UI panel. Move it to `@/api/status.ts`, next to `BOOK_STATUSES`.

**L9. The menu stays open if `disabled` flips while it is open** (book-actions-menu.tsx:17, e.g. the status turns `deleting` through SSE). Add `useEffect(() => { if (disabled) setOpen(false) }, [disabled])`.

**L10. No route-level test for `/books/new?from=`.** validateSearch, the loading gate, and the prefill from `bookQuery` are untested. Only the `initialIdea` prop is tested.

## Race and lifecycle checks
- **ChangeKeyPanel unmounting mid-flow (key-missing banner):** correct. `mutateAsync` promises resolve after unmount, hook-level `onSuccess` still writes the cache, and `onDone`/`onResumeError` go to `Loaded`, which stays mounted. The integration test covers it (book-actions.test.tsx:347).
- **`forgetBook` after navigation:** correct ordering. The removal is by prefix `['book', id]`, so chapters and outline are dropped too. It is lost only on dismiss (M1).
- **StrictMode double effects:** guarded by `actionUsed` ref (see L4).
- **Menu focus return:** `close(true)` focuses the button before `onSelect`, so `Dialog` captures the button as `prev` and focus returns to "⋯" on close. Tested (book-actions.test.tsx:312). Esc calls `stopPropagation`, so it does not reach document listeners.

## Security / data
- PATCH `api_key_id` is checked for ownership and provider on the server. DELETE goes through `s.load(r)`, which checks ownership.
- `?from=` fetches through the owner-scoped GET. A foreign id gets a 404 and the form stays empty. The prefill is a React-escaped textarea value.
- No secrets or PII exposed. Key labels show only `last4`.

## Metrics
- Typecheck: 0 errors. Lint: 0 issues. Tests: 251/251. Build: OK.
- Coverage was not measured. The new behavior is covered by behavioral tests that assert request sequences, bodies, focus and navigation. I found no phantom tests.

## Recommended actions (priority order)
1. M1 and M2: switch both dialogs to `mutateAsync` and block dismissal while pending.
2. M3: drop `role=status` from `Box`.
3. M4: lead decides on preferring a different provider's key when `from` is set.
4. M5: lead files a server follow-up so `Count` excludes `deleting`.
5. L1, L2, L5: small fixes to notices and the header button.

## Unresolved questions
- M4: may the form be changed beyond the `book_limit` block to preselect a key from another provider?
- M5: is a server follow-up allowed in this plan, or should it be deferred to P2?

Status: DONE_WITH_CONCERNS
Summary: Phase 11 meets every acceptance criterion and all gates pass. Two dialogs (delete, budget) drop their follow-up action when dismissed mid-request, and the banner live region is oversized. These should be fixed before shipping.
