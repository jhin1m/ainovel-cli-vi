# Cook phase 6: icon provider + UI key

Ngày: 2026-10-13 | Phase: [phase-06](../phase-06-provider-icons-and-key-ui.md) | Review: [from-code-reviewer-to-cook-p06-review-report.md](./from-code-reviewer-to-cook-p06-review-report.md)

Review chấm 7.5/10: 1 High, 1 Medium, 7 Low. Đã sửa High, Medium và 4 Low. Các Low còn lại ghi ở dưới.

## Đã làm
- `components/provider-icon.tsx`:
  - Path SVG lấy từ `@lobehub/icons-static-svg@1.95.1` (MIT, LobeHub). Header ghi nguồn. Không thêm dependency.
  - Bản màu đặc cho anthropic (claude), deepseek, glm (zhipu). Còn lại dùng mono `currentColor`: openrouter, gemini, openai, qwen, zai/zai-coding, grok (xai).
  - Không dùng bản gradient, vì id trùng khi render nhiều icon.
  - Provider lạ hiện ô chữ cái đầu. `Object.hasOwn` chặn nhầm `toString`.
- `features/keys/provider-meta.ts`: `keyUrl` + mô tả 1 dòng.
  - URL đã curl ngày 2026-10-13: openrouter, anthropic (platform.claude.com), gemini, qwen (bailian, đúng endpoint dashscope.aliyuncs.com), z.ai, bigmodel.cn trả 200 hoặc chuyển tới trang đăng nhập.
  - OpenAI, DeepSeek và x.ai trả 403: trang chặn bot của Cloudflare, URL là trang key chính thức.
- `features/keys/provider-picker.tsx`:
  - Radiogroup, roving tabindex. Mũi tên (quay vòng), Home và End chuyển focus và chọn.
  - Thẻ cao ≥44px. Badge "Thuê bao" màu `warm`. Lưới 2 cột, từ md lên 3 cột.
- `features/keys/add-key-form.tsx`:
  - Luồng provider → tên gợi nhớ (placeholder là nhãn provider) → API key (nút hiện/ẩn) → "Lưu key".
  - Link "Lấy key {provider} ↗" nằm ngoài radio, có `noopener noreferrer` và chữ sr-only "(mở tab mới)".
  - Lưu xong: xoá key thô khỏi state, đổi sang khối "Đã lưu key …", tự kiểm theo `key_id`, rồi gọi `onSaved`.
  - Lỗi 409 `key_limit` hiện thông báo kèm link tới trang key.
  - Lỗi field có `aria-describedby` và lỗi đầu tiên được focus.
  - Props: `onSaved`, `defaultProvider?`, `compact?`, `onClose?` (nút Huỷ/Xong).
- `features/keys/key-check-status.tsx`: vùng `role="status"` dùng chung.
  - Hiện: đang kiểm / "Key hoạt động với {model}" / `message` của server kèm "Mã lỗi gốc: {code}".
  - `plan_mismatch`/`auth_failed` thì gợi ý xoá rồi thêm lại. 429 thì "Đã kiểm quá nhiều lần, thử lại sau."
- `add-key-dialog.tsx`: chỉ còn vỏ `Dialog size="lg"`.
- `keys-page.tsx`:
  - Thẻ key có icon, nhãn provider, label (ẩn nếu trùng nhãn provider), `••••last4`, ngày, nút "Kiểm tra" và nút xoá.
  - Lưới 1 / 2 cột (lg) / 3 cột (2xl). Empty state có nút thêm key.
  - Đã đổi px lẻ sang token, lề `md:px-0`.
- `account-page.tsx`: ≥1024px chia 2 cột (thông tin tài khoản + đăng xuất | đổi mật khẩu). Đổi mật khẩu vẫn cần mật khẩu cũ.
- `queries.ts`: `useTestKey()` chỉ gửi `{key_id}`, không invalidate.
- `messages.ts`: khối `keys` (thêm) và `account` (`infoTitle`, `email`, `name`).

## Sửa theo review
- **High:** focus thoát khỏi Dialog khi Shift+Tab, vì radio `tabindex=-1` vẫn bị tính vào danh sách focus.
  - Sửa ở `components/ui/dialog.tsx` (file của phase 5, đã ghi lý do vào phase file): lọc `tabIndex >= 0`, và bắt focus nằm ngoài danh sách.
  - Test mới trong `dialog.test.tsx`: đỏ khi chưa sửa, xanh sau khi sửa.
- **Medium:** đóng dialog lúc đang lưu thì `onSaved` và lượt tự kiểm bị bỏ qua. Đổi sang `mutateAsync().then`, có test unmount giữa chừng.
- **Low:**
  - Icon đổi `Eye`/`EyeOff` theo trạng thái; bỏ chuỗi `hideKey` không dùng.
  - Nút "Kiểm tra" dùng `aria-disabled` thay `disabled` để giữ focus bàn phím.
  - Danh sách key tải lỗi vẫn có nút thêm key.
  - `min-h-[52px]` → `min-h-13`.

## Tests mới
- `provider-icon.test.tsx`: 10 provider, zai-coding trùng logo zai, fallback, chặn `toString`.
- `provider-picker.test.tsx`: một điểm dừng Tab, mũi tên/Home/End, Tab rời nhóm, chuột, badge.
- `add-key-form.test.tsx`:
  - lưu → kiểm chỉ với `{key_id}`; body tạo key đã trim;
  - plan_mismatch có mã 1113 và gợi ý;
  - 429 không chặn `onSaved`;
  - 409 `key_limit` có link, không kiểm;
  - focus và describedby của lỗi;
  - `defaultProvider` và link ngoài radio;
  - nút hiện/ẩn;
  - unmount lúc đang lưu.
- `dialog.test.tsx`: ca `tabindex=-1`.

## Verify
- `pnpm -C web typecheck && lint && test && build`: pass, 16 file / 108 test. Chạy lại sau khi sửa theo review.
- `go build ./... && go vet ./internal/server/... ./internal/entry/worker/...`: pass.
- `go test ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL`: mọi package ok, flow test DB có chạy. Phase này không đổi Go.
- Chạy app thật (binary `serve` ở cổng 8096, DB tạm `ainovel_p06`, đã xoá sau khi kiểm), dùng agent-browser headless:
  - 1440 light và dark: lưới thẻ key 2 cột; dialog 3 cột với đủ 10 icon; trang tài khoản 2 cột.
  - 375 dark: dialog 2 cột, `scrollWidth = 375` ở trang key và trang tài khoản, không tràn ngang.
  - "Kiểm tra" với key giả hiện `auth_failed` kèm gợi ý.
  - Chọn Z.ai Coding Plan rồi bấm mũi tên phải sang Z.ai, lưu: tự kiểm và hiện kết quả trong dialog.

## Lệch spec / để lại
- `AddKeyForm` thêm prop `onClose?`. Sau khi lưu, form đổi thành khối kết quả; phase 8 nhúng form có thể remount nó để thêm key khác.
- Link "Lấy key" là một link duy nhất cho provider đang chọn, đặt dưới lưới, thay vì mỗi thẻ một link. Cách này tránh 10 điểm dừng Tab thừa.
- Icon provider ở `new-book-form` thuộc phase 8. Tiêu chí toàn đợt "Mọi chỗ chọn hoặc hiện provider có icon" chưa tick.
- Low chưa sửa:
  - Key thô còn trong `variables` của mutation trong cache React Query tới 5 phút. Chỉ nằm trong bộ nhớ tab của chính user, không có trong DOM. Muốn sửa thì thêm `gcTime: 0` cho `useCreateKey`, nhưng plan chỉ cho phase 6 thêm `useTestKey` vào `queries.ts`.
  - Ô key `type="password"`: trình quản lý mật khẩu có thể đề nghị lưu.
  - Mở dialog từ empty state rồi lưu thì nút mở biến mất, nên focus không trả về được khi đóng.

## Câu hỏi chưa giải quyết
- Phase 8 có cần biết kết quả tự kiểm không, ví dụ để chặn tạo truyện khi `plan_mismatch`? Nếu cần thì thêm callback `onChecked` cho `AddKeyForm` ở phase 8. Hiện chưa thêm (YAGNI).

Status: DONE
