//go:build !linux

package worker

// lowerOOMPriority chỉ có tác dụng trên Linux.
func lowerOOMPriority() {}
