package fix

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Another machine's gc.pid stops automatic gc here: an old one goes into
// the backup, a fresh one (its gc may still run there) stays. gc.log goes
// too; undo brings both back.
func TestGCLeftoversAreMoved(t *testing.T) {
	g, r, _ := scenario(t, "clean")
	pid := filepath.Join(r.CommonDir, "gc.pid")
	os.WriteFile(pid, []byte("123 some-other-host\n"), 0o644)
	os.WriteFile(filepath.Join(r.CommonDir, "gc.log"), []byte("error: oops\n"), 0o644)
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "quarantine" || p.Actions[0].Path != filepath.Join(r.CommonDir, "gc.log") {
		t.Fatalf("fresh gc.pid: plan %+v %v", p.Actions, err)
	}
	old := time.Now().Add(-2 * time.Hour)
	os.Chtimes(pid, old, old)
	before := content(t, r.Worktree)
	p, err = MakePlan(context.Background(), g, o)
	if err != nil || kinds(p.Actions) != "quarantine,quarantine" {
		t.Fatalf("old gc.pid: plan %+v %v", p.Actions, err)
	}
	e, err := Apply(context.Background(), g, p, o, store)
	if err != nil {
		t.Fatal(err)
	}
	if exists(pid) {
		t.Fatal("gc.pid stayed")
	}
	if kept, err := Reverse(context.Background(), g, r, e, store); err != nil || len(kept) > 0 {
		t.Fatalf("undo: %v, kept %v", err, kept)
	}
	sameContent(t, before, content(t, r.Worktree))
}
