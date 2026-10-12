package userrules

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/voocel/ainovel-cli/internal/lang"
	"github.com/voocel/ainovel-cli/internal/rules"
)

// snapshotRelPath là vị trí snapshot quy tắc của cuốn sách, trùng với store.UserRulesStore.
const snapshotRelPath = "meta/user_rules.json"

// systemDefaultsSource là nhãn nguồn của rules.SystemDefaults() trong Snapshot.Sources.
const systemDefaultsSource = "system_defaults"

// snapshotFile là dạng ghi đĩa của snapshot: rules.Snapshot cộng thêm ngôn ngữ của
// bảng mặc định đã nhúng vào. rules.Snapshot không có trường này nên ghi kèm ở đây;
// người đọc khác (store.UserRulesStore.Load) bỏ qua trường lạ khi giải mã JSON.
type snapshotFile struct {
	rules.Snapshot
	Language lang.Code `json:"language,omitempty"`
}

// snapshotPath trả về đường dẫn tuyệt đối tới snapshot của cuốn sách.
func (s *Service) snapshotPath() string {
	return filepath.Join(s.store.Dir(), snapshotRelPath)
}

// loadSnapshotLanguage đọc ngôn ngữ đã ghi trong snapshot. Snapshot thiếu trường
// language được tạo trước khi có cơ chế này, lúc mặc định còn là tiếng Trung → coi là zh.
func (s *Service) loadSnapshotLanguage() (lang.Code, error) {
	data, err := os.ReadFile(s.snapshotPath())
	if err != nil {
		return "", err
	}
	var meta struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", err
	}
	if meta.Language == "" {
		return lang.ZH, nil
	}
	code, err := lang.Parse(meta.Language)
	if err != nil {
		return lang.ZH, nil
	}
	return code, nil
}

// saveSnapshot ghi snapshot kèm ngôn ngữ hiện hành, ghi nguyên tử (file tạm + rename)
// giống store để người đọc đồng thời không thấy file ghi dở.
func (s *Service) saveSnapshot(snap *rules.Snapshot) error {
	data, err := json.MarshalIndent(snapshotFile{Snapshot: *snap, Language: lang.Current()}, "", "  ")
	if err != nil {
		return err
	}
	p := s.snapshotPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), filepath.Base(p)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

// relocalizeDefaults dựng lại phần system_defaults của snapshot khi ngôn ngữ hiện
// hành khác ngôn ngữ lúc tạo snapshot. Trả về true nếu snapshot đã được sửa.
//
// Snapshot là kết quả đã hợp nhất, nên không tách được chính xác phần nào do mặc định
// đóng góp; quy tắc gỡ:
//   - forbidden_phrases: nguồn cao hơn ghi đè cả danh sách, nên chỉ khi danh sách trùng
//     đúng bảng mặc định cũ mới thay bằng bảng mặc định hiện tại; người dùng tự đặt thì giữ.
//   - fatigue_words: hợp nhất theo từ; gỡ các từ trùng cả từ lẫn ngưỡng với bảng cũ
//     (ngưỡng khác nghĩa là người dùng đã ghi đè → giữ), rồi bổ sung bảng mặc định hiện
//     tại ở mức ưu tiên thấp nhất (không đè từ người dùng đã khai).
//
// Chỉ biết bảng mặc định cũ của tiếng Trung (rules.SystemDefaultsZH). Chuyển từ vi sang zh thì
// không gỡ được bảng vi cũ, chỉ bổ sung bảng hiện tại — từ tiếng Việt không bao giờ khớp
// văn tiếng Trung nên vô hại.
func relocalizeDefaults(snap *rules.Snapshot, from lang.Code) bool {
	if from == lang.Current() || !slices.Contains(snap.Sources, systemDefaultsSource) {
		return false
	}
	cur := rules.SystemDefaults().Structured
	st := &snap.Structured

	if from == lang.ZH && slices.Equal(st.ForbiddenPhrases, legacyZHDefaults().ForbiddenPhrases) {
		st.ForbiddenPhrases = slices.Clone(cur.ForbiddenPhrases)
	}

	fatigue := make(map[string]int, len(st.FatigueWords)+len(cur.FatigueWords))
	maps.Copy(fatigue, cur.FatigueWords)
	for word, limit := range st.FatigueWords {
		if from == lang.ZH {
			if old, ok := legacyZHDefaults().FatigueWords[word]; ok && old == limit {
				continue
			}
		}
		fatigue[word] = limit
	}
	if len(fatigue) == 0 {
		fatigue = nil
	}
	st.FatigueWords = fatigue
	return true
}

// isNotExist gói os.IsNotExist cho lỗi đã bọc.
func isNotExist(err error) bool { return errors.Is(err, os.ErrNotExist) }

// legacyZHDefaults là bảng mặc định tiếng Trung mà snapshot cũ (tạo khi language=zh) đã nhúng.
func legacyZHDefaults() rules.Structured { return rules.SystemDefaultsZH() }
