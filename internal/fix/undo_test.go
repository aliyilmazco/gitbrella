package fix

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// git used the rebuilt index after the fix: undo leaves it as it is, says
// so, and still finishes.
func TestUndoKeepsWhatChangedSince(t *testing.T) {
	g, r, _ := scenario(t, "index-missing")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	fx, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	if u, err := PlanUndo(context.Background(), g, o, *fx); err != nil || len(u.Refusals) != 0 || len(u.Kept) != 0 {
		t.Fatalf("fresh fix: %+v %v", u, err)
	}
	// git writes the rebuilt index (a stat refresh): it is no longer the fix's
	if res, _ := g.Run(context.Background(), r.Worktree, nil, "update-index", "--refresh"); res.ExitCode > 1 {
		t.Fatalf("refresh: %s", res.Stderr)
	}
	index := content(t, r.Worktree)[".git/index"]
	u, err := PlanUndo(context.Background(), g, o, *fx)
	if err != nil || len(u.Refusals) != 0 || len(u.Kept) != 1 || !strings.Contains(u.Kept[0], "git used it") {
		t.Fatalf("after git used the index: %+v %v", u, err)
	}
	_, kept, err := Undo(context.Background(), g, u, store)
	if err != nil || len(kept) != 1 {
		t.Fatalf("undo: %v %v", kept, err)
	}
	if got := content(t, r.Worktree)[".git/index"]; got != index {
		t.Fatal("undo replaced the index git had used")
	}
	if _, ok, _ := store.LastUndoable(p.RepoID); ok {
		t.Fatal("the fix is still undoable")
	}
}

func TestUndoMarksTheFixUndone(t *testing.T) {
	g, r, _ := scenario(t, "index-copies")
	before := content(t, r.Worktree)
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	fx, _ := Apply(context.Background(), g, p, o, store)
	if last, ok, _ := store.LastUndoable(p.RepoID); !ok || last.ID != fx.ID {
		t.Fatalf("last undoable %+v", last)
	}
	u, _ := PlanUndo(context.Background(), g, o, *fx)
	if _, kept, err := Undo(context.Background(), g, u, store); err != nil || len(kept) > 0 {
		t.Fatal(err)
	}
	sameContent(t, before, content(t, r.Worktree))
	if _, ok, _ := store.LastUndoable(p.RepoID); ok {
		t.Fatal("the fix is still undoable")
	}
}

// The user deleted the backups: undo cannot put back what is gone, keeps the
// branch the fix repaired as it is rather than emptying it, says so, and
// still finishes.
func TestUndoWithTheBackupGone(t *testing.T) {
	g, r, _ := scenario(t, "empty-ref")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	fx, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	branch := content(t, r.Worktree)[".git/refs/heads/main"]
	if err := os.RemoveAll(o.Dirs.Backups()); err != nil {
		t.Fatal(err)
	}
	u, err := PlanUndo(context.Background(), g, o, *fx)
	if err != nil || len(u.Kept) != 1 || !strings.Contains(u.Kept[0], "is gone") {
		t.Fatalf("plan %+v %v", u, err)
	}
	if _, kept, err := Undo(context.Background(), g, u, store); err != nil || len(kept) != 1 {
		t.Fatalf("undo: %v %v", kept, err)
	}
	if content(t, r.Worktree)[".git/refs/heads/main"] != branch {
		t.Fatal("undo changed the branch")
	}
	if _, ok, _ := store.LastUndoable(p.RepoID); ok {
		t.Fatal("the fix is still undoable")
	}
}

// The sync client brought back the copy fix moved away: undo leaves the copy
// in the backup, and so the rescue ref that keeps its commit alive.
func TestUndoKeepsTheRescueWhileItsCopyStays(t *testing.T) {
	g, r, _ := scenario(t, "ref-copy-newer")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	fx, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	write(t, r, "refs/heads/main 2", git(t, g, r, "", "rev-parse", "HEAD")+"\n")
	u, _ := PlanUndo(context.Background(), g, o, *fx)
	if _, kept, err := Undo(context.Background(), g, u, store); err != nil || len(kept) != 2 {
		t.Fatalf("undo: %v %v", kept, err)
	}
	git(t, g, r, "", "rev-parse", "--verify", p.Actions[0].Ref)
}

// .git became a .git file after the fix (the git directory moved): the
// paths the fix used are not the repository's any more. undo touches
// nothing there and keeps the rescue.
func TestUndoAfterTheGitDirectoryMoved(t *testing.T) {
	g, r, _ := scenario(t, "ref-copy-newer")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	fx, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "gitdir")
	if err := os.Rename(r.GitDir, moved); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(r.GitDir, []byte("gitdir: "+moved+"\n"), 0o644)
	u, err := PlanUndo(context.Background(), g, o, *fx)
	if err != nil || len(u.Kept) != 2 || !strings.Contains(u.Kept[0], "somewhere else") {
		t.Fatalf("plan %+v %v", u, err)
	}
	if _, _, err := Undo(context.Background(), g, u, store); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(r.GitDir); err != nil || info.IsDir() {
		t.Fatalf(".git: %v %v", info, err)
	}
	res, _ := g.Run(context.Background(), r.Worktree, nil, "rev-parse", "--verify", "-q", p.Actions[0].Ref)
	if res.ExitCode != 0 {
		t.Fatal("the rescue ref is gone")
	}
}

// An undo killed half way, then the folder renamed: the recovery finds the
// repository under its new name, keeps what was recorded under the old one,
// and ends instead of blocking every change command.
func TestRecoverUnfixAfterTheFolderWasRenamed(t *testing.T) {
	g, r, _ := scenario(t, "index-copies")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	fx, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	e, err := store.Begin("unfix", fx.RepoID, fx.Worktree, func(string) map[string]string {
		return map[string]string{"undoes": fx.ID}
	})
	if err != nil {
		t.Fatal(err)
	}
	renamed := r.Worktree + " renamed"
	if err := os.Rename(r.Worktree, renamed); err != nil {
		t.Fatal(err)
	}
	e.Worktree = renamed // as start does for an unfinished change of a renamed folder
	did, err := RecoverUnfix(context.Background(), g, e, store)
	if err != nil || !strings.Contains(did, "somewhere else") {
		t.Fatalf("recover: %q %v", did, err)
	}
	if left, _ := store.Unfinished(fx.RepoID); len(left) != 0 {
		t.Fatalf("still unfinished: %+v", left)
	}
}

// fix made info/ to hold the exclude line: undo removes it again, as it was.
func TestUndoRemovesTheDirectoryItMade(t *testing.T) {
	g, r, _ := scenario(t, "nosync-unignored")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	if len(p.Actions) != 1 || p.Actions[0].Kind != AddExclude {
		t.Fatalf("plan %+v", p.Actions)
	}
	if err := os.RemoveAll(filepath.Dir(p.Actions[0].Path)); err != nil {
		t.Fatal(err)
	}
	before := content(t, r.Worktree)
	p, _ = MakePlan(context.Background(), g, o)
	fx, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := PlanUndo(context.Background(), g, o, *fx)
	if _, kept, err := Undo(context.Background(), g, u, store); err != nil || len(kept) != 0 {
		t.Fatalf("undo: %v %v", kept, err)
	}
	sameContent(t, before, content(t, r.Worktree))
}

// A crash after a step's reversal is saved as begun and before the disk
// changes, at every step of an undo: the next run finishes the undo and the
// repository is as it was before the fix.
func TestRecoverAfterACrashInEveryReversal(t *testing.T) {
	for _, name := range []string{"index-missing", "ref-copy-newer", "orphan-index-lock", "empty-ref", "nosync-unignored", "index-copies"} {
		g0, r0, _ := scenario(t, name)
		o0, _ := fixOptions(t, r0.Worktree)
		p0, _ := MakePlan(context.Background(), g0, o0)
		for i := 0; i < len(p0.Actions); i++ {
			t.Run(name+"/"+strconv.Itoa(i+1), func(t *testing.T) {
				g, r, _ := scenario(t, name)
				before := content(t, r.Worktree)
				o, store := fixOptions(t, r.Worktree)
				p, _ := MakePlan(context.Background(), g, o)
				fx, err := Apply(context.Background(), g, p, o, store)
				if err != nil {
					t.Fatal(err)
				}
				u, _ := PlanUndo(context.Background(), g, o, *fx)
				n := 0
				defer func() { reverseHook = func(string) {} }()
				reverseHook = func(string) {
					if n == i {
						panic(crash{})
					}
					n++
				}
				func() {
					defer func() { recover() }()
					Undo(context.Background(), g, u, store)
					t.Fatal("no crash")
				}()
				reverseHook = func(string) {}
				open, _ := store.Unfinished(p.RepoID)
				if len(open) != 1 || open[0].Kind != "unfix" {
					t.Fatalf("unfinished: %+v", open)
				}
				if did, err := RecoverUnfix(context.Background(), g, &open[0], store); err != nil || did != "undone" {
					t.Fatalf("recover: %q %v", did, err)
				}
				sameContent(t, before, content(t, r.Worktree))
			})
		}
	}
}
