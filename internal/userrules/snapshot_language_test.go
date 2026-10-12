package userrules

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/voocel/ainovel-cli/internal/lang"
	"github.com/voocel/ainovel-cli/internal/rules"
	"github.com/voocel/ainovel-cli/internal/store"
)

func setLang(t *testing.T, c lang.Code) {
	t.Helper()
	prev := lang.Current()
	lang.Set(c)
	t.Cleanup(func() { lang.Set(prev) })
}

// writeLegacySnapshot ghi một snapshot dạng cũ (không có trường language), như được tạo
// khi mặc định còn là bảng từ cấm tiếng Trung.
func writeLegacySnapshot(t *testing.T, st *store.Store, snap rules.Snapshot) {
	t.Helper()
	if err := st.UserRules.Save(&snap); err != nil {
		t.Fatal(err)
	}
}

func readLanguageField(t *testing.T, st *store.Store) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(st.Dir(), snapshotRelPath))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	return meta.Language
}

func legacyCandidate() rules.Candidate {
	return rules.Candidate{
		Source: systemDefaultsSource,
		Structured: rules.Structured{
			ForbiddenPhrases: slices.Clone(legacyZHDefaults().ForbiddenPhrases),
			FatigueWords:     maps.Clone(legacyZHDefaults().FatigueWords),
		},
	}
}

func TestGetOrBuild_RelocalizesLegacyZHDefaults(t *testing.T) {
	setLang(t, lang.VI)
	svc, st := newDegradedService(t)

	user := rules.Candidate{
		Source:     "startup_prompt",
		Structured: rules.Structured{FatigueWords: map[string]int{"bỗng nhiên": 1, "仿佛": 5}},
	}
	writeLegacySnapshot(t, st, rules.BuildSnapshot([]rules.Candidate{legacyCandidate(), user}))

	snap, err := svc.GetOrBuild(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cur := rules.SystemDefaults().Structured
	if !slices.Equal(snap.Structured.ForbiddenPhrases, cur.ForbiddenPhrases) {
		t.Errorf("forbidden_phrases phải theo mặc định hiện hành, got %v want %v", snap.Structured.ForbiddenPhrases, cur.ForbiddenPhrases)
	}
	want := maps.Clone(cur.FatigueWords)
	if want == nil {
		want = map[string]int{}
	}
	want["bỗng nhiên"] = 1
	want["仿佛"] = 5 // ngưỡng khác bảng cũ = người dùng ghi đè → giữ
	for word, limit := range legacyZHDefaults().FatigueWords {
		if _, isCurrent := cur.FatigueWords[word]; isCurrent || word == "仿佛" {
			continue
		}
		if got, ok := snap.Structured.FatigueWords[word]; ok {
			t.Errorf("mặc định zh cũ %q=%d phải bị gỡ, còn %d", word, limit, got)
		}
	}
	for word, limit := range want {
		if snap.Structured.FatigueWords[word] != limit {
			t.Errorf("fatigue %q: got %d want %d", word, snap.Structured.FatigueWords[word], limit)
		}
	}
	if got := readLanguageField(t, st); got != string(lang.VI) {
		t.Errorf("snapshot phải ghi language=vi, got %q", got)
	}
	reloaded, _ := st.UserRules.Load()
	if !maps.Equal(reloaded.Structured.FatigueWords, snap.Structured.FatigueWords) {
		t.Errorf("snapshot dựng lại phải được lưu: %+v", reloaded.Structured.FatigueWords)
	}
}

func TestGetOrBuild_KeepsUserForbiddenPhrases(t *testing.T) {
	setLang(t, lang.VI)
	svc, st := newDegradedService(t)
	user := rules.Candidate{
		Source:     "startup_prompt",
		Structured: rules.Structured{ForbiddenPhrases: []string{"bất giác"}},
	}
	writeLegacySnapshot(t, st, rules.BuildSnapshot([]rules.Candidate{legacyCandidate(), user}))

	snap, err := svc.GetOrBuild(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(snap.Structured.ForbiddenPhrases, []string{"bất giác"}) {
		t.Errorf("forbidden_phrases do người dùng đặt phải giữ, got %v", snap.Structured.ForbiddenPhrases)
	}
}

func TestGetOrBuild_SameLanguageUntouched(t *testing.T) {
	setLang(t, lang.ZH)
	svc, st := newDegradedService(t)
	writeLegacySnapshot(t, st, rules.BuildSnapshot([]rules.Candidate{legacyCandidate()}))

	snap, err := svc.GetOrBuild(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(snap.Structured.FatigueWords, legacyZHDefaults().FatigueWords) {
		t.Errorf("cùng ngôn ngữ không được đổi snapshot: %+v", snap.Structured.FatigueWords)
	}
	if got := readLanguageField(t, st); got != "" {
		t.Errorf("không cần dựng lại thì không ghi lại file, got language=%q", got)
	}
}

func TestGetOrBuild_NoSystemDefaultsSourceUntouched(t *testing.T) {
	setLang(t, lang.VI)
	svc, st := newDegradedService(t)
	user := rules.Candidate{
		Source:     "startup_prompt",
		Structured: rules.Structured{FatigueWords: map[string]int{"仿佛": 2}},
	}
	writeLegacySnapshot(t, st, rules.BuildSnapshot([]rules.Candidate{user}))

	snap, err := svc.GetOrBuild(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(snap.Structured.FatigueWords, map[string]int{"仿佛": 2}) {
		t.Errorf("không có system_defaults thì giữ nguyên: %+v", snap.Structured.FatigueWords)
	}
}

func TestBuildRecordsLanguage(t *testing.T) {
	setLang(t, lang.VI)
	svc, st := newDegradedService(t)
	if _, err := svc.Build(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if got := readLanguageField(t, st); got != string(lang.VI) {
		t.Errorf("Build phải ghi language, got %q", got)
	}
	// Đọc lại cùng ngôn ngữ: không dựng lại.
	if _, err := svc.GetOrBuild(t.Context()); err != nil {
		t.Fatal(err)
	}
}
