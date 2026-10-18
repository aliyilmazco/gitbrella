package fix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/scan"
)

// scenario builds a damaged repository from the check's fixtures (or
// "index-missing": a clean one without its index) and returns it with its
// check report.
func scenario(t *testing.T, name string) (*gitx.Runner, repo.Repo, check.Report) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_COUNT", "1") // no detached "git maintenance" after commits
	t.Setenv("GIT_CONFIG_KEY_0", "maintenance.auto")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo with space")
	build := name
	if name == "index-missing" { // the check's fixtures have no such one
		build = "clean"
	}
	if err := fixtures.Build(build, dir); err != nil {
		t.Fatal(err)
	}
	if name == "index-missing" {
		if err := os.Remove(filepath.Join(dir, ".git", "index")); err != nil {
			t.Fatal(err)
		}
	}
	r, err := repo.Locate(context.Background(), g, dir)
	if err != nil {
		t.Fatal(err)
	}
	return g, r, report(t, g, dir)
}

// report runs the check on the repository at dir as it is now.
func report(t *testing.T, g *gitx.Runner, dir string) check.Report {
	t.Helper()
	rep, err := check.Run(context.Background(), g, check.Options{Path: dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(),
		Flags: scan.DefaultFlagReader(), Procs: func(context.Context) ([]procs.Proc, error) { return nil, nil }, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

// git runs git in r with stdin and returns its output; the test fails when
// git does.
func git(t *testing.T, g *gitx.Runner, r repo.Repo, stdin string, args ...string) string {
	t.Helper()
	env := append(r.Env(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	res, err := g.RunInput(context.Background(), r.Worktree, env, stdin, args...)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("git %v: %v %s", args, err, res.Stderr)
	}
	return strings.TrimSpace(res.Stdout)
}

// write puts content at rel in the git directory, ten minutes old: left
// behind, not being written.
func write(t *testing.T, r repo.Repo, rel, content string) {
	t.Helper()
	p := filepath.Join(r.CommonDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
}

// manualFor returns what the plan leaves to the user for code.
func manualFor(ms []Manual, code string) (Manual, bool) {
	for _, m := range ms {
		if m.Code == code {
			return m, true
		}
	}
	return Manual{}, false
}

func kinds(as []Action) string {
	var out []string
	for _, a := range as {
		out = append(out, a.Kind)
	}
	return strings.Join(out, ",")
}

func TestActionsForEachScenario(t *testing.T) {
	for name, want := range map[string]string{
		"clean":                 "",
		"index-copies":          "quarantine,quarantine",
		"orphan-index-lock":     "restore-index",
		"ref-copy-older":        "quarantine",
		"ref-copy-newer":        "rescue-ref,quarantine",
		"dropbox-copy":          "quarantine",
		"stale-lock":            "",
		"corrupt-index":         "",
		"index-missing":         "rebuild-index",
		"third-party":           "",
		"dir-copy":              "quarantine",
		"syncthing-copy":        "quarantine",
		"desktop-ini":           "quarantine,quarantine",
		"foreign-object":        "",
		"empty-object":          "",
		"empty-ref":             "set-ref",
		"stale-rebase":          "quarantine",
		"gitdir-copy":           "quarantine",
		"broken-worktree-admin": "",
		"nosync-unignored":      "add-exclude",
	} {
		t.Run(name, func(t *testing.T) {
			g, r, rep := scenario(t, name)
			as, _, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
			if err != nil {
				t.Fatal(err)
			}
			if got := kinds(as); got != want {
				t.Fatalf("actions %q, want %q (findings %v)", got, want, rep.Findings)
			}
			for _, a := range as {
				if a.Kind != RescueRef && !filepath.IsAbs(a.Path) {
					t.Fatalf("%s: path %q is not absolute", a.Kind, a.Path)
				}
			}
		})
	}
}

func TestRescueNamesTheCopy(t *testing.T) {
	g, r, rep := scenario(t, "ref-copy-newer")
	as, _, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if as[0].Ref != "refs/gcd-rescue/heads/main-2" || len(as[0].Value) != 40 {
		t.Fatalf("rescue %+v", as[0])
	}
}

func TestSparseCheckoutLeavesTheIndexToTheUser(t *testing.T) {
	g, r, rep := scenario(t, "index-missing")
	as, manual, err := actionsFor(context.Background(), g, r, rep, true, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 || len(manual) != 1 || !strings.Contains(manual[0].Do, "sparse checkout") {
		t.Fatalf("actions %v, manual %+v", as, manual)
	}
}

func TestShrunkIndexIsManual(t *testing.T) {
	g, r, rep := scenario(t, "clean")
	rep.Findings = append(rep.Findings, check.Finding{Code: check.CodeIndexShrunk, Severity: check.Damage, Path: "index"})
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil || len(as) != 0 || len(manual) != 1 || !strings.Contains(manual[0].Do, "git reset --mixed") {
		t.Fatalf("actions %v manual %+v err %v", as, manual, err)
	}
}

// A copy whose original is gone is the only copy: moving it away would lose
// the ref. fix says how to put it back instead.
func TestCopyWithoutItsOriginalIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	write(t, r, "refs/tags/v1 2", git(t, g, r, "", "rev-parse", "HEAD")+"\n")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := manualFor(manual, check.CodeRefCopy)
	if len(as) != 0 || !ok || !strings.Contains(m.Do, "mv ") {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// A branch lock git wrote the new tip into and died: the tip may be the only
// pointer to a commit, and git elsewhere may still hold it. fix leaves it.
func TestStaleRefLockIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	write(t, r, "refs/heads/main.lock", git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "tip")+"\n")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manualFor(manual, check.CodeStaleLock); len(as) != 0 || !ok {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// An index lock that is a valid index of an older HEAD would quietly stage
// the reversal of every later commit. Only one equal to HEAD becomes the
// index.
func TestOldIndexLockIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	old, err := os.ReadFile(filepath.Join(r.GitDir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(r.Worktree, "note.md"), []byte("three\n"), 0o644)
	git(t, g, r, "", "commit", "-q", "-am", "three")
	os.Remove(filepath.Join(r.GitDir, "index"))
	write(t, r, "index.lock", string(old))
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manualFor(manual, check.CodeOrphanIndexLock); len(as) != 0 || !ok {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// Without a commit there is nothing to rebuild the index from (and check
// sees nothing wrong with a fresh repository without one).
func TestUnbornHeadLeavesTheIndexToTheUser(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	git(t, g, r, "", "checkout", "-q", "--orphan", "fresh")
	os.Remove(filepath.Join(r.GitDir, "index"))
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
	if _, ok := manualFor(manual, check.CodeIndexMissing); len(rep.Findings) > 0 && !ok {
		t.Fatalf("manual %+v (findings %+v)", manual, rep.Findings)
	}
}

// Rescue names are refs git accepts, never one that exists already.
func TestRescueNamesAreValidAndFree(t *testing.T) {
	g, r, _ := scenario(t, "ref-copy-newer")
	git(t, g, r, "", "update-ref", "refs/gcd-rescue/heads/main-2", "HEAD")
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "in a copy named like a lock")
	write(t, r, "refs/heads/topic 2.lock", c+"\n")
	rep := report(t, g, r.Worktree)
	as, _, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.Kind != RescueRef {
			continue
		}
		if a.Ref == "refs/gcd-rescue/heads/main-2" {
			t.Fatalf("rescue reuses an existing ref: %+v", a)
		}
		git(t, g, r, "", "check-ref-format", a.Ref)
	}
	if as[0].Ref != "refs/gcd-rescue/heads/main-2-2" {
		t.Fatalf("actions %+v", as)
	}
}

// A copy of a lock is never put back as the lock: that would stop git. Its
// commit is kept as a rescue ref, then the copy goes into the backup.
func TestCopyOfALockIsRescuedNotPutBack(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "tip in a lock copy")
	write(t, r, "refs/heads/main 2.lock", c+"\n")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "rescue-ref,quarantine" || p.Actions[0].Value != c {
		t.Fatalf("plan %+v %v", p, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	if got := git(t, g, r, "", "rev-parse", p.Actions[0].Ref); got != c {
		t.Fatalf("rescue ref %s = %s", p.Actions[0].Ref, got)
	}
	if exists(filepath.Join(r.CommonDir, "refs", "heads", "main.lock")) {
		t.Fatal("the copy was put back as the lock")
	}
}

// A damaged index git may still read holds whatever is staged: rebuilding
// it from HEAD would drop that work (and gc then its blobs). fix leaves it.
func TestDamagedIndexIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	os.WriteFile(filepath.Join(r.Worktree, "note.md"), []byte("staged\n"), 0o644)
	os.WriteFile(filepath.Join(r.Worktree, "new.md"), []byte("only staged\n"), 0o644)
	git(t, g, r, "", "add", "note.md", "new.md")
	os.Remove(filepath.Join(r.Worktree, "new.md"))
	p := filepath.Join(r.GitDir, "index")
	b, _ := os.ReadFile(p)
	b[len(b)-1] ^= 0xff // only the checksum: git status still reads it
	os.WriteFile(p, b, 0o644)
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manualFor(manual, check.CodeIndexInvalid); len(as) != 0 || !ok {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// An index copy that differs from both the index and HEAD may be the one
// holding the staged work: fix leaves it. One identical to the index is
// moved away as before ("index-copies" above).
func TestIndexCopyWithStagedWorkIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	os.WriteFile(filepath.Join(r.Worktree, "new.md"), []byte("only staged\n"), 0o644)
	git(t, g, r, "", "add", "new.md")
	staged, _ := os.ReadFile(filepath.Join(r.GitDir, "index"))
	git(t, g, r, "", "reset", "-q")
	write(t, r, "index 2", string(staged))
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := manualFor(manual, check.CodeIndexCopy); len(as) != 0 || !ok || !strings.Contains(m.Do, "staged work") {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// Copies under objects/ are left to the user: which of the two holds the
// good bytes is not fix's to say, and an empty object with a copy of it is
// left too.
func TestObjectCopiesAreManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	oid := git(t, g, r, "", "rev-parse", "HEAD:note.md")
	obj := filepath.Join(r.CommonDir, "objects", oid[:2], oid[2:])
	good, err := os.ReadFile(obj)
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(obj, 0o644)
	os.WriteFile(obj, nil, 0o644)
	write(t, r, "objects/"+oid[:2]+"/"+oid[2:]+" 2", string(good))
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 || len(manual) == 0 {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
	for _, m := range manual {
		if strings.Contains(m.Do, "--refetch") {
			t.Fatalf("refetch offered for local bytes: %+v", m)
		}
	}
}

// An empty branch file with a copy of the branch next to it: the reflog's
// commit and the copy's may differ, so neither is chosen.
func TestEmptyRefWithACopyIsManual(t *testing.T) {
	g, r, _ := scenario(t, "empty-ref")
	write(t, r, "refs/heads/main 2", git(t, g, r, "", "commit-tree", git(t, g, r, "", "write-tree"), "-m", "elsewhere")+"\n")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 || len(manual) != 2 {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// A copy naming more objects than fix checks one by one (a long reflog) is
// left to the user rather than failing the plan.
func TestACopyNamingManyObjectsIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	var b strings.Builder
	for i := 0; i <= maxRescues; i++ {
		fmt.Fprintf(&b, "%040x %040x t <t@t> 0 +0000\tline %d\n", i, i+1, i)
	}
	write(t, r, "logs/HEAD 2", b.String())
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := manualFor(manual, check.CodeCopy); len(as) != 0 || !ok || !strings.Contains(m.Do, "more than") {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// A stale branch lock's advice names the commit in it, so moving the lock
// away does not quietly drop the only pointer to it.
func TestStaleLockAdviceNamesItsCommit(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	c := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "tip")
	write(t, r, "refs/heads/main.lock", c+"\n")
	rep := report(t, g, r.Worktree)
	_, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := manualFor(manual, check.CodeStaleLock); !ok || !strings.Contains(m.Do, "git branch <name> "+c) {
		t.Fatalf("manual %+v", manual)
	}
}

// A rescue ref named like a directory of the new one (refs-2 and refs-2/1)
// cannot coexist with it: the name moves on instead of failing every fix.
func TestRescueNamesDoNotClashAsDirectories(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	git(t, g, r, "", "update-ref", "refs/gcd-rescue/refs-2", "HEAD")
	c1 := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "one")
	c2 := git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "two")
	write(t, r, "refs 2/heads/a", c1+"\n")
	write(t, r, "refs 2/heads/b", c2+"\n")
	rep := report(t, g, r.Worktree)
	as, _, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.Kind == RescueRef && strings.HasPrefix(a.Ref, "refs/gcd-rescue/refs-2/") {
			t.Fatalf("rescue under an existing ref: %+v", a)
		}
	}
	if kinds(as) != "rescue-ref,rescue-ref,quarantine" {
		t.Fatalf("actions %+v (findings %+v)", as, rep.Findings)
	}
}

// A copy of the git directory with a commit the repository does not have
// may be the current one: fix leaves the choice to the user. An identical
// copy ("gitdir-copy" above) is moved away as before.
func TestGitDirCopyAheadIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	cp := filepath.Join(r.Worktree, ".git 2")
	if out, err := exec.Command("cp", "-R", r.GitDir, cp).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	in := repo.Repo{Worktree: r.Worktree, GitDir: cp, CommonDir: cp}
	c := git(t, g, in, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "only in the copy")
	git(t, g, in, "", "update-ref", "refs/heads/main", c)
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := manualFor(manual, check.CodeGitDirCopy); len(as) != 0 || !ok || !strings.Contains(m.Do, c) {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// shell runs a sh script in dir, with test identities, for setups git has no
// single command for (a submodule with work in it).
func shell(t *testing.T, dir, script string) string {
	t.Helper()
	cmd := exec.Command("sh", "-c", "set -e\n"+script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// withSubmodule adds a submodule "sub" to the repository in r.
func withSubmodule(t *testing.T, r repo.Repo, setup string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "subsrc")
	shell(t, filepath.Dir(src), "git init -q -b main subsrc && cd subsrc && echo s > s.txt && git add s.txt && git commit -qm s")
	shell(t, r.Worktree, "git -c protocol.file.allow=always submodule add -q '"+src+"' sub\n"+setup+"\ngit commit -qm 'add sub'")
}

// A branch whose name git also gives to junk files (desktop.ini) is still a
// branch git reads: fix leaves it.
func TestABranchNamedLikeAForeignFileIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	git(t, g, r, "", "branch", "desktop.ini")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manualFor(manual, check.CodeForeign); len(as) != 0 || !ok {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// An empty branch file whose directory has a copy (iCloud "refs/heads 2",
// Syncthing "refs/heads.sync-conflict-...") with another device's tip: which
// commit is right is not fix's to say.
func TestEmptyRefWithADirectoryCopyIsManual(t *testing.T) {
	for _, dir := range []string{"refs/heads 2", "refs/heads.sync-conflict-20261014-120000-ABCDEF1"} {
		t.Run(dir, func(t *testing.T) {
			g, r, _ := scenario(t, "empty-ref")
			c := git(t, g, r, "", "commit-tree", git(t, g, r, "", "write-tree"), "-m", "device B")
			write(t, r, dir+"/main", c+"\n")
			rep := report(t, g, r.Worktree)
			as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
			if err != nil {
				t.Fatal(err)
			}
			if len(as) != 0 || len(manual) < 2 {
				t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
			}
		})
	}
}

// Damage inside a submodule's git directory (an index copy holding staged
// work, a ref copy naming a commit only it names): the superproject cannot
// read those objects, so fix leaves it all to a run in the submodule.
func TestSubmoduleDamageIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withSubmodule(t, r, "")
	shell(t, r.Worktree, `M=.git/modules/sub
echo staged > sub/staged.txt && git -C sub add staged.txt
mv $M/index "$M/index 2" && git -C sub read-tree HEAD && rm sub/staged.txt
git -C sub commit-tree HEAD^{tree} -p HEAD -m only > "$M/refs/heads/main 2"`)
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 || len(manual) < 2 {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
	for _, m := range manual {
		if !strings.HasPrefix(m.Path, "modules/") || !strings.Contains(m.Do, "submodule") {
			t.Fatalf("manual %+v", m)
		}
	}
}

// An index copy whose only difference is a staged submodule commit, with the
// submodule set to ignore=all: the comparison with HEAD still sees it.
func TestIndexCopyWithAnIgnoredSubmoduleBumpIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withSubmodule(t, r, "git config -f .gitmodules submodule.sub.ignore all && git add .gitmodules")
	shell(t, r.Worktree, `git -C sub commit -q --allow-empty -m bump && git add --force sub
mv .git/index ".git/index 2" && git read-tree HEAD`)
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manualFor(manual, check.CodeIndexCopy); len(as) != 0 || !ok {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// A ref copy naming a commit that has not arrived yet (refs can come before
// their objects): no rescue can keep it, so the copy stays.
func TestRefCopyNamingAMissingCommitIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	write(t, r, "refs/heads/main 2", strings.Repeat("ab", 20)+"\n")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manualFor(manual, check.CodeRefCopy); len(as) != 0 || !ok {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// Whatever may hold an object's bytes stays: an emptied object beside a file
// named after it ("<hash>-safeBackup-0001"), a copy of the whole object
// directory.
func TestObjectStoreLeftoversAreManual(t *testing.T) {
	t.Run("safe-backup", func(t *testing.T) {
		g, r, _ := scenario(t, "clean")
		oid := git(t, g, r, "", "rev-parse", "HEAD:note.md")
		obj := filepath.Join(r.CommonDir, "objects", oid[:2], oid[2:])
		good, _ := os.ReadFile(obj)
		os.Chmod(obj, 0o644)
		os.WriteFile(obj, nil, 0o644)
		write(t, r, "objects/"+oid[:2]+"/"+oid[2:]+"-safeBackup-0001", string(good))
		rep := report(t, g, r.Worktree)
		as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
		if err != nil || len(as) != 0 || len(manual) == 0 {
			t.Fatalf("actions %v, manual %+v, %v (findings %+v)", as, manual, err, rep.Findings)
		}
	})
	t.Run("objects-copy", func(t *testing.T) {
		g, r, _ := scenario(t, "clean")
		shell(t, r.CommonDir, `cp -R objects "objects 2"`)
		rep := report(t, g, r.Worktree)
		as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
		if err != nil || len(as) != 0 || len(manual) == 0 {
			t.Fatalf("actions %v, manual %+v, %v (findings %+v)", as, manual, err, rep.Findings)
		}
	})
}

// A rescue name whose parent is already a rescue ref (heads/feature-2 there,
// heads/feature-2/login-2 wanted) moves to a name without directories
// instead of failing the plan.
func TestRescueNameWithATakenParent(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	git(t, g, r, "", "update-ref", "refs/gcd-rescue/heads/feature-2", "HEAD")
	git(t, g, r, "", "branch", "feature-2/login")
	write(t, r, "refs/heads/feature-2/login 2", git(t, g, r, "", "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "c2")+"\n")
	rep := report(t, g, r.Worktree)
	as, _, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil || kinds(as) != "rescue-ref,quarantine" {
		t.Fatalf("actions %+v %v (findings %+v)", as, err, rep.Findings)
	}
	if strings.HasPrefix(as[0].Ref, "refs/gcd-rescue/heads/feature-2/") {
		t.Fatalf("rescue under an existing ref: %s", as[0].Ref)
	}
	git(t, g, r, "", "check-ref-format", as[0].Ref)
}

// The exclude line for a git directory named with glob characters matches
// that directory only.
func TestExcludeLineTakesGlobsLiterally(t *testing.T) {
	if got := excludeLine("x[1]*?"); got != `/x\[1]\*\?/` {
		t.Fatalf("%q", got)
	}
	g, r, _ := scenario(t, "clean")
	os.MkdirAll(filepath.Join(r.CommonDir, "info"), 0o755)
	os.WriteFile(filepath.Join(r.CommonDir, "info", "exclude"), []byte(excludeLine("x[1]")+"\n"), 0o644)
	for dir, want := range map[string]int{"x[1]": 0, "x1": 1} {
		os.MkdirAll(filepath.Join(r.Worktree, dir), 0o755)
		os.WriteFile(filepath.Join(r.Worktree, dir, "f"), nil, 0o644)
		res, _ := g.Run(context.Background(), r.Worktree, r.Env(), "check-ignore", "-q", dir+"/f")
		if res.ExitCode != want {
			t.Fatalf("%s: check-ignore exit %d, want %d", dir, res.ExitCode, want)
		}
	}
}

// In a reftable repository, a copy of the table list is the only thing that
// names another device's tables: fix leaves it, and the unlisted tables, to
// the user.
func TestReftableCopiesAreManual(t *testing.T) {
	if ok, _ := fixtures.GitAtLeast(2, 45); !ok {
		t.Skip("reftable needs git 2.45")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	fr, err := fixtures.New(filepath.Join(t.TempDir(), "repo"), "--ref-format=reftable")
	if err != nil {
		t.Fatal(err)
	}
	r, err := repo.Locate(context.Background(), g, fr.Dir)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := os.ReadFile(filepath.Join(r.CommonDir, "reftable", "tables.list"))
	write(t, r, "reftable/tables 2.list", string(list)+"0x000000000001-0x000000000009-0a1b2c3d.ref\n")
	write(t, r, "reftable/0x000000000001-0x000000000009-0a1b2c3d.ref", "REFT")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 || len(manual) != 2 {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// A rescue ref's name is never one the check reads as a copy, by any scheme
// it knows: the copy's own name is a valid ref name for Syncthing and for
// Nextcloud's older scheme.
func TestRescueNameIsNoCopyName(t *testing.T) {
	for rel, want := range map[string]string{
		"refs/heads/main.sync-conflict-20261014-120000-ABCDEF1": "heads/main-sync-conflict-20261014-120000-ABCDEF1",
		"refs/heads/main_conflict-20261014-120000":              "heads/main-conflict-20261014-120000",
		"refs/heads/main 2": "heads/main-2",
	} {
		if got := rescueName(rel); got != want {
			t.Errorf("%s: %s, want %s", rel, got, want)
		}
	}
}

// A reflog's first line names the null id ("created from nothing"): no object
// to rescue, and a copy of the reflog goes like any other copy.
func TestReflogCopyIsSetAside(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	b, err := os.ReadFile(filepath.Join(r.CommonDir, "logs", "HEAD"))
	if err != nil || !strings.HasPrefix(string(b), strings.Repeat("0", 40)) {
		t.Fatalf("no reflog to copy: %v %q", err, b)
	}
	write(t, r, "logs/HEAD 2", string(b))
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(manual) != 0 || kinds(as) != "quarantine" {
		t.Fatalf("actions %v, manual %+v", as, manual)
	}
}

// A ref some HEAD names is the branch checked out there, whatever its name
// looks like: fix leaves it (moving it would leave HEAD unborn).
func TestACheckedOutRefCopyIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	if err := os.WriteFile(filepath.Join(filepath.Dir(r.Worktree), ".sync_ab12cd34ef56.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, g, r, "", "branch", "release")
	git(t, g, r, "", "checkout", "-q", "-b", "release_conflict-20240101-120000")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 || len(manual) != 2 || !strings.Contains(manual[0].Do, "checked out") || !strings.Contains(manual[1].Do, "checked out") {
		t.Fatalf("actions %v, manual %+v (findings %+v)", as, manual, rep.Findings)
	}
}

// A HEAD that is a symlink (core.preferSymlinkRefs) or "ref:" with a tab
// names its branch as well: fix leaves that branch to the user.
func TestACheckedOutRefCopyIsManualWhateverHEADLooksLike(t *testing.T) {
	for _, form := range []string{"symlink", "tab"} {
		g, r, _ := scenario(t, "clean")
		git(t, g, r, "", "checkout", "-q", "-b", "main.sync-conflict-20180827-123456-ABCDEF1")
		head := filepath.Join(r.CommonDir, "HEAD")
		os.Remove(head)
		if form == "symlink" {
			if err := os.Symlink("refs/heads/main.sync-conflict-20180827-123456-ABCDEF1", head); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(head, []byte("ref:\trefs/heads/main.sync-conflict-20180827-123456-ABCDEF1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rep := report(t, g, r.Worktree)
		as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range as {
			if a.Kind == Quarantine && strings.Contains(a.Path, "sync-conflict") {
				t.Fatalf("%s: the checked-out branch goes aside: %+v (manual %+v)", form, as, manual)
			}
		}
	}
}

// A copied directory of branches holding the checked-out one: setting the
// directory aside would leave HEAD unborn.
func TestACheckedOutRefInACopiedDirectoryIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	if err := os.WriteFile(filepath.Join(filepath.Dir(r.Worktree), ".sync_ab12cd34ef56.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, g, r, "", "branch", "feature/x")
	git(t, g, r, "", "checkout", "-q", "-b", "feature_conflict-20240101-120000/login")
	rep := report(t, g, r.Worktree)
	as, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.Kind == Quarantine && strings.Contains(a.Path, "feature_conflict") {
			t.Fatalf("the directory holding the checked-out branch goes aside: %+v (manual %+v, findings %+v)", as, manual, rep.Findings)
		}
	}
}

// An older loose copy of a branch that is packed: the branch is there (in
// packed-refs), so the copy is no "only copy" to put back.
func TestOlderLooseCopyOfAPackedBranch(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	old := git(t, g, r, "", "rev-parse", "HEAD")
	git(t, g, r, "", "commit", "-q", "--allow-empty", "-m", "newer")
	git(t, g, r, "", "pack-refs", "--all")
	write(t, r, "refs/heads/main 2", old+"\n")
	rep := report(t, g, r.Worktree)
	_, manual, err := actionsFor(context.Background(), g, r, rep, false, scan.DefaultFlagReader(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range manual {
		if strings.Contains(m.Do, "the only copy") {
			t.Fatalf("an older copy of a packed branch offered as the only copy: %+v", m)
		}
	}
}
