# Trạng thái cuối: DONE

Ngày: 2026-10-12. Branch `vi` = Việt hoá đầy đủ ainovel-cli trên upstream/main 638ca24.

- Repo: https://github.com/jhin1m/ainovel-cli-vi (private, default branch `vi`). Push qua SSH (`core.sshCommand` trong repo trỏ `~/.ainovel/github_known_hosts`); token gh thiếu scope `workflow` nên không push qua HTTPS được.
- Remote `upstream` đã khoá push (`pushurl=DISABLED`).

## Kiểm tra
| Hạng mục | Kết quả |
|---|---|
| `go build ./... && go test ./...` | xanh (27 package có test, 0 FAIL) |
| `go run ./scripts/check-han` | model=0 log=0 asset=0, mã thoát 0 |
| `go run ./scripts/check-han -punct` | 5 chỗ dấu câu toàn chiều rộng, đều là dữ liệu parse (giữ có chủ đích) |
| README.vi.md, SYNC.md, NOTICE | có |
| Chạy thử headless | đạt (xem dưới) |

## Chạy thử headless (truyện tiên hiệp 3 chương, language=vi, GLM glm-5.3 qua z.ai)
Thư mục: `~/projects/ainovel-smoke/run{1..4}`, yêu cầu ở `yeu-cau.txt`.

| Lần | Binary | Kết quả |
|---|---|---|
| run1 | sớm (chỉ prompt/tui/rules) | 3 chương, 1219/1633/1331 chữ, 0 chữ Hán, 0 đoạn lặp |
| run2 | sau review | 3 chương, 1840/1708/1947 chữ, 0 chữ Hán, 0 đoạn/câu lặp |
| run3 | cuối, `reasoning_effort: medium` | DỪNG ở bước lập kế hoạch: z.ai trả HTTP 400 mã 1210 "This model always engages in thinking and cannot be disabled; please use low, high, or max". Arbiter nhận ra lỗi cấu hình và tạm dừng đúng cách |
| run4 | cuối (fff12ec), override dự án `reasoning_effort: high` | 3 chương, 1569/1440/1646 chữ (mục tiêu ~1500), 0 ký tự CJK, 0 đoạn/câu lặp; book.md, premise, nhân vật, dàn ý đều tiếng Việt |

Ghi nhận: GLM đôi khi tự chèn vài chữ Hán vào bản nháp (vd. "亲自", "断面"); luật `cjk_leak` + bước tự kiểm của writer bắt và sửa trước khi nộp, chương cuối sạch.

## Danh sách commit chính (upstream/main..vi)
- Khung: bfa52dd (config `language`, nạp assets theo ngôn ngữ), c5e6633 (scripts/check-han), 050f27f (sửa CheckLanguage).
- Đợt 1: prompts 2f10879 0f0574d bc24d35; rules bd76eec eeb4e38; tui e89405a 15b0994 738d630 46661ed.
- Đợt 2: host 1f492c8 ebc00bd c9c08fc; tools d0f7ac3 cc3c47f; rest 294fc8b c551059 e6cf017 981b99e 31ebbc6 2fcc5f1; 66944bb.
- Đợt 2b (bản vá kentjuno): 2ddd6c2 72d413e caa2707; a5befeb.
- Đợt 3: 097a974 (README.vi/SYNC/NOTICE), 67e6bd1, 6ef7157, e2a6bd5 (sửa theo review), fff12ec (dấu câu, check-han -punct), README khuyến nghị effort GLM.
Đầy đủ: `git log --oneline upstream/main..vi`.

## Bản vá kentjuno
Đã port (có test tái hiện fail trước/xanh sau):
- dcc09e1: SkeletonArcs (chặn hoàn kết khi còn cung khung), StalledOutline (dàn ý giậm chân), chuẩn hoá tiêu đề chương khi xuất (chuyển vào exporter); luật cjk_leak/broken_word (phần rules).
- 639a1dd: luật trùng lặp, rò rỉ tiếng Anh, phân bố hook (đã cải tiến, giảm báo nhầm).
- 998665f: plan_chapter được ghi kế hoạch cho chương trong hàng chờ viết lại; thông báo ReviseOutline rõ lối ra.
- 9b5c138: không dời con trỏ chương khi lập kế hoạch chương sau trong hàng chờ; chặn id phục bút giữ chỗ/đã bị chiếm (đặt ở kiểm tra trước PendingCommit, bỏ qua khi phát lại commit cũ).

Đã bỏ (lý do):
- dcc09e1 RenumberVolumes: upstream đã đánh số theo vị trí (assignLayeredIndexes).
- dcc09e1 lỗi expand_arc / mô tả update_compass: expand_arc đã được thay bằng expand_next_arc; phần mô tả không tái hiện được lỗi.
- 998665f OversizedArc (trần 8 chương/cung): mâu thuẫn prompt architect-long hiện tại (estimated_chapters ≥ 8); editor upstream chỉ đọc 3–5 chương gần nhất.
- 9b5c138 Router RewriteHeadNeedsDirective: novel_context đã tiêm brief viết lại cho writer.
- 9b5c138 plan_chapter cho architect + StopGuard: không tái hiện được lỗi trên code hiện tại.
- 7c615ec labels.go: thay bằng dịch trực tiếp các nhãn.
- Nội dung prompt kentjuno tự thêm không có trong upstream (style wuxia, mục bổ sung chapter-guide/quality-checklist...): không giữ.

## Việc còn treo / lưu ý
1. **Config GLM của user**: `~/.ainovel/config.json` vẫn là `"reasoning_effort": "medium"` (không sửa vì file chứa API key). Trước khi chạy bộ 100 chương nên đổi thành `"high"` (chất lượng tốt, mỗi lượt nghĩ có thể vài phút) hoặc `"low"` (nhanh hơn), hoặc đặt `./.ainovel/config.json` = `{"reasoning_effort": "high"}` trong thư mục truyện như run4.
2. Ở chế độ `language=zh`, các nhãn trong file .md phái sinh (outline.md, characters.md...) và chuỗi tool/TUI là tiếng Việt (chỉ hiển thị; prompt và luật vẫn bản Trung). Xuất TXT/EPUB đã theo ngôn ngữ.
3. `assets.LoadOptions{}` rỗng = zh (giữ test baseline upstream); main/eval luôn truyền ngôn ngữ thật.
4. userrules lưu `language` trong file snapshot qua wrapper; nếu bị ghi lại mất trường sẽ dựng lại mặc định (idempotent).
5. Ước lượng token nội bộ của agentcore (thư viện ngoài) vẫn byte/4 cho văn Latin; chỗ ctxpack tự ước lượng đã dùng byte/3.
6. TUI: dòng gợi ý phím bị cắt khi cửa sổ < ~120 cột (như bản gốc).
7. `ainovel-cli update` tải bản phát hành upstream (tiếng Trung) — không dùng cho bản Việt (đã ghi trong README.vi.md).
8. Workflow CI upstream chỉ chạy trên `main`/tag nên repo này chưa có CI cho `vi`.

## Cách đồng bộ upstream
Xem SYNC.md: `git fetch upstream` → nhánh sync từ `vi` → `git merge upstream/main` → `git diff <mốc cũ> upstream/main -- assets/prompts assets/references assets/styles assets/voice.md` → dịch lại sang `assets/*/vi/` → `go run ./scripts/check-han -v` → `go build ./... && go test ./...` → merge vào `vi`, push.
