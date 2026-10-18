package fix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/check"
)

// An index that lists no files (git rebuilt it after the sync client
// deleted it) holds nothing staged: it goes into the backup and the index
// is made from HEAD; undo gives the empty one back.
func TestEmptyIndexIsRebuilt(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	git(t, g, r, "", "read-tree", "--empty")
	before := content(t, r.Worktree)
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "quarantine,rebuild-index" {
		t.Fatalf("plan %+v %v", p, err)
	}
	e, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	if s := git(t, g, r, "", "status", "--porcelain"); s != "" {
		t.Fatalf("status after the fix:\n%s", s)
	}
	if kept, err := Reverse(context.Background(), g, r, e, store); err != nil || len(kept) > 0 {
		t.Fatalf("undo: %v, kept %v", err, kept)
	}
	sameContent(t, before, content(t, r.Worktree))
}

// A shrunk index stays manual (untracking on purpose looks the same); when
// nothing else is staged the advice says read-tree loses nothing, and when
// something is, it does not.
func TestShrunkIndexAdvice(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	os.WriteFile(filepath.Join(r.Worktree, "second.md"), []byte("two\n"), 0o644)
	git(t, g, r, "", "add", "second.md")
	git(t, g, r, "", "commit", "-q", "-m", "second")
	files := strings.Fields(git(t, g, r, "", "ls-files"))
	git(t, g, r, "", "rm", "-q", "--cached", files[0])
	o, _ := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	m, ok := manualFor(p.Manual, check.CodeIndexShrunk)
	if err != nil || len(p.Actions) != 0 || !ok || !strings.Contains(m.Do, "loses nothing") {
		t.Fatalf("plan %+v %v", p, err)
	}
	os.WriteFile(filepath.Join(r.Worktree, "new.md"), []byte("staged\n"), 0o644)
	git(t, g, r, "", "add", "new.md")
	p, _ = MakePlan(context.Background(), g, o)
	if m, ok := manualFor(p.Manual, check.CodeIndexShrunk); !ok || strings.Contains(m.Do, "loses nothing") {
		t.Fatalf("manual %+v", p.Manual)
	}
}
