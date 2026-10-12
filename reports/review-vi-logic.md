# Logic review: branch `vi` vs `upstream/main`

Scope: only behavior and logic changes. Translation quality is not covered. `go build ./...`, `go vet ./...`, and `go test ./...` all pass. Each finding was checked against the code path, and the lint false positives were reproduced with an overlay probe test (`go test -overlay`, nothing was written to the repo).

## Confirmed findings

### 1. [High] `english_residue` flags normal Vietnamese text as an error, which can force repeated polish passes
- File: `internal/rules/lint_vi.go:100` (the `englishWords` list) and `:83` (case filter). Severity is `SeverityError`.
- These sentences were reproduced as `english_residue` errors:
  - `the` is a real Vietnamese word: "giọng the thé" (shrill voice), "áo the" (gauze tunic).
  - Common urban/modern slang: "thả **like**", "**up** ảnh", "ngày **off**". `out` and `one` are also on the list.
  - Capitalized names and brands: "tên **Will**", "**The** Beatles". The check `tok[1:]==low[1:]` accepts any capitalized token, not only one at the start of a sentence.
- Failure path: `assets/prompts/vi/editor.md:113` says that `english_residue` at severity=error must produce at least one issue **and raise the verdict to polish**. A modern-setting book, or any chapter that uses "the thé", is sent to polish. If the writer keeps the word (it is correct), the same error comes back at every review. That means rewrite churn and wasted budget, and it can look like a polish loop. Commit is not blocked, because violations are only reported.
- Fix:
  - Remove `the`, `like`, `up`, `off`, `out`, and `one` from the list, at least.
  - Ignore capitalized tokens that are not at the start of a sentence, since they are proper nouns.
  - Optionally, only raise an error when there are ≥2 hits, or when a hit sits next to another English token or after a dash (the pattern seen in real leaks: "—not nhanh"). Otherwise report a warning.

### 2. [Medium] zh regression: TXT and EPUB export always use Vietnamese labels
- Files: `internal/host/exp/txt.go:131,133` and `volumeHeading`; `internal/host/exp/epub.go:126-128,133,178,198,216-218,245,249`.
- Export always writes `Chương N: …`, `Quyển N: …`, `xml:lang="vi"`, and `<dc:language>vi</dc:language>`. It also drops the `《name》` wrapper. None of this is gated on `lang.Current()`, so a book written with `language=zh` exports with Vietnamese chapter labels and wrong EPUB language metadata. Other zh paths (WordCount, Lint, countPhrase, stylestat, drafts) do keep upstream behavior, so this is inconsistent with them.
- Fix: choose the label formats and language tag from `lang.Current()`, keeping the zh strings in a `*_zh.go` file the same way `load_zh.go` does.

### 3. [Low] A crashed commit carried over from an upstream binary can get stuck because of the new foreshadow-id checks
- Files: `internal/tools/commit_validation.go:34,41` and `internal/tools/commit_chapter.go:129-142`.
- A non-rewrite `PendingCommit` at `StageStarted` replays its **frozen** payload through `validateCommitArgs`. If an upstream build saved a pending commit that uses a placeholder id (`new`, `tbd`, …) or a plant id that another chapter already owns, every retry now fails with `ErrToolArgs`. The auto-unfreeze path only runs when `existingPending.Rewrite` is true, so the book cannot move forward without fixing `meta/pending_commit.json` by hand. The comment says replay of history is not affected, but replay of a pending commit is affected.
- This only happens on upgrade, after a crash during commit. It cannot happen within this version, because validation runs before the pending commit is created and same-chapter replay is allowed.
- Fix: skip the two new checks when `existingPending != nil`, or extend the unfreeze branch to non-rewrite pending commits at `StageStarted`.

### 4. [Low, design hazard] An empty `LoadOptions.Language` silently means zh while the process default is vi
- File: `assets/load.go:98`.
- Every current caller sets `Language` (main.go:121, eval.go:108/122/129), so nothing is broken today. A future caller that uses `DefaultLoadOptions` without setting it would get Chinese prompts in a vi process with no error.
- Fix: set `Language: lang.Current()` in `DefaultLoadOptions`, or make the zero value fall back to `lang.Current()`. That second option needs the zh asset baseline tests to set the language explicitly.

## Areas checked with no issues
- **internal/lang**: `atomic.Value` is set once at startup, before any goroutine starts (main.go:116, eval.go). No test uses `t.Parallel()` together with `lang.Set`, so there is no race. Unknown config values are rejected in `Validate`, and `LanguageCode()` falls back to the default.
- **bootstrap**: language overlay merge, `FillDefaults` normalization.
- **assets/load.go**:
  - `localPath`, genre paths (`references/vi/genres/<style>/`), styles dir, and voice all resolve correctly.
  - `CheckLanguage` walks the base files and skips `vi/`.
  - `OverridePrompt` uses `b.Language`, so eval A/B arms stay symmetric.
- **domain.WordCount**: zh still counts runes. vi counting is correct (probe: "Ông ấy đi 3.000 km, tay-chân mỏi." = 8). Callers only use it for display, relative ratios (`WordCountAnomaly`), or vi-specific thresholds.
- **rules**:
  - `countPhrase` boundary logic is correct, including case folding ("Tuy nhiên … TUY NHIÊN" = 2).
  - zh still uses `strings.Count`, and `non_cjk_fragments` is correctly limited to zh.
  - Duplication thresholds are sane.
  - `CheckAgainstPrevious` is a no-op under zh and does not read from disk.
- **userrules/snapshot_language.go**:
  - Relocalization keeps user overrides.
  - vi→zh→vi round trips are harmless, and the snapshot is written atomically.
  - `commit_chapter.checkRules` reads the snapshot without relocalizing it, which is harmless: stale zh defaults just never match vi text.
- **ctxpack/token_estimate.go**: zh is unchanged and vi is the more conservative estimate. Note that `truncateJSONToTokens` can split a UTF-8 character, but that already happens upstream.
- **store/session.go** `chapterRe`: it only affects the session filename suffix. The submatch fallback is correct.
- **store/drafts.go**: thresholds and dialogue regex are gated by language, and zh is unchanged.
- **stylestat**: the profile is pinned per Tracker and `patternTotals` is resized if it does not match. zh only adds a `TrimSpace` on titles.
- **premise_structure.go**:
  - Vietnamese aliases, lowercase lookup, and stripping of the annotation after `(`/`:` all work.
  - zh keys are unchanged, and the output labels follow the language.
- **plan_chapter.go**: the rewrite-queue exception still goes through `ValidateChapterWork`, and `StartChapter` is skipped for queued chapters that are not the current one. No pointer hijack.
- **save_foundation.go**:
  - `SkeletonArcs` matches the existing `layeredStructurallyComplete` invariant.
  - `StalledOutline` only applies to full outline saves before the writing phase, and to `expand_next_arc`.
- **host/exp** title normalization: the regexes need digits or Roman numerals followed by a delimiter, so ordinary titles such as "Chương trình…" or "Hồi ức" are not stripped.
- **guard/summary_task_markers.go**: matches the router task texts (`flow/router.go:175,181,217,223`).

## Unresolved questions
- Is it intended that export (finding 2) is vi-only? If the fork drops zh export support, finding 2 becomes a docs note rather than a bug.

**Status:** DONE_WITH_CONCERNS
