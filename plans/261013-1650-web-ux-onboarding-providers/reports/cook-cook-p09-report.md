# Cook phase 9: trang truyện (tiến độ + layout)

Ngày: 2026-10-13 | Phase: [phase-09](../phase-09-book-screen-progress-and-layout.md) | Review: [from-code-reviewer-to-cook-p09-review-report.md](./from-code-reviewer-to-cook-p09-review-report.md)

Review chấm 7/10: 1 High, 3 Medium, 6 Low. Đã sửa High, cả 3 Medium và 5/6 Low. Low còn lại ghi ở cuối.

## Đã làm
- `progress-stepper.tsx`:
  - Stepper 4 bước "Lập dàn ý → Viết chương N → Biên tập → Chờ bạn duyệt".
  - Hàm thuần `stepFromSnapshot(snapshot, status)` map theo code worker/host:
    - `completed` hoặc phase `complete` → đã xong;
    - `waiting_review` → bước 4;
    - phase init/premise/outline hoặc chưa có snapshot → bước 1;
    - phase writing: flow reviewing/rewriting/polishing → Biên tập, còn lại → Viết chương.
    - Khi truyện đang chạy thì agent đang làm được ưu tiên: architect → Lập dàn ý (cung/quyển kế tiếp), editor → Biên tập.
  - Bước hiện tại có `aria-current="step"`. Bước đã qua có tick + sr-only "(Đã xong)".
  - Dòng agent: `agentActivity` bỏ tiền tố kỹ thuật ("engine → …", "writer → tool") và đổi 13 tool sang tiếng Việt (`msg.book.stepper.tools`).
  - Có `aria-live="polite"` throttle 4s (`useThrottled`); dòng nhìn thấy để `aria-hidden` để không đọc hai lần. Công tắc "Duyệt từng chương" chuyển vào thẻ này.
- `book-stats-strip.tsx`: vòng tiến độ + "x/y chương", số chữ, icon provider + model, token vào/ra (vi-VN, `tabular-nums`), chi phí/ngân sách + nút "Tăng ngân sách". Truyện thuê bao thì không có USD và không có nút. Mobile là một dòng cuộn ngang trong dải (đã khoá trục Y).
- `book-header.tsx`: nút về, tiêu đề, trạng thái, `controls` (dừng/tiếp tục/xuất) và **slot `actions` cho menu "⋯" của phase 11**. `ExportMenu` chuyển từ `book-screen.tsx` sang đây.
- `writing-tab.tsx`, `use-columns-layout.ts`:
  - Chế độ 2 cột (bản nháp `3fr` ≤72ch | hoạt động `2fr`): mỗi cột tự cuộn, trang không cuộn. Chỉ bật khi màn ≥1280px và phần dưới thanh tab còn ≥ 18rem. Hook đo bằng `matchMedia` + `ResizeObserver` vì banner làm phần đầu trang cao lên/thấp xuống.
  - Còn lại: xếp dọc, cuộn trang. Bản nháp hiện 4 đoạn cuối + nút "Xem toàn bộ bản nháp (n đoạn trước)" / "Chỉ xem đoạn mới nhất". Timeline là khung 60dvh.
- `steer-box.tsx`:
  - Bỏ sticky. Có `variant` inline/sheet.
  - `SteerFab` (<768px): nút "Góp ý" nổi, có chấm khi đang có góp ý chờ, mở `Dialog` dạng sheet có nút "Đóng". Nút có ở mọi tab.
  - Từ md: ô góp ý inline cạnh bản nháp; ở tab Chương/Dàn ý thì nằm cuối trang.
- `stream-text.tsx`: bỏ `max-h-[42dvh]`, thêm `className` và `tail`. `timeline.tsx`: chiều cao qua `className` (mặc định 60dvh); đổi px lẻ sang token.
- `book-screen.tsx` còn 234 dòng (từ 350). `messages.ts` chỉ sửa khối `book` (stats, stepper, writing.showAll/collapse, steer.open/hasPending/close). `stream-store.ts` không đổi.

## Sửa theo review
- **H1:** lúc editor duyệt chương vừa commit (`in_progress_chapter = 0`, `completed_count = N`, flow vẫn `writing`), stepper báo "Viết chương N+1" đã tick.
  - Sửa: ở bước Biên tập dùng `in_progress_chapter || max(done, 1)`.
  - Có test mới, test này đỏ với logic cũ. Test cũ khoá hành vi sai cũng đã sửa.
- **M1:** dưới 1280px bản nháp không còn tự bám chữ mới và đẩy timeline xuống quá xa. Sửa bằng chế độ `tail` (4 đoạn cuối + nút xem toàn bộ).
- **M2:** màn ≥1280px nhưng thấp, có banner, thì vừa cuộn khung trang vừa cuộn cột. Sửa bằng `useColumnsLayout`: không đủ chỗ thì xếp dọc. Bỏ fallback `xl:h-dvh xl:overflow-y-auto`.
- **M3:** dòng agent hiện chuỗi kỹ thuật. Sửa bằng `agentActivity` + bảng tên tool.
- **Low đã sửa:**
  - px lẻ (timeline, dải số liệu, menu xuất file) và `md:text-[28px]` thừa;
  - header không còn cột trống khi chưa có `actions`;
  - chip agent không bị đọc hai lần;
  - góp ý có lại ở mọi tab (`pb-24` giờ luôn đúng);
  - thêm test render cả `BookScreen` (nút ngân sách mở hộp thoại, nhánh thuê bao, tab Chương có ô góp ý).

## Tests
- `progress-stepper.test.tsx`:
  - mỗi phase/flow, snapshot null, editor ngay sau commit (H1), ưu tiên agent, agent cũ khi không chạy;
  - chờ duyệt, hoàn thành, lỗi/tạm dừng;
  - `agentActivity`, `activeAgent` (cả state "running" của fakeworker), đúng một `aria-current`, aria-live, `useThrottled` (fake timers).
- `book-screen-layout.test.tsx`: dải số liệu (thường/thuê bao), header một slot `actions`, `WritingTab` 2 cột / xếp dọc + tail, `SteerFab` sheet, `BookScreen` tích hợp.
- `use-columns-layout.test.tsx`: bật khi đủ cao; tắt khi phần đầu cao (banner), màn hẹp, hoặc không ở tab "Đang viết".

## Verify
- `pnpm -C web typecheck && lint && test && build`: pass, 25 file / 203 test. Chạy lại sau khi sửa theo review.
- `go build ./... && go vet ./internal/server/... ./internal/entry/worker/...` và `go test ./internal/server/... ./internal/entry/worker/... ./internal/bootstrap/... ./internal/llmretry/... ./internal/host/...` với `AINOVEL_TEST_DATABASE_URL`: mọi package ok, flow test DB có chạy. Phase này không đổi Go.
- App thật: `serve` dev ở cổng 8099 + fakeworker, DB tạm `ainovel_p09` (đã drop), agent-browser headless.
  - 1600×1000 và 1366×768 đang chạy: 2 cột, `scrollHeight = innerHeight`, dòng agent và aria-live có nội dung.
  - 1366×768 chờ duyệt (có banner): tự xếp dọc, trang cuộn, không còn vùng cuộn lồng (đo bằng script tìm phần tử overflow-y đang cuộn được).
  - 1600×1000 có banner: vẫn 2 cột, 0 vùng cuộn thừa.
  - 375×812: `scrollWidth = 375`, không còn vùng cuộn dọc thừa, nút "Góp ý" mở sheet.
  - Đã xem light + dark; trạng thái đang chạy, chờ duyệt, hoàn thành.

## Lệch spec / để lại
- Dưới chế độ 2 cột, timeline vẫn là khung cuộn riêng 60dvh: virtualizer cần phần tử cuộn; window virtualizer thì `scrollMargin` đổi liên tục vì bản nháp phía trên đang stream.
- Truyện hoàn thành: không bước nào có `aria-current`, thay bằng câu "Đã viết xong toàn bộ truyện.".
- Bước 4 "Chờ bạn duyệt" vẫn hiện cả khi truyện ở chế độ tự động (spec ghi cố định 4 bước).
- `activeAgent` coi mọi state khác `idle`/rỗng là đang chạy: host thật dùng "working", fakeworker dùng "running".
- Low chưa sửa: dải số liệu vẫn là một điểm dừng Tab ở màn ≥768px dù lúc đó không cuộn. Muốn bỏ thì phải đo bằng JS; giá trị thấp.
- Không chạy `docs-manager`/`git-manager`: docs thuộc phase 12, user cấm commit.

## Câu hỏi chưa giải quyết
- Có nên ẩn bước "Chờ bạn duyệt" khi truyện ở chế độ tự động?

Status: DONE
