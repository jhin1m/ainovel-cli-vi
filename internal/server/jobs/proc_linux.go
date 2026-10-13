//go:build linux

package jobs

import (
	"os/exec"
	"syscall"
)

// defaultWorkerBin: /proc/self/exe luôn trỏ tới đúng inode đang chạy, kể cả khi
// binary trên đĩa đã bị thay giữa chừng (nâng cấp) — worker luôn khớp phiên bản.
const defaultWorkerBin = "/proc/self/exe"

// setProcAttr cho worker nhóm process riêng (để giết cả nhóm) và tự chết khi
// server chết (Pdeathsig).
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
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
