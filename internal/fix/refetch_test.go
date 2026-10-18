package fix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/repo"
)

// withOrigin gives r a remote "origin": a bare clone of it as it is now.
func withOrigin(t *testing.T, g *gitx.Runner, r repo.Repo) {
	t.Helper()
	if ok, _ := fixtures.GitAtLeast(2, 36); !ok {
		t.Skip("git fetch --refetch needs git 2.36")
	}
	remote := filepath.Join(t.TempDir(), "origin.git")
	git(t, g, r, "", "clone", "-q", "--bare", "--no-local", r.Worktree, remote)
	git(t, g, r, "", "remote", "add", "origin", remote)
}

// emptyObject empties the loose object file of oid, as a sync client that
// delivered its name but not its bytes leaves it.
func emptyObject(t *testing.T, r repo.Repo, oid string) string {
	t.Helper()
	obj := filepath.Join(r.CommonDir, "objects", oid[:2], oid[2:])
	os.Chmod(obj, 0o644)
	if err := os.WriteFile(obj, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return obj
}

func refs(t *testing.T, g *gitx.Runner, r repo.Repo) string {
	t.Helper()
	return git(t, g, r, "", "for-each-ref", "--format=%(refname) %(objectname)")
}

// An empty object file the remote still has: it goes into the backup, the
// refetch brings the object back, and no ref changes on the way.
func TestRefetchBringsBackAnEmptyObject(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withOrigin(t, g, r)
	git(t, g, r, "", "update-ref", "refs/remotes/origin/main", "HEAD") // a tracking ref the fetch must leave
	wantRefs := refs(t, g, r)
	head := git(t, g, r, "", "rev-parse", "HEAD")
	obj := emptyObject(t, r, head)
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "quarantine,refetch" || p.Actions[1].Value != "origin" {
		t.Fatalf("plan %+v %v", p, err)
	}
	e, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(obj); err == nil && info.Size() == 0 {
		t.Fatal("the empty object file is still there")
	}
	git(t, g, r, "", "cat-file", "-e", head)
	if got := refs(t, g, r); got != wantRefs {
		t.Fatalf("refs changed:\n%s\nwant\n%s", got, wantRefs)
	}
	if exists(filepath.Join(r.CommonDir, "refs", "gcd-fetch")) {
		t.Fatal("refs/gcd-fetch left behind")
	}
	// undo reports only the empty file as kept: the object fetched again is
	// in its place, and the fetched objects themselves are nothing to undo
	kept, err := Reverse(context.Background(), g, r, e, store)
	if err != nil || len(kept) != 1 || !strings.HasPrefix(kept[0], "quarantine ") || !strings.Contains(kept[0], "exists again") {
		t.Fatalf("undo: %v, kept %v", err, kept)
	}
}

// The remote does not have the object (a commit made here after it was
// cloned): the refetch fails and the empty file goes back where it was.
func TestRefetchThatCannotBringTheObjectIsRolledBack(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withOrigin(t, g, r)
	git(t, g, r, "", "commit", "-q", "--allow-empty", "-m", "only here")
	obj := emptyObject(t, r, git(t, g, r, "", "rev-parse", "HEAD"))
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "quarantine,refetch" {
		t.Fatalf("plan %+v %v", p, err)
	}
	_, err = Apply(context.Background(), g, p, o, store)
	if err == nil || !strings.HasSuffix(err.Error(), "rolled back") {
		t.Fatalf("apply: %v", err)
	}
	if info, err := os.Stat(obj); err != nil || info.Size() != 0 {
		t.Fatalf("the empty object file was not put back: %v", err)
	}
	if exists(filepath.Join(r.CommonDir, "refs", "gcd-fetch")) {
		t.Fatal("refs/gcd-fetch left behind")
	}
}

// HEAD's commit is gone (the branch arrived before its objects): it is
// fetched again.
func TestRefetchBringsBackHEADsCommit(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withOrigin(t, g, r)
	head := git(t, g, r, "", "rev-parse", "HEAD")
	if err := os.Remove(filepath.Join(r.CommonDir, "objects", head[:2], head[2:])); err != nil {
		t.Fatal(err)
	}
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	// nothing else is reported: the index cannot be compared with a commit that is not here
	if err != nil || kinds(p.Actions) != "refetch" || p.Actions[0].Code != check.CodeHEADCommitMissing || len(p.Manual) != 0 {
		t.Fatalf("plan %+v %v", p, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	git(t, g, r, "", "cat-file", "-e", head)
}

// Without a remote the missing commit stays manual.
func TestMissingHEADCommitWithoutARemoteIsManual(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	head := git(t, g, r, "", "rev-parse", "HEAD")
	os.Remove(filepath.Join(r.CommonDir, "objects", head[:2], head[2:]))
	o, _ := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil {
		t.Fatal(err)
	}
	want := "no remote"
	if ok, _ := fixtures.GitAtLeast(2, 36); !ok {
		want = "needs git 2.36"
	}
	if m, ok := manualFor(p.Manual, check.CodeHEADCommitMissing); len(p.Actions) != 0 || !ok || !strings.Contains(m.Do, want) {
		t.Fatalf("actions %v, manual %+v", p.Actions, p.Manual)
	}
}

// cloudFlags reports the paths in cloud as cloud-only; once armed, each
// only until it was reported, as a download leaves it.
type cloudFlags struct {
	mu     sync.Mutex
	cloud  map[string]bool
	armed  bool
	served map[string]bool
}

func (c *cloudFlags) IsDataless(p string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cloud[p] || c.served[p] {
		return false, nil
	}
	if c.armed {
		c.served[p] = true
	}
	return true, nil
}

// Files of .git only in the cloud: fix downloads them and nothing else,
// leaving the other damage for the next run. Damage the check could not
// see before the download (HEAD's commit missing) is no reason to roll
// back.
func TestCloudOnlyFilesAreDownloadedFirst(t *testing.T) {
	g, r, _ := scenario(t, "stale-rebase")
	head := git(t, g, r, "", "rev-parse", "HEAD")
	os.Remove(filepath.Join(r.CommonDir, "objects", head[:2], head[2:]))
	desc := filepath.Join(r.CommonDir, "description")
	if err := os.WriteFile(desc, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags := &cloudFlags{cloud: map[string]bool{desc: true}, served: map[string]bool{}}
	o, store := fixOptions(t, r.Worktree)
	o.Flags = flags
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || len(p.Refusals) > 0 || kinds(p.Actions) != "download" || p.Actions[0].Path != r.CommonDir {
		t.Fatalf("plan %+v %v", p, err)
	}
	if _, ok := manualFor(p.Manual, check.CodeStaleOperation); !ok {
		t.Fatalf("the other damage is not left for the next run: %+v", p.Manual)
	}
	flags.armed = true
	e, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	if !flags.served[desc] {
		t.Fatal("the cloud-only file was not downloaded")
	}
	if kept, err := Reverse(context.Background(), g, r, e, store); err != nil || len(kept) > 0 {
		t.Fatalf("undo: %v, kept %v", err, kept)
	}
}

// HEAD's commit and the index are both gone: the refetch brings the commit,
// then the index is made from it.
func TestRefetchThenRebuildTheIndex(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withOrigin(t, g, r)
	head := git(t, g, r, "", "rev-parse", "HEAD")
	os.Remove(filepath.Join(r.CommonDir, "objects", head[:2], head[2:]))
	os.Remove(filepath.Join(r.GitDir, "index"))
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "refetch,rebuild-index" {
		t.Fatalf("plan %+v %v", p, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	if git(t, g, r, "", "status", "--porcelain") != "" {
		t.Fatal("the rebuilt index differs from HEAD")
	}
}

// Several remotes, none the branch's or origin: fix leaves the missing
// commit to the user unless --remote names one; a name that is no remote
// is said so.
func TestRemoteIsChosenByName(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withOrigin(t, g, r)
	git(t, g, r, "", "remote", "rename", "origin", "a")
	git(t, g, r, "", "remote", "add", "b", git(t, g, r, "", "remote", "get-url", "a"))
	head := git(t, g, r, "", "rev-parse", "HEAD")
	os.Remove(filepath.Join(r.CommonDir, "objects", head[:2], head[2:]))
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if m, ok := manualFor(p.Manual, check.CodeHEADCommitMissing); err != nil || len(p.Actions) != 0 || !ok || !strings.Contains(m.Do, "several remotes") {
		t.Fatalf("plan %+v %v", p, err)
	}
	o.Remote = "nope"
	p, _ = MakePlan(context.Background(), g, o)
	if m, ok := manualFor(p.Manual, check.CodeHEADCommitMissing); !ok || !strings.Contains(m.Do, `no remote "nope"`) {
		t.Fatalf("manual %+v", p.Manual)
	}
	o.Remote = "b"
	p, err = MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "refetch" || p.Actions[0].Value != "b" {
		t.Fatalf("plan %+v %v", p, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
}
