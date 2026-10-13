# Report: hub / reader / export

Files: internal/server/jobs/{hub,hub_test}.go; internal/server/books/{reader,export}{,_test}.go.
- Hub: mutex + ring 300 (upsert theo ID), sub buffer 64, non-blocking drop (close Dropped). Remove đóng mọi sub.
- Reader: layered ưu tiên nếu có chương, fallback flat; title: summary -> outline. Không IO ghi.
- Exporter: sem + per-user slot, cache `exports/<N>.<ext>`, temp file + rename; ErrExportBusy, ErrNothingToExport.
- gofmt/vet clean; `go test -race` Hub|Chapter|Outline|Export green.
Note: internal/server/jobs/fakeworker/main.go (not mine) is not gofmt-clean.
