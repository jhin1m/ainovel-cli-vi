---
phase: 12
title: "Docs + kiểm tra cuối"
status: completed
priority: P2
effort: "0.5d"
dependencies: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]
---

# Phase 12: Docs + kiểm tra cuối

## Overview
Cập nhật docs theo hành vi mới (đăng ký tự do, provider Z.ai, thuê bao, kiểm tra key, lỗi provider, chạy local bằng CLI), dọn chỗ còn sót và chạy toàn bộ verify + checklist thủ công cho cả đợt.

## Context
- `plan.md` (Success Criteria toàn đợt) và các file phase 1–11 (mục Success Criteria).
- Docs liên quan: `docs/web-server.md` (vận hành bản web), `docs/architecture.md` (~L464 nhắc `serve invite create`), `README.md` (mục "## Bản web" ~L308, "## Cài đặt" ~L199). Repo không có `docs/project-changelog.md`, nên không tạo mới.
- Research PA4 (`plans/reports/research-261013-1645-local-execution-report.md`): web là bản hosted, CLI/TUI là đường chạy local. Local runner là dự án riêng sau này.

## Requirements
- `docs/web-server.md`:
  - Bỏ mọi phần mã mời và `serve invite`. Sửa dòng `docs/web-server.md:211` ("chưa có xác minh email… dùng mã mời") thành mô tả rủi ro thật: không xác minh email, có thể bị chiếm email trước, có thể bị dò email qua 409 `email_taken`.
  - Mục "Giới hạn khi mở đăng ký": liệt kê đúng tên biến và giá trị mặc định đọc từ code (`internal/server/config.go`, `auth/handlers.go`, `keys/handlers.go`, `keys/check.go`):
    - signup 30/phút + 10/ngày theo IP
    - tạo key 30/ngày/user, tối đa 20 key
    - kiểm key 3/phút + 20/ngày/user, 60/ngày/IP, tối đa 4 đồng thời
    - `AINOVEL_MAX_JOBS`, `AINOVEL_MAX_JOBS_PER_USER`, `AINOVEL_MAX_BOOKS_PER_USER`, `AINOVEL_MAX_RUN_MINUTES` (chia lượt)
    - chặn tạo truyện khi đĩa < 1 GiB
    - dọn tài khoản rỗng sau 30 ngày
    - trần retry 10 lần/10 phút của `llmretry` (áp cho cả CLI)
    - Ghi rõ limiter nằm trong bộ nhớ, mất khi restart, đầy thì đóng an toàn.
  - **Bước deploy bắt buộc** (không còn là khuyến nghị, thay `docs/web-server.md:120`): rule rate limit của Cloudflare cho `/api/auth/*`, `/api/auth/signup` và `/api/keys*`.
  - Thêm key không cần mật khẩu. Kiểm tra key chỉ áp cho key đã lưu.
  <!-- Red Team: giới hạn, deploy bắt buộc, nâng cấp/rollback, provider_rejected theo phát hiện 1, 3, 9, 10 — 2026-10-13 -->
  - Provider: bảng name / nhãn / base URL / tính phí. Có `zai-coding` (thuê bao, không ngân sách USD, hiện token), `zai`, `glm` (bigmodel.cn). Nói rõ allowlist cố định chống SSRF.
  - Lỗi provider: `stop_reason`/code `provider_rejected`, ví dụ 1113 và cách xử lý (thêm key dưới đúng provider rồi tạo truyện mới; truyện cũ không đổi provider được). `POST /api/keys/test` chỉ nhận `key_id`, có các giới hạn nêu trên, response không chứa văn bản của provider.
  - Mục "Nâng cấp lên bản này":
    - Chạy `deploy/backup.sh` trước khi deploy.
    - Migration 0004 (drop `invite_codes`, có Down) và 0005 (CHECK `budget_usd >= 0`, Down dùng `NOT VALID`) tự chạy khi khởi động.
    - User đang mở tab cũ cần tải lại trang (request cũ có field lạ sẽ bị 400).
  - Mục "Rollback": chạy binary cũ trên DB mới sẽ hỏng signup (500) và truyện thuê bao. Phải chọn một trong hai: restore backup (mất dữ liệu sau nâng cấp), hoặc Down 0005 → 0004 bằng goose rồi mới chạy binary cũ.
  - Mục ngắn "Chạy trên máy bạn": web là bản hosted, CLI/TUI chạy local; link tới README.
- `docs/architecture.md`: sửa dòng bảng ~L464 (bỏ `serve invite create`, package `admin` đã xoá). Giữ ngôn ngữ hiện có của file.
- `README.md` mục "## Bản web": cập nhật đăng ký tự do + provider mới; thêm 2–3 dòng so sánh web và CLI/TUI local (khớp với khối `run-locally.tsx` ở phase 7).
- Dọn sót:
  - `grep -rnE "text-\[[0-9]+px\]|rounded-\[[0-9]+px\]" web/src`: đổi sang token những chỗ còn lại (chỉ thay class, không đổi layout).
  - `grep -rn -i "invite\|mã mời" internal/server web/src docs README.md --exclude-dir=node_modules` chỉ được còn: migration 0002/0004, `internal/server/db/db_test.go`, và mục "Nâng cấp lên bản này"/"Rollback" của `docs/web-server.md` (nhắc `invite_codes` bị drop). Ngoài các chỗ đó phải rỗng. <!-- Updated: Validation Session 1 - gỡ mâu thuẫn giữa grep rỗng và docs nâng cấp phải nhắc drop invite_codes -->
- Không sửa logic code ở phase này, trừ lỗi nhỏ phát hiện khi verify (ghi lại trong báo cáo).

## Related Code Files
- Modify: `docs/web-server.md`, `docs/architecture.md`, `README.md`
- Modify (chỉ class): các file `web/src/**` còn px lẻ
- Report: `plans/reports/` (báo cáo tổng kết đợt, đặt tên theo quy ước `{type}-{yymmdd-hhmm}-{slug}-report.md`)

## Implementation Steps
1. Đọc 3 file docs + `internal/server/config.go`.
2. Cập nhật docs. Kiểm mọi lệnh/biến môi trường được nhắc tới có thật trong code.
3. Dọn px lẻ + grep invite.
4. Chạy toàn bộ verify.
5. Checklist thủ công (nếu dựng được backend + Postgres local; nếu không thì ghi rõ mục nào chưa kiểm):
   - Đăng ký mới → checklist 3 bước → thêm key ngay trong trang tạo truyện → tạo truyện → thấy stepper.
   - Key Z.ai Coding Plan (nếu user cung cấp): lưu dưới `zai-coding` → tự kiểm OK; tạo truyện không có ô ngân sách, chạy được chương 1, hiện token, chi phí USD = 0. Cùng key lưu dưới `zai` → tự kiểm báo `plan_mismatch` / 1113; truyện (nếu tạo) dừng với `stop_reason = provider_rejected`, log chỉ một lần gọi model.
   - Giới hạn: key thứ 21 → 409; kiểm key lần thứ 4 trong một phút → 429; response kiểm key không chứa văn bản của provider.
   - Xoá truyện; xoá key đang dùng → banner đổi key; hết ngân sách → "Lưu và viết tiếp".
   - Reader: theme theo OS, ←/→/Esc, "Đọc tiếp chương N".
   - Responsive 375 / 768 / 1440 / 1920, light/dark; có truyện cần xử lý → tiêu đề có "(n)", kể cả khi tab ẩn.
6. Viết báo cáo tổng kết + cập nhật status plan (`ak plan check` từng phase đã xong).

## Tests / Verify
```bash
go build ./... && go vet ./internal/...
go test ./internal/server/... ./internal/entry/worker/... ./internal/host/... ./internal/llmretry/... ./internal/bootstrap/...
# types phải khớp Go. So checksum trước/sau make gen, vì git diff sẽ báo khác giả nếu các phase trước chưa commit
before=$(shasum web/src/api/types.gen.ts) && make gen && [ "$before" = "$(shasum web/src/api/types.gen.ts)" ] && echo types-ok
pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build
```

## Success Criteria
- [x] Docs khớp hành vi thật. Không còn nhắc mã mời/`serve invite`, trừ mục nâng cấp/rollback (nói `invite_codes` bị drop). Có mục giới hạn, bước Cloudflare bắt buộc, nâng cấp và rollback.
- [x] `types.gen.ts` khớp Go: lệnh checksum ở trên in `types-ok`. Bước "Generated types are up to date" của CI (`.github/workflows/web.yml`) dùng `git diff --exit-code` trên checkout sạch, nên sẽ pass theo.
- [x] Toàn bộ verify pass. Checklist thủ công có kết quả từng mục (đã kiểm / chưa kiểm + lý do).
- [x] Plan cập nhật status.

## Risk Assessment
- Không có key Z.ai thật thì không xác nhận được giả thuyết 1113 ở môi trường thật. Ghi rõ trong báo cáo, đề nghị user tự kiểm.
- Docs `architecture.md` viết tiếng Trung ở nhiều đoạn. Sửa đúng dòng liên quan, giữ ngôn ngữ của đoạn đó.

## Bổ sung (orchestrator)
- Khối "Chạy trên máy bạn" (phase 7) đang dùng `git clone` SSH → đa số user lỗi. Dùng cách cài dễ nhất có trong README (binary release / script cài); nếu không có thì `git clone https://...`. Sửa README cho khớp.
- Kiểm bằng trình duyệt thật (backend chạy với DB test container ainovel-testpg, cổng 55439): chụp màn hình 375/768/1440px light+dark cho thư viện, tạo truyện, trang truyện, reader, cài đặt key; xác nhận không tràn ngang và lề không cộng dồn; kiểm viền focus bằng Tab.
- Mở 3 URL "Lấy key" của OpenAI, DeepSeek, x.ai bằng trình duyệt để xác nhận đúng trang.
- Trang tạo truyện khi mở "Thêm key khác": có 2 ô cùng nhãn "API key" (select key đã lưu + input của AddKeyForm). Đổi nhãn select thành "Key đã lưu" (hoặc tương đương) để screen reader phân biệt.
- Low từ phase 9: dải số liệu trang truyện nhận focus Tab ở ≥768px dù không cuộn → chỉ cho `tabIndex` khi thực sự cuộn ngang (<768px).
- Bug từ phase 11: `internal/server/books/repo.go` đếm cả truyện `deleting` khi kiểm `MaxBooksPerUser` → xoá xong tạo mới ngay vẫn bị báo hết giới hạn. Loại trạng thái `deleting` khỏi phép đếm, có test flow.
- Phase này nhiều việc: theo dõi context, quá ~35% thì ghi handoff như quy tắc chung.

## Kết quả (2026-10-13)
- Docs: `docs/web-server.md` (giới hạn, Cloudflare bắt buộc theo gói Free/Pro, provider, kiểm key + `provider_rejected`, nâng cấp, rollback bằng goose qua `GOOSE_DBSTRING`, chạy trên máy bạn), `docs/architecture.md` (bỏ `serve invite create`, package `admin`), `README.md` (bản web + so sánh CLI/TUI; clone HTTPS).
- Lệch phase ↔ code (docs theo code): trần retry `llmretry` chỉ áp cho worker, không áp cho CLI/TUI; Down 0005 đổi budget 0 → 0.01 (không `NOT VALID`); `provider_mismatch` là 400.
- Sửa nhỏ khi verify: `books/repo.go` `Count` bỏ truyện `deleting` (+ test flow, đã thấy đỏ trước fix); nhãn select "Key đã lưu"; dải số liệu chỉ có `tabIndex` khi tràn ngang; khối "Chạy trên máy bạn" clone HTTPS + gợi ý PATH; px lẻ → token; reader theo theme OS khi đang đọc; lề `PageHeader`; client huỷ request không còn log ERROR.
- Verify: Go build/vet/test (DB ainovel-testpg) pass 16 package; `types-ok`; web typecheck/lint/test (29 file, 254 test)/build pass.
- Checklist thủ công: xem `reports/browser-check-p12-report.md`. Chưa kiểm: key Z.ai thật (không có key), link DeepSeek (bị trang xác minh chặn).
- Báo cáo: `reports/cook-cook-p12-report.md`, review: `reports/from-code-reviewer-to-cook-p12-review-report.md`.
