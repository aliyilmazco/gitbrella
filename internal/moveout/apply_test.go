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

// healthy asserts that git works in the lab with the HEAD and status it had.
func (l *lab) healthy(head, status string) {
	l.t.Helper()
	if got := l.git("rev-parse", "HEAD"); got != head {
		l.t.Fatalf("HEAD %s, want %s", got, head)
	}
	if got := l.git("status", "--porcelain"); got != status {
		l.t.Fatalf("status %q, want %q", got, status)
	}
}

func TestApplyMovesOut(t *testing.T) {
	l := newLab(t)
	head, st := l.git("rev-parse", "HEAD"), l.git("status", "--porcelain")
	p := l.plan()
	e, err := Apply(context.Background(), l.g, p, l.opts, l.store)
	if err != nil {
		t.Fatal(err)
	}
	if e.Phase != journal.Done {
		t.Fatalf("phase %s", e.Phase)
	}
	b, _ := os.ReadFile(filepath.Join(l.worktree, ".git"))
	if string(b) != p.Gitfile {
		t.Fatalf(".git is %q, want %q", b, p.Gitfile)
	}
	if !exists(filepath.Join(e.Data["backup"], "HEAD")) {
		t.Fatal("the original is not in the backup")
	}
	l.healthy(head, st)
	if got := l.git("rev-parse", "--absolute-git-dir"); got != p.Target {
		t.Fatalf("git dir %s, want %s", got, p.Target)
	}
	l.git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "after the move")
	if again := l.plan(); !again.Already {
		t.Fatalf("second plan: %+v", again)
	}
}

func TestApplyRollsBackWhenTheRepositoryChangedDuringTheCopy(t *testing.T) {
	l := newLab(t)
	head, st := l.git("rev-parse", "HEAD"), l.git("status", "--porcelain")
	p := l.plan()
	defer func() { stepHook = func(string) {} }()
	stepHook = func(step string) {
		if step == "copied" { // a git writes between the copy and the switch
			os.WriteFile(filepath.Join(l.worktree, ".git", "refs", "heads", "late"), []byte(head+"\n"), 0o644)
		}
	}
	e, err := Apply(context.Background(), l.g, p, l.opts, l.store)
	if err == nil || !strings.Contains(err.Error(), "changed while it was copied") {
		t.Fatalf("err = %v", err)
	}
	if e.Phase != journal.RolledBack || exists(p.Target) {
		t.Fatalf("phase %s, target left: %v", e.Phase, exists(p.Target))
	}
	if info, err := os.Lstat(filepath.Join(l.worktree, ".git")); err != nil || !info.IsDir() {
		t.Fatal(".git is not the directory it was")
	}
	l.healthy(head, st)
}

type crash struct{}

// crashAt runs Apply and stops it, like a killed process, at the journal
// phase or disk step named point.
func (l *lab) crashAt(point string) *journal.Entry {
	l.t.Helper()
	p := l.plan()
	store := l.store
	store.AfterSave = func(ph journal.Phase) {
		if string(ph) == point {
			panic(crash{})
		}
	}
	defer func() { stepHook = func(string) {} }()
	stepHook = func(step string) {
		if "step:"+step == point {
			panic(crash{})
		}
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(crash); !ok {
					panic(r)
				}
			}
		}()
		Apply(context.Background(), l.g, p, l.opts, store)
		l.t.Fatalf("Apply did not stop at %s", point)
	}()
	open, err := l.store.Unfinished(p.RepoID)
	if err != nil || len(open) != 1 {
		l.t.Fatalf("after a crash at %s: unfinished %v %v", point, open, err)
	}
	return &open[0]
}

func TestRecoverAfterACrashAnywhere(t *testing.T) {
	for _, tc := range []struct{ point, want string }{
		{"planned", "rolled back"},
		{"step:copied", "rolled back"},
		{"copied", "rolled back"},
		{"step:original-moved", "rolled back"}, // the original is aside and no .git at all
		{"original-moved", "rolled back"},
		{"step:gitfile-written", "finished"}, // the .git file is there, its phase not saved
		{"gitfile-written", "finished"},
	} {
		t.Run(tc.point, func(t *testing.T) {
			l := newLab(t)
			head, st := l.git("rev-parse", "HEAD"), l.git("status", "--porcelain")
			e := l.crashAt(tc.point)
			did, err := Recover(context.Background(), l.g, e, l.store)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(did, tc.want) {
				t.Fatalf("recovery %s, want %s", did, tc.want)
			}
			l.healthy(head, st)
			info, _ := os.Lstat(filepath.Join(l.worktree, ".git"))
			if moved := !info.IsDir(); moved != (tc.want == "finished") {
				t.Fatalf(".git is a file: %v", moved)
			}
			if open, _ := l.store.Unfinished(e.RepoID); len(open) != 0 {
				t.Fatalf("still unfinished: %+v", open)
			}
		})
	}
}

func TestRollBackRefusesWhatItDidNotMake(t *testing.T) {
	l := newLab(t)
	e := l.crashAt("original-moved")
	os.WriteFile(filepath.Join(l.worktree, ".git"), []byte("gitdir: /somewhere/else\n"), 0o644)
	if _, err := Recover(context.Background(), l.g, e, l.store); err == nil {
		t.Fatal("recovery replaced a .git file it did not write")
	}
	if !exists(e.Data["backup"]) {
		t.Fatal("the backup is gone")
	}
}

// After the switch git works through the .git file; a commit made then lives
// only in the moved directory. If recovery cannot use it (here the directory
// is out of reach), it must stop with both paths named, never put the old
// original back.
func TestRecoverStopsWhenTheMovedDirectoryHasNewHistory(t *testing.T) {
	l := newLab(t)
	e := l.crashAt("gitfile-written")
	l.git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "made after the crash")
	newHead := l.git("rev-parse", "HEAD")
	os.Rename(e.Data["target"], e.Data["target"]+".away")
	_, err := Recover(context.Background(), l.g, e, l.store)
	if err == nil || !strings.Contains(err.Error(), e.Data["backup"]) {
		t.Fatalf("recovery: %v", err)
	}
	os.Rename(e.Data["target"]+".away", e.Data["target"])
	if got := l.git("rev-parse", "HEAD"); got != newHead {
		t.Fatalf("HEAD %s, want the commit made since (%s)", got, newHead)
	}
}

// A path relative to .git (core.hooksPath=.git/hooks-x) stops resolving once
// .git is a file: the move is rolled back instead of silently losing a hook.
func TestApplyRollsBackWhenAPathRelativeToGitStopsResolving(t *testing.T) {
	l := newLab(t)
	os.MkdirAll(filepath.Join(l.worktree, ".git", "hooks-x"), 0o755)
	l.git("config", "core.hooksPath", ".git/hooks-x")
	head := l.git("rev-parse", "HEAD")
	e, err := Apply(context.Background(), l.g, l.plan(), l.opts, l.store)
	if err == nil || !strings.Contains(err.Error(), "hooks") {
		t.Fatalf("apply: %v", err)
	}
	if e.Phase != journal.RolledBack {
		t.Fatalf("phase %s", e.Phase)
	}
	if info, err := os.Lstat(filepath.Join(l.worktree, ".git")); err != nil || !info.IsDir() {
		t.Fatal(".git is not the directory again")
	}
	if got := l.git("rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD %s", got)
	}
}

// A .git directory the sync client brought back after a move: two exist, and
// a second move-out would bury the history of the first; it is refused.
func TestAReturnedGitDirectoryBlocksAnotherMove(t *testing.T) {
	l := newLab(t)
	p := l.plan()
	e, err := Apply(context.Background(), l.g, p, l.opts, l.store)
	if err != nil {
		t.Fatal(err)
	}
	os.Rename(filepath.Join(l.worktree, ".git"), filepath.Join(l.home, "gitfile-aside"))
	if out, err := exec.Command("cp", "-R", e.Data["backup"], filepath.Join(l.worktree, ".git")).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if got := codes(l.plan().Refusals); !strings.Contains(got, RefuseBusy) {
		t.Fatalf("refusals %q", got)
	}
}

// R1. After the switch (the .git file is written), the git directory in use is
// the copy, and commits go there. A recovery whose verify fails for any reason
// (here: the signal context is cancelled, as Ctrl-C during recovery does)
// rolls back: the stale original returns as .git and the copy holding the new
// commit is set aside, reported as "rolled back".
func TestRecoverNeverRollsBackOverNewerHistory(t *testing.T) {
	l := newLab(t)
	e := l.crashAt("gitfile-written")
	l.git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "made after the crash")
	newHead := l.git("rev-parse", "HEAD")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	did, err := Recover(ctx, l.g, e, l.store)
	t.Logf("recovery: %q %v", did, err)
	if got := l.git("rev-parse", "HEAD"); got != newHead {
		t.Fatalf("after recovery HEAD is %s, the commit made since the switch (%s) is gone from the repository", got, newHead)
	}
}

// R2. ours() decides by content, and every move-out of a repository writes the
// same bytes. A second move-out whose plan was made before the first finished
// (two runs whose path spellings give different lock ids, see R4) copies the
// .git FILE, treecopy.Compare calls a file identical to anything, the rename
// onto the live target fails, and its roll-back takes the first move's .git
// file for its own: it moves it aside and then cannot restore anything.
func TestRollBackLeavesAFinishedMoveAlone(t *testing.T) {
	l := newLab(t)
	head := l.git("rev-parse", "HEAD")
	p1, p2 := l.plan(), l.plan()
	if _, err := Apply(context.Background(), l.g, p1, l.opts, l.store); err != nil {
		t.Fatal(err)
	}
	_, err := Apply(context.Background(), l.g, p2, l.opts, l.store)
	t.Logf("second apply: %v", err)
	if _, err := os.Lstat(filepath.Join(l.worktree, ".git")); err != nil {
		t.Fatalf("the finished move lost its .git: %v", err)
	}
	if got := l.git("rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD %s", got)
	}
}

// R3. rollBack sets aside e.Data["target"] whenever it exists, also when this
// change never created it (here: it appeared between plan and apply).
func TestRollBackLeavesATargetItDidNotMake(t *testing.T) {
	l := newLab(t)
	p := l.plan()
	if err := os.MkdirAll(filepath.Join(p.Target, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(p.Target, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	e, err := Apply(context.Background(), l.g, p, l.opts, l.store)
	t.Logf("apply: %v", err)
	if !exists(p.Target) {
		t.Fatalf("roll-back moved %s, which it did not make, to %s", p.Target, filepath.Dir(e.Data["backup"]))
	}
}

// R6. A move-out killed after .git went aside, then the user (or the sync
// client) puts a .git directory back: recovery refuses ("leaving both") and
// stays unfinished for ever, so every change command exits 2 and check
// reports CHANGE_UNFINISHED (blocking commits under the hook) with no
// command that resolves it.
func TestRecoverHasAWayOutWhenBothExist(t *testing.T) {
	l := newLab(t)
	e := l.crashAt("original-moved")
	l.git("init", "-q", "-b", "main") // what a user does at "not a git repository"
	_, err := Recover(context.Background(), l.g, e, l.store)
	// The way out is the message: both paths named, what to do, nothing overwritten.
	if err == nil || !strings.Contains(err.Error(), e.Data["backup"]) || !strings.Contains(err.Error(), "move-out --apply") {
		t.Fatalf("recovery with both: %v", err)
	}
	if !exists(e.Data["backup"]) {
		t.Fatal("the original left the backup")
	}
}

// R7. Needs the one-line seam stepHook("compared") just before
// rename(dotgit, backup) in apply.go. A git that writes after the last
// Compare and before the rename (a fetch, a tag, a branch, a config change:
// anything that leaves HEAD and status alone) lands in the backup only; the
// strict verify cannot see it, and the move is reported done.
func TestNoWriteIsLostBetweenTheLastCompareAndTheSwitch(t *testing.T) {
	l := newLab(t)
	head := l.git("rev-parse", "HEAD")
	p := l.plan()
	defer func() { stepHook = func(string) {} }()
	stepHook = func(step string) {
		if step == "compared" {
			os.WriteFile(filepath.Join(l.worktree, ".git", "refs", "heads", "late"), []byte(head+"\n"), 0o644)
			l.git("config", "remote.origin.url", "https://example.invalid/late.git")
		}
	}
	e, err := Apply(context.Background(), l.g, p, l.opts, l.store)
	if err != nil {
		t.Logf("apply refused: %v", err)
		return
	}
	if e.Phase == journal.Done && !exists(filepath.Join(p.Target, "refs", "heads", "late")) {
		t.Fatalf("done, but refs/heads/late and the config change exist only in the backup %s", e.Data["backup"])
	}
}
