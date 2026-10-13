# Báo cáo docs-manager: tài liệu chế độ serve

## Đã sửa
- `docs/architecture.md` (tiếng Trung, giữ văn phong): thêm §9.2 "Web 服务模式" (sơ đồ server -> supervisor -> worker -> Host/Engine, bảng package, 4 ý chính gồm SSE và quy tắc không có auto-resume), bổ sung `entry/worker`, `server/`, `web/`, `deploy/` vào cây thư mục §9, trỏ sang `docs/web-server.md`. Không đổi số mục nên không gãy tham chiếu.

## Kiểm chéo docs/web-server.md với code
Khớp, không cần sửa: toàn bộ env và giá trị mặc định (config.go), `serve invite create --uses/--expires` (cmd.go), 4 biến env của worker (HOME/PATH/LANG/TZ), notify tắt, đường dẫn users/<uid>/books/<id> và home, SSE ping 20s, drain 60s, stopGrace, ngưỡng disk_low 1 GiB, deploy/{ainovel-serve.service,Caddyfile,backup.sh}, Go 1.25 / Node 24 / pnpm 12, Makefile `make build`.

## Chưa xác minh
- Sơ đồ ở §9.2 nêu giao thức stdin/stdout "init/steer/abort/shutdown" dựa trên protocol.go; chưa đối chiếu hết từng loại message stdout.
- `make gen` (tygo) sinh `web/src/api/types.gen.ts`: lấy từ Makefile, chưa chạy.
- Không chạy validate-docs.cjs.

Status: DONE
Summary: Thêm §9.2 và mục cây thư mục vào architecture.md; web-server.md khớp code nên không sửa.
Concerns: Không commit. docs/ khác không liên quan nên không động tới.
