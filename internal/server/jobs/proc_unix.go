//go:build unix && !linux

package jobs

import (
	"os/exec"
	"syscall"
)

// defaultWorkerBin rỗng: dùng os.Executable() (không có /proc/self/exe).
const defaultWorkerBin = ""

// setProcAttr cho worker nhóm process riêng để giết cả nhóm.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalGroup(cmd *exec.Cmd, hard bool) {
	if cmd.Process == nil {
		return
	}
	sig := syscall.SIGTERM
	if hard {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}
