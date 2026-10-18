package scan

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/cloud"
	"github.com/aliyilmazco/gitbrella/internal/fixtures"
)

type fakeFlags map[string]bool // keyed by path suffix

func (f fakeFlags) IsDataless(p string) (bool, error) {
	for suffix, v := range f {
		if strings.HasSuffix(filepath.ToSlash(p), suffix) {
			return v, nil
		}
	}
	return false, nil
}

func scanScenario(t *testing.T, name string, fr FlagReader) Result {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build(name, dir); err != nil {
		t.Fatal(err)
	}
	res, err := Scan(filepath.Join(dir, ".git"), fr, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func copyRels(res Result) []string {
	var out []string
	for _, c := range res.Copies {
		out = append(out, c.Rel)
	}
	return out
}

func TestCleanRepoHasNoFindings(t *testing.T) {
	res := scanScenario(t, "clean", fakeFlags{})
	if len(res.Copies)+len(res.Locks)+len(res.Dataless)+len(res.Unknown)+len(res.Unreadable) != 0 {
		t.Fatalf("clean repo produced findings: %+v", res)
	}
}

func TestFindsCopies(t *testing.T) {
	cases := map[string][]string{
		"index-copies":   {"index 2", "index 3"},
		"ref-copy-older": {"refs/heads/main 2"},
		"dropbox-copy":   {"HEAD (Fixture's conflicted copy 2026-10-08)"},
		"dir-copy":       {"refs 2"},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			got := copyRels(scanScenario(t, name, fakeFlags{}))
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Fatalf("copies = %v, want %v", got, want)
			}
		})
	}
}

func TestDirCopyIsReportedOnce(t *testing.T) {
	res := scanScenario(t, "dir-copy", fakeFlags{})
	if len(res.Copies) != 1 || !res.Copies[0].IsDir || res.Copies[0].Kind != cloud.KindICloud || res.Copies[0].OriginalRel != "refs" {
		t.Fatalf("unexpected copies: %+v", res.Copies)
	}
}

func TestCopyOfMissingGitFileStillCounts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("index-copies", dir); err != nil {
		t.Fatal(err)
	}
	r := &fixtures.Repo{Dir: dir, GitDir: filepath.Join(dir, ".git")}
	if err := r.RenameFile("index", "index.lock"); err != nil {
		t.Fatal(err)
	}
	res, err := Scan(r.GitDir, fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Copies {
		if c.OriginalExists {
			t.Fatalf("index is missing, but %s claims its original exists", c.Rel)
		}
	}
	if len(res.Copies) != 2 {
		t.Fatalf("want 2 copies, got %+v", res.Copies)
	}
}

func TestThirdPartyFilesAreUnknownNotLocks(t *testing.T) {
	res := scanScenario(t, "third-party", fakeFlags{})
	slices.Sort(res.Unknown)
	if !slices.Equal(res.Unknown, []string{"notesync-transactions", "notesync.lock"}) {
		t.Fatalf("unknown = %v", res.Unknown)
	}
	if len(res.Locks) != 0 {
		t.Fatalf("third-party lock reported as git lock: %+v", res.Locks)
	}
}

func TestGitLocksAndAge(t *testing.T) {
	res := scanScenario(t, "stale-lock", fakeFlags{})
	if len(res.Locks) != 1 || res.Locks[0].Rel != "refs/heads/main.lock" || res.Locks[0].Age < 9*time.Minute {
		t.Fatalf("unexpected locks: %+v", res.Locks)
	}
	res = scanScenario(t, "orphan-index-lock", fakeFlags{})
	if len(res.Locks) != 1 || res.Locks[0].Rel != "index.lock" {
		t.Fatalf("unexpected locks: %+v", res.Locks)
	}
}

func TestDatalessIsReportedAndNotDescended(t *testing.T) {
	res := scanScenario(t, "clean", fakeFlags{"/.git/refs": true, "/.git/index": true})
	slices.Sort(res.Dataless)
	if !slices.Equal(res.Dataless, []string{"index", "refs"}) {
		t.Fatalf("dataless = %v", res.Dataless)
	}
}

func TestIsGitName(t *testing.T) {
	for _, n := range []string{"index", "index.lock", "HEAD", "refs/heads/main", "BISECT_LOG", "objects/pack/x.pack", "sharedindex.abc", "REBASE_HEAD", "fsmonitor--daemon.ipc"} {
		if !IsGitName(n) {
			t.Errorf("IsGitName(%q) = false", n)
		}
	}
	for _, n := range []string{"notesync.lock", "notesync-transactions", "notes.md", "foo/bar"} {
		if IsGitName(n) {
			t.Errorf("IsGitName(%q) = true", n)
		}
	}
}

func TestForeignAndEmptyFiles(t *testing.T) {
	res := scanScenario(t, "desktop-ini", fakeFlags{})
	slices.Sort(res.Foreign)
	if !slices.Equal(res.Foreign, []string{"objects/desktop.ini", "refs/heads/desktop.ini"}) {
		t.Fatalf("foreign = %v", res.Foreign)
	}
	res = scanScenario(t, "foreign-object", fakeFlags{})
	if len(res.Foreign) != 1 || !strings.HasSuffix(res.Foreign[0], "-safeBackup-0001") {
		t.Fatalf("foreign = %v", res.Foreign)
	}
	res = scanScenario(t, "empty-ref", fakeFlags{})
	if !slices.Equal(res.Empty, []string{"refs/heads/main"}) {
		t.Fatalf("empty = %v", res.Empty)
	}
	res = scanScenario(t, "empty-object", fakeFlags{})
	if len(res.Empty) != 1 || !strings.HasPrefix(res.Empty[0], "objects/") {
		t.Fatalf("empty = %v", res.Empty)
	}
}

func TestBrokenWorktreeAdmin(t *testing.T) {
	res := scanScenario(t, "broken-worktree-admin", fakeFlags{})
	if len(res.BrokenWorktrees) != 1 || !strings.HasPrefix(res.BrokenWorktrees[0], "worktrees/") {
		t.Fatalf("broken = %v", res.BrokenWorktrees)
	}
}

func TestScanWorktree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("worktree-copies", dir); err != nil {
		t.Fatal(err)
	}
	r := &fixtures.Repo{Dir: dir, GitDir: filepath.Join(dir, ".git")}
	if err := r.CopyDir(".", "../.git 2"); err != nil {
		t.Fatal(err)
	}
	res, err := ScanWorktree(dir, fakeFlags{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Copies, []string{"note 2.md"}) || !slices.Equal(res.GitDirs, []string{".git 2"}) || res.Truncated {
		t.Fatalf("worktree = %+v", res)
	}
	res, err = ScanWorktree(dir, fakeFlags{}, 1)
	if err != nil || !res.Truncated {
		t.Fatalf("limit must truncate: %+v %v", res, err)
	}
}

func TestGitOwnedObjectFilesAreNotForeign(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"objects/maintenance.lock", "objects/pack/multi-pack-index.lock", "objects/pack/.tmp-4242-pack-abc.pack"} {
		if err := r.WriteFile(rel, "x"); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Scan(r.GitDir, fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Foreign) != 0 {
		t.Fatalf("git's own files reported as foreign: %v", res.Foreign)
	}
	var locks []string
	for _, l := range res.Locks {
		locks = append(locks, l.Rel)
	}
	slices.Sort(locks)
	if want := []string{"objects/maintenance.lock", "objects/pack/multi-pack-index.lock"}; !slices.Equal(locks, want) {
		t.Fatalf("locks = %v, want %v", locks, want)
	}
}

func TestCopiesOfThirdPartyFilesAreUnknown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"notesync.lock", "notesync 2.lock", "notesync-transactions/a", "notesync-transactions 2/a"} {
		if err := r.WriteFile(rel, "x"); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Scan(r.GitDir, fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Copies) != 0 {
		t.Fatalf("copies of third-party files reported as git damage: %v", copyRels(res))
	}
	for _, want := range []string{"notesync 2.lock", "notesync-transactions 2"} {
		if !slices.Contains(res.Unknown, want) {
			t.Errorf("%s not reported as unknown: %v", want, res.Unknown)
		}
	}
}

func TestUserNamedEntriesNeedTheirOriginal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A submodule whose own name ends in " 5" is not a conflict copy.
	if err := r.WriteFile("modules/vendor/Font Awesome 5/HEAD", "ref: refs/heads/main\n"); err != nil {
		t.Fatal(err)
	}
	res, err := Scan(r.GitDir, fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Copies) != 0 {
		t.Fatalf("submodule name reported as a copy: %v", copyRels(res))
	}
}

func TestTrackedFolderNamedLikeGitIsNotAGitDirCopy(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(r.Dir, "git 2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Dir, "git 2", "notes.md"), []byte("about git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ScanWorktree(r.Dir, fakeFlags{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GitDirs) != 0 {
		t.Fatalf("ordinary folder reported as a git directory copy: %v", res.GitDirs)
	}
}

func TestGitHousekeepingNamesAreKnown(t *testing.T) {
	for _, n := range []string{"gc.pid", "gc.log", "next-index-4242.lock", "index.stash.4242", "MERGE_AUTOSTASH"} {
		if !IsGitName(n) {
			t.Errorf("IsGitName(%q) = false", n)
		}
	}
}

func TestScanWorktreeSkipsGivenDirs(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{".git.nosync/HEAD", ".git.nosync/HEAD 2", "notes.md"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := ScanWorktree(dir, fakeFlags{}, 0, ".git.nosync")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Copies) != 0 {
		t.Fatalf("descended into a skipped directory: %v", res.Copies)
	}
}

func TestPidNamedTempFilesAreNotLocks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"next-index-4242.lock", "index.stash.4242", "index.stash.4242.lock"} {
		if err := r.WriteFile(rel, "x"); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Scan(r.GitDir, fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Locks) != 0 {
		t.Fatalf("PID-named temp files judged as locks: %+v", res.Locks)
	}
	if want := []string{"index.stash.4242", "index.stash.4242.lock", "next-index-4242.lock"}; !slices.Equal(res.Temp, want) {
		t.Fatalf("temp = %v, want %v", res.Temp, want)
	}
}

func TestIncrementalMultiPackIndexIsGits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"objects/pack/multi-pack-index.d/multi-pack-index-4ddab9dda0abcf64f30f5ac77d3e8400d7062e80.midx",
		"objects/pack/multi-pack-index.d/multi-pack-index-chain",
	} {
		if err := r.WriteFile(rel, "x"); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Scan(r.GitDir, fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Foreign) != 0 {
		t.Fatalf("git's incremental MIDX reported as foreign: %v", res.Foreign)
	}
}

func TestIncrementalMultiPackIndexTempFileIsGits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	r, err := fixtures.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile("objects/pack/multi-pack-index.d/tmp_midx_AbC123", "x"); err != nil {
		t.Fatal(err)
	}
	res, err := Scan(r.GitDir, fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Foreign) != 0 {
		t.Fatalf("git's tmp_midx_ file reported as foreign: %v", res.Foreign)
	}
}

func TestGitsOwnTemporaryObjectFilesAreNotDamage(t *testing.T) {
	// git writes loose objects through objects/xx/tmp_obj_*, empty at first,
	// and gc's bitmap step leaves objects/bitmap-ref-tips_* while it runs.
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	objects := filepath.Join(dir, ".git", "objects")
	if err := os.MkdirAll(filepath.Join(objects, "ab"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(objects, "ab", "tmp_obj_Xy12Ab"), filepath.Join(objects, "bitmap-ref-tips_q1W2e3"),
		filepath.Join(objects, "pack", "tmp_pack_Zx9"), filepath.Join(objects, "pack", ".tmp-4242-pack-abc.pack")} {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Scan(filepath.Join(dir, ".git"), fakeFlags{}, time.Now(), cloud.None)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Empty) != 0 || len(res.Foreign) != 0 {
		t.Fatalf("empty = %v, foreign = %v", res.Empty, res.Foreign)
	}
}
