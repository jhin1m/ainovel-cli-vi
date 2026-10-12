package diag

import "fmt"

// PlanActions 根据高置信 Finding 生成可执行动作。
// 只有 Confidence==high && AutoLevel==safe 的 Finding 才会产出 Action。
func PlanActions(findings []Finding) []Action {
	var actions []Action
	seen := make(map[string]struct{})

	for _, f := range findings {
		if f.Confidence != ConfHigh || f.AutoLevel != AutoSafe {
			continue
		}
		if _, ok := seen[f.Rule]; ok {
			continue
		}
		seen[f.Rule] = struct{}{}

		actions = append(actions, planRule(f)...)
	}
	return actions
}

func planRule(f Finding) []Action {
	key := findingFingerprint(f)

	switch f.Rule {
	case "PhaseFlowMismatch":
		return []Action{
			{SourceRule: f.Rule, Kind: ActionEmitNotice, Severity: f.Severity, Summary: f.Title, Message: f.Title, Fingerprint: key},
			{SourceRule: f.Rule, Kind: ActionEnqueueFollowUp, Severity: f.Severity, Summary: "Sửa lỗi máy trạng thái", Message: "Máy trạng thái bất thường: " + f.Evidence + ". Hãy kiểm tra và sửa trạng thái phase/flow của progress trước, rồi mới chạy tiếp.", Fingerprint: key},
		}
	case "OutlineExhausted":
		return []Action{
			{SourceRule: f.Rule, Kind: ActionEnqueueFollowUp, Severity: f.Severity, Summary: "Xử lý dàn ý đã cạn", Message: "Số chương đã hoàn thành đã chạm giới hạn đã lên dàn ý. Hãy ưu tiên gọi Architect triển khai cung truyện kế tiếp hoặc thêm quyển mới, rồi mới viết tiếp.", Fingerprint: key},
		}
	case "OrphanedSteer":
		return []Action{
			{SourceRule: f.Rule, Kind: ActionEnqueueFollowUp, Severity: f.Severity, Summary: "Xử lý can thiệp người dùng còn tồn", Message: "Còn chỉ thị can thiệp của người dùng chưa được xử lý; hãy ưu tiên xử lý pending steer rồi mới tiếp tục nhiệm vụ hiện tại.", Fingerprint: key},
		}
	default:
		return nil
	}
}

func findingFingerprint(f Finding) string {
	return fmt.Sprintf("%s|%s|%s|%s", f.Rule, f.Target, f.Title, f.Evidence)
}
