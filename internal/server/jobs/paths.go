// Package jobs quản lý vòng đời process worker cho từng truyện: hàng đợi, giới hạn
// đồng thời, gửi lệnh, dừng, khôi phục sau restart, và phát realtime qua hub.
package jobs

import (
	"fmt"
	"path/filepath"
	"regexp"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// BookDir là thư mục truyện. Đường dẫn chỉ suy từ ID trong DB; kiểm dạng UUID để
// không thể trèo ra ngoài DATA_DIR dù dữ liệu có bị sửa.
func BookDir(dataDir, userID, bookID string) (string, error) {
	if !uuidRe.MatchString(userID) || !uuidRe.MatchString(bookID) {
		return "", fmt.Errorf("id không hợp lệ")
	}
	return filepath.Join(dataDir, "users", userID, "books", bookID), nil
}

// HomeDir là HOME riêng của user cho worker: ~/.ainovel (rules, cache giá…) không
// dùng chung giữa các user.
func HomeDir(dataDir, userID string) (string, error) {
	if !uuidRe.MatchString(userID) {
		return "", fmt.Errorf("id không hợp lệ")
	}
	return filepath.Join(dataDir, "users", userID, "home"), nil
}
