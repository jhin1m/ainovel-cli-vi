# Brief: lập plan đợt web UX + onboarding + providers

Chạy skill `ak:plan` (chế độ mặc định, không --tdd) với các input:
- plans/reports/brainstorm-261013-1634-web-app-ux-onboarding-zai-report.md (6 mục đã duyệt)
- plans/reports/ux-audit-261013-1645-web-app-improvements-report.md
- plans/reports/research-261013-1645-local-execution-report.md

Quyết định của user (KHÔNG hỏi lại):
1. Đăng ký tự do, bỏ hoàn toàn mã mời (API, form, CLI `serve invite`, migration drop bảng). Không làm guest.
2. Thêm API key không cần nhập lại mật khẩu (đổi mật khẩu vẫn cần).
3. Provider: thêm "Z.ai Coding Plan" (base URL cố định https://api.z.ai/api/coding/paas/v4, cờ subscription → không áp ngân sách USD, hiện token) VÀ "Z.ai" API thường (https://api.z.ai/api/paas/v4, trả theo token). KHÔNG hiện cảnh báo điều khoản. Đổi label `glm` → "GLM (bigmodel.cn)". Giữ allowlist chống SSRF. Worker giữ mã/nội dung lỗi gốc của provider; 1113 → thông báo rõ, không retry. Nút "Kiểm tra key". Xác minh mã model + context window với docs z.ai lúc làm.
4. Icon provider: SVG inline (nguồn @lobehub/icons-static-svg, MIT), lưới thẻ provider thay <select>.
5. Layout gần full (≤1600px), reader giữ 65ch.
6. Onboarding "Bắt đầu trong 3 bước", thêm key inline ở trang tạo truyện.
7. Từ audit UX: làm toàn bộ P0 (0.1–0.3), toàn bộ P1 (1.1–1.11; mục 1.2 chỉ làm tiền tố tiêu đề tab, KHÔNG Notification API), và "Hướng thị giác" (token: bóng, màu phụ ấm, thang chữ, bo góc, bìa thẻ). P2 để sau.
8. Local runner: KHÔNG làm đợt này (dự án riêng). Chỉ thêm khối "Chạy trên máy bạn" (CLI) trong web + docs (PA4, effort S).

Yêu cầu chia phase:
- Mỗi phase sẽ được cook trong MỘT pane Claude riêng, context mới → mỗi phase phải tự đủ (context links, file sở hữu, bước làm, test), vừa sức một phiên (~ dưới 35% context của model 200K), không quá to.
- Ranh giới file rõ để tránh 2 phase sửa cùng file khi không cần; ghi rõ phụ thuộc thứ tự. Gợi ý thứ tự: backend (auth/keys/providers/worker errors) → design tokens + layout → provider icons/key UI → onboarding/library → create-book form → book screen → reader → P0 bits → docs.
- Nêu lệnh verify từng phase (go test ./internal/server/... ; cd web && npm run typecheck/lint/test/build — kiểm tra package.json để lấy đúng script).
- Plan dir: plans/261013-1650-web-ux-onboarding-providers/

Trả lời câu hỏi của skill bằng các quyết định ở trên; chỉ hỏi user khi thật sự có điểm mới chưa được quyết.
Kết thúc in: Status + đường dẫn plan.md + danh sách phase.
