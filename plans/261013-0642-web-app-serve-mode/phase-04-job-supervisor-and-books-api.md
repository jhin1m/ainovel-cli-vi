---
phase: 4
title: "Supervisor, Books API, SSE"
status: pending
priority: P1
effort: "3.5d"
dependencies: [1, 3]
---

# Phase 4: Supervisor, Books API, SSE

## Overview

Quản lý vòng đời process `worker` cho từng truyện (hàng đợi, giới hạn, dừng, khôi phục sau restart), phát realtime qua SSE, đọc chương và xuất file trực tiếp từ store. Worker **thoát mỗi khi engine dừng**; truyện đang chờ duyệt, đã dừng hay đã xong đều không giữ process hay slot.

## Requirements

### API

Mọi route đều yêu cầu đăng nhập. Mọi truy vấn lọc theo `user_id`; truyện của người khác trả 404.

| Method | Path | Việc |
|---|---|---|
| GET | `/api/books` | Danh sách (đọc title, số chương, số chữ, chi phí từ `snapshot` jsonb) |
| POST | `/api/books` | `{prompt, api_key_id, model, budget_usd, advance_mode}` → tạo + `pending_action=start` + xếp hàng. `model` phải nằm trong `/api/providers` của provider ứng với key. |
| GET | `/api/books/{id}` | Chi tiết + snapshot |
| PATCH | `/api/books/{id}` | `{budget_usd?, advance_mode?}`; `budget_usd` ≤ `AINOVEL_MAX_BOOK_BUDGET_USD` và > chi phí đã tiêu; đang chạy → gửi `set_advance_mode`; truyện `budget_exhausted` được nâng ngân sách thì chuyển sang `paused` |
| DELETE | `/api/books/{id}` | `status=deleting` → dừng worker → chờ `Wait()` xong → xoá thư mục → xoá row |
| POST | `/api/books/{id}/pause` | Gửi `abort` (worker tự thoát với `stop_reason=user`) |
| POST | `/api/books/{id}/resume` | `pending_action=resume` + xếp hàng |
| POST | `/api/books/{id}/steer` | `{text}`; đang chạy → lệnh `steer`; đã dừng → `pending_action=steer, pending_text=text` + xếp hàng |
| POST | `/api/books/{id}/next` | Chỉ khi `waiting_review`: `pending_action=advance` + xếp hàng |
| GET | `/api/books/{id}/events` | SSE |
| GET | `/api/books/{id}/chapters` | Chương đã chốt (`Progress.CompletedChapters` + tiêu đề từ outline) |
| GET | `/api/books/{id}/chapters/{n}` | Văn bản chương (`DraftStore.LoadChapterText`, `internal/store/drafts.go:89`) |
| GET | `/api/books/{id}/outline` | Dàn ý + nhân vật (đọc store) |
| GET | `/api/books/{id}/export?format=txt\|epub` | Tải file |

Lệnh gửi cho worker đang chạy thì chờ `ack` tối đa 5s; quá hạn trả `202` (kết quả sẽ đến qua SSE).

### Trạng thái

`books.status`: `queued`, `running`, `waiting_review`, `paused`, `budget_exhausted`, `completed`, `failed`, `interrupted`, `deleting`.

Chuyển từ `exit.stop_reason` của worker:

| stop_reason | status mới |
|---|---|
| `review_wait` | `waiting_review` |
| `budget` | `budget_exhausted` |
| `complete`, `nothing_to_resume` | `completed` |
| `user`, `hold` | `paused` |
| `error` | `failed` (tăng `resume_failures`) |
| `lock_busy` (lỗi init) | giữ `queued`, thử lại sau 5s/15s/45s, không tính vào `resume_failures` |

Process chết không có `exit` → `failed` + 200 dòng stderr cuối vào `last_error`. Riêng khi server đang `draining` thì không đổi status.

### Giới hạn

- Chỉ process đang sống mới tính vào `AINOVEL_MAX_JOBS` và `AINOVEL_MAX_JOBS_PER_USER`.
- Thêm `AINOVEL_MAX_BOOKS_PER_USER`.
- Prompt ≤ 4000 ký tự, steer ≤ 2000 ký tự.
- `budget_usd` > 0 và ≤ trần.

### Khôi phục sau restart

- Khi server khởi động:
  - truyện ở `interrupted` → `pending_action=resume` → xếp hàng;
  - truyện ở `queued` → xếp hàng lại, giữ nguyên `pending_action`/`pending_text`;
  - truyện ở `running`/`waiting_review` mà không có process (crash) → coi như `interrupted`.
- `resume_failures` ≥ 3 → `failed`.

### Tắt server có kiểm soát

1. Trong một transaction: `UPDATE books SET status='interrupted' WHERE status='running'`.
2. Bật cờ `draining`: các lần worker thoát sau đó không ghi status.
3. Gửi `shutdown` song song cho mọi worker; sau 10s gửi SIGTERM cho nhóm; sau 10s nữa gửi SIGKILL cho nhóm.
4. Chờ mọi `Wait()` xong.

### Non-functional

- **Spawn worker:**
  - Lệnh: `exec.Command("/proc/self/exe", "worker")` trên Linux (`os.Executable()` ở nơi khác). Chạy đúng inode đang chạy kể cả khi binary bị thay giữa chừng.
  - `Dir` = thư mục truyện.
  - Env chỉ gồm `HOME=<data>/users/<uid>/home`, `PATH`, `LANG=C.UTF-8`, `TZ`. Không truyền gì khác.
  - `Setpgid`; Linux thêm `Pdeathsig: SIGKILL` (file `_linux.go`).
- **Bắt tay:** dòng đầu phải là `hello` với `protocol` và `version` khớp server. Không khớp → kill, giữ `interrupted`, ghi log.
- **Gửi init:** key giải mã qua `keys.DecryptForBook(bookID)` (FK kép). Ghi xong init thì xoá biến key. `pricing` lấy từ registry của server.
- **Đọc output:** stdout dùng `bufio.Scanner` buffer 4MB. stderr ghi vào `<book>/logs/worker-stderr.log` (cắt ở 5MB) và giữ ring 200 dòng.
- **Mỗi truyện một state machine trong supervisor** (`idle → starting → running → stopping → idle`). Spawn kế tiếp, xoá, hay `RemoveAll` chỉ chạy khi `Wait()` của process trước đã trả về (flock chỉ nhả cuối `Close()`, `internal/host/host.go:935-969`).
- **Đường dẫn** luôn suy từ ID trong DB, không bao giờ lấy từ input.

## Architecture

```
internal/server/books/
  repo.go         // CRUD, chuyển trạng thái có điều kiện (UPDATE ... WHERE status IN (...) RETURNING)
  handlers.go     // REST + kiểm quyền
  reader.go       // store.NewStore(dir): chương, outline
  export.go       // exp.Run với OutPath = <book>/exports/<CompletedCount>.<ext>, Overwrite=true; có cache thì dùng lại
internal/server/jobs/
  supervisor.go   // scheduler (đánh thức bằng channel), semaphore global/per-user, map[bookID]*bookState, recovery, drain
  proc.go         // spawn, hello, init, đọc stdout, stop sequence, map exit → status
  proc_linux.go / proc_other.go
  hub.go          // per-book: ring 300 event upsert theo event.id + snapshot mới nhất + subscribers
  sse.go
```

Migration `0003_books.sql`:

```sql
books(id uuid pk default gen_random_uuid(), user_id uuid not null, api_key_id uuid,
      prompt text, status text, provider text, model text, budget_usd numeric,
      advance_mode text,                       -- ý định của user; worker áp vào RunMeta mỗi lần spawn
      pending_action text, pending_text text,  -- start|resume|steer|advance, sống sót qua restart
      snapshot jsonb,                          -- api.BookSnapshot mới nhất; nguồn duy nhất cho title/số chương/chi phí
      last_error text, resume_failures int default 0,
      created_at, updated_at, started_at, finished_at,
      foreign key (user_id) references users(id) on delete cascade,
      foreign key (user_id, api_key_id) references api_keys(user_id, id))
-- index (user_id, updated_at desc), (status)
```

SSE:

- Header: `text/event-stream`, `Cache-Control: no-cache, no-transform`, `X-Accel-Buffering: no`. Không gzip.
- Gửi `retry: 3000` trước tiên. Gửi `: ping` mỗi 20s.
- Trước **mỗi** lần ghi đặt write deadline 10s bằng `http.NewResponseController`; ghi lỗi thì đóng kết nối. Không xoá deadline vô thời hạn.
- Khi kết nối: subscribe → gửi `snapshot` → gửi toàn bộ ring (`event`) → live.
- Client thay timeline mỗi lần (re)connect. Không dùng `Last-Event-ID`/seq/`reset`.
- Kiểu message:
  - `snapshot`
  - `event` (api.Event; start/finish cùng `id` → upsert)
  - `stream` (chỉ live, không vào ring)
  - `status` (đổi `books.status`, kèm `stop_reason`)
- Subscriber dùng channel buffer 64. Đầy → đóng kết nối đó, trình duyệt tự kết nối lại.
- Giới hạn `AINOVEL_MAX_SSE_PER_USER` / `AINOVEL_MAX_SSE_TOTAL`; vượt thì 429.
- Timeline mất khi server restart: chấp nhận cho MVP. Muốn giữ lại thì sau này worker ghi `logs/events.jsonl`.
- Snapshot ghi DB tối đa 1 lần/10s, và ghi ngay khi `Phase`/`CompletedCount`/`RuntimeState` đổi hoặc khi worker thoát.

Export:

- Semaphore toàn server `AINOVEL_MAX_EXPORTS` và 1 export/user cùng lúc; quá thì 429.
- File cache theo `CompletedCount`.
- `exp.Run` **có ghi file**; mặc định ghi vào thư mục truyện với tên lấy từ title và lỗi nếu file đã tồn tại (`internal/host/exp/exporter.go:107-118`). Vì vậy luôn đặt `OutPath` và `Overwrite`.

## Related Code Files

- Create: các file trên + test; `internal/server/db/migrations/0003_books.sql`
- Modify: `internal/server/server.go` (route, khởi động supervisor, hook shutdown hai bước), `internal/server/config.go` (nếu còn thiếu biến)
- Read only: `internal/store/{drafts.go,runtime.go}`, progress/outline loader trong `internal/store`, `internal/host/exp/{exporter.go,types.go}`

## Implementation Steps

1. `hub.go` + test (`-race`): upsert theo id; nhiều subscriber; subscriber chậm bị đóng mà publisher không block; connect giữa chừng nhận snapshot + ring.
2. `proc.go` với fake worker theo pattern `TestHelperProcess`. Các ca cần test:
   - hello lệch version → kill;
   - mỗi `stop_reason` → status đúng;
   - worker phớt lờ `shutdown` → SIGTERM → SIGKILL;
   - crash không có `exit` → `failed` + stderr;
   - `lock_busy` → thử lại.
3. `supervisor.go` + test:
   - giới hạn global/per-user; truyện `waiting_review` không chiếm slot;
   - không spawn chồng khi `Wait()` chưa xong;
   - recovery: `interrupted`/`queued`/crash, giữ `pending_text`;
   - drain: SIGTERM server khi có 2 fake worker → khởi động lại → cả hai được resume.
4. `repo.go` + test chuyển trạng thái sai bị từ chối.
5. `reader.go`, `export.go` + test với store fixture tạo bằng chính package `store`; export lần hai dùng cache, không lỗi.
6. `handlers.go` + `sse.go`. Test quyền:
   - user B gọi mọi route của truyện A → 404;
   - user B tạo truyện với `api_key_id` của A → 404/422 (FK kép).

   Test SSE: client không đọc → bị cắt sau ≤ 10s; vượt giới hạn kết nối → 429.
7. Smoke với worker thật + key thật:
   - tạo truyện ngắn ở chế độ review; `curl -N` theo dõi SSE;
   - `waiting_review` → `/next`; steer khi đang chạy và khi đã dừng; pause/resume;
   - `PATCH` hạ ngân sách sát chi phí → `budget_exhausted` → nâng lên → resume;
   - `systemctl restart`/SIGTERM giữa chừng → tự resume;
   - export EPUB mở được.
8. `go test ./internal/server/... -race`, `go vet ./...`.

## Success Criteria

- [ ] Hai user chạy song song, giới hạn đúng; job vượt giới hạn nằm `queued` rồi tự chạy khi có slot; truyện chờ duyệt không giữ slot.
- [ ] "Chương tiếp" chạy được: `waiting_review` → `/next` → viết đúng một chương → `waiting_review`.
- [ ] Hết ngân sách → `budget_exhausted`; nâng ngân sách là chạy tiếp; restart không biến nó thành `failed`.
- [ ] Restart/deploy → truyện đang chạy và đang xếp hàng đều tiếp tục, steer đang chờ không bị mất.
- [ ] Env của worker chỉ có 4 biến ở trên; key và secret không có trong env của server lẫn worker.
- [ ] User B không đọc, điều khiển hay dùng key được gì của A (test).
- [ ] Export TXT/EPUB đúng nội dung; 10 request export song song không làm RAM server vượt mức (bị 429).

## Risk Assessment

- **Đọc store khi worker đang ghi:** mỗi file ghi nguyên tử (`internal/store/io.go:36-62`) → an toàn. Đọc lỗi thì trả 409.
- **Ổ đĩa đầy:** `MAX_BOOKS_PER_USER` + cảnh báo dung lượng ở `/healthz`.
- **Mỗi lần dừng/tiếp đều spawn process mới:** tốn ~1–2s khởi động, chấp nhận được (đổi lại không giữ slot).
- **Upgrade binary:** `/proc/self/exe` + kiểm `hello` + quy trình "stop → thay binary → start" ghi trong docs.
