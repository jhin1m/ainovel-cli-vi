# Brief: nghiên cứu chạy engine trên máy người dùng (server không gánh)

Bối cảnh:
- Repo Go `ainovel-cli` + web React (./web). Chế độ `serve`: server Go (Postgres, auth, API key BYOK mã hoá) spawn worker subprocess cho mỗi lượt chạy engine; nội dung truyện lưu file trong AINOVEL_DATA_DIR. Đọc docs/web-server.md, docs/architecture.md, internal/server/jobs/supervisor.go, internal/entry/worker/.
- Thiết kế đã duyệt cho đợt UX: plans/reports/brainstorm-261013-1634-web-app-ux-onboarding-zai-report.md (đọc trước).
- Server hiện gánh RAM/CPU worker, đĩa, egress tới LLM. User muốn máy người dùng gánh phần chạy.

Câu hỏi: có cách nào để engine chạy trên máy người dùng mà vẫn dùng web làm giao diện? Đánh giá ít nhất:
1. Local runner: user cài `ainovel-cli` (đã có CLI), chạy `ainovel-cli runner --pair <token>` kết nối outbound tới server (WebSocket/long-poll), nhận job, chạy worker local, đồng bộ chương/sự kiện về server. Key có thể giữ local, server không cần giữ key.
2. Chạy hoàn toàn trên trình duyệt: Go → WASM (TinyGo/Go wasm), lưu trữ OPFS/IndexedDB, LLM gọi trực tiếp từ browser (kiểm CORS từng provider: OpenRouter, Anthropic `anthropic-dangerous-direct-browser-access`, OpenAI, Gemini, DeepSeek, Z.ai...). Kích thước bundle, giới hạn tab bị ngủ/đóng khi chạy lâu.
3. App desktop (Wails/Tauri) bọc engine + UI web local.
4. Giữ nguyên CLI/TUI làm đường "chạy local", web chỉ là bản hosted.
Với mỗi phương án: khả thi với codebase hiện tại (chỉ ra file/điểm nối), công sức ước lượng (S/M/L/XL), trải nghiệm người dùng, bảo mật (key, xác thực runner), server còn gánh gì, rủi ro. Kết luận khuyến nghị rõ ràng + có nên đưa vào đợt hiện tại hay tách dự án riêng.

Ràng buộc:
- CHỈ nghiên cứu, KHÔNG sửa code. Được dùng web search và đọc code.
- Viết tiếng Việt có dấu, ngắn gọn, dưới ~250 dòng.
- Output: plans/reports/research-261013-1645-local-execution-report.md
- Kết thúc bằng mục "Câu hỏi còn mở".
- Cuối cùng in: Status: DONE | DONE_WITH_CONCERNS | BLOCKED + đường dẫn báo cáo.
