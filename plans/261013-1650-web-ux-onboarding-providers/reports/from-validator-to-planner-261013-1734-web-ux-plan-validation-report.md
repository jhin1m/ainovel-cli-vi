# Báo cáo validate: đợt web UX + onboarding + providers

- Ngày: 2026-10-13
- Plan: `plans/261013-1650-web-ux-onboarding-providers/plan.md` (12 phase)
- Chế độ: `ak:plan validate`. Tier Full (12 phase), chạy sau red-team.
- Kết luận: **ĐÃ VALIDATE, cook được.** Còn 0 lỗi sai lệch với code và 0 mâu thuẫn.

## Đầu vào đã chốt (không hỏi lại)

- Mọi quyết định trong `plans/reports/briefs/brief-plan-web-ux-round.md`.
- Toàn bộ giới hạn red-team:
  - signup 10/ngày/IP
  - tạo key 30/ngày, tối đa 20 key
  - kiểm key 3/phút + 20/ngày/user, chỉ key đã lưu
  - `AINOVEL_MAX_RUN_MINUTES` = 60
  - chặn khi đĩa sắp đầy
  - dọn tài khoản rỗng sau 30 ngày
- Không captcha đợt này.
- Truyện chỉ đổi key cùng provider (`zai` ≠ `zai-coding`).

Số câu hỏi đã hỏi user: **0**. Mọi chỗ cần sửa đều là sửa cho khớp code hoặc tooling, hoặc cho đúng ý định đã chốt. Không có điểm mới cần user quyết.

## Kiểm theo yêu cầu

| Tiêu chí | Kết quả |
|---|---|
| Mỗi phase tự đủ để cook trong phiên mới (context links, file sở hữu, bước, verify) | Đạt. Phase nào cũng có Context (đường dẫn file:dòng), Related Code Files, Implementation Steps, Tests/Verify, Success Criteria. Thêm hướng dẫn: vượt ~35% context thì dừng ở ranh giới bước rồi làm tiếp ở pane mới (phase 3 và 4 là hai phase nặng nhất). |
| Lệnh verify đúng script `web/package.json` và `go test` | **Đã sửa.** Trước: `cd web && npm run …`. Repo dùng pnpm (`packageManager: pnpm@12.11.2`, `pnpm-lock.yaml`, CI `pnpm -C web …`, Makefile). Ngoài ra `cd web` làm hỏng các lệnh chạy sau nó trong cùng khối: phase 1 `grep … internal/server`, phase 11 `go test ./internal/server/...`. Nay mọi phase dùng `pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test && pnpm -C web build`, chạy từ gốc repo. Phase 2–4 trước đây thiếu `lint`/`test`, nay đã đủ (phase 2 có sửa file test). |
| Phụ thuộc thứ tự | Đạt. `dependencies` trong frontmatter khớp sơ đồ: 1→2→3→4, 1→5, {4,5}→6→7→8→9→10→11→12. Mỗi phase chỉ dùng output của phase đứng trước nó: `NewLimiterPer` (1→4), `ClassifyProviderError` (3→4), `AddKeyForm` (6→8), `progress-ring`/`last-read` (7→9/10), slot `actions` (9→11). |
| Không hai phase song song sửa cùng file | Đạt. Chỉ có một cặp song song: phase 5 với 2–4, trong worktree riêng. Phase 5 sở hữu `styles/*`, `components/*`, `lib/{use-tab-title,needs-attention,cover-tone}.ts`, khối `title` của `messages.ts`, và một dòng trong `queries.ts`. Phase 2–4 chỉ chạm web ở `types.gen.ts` và 2 fixture test. Đã grep: các fixture `Book` khác (`status-banner.test.tsx`, `stream-store.test.ts`) dùng `as Book`, nên không vỡ khi thêm field bắt buộc. Bảng file dùng chung thêm 5 dòng còn thiếu (`books/handlers.go`, `jobs/supervisor.go`, `worker/config.go`, flow test, wiring đĩa). |
| Acceptance criteria đo được | **Đã sửa 5 chỗ mơ hồ**: AA (phase 5) đổi thành tỉ lệ ≥ 4.5:1 kèm số đo; "axe/jsx-a11y nếu có" (phase 7) đổi thành assert selector, vì repo không có hai công cụ này; "nhìn là biết" (phase 9) đổi thành `aria-current="step"` + dòng summary; "rộng hơn hiện tại" (phase 9) đổi thành `scrollWidth ≤ 375` và bỏ `42dvh`; `git diff --exit-code` (phase 12) đổi thành so checksum. |

## Verification pass (đối chiếu code)

- Đã kiểm khoảng 65 tham chiếu file:dòng/symbol trên 12 phase, bằng grep/sed trên code thật.
- Verified: 59 | Failed (đã sửa trong plan): 6 | Unverified: 0. Tier: Full.
- Failed:
  1. `npm run` sai package manager. Bằng chứng: `web/package.json` có `packageManager: pnpm@12.11.2`, `.github/workflows/web.yml:60-69`. Sửa ở plan.md và mọi phase.
  2. Phase 7 lấy URL repo từ `go.mod`, nhưng `go.mod:1` là `github.com/voocel/ainovel-cli` (upstream tiếng Trung). Đổi sang `https://github.com/jhin1m/ainovel-cli-vi` (`README.md:204`).
  3. Phase 4 gọi `diskFree(DataDir)` trong `books/handlers.go`, nhưng `diskFree`/`lowDiskBytes` thuộc package `server` (`internal/server/diskfree_unix.go:8`, `server.go:279`). Sửa: `server.go` truyền hàm `func() (uint64, bool)` vào `books`; `ok=false` thì không chặn.
  4. Phase 7 dựa vào axe/eslint-plugin-jsx-a11y, nhưng `web/eslint.config.js` và `package.json` không có. Sửa: assert selector trong test.
  5. Phase 11 trỏ `books/handlers.go:255-258`, chuỗi `provider_mismatch` thực ở L261. Đã cập nhật.
  6. Phase 1 có thể xoá nhầm `VerifyUserPassword`, vì `/api/me/password` vẫn gọi hàm này (`auth/handlers.go:372`). Đã ghi rõ: giữ hàm và `userLimiter`, chỉ sửa comment.

## Lỗi logic phát hiện thêm (đã sửa)

- **Chia lượt quay vòng (phase 4).** Plan cũ cắt lượt chạy khi "có truyện queued của user khác". `schedule()` bỏ qua truyện chờ khi user của nó đã chạm `MaxJobsPerUser` hoặc truyện đang trong backoff (`supervisor.go:146-147`). Nếu truyện chờ thuộc loại đó, truyện bị cắt sẽ được chạy lại ngay, rồi bị cắt lại ở tick sau. Sửa: chỉ cắt khi (1) quá N phút, (2) hết slot toàn cục, (3) có truyện chờ của user khác đủ điều kiện chạy theo đúng điều kiện `blocked`. Có test âm cho từng điều kiện.
- **Mâu thuẫn về grep "invite".** plan.md và phase 12 đòi grep rỗng trong docs. Nhưng cũng phase 12 lại bắt docs nâng cấp/rollback phải nhắc drop `invite_codes`, và test Up/Down của 0004 tất yếu chứa tên bảng. Sửa: ngoại lệ rõ ràng gồm migration 0002/0004, `internal/server/db/db_test.go` (file test migration có sẵn, nay chốt là chỗ đặt test 0004/0005), và mục nâng cấp/rollback của `docs/web-server.md`.
- **Kiểm `types.gen.ts` báo khác giả (phase 12).** `git diff --exit-code` sau `make gen` sẽ fail nếu các phase trước chưa commit. Đổi sang so checksum trước và sau `make gen`. CI vẫn dùng `git diff` trên checkout sạch, nên vẫn khớp.
- **Worktree cho phase 5:** thêm bước `pnpm -C web install --frozen-lockfile` và thời điểm merge (sau phase 4).

## Câu hỏi mở của red-team: đã đóng

plan.md đổi "Câu hỏi còn mở" thành "Câu hỏi đã chốt khi validate" (captcha A; không PATCH chéo `zai` ↔ `zai-coding`; chấp nhận mọi con số giới hạn). Chỉ còn một việc xác minh cần làm, không phải quyết định: model, context window và giá Z.ai, kiểm ở phase 2.

## Whole-Plan Consistency Sweep

- Đọc lại: plan.md, phase-01 … phase-12.
- Số thay đổi đã đối chiếu: 9. Gồm lệnh pnpm, ngoại lệ grep invite, file test migration, điều kiện chia lượt, chỗ đặt hàm đo đĩa, URL repo, checksum types, tiêu chí đo được, và đóng câu hỏi mở.
- Grep tìm thuật ngữ cũ (`npm run`, `cd web &&`, `git diff --exit-code`, `go.mod`, `câu hỏi mở`, `255-258`, `trừ migration 0002`, `có người khác đang chờ`, `diskFree(DataDir)`, `jsx-a11y`): mọi chỗ còn lại đều có chủ ý. Đó là log red-team cũ, câu "không lấy từ go.mod", và ghi chú CI.
- `ak plan status`: nhận đủ 12 phase, frontmatter `validated` không làm hỏng parse.
- Mâu thuẫn chưa giải quyết: **0**.

## Khuyến nghị

Cook được. Bắt đầu bằng `/ak:cook /Users/jhin1m/Desktop/ducanh-project/ainovel-cli-vi/plans/261013-1650-web-ux-onboarding-providers/plan.md` ở pane mới, mỗi phase một pane.

## Câu hỏi chưa giải quyết

Không có.
