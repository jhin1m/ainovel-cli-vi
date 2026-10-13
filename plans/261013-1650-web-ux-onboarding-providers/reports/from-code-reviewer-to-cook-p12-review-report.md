# Review phase 12: docs và dọn sót

Điểm: **8/10**. Không có lỗi chặn. Thay đổi code đúng và có test. Docs khớp code gần như toàn bộ, nhưng có 2 chỗ hướng dẫn vận hành không làm theo được như đang viết (M1, M2).

## Đã kiểm, khớp code
- `Count` (books/repo.go:94-98) chỉ có một caller là `handleCreate` (books/handlers.go:219). Truyện đang xoá vẫn giữ slot worker, vì `perUser` chỉ giảm khi process thoát (jobs/supervisor.go:282), nên không lách được `MAX_JOBS_PER_USER`. Đĩa có chặn 507 riêng (handlers.go:183).
- Bảng biến môi trường khớp với config.go:58-185. Các giới hạn sau khớp code:
  - signup 10/ngày (auth/handlers.go:31)
  - tạo key 30/ngày và tối đa 20 key, trả 409 `key_limit` (keys/handlers.go:24-25)
  - kiểm key 3/phút, 20/ngày theo user, 60/ngày theo IP, tối đa 4 đồng thời, trả 429 `busy` (keys/check.go:23-26, 94)
  - limiter giữ tối đa 100.000 khoá, khi đầy thì từ chối hoặc bỏ khoá cũ nhất (auth/ratelimit.go:57-87)
  - đĩa dưới 1 GiB thì chặn (server.go:318)
  - dọn tài khoản rỗng sau 30 ngày (server.go:300, auth/handlers.go:64-75)
- Route, mã lỗi và HTTP status được nhắc trong docs đều có thật. Field lạ bị trả 400 (httpx/json.go:88).
- Bảng provider khớp providers.go:19-28 và bootstrap/setup.go:53-60.
- Migration 0004 và 0005 được mô tả đúng. Down của 0005 không dùng `NOT VALID`: docs viết theo code, đúng. Plan ghi "Down dùng NOT VALID" là sai.
- Lệnh goose đúng cú pháp: `-dir … postgres <dsn> down`, phiên bản v3.27.0 khớp go.mod:13, bảng mặc định `goose_db_version` khớp `goose.NewProvider` ở db.go:55.
- Trần retry của llmretry: chỉ `worker.Run` gọi `SetLimits` (entry/worker/run.go:41, 61), không có caller nào khác. Docs ghi "chỉ worker, CLI/TUI retry không giới hạn" là đúng. Yêu cầu trong plan ("áp cho cả CLI") sai so với code.
- Grep `invite|mã mời`: chỉ còn ở 0002, 0004, db_test.go và mục Nâng cấp/Rollback. Grep px lẻ: rỗng.
- `go build ./...` và `go vet ./internal/server/...`: OK.
- Test flow books (13 test) PASS với DB 55439. `pnpm -C web typecheck` và `pnpm -C web lint`: sạch. `pnpm -C web test`: 29 file, 253 test PASS.
- Chưa chạy: `make gen` checksum và `pnpm build`. Phase này không đổi type API.

## Medium
- **M1. Bước Cloudflare "bắt buộc" không làm được trên gói Free/Pro.** Chỗ: docs/web-server.md:120-125 và :269. Docs đòi 3 rule. Gói Free chỉ cho 1 rule, chu kỳ đếm cố định 10 giây, chỉ lọc được theo Path. Gói Pro cho 2 rule, chu kỳ tối đa 1 phút. Vì vậy câu "vài chục request mỗi phút" không đặt được trên Free.
  - Sửa: Free gộp `/api/auth/` và `/api/keys` vào 1 rule (ngưỡng theo 10 giây). Pro tách thêm rule `/api/auth/signup`. Đủ 3 rule thì cần Business.
- **M2. "Tải lại trang" không đủ để nhận bản mới.** Chỗ: docs/web-server.md:274. Service worker đăng ký với `registerType: 'prompt'` (web/vite.config.ts:15), nên F5 vẫn trả SPA cũ đã precache. Form cũ vẫn gửi `invite_code`/`current_password` và nhận 400 cho tới khi user bấm "Cập nhật" trên banner "Có phiên bản mới" (lib/pwa-update.ts:66, messages.ts:15-18) hoặc đóng hết tab.
  - Sửa câu thành: "bấm Cập nhật trên thông báo Có phiên bản mới (hoặc đóng mọi tab rồi mở lại)".
- **M3. Lệnh "Chạy" trong thẻ chạy local sẽ lỗi.** Chỗ: web/src/features/onboarding/run-locally.tsx:13 và :82-84. Khối cài build ra `./ainovel-cli` ngay trong thư mục repo, rồi khối Chạy lại gọi `ainovel-cli`, nên shell báo command not found. README.md:209 có dòng hướng dẫn chép vào PATH, nhưng thẻ trên web không có. Đây là khối orchestrator yêu cầu làm cho "dễ nhất".
  - Sửa: thêm vào `runHint` câu "chép ainovel-cli vào PATH (vd. ~/.local/bin) hoặc chạy ./ainovel-cli". Test README vẫn pass.

## Low
- **L1.** docs/architecture.md:416: cây thư mục `server/` vẫn liệt kê `admin`, dù package đã bị xoá. Chỉ dòng bảng ~L464 được sửa.
- **L2.** docs/web-server.md:166 ghi "Đăng ký 30/phút theo IP". Thực tế bucket 30/phút này dùng chung cho đăng nhập và đăng ký (auth/handlers.go:51, 218, 278). IPv6 được gom theo /64. Token bucket cho tới khoảng 2n lượt trong cửa sổ đầu (ratelimit.go:36-40).
- **L3.** docs/web-server.md:170: dòng "Số truyện" thiếu mã `409 book_limit`, trong khi các dòng khác đều ghi mã. Nên thêm câu "truyện đang xoá không tính".
- **L4.** docs/web-server.md:210 ghi response kiểm key "chỉ có ok/kind/code/message". Thiếu field `model` (api/account.go:74).
- **L5.** docs/web-server.md:289-290: lệnh goose đặt DSN có mật khẩu trên argv, nên `ps` thấy được. Điều này trái với cách làm ở dòng :245. Nên dùng `GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(sudo cat …)" goose -dir … down`. Cũng nên nhắc `~/go/bin` có thể chưa nằm trong PATH.
- **L6.** docs/web-server.md:179: câu "Lưu ý lệch với kỳ vọng ban đầu" là lịch sử của plan, lọt vào docs vận hành. Chỉ cần nói thẳng hành vi.
- **L7. Đổi px sang token chưa đồng nhất.**
  - book-screen.tsx:50: chữ hint 13px bị đổi thành `text-xs` (12px), vi phạm quy tắc "chữ thân ≥ 14px" mà chính app.css vừa thêm. Trong khi đó book-tabs.tsx đổi 13px thành `text-sm`.
  - book-screen.tsx:118: `rounded-[9px]` bị đổi thành `rounded-md` (6px, mặc định Tailwind), không phải token của dự án.
  - auth-layout.tsx: h1 đổi từ 32px thành 28px ở mobile và 36px ở desktop, tức là có đổi kích thước dù yêu cầu là "chỉ đổi class". Chấp nhận được.
- **L8. Truyện kẹt ở `deleting` không còn bị giới hạn.** Chỗ: books/repo.go:96. Khi `finishDelete` lỗi (RemoveAll hoặc hardDelete, handlers.go:345-353), dòng truyện kẹt ở `deleting` tới lần restart sau. Trước đây các dòng này vẫn được đếm, nên số truyện kẹt tự bị chặn ở trần. Giờ chúng không được đếm và thư mục có thể còn trên đĩa. Chỉ xảy ra khi có lỗi IO, đã có log. Không cần sửa, chỉ cần biết.

## Ghi nhận thêm
- Count rồi Create không atomic, nên 2 request đồng thời có thể vượt `MAX_BOOKS_PER_USER`. Lỗi này có từ trước, không thuộc phase này.
- `useScrollsX` (book-stats-strip.tsx:89-110) đúng hướng. Nếu dải đang có focus mà mất tabIndex (resize lên ≥768px), focus sẽ rơi về `body`. Trường hợp hiếm, chấp nhận được.
- Nhãn "Key đã lưu" chỉ dùng ở config-panel.tsx:71. Select (`id="key"`) không trùng id với input của AddKeyForm (dùng `useId`).

## Đề xuất (theo thứ tự)
1. Sửa M1 và M2 trong docs/web-server.md.
2. Thêm gợi ý PATH vào `runHint` (M3).
3. Bỏ `admin` ở architecture.md:416 (L1). Gom các chỉnh chữ L2-L6 vào cùng một lượt sửa.

Status: DONE_WITH_CONCERNS
Summary: Phần code (Count bỏ truyện deleting, tabIndex theo tràn ngang, nhãn "Key đã lưu", clone HTTPS) đúng, có test, và build/lint/test đều xanh. Docs khớp code ở biến môi trường, giới hạn, provider, migration và goose. Còn 2 hướng dẫn vận hành không làm theo được: số rule Cloudflare vượt gói Free/Pro, và "tải lại trang" không đủ khi có service worker prompt. Thêm lệnh chạy trên web card sẽ báo command not found.
