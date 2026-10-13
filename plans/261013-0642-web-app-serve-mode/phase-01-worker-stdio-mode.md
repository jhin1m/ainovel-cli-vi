---
phase: 1
title: "Worker stdio mode"
status: pending
priority: P1
effort: "2d"
dependencies: []
---

# Phase 1: Worker stdio mode

## Overview

Thêm subcommand `ainovel-cli worker`: chạy **một lượt** engine cho đúng một truyện, nhận lệnh JSON Lines qua stdin, phát JSON Lines qua stdout, rồi **thoát mỗi khi engine dừng** (xong chương cần duyệt, hết ngân sách, người dùng dừng, hoàn thành, lỗi). Không đụng logic engine.

## Requirements

- Functional:
  - Message đầu tiên trên stdin là `init`. Worker **không** đọc `~/.ainovel/config.json` hay `./.ainovel/config.json`.
  - `init.action`:
    - `start` — truyện mới: `startup.PrepareQuick` → `PrepareUserRules(prompt)` → `StartPrepared(prompt)` (theo `internal/entry/headless/run.go:52-63`).
    - `resume` — `Resume()`; label rỗng (`internal/host/resume.go:151-153`, phase complete) → `exit{stop_reason:"nothing_to_resume"}`. **Không** gọi `ReplayQueue(0)` (đọc cả `queue.jsonl`, một dòng hỏng là chặn resume vĩnh viễn — `internal/store/runtime.go:58-71,167`).
    - `steer` (kèm `text`, dùng khi truyện đang dừng): nếu còn việc để resume → `Resume()` (tự xử lý `PendingSteer` cũ, `internal/host/host.go:590-604`) rồi `Steer(text)`; nếu đã hoàn thành → `Continue(text)`; nếu `RunMeta.PendingSteer` khác rỗng mà truyện đã hoàn thành → gộp hai đoạn rồi `Continue`. Khớp cách TUI làm (`internal/entry/tui/events.go:152-172`).
    - `advance` — `AdvanceOneChapter()` (tự `startEngine`, `internal/host/host.go:880`).
  - Mọi action: sau `host.New` gọi `SetAdvanceMode(init.advance_mode)` **trước** khi chạy (field này không có trong `bootstrap.Config`, chỉ nằm ở `RunMeta`, `internal/store/run_meta.go:48-75`).
  - Lệnh runtime trong lúc chạy: `steer`, `set_advance_mode`, `abort`, `shutdown`.
  - Phát: `hello`, `event`, `stream`, `snapshot` (mỗi 3s và khi `Phase`/`Flow`/`RuntimeState`/`CompletedCount` đổi), `ack`, `error`, `exit`.
  - `Done()` nhận được (ok=true) → gửi snapshot cuối → `exit{reason:"stopped", stop_reason}` → `Close()` → thoát. `stop_reason` tính trong worker:
    - `review_wait`: `AdvanceMode=="review" && !IsRunning && Phase=="writing" && !HasAdvanceHold` (đúng điều kiện TUI, `internal/entry/tui/panels_sidebar.go:235`).
    - `budget`: `TotalCostUSD >= BudgetLimitUSD > 0`.
    - `complete`: `Phase=="complete"`.
    - `hold`: `HasAdvanceHold` (dừng theo ý định của một lần can thiệp).
    - `user`: sau `abort`.
    - `error`: còn lại, kèm message cuối.
  - Lỗi `budget.Refuse()` khi khởi động (`internal/host/budget.go:122-130`) → `error{code:"budget_exhausted"}` + `exit{stop_reason:"budget"}`.
  - SIGTERM/SIGINT hoặc stdin EOF → như `shutdown`.
- Non-functional:
  - **Đọc stdin không bao giờ chờ engine.** Luồng đọc chỉ parse; `abort` và `shutdown` xử lý ngay trên luồng đọc; các lệnh khác vào một executor tuần tự (goroutine riêng), `ack` khi gọi xong. Cờ `abortRequested` dính: được kiểm lại ngay sau khi `StartPrepared`/`Resume`/`AdvanceOneChapter` trả về (lúc đó `Abort()` mới có tác dụng, `internal/host/host.go:907-920`). `shutdown` chạy `Close()` ở goroutine riêng (`Close` huỷ `runCtx`, cắt cả lời gọi arbiter đang treo).
  - Không log nội dung `init`; key không vào env; có test khẳng định key không xuất hiện trong stdout/stderr/`logs/worker.log`.
  - Worker kiểm `os.Getwd() == book_dir`, sai thì từ chối (rules dự án đọc theo cwd, `internal/rules/loader.go:69-74`; `EffectiveConfigPath` cũng xét `./.ainovel`).
  - Linux: ghi `500` vào `/proc/self/oom_score_adj` (OOM ưu tiên giết worker chứ không phải server); đặt `GOMEMLIMIT` mềm qua `debug.SetMemoryLimit`.
  - `--headless` và TUI giữ nguyên hành vi.

## Architecture

Wire types dùng chung với server nằm ở `internal/server/api` (package chỉ có struct + json tag, không phụ thuộc gì) — tạo ở phase này.

```jsonc
// stdin → worker
{"type":"init","protocol":1,"action":"start|resume|steer|advance",
 "prompt":"...","text":"...","book_dir":"/data/users/u1/books/b1",
 "advance_mode":"auto|review",
 "llm":{"provider":"openrouter","model":"...","reasoning_effort":"","api_key":"<secret>"},
 "pricing":{...models.ModelEntry của model đã chọn...},
 "language":"vi","style":"","budget_usd":5}
{"type":"steer","id":"c1","text":"..."}
{"type":"set_advance_mode","id":"c2","mode":"review"}
{"type":"abort","id":"c3"}
{"type":"shutdown","id":"c4"}

// worker → stdout
{"type":"hello","protocol":1,"version":"<build version>"}
{"type":"event","event":{"id":"...","time":"...","category":"TOOL","agent":"writer","summary":"...","detail":"...","kind":"...","level":"...","depth":0,"duration_ms":0,"failed":false,"finished":true}}
{"type":"stream","delta":"...","thinking":false}      // sentinel → {"type":"stream","clear":true}
{"type":"snapshot","snapshot":{...api.BookSnapshot...}}
{"type":"ack","id":"c1","ok":true,"error":""}
{"type":"error","code":"config_invalid|lock_busy|budget_exhausted|model_unpriced|...","message":"..."}
{"type":"exit","reason":"stopped|shutdown|error","stop_reason":"review_wait|budget|complete|hold|user|nothing_to_resume|error","error":""}
```

- `api.Event`: map từ `host.Event` (`internal/host/events.go:15-29`, không có json tag) — start và finish của cùng một lời gọi **dùng chung `id`** (`events.go:7-14`), client phải upsert theo `id`.
- `api.BookSnapshot`: map các field UI cần từ `host.UISnapshot` (`events.go:50-122`): BookTitle, Phase, Flow, RuntimeState, IsRunning, StatusLabel, CurrentChapter, TotalChapters, CompletedCount, TotalWordCount, AdvanceMode, HasAdvanceHold, AdvanceHoldReason, PendingSteer, Agents (rút gọn), TotalCostUSD, BudgetLimitUSD, token tổng. **Không** gồm Outline/Characters/RecentSummaries (nặng; phục vụ qua `/outline`).
- Package `internal/entry/worker`:
  - `protocol.go` — decode `DisallowUnknownFields`; `type` lạ → `error`, không panic.
  - `config.go` — dựng `bootstrap.Config` **từng field**: chỉ provider trong allowlist biên dịch sẵn (`providers.go`: openrouter, anthropic, gemini, openai, deepseek, qwen, glm, grok — `internal/bootstrap/setup.go:52-64`); `BaseURL=""`, `Type=""`, `Extra=nil`, `ExtraBody=nil`, `Roles=nil`, `Notify.Enabled=false` (mặc định đang bật và chạy `sh -c`, `internal/bootstrap/config.go:250-257`, `internal/notify/notify.go:120-163`), `DisableUpdateCheck=true`, `Budget{BookUSD, HardStop:true}`, `OutputDir=book_dir`; rồi `FillDefaults` + `ValidateBase`.
  - `pricing` — trước `host.New`: `models.DefaultRegistry().MergeModels([]ModelEntry{init.pricing})` để cost không bao giờ là $0 (`internal/host/usage.go:710-724`); kiểm `Resolve(model)` có giá > 0, không có → `error{code:"model_unpriced"}`. (Kiểm lại tên kiểu/hàm thật trong `internal/models` ở bước 1.)
  - `run.go` — `Run(ctx, in, out) error`; interface hẹp `engine` (các method Host dùng tới) để test bằng fake. Một writer duy nhất có mutex.
  - `mapping.go` — host → api DTO; golden JSON test.
- `cmd/ainovel-cli/main.go`: `os.Args[1] == "worker"` → `worker.Run(signal.NotifyContext(SIGINT, SIGTERM), os.Stdin, os.Stdout)`; bỏ qua first-run setup và `LoadConfig` (`main.go:32-34` đã có tiền lệ subcommand `eval`).

## Related Code Files

- Create: `internal/entry/worker/{protocol.go,config.go,providers.go,run.go,engine.go,mapping.go,stop_reason.go,oom_linux.go,oom_other.go}` + `*_test.go`; `internal/server/api/{worker.go,book.go,event.go}`
- Modify: `cmd/ainovel-cli/main.go`
- Read only: `internal/entry/headless/run.go`, `internal/host/{host.go,events.go,resume.go,budget.go,advance_gate.go}`, `internal/entry/tui/{events.go,model_update.go,commands.go,panels_sidebar.go}`, `internal/bootstrap/config.go`, `internal/models/{registry.go,pricing.go}`, `internal/store/run_meta.go`

## Implementation Steps

1. Xác nhận chữ ký thật: `ModelEntry`, `MergeModels`, `Resolve`; `RunMeta` load để đọc `PendingSteer`; cách lấy `resumeLabel` từ ngoài package `host` (nếu không export được → dùng `Snapshot().Phase`).
2. `internal/server/api` DTO + `mapping.go` + golden test.
3. `protocol.go` + test (JSON hỏng, field lạ, type lạ).
4. `config.go` + `providers.go` + test: provider ngoài allowlist bị từ chối; `base_url`/`notify`/`roles` trong init bị từ chối (DisallowUnknownFields); `Notify` tắt; HOME tạm rỗng không bị đọc.
5. `stop_reason.go` + test bảng (mọi tổ hợp snapshot ở trên).
6. `run.go` với fake engine, test (`-race`):
   - `Steer` treo 30s mà `shutdown` vẫn xong < 1s.
   - `abort` trong lúc `StartPrepared` đang chạy → engine dừng ngay khi `StartPrepared` trả về.
   - `Done` → snapshot cuối + `exit` + `Close`.
   - stdin EOF → `Close`.
   - action `steer` đi đúng nhánh Resume+Steer / Continue.
   - `SetAdvanceMode` được gọi trước mọi action.
7. Nối `main.go`.
8. Smoke tay với key thật, model rẻ, `budget_usd` nhỏ: start → vài event → `abort` → `exit{stop_reason:"user"}`; chạy lại `resume`; bật `review` → dừng sau chương 1 với `review_wait`; `advance` → viết chương 2. Kiểm lock được nhả sau mỗi lần thoát.
9. `gofmt -l`, `go vet ./...`, `go test ./internal/entry/worker/... ./internal/server/api/... -race`, `go test ./...`.

## Success Criteria

- [ ] Mọi `stop_reason` trong bảng đều có test; smoke tay đi qua `user`, `review_wait`, `advance`, `resume`.
- [ ] `shutdown` luôn hoàn tất < 1s khi chỉ có lời gọi LLM đang treo (fake), và < 15s với engine thật.
- [ ] Key không có trong stdout/stderr/`worker.log` (test).
- [ ] `ainovel-cli --headless` và TUI không đổi (test cũ xanh).

## Risk Assessment

- `resumeLabel` không export → dùng `Snapshot()` sau `New` để phân nhánh; test nhánh hoàn thành.
- `pricing` từ server lệch giá thật → chỉ ảnh hưởng độ chính xác của ngân sách, không tắt nó; registry vẫn tự làm mới.
- Sync upstream đổi chữ ký `host` → interface `engine` + golden test bắt lỗi khi compile/test.
