# Cook phase 7: onboarding + thư viện

Ngày: 2026-10-13 | Phase: [phase-07](../phase-07-onboarding-and-library.md) | Review: [from-code-reviewer-to-cook-p07-review-report.md](./from-code-reviewer-to-cook-p07-review-report.md)

Review chấm 8/10, gồm 1 High, 2 Medium và 4 Low. Đã sửa High, cả 2 Medium và 3 Low. Còn 1 Low cần user quyết, ghi ở cuối.

## Đã làm
- `lib/last-read.ts`:
  - `getLastRead` / `setLastRead`, key `ainovel:last-read:{bookId}`.
  - Bọc try/catch. Số không hợp lệ (≤0, số lẻ) bị bỏ qua.
- `components/progress-ring.tsx`: vòng SVG, `role="img"`, `aria-label` "x%", màu theo trạng thái. Phase 9 dùng lại được.
- `features/onboarding/getting-started.tsx`:
  - Tính 3 bước từ dữ liệu thật: số key, số truyện, có truyện `completed_count ≥ 1`. Không lưu cờ.
  - Bước hiện tại có `.callout` + `aria-current="step"`. Bước đã xong có tick và sr-only "(Đã xong)".
  - Bước 3 dẫn tới truyện có chương (nếu không có thì truyện mới nhất). Chưa có truyện thì không hiện nút.
  - Xong cả 3 bước thì ẩn hẳn.
- `features/onboarding/run-locally.tsx`:
  - `<details>` gồm so sánh web/CLI, lệnh cài và lệnh chạy lấy đúng từ README (`git clone git@github.com:jhin1m/ainovel-cli-vi.git` …), nút chép, link README.
  - Nút chép dùng `navigator.clipboard` trong try/catch. Lỗi thì báo "chép thủ công". Kết quả báo qua `role="status"`.
  - Không gọi backend.
- `features/books/book-card.tsx` (làm lại):
  - Dải bìa 4px theo `coverTone(id)`.
  - Tiêu đề là `<Link>` duy nhất tới trang truyện. Lớp `::after` phủ cả thẻ để bấm chỗ nào cũng mở được. Các nút nằm trên lớp đó (`z-10`), không lồng nhau.
  - Số liệu lớn Literata `tabular-nums`, vòng tiến độ, badge, icon provider + model ngắn (bỏ tiền tố `hãng/`). Truyện thuê bao hiện token.
  - Nút nhanh (`quickActions`, tối đa 2):
    - chờ duyệt → "Đọc chương N" + "Viết tiếp" (`useNextChapter`, cùng lệnh với trang truyện);
    - hoàn thành → "Đọc từ đầu", hoặc "Đọc tiếp chương N" nếu có `lastRead`;
    - lỗi/gián đoạn → "Xem lỗi" (+ đọc tiếp);
    - còn lại → "Đọc tiếp chương N" nếu có `lastRead` ≤ số chương đã xong.
- `features/books/library-page.tsx`:
  - Thương hiệu chỉ hiện dưới lg. H1 28→36px kèm nút "Viết truyện mới".
  - Thứ tự: checklist (chỉ render khi keys và books đã tải xong) → dải "Cần bạn xử lý" (`needsAttention`, dùng chung với tiêu đề tab) → bộ lọc → lưới 1/2/3/4 cột (`sm`/`lg`/`min-[90rem]`) → RunLocally (cả ở empty state).
  - Lề `px-4 md:px-0`.
- `api/queries.ts` (mục "Bổ sung" của phase):
  - `booksPollInterval()`: 15s khi tab hiển thị, 60s khi tab ẩn. Vẫn giữ `refetchIntervalInBackground`. Bật `refetchOnWindowFocus` cho `useBooks`.
  - `useCreateKey` có `gcTime: 0`.
- `messages.ts`: mở rộng khối `library`, thêm khối `onboarding`. Bỏ `library.words` vì không còn dùng.

## Sửa theo review
- **H1:** focus ring của thẻ bị `overflow-hidden` cắt mất. Sửa bằng `focus-visible:after:-outline-offset-2` để outline nằm trong thẻ.
- **M1:** quay lại tab vẫn có thể phải chờ chu kỳ 60s. Sửa bằng `refetchOnWindowFocus: true` trong `useBooks` (mặc định chung đang tắt).
- **M2:** "Viết tiếp" bấm lại được trước lượt poll, server trả 409. Sửa bằng `disabled={isPending || isSuccess}`, có assert trong test.
- **Low đã sửa:**
  - bỏ chuỗi không dùng;
  - báo chép thành công cho screen reader;
  - `<main>` bọc cả trang.

## Lỗi tự gây trong lúc làm, đã sửa
- Xoá `library.words` bằng `str.replace` làm mất luôn `book.chaptersTab.words`. Typecheck bắt được, đã khôi phục. `git diff` chỉ còn mất `library.words`.
- `min-[1440px]:grid-cols-4` thua `lg:grid-cols-3` vì sắp thứ tự CSS khác đơn vị (px với rem). Đổi sang `min-[90rem]`.
- Vòng tiến độ của truyện hoàn thành gần như không thấy ở light mode. Đổi sang `text-st-done-bg`.

## Tests mới
- `last-read.test.ts`: 4 ca, gồm storage ném lỗi.
- `getting-started.test.tsx`: 5 tổ hợp: chưa có gì / có key / có truyện chưa có chương / xong hết (ẩn) / có chương nhưng đã xoá key.
- `book-card.test.tsx`:
  - thẻ chờ duyệt có 2 nút; `a a, a button, button a, button button` = null;
  - "Viết tiếp" POST `/api/books/b1/next` rồi khoá nút;
  - bìa, số liệu, vòng 30%, icon provider; thuê bao hiện token;
  - lỗi + lastRead; `quickActions`, `shortModel`.
- `library-page.test.tsx`: user mới; dải xử lý đúng 4 trạng thái trong 7, 7 thẻ, checklist ẩn.
- `run-locally.test.tsx`: lệnh khớp README thật (đọc file), chép OK, chép lỗi, link `noopener`.
- `queries.test.tsx`:
  - chu kỳ poll 15s/60s;
  - cache mutation tạo key rỗng sau unmount. Đã xác nhận test đỏ khi bỏ `gcTime: 0`.

## Verify
- `pnpm -C web typecheck && lint && test && build`: pass, 22 file / 134 test. Chạy lại sau khi sửa theo review.
- `go build ./... && go vet ./internal/server/... ./internal/entry/worker/...`: pass.
- `go test ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL`: mọi package ok, flow test DB có chạy. Phase này không đổi Go.
- App thật: binary `serve` ở cổng 8097, DB tạm `ainovel_p07` (đã drop), 5 truyện mẫu chèn thẳng vào DB, agent-browser headless.
  - Lưới 1/2/3/4/4 cột ở 375/640/1024/1440/1920. `scrollWidth` bằng viewport ở mọi mốc.
  - Đã xem bằng mắt dark 375/1440/1920 và light 1440. Checklist của user mới đúng.
  - Chưa chụp lại sau khi sửa H1 (focus ring). Đã kiểm CSS build có `outline-offset: calc(2px * -1)`.
  - Lưu ý cho lần kiểm sau: service worker PWA giữ bản cũ, phải mở phiên browser mới.

## Lệch spec / để lại
- Dải "Cần bạn xử lý": mỗi dòng là link tới trang truyện, nhãn theo trạng thái ("Duyệt chương", "Xem lỗi", "Tăng ngân sách", "Chạy lại"). Thao tác thật nằm ở banner của trang truyện, cách một lần bấm. "Viết tiếp" trực tiếp có ở thẻ.
- Thẻ chờ duyệt không hiện thêm "Đọc tiếp chương N" để giữ tối đa 2 nút.
- Lệnh `git clone` dùng SSH theo README. User không có SSH key với GitHub sẽ lỗi. Muốn đổi sang HTTPS thì phải sửa README trước, vì test bắt khi hai bên lệch nhau.

## Câu hỏi chưa giải quyết
- Nhãn "Tăng ngân sách" / "Chạy lại" trong dải xử lý chỉ mở trang truyện. Có nên đổi thành nhãn trung tính (vd. "Mở truyện"), hoặc gắn hành động thật ngay tại dải không?

Status: DONE
