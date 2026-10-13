//go:build !unix

package server

// diskFree chưa hỗ trợ ngoài unix; /healthz bỏ qua field dung lượng.
func diskFree(string) (uint64, bool) { return 0, false }
