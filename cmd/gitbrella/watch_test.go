package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/moveout"
)

// movedLayout is what a finished move-out leaves: a .git file in the
// worktree and the git directory it names.
func movedLayout(t *testing.T) (moveout.Plan, string) {
	t.Helper()
	wt, target := t.TempDir(), t.TempDir()
	p := moveout.Plan{Target: target, Gitfile: "gitdir: " + target + "\n", Repo: check.RepoInfo{Worktree: wt}}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte(p.Gitfile), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p, wt
}

func TestWatchAfterTheMove(t *testing.T) {
	ctx := context.Background()
	t.Run("left alone", func(t *testing.T) {
		p, wt := movedLayout(t)
		if w := watch(ctx, time.Nanosecond, p, wt); len(w) != 0 {
			t.Fatalf("warnings %v", w)
		}
	})
	t.Run("not asked to watch", func(t *testing.T) {
		p, wt := movedLayout(t)
		os.Remove(filepath.Join(wt, ".git"))
		if w := watch(ctx, 0, p, wt); len(w) != 0 {
			t.Fatalf("warnings %v", w)
		}
	})
	t.Run("old directory brought back", func(t *testing.T) {
		p, wt := movedLayout(t)
		os.Remove(filepath.Join(wt, ".git"))
		os.Mkdir(filepath.Join(wt, ".git"), 0o755)
		if w := watch(ctx, time.Nanosecond, p, wt); len(w) != 1 || !strings.HasPrefix(w[0], "GIT_DIR_RETURNED") {
			t.Fatalf("warnings %v", w)
		}
	})
	t.Run("git directory gone", func(t *testing.T) {
		p, wt := movedLayout(t)
		os.Remove(filepath.Join(p.Target, "HEAD"))
		if w := watch(ctx, time.Nanosecond, p, wt); len(w) != 1 || !strings.Contains(w[0], "disappeared") {
			t.Fatalf("warnings %v", w)
		}
	})
	t.Run("interrupted", func(t *testing.T) {
		p, wt := movedLayout(t)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if w := watch(cancelled, time.Hour, p, wt); len(w) != 1 || !strings.Contains(w[0], "interrupted") {
			t.Fatalf("warnings %v", w)
		}
	})
}
