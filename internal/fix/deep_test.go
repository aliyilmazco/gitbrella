package fix

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fix --deep: an object only git fsck misses (a blob of an older commit) is
// fetched again from the remote, and fsck after the fix finds nothing.
func TestDeepFixRefetchesWhatFsckFinds(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	os.WriteFile(filepath.Join(r.Worktree, "old.txt"), []byte("only in the first commit\n"), 0o644)
	git(t, g, r, "", "add", "old.txt")
	git(t, g, r, "", "commit", "-q", "-m", "old")
	blob := git(t, g, r, "", "rev-parse", "HEAD:old.txt")
	git(t, g, r, "", "rm", "-q", "old.txt")
	git(t, g, r, "", "commit", "-q", "-m", "gone")
	withOrigin(t, g, r)
	if err := os.Remove(filepath.Join(r.CommonDir, "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	o, store := fixOptions(t, r.Worktree)
	if p, err := MakePlan(context.Background(), g, o); err != nil || len(p.Actions)+len(p.Manual) != 0 {
		t.Fatalf("without --deep the loss is not seen: %+v %+v %v", p.Actions, p.Manual, err)
	}
	o.Deep = true
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "refetch" || len(p.Manual) != 0 {
		t.Fatalf("plan %+v %+v %v", p.Actions, p.Manual, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err != nil {
		t.Fatal(err)
	}
	git(t, g, r, "", "cat-file", "-e", blob)
}

// A damaged object fsck finds that no remote has: the fix is rolled back.
func TestDeepFixThatCannotRepairIsRolledBack(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	withOrigin(t, g, r)
	os.WriteFile(filepath.Join(r.Worktree, "old.txt"), []byte("only here\n"), 0o644)
	git(t, g, r, "", "add", "old.txt")
	git(t, g, r, "", "commit", "-q", "-m", "old")
	blob := git(t, g, r, "", "rev-parse", "HEAD:old.txt")
	git(t, g, r, "", "rm", "-q", "old.txt")
	git(t, g, r, "", "commit", "-q", "-m", "gone")
	os.Remove(filepath.Join(r.CommonDir, "objects", blob[:2], blob[2:]))
	o, store := fixOptions(t, r.Worktree)
	o.Deep = true
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "refetch" {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	if _, err := Apply(context.Background(), g, p, o, store); err == nil {
		t.Fatal("the fix claimed success with the object still missing")
	}
}
