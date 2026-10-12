# Đồng bộ với upstream

Branch `vi` theo dõi `voocel/ainovel-cli` (remote `upstream`, nhánh `main`). Bản gốc tiếng Trung
được giữ nguyên trong repo nên mỗi lần đồng bộ chỉ cần dịch lại phần upstream vừa đổi.

## Quy ước cần nhớ

| Bản gốc (Trung, không sửa) | Bản Việt (sửa ở đây) |
|---|---|
| `assets/prompts/<tên>.md` | `assets/prompts/vi/<tên>.md` |
| `assets/voice.md` | `assets/prompts/vi/voice.md` |
| `assets/references/**` | `assets/references/vi/**` (cùng cấu trúc) |
| `assets/styles/<tên>.md` | `assets/styles/vi/<tên>.md` |
| `README.md` của upstream | `README.zh.md` (bản gốc), `README.md` (bản Việt) |

- Chuỗi trong mã Go được dịch thẳng tại chỗ (không có bảng i18n). Dữ liệu chỉ dùng khi
  `language=zh` nằm trong file có `//check-han:ignore-file` hoặc dòng có `//check-han:ok`.
- `go run ./scripts/check-han` liệt kê chữ Hán còn sót trong string literal Go và trong
  `assets/*/vi`, kèm file vi còn thiếu. Mã thoát 0 = sạch.

## Các bước

```bash
# 1. Lấy upstream mới, tạo nhánh làm việc
git fetch upstream
git switch vi
BR=sync-$(date +%Y%m%d)
git switch -c $BR
LAST=$(git merge-base HEAD upstream/main)   # mốc upstream đã đồng bộ lần trước

# 2. Merge
git merge upstream/main
#   Xung đột trong file .go: giữ logic mới của upstream, dịch lại chuỗi tiếng Trung mới
#   thêm vào sang tiếng Việt. Xung đột trong assets gốc: luôn lấy bản upstream
#   (git checkout --theirs assets/prompts/<tên>.md ...).
#   README.md ở đây là bản Việt, bản gốc nằm ở README.zh.md. Nếu upstream sửa README:
#     git checkout --ours README.md
#     git show upstream/main:README.md > README.zh.md   # rồi thêm lại dòng chọn ngôn ngữ ở đầu
#     git diff $LAST upstream/main -- README.md          # dịch phần đổi vào README.md

# 3. Xem upstream đã đổi gì trong tài nguyên gốc
git diff --stat $LAST upstream/main -- assets/prompts assets/references assets/styles assets/voice.md
git diff $LAST upstream/main -- assets/prompts/writer.md      # từng file một

# 4. Dịch lại phần thay đổi sang file vi tương ứng (theo bảng trên).
#    File gốc mới thêm → tạo bản vi cùng tên; file gốc bị xoá → xoá bản vi.
#    Giữ nguyên tên tool, khoá JSON, placeholder {{...}}.

# 5. Kiểm tra chữ Hán còn sót (chuỗi Go mới từ upstream, bản vi thiếu)
go run ./scripts/check-han -v

# 6. Build và test
go build ./... && go test ./...

# 7. Merge vào vi và push
git switch vi
git merge --no-ff $BR
git push origin vi
```

## Kiểm tra nhanh sau đồng bộ

- `go test ./assets` kiểm tra bản vi đủ file, giữ đủ placeholder và tên tool so với bản gốc.
- Chạy thử headless một truyện ngắn (xem README.md) và xem chương ra có chữ Hán hoặc
  lặp đoạn không.
