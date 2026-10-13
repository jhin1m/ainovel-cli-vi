//go:build linux

package worker

import "os"

// lowerOOMPriority đánh dấu worker là ứng viên bị OOM killer giết trước server.
func lowerOOMPriority() {
	_ = os.WriteFile("/proc/self/oom_score_adj", []byte("500"), 0o644)
}
