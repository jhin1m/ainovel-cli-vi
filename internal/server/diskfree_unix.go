//go:build unix

package server

import "syscall"

// diskFree trả về số byte trống cho user thường trên filesystem chứa dir.
func diskFree(dir string) (uint64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return uint64(st.Bavail) * uint64(st.Bsize), true
}
