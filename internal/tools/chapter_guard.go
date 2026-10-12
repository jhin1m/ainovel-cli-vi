package tools

import (
	"fmt"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/errs"
	"github.com/voocel/ainovel-cli/internal/store"
)

// EnsureChapterExpanded verifies that chapter work is in the writing phase and,
// for layered books, inside the currently expanded outline.
func EnsureChapterExpanded(st *store.Store, chapter int) error {
	if st == nil {
		return fmt.Errorf("store không được rỗng: %w", errs.ErrToolPrecondition)
	}
	if chapter <= 0 {
		return fmt.Errorf("chapter must be > 0: %w", errs.ErrToolArgs)
	}
	progress, err := st.Progress.Load()
	if err != nil {
		return fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	if progress == nil {
		return fmt.Errorf("progress chưa được khởi tạo: %w", errs.ErrToolPrecondition)
	}
	if progress.Phase != domain.PhaseWriting {
		return fmt.Errorf("chỉ được viết chương trong giai đoạn writing (phase hiện tại=%s): %w", progress.Phase, errs.ErrToolPrecondition)
	}
	if !progress.Layered {
		return nil
	}
	boundary, err := st.Outline.CheckArcBoundary(chapter)
	if err != nil {
		return fmt.Errorf("check layered outline: %w: %w", errs.ErrStoreRead, err)
	}
	if boundary != nil {
		return nil
	}
	return fmt.Errorf(
		"chương %d nằm ngoài phạm vi dàn ý phân tầng: muốn viết phải expand_next_arc để triển khai cung hoặc append_volume để thêm quyển trước; nếu toàn truyện đã kết thúc thì gọi save_foundation type=complete_book: %w",
		chapter, errs.ErrToolPrecondition)
}
