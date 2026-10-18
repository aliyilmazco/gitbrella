package fix

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Conflict copies in the working tree: only an untracked one with its
// original's bytes goes into the backup; one that differs, a tracked one
// and one inside a nested repository stay. Undo puts it back.
func TestIdenticalWorktreeCopiesAreMoved(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	put := func(rel, s string) {
		p := filepath.Join(r.Worktree, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("a.txt", "same\n")
	put("b.txt", "one\n")
	put("t.txt", "t\n")
	put("t 2.txt", "t\n")
	git(t, g, r, "", "add", "a.txt", "b.txt", "t.txt", "t 2.txt")
	git(t, g, r, "", "commit", "-q", "-m", "files")
	put("a 2.txt", "same\n")  // moved
	put("b 2.txt", "other\n") // differs: stays
	put("sub/.git", "gitdir: elsewhere\n")
	put("sub/c.txt", "c\n")
	put("sub/c 2.txt", "c\n") // nested repository: stays
	before := content(t, r.Worktree)
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "quarantine" || p.Actions[0].Path != filepath.Join(r.Worktree, "a 2.txt") {
		t.Fatalf("plan %+v %v", p.Actions, err)
	}
	e, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(r.Worktree, "a 2.txt")) || !exists(filepath.Join(r.Worktree, "b 2.txt")) || !exists(filepath.Join(r.Worktree, "t 2.txt")) {
		t.Fatal("wrong copies moved")
	}
	if kept, err := Reverse(context.Background(), g, r, e, store); err != nil || len(kept) > 0 {
		t.Fatalf("undo: %v, kept %v", err, kept)
	}
	sameContent(t, before, content(t, r.Worktree))
}
