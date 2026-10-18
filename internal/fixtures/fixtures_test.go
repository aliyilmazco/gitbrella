package fixtures

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryScenarioBuilds(t *testing.T) {
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "repo with space")
			err := Build(name, dir)
			if errors.Is(err, ErrUnsupported) {
				t.Skip(err)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
				t.Fatalf("no .git in %s: %v", dir, err)
			}
		})
	}
}

func TestScenarioDamageIsPresent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	if err := Build("orphan-index-lock", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(Abs(dir, "index")); !os.IsNotExist(err) {
		t.Fatalf("index should be missing, got %v", err)
	}
	if _, err := os.Lstat(Abs(dir, "index.lock")); err != nil {
		t.Fatalf("index.lock should exist: %v", err)
	}
}

func TestSnapshotDetectsChanges(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	if err := Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	a, err := TakeSnapshot(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := TakeSnapshot(dir, nil)
	if d := a.Diff(b); len(d) != 0 {
		t.Fatalf("unchanged tree reported changes: %v", d)
	}
	if err := os.WriteFile(Abs(dir, "HEAD"), []byte("ref: refs/heads/other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := TakeSnapshot(dir, nil)
	if d := a.Diff(c); len(d) != 1 || d[0] != "changed: .git/HEAD" {
		t.Fatalf("want one change to .git/HEAD, got %v", d)
	}
}

// The hazard behind GIT_DIR_NOT_IGNORED is git's own behaviour: a git
// directory inside the working tree under another name is untracked content.
func TestNoSyncGitDirIsUntrackedUnlessExcluded(t *testing.T) {
	for name, wantListed := range map[string]bool{"nosync-unignored": true, "nosync-ignored": false} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "repo with space")
			if err := Build(name, dir); err != nil {
				t.Fatal(err)
			}
			out, err := gitIn(dir, "status", "--porcelain")
			if err != nil {
				t.Fatal(err)
			}
			if listed := strings.Contains(out, "?? .git.nosync/"); listed != wantListed {
				t.Fatalf("git status lists .git.nosync/ = %v, want %v; status:\n%s", listed, wantListed, out)
			}
		})
	}
}
