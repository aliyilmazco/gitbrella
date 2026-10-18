// Package gitfile writes and reads the .git file that points a working tree
// at a git directory elsewhere ("gitdir: <path>").
package gitfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Content is the .git file pointing worktree at gitDir. The path is relative
// when both are under home: devices that keep the folder and the git
// directory at the same places under their homes then write the same bytes,
// and the synced .git file never flips between them. Otherwise it is
// absolute.
func Content(worktree, gitDir, home string) string {
	if under(worktree, home) && under(gitDir, home) {
		if rel, err := filepath.Rel(worktree, gitDir); err == nil {
			return "gitdir: " + filepath.ToSlash(rel) + "\n"
		}
	}
	return "gitdir: " + gitDir + "\n"
}

// Read reads worktree/.git. isFile is false when it is a directory; target is
// the git directory a .git file names, resolved against worktree. A missing
// .git is an error wrapping fs.ErrNotExist.
func Read(worktree string) (target string, isFile bool, err error) {
	p := filepath.Join(worktree, ".git")
	info, err := os.Lstat(p)
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return "", false, nil
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("%s is neither a directory nor a file", p)
	}
	f, err := os.Open(p)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return "", false, err
	}
	line := strings.TrimRight(strings.SplitN(string(b), "\n", 2)[0], "\r")
	target, ok := strings.CutPrefix(line, "gitdir: ")
	if !ok || target == "" {
		return "", false, fmt.Errorf("%s is not a .git file (no \"gitdir: \" line)", p)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(worktree, filepath.FromSlash(target))
	}
	return filepath.Clean(target), true, nil
}

// Write creates worktree/.git holding content. It never replaces anything:
// an existing .git (file or directory) is an error wrapping fs.ErrExist.
func Write(worktree, content string) error {
	f, err := os.OpenFile(filepath.Join(worktree, ".git"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, werr := io.WriteString(f, content)
	serr := f.Sync()
	if cerr := f.Close(); werr != nil || serr != nil || cerr != nil {
		return fmt.Errorf("writing %s/.git: %v %v %v", worktree, werr, serr, cerr)
	}
	return nil
}

func under(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
