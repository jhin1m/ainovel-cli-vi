//go:build !unix

package jobs

import "os/exec"

const defaultWorkerBin = ""

func setProcAttr(*exec.Cmd) {}

// signalGroup: ngoài unix không có nhóm process, chỉ giết được process chính.
func signalGroup(cmd *exec.Cmd, _ bool) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
