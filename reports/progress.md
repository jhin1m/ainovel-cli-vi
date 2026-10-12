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
| 0 | branch vi + baseline + repo GitHub + push | xong | origin = git@github.com:jhin1m/ainovel-cli-vi (private, default vi); push qua SSH với core.sshCommand trỏ ~/.ainovel/github_known_hosts (token gh thiếu scope workflow) |
| 0 | khung ngôn ngữ (config, lang, assets loader) + test | xong | bfa52dd |
| 0 | scripts/check-han | xong | baseline: reports/check-han-baseline.txt (model=2109 log=93 asset thiếu 39) |
| 1 | prompts | xong | 39 file vi, test load_vi_test.go; sửa lỗi CheckLanguage (voice.md) 050f27f |
| 1 | rules + domain | xong | WordCount vi theo từ; luật cjk_leak, english_residue, technical_residue, broken_word, self_duplication, repeated_sentence, copied_previous_chapter; từ mỏi/cụm cấm vi |
| 1 | tui | xong | merge b1e96bf, check-han internal/entry 443→0; lưu ý: gợi ý phím bị cắt <120 cột (như bản gốc) |
| 2 | host | đang chạy | worktree ~/projects/ainovel-wt/host, agent `host` |
| 2 | tools | đang chạy | worktree ~/projects/ainovel-wt/tools, agent `tools` |
| 2 | rest | đang chạy | worktree ~/projects/ainovel-wt/rest, agent `rest` |
| 2b | patches: port bản vá quy hoạch kentjuno (dcc09e1 phần domain/tools/store, 998665f, 9b5c138, 7c615ec) | chờ | sau Đợt 2 để tránh đụng file; chỉ port lỗi còn tái hiện |
| 3 | check-han=0, docs (README.vi, SYNC, NOTICE), smoke test, push, final-status | chờ | controller |

## Nhật ký
- 2026-10-12: tạo branch vi, baseline xanh.
- Prompt worker lưu ở ~/projects/ainovel-wt/_prompts/ (common.md + task-<x>.md), khởi chạy bằng launch.sh <task> <pane> <hướng>.
- D6. Bản vá quy hoạch kentjuno tách thành việc riêng `patches` sau Đợt 2 (chúng trải qua domain/tools/store/flow — chạy song song sẽ xung đột với worker dịch chuỗi).
- 2026-10-12: Đợt 0 xong, đã push. Đợt 1 khởi chạy 3 worker.
- tui DONE_WITH_CONCERNS (chỉ quan sát), đã merge + push. host khởi chạy sớm (không đụng file Đợt 1). tools chờ rules merge (rules được sửa commit_chapter.go, novel_context_builders.go).
- prompts DONE_WITH_CONCERNS: các việc ngoài phạm vi đã chuyển: premise heading alias → tools; nhãn "Can thiệp gốc của người dùng" engine.go → host (đã nhắn); guard "tóm tắt quyển/cung" → rest. Bỏ phần kentjuno tự thêm không có trong upstream (wuxia, mục bổ sung chapter-guide...).
- rules DONE_WITH_CONCERNS: đã merge. Việc treo chuyển: editor.md bảng luật (controller làm xong), userrules snapshot đổi lang + đo độ dài rune ở store/stylestat/ctxpack → rest; host/cocreate.go → host; read_chapter.go word_count → tools.
