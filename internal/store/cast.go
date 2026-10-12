package store

import (
	"fmt"

	"github.com/voocel/ainovel-cli/internal/domain"
)

// BuildCast 从接纳记录构建当前配角视图。
func (s *Store) BuildCast(chapters []int) ([]domain.CastEntry, error) {
	records, err := s.ChapterRecords.LoadCompleted(chapters)
	if err != nil {
		return nil, fmt.Errorf("đọc bản ghi chương: %w", err)
	}
	characters, err := s.Characters.Load()
	if err != nil {
		return nil, fmt.Errorf("đọc nhân vật cốt lõi: %w", err)
	}
	return domain.ProjectCast(records, characters), nil
}
