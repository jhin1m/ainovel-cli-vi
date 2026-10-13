// Package web nhúng SPA đã build (web/dist) vào binary để `ainovel-cli serve`
// chạy được mà không cần Node hay thư mục web đi kèm.
package web

import "embed"

// Dist chứa nội dung web/dist. Khi chưa build SPA thư mục chỉ có .gitkeep;
// server khi đó trả trang báo chưa build.
//
//go:embed all:dist
var Dist embed.FS
