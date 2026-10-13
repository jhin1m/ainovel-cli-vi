# Cook phase 8: form tạo truyện

Ngày: 2026-10-13 | Phase: [phase-08](../phase-08-create-book-form.md) | Review: [from-code-reviewer-to-cook-p08-review-report.md](./from-code-reviewer-to-cook-p08-review-report.md)

Review chấm 8/10: 1 High, 1 Medium, 6 Low. Đã sửa High, Medium và 4 Low. 2 Low còn lại ghi ở dưới.

## Đã làm
- `new-book-logic.ts`, các hàm thuần (có test):
  - `composePrompt(idea, options)`: ghép thể loại, độ dài, giọng văn, nhân vật chính, bối cảnh thành các dòng tiếng Việt nối sau ý tưởng. Ý tưởng trống thì trả rỗng.
  - `pickDefaultModel`: model `recommended`, không có thì model đầu.
  - `priceTiers`: xếp hạng theo các mức giá khác nhau của `input+output`, chia ba khoảng; cùng giá thì cùng bậc. Trả null khi có dưới 3 model có giá hoặc mọi model cùng giá.
  - `estimateChapters`: Σ chi phí / Σ chương của các truyện cùng model, bỏ truyện thuê bao. Y = ngân sách / chi phí mỗi chương, X = 0,75·Y.
  - `validateNewBook(input, {subscription})`: thuê bao thì bỏ qua ô ngân sách. Prompt kiểm sau khi ghép, giới hạn 4000 ký tự.
- `prompt-presets.ts`: 8 mẫu (tu tiên, ngôn tình học đường/cổ đại, đô thị, trinh thám, kinh dị, khoa huyễn, võ hiệp). Bỏ `msg.newBook.ideas`.
- `prompt-section.tsx` (cột trái):
  - Chip gợi ý: ô trống thì điền thẳng; có chữ thì thay và hiện "Hoàn tác" 5s (`role="status"`).
  - "Tuỳ chỉnh thêm" `<details>`: chip thể loại (6), độ dài (3), giọng văn (3) dạng `aria-pressed`, bấm lại để bỏ chọn; ô nhân vật chính, bối cảnh.
  - Khối "Xem trước nội dung gửi cho AI" (chỉ đọc). Bộ đếm ký tự tính trên nội dung đã ghép.
- `config-panel.tsx` (thẻ "Cấu hình", sticky từ lg):
  - 0 key: `AddKeyForm compact` ngay trong thẻ. Lưu xong: tự chọn key mới, model đề xuất được chọn sẵn, khối kết quả tự kiểm (gồm cảnh báo `plan_mismatch`/`auth_failed`) nằm cạnh select key tới khi bấm "Xong". Không chặn submit.
  - Đã có key: icon provider cạnh select, nút "+ Thêm key khác" mở form tại chỗ (có Huỷ).
  - Model: option kèm "· Đề xuất" và bậc giá; badge "Đề xuất" màu warm; "Chi tiết giá" bằng lời thường. Thuê bao thì không có bậc giá và không có chi tiết giá.
  - Ngân sách: chip 2/5/10/20 US$, ô số, ước tính chương hoặc "Chưa có số đo…". Thuê bao thì thay bằng callout "Dùng quota gói …, tính bằng token, không trừ USD", và request gửi `budget_usd: 0`.
  - Lỗi 507 `disk_full` hiện câu chung. Focus ô lỗi đầu tiên.
- `new-book-form.tsx`: giữ state, lưới 2 cột từ lg (`minmax(0,1fr)` + 20–24rem). Mobile 1 cột, nút gửi ở cuối.
- `routes/_app/books/new.tsx`: bọc `max-w-6xl`.
- `messages.ts`: chỉ khối `newBook`.

## Lỗi tự gây trong lúc làm, đã sửa
- **`<form>` lồng nhau** (kiểm tay phát hiện): `AddKeyForm` là `<form>` nằm trong form tạo truyện. Bấm "Lưu key" nổi bọt submit lên form cha, làm hiện lỗi "Hãy nhập ý tưởng".
  - Sửa: form tạo truyện chỉ bọc cột trái; thẻ cấu hình nằm ngoài, nối vào form bằng `form={formId}`.
  - Test có assert `form form` = null. Đã xác nhận test đỏ trước khi sửa.
- Lưới provider 3 cột tràn chữ trong thẻ hẹp. Ép 2 cột bằng selector ở wrapper (`[&_[role=radiogroup]]:grid-cols-2`), không sửa file phase 6.

## Sửa theo review
- **H1:** chip vẫn làm mất chữ gốc khi bấm lại cùng chip (hoặc double-click), hoặc bấm hai chip liên tiếp.
  - Sửa: state hoàn tác lưu thêm `applied`. Bấm lại chip đang hiện thì không làm gì. Đổi chip khác thì giữ `prev` gốc.
  - 2 test mới, đã xác nhận đỏ với logic cũ.
- **M1:** thẻ sticky cao hơn màn 1366×768 khi form thêm key mở.
  - Sửa: thẻ có `lg:max-h-[calc(100dvh-5.5rem)] lg:overflow-y-auto`, khối nút gửi dính đáy thẻ.
  - Đo trên app thật khi form thêm key mở (nút nằm trong màn và `elementFromPoint` trúng nút):
    - 1366×768: đáy nút 683px, đáy thẻ 748px.
    - 1024×768: đáy nút 683px, đáy thẻ 748px.
    - 1440×900: đáy nút 815px, đáy thẻ 880px.
- **Low đã sửa:**
  - Enter ở ô nhân vật chính hoặc bối cảnh không tạo truyện (có test). Ô ngân sách giữ hành vi Enter như form cũ.
  - Nút "Hoàn tác" hết hạn lúc đang có focus thì trả focus về ô ý tưởng.
  - "≈ 0 chương" đổi thành "Ngân sách này chưa đủ cho 1 chương…" (có test). Lúc danh sách truyện đang tải thì không hiện câu "Chưa có số đo".
  - `aria-describedby` của select model chỉ trỏ tới badge khi badge có render.

## Tests
- `new-book-logic.test.ts`: 16 ca (`composePrompt`, `pickDefaultModel`, `priceTiers` gồm giá 0/trùng giá/hai mức, `estimateChapters` gồm loại truyện thuê bao và chia 0, validate thuê bao).
- `new-book-form.test.tsx`: viết lại, 22 ca, gồm:
  - 0 key → thêm key → tự chọn → tạo truyện;
  - `plan_mismatch` không chặn submit; "Thêm key khác" / Huỷ;
  - model đề xuất chọn sẵn khi đổi key; bậc giá chỉ ở provider có ≥3 model;
  - thuê bao: không ô ngân sách, gửi 0;
  - chip điền thẳng, chip có hoàn tác, bấm lại chip, hai chip liên tiếp, tự ẩn sau 5s;
  - tuỳ chỉnh ghép đúng và body đúng 5 trường API;
  - ước tính / chưa có số đo / chưa đủ 1 chương; chi tiết giá;
  - focus lỗi ý tưởng và ngân sách; Enter ở ô phụ; lỗi server; 507.

## Verify
- `pnpm -C web typecheck && lint && test && build`: pass, 22 file / 162 test. Chạy lại sau khi sửa theo review.
- `go build ./... && go vet ./internal/server/... ./internal/entry/worker/...` và `go test ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL`: mọi package ok, flow test DB có chạy. Phase này không đổi Go.
- App thật: binary `serve` ở cổng 8098, DB tạm `ainovel_p08` (đã drop), agent-browser headless, dark mode.
  - Thêm key giả qua UI: key được tự chọn, model đề xuất chọn sẵn, hiện cảnh báo `auth_failed` cạnh select.
  - Đã xem bằng mắt 375 / 1024 / 1440. `scrollWidth` bằng viewport ở mọi mốc.
  - Chưa xem light mode bằng mắt. Phase chỉ dùng token sẵn có.

## Lệch spec / để lại
- "+ Thêm key khác" là nút dưới select, không phải option trong select. Icon provider đặt cạnh select vì `<option>` native không chứa được SVG.
- Lỗi `keyId` hiện dưới khu vực key (spec để P2). Lỗi này chỉ xảy ra khi chưa có key; không hiện thì submit thất bại im lặng.
- Độ dài truyện chỉ ghép vào prompt. Architect có tôn trọng số chương hay không thì chưa kiểm. **Cần user thử 2–3 truyện thật** (rủi ro đã ghi trong phase).
- Low chưa sửa:
  - Khi mở "Thêm key khác" có hai control cùng tên "API key" (select của trang và ô nhập của `AddKeyForm` phase 6).
  - Nếu tải lại danh sách key lỗi ngay sau khi lưu key mới, form dùng key đầu tiên của danh sách cũ. Trường hợp tải chậm vẫn an toàn.
- Không chạy `docs-manager`/`git-manager`: docs thuộc phase 12, user cấm commit.

## Câu hỏi chưa giải quyết
- Có cần đổi nhãn ô nhập trong `AddKeyForm` (vd. "Nội dung API key") để không trùng tên với select "API key"? Việc này sửa file của phase 6.

Status: DONE
