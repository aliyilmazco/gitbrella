//go:build !unix

package check

import "os"

// fileID reports whether p exists. Without inodes the id is zero, which
// matches no other file (renames are not recognised).
func fileID(p string) ([2]uint64, bool) {
	if _, err := os.Lstat(p); err != nil {
		return [2]uint64{}, false
	}
	return [2]uint64{}, true
}
