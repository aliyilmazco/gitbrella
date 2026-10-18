package check

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/scan"
)

func runAt(t *testing.T, dir string, mutate func(*Options)) Report {
	t.Helper()
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	o := Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"}
	if mutate != nil {
		mutate(&o)
	}
	rep, err := Run(context.Background(), g, o)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func has(rep Report, code string) bool {
	for _, f := range rep.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// Why automatic gc may not run: another machine's gc.pid (12 hours) or a
// fresh gc.log (a day); this machine's own or old files are nothing.
func TestGCFiles(t *testing.T) {
	host, _ := os.Hostname()
	old := time.Now().Add(-13 * time.Hour)
	for _, tc := range []struct {
		name, file, content string
		mtime               time.Time
		code                string
	}{
		{"another machine's gc.pid", "gc.pid", "4242 other-machine.local", time.Now(), CodeGCPIDForeign},
		{"this machine's gc.pid", "gc.pid", "4242 " + host, time.Now(), ""},
		{"an old foreign gc.pid", "gc.pid", "4242 other-machine.local", old, ""},
		{"a gc.pid git cannot read (no pid)", "gc.pid", "notanumber other-machine.local", time.Now(), ""},
		{"a fresh gc.log", "gc.log", "error: bad object\n", time.Now(), CodeGCLogFresh},
		{"an empty gc.log", "gc.log", "", time.Now(), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"))
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(r.GitDir, tc.file)
			os.WriteFile(p, []byte(tc.content), 0o644)
			os.Chtimes(p, tc.mtime, tc.mtime)
			rep := runAt(t, r.Dir, nil)
			for _, code := range []string{CodeGCPIDForeign, CodeGCLogFresh} {
				if has(rep, code) != (code == tc.code) {
					t.Fatalf("%s: %v", code, codes(rep))
				}
			}
			if ExitCode(rep) != 0 {
				t.Fatalf("information only, yet exit %d", ExitCode(rep))
			}
		})
	}
}

// A reftable repository with a table its list does not name (another
// device's, carried in by the sync client) is damage: the next pack-refs
// deletes it. Not while git holds the list's lock.
func TestReftableUnlistedTable(t *testing.T) {
	if ok, _ := fixtures.GitAtLeast(2, 45); !ok {
		t.Skip("reftable needs git 2.45")
	}
	r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"), "--ref-format=reftable")
	if err != nil {
		t.Fatal(err)
	}
	if has(runAt(t, r.Dir, nil), CodeReftableUnlisted) {
		t.Fatal("a sound reftable repository")
	}
	other := filepath.Join(r.GitDir, "reftable", "0x000000000001-0x000000000009-0a1b2c3d.ref")
	os.WriteFile(other, []byte("REFT"), 0o644)
	if f := find(t, runAt(t, r.Dir, nil), CodeReftableUnlisted); f.Severity != Damage || f.Path != "reftable/0x000000000001-0x000000000009-0a1b2c3d.ref" {
		t.Fatalf("%+v", f)
	}
	os.WriteFile(filepath.Join(r.GitDir, "reftable", "tables.list.lock"), nil, 0o644)
	if has(runAt(t, r.Dir, nil), CodeReftableUnlisted) {
		t.Fatal("reported while git writes the list")
	}
	os.Remove(filepath.Join(r.GitDir, "reftable", "tables.list.lock"))
	// a compaction drops the old tables from the list before it removes them,
	// each still locked: those are git's, not another device's
	os.WriteFile(other+".lock", nil, 0o644)
	if has(runAt(t, r.Dir, nil), CodeReftableUnlisted) {
		t.Fatal("reported a table git is removing")
	}
	os.Remove(other)
	os.Remove(other + ".lock")

	// a linked worktree keeps its own stack (HEAD and the per-worktree refs)
	if _, err := r.Git("worktree", "add", "-q", filepath.Join(filepath.Dir(r.Dir), "wt"), "-b", "side"); err != nil {
		t.Fatal(err)
	}
	if has(runAt(t, r.Dir, nil), CodeReftableUnlisted) {
		t.Fatal("a sound worktree stack")
	}
	os.WriteFile(filepath.Join(r.GitDir, "worktrees", "wt", "reftable", "0x000000000001-0x000000000009-0a1b2c3d.ref"), []byte("REFT"), 0o644)
	if f := find(t, runAt(t, r.Dir, nil), CodeReftableUnlisted); f.Path != "worktrees/wt/reftable/0x000000000001-0x000000000009-0a1b2c3d.ref" {
		t.Fatalf("worktree stack: %+v", f)
	}
}

// Names a user picks that carry an unmarked client scheme's letters, or a
// lowercase suffix after a git name, are no copies: no damage, so a hook
// built on the check lets the work go on. OneDrive's provider is forced, the
// strictest case.
func TestNamesWithAClientsLettersAreNoDamage(t *testing.T) {
	r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"fix.Conflict", "resolve.conflicted", "merge_conflict-20240101-120000"} {
		if _, err := r.Git("branch", b); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Git("tag", "v1.0.Conflict"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Git("worktree", "add", "-q", filepath.Join(filepath.Dir(r.Dir), "wt.Conflict"), "-b", "side"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config-backup", "index-old", "description-old", "hooks-disabled/pre-commit"} {
		p := filepath.Join(r.GitDir, filepath.FromSlash(f))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x\n"), 0o644)
	}
	for _, prov := range []string{"", "onedrive", "nextcloud"} {
		rep := runAt(t, r.Dir, func(o *Options) { o.Provider = prov })
		for _, f := range rep.Findings {
			if f.Severity == Damage {
				t.Errorf("provider %q: %s %s", prov, f.Code, f.Path)
			}
		}
	}
}

// A config that is no regular file (a FIFO here) is not read: git would wait
// on it forever.
func TestConfigWorktreeSkipsAFIFO(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	if err := syscall.Mkfifo(p, 0o644); err != nil {
		t.Skip(err)
	}
	done := make(chan string, 1)
	go func() { done <- configWorktree(p) }()
	select {
	case got := <-done:
		if got != "" {
			t.Fatalf("%q", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("configWorktree waits on a FIFO")
	}
}

// Hooks or an fsmonitor command taken from a synced folder outside the git
// directory are named; the built-in fsmonitor is not.
func TestHooksInASyncedFolder(t *testing.T) {
	r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	hooks := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "hooks")
	os.MkdirAll(hooks, 0o755)
	if _, err := r.Git("config", "core.hooksPath", hooks); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Git("config", "core.fsmonitor", "true"); err != nil {
		t.Fatal(err)
	}
	rep := runAt(t, r.Dir, func(o *Options) { o.Home, o.GOOS = home, "darwin" })
	f := find(t, rep, CodeSyncedHooks)
	if f.Details["config"] != "core.hooksPath" || f.Severity != Info {
		t.Fatalf("%+v", f)
	}
	n := 0
	for _, f := range rep.Findings {
		if f.Code == CodeSyncedHooks {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("the built-in fsmonitor reported too: %v", codes(rep))
	}
}

// Another machine's core.worktree makes every git command fail; the
// not-a-repository hint names it and how to undo it.
func TestForeignCoreWorktreeHint(t *testing.T) {
	for _, name := range []string{"repo", "cs $(echo Z) x", "it's here"} {
		r, err := fixtures.New(filepath.Join(t.TempDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		cfg := filepath.Join(r.GitDir, "config")
		cmd := exec.Command("git", "config", "--file", cfg, "core.worktree", "/sessions/abc123/work")
		cmd.Env = gitx.Env(os.Environ())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
		hints := NotRepoHints(r.Dir, scan.DefaultFlagReader())
		if len(hints) != 1 || !strings.Contains(hints[0], "core.worktree") {
			t.Fatalf("%s: %q", name, hints)
		}
		if name == "repo" { // from inside the repository, where git itself fails on the setting
			t.Chdir(r.Dir)
			if again := NotRepoHints(".", scan.DefaultFlagReader()); len(again) != 1 || !strings.Contains(again[0], "core.worktree") {
				t.Fatalf("from inside: %q", again)
			}
		}
		// the remedy, run as printed, works, even in a folder named to trip a shell
		_, remedy, ok := strings.Cut(hints[0], "remove it from outside any repository: ")
		if !ok {
			t.Fatalf("no remedy in %q", hints[0])
		}
		fix := exec.Command("sh", "-c", remedy)
		fix.Env = gitx.Env(os.Environ())
		if out, err := fix.CombinedOutput(); err != nil {
			t.Fatalf("%s: the remedy %q failed: %v %s", name, remedy, err, out)
		}
		if again := NotRepoHints(r.Dir, scan.DefaultFlagReader()); len(again) != 0 {
			t.Fatalf("%s: after the remedy: %q", name, again)
		}
	}
}

// With extensions.worktreeConfig, core.worktree may sit in config.worktree:
// the hint names it, and its remedy works.
func TestForeignCoreWorktreeInConfigWorktree(t *testing.T) {
	r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"config", "extensions.worktreeConfig", "true"},
		{"config", "--file", filepath.Join(r.GitDir, "config.worktree"), "core.worktree", "/sessions/abc123/work"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = r.Dir, gitx.Env(os.Environ())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	hints := NotRepoHints(r.Dir, scan.DefaultFlagReader())
	if len(hints) != 1 || !strings.Contains(hints[0], "config.worktree") {
		t.Fatalf("%q", hints)
	}
	_, remedy, _ := strings.Cut(hints[0], "remove it from outside any repository: ")
	fix := exec.Command("sh", "-c", remedy)
	fix.Env = gitx.Env(os.Environ())
	if out, err := fix.CombinedOutput(); err != nil {
		t.Fatalf("the remedy %q failed: %v %s", remedy, err, out)
	}
}

// A FIFO where git keeps a small file (gc.pid here) is not read: the check
// would wait on it for ever while git itself goes on.
func TestFIFOsDoNotHangTheCheck(t *testing.T) {
	r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(r.GitDir, "gc.pid"), 0o644); err != nil {
		t.Skip(err)
	}
	done := make(chan Report, 1)
	go func() { done <- runAt(t, r.Dir, nil) }()
	select {
	case rep := <-done:
		if has(rep, CodeGCPIDForeign) {
			t.Fatalf("a FIFO taken for another machine's gc.pid: %v", codes(rep))
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the check waits on a FIFO gc.pid")
	}
}
