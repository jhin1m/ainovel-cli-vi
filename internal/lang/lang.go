// Package lang giữ ngôn ngữ sáng tác của tiến trình (tiếng Việt hoặc tiếng Trung).
//
// Ngôn ngữ được đặt một lần lúc khởi động từ config `language` rồi chỉ đọc. Các hàm
// thuần như đếm chữ hay luật kiểm tra văn bản đọc giá trị này thay vì nhận tham số,
// để không phải xuyên tham số qua hàng trăm call site.
package lang

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Code là mã ngôn ngữ đã chuẩn hoá.
type Code string

const (
	VI Code = "vi" // tiếng Việt (mặc định)
	ZH Code = "zh" // tiếng Trung (hành vi gốc của upstream)
)

// Default là ngôn ngữ dùng khi config không khai báo.
const Default = VI

var current atomic.Value // Code

func init() { current.Store(Default) }

// Parse chuẩn hoá chuỗi cấu hình thành Code. Chuỗi rỗng trả về Default.
func Parse(s string) (Code, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return Default, nil
	case "vi", "vn", "vietnamese", "tiếng việt", "tieng viet":
		return VI, nil
	case "zh", "cn", "chinese", "中文": //check-han:ok bí danh cấu hình
		return ZH, nil
	}
	return "", fmt.Errorf("language %q không hỗ trợ (chỉ nhận \"vi\" hoặc \"zh\")", s)
}

// Set đặt ngôn ngữ của tiến trình.
func Set(c Code) { current.Store(c) }

// Current trả về ngôn ngữ đang dùng.
func Current() Code { return current.Load().(Code) }

// IsVI cho biết tiến trình đang sáng tác tiếng Việt.
func IsVI() bool { return Current() == VI }
