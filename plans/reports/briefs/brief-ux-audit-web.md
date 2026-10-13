# Brief: audit UI/UX web app và đề xuất cải tiến

Bối cảnh:
- Web React 19 + Vite + Tailwind v4 + TanStack Router/Query, lucide-react, component UI tự viết (web/src/components/ui). Routes: /login, /signup, /, /books/new, /settings/account, /settings/keys, /books/$id, /books/$id/read/$n. Layout: web/src/components/app-shell.tsx.
- Thiết kế đã duyệt: plans/reports/brainstorm-261013-1634-web-app-ux-onboarding-zai-report.md (đọc trước — không đề xuất ngược các quyết định ở đó).
- Phản hồi user: "khó dùng", "cảm giác hơi đơn giản, nhìn không biết bắt đầu từ đâu", "giao diện cần rộng hơn đáng kể".

Nhiệm vụ:
1. Đọc toàn bộ web/src (features, routes, components). Nếu chạy được `cd web && npm run dev` với mock thì tốt, không bắt buộc (backend cần Postgres).
2. Dùng skill ak:ui-ux-pro-max để audit: thông tin trang chủ, trang tạo truyện (form gì, mặc định gì, giải thích gì), trang theo dõi truyện đang chạy (tiến độ, chi phí, sự kiện), trang đọc, cài đặt, trạng thái rỗng/lỗi/loading, mobile, dark mode, phân cấp thị giác, typography.
3. Đề xuất cải tiến cụ thể, ưu tiên P0/P1/P2, mỗi mục: vấn đề → đề xuất → file chạm tới → công sức S/M/L. Ngoài 6 mục đã duyệt, tự tìm thêm cải tiến về cách sử dụng (vd. mẫu truyện gợi ý, preset thể loại, ước tính chi phí/thời gian trước khi chạy, thông báo khi xong chương, phím tắt đọc...). Giữ YAGNI: chỉ đề xuất cái có giá trị rõ.
4. Đề xuất hướng thị giác (bảng màu, font hiện có Be Vietnam Pro + Literata, mật độ, card) để bớt "đơn giản" nhưng không loè loẹt.

Ràng buộc:
- CHỈ audit/đề xuất, KHÔNG sửa code.
- Tiếng Việt có dấu, ngắn gọn, dưới ~250 dòng.
- Output: plans/reports/ux-audit-261013-1645-web-app-improvements-report.md
- Kết thúc bằng "Câu hỏi còn mở". Cuối cùng in: Status: DONE | DONE_WITH_CONCERNS | BLOCKED + đường dẫn báo cáo.
