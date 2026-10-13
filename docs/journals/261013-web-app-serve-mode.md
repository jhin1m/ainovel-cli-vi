# Xây chế độ web `ainovel-cli serve` (nhánh feat/web-app)

**Date**: 2026-10-13
**Severity**: Medium
**Component**: internal/server, internal/entry/worker, web/, deploy/
**Status**: Ongoing (code xong 6 phase, chưa qua thử nghiệm thật)

## What Happened

Trong một ngày, nhánh `feat/web-app` (tách từ `vi`) đi từ 0 tới 15 commit: worker stdio, khung server + Postgres, auth + BYOK, supervisor + Books API + SSE, React SPA nhúng `embed.FS`, PWA + systemd + Caddy + docs. Plan ghi cả 6 phase "Completed". Con số đó nói quá: mọi e2e đều chạy bằng fake worker, chưa có lần nào gọi LLM thật.

## The Brutal Truth

Gate xanh tạo cảm giác an toàn giả. Reviewer vẫn tìm ra High và Medium ở gần như mọi phase, và vài lỗi chỉ lộ vì một người đọc kỹ state machine chứ không phải do test. Thành thật mà nói, chưa ai biết worker thật chạy ổn dưới supervisor này hay không.

## Quyết định kiến trúc

- **Mỗi lượt chạy là một process `ainovel-cli worker`**, nói chuyện qua NDJSON stdin/stdout, thoát mỗi khi engine dừng. Không dùng nhiều `host.Host` trong một process vì `lang.Set`, `slog.SetDefault`, `~/.ainovel` và registry pricing là global, kênh event chỉ có một consumer, một panic sẽ giết cả server.
- **Supervisor** giữ hàng đợi, giới hạn toàn server/mỗi user, tự resume sau restart.
- **SSE** cho realtime; server đọc store trực tiếp để phục vụ chương và xuất TXT/EPUB.
- **BYOK mã hoá** trong DB (cột `key_version` để sau này xoay master key). Key chỉ đi qua message `init` trên stdin.
- **Không sửa `internal/host`**: dễ sync upstream, đổi lại worker phải dùng interface hẹp.

## Bug thật do review/test bắt được

- **PATCH /api/books/{id} luôn 500**: `$3` vừa là `budget_usd` (float8) vừa nằm trong `$3 > $6` -> `inconsistent types deduced for parameter $3 (42P08)`. Sửa bằng `$3::float8`. `TestBookBudget` bắt được.
- **Xoá truyện treo mãi**: scheduler giữ chỗ, claim lỗi (`status` không còn `queued`), nhánh lỗi không gọi `markDone` nên `StopAndWait` chờ vô hạn, thư mục không xoá, goroutine rò. Test `TestStopAndWaitDuringFailedClaim` đã xác nhận FAIL khi bỏ `markDone`.
- **Pending mất khi lượt lỗi**: truyện mới lỗi lượt đầu mất `pending_action=start`; resume sau đó chạy `Resume()` trên thư mục rỗng -> `nothing_to_resume` -> truyện 0 chương ở trạng thái `completed`. Giờ chỉ lượt sạch mới `ClearPending`.
- **Drain làm `/next` chạy lặp**: worker tự dừng trong lúc drain bị bỏ qua, `pending_action=advance` còn sót, restart xong viết thêm chương N+1 chưa được duyệt (tốn tiền user). Giờ chỉ bỏ qua lượt do chính server cắt.
- **Start chạy lại tạo lại truyện**: `start` trên truyện đã có progress/complete đụng `refuseNewBookOverExisting` rồi `failed` khó hiểu. Sửa ở worker: có progress thì báo `nothing_to_resume`.
- **SSE reconnect vô hạn**: truyện đã xoá (404) hoặc hết phiên (401) mà EventSource vẫn thử lại. Giờ `fetchQuery` lại book, 401 về login, 404 dừng và hiện banner.
- **TLS sau Cloudflare**: proxy cam chặn TLS-ALPN-01, firewall chỉ mở 443 nên HTTP-01 cũng chết; Caddy không lấy được cert, Full (strict) trả 525/526. Docs nhắc Origin Certificate nhưng Caddyfile không có `tls <cert> <key>`.
- **OOMPolicy**: systemd mặc định `OOMPolicy=stop`; một worker bị OOM kill (do `oom_score_adj=500` chủ đích) sẽ làm dừng cả service. Cần `OOMPolicy=continue`. Docs ghi ngược lại.
- Vặt nhưng đáng kể: steer trả 202 khi worker chưa `ready` (nay 503 `worker_starting`), cache export không vô hiệu khi chương đổi (nay khoá theo chữ ký size+mtime), `last_error` lộ chi tiết nội bộ (nay chung chung, chi tiết vào log).

## Bài học

- **Fake worker khác worker thật.** Fake cho `resume` trên thư mục rỗng chạy bình thường, còn worker thật trả `nothing_to_resume`. Chính độ lệch này che mất lỗi "completed rỗng". Mỗi khi sửa hành vi worker, phải so lại fake và ưu tiên smoke thật.
- **Commit gom nhầm file đang staged.** Có commit kéo theo file staged không thuộc phạm vi. Trước khi commit phải `git status`/`git diff --cached` và add từng đường dẫn, đừng tin index còn sạch.
- Test chạy trên mô hình giả chỉ chứng minh mô hình giả đúng. Review đọc state machine có giá trị hơn một lượt test xanh thêm.
- Tài liệu deploy phải kiểm bằng công cụ thật (`caddy adapt`, `systemd-analyze`), không chỉ đọc lại.

## Next Steps (làm tay, agent không làm được)

1. Smoke với API key LLM thật: worker (phase 1 bước 8) và supervisor (phase 4 bước 7). Chặn mọi thứ còn lại.
2. Deploy staging lên VPS, kiểm SSE qua Cloudflare; chốt chế độ TLS (khuyến nghị Origin Certificate) và thêm `OOMPolicy=continue`.
3. Cài PWA trên Android và iOS thật.
4. Cho 3 người dùng thử, đo INP và kiểm 360px/dark trên trình duyệt thật (jsdom không có layout).
5. Quyết định đã chốt theo hướng an toàn: PATCH budget lúc truyện đang chờ/chạy trả 409 `book_running` (worker chỉ đọc ngân sách lúc khởi động); `resume_failures` cộng cả lỗi worker (theo spec), thông báo dừng tự khôi phục đã viết trung tính.
6. `snapshot.current_chapter` lệch tổng sau khi hoàn thành (4 so với 3), web đã clamp nhưng consumer khác vẫn nhận sai.

Status: DONE
Summary: Journal đã ghi lại hướng đi, các bug bắt được và việc còn lại; chưa publish, chưa commit.
