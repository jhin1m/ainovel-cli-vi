// Package books đọc (chỉ đọc) thư mục truyện do engine ghi và xuất file tải về.
package books

import (
	"errors"
	"fmt"
	"sort"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/store"
)

// ErrChapterNotFound: chương không tồn tại hoặc chưa hoàn thành.
var ErrChapterNotFound = errors.New("không tìm thấy chương")

// readerCompleted trả về danh sách chương đã chốt (tăng dần, đã khử trùng) và progress.
// progress nil (chưa có file) → danh sách rỗng.
func readerCompleted(st *store.Store) ([]int, *domain.Progress, error) {
	p, err := st.Progress.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("đọc tiến độ truyện: %w", err)
	}
	if p == nil {
		return nil, nil, nil
	}
	seen := make(map[int]struct{}, len(p.CompletedChapters))
	nums := make([]int, 0, len(p.CompletedChapters))
	for _, n := range p.CompletedChapters {
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		nums = append(nums, n)
	}
	sort.Ints(nums)
	return nums, p, nil
}

// readerOutlineEntries trả về dàn ý phẳng (hoặc dàn ý phân tầng) cùng danh sách quyển;
// volumes rỗng nghĩa là truyện không phân tầng.
func readerOutlineEntries(st *store.Store) (flat []domain.OutlineEntry, volumes []domain.VolumeOutline, err error) {
	volumes, err = st.Outline.LoadLayeredOutline()
	if err != nil {
		return nil, nil, fmt.Errorf("đọc dàn ý phân tầng: %w", err)
	}
	hasChapters := false
	for _, v := range volumes {
		for _, a := range v.Arcs {
			if len(a.Chapters) > 0 {
				hasChapters = true
			}
		}
	}
	if hasChapters {
		for _, v := range volumes {
			for _, a := range v.Arcs {
				flat = append(flat, a.Chapters...)
			}
		}
		return flat, volumes, nil
	}
	flat, err = st.Outline.LoadOutline()
	if err != nil {
		return nil, nil, fmt.Errorf("đọc dàn ý: %w", err)
	}
	return flat, nil, nil
}

func readerTitle(st *store.Store, titles map[int]string, n int) string {
	if t, err := st.Summaries.LoadSummaryTitle(n); err == nil && t != "" {
		return t
	}
	return titles[n]
}

func readerTitleMap(entries []domain.OutlineEntry) map[int]string {
	m := make(map[int]string, len(entries))
	for _, e := range entries {
		m[e.Chapter] = e.Title
	}
	return m
}

// ListChapters liệt kê các chương đã chốt theo thứ tự tăng dần.
func ListChapters(dir string) ([]api.ChapterInfo, error) {
	st := store.NewStore(dir)
	nums, p, err := readerCompleted(st)
	if err != nil {
		return nil, err
	}
	out := make([]api.ChapterInfo, 0, len(nums))
	if len(nums) == 0 {
		return out, nil
	}
	entries, _, err := readerOutlineEntries(st)
	if err != nil {
		return nil, err
	}
	titles := readerTitleMap(entries)
	for _, n := range nums {
		out = append(out, api.ChapterInfo{
			Number:    n,
			Title:     readerTitle(st, titles, n),
			WordCount: p.ChapterWordCounts[n],
		})
	}
	return out, nil
}

// ReadChapter đọc nội dung một chương đã chốt; chương chưa hoàn thành trả ErrChapterNotFound.
func ReadChapter(dir string, n int) (api.ChapterText, error) {
	st := store.NewStore(dir)
	nums, _, err := readerCompleted(st)
	if err != nil {
		return api.ChapterText{}, err
	}
	idx := sort.SearchInts(nums, n)
	if idx >= len(nums) || nums[idx] != n {
		return api.ChapterText{}, ErrChapterNotFound
	}
	content, err := st.Drafts.LoadChapterText(n)
	if err != nil {
		return api.ChapterText{}, fmt.Errorf("đọc chương %d: %w", n, err)
	}
	entries, _, err := readerOutlineEntries(st)
	if err != nil {
		return api.ChapterText{}, err
	}
	res := api.ChapterText{
		Number:  n,
		Title:   readerTitle(st, readerTitleMap(entries), n),
		Content: content,
	}
	if idx > 0 {
		res.Prev = nums[idx-1]
	}
	if idx+1 < len(nums) {
		res.Next = nums[idx+1]
	}
	return res, nil
}

// ReadOutline đọc dàn ý, nhân vật và thông tin truyện. File thiếu → giá trị rỗng.
func ReadOutline(dir string) (api.Outline, error) {
	st := store.NewStore(dir)
	res := api.Outline{
		Volumes:    []api.OutlineVolume{},
		Characters: []api.OutlineCharacter{},
	}
	book, err := st.Book.Load()
	if err != nil {
		return res, fmt.Errorf("đọc thông tin truyện: %w", err)
	}
	if book != nil {
		res.Title, res.Synopsis = book.Title, book.Synopsis
	}
	nums, _, err := readerCompleted(st)
	if err != nil {
		return res, err
	}
	done := make(map[int]bool, len(nums))
	for _, n := range nums {
		done[n] = true
	}
	toChapters := func(entries []domain.OutlineEntry) []api.OutlineChapter {
		chs := make([]api.OutlineChapter, 0, len(entries))
		for _, e := range entries {
			chs = append(chs, api.OutlineChapter{Number: e.Chapter, Title: e.Title, CoreEvent: e.CoreEvent, Done: done[e.Chapter]})
		}
		return chs
	}
	flat, volumes, err := readerOutlineEntries(st)
	if err != nil {
		return res, err
	}
	if len(volumes) > 0 {
		for i, v := range volumes {
			var entries []domain.OutlineEntry
			for _, a := range v.Arcs {
				entries = append(entries, a.Chapters...)
			}
			idx := v.Index
			if idx <= 0 {
				idx = i + 1
			}
			title := fmt.Sprintf("Quyển %d", idx)
			if v.Title != "" {
				title += ": " + v.Title
			}
			res.Volumes = append(res.Volumes, api.OutlineVolume{Title: title, Chapters: toChapters(entries)})
		}
	} else if len(flat) > 0 {
		res.Volumes = append(res.Volumes, api.OutlineVolume{Chapters: toChapters(flat)})
	}
	chars, err := st.Characters.Load()
	if err != nil {
		return res, fmt.Errorf("đọc nhân vật: %w", err)
	}
	for _, c := range chars {
		res.Characters = append(res.Characters, api.OutlineCharacter{Name: c.Name, Role: c.Role, Description: c.Description})
	}
	return res, nil
}
