//go:build unix

package check

import (
	"os"
	"syscall"
)

// fileID returns the device and inode of p (not following a final symlink);
// ok is false when p does not exist. A file with more than one name (a hard
// link) gets the zero id, which matches no other file: two names for one
// inode are not a rename. A directory always has more than one link ("."
// and its parent's entry) and keeps its id: a submodule renamed in case only
// is the same directory.
func fileID(p string) ([2]uint64, bool) {
	info, err := os.Lstat(p)
	if err != nil {
		return [2]uint64{}, false
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && (info.IsDir() || uint64(st.Nlink) <= 1) {
		return [2]uint64{uint64(st.Dev), uint64(st.Ino)}, true
	}
	return [2]uint64{}, true
}
