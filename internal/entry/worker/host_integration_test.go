package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/ainovel-cli/internal/server/api"
)

// Chạy worker với host.Host thật trên thư mục truyện rỗng (resume → không có gì để
// khôi phục, không gọi LLM): kiểm khoá được nhả và API key không nằm trong bất kỳ
// file nào worker để lại (worker.log, meta…) lẫn stdout.
func TestRunWithRealHostDoesNotLeakKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Cache giá còn hạn trong HOME: host.New không phải gọi OpenRouter (test không cần mạng).
	cacheDir := filepath.Join(home, ".ainovel")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cache := fmt.Sprintf(`{"fetched_at":%q,"models":[]}`, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(cacheDir, "models-cache.json"), []byte(cache), 0o644); err != nil {
		t.Fatal(err)
	}
	bookDir := t.TempDir()
	t.Chdir(bookDir)

	eng := newFakeEngine() // chỉ để thoả chữ ký harness; engine thật được dùng bên dưới
	h := startHarness(t, eng, func(d *deps) {
		d.newEngine = newHostEngine
		d.prepare = prepareRuntime
		d.registry = models.NewModelRegistry()
		d.getwd = os.Getwd
		d.closeGrace = 10 * time.Second
	})
	init := testInit(bookDir, api.ActionResume)
	h.send(init)
	exit := h.expect(api.MsgExit, 15*time.Second)
	if exit.StopReason != api.StopNothingToResume {
		t.Fatalf("exit sai: %+v", exit)
	}
	if err := h.wait(15 * time.Second); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(bookDir, "logs", "worker.log")); err != nil {
		t.Fatalf("thiếu logs/worker.log: %v", err)
	}
	for _, root := range []string{bookDir, home} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(b), testAPIKey) {
				t.Errorf("API key xuất hiện trong %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Khoá thư mục đã nhả: chạy lại ngay phải không bị lock_busy.
	h2 := startHarness(t, newFakeEngine(), func(d *deps) {
		d.newEngine = newHostEngine
		d.prepare = prepareRuntime
		d.registry = models.NewModelRegistry()
		d.getwd = os.Getwd
	})
	h2.send(init)
	if exit := h2.expect(api.MsgExit, 15*time.Second); exit.StopReason != api.StopNothingToResume {
		t.Fatalf("lượt hai exit sai (khoá chưa nhả?): %+v", exit)
	}
	_ = h2.wait(15 * time.Second)
}
