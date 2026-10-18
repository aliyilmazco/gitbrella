// Package treecopy copies a git directory with everything git relies on
// (modes, times, symlinks) and proves the copy is identical.
package treecopy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Clone copies the directory src to dst, which must not exist. On macOS it
// makes an APFS clone (cp -c -pR: no data is duplicated) and falls back to a
// plain copy where cloning is not possible; on Linux it uses a reflink where
// the filesystem has them (cp -a --reflink=auto). Both keep modes, times,
// symlinks and, on macOS, file flags.
func Clone(ctx context.Context, src, dst, goos string) error {
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("copy destination %s already exists", dst)
	}
	tries := [][]string{{"-a", "--reflink=auto", src, dst}}
	if goos == "darwin" {
		tries = [][]string{{"-c", "-pR", src, dst}, {"-pR", src, dst}}
	}
	var errs []string
	for _, args := range tries {
		out, err := exec.CommandContext(ctx, "/bin/cp", args...).CombinedOutput()
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Sprintf("cp %s: %v: %s", strings.Join(args[:len(args)-2], " "), err, strings.TrimSpace(string(out))))
		// Our own partial copy, never anything that was there before.
		if rmErr := os.RemoveAll(dst); rmErr != nil {
			return fmt.Errorf("%s; removing the partial copy: %v", strings.Join(errs, "; "), rmErr)
		}
	}
	return fmt.Errorf("copying %s: %s", src, strings.Join(errs, "; "))
}

// entry is what Compare checks for one path.
type entry struct {
	mode    fs.FileMode
	size    int64
	modNano int64
	link    string
}

// Compare lists how the directory b differs from the directory a (both must
// be directories): entries missing or extra, and for each
// path its kind, permissions, size, modification time (files and symlinks),
// symlink target and content. It reads every file; at most 20 differences
// are listed.
func Compare(a, b string) ([]string, error) {
	for _, root := range []string{a, b} {
		if info, err := os.Lstat(root); err != nil {
			return nil, err
		} else if !info.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", root)
		}
	}
	ea, err := walk(a)
	if err != nil {
		return nil, err
	}
	eb, err := walk(b)
	if err != nil {
		return nil, err
	}
	var diffs []string
	add := func(format string, args ...any) bool {
		diffs = append(diffs, fmt.Sprintf(format, args...))
		return len(diffs) < 20
	}
	paths := make([]string, 0, len(ea))
	for p := range ea {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		x, y := ea[p], eb[p]
		switch {
		case y == nil:
			if !add("%s: missing from the copy", p) {
				return diffs, nil
			}
			continue
		case x.mode != y.mode:
			if !add("%s: mode %v, copy %v", p, x.mode, y.mode) {
				return diffs, nil
			}
			continue
		case x.mode.IsDir():
			continue
		case x.link != y.link:
			if !add("%s: links to %q, copy to %q", p, x.link, y.link) {
				return diffs, nil
			}
			continue
		case x.size != y.size || x.modNano != y.modNano:
			if !add("%s: size or time differs", p) {
				return diffs, nil
			}
			continue
		}
		if x.mode.IsRegular() {
			same, err := sameContent(filepath.Join(a, p), filepath.Join(b, p))
			if err != nil {
				return nil, err
			}
			if !same && !add("%s: content differs", p) {
				return diffs, nil
			}
		}
	}
	extra := make([]string, 0)
	for p := range eb {
		if ea[p] == nil {
			extra = append(extra, p)
		}
	}
	sort.Strings(extra)
	for _, p := range extra {
		if !add("%s: only in the copy", p) {
			break
		}
	}
	return diffs, nil
}

// Size returns the bytes of the regular files under dir.
func Size(dir string) (int64, error) {
	var n int64
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			n += info.Size()
		}
		return nil
	})
	return n, err
}

func walk(root string) (map[string]*entry, error) {
	out := map[string]*entry{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		e := &entry{mode: info.Mode(), size: info.Size(), modNano: info.ModTime().UnixNano()}
		if info.Mode()&fs.ModeSymlink != 0 {
			if e.link, err = os.Readlink(p); err != nil {
				return err
			}
		}
		if info.IsDir() {
			e.size, e.modNano = 0, 0 // a directory's time changes as entries are made
		}
		out[filepath.ToSlash(rel)] = e
		return nil
	})
	return out, err
}

func sameContent(a, b string) (bool, error) {
	ha, err := hashFile(a)
	if err != nil {
		return false, err
	}
	hb, err := hashFile(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

func hashFile(p string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	f, err := os.Open(p)
	if err != nil {
		return sum, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, fmt.Errorf("reading %s: %w", p, err)
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}
