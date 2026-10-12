//check-han:ignore-file
// Bảng mặc định tiếng Trung cũ: chỉ dùng để nhận ra và gỡ phần system_defaults khỏi
// các snapshot được tạo khi mặc định còn là bảng từ cấm tiếng Trung.

package userrules

// legacyZHDefaults sao y rules.SystemDefaults() của bản gốc (language=zh). Giữ cố định
// ở đây vì rules.SystemDefaults() hiện trả về bảng theo ngôn ngữ đang chạy, nên không
// còn cách nào khác để biết snapshot cũ đã nhúng những mục mặc định nào.
var legacyZHDefaults = struct {
	ForbiddenPhrases []string
	FatigueWords     map[string]int
}{
	ForbiddenPhrases: []string{"某种程度上", "值得注意的是", "不知为何", "五味杂陈"},
	FatigueWords: map[string]int{
		"不禁": 1, "竟然": 1, "仿佛": 2, "此外": 1, "然而": 2,
		"一丝": 2, "一抹": 2, "一缕": 2, "宛如": 1, "不由得": 1,
		"像一": 3, "沉默了": 2, "没有说话": 2, "几息": 3, "一息": 3, "数息": 2,
	},
}
