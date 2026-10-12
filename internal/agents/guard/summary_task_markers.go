package guard

import (
	"strings"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// Cụm tiếng Việt nhận diện task tóm tắt, khớp cách prompt Editor gọi tên
// ("tóm tắt quyển", "tóm tắt cung"/"tóm tắt cung truyện"). So khớp không phân biệt hoa thường.
var (
	volumeSummaryMarkersVI = []string{"tóm tắt quyển"}
	arcSummaryMarkersVI    = []string{"tóm tắt cung"}
)

// isVolumeSummaryTask báo task có phải yêu cầu sinh tóm tắt quyển không.
func isVolumeSummaryTask(task string) bool {
	return strings.Contains(task, "save_volume_summary") ||
		containsAnyMarker(task, volumeSummaryMarkersVI, volumeSummaryMarkersZH)
}

// isArcSummaryTask báo task có phải yêu cầu sinh tóm tắt cung truyện không.
func isArcSummaryTask(task string) bool {
	return strings.Contains(task, "save_arc_summary") ||
		containsAnyMarker(task, arcSummaryMarkersVI, arcSummaryMarkersZH)
}

// containsAnyMarker so khớp cụm theo ngôn ngữ hiện tại: cụm tiếng Trung chỉ dùng khi language=zh.
func containsAnyMarker(task string, vi, zh []string) bool {
	markers := vi
	if lang.Current() == lang.ZH {
		markers = zh
	}
	lower := strings.ToLower(task)
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return false
}
