# Audit UI/UX web app — đề xuất cải tiến

Ngày: 2026-10-13 · Branch: feat/web-app · Phạm vi: `web/src` (đọc toàn bộ code) · Chỉ audit, không sửa code.
Phương pháp: đọc tĩnh code + đối chiếu checklist ak:ui-ux-pro-max (a11y, touch, layout, form, nav, typography). Không chạy `npm run dev` (backend cần Postgres; không dựng mock) → chưa kiểm tra bằng ảnh chụp.
Nền: các quyết định trong `brainstorm-261013-1634-web-app-ux-onboarding-zai-report.md` (6 mục) giữ nguyên; báo cáo này chỉ bổ sung/chi tiết hoá.

## Tóm tắt
- Gốc của "khó dùng / không biết bắt đầu từ đâu": (1) màn chính không nói AI **đang làm gì** và **bước tiếp theo** là gì; (2) form tạo truyện đưa quyết định kỹ thuật (key, model, US$) mà không có ngữ cảnh ("5 US$ được mấy chương?"); (3) vài luồng **tắc đường** không có lối ra trong UI.
- Gốc của "đơn giản": mọi thứ là thẻ trắng viền xám cùng một cấp, thiếu điểm nhấn, thiếu số liệu trực quan; nội dung bị ép cột 768px.
- Ngoài 6 mục đã duyệt: 3 lỗi P0 chặn luồng, 9 cải tiến P1, 9 mục P2.

## P0 — chặn luồng, làm trước

| # | Vấn đề | Đề xuất | File | Công sức |
|---|---|---|---|---|
| 0.1 | **Không xoá được truyện.** API `DELETE /api/books/{id}` có, UI không có. Khi chạm `MaxBooksPerUser`, server báo "hãy xoá bớt truyện cũ" (`internal/server/books/handlers.go:200`) — user không có cách làm. Đăng ký mở (đã duyệt) → lỗi này sẽ gặp nhiều hơn. | Thêm `useDeleteBook`; nút "Xoá truyện" trong menu "⋯" ở header trang truyện (cạnh Xuất), dialog xác nhận nêu tên truyện + "không hoàn tác". Sau xoá → về thư viện. Trạng thái `deleting` đã có badge. | `api/queries.ts`, `features/books/book-screen.tsx`, `lib/messages.ts` | S |
| 0.2 | **Không đổi được key của truyện.** Xoá key → dialog nói "cho đến khi bạn chọn key khác" (`messages.ts` keys.confirmBody) nhưng không chỗ nào chọn; `api_key_id` rỗng → truyện kẹt vĩnh viễn. `PATCH` đã hỗ trợ `api_key_id`. | Ở trang truyện, khi `api_key_id === ''` (hoặc lỗi key) hiện banner "Key của truyện đã bị xoá" + select key cùng provider + nút "Dùng key này". Cũng cho đổi key trong panel thống kê (ít dùng → để trong menu "⋯"). | `features/books/status-banner.tsx`, `book-screen.tsx`, `messages.ts` | S |
| 0.3 | **Tăng ngân sách xong vẫn đứng.** Server đổi `budget_exhausted → paused` (`handlers.go:269`), dialog đóng, user phải tự tìm nút Tiếp tục; chuỗi `msg.book.budget.saved` có sẵn nhưng không dùng. | Nút chính trong dialog "Lưu và viết tiếp" (PATCH rồi `resume`), nút phụ "Chỉ lưu". Áp dụng tương tự cho 0.2. | `features/books/budget-dialog.tsx` | S |

## P1 — giá trị rõ cho người mới và người theo dõi truyện

### Trang chủ / thư viện (bổ sung mục 6 đã duyệt)
| # | Vấn đề | Đề xuất | File | Công sức |
|---|---|---|---|---|
| 1.1 | Thẻ truyện không có hành động nhanh; truyện "Chờ duyệt" cần 2 click mới tới nút "Viết chương tiếp". | Hàng nút phụ dưới thẻ theo trạng thái: Chờ duyệt → "Đọc chương N" + "Viết tiếp"; Hoàn thành → "Đọc từ đầu"; Lỗi → "Xem lỗi". Thẻ không còn là 1 `<Link>` bao trọn (tránh link lồng nút): tiêu đề là link, nút là nút. Đầu thư viện: dải "Cần bạn xử lý" gom các truyện chờ duyệt/lỗi/hết ngân sách. | `features/books/book-card.tsx`, `library-page.tsx` | M |
| 1.2 | Thông báo khi xong chương: chế độ duyệt mặc định bật → AI dừng chờ, user không biết nếu đang ở tab khác. Thư viện đã poll 15s (`queries.ts:62`), trang truyện có SSE. | (a) `document.title` theo route + tiền tố "(1)" khi có truyện chuyển sang chờ duyệt/hoàn thành/lỗi; (b) tuỳ chọn bật Notification API (xin quyền khi user bấm, không tự hỏi). Không cần push server/web-push (YAGNI). | `components/app-shell.tsx`, hook mới `lib/use-book-alerts.ts`, `messages.ts` | M |

### Trang tạo truyện
| # | Vấn đề | Đề xuất | File | Công sức |
|---|---|---|---|---|
| 1.3 | Bấm chip "Gợi ý nhanh" **ghi đè** ý tưởng user đã gõ, không hoàn tác. | Chỉ điền khi ô trống; nếu đã có chữ → hỏi "Thay thế ý tưởng hiện tại?" hoặc hiện "Hoàn tác" 5s. | `features/books/new-book-form.tsx` | S |
| 1.4 | Chỉ có 1 ô prompt tự do; người mới không biết viết gì cho "đủ cụ thể". Engine chỉ nhận `prompt` (không có số chương/thể loại). | "Tuỳ chỉnh thêm" (thu gọn mặc định): chip **thể loại** (tu tiên, ngôn tình, đô thị, trinh thám, kinh dị, khoa huyễn), **độ dài** (truyện ngắn ~5–10 chương / vừa / dài), **giọng văn** (nghiêm túc / hài / u tối), ô "nhân vật chính", "bối cảnh". Ghép thành đoạn văn nối vào prompt — **không đổi API**. Xem trước prompt cuối cùng. Thay 4 ý tưởng cố định bằng ~8 mẫu theo thể loại. | `new-book-form.tsx`, file mới `features/books/prompt-presets.ts`, `messages.ts` | M |
| 1.5 | Ngân sách "5 US$" vô nghĩa với người mới; dòng giá nói "đầu vào/đầu ra mỗi 1 triệu token… để ngân sách luôn có hiệu lực" — thuật ngữ. | Dưới ô ngân sách: "≈ X–Y chương với model này" = ngân sách ÷ chi phí/chương ước tính. Chi phí/chương: ưu tiên số thực của user (`total_cost_usd / completed_count` các truyện cùng model), fallback hằng số token/chương × giá model. Thêm chip nhanh 2 / 5 / 10 / 20 US$. Thời gian: "mỗi chương thường mất vài phút" (chỉ khi có số đo thật). Giá token chuyển vào `<details>` "Chi tiết giá". Với provider thuê bao (Z.ai Coding Plan, đã duyệt bỏ USD) → ẩn ô ngân sách, hiện "dùng quota gói, tính bằng token". | `new-book-form.tsx`, `new-book-logic.ts`, `lib/format.ts` | M |
| 1.6 | Chọn model là `<select>` tên trần; không biết model nào hợp viết truyện. | Đánh dấu 1 model "Đề xuất" mỗi provider (danh sách cứng phía client hoặc cờ từ catalog), mặc định chọn nó; hiện "rẻ / cân bằng / chất lượng" theo giá tương đối. | `new-book-form.tsx`, có thể `internal/server/keys/catalog.go` | S |
| 1.7 | Màn rộng: form 1 cột dài, nút "Bắt đầu viết" nằm cuối cuộn. | ≥1024px: 2 cột — trái: ý tưởng + gợi ý + tuỳ chỉnh; phải: thẻ "Cấu hình" sticky (key/model/ngân sách/ước tính/duyệt chương) + nút submit. Mobile giữ 1 cột. Chi tiết hoá mục 5 đã duyệt. | `new-book-form.tsx`, `routes/_app/books/new.tsx` | M |

### Trang theo dõi truyện
| # | Vấn đề | Đề xuất | File | Công sức |
|---|---|---|---|---|
| 1.8 | Không trả lời "AI đang làm gì, còn bao lâu". `snapshot.agents[].summary/tool`, `phase`, `in_progress_chapter`, `model_name`, token đều **không được dùng**; `msg.book.stats.model` định nghĩa mà không hiện. Timeline thô là nguồn duy nhất. | Dải "Đang làm" dưới tiêu đề: stepper 4 bước (Lập dàn ý → Viết chương N → Biên tập → Chờ bạn duyệt) + 1 dòng `agents[active].summary`. Panel thống kê thêm Model (kèm icon provider, đã duyệt), Token vào/ra (bắt buộc cho thuê bao), vòng tiến độ % chương. | `book-screen.tsx`, file mới `features/books/progress-stepper.tsx`, `messages.ts` | M |
| 1.9 | Layout tab "Đang viết": bản nháp (42dvh) + timeline (48dvh) xếp dọc, **2 vùng cuộn lồng** trong trang cuộn; mobile còn thêm SteerBox sticky + dòng hint → màn 375px còn rất ít chỗ đọc. Khi nới 1600px (đã duyệt), bản nháp sẽ dài >150 ký tự/dòng. | ≥1280px: bản nháp trái (giới hạn 68–72ch, giống reader) — timeline phải (thay cột stats 320px; stats chuyển lên dải đầu trang). Mobile: stats gọn 1 dòng dưới tiêu đề; SteerBox thu thành nút "Góp ý" mở sheet; hint chuyển vào sheet. | `book-screen.tsx`, `steer-box.tsx`, `book-stream/stream-text.tsx`, `timeline.tsx` | M |

### Trang đọc
| # | Vấn đề | Đề xuất | File | Công sức |
|---|---|---|---|---|
| 1.10 | Theme reader mặc định `light` kể cả khi OS đang tối → mở chương bị chói. | Mặc định theo `prefers-color-scheme` khi chưa lưu cài đặt. | `features/reader/reader-settings.ts` | S |
| 1.11 | Thiếu tiện ích đọc cơ bản: không phím tắt, header chỉ "Chương n" (không tên truyện/tên chương), không nhớ chỗ đọc, `msg.reader.last` có mà không dùng. | Phím ←/→ chương trước/sau, `Esc` về truyện; header: tên truyện · tên chương; thanh tiến độ cuộn mỏng; lưu `lastRead` (localStorage) → thẻ thư viện + trang truyện có "Đọc tiếp chương N"; chương cuối hiện "Đây là chương mới nhất" + nút "Viết chương tiếp" nếu truyện đang chờ duyệt. | `features/reader/reader-page.tsx`, `reader-settings.ts`, `book-card.tsx` | M |

## P2 — đánh bóng, làm khi rảnh
| # | Vấn đề → Đề xuất | File | Công sức |
|---|---|---|---|
| 2.1 | Loading toàn spinner → skeleton cho lưới thẻ thư viện và đầu trang truyện (tránh nhảy layout). | `components/ui/states.tsx`, `library-page.tsx`, `book-screen.tsx` | S |
| 2.2 | Lỗi form: không focus ô lỗi đầu tiên; lỗi `keyId` không hiển thị ở đâu. → focus ô lỗi đầu, gắn error dưới select key. | `new-book-form.tsx` | S |
| 2.3 | Tên thương hiệu lặp: desktop có ở sidebar **và** header thư viện. → header thư viện chỉ hiện <lg. | `library-page.tsx` | S |
| 2.4 | Nhãn lệch: nút "Dừng" nhưng badge "Tạm dừng". → thống nhất "Tạm dừng". | `messages.ts` | S |
| 2.5 | Nút Xuất khi chưa có chương là `<span title>` — tooltip không hiện trên cảm ứng. → hiện chữ phụ "Chưa có chương để xuất" cạnh nút. | `book-screen.tsx` | S |
| 2.6 | Đổi route không chuyển focus/đọc tiêu đề cho screen reader; không có skip link. → focus `<h1>` khi đổi route + skip link "Tới nội dung". | `app-shell.tsx` | S |
| 2.7 | Danh sách key: không biết key nào đang được truyện dùng. → "Đang dùng bởi N truyện" (đếm từ `useBooks`), cảnh báo mạnh hơn khi xoá key đang dùng. | `features/keys/keys-page.tsx` | S |
| 2.8 | Trang đăng nhập/đăng ký trống trải, không giải thích sản phẩm. → ≥1024px chia đôi: phải là "Cách hoạt động" 3 bước + ảnh minh hoạ trang theo dõi; mobile giữ form. | `features/auth/auth-layout.tsx` | M |
| 2.9 | Dàn ý: nhân vật/chương là danh sách dọc dài. → màn rộng: nhân vật dạng lưới 2–3 cột; chương đã xong có link "Đọc". | `features/books/book-tabs.tsx` | S |

## Trạng thái rỗng / lỗi / loading / mobile / dark mode — ghi nhận
- Tốt: mọi query có Loading/Error/Empty; ErrorState có "Thử lại"; touch target ≥44px nhất quán; `prefers-reduced-motion` tôn trọng; dialog có focus trap + Esc; `role=log/status/alert` đúng chỗ; token màu có cặp dark mode đầy đủ; tương phản đạt AA (muted #5a606a/#f4f5f3 ≈ 6:1, dark #9aa1ab/#101214 ≈ 7:1, chữ trạng thái trên nền trạng thái ≥5:1).
- Thiếu: skeleton (2.1); empty state thư viện chỉ 1 câu (mục 6 đã duyệt xử lý); mobile trang truyện quá dài (1.9); reader không theo OS (1.10); không có nút đổi theme app — **không đề xuất** (theo OS là đủ, YAGNI).

## Hướng thị giác — bớt "đơn giản" mà không loè loẹt
Giữ tinh thần "Thư phòng mực" (teal mực + giấy), Be Vietnam Pro cho UI, Literata cho tiêu đề và văn bản truyện. Không đổi font/màu chủ đạo (gợi ý tự động của skill là xanh dương + cam + Playfair — bỏ qua, không hợp sản phẩm và tốn công đổi).

1. **Phân cấp bằng bề mặt, không bằng viền.** Hiện tại mọi khối = `bg-surface` + `border-line` → phẳng đều. Thêm 3 cấp: nền trang `--bg`; thẻ `--surface` + bóng rất nhẹ (`0 1px 2px rgb(0 0 0/.04), 0 4px 12px rgb(0 0 0/.04)`), bỏ viền ở light mode; khối nhấn (dải "Cần bạn xử lý", stepper, CTA trang chủ) dùng `--accent-soft` với viền trái 3px accent. Dark mode giữ viền, bỏ bóng.
2. **Thêm 1 màu phụ ấm** cho điểm nhấn văn chương: `--ink-warm: #9a5b1e` (dark: `#e3a96b`) + nền `#f7eee3` / `#2e2216` — dùng cho số chương, trích dẫn, badge "Đề xuất". Teal vẫn là màu hành động duy nhất (1 CTA chính/màn).
3. **Số liệu là điểm nhấn thị giác:** thẻ truyện và trang truyện hiển thị số lớn (Literata 600, `tabular-nums`) cho "12/40 chương", "38.200 chữ"; vòng tiến độ nhỏ thay thanh 6px. Đây là chỗ chữa cảm giác "đơn giản" rẻ nhất.
4. **Thẻ truyện có "bìa":** dải màu 4px trên đầu thẻ (hoặc ô chữ cái đầu 48px kiểu gáy sách) sinh từ hash id truyện trong 6 tông trầm (teal, chàm, gạch, rêu, mận, đất) — có cá tính mà không cần ảnh.
5. **Thang chữ cố định** (hiện có 15 cỡ, nhiều px lẻ 13/15/17/19/28/32): 12 · 14 · 16 · 18 · 22 · 28 · 36, khai báo token trong `@theme`; nhãn nhỏ uppercase giữ 12px tracking rộng. Chữ thân ≥14px; ô nhập giữ 16px (chống zoom iOS).
6. **Bo góc 3 mức:** `control` 10px, `card` 16px, `pill`; bỏ các giá trị lẻ 3/9/14/18px. Khoảng cách theo bậc 4/8: gap 8/12/16/24/32, padding trang 16 (mobile) → 24 (≥768) → 32 (≥1280).
7. **Mật độ:** desktop dùng lưới (thư viện 2–4 cột đã duyệt), section cách 32px, tiêu đề trang 28–36px Literata; mobile giữ thoáng hiện tại.
8. **Chuyển động tối thiểu:** hover thẻ nâng bóng + `translateY(-1px)` 150ms; số liệu stream đổi giá trị không nhảy (tabular-nums). Không thêm hoạt ảnh trang trí.

File chạm: `styles/tokens.css`, `styles/app.css`, `book-card.tsx`, `components/ui/*` (S–M tuỳ phạm vi; nên làm cùng đợt layout rộng để tránh sửa 2 lần).

## Thứ tự gợi ý
1. P0 (0.1–0.3) — nhỏ, gỡ tắc đường.
2. Gộp đợt "layout rộng + onboarding" đã duyệt với 1.7, 1.9, 1.1 và hướng thị giác (cùng chạm file).
3. 1.3–1.6 (form tạo truyện) → 1.8 (stepper) → 1.10–1.11 (reader) → 1.2 (thông báo).
4. P2 khi rảnh.

## Câu hỏi còn mở
- Ước tính chi phí/chương (1.5): có số đo thật từ CLI (token trung bình/chương theo model) để làm hằng số fallback không? Không có thì chỉ hiện ước tính khi user đã có ≥1 chương.
- Preset độ dài (1.4) ghép vào prompt có đủ để Architect tôn trọng số chương không, hay cần trường `target_chapters` ở API? Cần thử 2–3 truyện.
- Model "Đề xuất" (1.6) do ai chọn — cứng ở client hay cờ trong catalog server?
- Thông báo trình duyệt (1.2) có cần không, hay chỉ tiền tố tiêu đề tab là đủ?
- Màu phụ ấm và "bìa" sinh từ hash (hướng thị giác 2, 4) — user muốn xem mockup trước khi làm không?
