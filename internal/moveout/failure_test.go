package moveout

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/journal"
)

// failRename makes the n-th rename from now on fail, like a disk that
// refuses one move; every other rename, the roll-back's included, works.
func failRename(t *testing.T, n int) {
	t.Helper()
	saved := rename
	t.Cleanup(func() { rename = saved })
	calls := 0
	rename = func(from, to string) error {
		calls++
		if calls == n {
			return errors.New("simulated rename failure")
		}
		return saved(from, to)
	}
}

func (l *lab) dotgitIsDir() bool {
	l.t.Helper()
	info, err := os.Lstat(filepath.Join(l.worktree, ".git"))
	return err == nil && info.IsDir()
}

// A move that fails at any of its moves rolls back: .git is the directory
// it was, git sees what it saw, and nothing is left at the target.
func TestApplyRollsBackWhenAMoveFails(t *testing.T) {
	for _, n := range []int{1, 2} { // the copy into place, the original aside
		l := newLab(t)
		head, st := l.git("rev-parse", "HEAD"), l.git("status", "--porcelain")
		p := l.plan()
		failRename(t, n)
		e, err := Apply(context.Background(), l.g, p, l.opts, l.store)
		if err == nil || !strings.Contains(err.Error(), "simulated") {
			t.Fatalf("move %d: err = %v", n, err)
		}
		if e.Phase != journal.RolledBack || exists(p.Target) || !l.dotgitIsDir() {
			t.Fatalf("move %d: phase %s, target left %v, .git a directory %v", n, e.Phase, exists(p.Target), l.dotgitIsDir())
		}
		l.healthy(head, st)
	}
}

// A git that starts while the directory is copied stops the move before the
// original is touched.
func TestApplyStopsWhenTheRepositoryBecomesBusy(t *testing.T) {
	l := newLab(t)
	head, st := l.git("rev-parse", "HEAD"), l.git("status", "--porcelain")
	p := l.plan()
	lock := filepath.Join(l.worktree, ".git", "index.lock")
	defer func() { stepHook = func(string) {} }()
	stepHook = func(step string) {
		if step == "copied" {
			os.WriteFile(lock, nil, 0o644)
		}
	}
	e, err := Apply(context.Background(), l.g, p, l.opts, l.store)
	if err == nil || !strings.Contains(err.Error(), "became busy") {
		t.Fatalf("err = %v", err)
	}
	if e.Phase != journal.RolledBack || exists(p.Target) || !l.dotgitIsDir() {
		t.Fatalf("phase %s, target left %v", e.Phase, exists(p.Target))
	}
	os.Remove(lock)
	l.healthy(head, st)
}

// An undo that fails at either move puts the moved-out state back: the .git
// file, the git directory outside, and the move-out still undoable.
func TestMoveInRollsBackWhenAMoveFails(t *testing.T) {
	for _, n := range []int{1, 2} { // the .git file aside, the directory back
		l := newLab(t)
		p := l.movedOut()
		head, st := l.git("rev-parse", "HEAD"), l.git("status", "--porcelain")
		u := l.planIn()
		failRename(t, n)
		e, err := MoveIn(context.Background(), l.g, u, l.opts, l.store)
		if err == nil || !strings.Contains(err.Error(), "simulated") {
			t.Fatalf("move %d: err = %v", n, err)
		}
		if e.Phase != journal.RolledBack || l.dotgitIsDir() || !exists(p.Target) {
			t.Fatalf("move %d: phase %s, .git a directory %v, target there %v", n, e.Phase, l.dotgitIsDir(), exists(p.Target))
		}
		l.healthy(head, st)
		if again := l.planIn(); len(again.Refusals) != 0 {
			t.Fatalf("move %d: the move-out can no longer be undone: %+v", n, again.Refusals)
		}
	}
}

// A prepare whose copy cannot be moved into place leaves no target, so a
// later attach does not find a copy it cannot vouch for.
func TestPrepareSetsTheCopyAsideWhenItFails(t *testing.T) {
	l := newLab(t)
	head := l.git("rev-parse", "HEAD")
	p := l.plan()
	failRename(t, 1)
	e, err := Prepare(context.Background(), p, l.opts, l.store)
	if err == nil || e.Phase != journal.RolledBack {
		t.Fatalf("err %v, entry %+v", err, e)
	}
	if exists(p.Target) || exists(e.Data["partial"]) || !exists(filepath.Join(filepath.Dir(e.Data["backup"]), "abandoned-partial")) {
		t.Fatalf("target %v, partial %v left in place", exists(p.Target), exists(e.Data["partial"]))
	}
	if !l.dotgitIsDir() {
		t.Fatal("prepare touched .git")
	}
	l.healthy(head, "M a.md")
}

// An attach whose clone cannot be moved to the target sets the clone aside.
func TestAttachSetsTheCloneAsideWhenTheMoveFails(t *testing.T) {
	l := newLab(t)
	remote := l.withRemote()
	p := l.plan()
	if _, err := Apply(context.Background(), l.g, p, l.opts, l.store); err != nil {
		t.Fatal(err)
	}
	os.Rename(p.Target, p.Target+".device-a")
	ap := l.planAttach(remote, "main")
	failRename(t, 1)
	e, err := Attach(context.Background(), l.g, ap, l.opts, l.store)
	if err == nil || e.Phase != journal.RolledBack {
		t.Fatalf("err %v, entry %+v", err, e)
	}
	if exists(p.Target) || exists(e.Data["partial"]) {
		t.Fatalf("target %v, clone %v left in place", exists(p.Target), exists(e.Data["partial"]))
	}
}

// A refused plan is never carried out, whoever calls.
func TestRefusedPlansAreNotCarriedOut(t *testing.T) {
	l := newLab(t)
	p := l.plan()
	p.Refusals = []Refusal{{Code: RefuseTargetExists}}
	if _, err := Apply(context.Background(), l.g, p, l.opts, l.store); err == nil {
		t.Error("move-out ran a refused plan")
	}
	if _, err := Prepare(context.Background(), p, l.opts, l.store); err == nil {
		t.Error("prepare ran a refused plan")
	}
	if _, err := MoveIn(context.Background(), l.g, UndoPlan{Refusals: p.Refusals}, l.opts, l.store); err == nil {
		t.Error("move-in ran a refused plan")
	}
	if _, err := Attach(context.Background(), l.g, AttachPlan{Refusals: p.Refusals}, l.opts, l.store); err == nil {
		t.Error("attach ran a refused plan")
	}
	if exists(p.Target) || !l.dotgitIsDir() {
		t.Fatal("a refused plan changed the disk")
	}
	if open, _ := l.store.Unfinished(p.RepoID); len(open) != 0 {
		t.Fatalf("a refused plan opened a journal entry: %+v", open)
	}
}

// Recovery is kind-specific: a move-out recovery never acts on another
// change's entry.
func TestRecoverRefusesOtherKinds(t *testing.T) {
	l := newLab(t)
	e := &journal.Entry{ID: "x", Kind: "fix", Worktree: l.worktree}
	if _, err := Recover(context.Background(), l.g, e, l.store); err == nil || !strings.Contains(err.Error(), "cannot recover a fix") {
		t.Fatalf("err = %v", err)
	}
}

// A cancelled recovery changes nothing and says how to resume.
func TestRecoveryStopsWhenCancelled(t *testing.T) {
	l := newLab(t)
	e := l.crashAt("original-moved")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Recover(ctx, l.g, e, l.store); err == nil || !strings.Contains(err.Error(), "again") {
		t.Fatalf("err = %v", err)
	}
	if !exists(e.Data["backup"]) || exists(filepath.Join(l.worktree, ".git")) {
		t.Fatal("a cancelled recovery moved something")
	}
	l = newLab(t)
	l.movedOut()
	u := l.planIn()
	store := l.store
	store.AfterSave = func(ph journal.Phase) {
		if ph == journal.GitfileMoved {
			panic(crash{})
		}
	}
	func() { defer func() { recover() }(); MoveIn(context.Background(), l.g, u, l.opts, store) }()
	open, _ := l.store.Unfinished(u.RepoID)
	if len(open) != 1 {
		t.Fatalf("unfinished %+v", open)
	}
	if _, err := RecoverMoveIn(ctx, l.g, &open[0], l.store); err == nil || !strings.Contains(err.Error(), "again") {
		t.Fatalf("move-in: err = %v", err)
	}
}
