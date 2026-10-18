package moveout

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/journal"
)

// movedOut moves the lab out and makes a commit afterwards, which the undo
// must keep.
func (l *lab) movedOut() Plan {
	l.t.Helper()
	p := l.plan()
	if _, err := Apply(context.Background(), l.g, p, l.opts, l.store); err != nil {
		l.t.Fatal(err)
	}
	l.git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "after the move")
	return p
}

func (l *lab) planIn() UndoPlan {
	l.t.Helper()
	u, err := PlanMoveIn(context.Background(), l.g, l.opts, l.store)
	if err != nil {
		l.t.Fatal(err)
	}
	return u
}

func TestMoveInKeepsCommitsMadeSinceTheMove(t *testing.T) {
	l := newLab(t)
	p := l.movedOut()
	head := l.git("rev-parse", "HEAD")
	u := l.planIn()
	if len(u.Refusals) != 0 || u.GitDir != p.Target {
		t.Fatalf("plan: %+v", u)
	}
	e, err := MoveIn(context.Background(), l.g, u, l.opts, l.store)
	if err != nil || e.Phase != journal.Done {
		t.Fatalf("%v %+v", err, e)
	}
	if info, err := os.Lstat(filepath.Join(l.worktree, ".git")); err != nil || !info.IsDir() {
		t.Fatal(".git is not a directory again")
	}
	l.healthy(head, "")
	if exists(p.Target) {
		t.Fatal("the moved git directory is still outside")
	}
	if out, _, _ := l.store.LastDone(p.RepoID, "move-out"); out.ID != "" {
		t.Fatalf("the move-out is not marked undone: %+v", out)
	}
	if again := l.planIn(); codes(again.Refusals) != RefuseNothingToUndo {
		t.Fatalf("second undo: %+v", again.Refusals)
	}
}

func TestMoveInRefusesAChangedGitfile(t *testing.T) {
	l := newLab(t)
	l.movedOut()
	if again := l.planIn(); len(again.Refusals) != 0 {
		t.Fatalf("%+v", again.Refusals)
	}
	// rewritten in place: the same inode on every system
	os.WriteFile(filepath.Join(l.worktree, ".git"), []byte("gitdir: /else\n"), 0o644)
	if u := l.planIn(); codes(u.Refusals) != RefuseChangedSince {
		t.Fatalf("rewritten: %+v", u.Refusals)
	}
	// replaced: Linux hands the freed inode to the new file at once
	os.Remove(filepath.Join(l.worktree, ".git"))
	os.WriteFile(filepath.Join(l.worktree, ".git"), []byte("gitdir: /else\n"), 0o644)
	if u := l.planIn(); codes(u.Refusals) != RefuseChangedSince {
		t.Fatalf("replaced: %+v", u.Refusals)
	}
}

// The git directory outside was replaced since the move (a restore from a
// backup, another tool): undo does not bring back what it did not move.
func TestMoveInRefusesAReplacedGitDir(t *testing.T) {
	l := newLab(t)
	p := l.movedOut()
	if err := os.Rename(p.Target, p.Target+".old"); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-R", p.Target+".old", p.Target).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if u := l.planIn(); codes(u.Refusals) != RefuseChangedSince {
		t.Fatalf("%+v", u.Refusals)
	}
	if _, err := MoveIn(context.Background(), l.g, l.planIn(), l.opts, l.store); err == nil {
		t.Fatal("moved in a refused plan")
	}
}

func TestRecoverMoveInAfterACrashAnywhere(t *testing.T) {
	for _, tc := range []struct{ point, want string }{
		{"planned", "rolled back"},
		{"step:gitfile-moved", "rolled back"},
		{"gitfile-moved", "rolled back"},
		{"step:gitdir-returned", "finished"},
		{"gitdir-returned", "finished"},
	} {
		t.Run(tc.point, func(t *testing.T) {
			l := newLab(t)
			l.movedOut()
			head := l.git("rev-parse", "HEAD")
			u := l.planIn()
			store := l.store
			store.AfterSave = func(ph journal.Phase) {
				if string(ph) == tc.point {
					panic(crash{})
				}
			}
			defer func() { stepHook = func(string) {} }()
			stepHook = func(step string) {
				if "step:"+step == tc.point {
					panic(crash{})
				}
			}
			func() {
				defer func() { recover() }()
				MoveIn(context.Background(), l.g, u, l.opts, store)
				t.Fatal("no crash")
			}()
			open, _ := l.store.Unfinished(u.RepoID)
			if len(open) != 1 {
				t.Fatalf("unfinished %+v", open)
			}
			did, err := RecoverMoveIn(context.Background(), l.g, &open[0], l.store)
			if err != nil || did != tc.want {
				t.Fatalf("recovery %q %v, want %s", did, err, tc.want)
			}
			l.healthy(head, "")
			info, _ := os.Lstat(filepath.Join(l.worktree, ".git"))
			if info.IsDir() != (tc.want == "finished") {
				t.Fatalf(".git is a directory: %v", info.IsDir())
			}
		})
	}
}

// cloudOnly reports the paths in it as dataless.
type cloudOnly map[string]bool

func (c cloudOnly) IsDataless(p string) (bool, error) { return c[p], nil }

// The sync client evicted the .git file (only its name is local): reading it
// would download it, so undo says to download it first instead of calling it
// someone else's file.
func TestMoveInWaitsForAGitfileOnlyInTheCloud(t *testing.T) {
	l := newLab(t)
	l.movedOut()
	l.opts.Flags = cloudOnly{filepath.Join(l.worktree, ".git"): true}
	if u := l.planIn(); codes(u.Refusals) != RefuseGitfileInCloud {
		t.Fatalf("%+v", u.Refusals)
	}
}

// The same during recovery: an interrupted move-in whose .git file is only in
// the cloud waits for it rather than refusing it as changed.
func TestRecoverMoveInWaitsForAGitfileOnlyInTheCloud(t *testing.T) {
	l := newLab(t)
	l.movedOut()
	u := l.planIn()
	store := l.store
	store.AfterSave = func(ph journal.Phase) {
		if ph == journal.Planned {
			panic(crash{})
		}
	}
	func() {
		defer func() { recover() }()
		MoveIn(context.Background(), l.g, u, l.opts, store)
		t.Fatal("no crash")
	}()
	open, _ := l.store.Unfinished(u.RepoID)
	dotgit := filepath.Join(l.worktree, ".git")
	defer func() { inCloud = func(string) bool { return false } }()
	inCloud = func(p string) bool { return p == dotgit }
	if _, err := RecoverMoveIn(context.Background(), l.g, &open[0], l.store); err == nil || !strings.Contains(err.Error(), "only in the cloud") {
		t.Fatalf("recovery with the .git file in the cloud: %v", err)
	}
	inCloud = func(string) bool { return false }
	if did, err := RecoverMoveIn(context.Background(), l.g, &open[0], l.store); err != nil || did != "rolled back" {
		t.Fatalf("recovery once it is here: %q %v", did, err)
	}
}
