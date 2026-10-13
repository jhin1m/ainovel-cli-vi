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

## Phần bản web (`serve`) của fork

Bản web nằm trong thư mục riêng của fork, upstream không có nên không xung đột:
`internal/server/`, `internal/entry/worker/`, `web/`, `deploy/`, `docs/web-server.md`, `Makefile`, `tygo.yaml`,
`.github/workflows/web.yml`. Không sửa `.github/workflows/ci.yml`, `.goreleaser.yml`, `Dockerfile`; bản release CLI giữ nguyên.

File upstream mà nhánh này có sửa (cần để ý khi merge `upstream/main`; liệt kê bằng
`git diff --name-only --diff-filter=M vi...feat/web-app`):

| File | Thay đổi của fork |
|---|---|
| `cmd/ainovel-cli/main.go` | Thêm hai lệnh con `worker` và `serve` (cùng import `os/signal`, `syscall`, `internal/entry/worker`, `internal/server`) và hàm `runWorker`. |
| `.gitignore` | Thêm quy tắc cho `web/dist/*` (giữ `.gitkeep`) và `web/node_modules/`. |
| `go.mod`, `go.sum` | Thêm dependency của server: pgx v5, goose v3, golang.org/x/crypto, golang.org/x/time. |
| `README.md` | Thêm mục "Bản web" (trước mục "Cấu hình"). |
| `docs/architecture.md` | Thêm mục §9.2 về chế độ web serve và các thư mục mới trong cây §9. |

Cách giải conflict:

- `cmd/ainovel-cli/main.go`: giữ phần mới của upstream, chép lại hai khối `if len(os.Args) > 1 && os.Args[1] == "worker"` / `"serve"`
  ở đầu `main()` và hàm `runWorker` cuối file.
- `go.mod` / `go.sum`: lấy bản của upstream rồi để Go tính lại, đừng sửa tay:
  ```bash
  git checkout --theirs go.mod go.sum
  go mod tidy
  ```
  Nếu `go mod tidy` bỏ mất dependency của server thì `go build ./...` sẽ báo thiếu; chạy lại `go mod tidy` sau khi
  chắc chắn mã `internal/server` còn trong cây.
- `.gitignore`: giữ cả hai bên (thêm khối "Bản web" vào cuối file).
- `README.md`: xem hướng dẫn README ở trên, rồi thêm lại mục "Bản web".
- Sau merge chạy thêm: `go test ./internal/server/... ./internal/entry/worker/...` (cần `AINOVEL_TEST_DATABASE_URL` để chạy test DB)
  và `make gen && git diff --exit-code` nếu upstream đổi kiểu mà `internal/server/api` dùng.
