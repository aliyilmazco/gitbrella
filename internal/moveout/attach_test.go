package moveout

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/journal"
)

// withRemote gives the lab a bare remote holding its commit.
func (l *lab) withRemote() string {
	l.t.Helper()
	remote := filepath.Join(l.home, "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", remote).CombinedOutput(); err != nil {
		l.t.Fatalf("%v %s", err, out)
	}
	l.git("remote", "add", "origin", remote)
	l.git("push", "-q", "-u", "origin", "main")
	return remote
}

// inodes records identity and times of the working-tree files.
func (l *lab) inodes() map[string][3]int64 {
	l.t.Helper()
	out := map[string][3]int64{}
	filepath.Walk(l.worktree, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.Name() == ".git" {
			return err
		}
		st := info.Sys().(*syscall.Stat_t)
		out[p] = [3]int64{int64(st.Ino), info.ModTime().UnixNano(), info.Size()}
		return nil
	})
	return out
}

// otherDevice plays what the sync client does on a device the move has not
// reached: the .git directory disappears and the .git file arrives.
func (l *lab) otherDevice(gitfileContent string) {
	l.t.Helper()
	gone := filepath.Join(l.home, "deleted-by-sync")
	if err := os.Rename(filepath.Join(l.worktree, ".git"), gone); err != nil {
		l.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l.worktree, ".git"), []byte(gitfileContent), 0o644); err != nil {
		l.t.Fatal(err)
	}
}

func (l *lab) planAttach(from, branch string) AttachPlan {
	l.t.Helper()
	a, err := PlanAttach(context.Background(), l.g, l.opts, from, branch, l.store)
	if err != nil {
		l.t.Fatal(err)
	}
	return a
}

func TestPrepareThenAttach(t *testing.T) {
	l := newLab(t)
	head := l.git("rev-parse", "HEAD")
	p := l.plan()
	e, err := Prepare(context.Background(), p, l.opts, l.store)
	if err != nil || e.Phase != journal.Done {
		t.Fatalf("prepare: %v %+v", err, e)
	}
	if info, err := os.Lstat(filepath.Join(l.worktree, ".git")); err != nil || !info.IsDir() {
		t.Fatal("prepare switched .git")
	}
	if again := l.plan(); codes(again.Refusals) != RefuseTargetExists {
		t.Fatalf("a prepared device would move out too: %+v", again.Refusals)
	}
	l.otherDevice(p.Gitfile) // the move happened on another device
	before := l.inodes()
	ap := l.planAttach("", "")
	if len(ap.Refusals) != 0 || ap.Source != "prepared" || ap.Target != p.Target {
		t.Fatalf("attach plan: %+v", ap)
	}
	if _, err := Attach(context.Background(), l.g, ap, l.opts, l.store); err != nil {
		t.Fatalf("attach: %v", err)
	}
	l.healthy(head, "M a.md")
	if after := l.inodes(); len(after) != len(before) {
		t.Fatal("attach changed the working tree")
	}
}

func TestAttachFromRemote(t *testing.T) {
	l := newLab(t)
	remote := l.withRemote()
	head := l.git("rev-parse", "HEAD")
	p := l.plan()
	if _, err := Apply(context.Background(), l.g, p, l.opts, l.store); err != nil {
		t.Fatal(err)
	}
	// This device never had the git directory: hide the one Apply made.
	os.Rename(p.Target, p.Target+".device-a")
	if q := l.plan(); codes(q.Refusals) != RefuseGitDirMissing {
		t.Fatalf("plan on the other device: %+v", q.Refusals)
	}
	if ap := l.planAttach("", ""); codes(ap.Refusals) != RefuseNeedSource {
		t.Fatalf("attach without a source: %+v", ap.Refusals)
	}
	before := l.inodes()
	e, err := Attach(context.Background(), l.g, l.planAttach(remote, "main"), l.opts, l.store)
	if err != nil || e.Phase != journal.Done {
		t.Fatalf("attach: %v %+v", err, e)
	}
	l.healthy(head, "M a.md") // the edit the sync client brought shows as a change
	after := l.inodes()
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("attach touched %s", k)
		}
	}
	if got := l.git("rev-parse", "--abbrev-ref", "@{u}"); got != "origin/main" {
		t.Fatalf("upstream %s", got)
	}
}

func TestAttachRefusesWhereNothingMoved(t *testing.T) {
	l := newLab(t)
	if ap := l.planAttach("", ""); codes(ap.Refusals) != RefuseNotMoved {
		t.Fatalf("attach on a .git directory: %+v", ap.Refusals)
	}
}

func TestAttachSetsTheCloneAsideWhenItFails(t *testing.T) {
	l := newLab(t)
	p := l.plan()
	Apply(context.Background(), l.g, p, l.opts, l.store)
	os.Rename(p.Target, p.Target+".device-a")
	e, err := Attach(context.Background(), l.g, l.planAttach(filepath.Join(l.home, "no-such-remote.git"), ""), l.opts, l.store)
	if err == nil || e.Phase != journal.RolledBack || exists(p.Target) {
		t.Fatalf("err %v, entry %+v, target left %v", err, e, exists(p.Target))
	}
}

// The .git file comes through the sync client: attach trusts it only as far
// as this device's own records go.
func TestAttachDoesNotTrustTheSyncedGitfile(t *testing.T) {
	l := newLab(t)
	other := filepath.Join(l.home, "code", "other")
	if out, err := exec.Command("git", "init", "-q", other).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	l.otherDevice("gitdir: ../../../../code/other/.git\n") // somewhere else on this device
	if ap := l.planAttach("", ""); codes(ap.Refusals) != RefuseUnexpectedTarget {
		t.Fatalf("a .git file into another repository: %+v", ap.Refusals)
	}
	l2 := newLab(t)
	p := l2.plan()
	os.MkdirAll(p.Target, 0o755) // a directory there that no prepare made
	exec.Command("git", "init", "-q", "--bare", p.Target).Run()
	l2.otherDevice(p.Gitfile)
	if ap := l2.planAttach("", ""); codes(ap.Refusals) != RefuseUnknownCopy {
		t.Fatalf("an unprepared directory: %+v", ap.Refusals)
	}
}

// Prepare and attach killed at every save: recovery leaves this device with
// its own .git directory untouched, or attached, never half way.
func TestRecoverPrepareAndAttachAfterACrash(t *testing.T) {
	for _, point := range []string{"planned", "copied"} {
		t.Run("prepare/"+point, func(t *testing.T) {
			l := newLab(t)
			head := l.git("rev-parse", "HEAD")
			p := l.plan()
			store := l.store
			store.AfterSave = func(ph journal.Phase) {
				if string(ph) == point {
					panic(crash{})
				}
			}
			func() { defer func() { recover() }(); Prepare(context.Background(), p, l.opts, store) }()
			open, _ := l.store.Unfinished(p.RepoID)
			if len(open) != 1 {
				t.Fatalf("unfinished %+v", open)
			}
			if _, err := RecoverAttach(context.Background(), l.g, &open[0], l.store); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Lstat(filepath.Join(l.worktree, ".git")); err != nil || !info.IsDir() {
				t.Fatal("prepare recovery touched .git")
			}
			l.healthy(head, "M a.md")
		})
	}
	for _, point := range []string{"planned", "copied"} {
		t.Run("attach-from-remote/"+point, func(t *testing.T) {
			l := newLab(t)
			remote := l.withRemote()
			head := l.git("rev-parse", "HEAD")
			p := l.plan()
			if _, err := Apply(context.Background(), l.g, p, l.opts, l.store); err != nil {
				t.Fatal(err)
			}
			os.Rename(p.Target, p.Target+".device-a")
			ap := l.planAttach(remote, "main")
			store := l.store
			store.AfterSave = func(ph journal.Phase) {
				if string(ph) == point {
					panic(crash{})
				}
			}
			func() { defer func() { recover() }(); Attach(context.Background(), l.g, ap, l.opts, store) }()
			open, _ := l.store.Unfinished(ap.RepoID)
			if len(open) != 1 {
				t.Fatalf("unfinished %+v", open)
			}
			did, err := RecoverAttach(context.Background(), l.g, &open[0], l.store)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(did, "finished") {
				l.healthy(head, "M a.md")
			} else if exists(p.Target) {
				t.Fatalf("rolled back but %s is still there", p.Target)
			}
		})
	}
}

// R8. Attach with a prepared copy is killed after Begin and before Done. The
// entry has no "backup" key, so setAside's aside is filepath.Dir("") = ".",
// the process's working directory, and the "abandoned-copy" rule moves the
// LIVE git directory there: into the synced working tree when run from it.
func TestRecoverAttachKeepsThePreparedCopy(t *testing.T) {
	l := newLab(t)
	head := l.git("rev-parse", "HEAD")
	p := l.plan()
	if _, err := Prepare(context.Background(), p, l.opts, l.store); err != nil {
		t.Fatal(err)
	}
	l.otherDevice(p.Gitfile)
	ap := l.planAttach("", "")
	store := l.store
	store.AfterSave = func(ph journal.Phase) {
		if ph == journal.Planned {
			panic(crash{})
		}
	}
	func() {
		defer func() { recover() }()
		Attach(context.Background(), l.g, ap, l.opts, store)
	}()
	open, _ := l.store.Unfinished(ap.RepoID)
	if len(open) != 1 {
		t.Fatalf("unfinished %+v", open)
	}
	t.Chdir(l.worktree) // the CLI runs from the working tree
	did, err := RecoverAttach(context.Background(), l.g, &open[0], l.store)
	t.Logf("recovery %q %v", did, err)
	if !exists(p.Target) || exists(filepath.Join(l.worktree, "abandoned-copy")) {
		t.Fatalf("the prepared git directory left %s (now in the synced folder: %v)", p.Target, exists(filepath.Join(l.worktree, "abandoned-copy")))
	}
	if got := l.git("rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD %s", got)
	}
}
