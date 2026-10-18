package fix

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// content maps every path under the repository (git directory included) to
// its kind, permissions and content hash: what undo must give back.
func content(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, _ := d.Info()
		v := info.Mode().String()
		if d.Type().IsRegular() {
			b, _ := os.ReadFile(p)
			sum := sha256.Sum256(b)
			v += " " + hex.EncodeToString(sum[:])
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = v
		return nil
	})
	return out
}

func sameContent(t *testing.T, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s: was %q, now %q", k, v, after[k])
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s: new", k)
		}
	}
}

func fixOptions(t *testing.T, worktree string) (Options, journal.Store) {
	home := t.TempDir()
	dirs, err := state.Resolve(home, "linux", func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	o := Options{Path: worktree, Dirs: dirs, Env: change.Env{Home: home, GOOS: "linux", Flags: scan.DefaultFlagReader(),
		Version: "test", Procs: func(context.Context) ([]procs.Proc, error) { return nil, nil }}}
	return o, journal.Store{Dir: dirs.Journal()}
}

var repairable = []string{"index-copies", "orphan-index-lock", "ref-copy-older", "ref-copy-newer", "dropbox-copy",
	"index-missing", "dir-copy", "syncthing-copy", "desktop-ini", "empty-ref",
	"stale-rebase", "gitdir-copy", "nosync-unignored"}

func TestFixThenUndoEachScenario(t *testing.T) {
	for _, name := range repairable {
		t.Run(name, func(t *testing.T) {
			g, r, _ := scenario(t, name)
			before := content(t, r.Worktree)
			o, store := fixOptions(t, r.Worktree)
			p, err := MakePlan(context.Background(), g, o)
			if err != nil || len(p.Refusals) > 0 || len(p.Actions) == 0 {
				t.Fatalf("plan: %+v %v", p, err)
			}
			e, err := Apply(context.Background(), g, p, o, store)
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			rep, err := change.RunCheck(context.Background(), g, o.Env, r.Worktree)
			if err != nil {
				t.Fatal(err)
			}
			if check.ExitCode(rep) != 0 {
				t.Fatalf("damage left after the fix: %+v", rep.Findings)
			}
			if kept, err := Reverse(context.Background(), g, r, e, store); err != nil || len(kept) > 0 {
				t.Fatalf("undo: %v, kept %v", err, kept)
			}
			sameContent(t, before, content(t, r.Worktree))
		})
	}
}

func TestRescueKeepsTheCopysCommits(t *testing.T) {
	g, r, _ := scenario(t, "ref-copy-newer")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	res, _ := g.Run(context.Background(), r.Worktree, r.Env(), "rev-parse", "refs/gcd-rescue/heads/main-2")
	if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) != p.Actions[0].Value {
		t.Fatalf("rescue ref: %q", res.Stdout)
	}
}

type crash struct{}

// A crash right after any step changed the disk (its Done not saved yet),
// or after the last step before the closing check: recovery must give the
// repository back exactly as it was.
func TestRecoverAfterACrashAtEveryStep(t *testing.T) {
	for _, name := range []string{"index-missing", "ref-copy-newer", "orphan-index-lock", "empty-ref", "nosync-unignored", "index-copies"} {
		g0, r0, _ := scenario(t, name)
		o0, _ := fixOptions(t, r0.Worktree)
		p0, _ := MakePlan(context.Background(), g0, o0)
		for i := 0; i <= len(p0.Actions); i++ {
			point := "applied"
			if i < len(p0.Actions) {
				point = strconv.Itoa(i+1) + "-" + p0.Actions[i].Kind
			}
			t.Run(name+"/"+point, func(t *testing.T) {
				g, r, _ := scenario(t, name)
				before := content(t, r.Worktree)
				o, store := fixOptions(t, r.Worktree)
				p, _ := MakePlan(context.Background(), g, o)
				n := 0
				defer func() { stepHook = func(string) {} }()
				stepHook = func(string) {
					if n == i {
						panic(crash{})
					}
					n++
				}
				store.AfterSave = func(ph journal.Phase) {
					if ph == journal.Applied && i == len(p.Actions) {
						panic(crash{})
					}
				}
				func() {
					defer func() {
						if v := recover(); v != nil {
							if _, ok := v.(crash); !ok {
								panic(v)
							}
						}
					}()
					Apply(context.Background(), g, p, o, store)
					t.Fatal("no crash")
				}()
				open, err := store.Unfinished(p.RepoID)
				if err != nil || len(open) != 1 {
					t.Fatalf("unfinished: %v %v", open, err)
				}
				if did, err := Recover(context.Background(), g, &open[0], o, store); err != nil || did != "rolled back" {
					t.Fatalf("recover: %q %v", did, err)
				}
				sameContent(t, before, content(t, r.Worktree))
			})
		}
	}
}

func TestBusyRepositoryIsRefused(t *testing.T) {
	g, r, _ := scenario(t, "index-copies")
	o, _ := fixOptions(t, r.Worktree)
	os.WriteFile(filepath.Join(r.CommonDir, "MERGE_HEAD"), []byte(strings.Repeat("a", 40)+"\n"), 0o644)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || len(p.Refusals) == 0 || p.Refusals[0].Code != change.RefuseBusy {
		t.Fatalf("%+v %v", p.Refusals, err)
	}
}

// An editor's git uses the rebuilt index at once and is still running when
// fix looks before its next step. fix stops and reverses: the index git used
// stays as it is, the older rescue is reversed, and the journal ends.
// Nothing is left for a later run to trip on.
func TestEditorGitDuringTheFixDoesNotWedgeTheJournal(t *testing.T) {
	g, r, _ := scenario(t, "index-missing")
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "only in the copy")
	write(t, r, "refs/heads/main 2", c+"\n")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "rescue-ref,rebuild-index,quarantine" {
		t.Fatalf("plan %s %v", kinds(p.Actions), err)
	}
	editor, used := false, ""
	defer func() { stepHook = func(string) {} }()
	stepHook = func(kind string) {
		if kind == RebuildIndex {
			git(t, g, r, "", "update-index", "-q", "--refresh")
			editor, used = true, content(t, r.Worktree)[".git/index"]
		}
	}
	o.Procs = func(context.Context) ([]procs.Proc, error) {
		if editor {
			return []procs.Proc{{PID: 4242, Cwd: r.Worktree}}, nil
		}
		return nil, nil
	}
	_, err = Apply(context.Background(), g, p, o, store)
	if err == nil || !strings.Contains(err.Error(), "rolled back except") {
		t.Fatalf("apply: %v", err)
	}
	if open, _ := store.Unfinished(p.RepoID); len(open) != 0 {
		t.Fatalf("left unfinished: %+v", open)
	}
	if got := content(t, r.Worktree)[".git/index"]; got != used {
		t.Fatal("the index git used was replaced")
	}
	if res, _ := g.Run(context.Background(), r.Worktree, r.Env(), "rev-parse", "--verify", "-q", p.Actions[0].Ref); res.ExitCode == 0 {
		t.Fatal("the rescue ref was not reversed")
	}
}

// The sync client brings back a copy fix moved away, then fix is killed:
// recovery leaves both, the copy in the backup and the one that came back,
// and ends.
func TestRecoveryKeepsWhatCameBack(t *testing.T) {
	g, r, _ := scenario(t, "index-copies")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	store.AfterSave = func(ph journal.Phase) {
		if ph == journal.Applied {
			panic(crash{})
		}
	}
	func() {
		defer func() { recover() }()
		Apply(context.Background(), g, p, o, store)
	}()
	b, _ := os.ReadFile(filepath.Join(r.GitDir, "index"))
	os.WriteFile(filepath.Join(r.GitDir, "index 2"), b, 0o644)
	open, _ := store.Unfinished(p.RepoID)
	if len(open) != 1 {
		t.Fatalf("unfinished: %+v", open)
	}
	did, err := Recover(context.Background(), g, &open[0], o, store)
	if err != nil || !strings.Contains(did, "exists again") {
		t.Fatalf("recover: %q %v", did, err)
	}
	if left, _ := store.Unfinished(p.RepoID); len(left) != 0 {
		t.Fatalf("still unfinished: %+v", left)
	}
	if !exists(open[0].Steps[0].Backup) {
		t.Fatal("the copy in the backup is gone")
	}
}

// fix is killed after rebuilding the index and the user runs git status,
// which rewrites it: recovery keeps the index git wrote and ends.
func TestRecoveryAfterGitUsedTheRebuiltIndex(t *testing.T) {
	g, r, _ := scenario(t, "index-missing")
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	store.AfterSave = func(ph journal.Phase) {
		if ph == journal.Applied {
			panic(crash{})
		}
	}
	func() {
		defer func() { recover() }()
		Apply(context.Background(), g, p, o, store)
	}()
	git(t, g, r, "", "update-index", "-q", "--refresh")
	index := content(t, r.Worktree)[".git/index"]
	open, _ := store.Unfinished(p.RepoID)
	did, err := Recover(context.Background(), g, &open[0], o, store)
	if err != nil || !strings.HasPrefix(did, "rolled back except") {
		t.Fatalf("recover: %q %v", did, err)
	}
	if content(t, r.Worktree)[".git/index"] != index {
		t.Fatal("recovery replaced the index git wrote")
	}
}

// A copy of an annotated tag names a tag object no ref points at (its commit
// only the tag reaches): the tag object is kept, and with it the commit.
func TestRescueKeepsAnAnnotatedTag(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	git(t, g, r, "", "tag", "-a", "v1", "-m", "here", "HEAD")
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "only on the other device")
	tag := git(t, g, r, "object "+c+"\ntype commit\ntag v1\ntagger t <t@example.invalid> 1767225600 +0000\n\nthere\n", "mktag")
	write(t, r, "refs/tags/v1 2", tag+"\n")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "rescue-ref,quarantine" || p.Actions[0].Value != tag {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	if lost := git(t, g, r, "", "fsck", "--unreachable", "--no-reflogs"); lost != "" {
		t.Fatalf("unreachable after the fix:\n%s", lost)
	}
}

// git read-tree was stopped half way (its lock is left beside the index fix
// was building): the recovery sets the lock aside with the rest.
func TestRecoverSetsAsideAReadTreeLock(t *testing.T) {
	g, r, _ := scenario(t, "index-missing")
	before := content(t, r.Worktree)
	o, store := fixOptions(t, r.Worktree)
	p, _ := MakePlan(context.Background(), g, o)
	defer func() { stepHook = func(string) {} }()
	stepHook = func(kind string) {
		if kind == RebuildIndex {
			os.WriteFile(newIndex(filepath.Join(r.GitDir, "index"))+".lock", []byte("half"), 0o644)
			panic(crash{})
		}
	}
	func() {
		defer func() { recover() }()
		Apply(context.Background(), g, p, o, store)
		t.Fatal("no crash")
	}()
	open, _ := store.Unfinished(p.RepoID)
	if did, err := Recover(context.Background(), g, &open[0], o, store); err != nil || did != "rolled back" {
		t.Fatalf("recover: %q %v", did, err)
	}
	sameContent(t, before, content(t, r.Worktree))
}

// The backup directory on another disk than the repository: what fix moves
// aside could not move in one step, so fix refuses.
func TestOtherFilesystemIsRefused(t *testing.T) {
	g, r, _ := scenario(t, "index-copies")
	o, _ := fixOptions(t, r.Worktree)
	if sameDevice(r.CommonDir, "/dev") {
		t.Skip("/dev is on the same filesystem as the temporary directory here")
	}
	// planning writes nothing: the directory need not exist, its parent's disk counts
	o.Dirs = state.Dirs{State: "/dev/gitbrella-test/state", GitDirs: "/dev/gitbrella-test/gitdirs"}
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || len(p.Refusals) != 1 || p.Refusals[0].Code != "OTHER_FILESYSTEM" {
		t.Fatalf("%+v %v", p.Refusals, err)
	}
}

// A Syncthing conflict copy of a branch is a valid ref name, so git lists it
// as a branch; it must not count as what keeps its own commit alive.
func TestSyncthingRefCopyIsRescued(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "device B")
	write(t, r, "refs/heads/main.sync-conflict-20261014-120000-ABCDEF1", c+"\n")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "rescue-ref,quarantine" || p.Actions[0].Value != c {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	if lost := git(t, g, r, "", "fsck", "--unreachable", "--no-reflogs"); lost != "" {
		t.Fatalf("unreachable after the fix:\n%s", lost)
	}
}

// HEAD's commit object is empty (left to the user): the unrelated repair of
// a leftover rebase directory still lands instead of being reversed every
// time.
func TestAnEmptyHeadObjectDoesNotBlockOtherRepairs(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	os.Mkdir(filepath.Join(r.CommonDir, "rebase-merge"), 0o755)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(r.CommonDir, "rebase-merge"), old, old)
	h := git(t, g, r, "", "rev-parse", "HEAD")
	obj := filepath.Join(r.CommonDir, "objects", h[:2], h[2:])
	os.Chmod(obj, 0o644)
	os.WriteFile(obj, nil, 0o644)
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "quarantine" {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
}

// The sync client writes the branch after the plan (no process list shows a
// sync client): fix must not overwrite the new value with the reflog's.
func TestSetRefStopsWhenTheBranchChangedSinceThePlan(t *testing.T) {
	g, r, _ := scenario(t, "empty-ref")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "set-ref" {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	c := git(t, g, r, "", "commit-tree", git(t, g, r, "", "write-tree"), "-m", "device B")
	branch := filepath.Join(r.CommonDir, "refs", "heads", "main")
	os.WriteFile(branch, []byte(c+"\n"), 0o644)
	if _, err := Apply(context.Background(), g, p, o, store); err == nil || !strings.Contains(err.Error(), "changed since the plan") {
		t.Fatalf("apply: %v", err)
	}
	if b, _ := os.ReadFile(branch); string(b) != c+"\n" {
		t.Fatalf("branch now %q", b)
	}
}

// A copy named by one of the newer clients' schemes (Nextcloud here) goes the
// same way as an iCloud one: its commit is rescued first.
func TestRescueKeepsANextcloudRefCopy(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "on the other device")
	write(t, r, "refs/heads/main (conflicted copy jane 2026-10-14 120000)", c+"\n")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "rescue-ref,quarantine" || p.Actions[0].Value != c {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	if lost := git(t, g, r, "", "fsck", "--unreachable", "--no-reflogs"); lost != "" {
		t.Fatalf("unreachable after the fix:\n%s", lost)
	}
}

// Nextcloud's older copy name ("main_conflict-...") is a valid ref name, and
// in a Nextcloud folder the check reads it as a copy: the rescue ref must not
// keep it, or the closing check takes the rescue for a copy and rolls back.
func TestRescueOfAnOldNextcloudRefCopy(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	if err := os.WriteFile(filepath.Join(filepath.Dir(r.Worktree), ".sync_ab12cd34ef56.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "on the other device")
	write(t, r, "refs/heads/main_conflict-20261014-120000", c+"\n")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "rescue-ref,quarantine" || p.Actions[0].Value != c {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatalf("%v (rescue ref %s)", err, p.Actions[0].Ref)
	}
	if lost := git(t, g, r, "", "fsck", "--unreachable", "--no-reflogs"); lost != "" {
		t.Fatalf("unreachable after the fix:\n%s", lost)
	}
}
