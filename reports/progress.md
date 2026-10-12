# Tiến độ Việt hoá ainovel-cli (branch `vi`)

Khởi động lại: đọc file này trước, rồi `git log --oneline vi`, `git worktree list`, `herdr agent list`.

## Baseline
- `vi` tạo từ upstream/main 638ca24 (2026-10-12). Đã gỡ tracking upstream khỏi `vi`.
- Baseline `go build ./... && go test ./...`: XANH (32 package ok).

## Quyết định thiết kế
- D1. Ngôn ngữ: package `internal/lang` giữ ngôn ngữ hiện tại (mặc định `vi`), set một lần lúc khởi động từ `config.language`. Lý do: WordCount/rules là hàm thuần gọi khắp nơi; truyền tham số sẽ đụng hàng trăm call site → xung đột upstream lớn.
- D2. Tài nguyên tiếng Việt nằm cạnh bản gốc theo mẫu `<dir>/vi/`: `assets/prompts/vi/*.md`, `assets/references/vi/**`, `assets/styles/vi/*.md`, voice ở `assets/prompts/vi/voice.md`. Bản gốc Trung giữ nguyên → SYNC chỉ cần diff file gốc rồi dịch lại.
- D3. `assets.LoadOptions.Language` rỗng = zh (giữ baseline test upstream); main/eval truyền ngôn ngữ thật. `assets.CheckLanguage(lang)` báo lỗi rõ danh sách file vi thiếu.
- D4. Chuỗi Go (mô tả tool, lỗi, chỉ dẫn context, TUI, log) dịch thẳng sang tiếng Việt, không làm i18n hai chiều (giảm khối lượng, ít đụng cấu trúc). Dữ liệu chỉ dùng cho zh (vd. từ cấm tiếng Trung) để trong file có dòng `//check-han:ignore-file` hoặc dòng có `//check-han:ok`.
- D5. `scripts/check-han` là chương trình Go (go/parser) quét string literal (bỏ comment, _test.go, testdata, assets gốc) + assets/*/vi/. Mục tiêu = 0.

## Bảng việc
| Đợt | Việc | Trạng thái | Ghi chú |
|---|---|---|---|
| 0 | branch vi + baseline + repo GitHub + push | đang làm | |
| 0 | khung ngôn ngữ (config, lang, assets loader) + test | chờ | |
| 0 | scripts/check-han | chờ | |
| 1 | prompts (assets vi) | chờ | worker |
| 1 | rules + domain | chờ | worker |
| 1 | tui | chờ | worker |
| 2 | host | chờ | worker |
| 2 | tools | chờ | worker |
| 2 | rest | chờ | worker |
| 3 | check-han=0, docs (README.vi, SYNC, NOTICE), smoke test, push, final-status | chờ | controller |

## Nhật ký
- 2026-10-12: tạo branch vi, baseline xanh.
