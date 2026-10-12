package tools

import (
	"fmt"
	"strings"

	"github.com/voocel/ainovel-cli/internal/chapterfacts"
	"github.com/voocel/ainovel-cli/internal/errs"
)

// validateCommitArgs 在创建 PendingCommit 前校验模型提交的完整语义载荷。
// 错误直接返回模型修正；不生成半成品状态，也不猜测缺失值。
// replay=true 表示重放已冻结的 PendingCommit：此时跳过 id 占位/占用检查——旧版本
// 冻结的载荷可能不满足这些后加的规则，而普通提交的冻结不会自动解除，拦下会让本书卡死。
func (t *CommitChapterTool) validateCommitArgs(a commitArgs, replay bool) error {
	if err := chapterfacts.Validate(a.ChapterFacts); err != nil {
		return fmt.Errorf("%v: %w", err, errs.ErrToolArgs)
	}

	if len(a.ForeshadowUpdates) > 0 {
		ledger, err := t.store.World.LoadForeshadowLedger()
		if err != nil {
			return fmt.Errorf("load foreshadow ledger: %w: %w", errs.ErrStoreRead, err)
		}
		// 账本是全书投影，而 Projector 按章序重放章节记录。重写早期章节时账本里
		// 还躺着后续章节才种下的伏笔——放行它们，提交前校验就与重放结论相反，
		// 模型无从修正，返工队列随之锁死。故一律以"本章可见"为准。
		plantedAt := make(map[string]int, len(ledger))
		for _, entry := range ledger {
			plantedAt[entry.ID] = entry.PlantedAt
		}
		declared := make(map[string]struct{}, len(a.ForeshadowUpdates))
		for i, update := range a.ForeshadowUpdates {
			switch update.Action {
			case "plant":
				if !replay && placeholderForeshadowID(update.ID) {
					return fmt.Errorf("foreshadow_updates[%d].id = %q là id giữ chỗ, không phải định danh. Mỗi phục bút cần một id duy nhất và ổn định "+
						"(ví dụ FORESHADOW_OBSERVER, jade-pendant) để các chương sau advance/resolve; dùng lại id giữ chỗ sẽ khiến phục bút mới đè lên mục cũ: %w",
						i, update.ID, errs.ErrToolArgs)
				}
				// 账本按 id 去重：别的章已种下同一 id 时，本次 plant 会被静默吞掉（描述一并
				// 丢失）。同章重放（返工/恢复）是幂等的，放行。
				if at := plantedAt[update.ID]; !replay && at != 0 && at != a.Chapter {
					return fmt.Errorf("foreshadow_updates[%d].id %q đã được chương %d dùng; phục bút mới cần id riêng chưa từng dùng, "+
						"muốn tiếp nối phục bút cũ thì dùng action advance/resolve: %w",
						i, update.ID, at, errs.ErrToolArgs)
				}
				declared[update.ID] = struct{}{}
			case "advance", "resolve":
				if _, ok := declared[update.ID]; ok {
					continue
				}
				at, known := plantedAt[update.ID]
				if !known {
					return fmt.Errorf("foreshadow_updates[%d] references unknown id %q: %w", i, update.ID, errs.ErrToolPrecondition)
				}
				if at > a.Chapter {
					return fmt.Errorf("foreshadow_updates[%d] phục bút %q được gieo ở chương %d, không thể đẩy tiến hoặc thu hồi ở chương %d: %w",
						i, update.ID, at, a.Chapter, errs.ErrToolPrecondition)
				}
			}
		}
	}
	return nil
}

// placeholderForeshadowIDs 是模型常写进 id 字段的占位词：它们没有标识作用，复用后
// 第二条起的新伏笔都会撞上第一条。仅在新提交时拦截，历史章节记录重放不受影响。
var placeholderForeshadowIDs = map[string]struct{}{
	"new": {}, "none": {}, "null": {}, "nil": {}, "tbd": {}, "todo": {},
	"auto": {}, "id": {}, "-": {}, "n/a": {}, "na": {}, "unknown": {}, "?": {},
}

func placeholderForeshadowID(id string) bool {
	_, ok := placeholderForeshadowIDs[strings.ToLower(strings.TrimSpace(id))]
	return ok
}
