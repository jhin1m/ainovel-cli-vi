# fakeworker report

Built: `internal/server/jobs/fakeworker/{main.go,main_test.go}`.

Invoke: `fakeworker worker --version V` with cwd = book dir; stdin JSON Lines (init, then commands), stdout api.WorkerMessage lines.

- Story: real `store` (book meta, premise, 3-chapter outline, progress, chapters/NN.md, summaries). Cost 0.40/chapter in `meta/fake_cost.json`.
- Actions: start/resume/advance/steer; review -> review_wait, auto -> complete; budget -> stop_reason budget.
- Commands: abort (stop_reason user, mid-chapter draft discarded), shutdown (reason shutdown, <1s), steer (USER event), set_advance_mode; stdin EOF/SIGTERM/SIGINT = shutdown.
- Markers in prompt: `[fake:fast]` (~50ms/chapter), `[fake:crash]` (stderr + exit 3, no exit msg), `[fake:hang]` (ignores shutdown/SIGTERM/EOF), `[fake:lockbusy]` (first attempt lock_busy + exit 1, via `.fake_lockbusy_count`), `[fake:error]` (1 chapter then stop_reason error "lỗi giả lập").
- Signal handlers installed before hello; supervisor must wait for hello before sending SIGTERM to hang scenario.
- Init errors (protocol/book_dir mismatch) -> error config_invalid + exit error, code 1.

Tests: `go test -race -count=3` green; gofmt, go vet, go build ./... clean.
