// Package fixtures builds git repositories with the damage that cloud sync
// clients leave behind. It is used only by tests.
package fixtures

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrUnsupported is returned when the installed git cannot build a scenario.
var ErrUnsupported = errors.New("scenario not supported by the installed git")

// Repo is a fixture repository with a worktree at Dir.
type Repo struct {
	Dir     string
	GitDir  string
	Commits []string // oldest first
}

var testEnv = []string{
	"GIT_CONFIG_NOSYSTEM=1",
	"GIT_CONFIG_GLOBAL=" + os.DevNull,
	// git >= 2.47 starts "git maintenance run --auto --detach" after a
	// commit; the detached process can still write in .git while a test's
	// temporary directory is removed.
	"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false",
	"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
	"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
	"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
}

func gitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), testEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// New creates a repository with two commits on main.
func New(dir string, initArgs ...string) (*Repo, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	args := append([]string{"init", "-q", "-b", "main"}, initArgs...)
	if _, err := gitIn(dir, append(args, ".")...); err != nil {
		return nil, err
	}
	r := &Repo{Dir: dir, GitDir: filepath.Join(dir, ".git")}
	for i, content := range []string{"one\n", "two\n"} {
		if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte(content), 0o644); err != nil {
			return nil, err
		}
		if _, err := gitIn(dir, "add", "note.md"); err != nil {
			return nil, err
		}
		if _, err := gitIn(dir, "commit", "-q", "-m", fmt.Sprintf("commit %d", i+1)); err != nil {
			return nil, err
		}
		oid, err := gitIn(dir, "rev-parse", "HEAD")
		if err != nil {
			return nil, err
		}
		r.Commits = append(r.Commits, oid)
	}
	return r, nil
}

// Git runs git in the worktree with the fixture environment.
func (r *Repo) Git(args ...string) (string, error) { return gitIn(r.Dir, args...) }

func (r *Repo) path(rel string) string { return filepath.Join(r.GitDir, filepath.FromSlash(rel)) }

// CopyFile copies .git/<rel> to .git/<copyRel>, keeping the mode.
func (r *Repo) CopyFile(rel, copyRel string) error { return copyFile(r.path(rel), r.path(copyRel)) }

// RenameFile renames .git/<from> to .git/<to>, the way iCloud reverts a rename.
func (r *Repo) RenameFile(from, to string) error { return os.Rename(r.path(from), r.path(to)) }

// WriteFile writes data to .git/<rel>, creating parent directories.
func (r *Repo) WriteFile(rel, data string) error {
	p := r.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(data), 0o644)
}

// Age sets the modification time of .git/<rel> to now-age.
func (r *Repo) Age(rel string, age time.Duration) error {
	t := time.Now().Add(-age)
	return os.Chtimes(r.path(rel), t, t)
}

// FlipByte inverts one byte of .git/<rel>.
func (r *Repo) FlipByte(rel string, offset int) error {
	b, err := os.ReadFile(r.path(rel))
	if err != nil {
		return err
	}
	if offset >= len(b) {
		return fmt.Errorf("offset %d beyond %d bytes", offset, len(b))
	}
	b[offset] ^= 0xff
	return os.WriteFile(r.path(rel), b, 0o644)
}

// CopyDir copies the directory .git/<rel> to .git/<copyRel>.
func (r *Repo) CopyDir(rel, copyRel string) error {
	src, dst := r.path(rel), r.path(copyRel)
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, strings.TrimPrefix(p, src))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
