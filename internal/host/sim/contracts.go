package sim

import (
	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/internal/llmcontract"
)

func textList(description string) map[string]any {
	return schema.Array(description, schema.String(description))
}

var sourceReportContract = llmcontract.Contract{
	Name:        "simulation_source_report",
	Description: "Từ một văn bản mẫu, chắt lọc phương pháp viết có thể tái sử dụng mà không sao chép nguyên văn",
	Schema: schema.Object(
		schema.Property("title", llmcontract.Nullable(schema.String("Tiêu đề (tuỳ chọn); null nếu không xác định được"))).Required(),
		schema.Property("summary", schema.String("Tóm lược giá trị về lối viết của văn bản mẫu")).Required(),
		schema.Property("style_observations", textList("Nhận xét về điểm nhìn trần thuật, cấu trúc câu và chất liệu miêu tả")).Required(),
		schema.Property("common_words", textList("Nhóm từ tần suất cao, hình ảnh và từ chuyển cảnh")).Required(),
		schema.Property("plot_patterns", textList("Mô thức đẩy tình tiết, bước ngoặt và leo thang xung đột")).Required(),
		schema.Property("hook_patterns", textList("Mô thức móc câu ở mở đầu, cuối chương và chênh lệch thông tin")).Required(),
		schema.Property("pacing_notes", textList("Mật độ cảnh và nhịp tiết lộ thông tin")).Required(),
		schema.Property("reader_appeal", textList("Cách giữ chân người đọc đọc tiếp")).Required(),
		schema.Property("reusable_techniques", textList("Kỹ thuật cấu trúc có thể học hỏi")).Required(),
		schema.Property("warnings", textList("Rủi ro sao chép và rập khuôn phải tránh")).Required(),
	),
}

var synthesisContract = llmcontract.Contract{
	Name:        "simulation_synthesis",
	Description: "Hợp nhất hồ sơ hiện có và các báo cáo văn bản mẫu thành hồ sơ phương pháp mô phỏng có thể áp dụng",
	Schema: schema.Object(
		schema.Property("style", schema.Object(
			schema.Property("narrative_voice", textList("Ngôi kể, khoảng cách và kiểm soát thông tin")).Required(),
			schema.Property("sentence_rhythm", textList("Nhịp câu")).Required(),
			schema.Property("prose_texture", textList("Chất liệu miêu tả")).Required(),
			schema.Property("perspective", textList("Quy tắc điểm nhìn")).Required(),
			schema.Property("mood", textList("Tông cảm xúc")).Required(),
			schema.Property("do_not_copy", textList("Nội dung cấm sao chép")).Required(),
		)).Required(),
		schema.Property("lexicon", schema.Object(
			schema.Property("common_words", textList("Nhóm từ thường dùng")).Required(),
			schema.Property("emotion_words", textList("Nhóm từ cảm xúc")).Required(),
			schema.Property("scene_words", textList("Nhóm từ bối cảnh")).Required(),
			schema.Property("transition_words", textList("Nhóm từ chuyển cảnh")).Required(),
			schema.Property("signature_phrases", textList("Đặc trưng giọng văn đã trừu tượng hoá, không chứa câu gốc")).Required(),
		)).Required(),
		schema.Property("plot_design", schema.Object(
			schema.Property("opening_patterns", textList("Cách mở đầu")).Required(),
			schema.Property("escalation_patterns", textList("Cách leo thang xung đột")).Required(),
			schema.Property("turning_point_patterns", textList("Thiết kế bước ngoặt")).Required(),
			schema.Property("payoff_patterns", textList("Cách thu hồi và trả thưởng")).Required(),
		)).Required(),
		schema.Property("hook_design", schema.Object(
			schema.Property("hook_types", textList("Loại móc câu")).Required(),
			schema.Property("placement", textList("Vị trí móc câu")).Required(),
			schema.Property("cliffhanger_patterns", textList("Cách ngắt ở điểm treo")).Required(),
			schema.Property("payoff_rules", textList("Quy tắc trả móc câu")).Required(),
		)).Required(),
		schema.Property("pacing_density", schema.Object(
			schema.Property("scene_density", textList("Mật độ thông tin mỗi cảnh")).Required(),
			schema.Property("information_release", textList("Nhịp tiết lộ thông tin")).Required(),
			schema.Property("dialogue_action_ratio", textList("Tỉ lệ thoại, hành động và nội tâm")).Required(),
			schema.Property("compression_rules", textList("Quy tắc triển khai và nén nội dung")).Required(),
		)).Required(),
		schema.Property("reader_engagement", schema.Object(
			schema.Property("methods", textList("Cách thu hút người đọc")).Required(),
			schema.Property("emotional_drivers", textList("Động lực cảm xúc")).Required(),
			schema.Property("progression_rewards", textList("Phần thưởng tiến triển theo giai đoạn")).Required(),
			schema.Property("anti_patterns", textList("Phản mẫu làm giảm sức hút")).Required(),
		)).Required(),
		schema.Property("role_guidance", schema.Object(
			schema.Property("architect", textList("Quy tắc kiến trúc sư (Architect) dùng hồ sơ")).Required(),
			schema.Property("writer", textList("Quy tắc người viết (Writer) học hỏi nhưng không sao chép")).Required(),
			schema.Property("editor", textList("Quy tắc biên tập (Editor) kiểm tra định hướng và rủi ro vi phạm bản quyền")).Required(),
		)).Required(),
	),
}
