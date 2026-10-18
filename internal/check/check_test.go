package check

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/verify"
)

type noDataless struct{}

func (noDataless) IsDataless(string) (bool, error) { return false, nil }

type suffixDataless string

func (s suffixDataless) IsDataless(p string) (bool, error) {
	return strings.HasSuffix(filepath.ToSlash(p), string(s)), nil
}

func noProcs(context.Context) ([]procs.Proc, error) { return nil, nil }

func run(t *testing.T, scenario string, mutate func(*Options)) Report {
	t.Helper()
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo with space")
	if err := fixtures.Build(scenario, dir); errors.Is(err, fixtures.ErrUnsupported) {
		t.Skip(err)
	} else if err != nil {
		t.Fatal(err)
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

func codes(r Report) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.Code)
	}
	return out
}

func find(t *testing.T, r Report, code string) Finding {
	t.Helper()
	for _, f := range r.Findings {
		if f.Code == code {
			return f
		}
	}
	t.Fatalf("no %s in %v", code, codes(r))
	return Finding{}
}

func TestScenarios(t *testing.T) {
	cases := []struct {
		scenario string
		want     []string
		exit     int
	}{
		{"clean", nil, 0},
		{"index-copies", []string{CodeIndexCopy, CodeIndexCopy}, 1},
		{"orphan-index-lock", []string{CodeOrphanIndexLock}, 1},
		{"ref-copy-older", []string{CodeRefCopy}, 1},
		{"ref-copy-newer", []string{CodeRefCopy}, 1},
		{"dropbox-copy", []string{CodeCopy}, 1},
		{"stale-lock", []string{CodeStaleLock}, 1},
		{"merge-in-progress", []string{CodeInProgress}, 0},
		{"corrupt-index", []string{CodeIndexInvalid}, 1},
		{"third-party", []string{CodeUnknown, CodeUnknown}, 0},
		{"dir-copy", []string{CodeCopy}, 1},
		{"linked-worktree", []string{CodeIndexCopy}, 1},
		{"reftable", nil, 0},
		{"syncthing-copy", []string{CodeIndexCopy}, 1},
		{"desktop-ini", []string{CodeForeign, CodeForeign}, 1},
		{"foreign-object", []string{CodeForeign}, 1},
		{"empty-object", []string{CodeEmpty}, 1},
		{"empty-ref", []string{CodeEmpty}, 1},
		{"stale-rebase", []string{CodeStaleOperation}, 1},
		{"gitdir-copy", []string{CodeGitDirCopy}, 1},
		{"worktree-copies", []string{CodeWorktreeCopies}, 0},
		{"broken-worktree-admin", []string{CodeWorktreeAdmin}, 1},
		{"no-checkout", []string{CodeNoCheckout}, 0},
		{"gitdir-outside", nil, 0},
		{"nosync-unignored", []string{CodeGitDirNotIgnored}, 1},
		{"nosync-ignored", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			rep := run(t, c.scenario, nil)
			got := codes(rep)
			if c.scenario == "reftable" {
				c.want = []string{CodeReftable}
			}
			if !slices.Equal(got, c.want) {
				t.Fatalf("codes = %v, want %v", got, c.want)
			}
			if ExitCode(rep) != c.exit {
				t.Fatalf("exit = %d, want %d", ExitCode(rep), c.exit)
			}
		})
	}
}

func TestOrphanLockEqualsHEAD(t *testing.T) {
	f := find(t, run(t, "orphan-index-lock", nil), CodeOrphanIndexLock)
	if f.Details["validIndex"] != "true" || f.Details["equalsHEAD"] != "true" {
		t.Fatalf("details = %v", f.Details)
	}
}

func TestRefCopyRelations(t *testing.T) {
	if f := find(t, run(t, "ref-copy-older", nil), CodeRefCopy); f.Details["relation"] != "copy-older" {
		t.Fatalf("older: %v", f.Details)
	}
	f := find(t, run(t, "ref-copy-newer", nil), CodeRefCopy)
	if f.Details["relation"] != "copy-newer" || !strings.Contains(f.Message, "commits the branch does not have") {
		t.Fatalf("newer: %s %v", f.Message, f.Details)
	}
}

func TestRunningGitMakesLocksInfo(t *testing.T) {
	rep := run(t, "stale-lock", func(o *Options) {
		dir := o.Path
		o.Procs = func(context.Context) ([]procs.Proc, error) { return []procs.Proc{{PID: 7, Cwd: dir}}, nil }
	})
	if got := codes(rep); !slices.Equal(got, []string{CodeGitRunning, CodeLockInUse}) || ExitCode(rep) != 0 {
		t.Fatalf("codes = %v exit %d", got, ExitCode(rep))
	}
}

func TestProcessListFailureNeverCallsLocksStale(t *testing.T) {
	rep := run(t, "stale-lock", func(o *Options) {
		o.Procs = func(context.Context) ([]procs.Proc, error) { return nil, errors.New("boom") }
	})
	if slices.Contains(codes(rep), CodeStaleLock) {
		t.Fatalf("lock judged stale without a process list: %v", codes(rep))
	}
}

// A git whose folder cannot be read may be working here: its lock is not
// called stale, and the report says why.
func TestGitOfUnknownFolderMayHoldTheLock(t *testing.T) {
	rep := run(t, "stale-lock", func(o *Options) {
		o.Procs = func(context.Context) ([]procs.Proc, error) { return []procs.Proc{{PID: 7}}, nil }
	})
	if got := codes(rep); slices.Contains(got, CodeStaleLock) || !slices.Contains(got, CodeProcessCheckFail) {
		t.Fatalf("codes = %v", got)
	}
	if f := find(t, rep, CodeProcessCheckFail); !strings.Contains(f.Message, "1 git process(es)") {
		t.Fatalf("message %q", f.Message)
	}
}

func TestDatalessSkipsObjectReads(t *testing.T) {
	rep := run(t, "ref-copy-older", func(o *Options) { o.Flags = suffixDataless("/objects") })
	if f := find(t, rep, CodeDataless); f.Severity != Damage {
		t.Fatalf("dataless must be damage: %+v", f)
	}
	if f := find(t, rep, CodeRefCopy); f.Details["verified"] != "false" || f.Details["relation"] != "" {
		t.Fatalf("ref copy must not be verified while objects are dataless: %v", f.Details)
	}
}

func TestCloudFolderIsInfoOnly(t *testing.T) {
	rep := run(t, "clean", func(o *Options) { o.Provider = "icloud" })
	if got := codes(rep); !slices.Equal(got, []string{CodeInCloudFolder}) || ExitCode(rep) != 0 {
		t.Fatalf("codes = %v exit %d", got, ExitCode(rep))
	}
}

func TestDatalessMissingIndexIsNotVerified(t *testing.T) {
	rep := run(t, "orphan-index-lock", func(o *Options) { o.Flags = suffixDataless("/objects") })
	f := find(t, rep, CodeOrphanIndexLock)
	if f.Details["verified"] != "false" || f.Details["equalsHEAD"] != "not checked" {
		t.Fatalf("orphan lock must not be compared with HEAD while objects are dataless: %v", f.Details)
	}
	if ExitCode(rep) != 1 {
		t.Fatalf("exit = %d", ExitCode(rep))
	}
}

func TestWorktreeCopiesIgnoreNamesWithoutOriginal(t *testing.T) {
	f := find(t, run(t, "worktree-copies", nil), CodeWorktreeCopies)
	if f.Details["count"] != "1" || f.Details["sample"] != "note 2.md" {
		t.Fatalf("details = %v", f.Details)
	}
}

func TestDeepCheckFindsBrokenObjects(t *testing.T) {
	rep := run(t, "empty-object", func(o *Options) { o.Deep = true })
	if got := codes(rep); !slices.Equal(got, []string{CodeEmpty, CodeFsck}) {
		t.Fatalf("codes = %v", got)
	}
	if got := codes(run(t, "clean", func(o *Options) { o.Deep = true })); len(got) != 0 {
		t.Fatalf("clean deep check: %v", got)
	}
}

func TestDeepCheckSkippedWhenDataless(t *testing.T) {
	rep := run(t, "clean", func(o *Options) { o.Deep = true; o.Flags = suffixDataless("/objects") })
	if !slices.Contains(codes(rep), CodeDeepSkipped) || slices.Contains(codes(rep), CodeFsck) {
		t.Fatalf("codes = %v", codes(rep))
	}
}

func TestGitDirCopyHint(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{".git 2", "notes", "git 3"} {
		if err := os.Mkdir(filepath.Join(dir, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := GitDirCopyHint(dir); !slices.Equal(got, []string{".git 2", "git 3"}) {
		t.Fatalf("hint = %v", got)
	}
}

func TestGitDirOutsideCloudIsSafeLayout(t *testing.T) {
	rep := run(t, "gitdir-outside", func(o *Options) { o.Provider = "icloud" })
	if got := codes(rep); !slices.Equal(got, []string{CodeGitDirNotSynced}) || ExitCode(rep) != 0 {
		t.Fatalf("codes = %v exit %d", got, ExitCode(rep))
	}
}

func TestNoSyncGitDirIsNotSyncedByICloud(t *testing.T) {
	rep := run(t, "nosync-ignored", func(o *Options) { o.Provider = "icloud" })
	if got := codes(rep); !slices.Equal(got, []string{CodeGitDirNotSynced}) || ExitCode(rep) != 0 {
		t.Fatalf("codes = %v exit %d", got, ExitCode(rep))
	}
	// .nosync is an iCloud rule; Dropbox still syncs the directory.
	rep = run(t, "nosync-ignored", func(o *Options) { o.Provider = "dropbox" })
	if got := codes(rep); !slices.Equal(got, []string{CodeInCloudFolder}) {
		t.Fatalf("dropbox codes = %v", got)
	}
}

func TestNoSyncUnignoredIsDamageEvenWhenNotSynced(t *testing.T) {
	rep := run(t, "nosync-unignored", func(o *Options) { o.Provider = "icloud" })
	if got := codes(rep); !slices.Equal(got, []string{CodeGitDirNotIgnored, CodeGitDirNotSynced}) || ExitCode(rep) != 1 {
		t.Fatalf("codes = %v exit %d", got, ExitCode(rep))
	}
	f := find(t, rep, CodeGitDirNotIgnored)
	if f.Path != ".git.nosync" || !strings.Contains(f.Message, "/.git.nosync/") || !strings.Contains(f.Message, ".git.nosync/info/exclude") {
		t.Fatalf("finding = %+v", f)
	}
}

func TestNoSyncGitDirIsNotScannedAsWorktree(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("nosync-ignored", dir); err != nil {
		t.Fatal(err)
	}
	// A conflict copy inside the git directory is not a working-tree copy.
	if err := os.WriteFile(filepath.Join(dir, ".git.nosync", "HEAD 2"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if f.Code == CodeWorktreeCopies {
			t.Fatalf("git directory scanned as working tree: %+v", f)
		}
	}
}

func procsAt(dir string) func(context.Context) ([]procs.Proc, error) {
	return func(context.Context) ([]procs.Proc, error) { return []procs.Proc{{PID: 4242, Cwd: dir}}, nil }
}

func TestGitInAnotherWorktreeKeepsLocksLive(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	linked := filepath.Join(t.TempDir(), "wt")
	if err := fixtures.Build("linked-worktree", linked); err != nil {
		t.Fatal(err)
	}
	main := linked + "-main"
	lock := filepath.Join(main, ".git", "HEAD.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), g, Options{Path: main, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: procsAt(linked), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(codes(rep), CodeStaleLock) {
		t.Fatalf("git running in a linked worktree, but its lock was called stale: %v", codes(rep))
	}
	find(t, rep, CodeLockInUse)
}

func TestBrokenAncestryDegradesInsteadOfAborting(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"three\n", "four\n"} {
		if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Git("commit", "-q", "-am", c); err != nil {
			t.Fatal(err)
		}
	}
	c3, _ := r.Git("rev-parse", "HEAD")
	c2, _ := r.Git("rev-parse", "HEAD~1")
	if err := r.WriteFile("refs/heads/main 2", c3+"\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Git("update-ref", "refs/heads/main", r.Commits[1]); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(r.GitDir, "objects", c2[:2], c2[2:])); err != nil {
		t.Fatal(err)
	}
	if err := r.CopyFile("HEAD", "HEAD 2"); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatalf("one unreadable commit aborted the whole check: %v", err)
	}
	// git 2.45+ fails on the missing parent (unverifiable); older git walks
	// past it and answers "diverged". Either way the copy must not look safe
	// to drop, and the run must go on.
	switch f := find(t, rep, CodeRefCopy); f.Details["relation"] {
	case "unverifiable", string(verify.RelDiverged), string(verify.RelCopyNewer):
	default:
		t.Fatalf("ref copy details = %v", f.Details)
	}
	if f := find(t, rep, CodeCopy); f.Path != "HEAD 2" {
		t.Fatalf("HEAD 2 lost: %+v", f)
	}
}

func TestOrphanIndexLockWhileGitRunsIsNotDamage(t *testing.T) {
	rep := run(t, "orphan-index-lock", func(o *Options) { o.Procs = procsAt(o.Path) })
	if ExitCode(rep) != 0 || slices.Contains(codes(rep), CodeOrphanIndexLock) {
		t.Fatalf("codes = %v exit %d", codes(rep), ExitCode(rep))
	}
	find(t, rep, CodeLockInUse)
}

func TestRecentOrphanIndexLockIsNotDamage(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("orphan-index-lock", dir); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(filepath.Join(dir, ".git", "index.lock"), now, now); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: now, Flags: noDataless{}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if ExitCode(rep) != 0 {
		t.Fatalf("a lock written a moment ago is damage: %v", codes(rep))
	}
	find(t, rep, CodeLockRecent)
}

func TestPartialCloneRefCopyDoesNotFetch(t *testing.T) {
	rep := run(t, "partial-clone", nil)
	f := find(t, rep, CodeRefCopy)
	g, _ := gitx.New()
	if lazyFetchSafe(context.Background(), g) {
		// git 2.45+: the read fails instead of fetching.
		if f.Details["relation"] != string(verify.RelMissingObject) {
			t.Fatalf("ref copy details = %v", f.Details)
		}
		return
	}
	// older git: object reads are skipped altogether.
	if f.Details["verified"] != "false" {
		t.Fatalf("ref copy details = %v", f.Details)
	}
	find(t, rep, CodePartialClone)
}

func TestPartialCloneOnOldGitSkipsObjectReads(t *testing.T) {
	saved := lazyFetchSafe
	lazyFetchSafe = func(context.Context, *gitx.Runner) bool { return false }
	defer func() { lazyFetchSafe = saved }()
	rep := run(t, "partial-clone", nil)
	if f := find(t, rep, CodeRefCopy); f.Details["verified"] != "false" || f.Details["relation"] != "" {
		t.Fatalf("ref copy read objects on a git that cannot stop lazy fetch: %v", f.Details)
	}
	find(t, rep, CodePartialClone)
}

type someDataless []string // paths ending in any of these suffixes are dataless

func (s someDataless) IsDataless(p string) (bool, error) {
	for _, suf := range s {
		if strings.HasSuffix(filepath.ToSlash(p), suf) {
			return true, nil
		}
	}
	return false, nil
}

func TestNotRepoHints(t *testing.T) {
	// A renamed git directory (the existing hint, unchanged).
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git 2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(dir, noDataless{}); len(got) != 1 || !strings.Contains(got[0], `".git 2" here looks like a renamed or copied git directory`) {
		t.Fatalf("renamed: %q", got)
	}

	// git's own files evicted to iCloud: rev-parse fails, but nothing is wrong
	// that a download would not fix. Found from a subdirectory too.
	repoDir := filepath.Join(t.TempDir(), "repo")
	if _, err := fixtures.New(repoDir); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repoDir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := NotRepoHints(sub, someDataless{"/.git/HEAD", "/.git/config"})
	if len(got) != 1 || !strings.Contains(got[0], "HEAD, config") || !strings.Contains(got[0], "only in the cloud") {
		t.Fatalf("dataless: %q", got)
	}

	// A .git file whose git directory is gone, as after removing a linked worktree.
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /nonexistent-gcd/main/.git/worktrees/feat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = NotRepoHints(wt, noDataless{})
	if len(got) != 1 || !strings.Contains(got[0], "/nonexistent-gcd/main/.git/worktrees/feat") || !strings.Contains(got[0], "does not exist") {
		t.Fatalf("dangling gitfile: %q", got)
	}

	// The same .git file evicted: never read, reported as cloud-only.
	got = NotRepoHints(wt, someDataless{"/.git"})
	if len(got) != 1 || !strings.Contains(got[0], "only in the cloud") {
		t.Fatalf("dataless gitfile: %q", got)
	}

	// A .git that is a symlink to a .git file pointing nowhere: the file is
	// read through the link.
	linked := t.TempDir()
	gitfile := filepath.Join(t.TempDir(), "gitfile")
	if err := os.WriteFile(gitfile, []byte("gitdir: /nonexistent-gcd/x.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(gitfile, filepath.Join(linked, ".git")); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(linked, noDataless{}); len(got) != 1 || !strings.Contains(got[0], "does not exist") {
		t.Fatalf("symlinked gitfile: %q", got)
	}

	// HEAD emptied by a sync client: git no longer sees a repository.
	emptied := filepath.Join(t.TempDir(), "repo")
	if _, err := fixtures.New(emptied); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(emptied, ".git", "HEAD"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(emptied, noDataless{}); len(got) != 1 || !strings.Contains(got[0], "HEAD is empty") {
		t.Fatalf("empty HEAD: %q", got)
	}
}

func TestWorktreeUnderNoSyncIsNotSynced(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "proj.nosync", "repo")
	if err := fixtures.Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), g, Options{Path: dir, Provider: "icloud", Home: t.TempDir(), GOOS: "darwin", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := codes(rep); len(got) != 0 || rep.Repo.Provider != "none" {
		t.Fatalf("iCloud skips *.nosync folders; codes = %v provider = %s", got, rep.Repo.Provider)
	}
}

func TestTruncatedWorktreeScanIsReported(t *testing.T) {
	rep := run(t, "clean", func(o *Options) {
		for i := 0; i < 10; i++ {
			if err := os.WriteFile(filepath.Join(o.Path, fmt.Sprintf("f%d.md", i)), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		o.WorktreeLimit = 3
	})
	if f := find(t, rep, CodeWorktreeTruncated); f.Severity != Info {
		t.Fatalf("finding = %+v", f)
	}
	if ExitCode(rep) != 0 {
		t.Fatalf("truncation is not damage: %v", codes(rep))
	}
}

func TestIgnoreProbeFailureIsReported(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	wrapper := filepath.Join(t.TempDir(), "git")
	script := "#!/bin/sh\nfor a in \"$@\"; do [ \"$a\" = check-ignore ] && { echo 'fatal: simulated' >&2; exit 128; }; done\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	failing := *g
	failing.Bin = wrapper
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("nosync-unignored", dir); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), &failing, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(codes(rep), CodeGitDirNotIgnored) {
		t.Fatalf("damage claimed without an answer from git: %v", codes(rep))
	}
	find(t, rep, CodeIgnoreUnchecked)
}

type orderFlags struct{ procsListed, early *bool }

func (f orderFlags) IsDataless(string) (bool, error) {
	if !*f.procsListed {
		*f.early = true
	}
	return false, nil
}

func TestProcessesAreListedBeforeTheScan(t *testing.T) {
	listed, early := false, false
	run(t, "stale-lock", func(o *Options) {
		o.Flags = orderFlags{&listed, &early}
		o.Procs = func(context.Context) ([]procs.Proc, error) { listed = true; return nil, nil }
	})
	if early {
		t.Fatal("the .git scan ran before git processes were listed; a lock released in between could be called stale")
	}
}

func TestOldGitTempFileIsInfo(t *testing.T) {
	rep := run(t, "clean", func(o *Options) {
		p := filepath.Join(o.Path, ".git", "next-index-4242.lock")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	})
	if ExitCode(rep) != 0 || slices.Contains(codes(rep), CodeStaleLock) {
		t.Fatalf("a leftover temp file is not damage: %v", codes(rep))
	}
	find(t, rep, CodeGitTempFile)
}

// nosyncWithGitignore builds the .git.nosync layout whose ignore rule lives in
// the working tree's .gitignore.
func nosyncWithGitignore(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("nosync-unignored", dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("/.git.nosync/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestUnreadableGitignoreIsNotDamage(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads unreadable files")
	}
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := nosyncWithGitignore(t)
	ign := filepath.Join(dir, ".gitignore")
	if err := os.Chmod(ign, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ign, 0o644) })
	rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(codes(rep), CodeGitDirNotIgnored) {
		t.Fatalf("git could not read .gitignore, yet the rule was called missing: %v", codes(rep))
	}
	find(t, rep, CodeIgnoreUnchecked)
}

func TestCloudOnlyIgnoreFilesAreNotProbed(t *testing.T) {
	for name, flags := range map[string]someDataless{
		".gitignore":        {"/repo/.gitignore"},
		"core.excludesFile": {"/global-ignore"},
	} {
		t.Run(name, func(t *testing.T) {
			g, err := gitx.New()
			if err != nil {
				t.Skip("git not installed")
			}
			dir := nosyncWithGitignore(t)
			if _, err := gitIn(dir, "config", "core.excludesFile", filepath.Join(t.TempDir(), "global-ignore")); err != nil {
				t.Fatal(err)
			}
			rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: flags, Procs: noProcs, Version: "test"})
			if err != nil {
				t.Fatal(err)
			}
			if slices.Contains(codes(rep), CodeGitDirNotIgnored) {
				t.Fatalf("judged ignore rules that exist only in the cloud: %v", codes(rep))
			}
			find(t, rep, CodeIgnoreUnchecked)
		})
	}
}

func gitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), noMaintenance...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// noMaintenance stops git >= 2.47 from starting a detached "git maintenance
// run --auto" after commits, which can still write in .git while the test's
// temporary directory is removed.
var noMaintenance = []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false"}

func TestNotRepoHintsDoNotGuess(t *testing.T) {
	// The gitfile target exists but cannot be read: not "does not exist".
	if os.Getuid() != 0 {
		base := t.TempDir()
		locked := filepath.Join(base, "locked")
		if err := os.MkdirAll(filepath.Join(locked, "gitdir"), 0o755); err != nil {
			t.Fatal(err)
		}
		wt := filepath.Join(base, "wt")
		if err := os.MkdirAll(wt, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+filepath.Join(locked, "gitdir")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(locked, 0o755) })
		got := NotRepoHints(wt, noDataless{})
		if len(got) != 1 || strings.Contains(got[0], "does not exist") || !strings.Contains(got[0], "cannot be read") {
			t.Fatalf("unreadable target: %q", got)
		}
	}

	// The gitfile target itself exists only in the cloud.
	wt := t.TempDir()
	target := filepath.Join(t.TempDir(), "gitdir")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+target+"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(wt, someDataless{"/gitdir"}); len(got) != 1 || !strings.Contains(got[0], "only in the cloud") {
		t.Fatalf("cloud-only target: %q", got)
	}

	// git requires "gitdir: " with a space; anything else is not a gitfile.
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, ".git"), []byte("gitdir:/nonexistent-gcd/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(bad, noDataless{}); len(got) != 1 || !strings.Contains(got[0], "not a valid .git file") {
		t.Fatalf("bad gitfile: %q", got)
	}

	// A git directory without HEAD is not a repository to git.
	nohead := filepath.Join(t.TempDir(), "repo")
	if _, err := fixtures.New(nohead); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(nohead, ".git", "HEAD"), filepath.Join(nohead, ".git", "HEAD.moved")); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(nohead, noDataless{}); len(got) != 1 || !strings.Contains(got[0], "has no HEAD") {
		t.Fatalf("missing HEAD: %q", got)
	}

	// A linked worktree: git's config lives in the common directory.
	linked := filepath.Join(t.TempDir(), "wt")
	if err := fixtures.Build("linked-worktree", linked); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(linked, someDataless{"-main/.git/config"}); len(got) != 1 || !strings.Contains(got[0], "config") || !strings.Contains(got[0], "only in the cloud") {
		t.Fatalf("linked worktree, cloud-only common config: %q", got)
	}

	// A symlinked path is judged where it really is, not where the link sits.
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, ".git"), []byte("gitdir: /nonexistent-gcd/y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "d")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(filepath.Join(base, "link"), noDataless{}); len(got) != 0 {
		t.Fatalf("hint about a .git git never looked at: %q", got)
	}
}

func TestNestedDotGitIsNeverCommitted(t *testing.T) {
	// git never adds a directory named .git, wherever it sits, so a git dir
	// at meta/.git (core.worktree layout) needs no ignore rule.
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	top := filepath.Join(t.TempDir(), "a")
	meta := filepath.Join(top, "meta")
	if err := os.MkdirAll(meta, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", meta}, {"-C", meta, "config", "core.worktree", top}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(top, "f.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), g, Options{Path: meta, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if f.Code == CodeGitDirNotIgnored || f.Severity == Damage || strings.Contains(f.Message, "not a git repository") {
			t.Fatalf("healthy core.worktree layout: %+v", rep.Findings)
		}
	}
}

func TestNestedIgnoreFilesAreChecked(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(dir, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", "--separate-git-dir="+filepath.Join(dir, "meta", ".git.nosync"), dir).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta", ".gitignore"), []byte(".git.nosync/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(t.TempDir(), "real-ignore")
	if err := os.WriteFile(real, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked-ignore")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := gitIn(dir, "config", "core.excludesFile", link); err != nil {
		t.Fatal(err)
	}
	for name, flags := range map[string]someDataless{
		"meta/.gitignore":                    {"/repo/meta/.gitignore"},
		"symlinked core.excludesFile target": {"/real-ignore"},
	} {
		t.Run(name, func(t *testing.T) {
			rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: flags, Procs: noProcs, Version: "test"})
			if err != nil {
				t.Fatal(err)
			}
			find(t, rep, CodeIgnoreUnchecked)
		})
	}
}

func TestNotRepoHintsNeverBlockOrFollowBlindly(t *testing.T) {
	// A FIFO named .git: git skips it; reading it would block forever.
	fifoDir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(fifoDir, ".git"), 0o644); err != nil {
		t.Skip("mkfifo:", err)
	}
	done := make(chan []string, 1)
	go func() { done <- NotRepoHints(fifoDir, noDataless{}) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("NotRepoHints blocked reading a FIFO named .git")
	}

	// A .git symlink to a gitfile that exists only in the cloud.
	wt := t.TempDir()
	real := filepath.Join(t.TempDir(), "real.gitfile")
	if err := os.WriteFile(real, []byte("gitdir: /nonexistent-gcd/z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(wt, ".git")); err != nil {
		t.Fatal(err)
	}
	if got := NotRepoHints(wt, someDataless{"/real.gitfile"}); len(got) != 1 || !strings.Contains(got[0], "only in the cloud") {
		t.Fatalf("symlinked cloud-only gitfile: %q", got)
	}
}

func gitRun(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), noMaintenance...), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestHookIndexOfAMissingIndexIsDamage(t *testing.T) {
	// git commit -a after the sync client deleted .git/index: the hook runs
	// with GIT_INDEX_FILE=index.lock and no index; the commit would delete
	// every tracked file.
	rep := run(t, "clean", func(o *Options) {
		gd := filepath.Join(o.Path, ".git")
		if err := os.Rename(filepath.Join(gd, "index"), filepath.Join(gd, "index.lock")); err != nil {
			t.Fatal(err)
		}
		o.IndexFile = filepath.Join(gd, "index.lock")
		o.Procs = procsAt(o.Path)
	})
	find(t, rep, CodeIndexMissing)
	if ExitCode(rep) != 1 {
		t.Fatalf("codes = %v", codes(rep))
	}
}

func TestEmptyIndexWhileHEADHasFilesIsDamage(t *testing.T) {
	for name, viaHook := range map[string]bool{"standalone": false, "hook": true} {
		t.Run(name, func(t *testing.T) {
			rep := run(t, "clean", func(o *Options) {
				gitRun(t, o.Path, nil, "read-tree", "--empty") // what git commit rebuilds from a missing index
				if viaHook {
					o.IndexFile = filepath.Join(o.Path, ".git", "index")
					o.Procs = procsAt(o.Path) // the commit running the hook
				}
			})
			if f := find(t, rep, CodeIndexEmpty); f.Severity != Damage {
				t.Fatalf("%+v", f)
			}
		})
	}
}

func TestEmptyIndexThatMeansDeleteEverythingIsNotDamage(t *testing.T) {
	rep := run(t, "clean", func(o *Options) { gitRun(t, o.Path, nil, "rm", "-r", "-q", ".") })
	if slices.Contains(codes(rep), CodeIndexEmpty) || ExitCode(rep) != 0 {
		t.Fatalf("git rm -r . is a deliberate state: %v", codes(rep))
	}
}

func TestSplitIndexIsNotMistakenForEmpty(t *testing.T) {
	rep := run(t, "clean", func(o *Options) { gitRun(t, o.Path, nil, "update-index", "--split-index") })
	if slices.Contains(codes(rep), CodeIndexEmpty) || ExitCode(rep) != 0 {
		t.Fatalf("split index read as empty: %v", codes(rep))
	}
}

func TestMissingHEADCommitIsDamage(t *testing.T) {
	rep := run(t, "clean", func(o *Options) {
		out, err := exec.Command("git", "-C", o.Path, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		c := strings.TrimSpace(string(out))
		if err := os.Remove(filepath.Join(o.Path, ".git", "objects", c[:2], c[2:])); err != nil {
			t.Fatal(err)
		}
	})
	if f := find(t, rep, CodeHEADCommitMissing); f.Severity != Damage || ExitCode(rep) != 1 {
		t.Fatalf("%+v", f)
	}
}

func procsStarted(dir string, start time.Time) func(context.Context) ([]procs.Proc, error) {
	return func(context.Context) ([]procs.Proc, error) {
		return []procs.Proc{{PID: 4242, Cwd: dir, Start: start}}, nil
	}
}

func TestLockOlderThanEveryRunningGitIsStale(t *testing.T) {
	// A git that started after the lock was written cannot hold it: a hook's
	// own git, an alias, a pager left open.
	rep := run(t, "stale-lock", func(o *Options) { o.Procs = procsStarted(o.Path, time.Now()) })
	if f := find(t, rep, CodeStaleLock); f.Severity != Damage {
		t.Fatalf("%+v", f)
	}
	rep = run(t, "stale-lock", func(o *Options) { o.Procs = procsStarted(o.Path, time.Now().Add(-time.Hour)) })
	find(t, rep, CodeLockInUse)

	rep = run(t, "orphan-index-lock", func(o *Options) { o.Procs = procsStarted(o.Path, time.Now()) })
	find(t, rep, CodeOrphanIndexLock)
}

func TestLockFromTheFutureIsStale(t *testing.T) {
	rep := run(t, "stale-lock", func(o *Options) {
		later := time.Now().Add(time.Hour) // another device's clock
		if err := os.Chtimes(filepath.Join(o.Path, ".git", "refs", "heads", "main.lock"), later, later); err != nil {
			t.Fatal(err)
		}
	})
	find(t, rep, CodeStaleLock)
}

func TestGitInAnyWorktreeOfTheRepoIsSeen(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	old := time.Now().Add(-10 * time.Minute)
	backdate := func(p string) {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	check := func(t *testing.T, path, procCwd string) Report {
		rep, err := Run(context.Background(), g, Options{Path: path, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: procsStarted(procCwd, time.Time{}), Version: "test"})
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}
	t.Run("relative-path worktree", func(t *testing.T) {
		if ok, _ := fixtures.GitAtLeast(2, 48); !ok {
			t.Skip("git worktree add --relative-paths needs git 2.48")
		}
		base := t.TempDir()
		main := filepath.Join(base, "main")
		if _, err := fixtures.New(main); err != nil {
			t.Fatal(err)
		}
		gitRun(t, main, nil, "worktree", "add", "-q", "--relative-paths", "../wt", "-b", "feat")
		backdate(filepath.Join(main, ".git", "worktrees", "wt", "index.lock"))
		if rep := check(t, main, filepath.Join(base, "wt")); slices.Contains(codes(rep), CodeStaleLock) {
			t.Fatalf("live git in the linked worktree not seen: %v", codes(rep))
		}
	})
	t.Run("common dir not named .git", func(t *testing.T) {
		base := t.TempDir()
		main := filepath.Join(base, "main")
		if _, err := fixtures.New(main, "--separate-git-dir="+filepath.Join(base, "store.git")); err != nil {
			t.Fatal(err)
		}
		gitRun(t, main, nil, "worktree", "add", "-q", "../wt", "-b", "feat")
		backdate(filepath.Join(base, "store.git", "index.lock"))
		if rep := check(t, filepath.Join(base, "wt"), main); slices.Contains(codes(rep), CodeStaleLock) {
			t.Fatalf("live git in the main worktree not seen: %v", codes(rep))
		}
	})
	t.Run("worktree moved with mv", func(t *testing.T) {
		base := t.TempDir()
		main := filepath.Join(base, "main")
		if _, err := fixtures.New(main); err != nil {
			t.Fatal(err)
		}
		gitRun(t, main, nil, "worktree", "add", "-q", filepath.Join(base, "wtA"), "-b", "feat")
		if err := os.Rename(filepath.Join(base, "wtA"), filepath.Join(base, "wtB")); err != nil {
			t.Fatal(err)
		}
		backdate(filepath.Join(main, ".git", "worktrees", "wtA", "index.lock"))
		if rep := check(t, main, filepath.Join(base, "wtB")); slices.Contains(codes(rep), CodeStaleLock) {
			t.Fatalf("live git in a moved worktree not seen: %v", codes(rep))
		}
	})
	t.Run("unrelated repo nested in the worktree", func(t *testing.T) {
		main := filepath.Join(t.TempDir(), "main")
		if _, err := fixtures.New(main); err != nil {
			t.Fatal(err)
		}
		other := filepath.Join(main, "vendor", "other")
		if _, err := fixtures.New(other); err != nil {
			t.Fatal(err)
		}
		backdate(filepath.Join(main, ".git", "HEAD.lock"))
		if rep := check(t, main, other); !slices.Contains(codes(rep), CodeStaleLock) {
			t.Fatalf("a git working on another repository kept this lock alive: %v", codes(rep))
		}
	})
}

func TestNoCheckoutWithFinderMetadata(t *testing.T) {
	rep := run(t, "no-checkout", func(o *Options) {
		if err := os.WriteFile(filepath.Join(o.Path, ".DS_Store"), []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if got := codes(rep); !slices.Equal(got, []string{CodeNoCheckout}) {
		t.Fatalf("codes = %v", got)
	}
}

func TestHintInsideTheGitDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	if _, err := fixtures.New(dir); err != nil {
		t.Fatal(err)
	}
	got := NotRepoHints(filepath.Join(dir, ".git", "refs"), noDataless{})
	if len(got) != 1 || !strings.Contains(got[0], "inside the git directory") {
		t.Fatalf("hints = %q", got)
	}
}

func TestReviewFourLiveLocks(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	old := time.Now().Add(-10 * time.Minute)
	backdate := func(p string) {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	check := func(t *testing.T, path string, p procs.Proc) Report {
		rep, err := Run(context.Background(), g, Options{Path: path, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{},
			Procs: func(context.Context) ([]procs.Proc, error) { return []procs.Proc{p}, nil }, Version: "test"})
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}
	t.Run("git working in a submodule holds the submodule's lock", func(t *testing.T) {
		base := t.TempDir()
		src := filepath.Join(base, "src")
		if _, err := fixtures.New(src); err != nil {
			t.Fatal(err)
		}
		super := filepath.Join(base, "super")
		if _, err := fixtures.New(super); err != nil {
			t.Fatal(err)
		}
		gitRun(t, super, []string{"GIT_CONFIG_GLOBAL=/dev/null"}, "-c", "protocol.file.allow=always", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "submodule", "add", "-q", src, "sub")
		backdate(filepath.Join(super, ".git", "modules", "sub", "index.lock"))
		if rep := check(t, super, procs.Proc{PID: 4242, Cwd: filepath.Join(super, "sub")}); slices.Contains(codes(rep), CodeStaleLock) {
			t.Fatalf("live lock in the submodule called stale: %v", codes(rep))
		}
	})
	t.Run("a holder that started a little after the lock's mtime", func(t *testing.T) {
		// network file systems stamp mtimes with the server's clock
		rep := run(t, "stale-lock", func(o *Options) {
			lock := filepath.Join(o.Path, ".git", "refs", "heads", "main.lock")
			info, err := os.Stat(lock)
			if err != nil {
				t.Fatal(err)
			}
			o.Procs = procsStarted(o.Path, info.ModTime().Add(10*time.Second))
		})
		find(t, rep, CodeLockInUse)
	})
	t.Run("core.worktree layout inside an enclosing repository", func(t *testing.T) {
		encl := filepath.Join(t.TempDir(), "encl")
		if _, err := fixtures.New(encl); err != nil {
			t.Fatal(err)
		}
		a := filepath.Join(encl, "a")
		meta := filepath.Join(a, "meta")
		if err := os.MkdirAll(meta, 0o755); err != nil {
			t.Fatal(err)
		}
		gitRun(t, meta, nil, "init", "-q")
		gitRun(t, meta, nil, "config", "core.worktree", a)
		if err := os.WriteFile(filepath.Join(a, "f"), []byte("f\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		id := []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x"}
		gitRun(t, meta, id, "add", "../f")
		gitRun(t, meta, id, "commit", "-qm", "f")
		backdate(filepath.Join(meta, ".git", "index.lock"))
		// git chdirs to the worktree top, a/, where discovery finds encl
		rep := check(t, meta, procs.Proc{PID: 4242, Cwd: a})
		if slices.Contains(codes(rep), CodeStaleLock) || ExitCode(rep) != 0 {
			t.Fatalf("live lock called stale: %v", codes(rep))
		}
	})
}

func TestIntentionalDeleteAllWithIgnoredFilesIsNotDamage(t *testing.T) {
	rep := run(t, "clean", func(o *Options) {
		if err := os.WriteFile(filepath.Join(o.Path, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, o.Path, []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x"}, "add", ".gitignore")
		gitRun(t, o.Path, []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x"}, "commit", "-qm", "ignore")
		if err := os.WriteFile(filepath.Join(o.Path, ".env"), []byte("SECRET=1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, o.Path, nil, "rm", "-r", "-q", ".")
	})
	if slices.Contains(codes(rep), CodeIndexEmpty) {
		t.Fatalf("git rm -r . with an ignored file left is deliberate: %v", codes(rep))
	}
}

func TestCloudOnlyAlternateSkipsObjectReads(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	store := filepath.Join(base, "store")
	if _, err := fixtures.New(store); err != nil {
		t.Fatal(err)
	}
	gitRun(t, store, nil, "gc", "-q")
	clone := filepath.Join(base, "clone")
	if out, err := exec.Command("git", "clone", "-q", "--shared", store, clone).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	rep, err := Run(context.Background(), g, Options{Path: clone, Home: t.TempDir(), GOOS: "linux", Now: time.Now(),
		Flags: someDataless{".pack"}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(codes(rep), CodeHEADCommitMissing) {
		t.Fatalf("a cloud-only alternate made HEAD look missing: %v", codes(rep))
	}
	find(t, rep, CodeAlternateDataless)
}

func TestIntentionalDeleteAllWithNestedIgnoredFiles(t *testing.T) {
	id := []string{"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x"}
	rep := run(t, "clean", func(o *Options) {
		if err := os.MkdirAll(filepath.Join(o.Path, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range map[string]string{"pkg/mod.py": "x\n", ".gitignore": ".DS_Store\n__pycache__/\n"} {
			if err := os.WriteFile(filepath.Join(o.Path, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		gitRun(t, o.Path, id, "add", ".")
		gitRun(t, o.Path, id, "commit", "-qm", "pkg")
		if err := os.WriteFile(filepath.Join(o.Path, "pkg", ".DS_Store"), []byte{0}, 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, o.Path, nil, "rm", "-r", "-q", ".")
	})
	if slices.Contains(codes(rep), CodeIndexEmpty) {
		t.Fatalf("Finder's .DS_Store kept pkg/ on disk, but no tracked file is there: %v", codes(rep))
	}
}

func TestInsideComparesIdentity(t *testing.T) {
	base := filepath.Join(t.TempDir(), "Base")
	if err := os.MkdirAll(filepath.Join(base, "sub", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	lower := filepath.Join(filepath.Dir(base), "base", "sub", "x")
	if _, err := os.Stat(lower); err != nil {
		t.Skip("case-sensitive file system")
	}
	if !inside(lower, base) {
		t.Fatal("a differently spelled path to the same directory must count as inside")
	}
	if inside(base, base) || inside(filepath.Dir(base), base) {
		t.Fatal("not strictly inside")
	}
}

func TestNestedCloudOnlyAlternate(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	store := filepath.Join(base, "store")
	if _, err := fixtures.New(store); err != nil {
		t.Fatal(err)
	}
	gitRun(t, store, nil, "gc", "-q")
	middle := filepath.Join(base, "middle")
	clone := filepath.Join(base, "clone")
	for _, args := range [][]string{{"clone", "-q", "--shared", store, middle}, {"clone", "-q", "--shared", middle, clone}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	rep, err := Run(context.Background(), g, Options{Path: clone, Home: t.TempDir(), GOOS: "linux", Now: time.Now(),
		Flags: someDataless{"/store/.git/objects/pack"}, Procs: noProcs, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	find(t, rep, CodeAlternateDataless)
}

func TestAlternatesCycleEndsQuickly(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "a", "objects"), filepath.Join(base, "b", "objects")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(filepath.Join(d, "info"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// each lists the other eight times: git deduplicates, a naive walk explodes
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		if err := os.WriteFile(filepath.Join(pair[0], "info", "alternates"), []byte(strings.Repeat(pair[1]+"\n", 8)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan string, 1)
	go func() { done <- cloudOnlyAlternateFrom(a, noDataless{}, 0) }()
	select {
	case got := <-done:
		if got != "" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("alternates walk did not finish: cycles must be visited once")
	}
}

func TestHookWithNoGitOnTheRepositoryIsRefused(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	dir, other := filepath.Join(base, "repo"), filepath.Join(base, "other")
	for _, d := range []string{dir, other} {
		if err := fixtures.Build("clean", d); err != nil {
			t.Fatal(err)
		}
	}
	opts := func(index string) Options {
		return Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test", IndexFile: index}
	}
	// The hook's index belongs to another repository and no git works on this one.
	if _, err := Run(context.Background(), g, opts(filepath.Join(other, ".git", "index"))); !errors.Is(err, ErrNotTheHooksRepository) {
		t.Fatalf("a commit hook whose commit works elsewhere must not answer for this repository: %v", err)
	}
	// The hook's index is this repository's: a client that runs hooks itself
	// (libgit2) has no git process, and that is fine.
	if _, err := Run(context.Background(), g, opts(filepath.Join(dir, ".git", "index"))); err != nil {
		t.Fatalf("hook from a client without a git process refused: %v", err)
	}
}

func TestHookCwdAnchorsTheWorktree(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other")
	if err := fixtures.Build("clean", other); err != nil {
		t.Fatal(err)
	}
	// The hook changed into a directory outside any repository; git ran it
	// from dir's top with a relative index.
	away := t.TempDir()
	rep, err := Run(context.Background(), g, Options{Path: away, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{},
		Procs: procsAt(dir), Version: "test", IndexFile: ".git/index", HookCwd: dir})
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(dir); rep.Repo.Worktree != want {
		t.Fatalf("checked %s, want the hook's repository %s", rep.Repo.Worktree, want)
	}
	// Into another repository: .git/index fits that one too, so the question
	// is ambiguous and the check refuses rather than answer for dir.
	_, err = Run(context.Background(), g, Options{Path: other, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{},
		Procs: procsAt(dir), Version: "test", IndexFile: ".git/index", HookCwd: dir})
	if !errors.Is(err, ErrNotTheHooksRepository) {
		t.Fatalf("got %v, want ErrNotTheHooksRepository", err)
	}
	// An anchor that does not verify (its index is not the hook's) is not used.
	rep, err = Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{},
		Procs: procsAt(dir), Version: "test", IndexFile: filepath.Join(dir, ".git", "index"), HookCwd: other})
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(dir); rep.Repo.Worktree != want {
		t.Fatalf("followed an unverified anchor to %s", rep.Repo.Worktree)
	}
}

func TestAnchorTakesOnlyItsOwnWorktreesIndex(t *testing.T) {
	// git puts a commit's index in that worktree's own git directory; another
	// worktree's index under the common directory does not confirm the anchor.
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	linked := filepath.Join(t.TempDir(), "wt")
	if err := fixtures.Build("linked-worktree", linked); err != nil {
		t.Fatal(err)
	}
	main := linked + "-main"
	rMain, err := repo.Locate(context.Background(), g, main)
	if err != nil {
		t.Fatal(err)
	}
	rWT, err := repo.Locate(context.Background(), g, linked)
	if err != nil {
		t.Fatal(err)
	}
	mainIndex := filepath.Join(rMain.GitDir, "index")
	wtIndex := filepath.Join(rWT.GitDir, "index")
	for _, c := range []struct {
		name  string
		r     repo.Repo
		index string
		want  bool
	}{
		{"main, own index", rMain, mainIndex, true},
		{"linked, own index", rWT, wtIndex, true},
		{"linked, main's index", rWT, mainIndex, false},
		{"main, linked worktree's index", rMain, wtIndex, false},
	} {
		if got := anchorAgrees(c.r, nil, c.index); got != c.want {
			t.Errorf("%s: anchorAgrees = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCwdWithARelativeGitDir(t *testing.T) {
	// A .git.nosync repository used through GIT_DIR=.git.nosync, inside an
	// enclosing repository. From a subdirectory git finds no repository (the
	// relative GIT_DIR names nothing there): still the hook's. From a hidden
	// nested repository of the same kind, git uses that one.
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	enc := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"), noMaintenance...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	nosync := func(dir string) {
		t.Helper()
		run(filepath.Dir(dir), "init", "-q", "-b", "main", filepath.Base(dir))
		if err := os.Rename(filepath.Join(dir, ".git"), filepath.Join(dir, ".git.nosync")); err != nil {
			t.Fatal(err)
		}
	}
	run(enc, "init", "-q", "-b", "main")
	wt := filepath.Join(enc, "r")
	if err := os.MkdirAll(filepath.Join(wt, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	nosync(wt)
	nested := filepath.Join(wt, "n")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	nosync(nested)
	r, err := repo.Locate(context.Background(), g, wt, "GIT_DIR="+filepath.Join(wt, ".git.nosync"))
	if err != nil {
		t.Fatal(err)
	}
	if !cwdAgrees(context.Background(), g, r, filepath.Join(wt, "d"), wt, ".git.nosync", noDataless{}) {
		t.Error("a subdirectory where GIT_DIR names nothing was refused")
	}
	if cwdAgrees(context.Background(), g, r, nested, wt, ".git.nosync", noDataless{}) {
		t.Error("a nested repository GIT_DIR names was taken for this one")
	}
}

func TestARebaseDirectoryWhileGitRunsIsNotStale(t *testing.T) {
	// git creates rebase-merge before the files in it: while a git works on
	// the repository, a half-written directory is not a leftover.
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("stale-rebase", dir); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		procs func(context.Context) ([]procs.Proc, error)
		stale bool
	}{{procsAt(dir), false}, {noProcs, true}} {
		rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: c.procs, Version: "test"})
		if err != nil {
			t.Fatal(err)
		}
		if got := hasCode(rep.Findings, CodeStaleOperation); got != c.stale {
			t.Errorf("git running %v: STALE_OPERATION_STATE %v, want %v", !c.stale, got, c.stale)
		}
	}
}

func TestATempFileOfARunningProcessIsNotSafeToDelete(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		pid  int
		safe bool
	}{{os.Getpid(), false}, {999999, true}} {
		p := filepath.Join(dir, ".git", fmt.Sprintf("next-index-%d.lock", c.pid))
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: noProcs, Version: "test"})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range rep.Findings {
			if f.Code == CodeGitTempFile {
				if got := strings.Contains(f.Message, "safe to delete"); got != c.safe {
					t.Errorf("pid %d: %q", c.pid, f.Message)
				}
			}
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnOldRebaseDirectoryIsStaleWhileAnotherGitRuns(t *testing.T) {
	// A git that started after the directory was written (the commit running
	// this hook) cannot be the one writing it.
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("stale-rebase", dir); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, ".git", "rebase-merge"), old, old); err != nil {
		t.Fatal(err)
	}
	fresh := func(context.Context) ([]procs.Proc, error) {
		return []procs.Proc{{PID: 4242, Cwd: dir, Start: time.Now()}}, nil
	}
	rep, err := Run(context.Background(), g, Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(), Flags: noDataless{}, Procs: fresh, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCode(rep.Findings, CodeStaleOperation) {
		t.Errorf("a day-old rebase directory was excused by a git that started now: %v", rep.Findings)
	}
}
