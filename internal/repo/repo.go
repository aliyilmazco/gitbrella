// Package repo finds the worktree and git directories of a repository.
package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/gitx"
)

var (
	ErrNotRepo = errors.New("not a git repository")
	ErrBare    = errors.New("bare repositories are not supported")
	ErrDubious = errors.New("git refuses this repository because of its ownership (see 'git config --global --add safe.directory')")
)

// Repo describes a non-bare repository. All paths are absolute with symlinks resolved.
type Repo struct {
	WorktreeGuessed bool // taken from the current directory: GIT_DIR without an absolute GIT_WORK_TREE or core.worktree
	Worktree        string
	GitDir          string // per-worktree git dir (".git" or ".git/worktrees/<name>")
	CommonDir       string // shared git dir holding objects and refs
	ObjectFormat    string // "sha1" or "sha256"
	RefStorage      string // "files" or "reftable"
}

// classify turns a failed rev-parse into ErrDubious or ErrNotRepo.
func classify(stderr, path string) error {
	if strings.Contains(stderr, "dubious ownership") {
		return ErrDubious
	}
	return fmt.Errorf("%w: %s", ErrNotRepo, path)
}

// Locate finds the repository containing path. env may carry GIT_DIR and
// GIT_WORK_TREE as git exports them to hooks; they are used for discovery only.
//
// With GIT_DIR alone, git takes the current directory as the worktree top,
// which is wrong when a hook changed directory. So the worktree is found from
// the git directory itself whenever possible: plain discovery from path when
// it reaches the same git directory; a linked worktree's admin directory names
// its worktree in its gitdir file (relative to the admin directory with
// worktree.useRelativePaths); a directory named .git belongs to its parent.
// A linked worktree that cannot be traced back (moved without 'git worktree
// repair') is refused rather than answered for the current directory, and a
// worktree inside the git directory (receive hooks run in .git) is refused.
// A GIT_DIR that does not exist (relative to a directory already left) is
// ignored.
func Locate(ctx context.Context, g *gitx.Runner, path string, env ...string) (Repo, error) {
	gitDir, workTree := envValue(env, "GIT_DIR"), envValue(env, "GIT_WORK_TREE")
	if gitDir == "" {
		return locate(ctx, g, path)
	}
	if _, err := os.Stat(gitDir); err != nil {
		return locate(ctx, g, path)
	}
	given := filepath.Clean(gitDir) // a symlinked .git still belongs to its parent
	if real, err := filepath.EvalSymlinks(gitDir); err == nil {
		gitDir = real // a relative admin gitdir is relative to the real directory
	}
	if workTree == "" {
		if r, err := locate(ctx, g, path); err == nil && sameFile(r.GitDir, gitDir) {
			return r, nil
		}
		if _, err := os.Stat(filepath.Join(gitDir, "commondir")); err == nil {
			if b, err := readSmall(filepath.Join(gitDir, "gitdir")); err == nil {
				dotgit := strings.TrimSpace(string(b))
				if !filepath.IsAbs(dotgit) {
					dotgit = filepath.Join(gitDir, dotgit)
				}
				if r, err := locate(ctx, g, filepath.Dir(dotgit)); err == nil && sameFile(r.GitDir, gitDir) {
					return r, nil
				}
			}
			return Repo{}, fmt.Errorf("%w: GIT_DIR %s is a linked worktree whose working tree was not found (moved without 'git worktree repair'?); run the check from the worktree or give its path", ErrNotRepo, gitDir)
		}
		// Discovery from path did not reach it, so git takes path as the
		// worktree top: the .git's parent is a guess. The name as given
		// first (a .git linked to a store outside a synced folder belongs
		// where the link is), then the resolved one.
		names := []string{given}
		if c := filepath.Clean(gitDir); c != given {
			names = append(names, c)
		}
		for _, d := range names {
			if filepath.Base(d) == ".git" {
				if r, err := locate(ctx, g, filepath.Dir(d)); err == nil && sameFile(r.GitDir, gitDir) {
					r.WorktreeGuessed = true
					return r, nil
				}
			}
		}
	}
	r, err := locate(ctx, g, path, env...)
	if err != nil {
		return r, err
	}
	if sameFile(r.Worktree, r.GitDir) || within(r.Worktree, r.GitDir) {
		return Repo{}, fmt.Errorf("%w: with GIT_DIR=%s git would take %s, inside the git directory, as the working tree; give the working tree's path", ErrNotRepo, gitDir, r.Worktree)
	}
	// Without an absolute GIT_WORK_TREE or core.worktree, git takes the current
	// directory as the working tree; a hook that changed directory makes that
	// a guess, and checks that compare the working tree must not trust it.
	if workTree == "" || !filepath.IsAbs(workTree) {
		res, err := g.Run(ctx, r.Worktree, env, "config", "--get", "core.worktree")
		r.WorktreeGuessed = err != nil || res.ExitCode != 0
	}
	return r, nil
}

// within reports whether p lies below dir (resolved paths).
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func envValue(env []string, key string) string {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return ""
}

func sameFile(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

// readSmall reads at most 4 KiB. A cloud-only file fails to read (downloads
// are off for this process) rather than downloading.
func readSmall(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 4096))
}

func locate(ctx context.Context, g *gitx.Runner, path string, env ...string) (Repo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Repo{}, err
	}
	if abs, err = filepath.EvalSymlinks(abs); err != nil {
		return Repo{}, err
	}
	res, err := g.Run(ctx, abs, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return Repo{}, err
	}
	if res.ExitCode != 0 {
		return Repo{}, classify(res.Stderr, abs)
	}
	if strings.TrimSpace(res.Stdout) == "true" {
		return Repo{}, ErrBare
	}
	res, err = g.Run(ctx, abs, env, "rev-parse", "--absolute-git-dir",
		"--path-format=absolute", "--git-common-dir", "--show-toplevel", "--show-object-format", "--show-prefix")
	if err != nil {
		return Repo{}, err
	}
	lines := strings.Split(strings.TrimSuffix(res.Stdout, "\n"), "\n")
	if res.ExitCode != 0 || len(lines) != 5 {
		return Repo{}, fmt.Errorf("%w: %s: %s", ErrNotRepo, abs, strings.TrimSpace(res.Stderr))
	}
	r := Repo{ObjectFormat: lines[3], RefStorage: "files"}
	for i, dst := range []*string{&r.GitDir, &r.CommonDir, &r.Worktree} {
		if *dst, err = filepath.EvalSymlinks(lines[i]); err != nil {
			return Repo{}, err
		}
	}
	// git skips a .git it cannot use (no HEAD, unreadable, only in the cloud)
	// and keeps walking up. Answering for the enclosing repository would be
	// answering the wrong question, so stop at the first .git on the way.
	// The walk follows git's own spelling of the path (--show-prefix), so
	// case and symlinks in the given path cannot step around it.
	parts := strings.Split(strings.TrimSuffix(lines[4], "/"), "/")
	for k := len(parts); k > 0 && parts[0] != ""; k-- {
		dotgit := filepath.Join(r.Worktree, filepath.Join(parts[:k]...), ".git")
		if _, err := os.Lstat(dotgit); err == nil && !usedBy(dotgit, r) {
			return Repo{}, fmt.Errorf("%w: %s (git skipped %s and found the enclosing repository %s)",
				ErrNotRepo, abs, dotgit, r.Worktree)
		}
	}
	res, err = g.Run(ctx, abs, r.Env(), "config", "--get", "extensions.refStorage")
	if err != nil {
		return Repo{}, err
	}
	if v := strings.TrimSpace(res.Stdout); res.ExitCode == 0 && v != "" {
		r.RefStorage = v
	}
	return r, nil
}

// Env points git at exactly this repository, whatever the working directory:
// a git directory inside its own worktree (core.worktree) is not found by
// discovery from the top level.
func (r Repo) Env() []string {
	return []string{"GIT_DIR=" + r.GitDir, "GIT_WORK_TREE=" + r.Worktree}
}

// usedBy reports whether dotgit is the git directory git chose for r, either
// directly or as a .git file pointing at it. A .git file is read only when it
// is a small regular file; a cloud-only one fails to read (downloads are off)
// and so counts as skipped.
func usedBy(dotgit string, r Repo) bool {
	same := func(p string) bool {
		a, err := os.Stat(p)
		if err != nil {
			return false
		}
		for _, d := range []string{r.GitDir, r.CommonDir} {
			if b, err := os.Stat(d); err == nil && os.SameFile(a, b) {
				return true
			}
		}
		return false
	}
	info, err := os.Stat(dotgit)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return same(dotgit)
	}
	if !info.Mode().IsRegular() {
		return false
	}
	b, err := readSmall(dotgit)
	if err != nil {
		return false
	}
	target, ok := strings.CutPrefix(strings.TrimRight(strings.SplitN(string(b), "\n", 2)[0], "\r"), "gitdir: ")
	if !ok {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(dotgit), target)
	}
	return same(target)
}
