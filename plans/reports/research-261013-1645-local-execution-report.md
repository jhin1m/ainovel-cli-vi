# Nghiên cứu: chạy engine trên máy người dùng, web vẫn làm giao diện

Ngày: 2026-10-13 · Branch: feat/web-app · Loại: chỉ nghiên cứu, không sửa code
Brief: `plans/reports/briefs/brief-research-local-execution.md` · Liên quan: `brainstorm-261013-1634-web-app-ux-onboarding-zai-report.md`

## Tóm tắt
- **Khuyến nghị:** đợt UX hiện tại chỉ làm **PA4** (giữ CLI/TUI là đường chạy local, web nói rõ điều đó). Effort S.
- **Dự án riêng sau đợt UX:** **PA1 Local runner**. Đây là phương án duy nhất vừa giữ web làm giao diện vừa đẩy RAM/CPU/egress LLM về máy người dùng. Lý do khả thi: worker vốn là process JSON Lines qua stdin/stdout, chỉ cần đổi lớp vận chuyển. Effort L.
- **Không theo:** PA2 WASM (XL, đã build thử và thất bại, Z.ai preflight không có CORS, tab bị ngủ/đóng là truyện dừng). PA3 desktop (L–XL, cần backend local không dùng Postgres, ký số/notarize, auto-update 3 hệ điều hành) chỉ nên làm nếu sau này có nhu cầu "không cần server".

## Hiện trạng (điểm nối trong code)
| Thành phần | File | Ghi chú |
|---|---|---|
| Giao thức worker | `internal/server/api/worker.go`, `internal/entry/worker/protocol.go` | JSONL: `init/steer/set_advance_mode/abort/shutdown` ⇄ `hello/event/stream/snapshot/ack/error/exit`. Kiểm tra chặt, có version `WorkerProtocol`. |
| Entry worker | `internal/entry/worker/run.go:51` `Run(ctx, in io.Reader, out io.Writer, version)` | Đã tách khỏi stdin/stdout thật, nhận `io.Reader/Writer` bất kỳ, nên chạy được trên WebSocket mà không đổi engine. |
| Spawn | `internal/server/jobs/supervisor.go:199-330` | Gắn chặt `exec.Cmd` (pipe stdout/stderr, `signalGroup`, `cmd.Wait`). `proc` (`jobs/proc.go:42`) giữ `cmd *exec.Cmd` + `stdin io.WriteCloser`. |
| Init + key | `supervisor.go:407-445` `sendInit` | Server giải mã key (`keys.DecryptForBook`), lấy pricing, gửi qua stdin. |
| Đọc/xuất truyện | `internal/server/books/reader.go`, `export.go`, `handlers.go:121` | Đọc **file trên đĩa server** qua `store.NewStore(dir)`: `meta/progress.json`, `outline.json` (+ layered), `chapters/NN.md`, summaries (tiêu đề), book, characters. |
| Khoá truyện | `internal/host/book_lock.go` (gofrs/flock) | Một engine/một thư mục. |
| Allowlist provider | `internal/entry/worker/providers.go` | Chặn SSRF vì server là nơi gọi ra ngoài. |

Server hiện gánh: RAM worker (`MemoryMax=3G` cho cả service, `AINOVEL_MAX_JOBS=4`), CPU, đĩa truyện, egress LLM, và giữ key (rủi ro master key).

## PA1. Local runner (`ainovel-cli runner --pair <token>`)
**Luồng:** web → "Chạy trên máy tôi" → server tạo pairing token ngắn hạn → user chạy lệnh → runner đổi token lấy runner credential dài hạn và mở **WSS outbound** tới server → server giao job → runner gọi `worker.Run` in-process (hoặc spawn `ainovel-cli worker` như server đang làm) trên thư mục local → các dòng JSONL đi qua WSS → server dùng lại `proc.handle`, hub, SSE như cũ.

**Khả thi với codebase:** cao.
- Server: tách `proc` khỏi `exec.Cmd` thành interface vận chuyển (gửi dòng, nhận channel dòng, kết thúc/huỷ). Exec hiện tại thành một implementation, WSS là implementation thứ hai. `readHello`, `handle`, `outcome`, `finish` dùng lại nguyên.
- Scheduler (`supervisor.schedule`) cần biết truyện gắn runner nào: thêm cột `books.runner_id`, job chỉ được giao khi runner online. Runner offline thì trạng thái `waiting_runner`, không dùng `failed`.
- **Đồng bộ nội dung:** reader/export đọc đĩa server, nên runner phải đẩy về một tập con: `meta/progress.json`, `outline*.json`, `chapters/*.md`, file summary chứa tiêu đề, `book`, `characters`. Kích thước nhỏ (văn bản). Có thể thêm message mới `file_put {path, sha256, bytes}` sau mỗi commit chương, hoặc gửi lúc nhận event commit. Drafts, checkpoints, runtime để lại máy user.
- Key: runner đọc key từ cấu hình CLI có sẵn (`~/.ainovel/...`, wizard `bootstrap`) và tự dựng `WorkerInit` local. Server **không cần giữ key**, bỏ được `DecryptForBook` cho truyện chạy runner. Pricing/budget tính local (litellm registry đã có), nên cũng mở đường cho Z.ai Coding Plan/ollama/base URL tuỳ ý, vì SSRF không còn là vấn đề khi máy user tự gọi.
- Hạ tầng: Caddy `reverse_proxy` tự hỗ trợ WebSocket. Cloudflare hỗ trợ WS nhưng cắt kết nối im lặng ~100s, nên cần ping 20s như SSE.

**Công sức: L** (~2–3 tuần một người). Ước tính chi tiết:
| Hạng mục | Cỡ |
|---|---|
| Migration `runners` (id, user_id, tên, hash credential, last_seen, version) + pairing token | S |
| Endpoint pair/revoke + WSS `/api/runner/connect` + auth | M |
| Trừu tượng vận chuyển trong `jobs` + định tuyến job theo runner | M |
| Ingest file đồng bộ (giới hạn kích thước, whitelist path) | M |
| Subcommand `runner` (reconnect + backoff, nhiều truyện tuần tự, log, chạy nền) | M |
| UI: trang "Máy chạy" (ghép nối, trạng thái online, thu hồi), chọn nơi chạy khi tạo truyện, banner runner offline | M |
| Test: fake runner qua WS, ngắt mạng giữa chừng, lệch version | M |

**UX:** user phải cài binary và để máy/terminal mở khi truyện chạy. Đóng máy thì truyện dừng, mở lại thì runner tự nối và chạy tiếp (engine đã resume được nhờ checkpoint). Theo dõi/đọc trên điện thoại vẫn được vì nội dung đã đồng bộ lên server. Lệnh copy-paste một dòng là vừa sức người dùng trung cấp, còn với người mới thì vẫn khó (cần Homebrew/scoop/script cài đặt).

**Bảo mật:**
- Pairing token: ngẫu nhiên ≥128 bit, dùng 1 lần, hết hạn 10 phút, gắn user. Credential runner: lưu hash ở DB, file local quyền 0600, thu hồi được, mỗi máy một credential.
- Server phải coi **mọi dữ liệu từ runner là không tin cậy**: whitelist path (không `..`, không symlink), giới hạn kích thước file/dòng/tốc độ, quota đĩa mỗi user. Reader hiện render text (không thấy `dangerouslySetInnerHTML` trong `web/src`), nên XSS từ nội dung chương ít rủi ro, nhưng vẫn cần giữ nguyên tắc này.
- Hướng ngược lại: server chỉ gửi `init/steer/abort` có schema chặt (`protocol.go` đã kiểm tra), không gửi lệnh shell hay đường dẫn tuỳ ý. `book_dir` do runner tự quyết, không nhận từ server.
- Key không rời máy user, nên lộ DB/master key không làm lộ key của các truyện chạy runner.

**Server còn gánh:** auth, Postgres, relay event/stream (băng thông nhỏ so với egress LLM), SSE, lưu bản sao chương, xuất TXT/EPUB. Không còn RAM/CPU engine, egress LLM hay giữ key.

**Rủi ro:** hai nguồn sự thật (đĩa local vs bản đồng bộ). Nếu user sửa local bằng TUI thì bản web lệch, nên cần quy ước "server chỉ là bản sao đọc". Lệch phiên bản runner/server: đã có `hello` + version, cần thông báo "cập nhật runner". Runner trên Windows: worker hiện có `proc_other.go`, nhưng flock/đường dẫn chưa được kiểm thử trên Windows. Thêm một bề mặt tấn công mới (WSS endpoint).

## PA2. Chạy hoàn toàn trong trình duyệt (Go → WASM)
**Đã thử build (không sửa code, output ở scratchpad):**
- `GOOS=js GOARCH=wasm go build ./internal/entry/worker` **lỗi**:
  - `internal/bootstrap` import `bubbletea`/`lipgloss`/`termenv` (wizard setup), không biên dịch được cho js.
  - `agentcore/tools/bash_unix.go` dùng `syscall.SysProcAttr.Setpgid` (được kéo vào qua `internal/tools/edit_chapter.go`), cần patch upstream hoặc build tag.
  - `gofrs/flock` (`internal/host/book_lock.go`) cần thay bằng khoá no-op/Web Locks.
- `GOOS=wasip1` lỗi tương tự, và wasip1 không có mạng nên không gọi được LLM.
- Kể cả khi biên dịch được: Go `js/wasm` trong trình duyệt **không có filesystem**. Gói `os` gọi `globalThis.fs`, nên cần polyfill fs dựng trên OPFS/IndexedDB (vd. ZenFS) cho toàn bộ `internal/store`. `net/http` trên js dùng Fetch nên stream được. Binary native hiện 28 MB, WASM sẽ cùng cỡ (vài MB sau nén) và tải lần đầu chậm trên di động. TinyGo không thực tế với cây phụ thuộc nặng reflection/encoding/json này.

**CORS thực đo (OPTIONS preflight, Origin lạ, 2026-10-13):**
| Provider | Endpoint | Kết quả |
|---|---|---|
| OpenAI | api.openai.com/v1 | Cho phép (echo origin) |
| Anthropic | api.anthropic.com/v1/messages | `*`. Request thật cần header `anthropic-dangerous-direct-browser-access: true` |
| Gemini | generativelanguage.googleapis.com (native + /openai) | Cho phép nếu chỉ xin header chuẩn. Xin header lạ thì 403 |
| DeepSeek | api.deepseek.com | Cho phép |
| OpenRouter | openrouter.ai/api/v1 | `*` |
| Qwen | dashscope(-intl).aliyuncs.com/compatible-mode | Cho phép |
| GLM | open.bigmodel.cn/api/paas/v4 | Cho phép |
| Grok | api.x.ai/v1 | `*` |
| **Z.ai** | api.z.ai/api/paas/v4 và **/api/coding/paas/v4** | **Preflight 200 nhưng không có `Access-Control-Allow-Origin`**, nên trình duyệt sẽ chặn request có `Authorization`. POST trả ACAO nhưng không qua được preflight. Chính là provider mà đợt UX muốn hỗ trợ. |

Ngoài ra, docs Z.ai ghi endpoint Coding Plan "chỉ dành cho công cụ được hỗ trợ / ngữ cảnh coding", nên dùng cho viết truyện từ bất kỳ đâu đã có rủi ro điều khoản (độc lập với phương án).

**Vòng đời tab:** truyện chạy hàng giờ. Đóng tab là dừng truyện. Chrome Memory Saver loại bỏ tab nền khi thiếu RAM mà không có event báo trước. Di động (nhất là iOS) treo JS nền gần như ngay lập tức. Chạy trong Web Worker giảm throttle timer nhưng không cứu được trường hợp tab bị discard/đóng. Dù sao checkpoint có sẵn giúp resume khi mở lại.

**Bảo mật:** key nằm trong trình duyệt (IndexedDB), nên mọi XSS/extension đều đọc được. Chấp nhận được với BYOK, nhưng kém hơn PA1.
**Công sức: XL** (refactor bootstrap, patch agentcore, polyfill fs/OPFS, worker-in-WebWorker, đồng bộ lên server nếu muốn đọc đa thiết bị, xử lý Z.ai qua proxy làm mất mục tiêu). **Không khuyến nghị.**

## PA3. App desktop (Wails/Tauri) bọc engine + UI web local
- Wails v2 là bản ổn định. v3 vẫn đang beta/prerelease (v3.0.0-beta.x). Tauri v2 cần Go làm sidecar.
- Vướng chính: backend `serve` cần **Postgres + master key + auth nhiều user**, desktop không thể bắt user cài Postgres. Phải viết backend local một-user: lưu books/jobs bằng file/SQLite, bỏ auth/keys mã hoá, giữ API REST+SSE cho SPA. Đây là phần lớn công sức, và cũng là chỗ dễ trùng lặp với `serve`.
- Phát hành: ký số/notarize macOS (Apple Developer 99 USD/năm), chứng chỉ ký Windows, auto-update, build 3 OS. Cộng thêm WebView khác nhau (WebKit/WebView2).
- UX tốt nhất cho người không biết terminal, nhưng **không có web đa thiết bị** trừ khi kèm PA1 để đồng bộ.
- Biến thể nhẹ hơn: `ainovel-cli ui` mở `http://127.0.0.1:<port>` với backend một-user và SPA nhúng sẵn (không wrapper). Công sức M–L, không có chi phí ký số, nhưng vẫn phải viết backend local.
- **Công sức: L–XL.** Chỉ đáng làm khi mục tiêu là "bỏ server hẳn".

## PA4. Giữ CLI/TUI là đường chạy local, web là bản hosted
- Đã có sẵn: TUI, headless, cùng định dạng thư mục truyện (`internal/store`), hỗ trợ mọi provider kể cả base URL tuỳ ý.
- Việc cần làm (S): trang/khối "Chạy trên máy của bạn" trong web (liên kết cài đặt, lệnh mẫu, so sánh hosted vs local), cập nhật README/`docs/web-server.md`. Không đụng backend.
- Hạn chế: không theo dõi được trên web/điện thoại. Server vẫn gánh toàn bộ truyện hosted, nên vẫn cần các giới hạn `MAX_JOBS`/`MAX_BOOKS_PER_USER` như brainstorm đã nêu.
- Có thể làm cầu nối sau: "import thư mục truyện local lên web để đọc" (upload zip). Hiện đang nằm trong "chưa đưa lên web: import truyện có sẵn".

## So sánh
| | PA1 Runner | PA2 WASM | PA3 Desktop | PA4 CLI |
|---|---|---|---|---|
| Web làm giao diện | Có (đa thiết bị) | Có (chỉ tab đang mở) | Không (UI local) | Không |
| Server bớt RAM/CPU/egress | Có | Có | Có (hết server) | Chỉ với truyện chạy CLI |
| Server bớt giữ key | Có | Có | Có | Có (với CLI) |
| Z.ai Coding Plan | Được | **Bị CORS chặn** | Được | Được |
| Chạy lâu, không giám sát | Tốt (máy bật) | Kém | Tốt | Tốt |
| Tái sử dụng code | Cao (giao thức worker) | Thấp | Trung bình (SPA) | Toàn bộ |
| Công sức | L | XL | L–XL | S |
| Bề mặt bảo mật mới | WSS + pairing | Key trong browser | Ký số/update | Không |

## Khuyến nghị
1. **Đợt hiện tại (UX/onboarding/Z.ai):** không đưa runner vào. Chỉ thêm PA4 (S): khối "Chạy trên máy bạn" + docs. Lý do: đợt UX đã rộng (đăng ký tự do, provider mới, layout, onboarding). Runner đụng scheduler, DB, giao thức và bề mặt bảo mật, cần plan + review riêng.
2. **Dự án riêng tiếp theo: PA1 Runner**, chia 3 phase:
   - (a) trừu tượng vận chuyển trong `jobs`, có test fake runner;
   - (b) pairing + WSS + định tuyến job + đồng bộ file tối thiểu;
   - (c) UI "Máy chạy" + đóng gói cài đặt.
   Mặc định truyện mới vẫn chạy hosted. Runner là tuỳ chọn.
3. Ghi nhận tạm cho Z.ai: nếu hosted bị điều khoản Coding Plan hoặc quota gây khó, runner là đường hợp lệ hơn để hỗ trợ.
4. Bỏ PA2. Xem lại PA3 chỉ khi có yêu cầu "không cần server".

## Nguồn
- Đo trực tiếp bằng `curl -X OPTIONS` tới từng endpoint (2026-10-13). Build thử `GOOS=js/wasip1` trên repo (không sửa code).
- [Anthropic CORS – Simon Willison](https://simonwillison.net/2024/Aug/23/anthropic-dangerous-direct-browser-access)
- [Z.ai API reference](https://docs.z.ai/api-reference), [Z.ai terms of use](https://docs.z.ai/legal-agreement/terms-of-use)
- [Wails releases](https://github.com/wailsapp/wails/releases), [Wails v3 beta](https://newreleases.io/project/github/wailsapp/wails/release/v3.0.0-beta.0)
- [Chrome tab discarding](https://developer.chrome.com/blog/tab-discarding/), [Memory/Energy Saver cho dev](https://developer.chrome.com/blog/memory-and-energy-saver-mode)

## Câu hỏi còn mở
- Mục tiêu chính là **giảm tải server** hay **không giữ key**? Nếu chỉ là không giữ key thì PA1 vẫn là lời giải, nhưng có thể chọn phạm vi MVP nhỏ hơn (chưa cần đồng bộ đa thiết bị).
- Với truyện chạy runner, chấp nhận server lưu bản sao chương (để đọc trên điện thoại/xuất file) không, hay muốn nội dung chỉ ở máy user (khi đó reader phải proxy qua runner online)?
- Một runner chạy nhiều truyện song song hay tuần tự? Giới hạn theo máy user hay để user tự chỉnh?
- Kênh phân phối runner: chỉ GitHub Releases + script cài, hay cần Homebrew/scoop/winget? Có hỗ trợ Windows ngay từ đầu không (chưa kiểm thử flock/đường dẫn trên Windows)?
- Z.ai: CORS preflight mới đo từ curl, chưa thử trên trình duyệt thật. Điều khoản dùng endpoint Coding Plan cho viết truyện cần xác nhận lại với docs "supported tools".
- Mức effort L của PA1 là ước tính từ đọc code, chưa prototype. Nên spike 1–2 ngày cho phase (a) trước khi chốt plan.
