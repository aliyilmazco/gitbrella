package repo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
)

func runner(t *testing.T) *gitx.Runner {
	t.Helper()
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	return g
}

func TestLocateMainWorktree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo with space")
	if err := fixtures.Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	r, err := Locate(context.Background(), runner(t), filepath.Join(dir, "."))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	if r.Worktree != want || r.GitDir != filepath.Join(want, ".git") || r.CommonDir != r.GitDir {
		t.Fatalf("unexpected paths: %+v", r)
	}
	if r.ObjectFormat != "sha1" || r.RefStorage != "files" {
		t.Fatalf("unexpected formats: %+v", r)
	}
}

func TestLocateWorksWhenIndexIsMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	if err := fixtures.Build("orphan-index-lock", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Locate(context.Background(), runner(t), dir); err != nil {
		t.Fatal(err)
	}
}

func TestLocateLinkedWorktree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wt")
	if err := fixtures.Build("linked-worktree", dir); err != nil {
		t.Fatal(err)
	}
	r, err := Locate(context.Background(), runner(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	main, _ := filepath.EvalSymlinks(dir + "-main")
	if r.CommonDir != filepath.Join(main, ".git") || filepath.Dir(r.GitDir) != filepath.Join(main, ".git", "worktrees") {
		t.Fatalf("unexpected dirs: %+v", r)
	}
}

func TestLocateReftable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rt")
	err := fixtures.Build("reftable", dir)
	if errors.Is(err, fixtures.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	r, err := Locate(context.Background(), runner(t), dir)
	if err != nil || r.RefStorage != "reftable" {
		t.Fatalf("want reftable, got %+v %v", r, err)
	}
}

func TestLocateErrors(t *testing.T) {
	g := runner(t)
	if _, err := Locate(context.Background(), g, t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("want ErrNotRepo, got %v", err)
	}
	bare := filepath.Join(t.TempDir(), "bare.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := Locate(context.Background(), g, bare); !errors.Is(err, ErrBare) {
		t.Fatalf("want ErrBare, got %v", err)
	}
	if _, err := Locate(context.Background(), g, filepath.Join(t.TempDir(), "missing")); err == nil || errors.Is(err, os.ErrExist) {
		t.Fatalf("want an error for a missing path, got %v", err)
	}
}

func TestClassify(t *testing.T) {
	err := classify("fatal: detected dubious ownership in repository at '/x'\n", "/x")
	if !errors.Is(err, ErrDubious) {
		t.Fatalf("want ErrDubious, got %v", err)
	}
	if err := classify("fatal: not a git repository", "/y"); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("want ErrNotRepo, got %v", err)
	}
}

func TestLocateRefusesANestedGitDirGitSkipped(t *testing.T) {
	outer := filepath.Join(t.TempDir(), "outer")
	if _, err := fixtures.New(outer); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(outer, "inner")
	if _, err := fixtures.New(inner); err != nil {
		t.Fatal(err)
	}
	// Without HEAD git does not see inner/.git as a repository and walks up to outer.
	if err := os.Rename(filepath.Join(inner, ".git", "HEAD"), filepath.Join(inner, ".git", "HEAD.moved")); err != nil {
		t.Fatal(err)
	}
	_, err := Locate(context.Background(), runner(t), filepath.Join(inner, "sub-does-not-matter", ".."))
	if !errors.Is(err, ErrNotRepo) || !strings.Contains(err.Error(), filepath.Join("inner", ".git")) {
		t.Fatalf("checked the enclosing repository instead: err = %v", err)
	}
	// A plain subdirectory of a repository is still that repository.
	plain := filepath.Join(outer, "docs")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Locate(context.Background(), runner(t), plain); err != nil {
		t.Fatal(err)
	}
}

// coreWorktreeLayout builds a/meta/.git whose core.worktree is a: git uses
// meta/.git and reports a as the top level.
func coreWorktreeLayout(t *testing.T) (top, meta string) {
	t.Helper()
	top = filepath.Join(t.TempDir(), "a")
	meta = filepath.Join(top, "meta")
	if err := os.MkdirAll(filepath.Join(meta, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", meta}, {"-C", meta, "config", "core.worktree", top}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return top, meta
}

func TestLocateAcceptsTheGitDirGitUses(t *testing.T) {
	top, meta := coreWorktreeLayout(t)
	for _, p := range []string{meta, filepath.Join(meta, "sub")} {
		r, err := Locate(context.Background(), runner(t), p)
		if err != nil {
			t.Fatalf("Locate(%s): %v", p, err)
		}
		if want, _ := filepath.EvalSymlinks(top); r.Worktree != want {
			t.Fatalf("worktree = %s, want %s", r.Worktree, want)
		}
	}
}

func TestLocateRefusalIgnoresPathSpelling(t *testing.T) {
	outer := filepath.Join(t.TempDir(), "outer")
	if _, err := fixtures.New(outer); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(outer, "inner")
	if _, err := fixtures.New(inner); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(inner, ".git", "HEAD"), filepath.Join(inner, ".git", "HEAD.moved")); err != nil {
		t.Fatal(err)
	}
	upper := filepath.Join(filepath.Dir(outer), "OUTER", "INNER")
	if _, err := os.Lstat(upper); err != nil {
		t.Skip("case-sensitive file system")
	}
	if _, err := Locate(context.Background(), runner(t), upper); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("a differently spelled path answered for the outer repository: %v", err)
	}
}

func TestLocateFromAHookThatChangedDirectory(t *testing.T) {
	base := t.TempDir()
	main := filepath.Join(base, "main")
	if _, err := fixtures.New(main); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(base, "wt")
	if out, err := exec.Command("git", "-C", main, "worktree", "add", "-q", wt, "-b", "feat").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	src := filepath.Join(wt, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	admin := filepath.Join(main, ".git", "worktrees", "wt")
	// git exports GIT_DIR=<admin> (no GIT_WORK_TREE) to a linked worktree's hook; the hook did `cd src`.
	r, err := Locate(context.Background(), runner(t), src, "GIT_DIR="+admin)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(wt); r.Worktree != want {
		t.Fatalf("worktree = %s, want %s", r.Worktree, want)
	}
	// `git submodule foreach` exports GIT_DIR=.git relative to a directory it already left.
	if _, err := Locate(context.Background(), runner(t), src, "GIT_DIR="+filepath.Join(src, ".git")); err != nil {
		t.Fatalf("a GIT_DIR that does not exist must fall back to discovery: %v", err)
	}
}

func TestLocateFromAHookOutsideTheWorktree(t *testing.T) {
	// Shared hooks often start with `cd "$(dirname "$0")"`, i.e. inside main/.git/hooks.
	newMain := func(t *testing.T) (base, main string) {
		base = t.TempDir()
		main = filepath.Join(base, "main")
		if _, err := fixtures.New(main); err != nil {
			t.Fatal(err)
		}
		return base, main
	}
	gitDo := func(t *testing.T, dir string, args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	elsewhere := func(main string) string { return filepath.Join(main, ".git", "hooks") }
	t.Run("relative-path worktree", func(t *testing.T) {
		if ok, _ := fixtures.GitAtLeast(2, 48); !ok {
			t.Skip("needs git 2.48")
		}
		base, main := newMain(t)
		gitDo(t, main, "-c", "worktree.useRelativePaths=true", "worktree", "add", "-q", "../wtrel", "-b", "f")
		r, err := Locate(context.Background(), runner(t), elsewhere(main), "GIT_DIR="+filepath.Join(main, ".git", "worktrees", "wtrel"))
		if err != nil {
			t.Fatal(err)
		}
		if want, _ := filepath.EvalSymlinks(filepath.Join(base, "wtrel")); r.Worktree != want {
			t.Fatalf("worktree = %s, want %s", r.Worktree, want)
		}
	})
	t.Run("worktree moved without repair", func(t *testing.T) {
		base, main := newMain(t)
		gitDo(t, main, "worktree", "add", "-q", filepath.Join(base, "wtA"), "-b", "f")
		if err := os.Rename(filepath.Join(base, "wtA"), filepath.Join(base, "wtB")); err != nil {
			t.Fatal(err)
		}
		_, err := Locate(context.Background(), runner(t), elsewhere(main), "GIT_DIR="+filepath.Join(main, ".git", "worktrees", "wtA"))
		if !errors.Is(err, ErrNotRepo) {
			t.Fatalf("answered for some other directory instead of refusing: %v", err)
		}
	})
	t.Run("main worktree, GIT_DIR exported", func(t *testing.T) {
		// rebase -x and aliases export an absolute GIT_DIR
		_, main := newMain(t)
		r, err := Locate(context.Background(), runner(t), t.TempDir(), "GIT_DIR="+filepath.Join(main, ".git"))
		if err != nil {
			t.Fatal(err)
		}
		if want, _ := filepath.EvalSymlinks(main); r.Worktree != want {
			t.Fatalf("worktree = %s, want %s", r.Worktree, want)
		}
	})
	t.Run("receive hook inside a non-bare repository's .git", func(t *testing.T) {
		_, main := newMain(t)
		gd := filepath.Join(main, ".git")
		r, err := Locate(context.Background(), runner(t), gd, "GIT_DIR="+gd)
		if err != nil {
			t.Fatal(err)
		}
		if want, _ := filepath.EvalSymlinks(main); r.Worktree != want {
			t.Fatalf("worktree = %s, want %s (the .git directory is never a worktree)", r.Worktree, want)
		}
	})
}

func TestLocateMarksAGuessedWorktree(t *testing.T) {
	base := t.TempDir()
	sep := filepath.Join(base, "sep.git")
	wt := filepath.Join(base, "wt")
	if out, err := exec.Command("git", "init", "-q", "--separate-git-dir", sep, wt).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	elsewhere := t.TempDir()
	r, err := Locate(context.Background(), runner(t), elsewhere, "GIT_DIR="+sep)
	if err != nil {
		t.Fatal(err)
	}
	if !r.WorktreeGuessed {
		t.Fatalf("worktree %s came from the current directory but is not marked as guessed", r.Worktree)
	}
	r, err = Locate(context.Background(), runner(t), wt, "GIT_DIR="+sep)
	if err != nil {
		t.Fatal(err)
	}
	if r.WorktreeGuessed {
		t.Fatal("discovery from the worktree is not a guess")
	}
}

func TestLocateThroughASymlinkedGitDir(t *testing.T) {
	if ok, _ := fixtures.GitAtLeast(2, 48); !ok {
		t.Skip("needs git 2.48")
	}
	base := t.TempDir()
	main := filepath.Join(base, "main")
	if _, err := fixtures.New(main); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", main, "-c", "worktree.useRelativePaths=true", "worktree", "add", "-q", "../wtrel", "-b", "f").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(filepath.Join(main, ".git"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := Locate(context.Background(), runner(t), t.TempDir(), "GIT_DIR="+filepath.Join(link, "worktrees", "wtrel")); err != nil {
		t.Fatalf("a healthy relative-path worktree refused through a symlinked GIT_DIR: %v", err)
	}
}

func TestLocateASymlinkedDotGitBelongsToItsParent(t *testing.T) {
	// The forum layout for synced folders: .git is a symlink to a store
	// outside them. Given as GIT_DIR, it still names its parent's repository.
	base := t.TempDir()
	store := filepath.Join(base, "store", "repo.git")
	r := filepath.Join(base, "r")
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", "-b", "main", "--separate-git-dir", store, r).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	dotgit := filepath.Join(r, ".git")
	if err := os.Remove(dotgit); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(store, dotgit); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{t.TempDir(), filepath.Join(store, "hooks")} {
		got, err := Locate(context.Background(), runner(t), from, "GIT_DIR="+dotgit)
		if err != nil {
			t.Fatalf("from %s: %v", from, err)
		}
		// git itself would take the current directory as the worktree, so
		// the parent is marked as a guess (INDEX_EMPTY then always blocks).
		if want, _ := filepath.EvalSymlinks(r); got.Worktree != want || !got.WorktreeGuessed {
			t.Fatalf("from %s: worktree %s (guessed %v), want %s, guessed", from, got.Worktree, got.WorktreeGuessed, want)
		}
	}
}

func TestLocateASymlinkedDotGitToAnotherCheckout(t *testing.T) {
	// elsewhere/.git -> real/.git: either side of the link may hold the
	// files, so the worktree is a guess (the check's safety net for an empty
	// index is tested in symlinked_dotgit.txtar).
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if _, err := fixtures.New(real); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(base, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(real, ".git"), filepath.Join(elsewhere, ".git")); err != nil {
		t.Fatal(err)
	}
	got, err := Locate(context.Background(), runner(t), t.TempDir(), "GIT_DIR="+filepath.Join(elsewhere, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(filepath.Join(real, ".git")); got.GitDir != want || !got.WorktreeGuessed {
		t.Fatalf("git dir %s (guessed %v), want %s, guessed", got.GitDir, got.WorktreeGuessed, want)
	}
}
