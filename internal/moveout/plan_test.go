package moveout

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/icloud"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

func TestPlanCleanRepository(t *testing.T) {
	l := newLab(t)
	p := l.plan()
	if len(p.Refusals) != 0 {
		t.Fatalf("refusals: %+v", p.Refusals)
	}
	want := filepath.Join(l.home, ".local", "share", "gitbrella", "gitdirs", p.RepoID)
	if p.Target != want || !strings.HasPrefix(p.RepoID, "my-notes-") {
		t.Fatalf("target %s (id %s), want %s", p.Target, p.RepoID, want)
	}
	if p.Gitfile != "gitdir: ../../../../.local/share/gitbrella/gitdirs/"+p.RepoID+"\n" {
		t.Fatalf("gitfile %q", p.Gitfile)
	}
	if p.Head == "" || p.Bytes == 0 || p.Repo.Provider != "icloud" || p.Already {
		t.Fatalf("plan %+v", p)
	}
	if _, err := os.Stat(l.opts.Dirs.State); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("planning wrote the state directory")
	}
}

func TestPlanRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		setup func(l *lab)
		code  string
	}{
		"not synced": {func(l *lab) { l.opts.Home = filepath.Join(l.home, "elsewhere") }, RefuseNotSynced},
		"git running": {func(l *lab) {
			l.opts.Procs = func(context.Context) ([]procs.Proc, error) {
				return []procs.Proc{{PID: 4242, Cwd: l.worktree, Start: time.Now().Add(-time.Hour)}}, nil
			}
		}, RefuseBusy},
		"lock held": {func(l *lab) {
			os.WriteFile(filepath.Join(l.worktree, ".git", "index.lock"), nil, 0o644)
		}, RefuseBusy},
		"merge in progress": {func(l *lab) {
			os.WriteFile(filepath.Join(l.worktree, ".git", "MERGE_HEAD"), []byte(l.git("rev-parse", "HEAD")+"\n"), 0o644)
		}, RefuseBusy},
		"linked worktree": {func(l *lab) { l.git("worktree", "add", "-q", filepath.Join(l.home, "wt"), "-b", "x") }, RefuseLinkedWorktrees},
		"submodules":      {func(l *lab) { os.MkdirAll(filepath.Join(l.worktree, ".git", "modules", "lib"), 0o755) }, RefuseSubmodules},
		"alternates": {func(l *lab) {
			os.WriteFile(filepath.Join(l.worktree, ".git", "objects", "info", "alternates"), []byte("/elsewhere\n"), 0o644)
		}, RefuseAlternates},
		"target exists": {func(l *lab) { os.MkdirAll(filepath.Join(l.opts.Dirs.GitDirs, l.plan().RepoID), 0o755) }, RefuseTargetExists},
		"target inside": {func(l *lab) { l.opts.Dirs.GitDirs = filepath.Join(l.worktree, "gitdirs") }, RefuseTargetInside},
		"target synced": {func(l *lab) {
			l.opts.Dirs.GitDirs = filepath.Join(filepath.Dir(l.worktree), "gitdirs")
		}, RefuseTargetSynced},
		"socket": {func(l *lab) {
			// a relative name: a socket's full path may be at most ~100 bytes
			l.t.Chdir(filepath.Join(l.worktree, ".git"))
			ln, err := net.Listen("unix", "fsmonitor--daemon.ipc")
			if err != nil {
				l.t.Skip(err)
			}
			l.t.Cleanup(func() { ln.Close() })
		}, RefuseSocket},
		"symlinked .git": {func(l *lab) {
			os.Rename(filepath.Join(l.worktree, ".git"), filepath.Join(l.home, "real.git"))
			os.Symlink(filepath.Join(l.home, "real.git"), filepath.Join(l.worktree, ".git"))
		}, RefuseNotPlain},
		"icloud uploading": {func(l *lab) {
			l.opts.Probe = func(string) (icloud.Status, error) { return icloud.Status{Known: true, Ubiquitous: true}, nil }
		}, RefuseICloudUploading},
		"icloud unreadable": {func(l *lab) {
			l.opts.Probe = func(string) (icloud.Status, error) { return icloud.Status{}, errors.New("no answer") }
		}, RefuseICloudUploading},
	} {
		t.Run(name, func(t *testing.T) {
			l := newLab(t)
			tc.setup(l)
			if got := codes(l.plan().Refusals); !strings.Contains(got, tc.code) {
				t.Fatalf("refusals %q, want %s", got, tc.code)
			}
		})
	}
}

func TestPlanCarriesOtherDamage(t *testing.T) {
	l := newLab(t)
	b, _ := os.ReadFile(filepath.Join(l.worktree, ".git", "index"))
	os.WriteFile(filepath.Join(l.worktree, ".git", "index 2"), b, 0o644)
	p := l.plan()
	if len(p.Refusals) != 0 {
		t.Fatalf("refusals: %+v", p.Refusals)
	}
	if len(p.Carried) != 1 || p.Carried[0].Code != check.CodeIndexCopy {
		t.Fatalf("carried: %+v", p.Carried)
	}
}

func TestPlanAlreadyMoved(t *testing.T) {
	l := newLab(t)
	gd := filepath.Join(l.home, "outside.git")
	os.Rename(filepath.Join(l.worktree, ".git"), gd)
	os.WriteFile(filepath.Join(l.worktree, ".git"), []byte("gitdir: "+gd+"\n"), 0o644)
	if p := l.plan(); !p.Already || len(p.Refusals) != 0 || p.Target != gd {
		t.Fatalf("plan %+v", p)
	}
	os.Rename(gd, gd+".gone") // as on a device the move has not reached yet
	l.opts.Path = filepath.Join(l.worktree, "sub dir")
	if p := l.plan(); codes(p.Refusals) != RefuseGitDirMissing || p.Target != gd || p.Repo.Worktree != l.worktree {
		t.Fatalf("missing git dir: %+v", p)
	}
}

// R4. start() locks and recovers by state.RepoID of the path as the user
// spelled it (FindWorktree keeps $PWD's case and Unicode form); the journal
// entry and check use git's spelling. On APFS both name one directory.
func TestRepoIDIsTheSameForEverySpellingOfOneDirectory(t *testing.T) {
	l := newLab(t)
	upper := filepath.Join(filepath.Dir(l.worktree), "MY NOTES")
	if _, err := os.Stat(upper); err != nil {
		t.Skip("case-sensitive filesystem")
	}
	wt, ok := FindWorktree(upper)
	if !ok {
		t.Fatal("no worktree")
	}
	p := l.plan()
	if a, b := state.RepoID(wt, l.home), p.RepoID; a != b {
		t.Fatalf("lock/recovery id %s, journal id %s: one repository, two locks", a, b)
	}
}
