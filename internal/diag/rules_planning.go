package diag

import (
	"fmt"
	"strings"

	"github.com/voocel/ainovel-cli/internal/domain"
)

// StaleForeshadow 检测长期未推进的伏笔。
func StaleForeshadow(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Foreshadow) == 0 {
		return nil
	}
	latest := snap.LatestCompleted()
	threshold := staleForeshadowThreshold(snap.CompletedCount())

	var stale []string
	for _, f := range snap.Foreshadow {
		if f.Status != "planted" {
			continue
		}
		gap := latest - f.PlantedAt
		if gap > threshold {
			stale = append(stale, fmt.Sprintf("%s (cài ở ch%d, đã qua %d chương)", f.ID, f.PlantedAt, gap))
		}
	}
	if len(stale) == 0 {
		return nil
	}
	return []Finding{{
		Rule:       "StaleForeshadow",
		Category:   CatPlanning,
		Severity:   SevWarning,
		Confidence: ConfMedium,
		AutoLevel:  AutoNone,
		Target:     "context.foreshadow",
		Title:      fmt.Sprintf("Phục bút đứng yên: %d phục bút quá %d chương chưa được đẩy tiến", len(stale), threshold),
		Evidence:   strings.Join(stale, "; "),
		Suggestion: "Việc nạp nhắc nhở phục bút của novel_context có thể chưa hoạt động, hoặc prompt của Writer thiếu hướng dẫn đẩy tiến phục bút. Kiểm tra foreshadow_ledger và logic chèn ngữ cảnh.",
	}}
}

// CompassDrift 检测指南针长期未更新。
func CompassDrift(snap *Snapshot) []Finding {
	if snap.Progress == nil || !snap.Progress.Layered {
		return nil
	}
	if snap.Compass == nil {
		if snap.CompletedCount() > 5 {
			return []Finding{{
				Rule:       "CompassDrift",
				Category:   CatPlanning,
				Severity:   SevWarning,
				Confidence: ConfMedium,
				AutoLevel:  AutoNone,
				Target:     "prompt.architect",
				Title:      "Chế độ truyện dài thiếu la bàn (compass)",
				Evidence:   fmt.Sprintf("layered=true, completed=%d, compass=nil", snap.CompletedCount()),
				Suggestion: "Architect phải tạo compass khi lên kế hoạch ban đầu. Kiểm tra architect-long.md có chỉ thị tạo compass không.",
			}}
		}
		return nil
	}

	gap := snap.LatestCompleted() - snap.Compass.LastUpdated
	if gap <= ThresholdCompassDrift {
		return nil
	}
	return []Finding{{
		Rule:       "CompassDrift",
		Category:   CatPlanning,
		Severity:   SevInfo,
		Confidence: ConfLow,
		AutoLevel:  AutoNone,
		Target:     "prompt.architect",
		Title:      fmt.Sprintf("La bàn (compass) đã %d chương chưa cập nhật", gap),
		Evidence:   fmt.Sprintf("last_updated=ch%d, latest=ch%d, open_threads=%d", snap.Compass.LastUpdated, snap.LatestCompleted(), len(snap.Compass.OpenThreads)),
		Suggestion: "Architect phải cập nhật compass ở ranh giới cung/quyển. Kiểm tra architect-long.md có chỉ thị cập nhật compass không.",
	}}
}

// OutlineExhausted 检测大纲耗尽但小说未完结。
func OutlineExhausted(snap *Snapshot) []Finding {
	if snap.Progress == nil {
		return nil
	}
	p := snap.Progress
	if p.Phase == domain.PhaseComplete || p.Phase == domain.PhaseInit {
		return nil
	}

	completed := snap.CompletedCount()
	if completed == 0 {
		return nil
	}

	outlinedCount := p.TotalChapters
	if outlinedCount <= 0 {
		outlinedCount = len(snap.Outline)
	}
	if outlinedCount <= 0 {
		return nil
	}

	if completed < outlinedCount {
		return nil
	}

	return []Finding{{
		Rule:       "OutlineExhausted",
		Category:   CatPlanning,
		Severity:   SevCritical,
		Confidence: ConfHigh,
		AutoLevel:  AutoSafe,
		Target:     "runtime.recovery",
		Title:      fmt.Sprintf("Dàn ý đã cạn: đã hoàn thành %d chương >= đã lên dàn ý %d chương", completed, outlinedCount),
		Evidence:   fmt.Sprintf("phase=%s, completed=%d, outlined=%d", p.Phase, completed, outlinedCount),
		Suggestion: "Tín hiệu triển khai cung/thêm quyển có thể chưa được kích hoạt. Kiểm tra chiến lược commit và logic khôi phục phía host, xác nhận việc phát hiện ranh giới cung, expand_next_arc hoặc append_volume có chạy bình thường không.",
	}}
}

// MissingSummaries 检测已完成章节缺少摘要。
func MissingSummaries(snap *Snapshot) []Finding {
	if snap.Progress == nil || len(snap.Progress.CompletedChapters) == 0 {
		return nil
	}

	var missing []int
	for _, ch := range snap.Progress.CompletedChapters {
		if _, ok := snap.Summaries[ch]; !ok {
			missing = append(missing, ch)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []Finding{{
		Rule:       "MissingSummaries",
		Category:   CatPlanning,
		Severity:   SevWarning,
		Confidence: ConfHigh,
		AutoLevel:  AutoNone,
		Target:     "runtime.flow",
		Title:      fmt.Sprintf("Thiếu tóm tắt: %d chương chưa có tóm tắt", len(missing)),
		Evidence:   fmt.Sprintf("missing=[%s]", intsToStr(missing)),
		Suggestion: "Tóm tắt là chìa khoá cho tính liên tục của ngữ cảnh. Kiểm tra logic ghi tóm tắt của commit_chapter có hoạt động bình thường không.",
	}}
}
