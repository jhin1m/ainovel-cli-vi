package stylestat

import (
	"os"
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// TestMain mặc định chạy test của package ở chế độ zh (hành vi gốc, dữ liệu mẫu
// tiếng Trung); các test nhánh tiếng Việt tự chuyển qua useLang.
func TestMain(m *testing.M) {
	lang.Set(lang.ZH)
	os.Exit(m.Run())
}

// useLang đặt ngôn ngữ cho một test và khôi phục ngôn ngữ cũ khi test kết thúc.
func useLang(t *testing.T, c lang.Code) {
	t.Helper()
	prev := lang.Current()
	lang.Set(c)
	t.Cleanup(func() { lang.Set(prev) })
}
