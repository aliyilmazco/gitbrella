//go:build unix

package state

import (
	"fmt"
	"syscall"
)

// FileID names the file or directory at p by device and inode ("dev:ino"),
// the same whatever spelling, case or Unicode form reaches it, and kept by a
// rename. "" when p does not exist.
func FileID(p string) string {
	var st syscall.Stat_t
	if err := syscall.Lstat(p, &st); err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", uint64(st.Dev), uint64(st.Ino))
}
