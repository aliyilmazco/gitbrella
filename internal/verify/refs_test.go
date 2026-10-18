package verify

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
)

func TestCheckRefCopy(t *testing.T) {
	cases := map[string]RefRelation{
		"ref-copy-older": RelCopyOlder,
		"ref-copy-newer": RelCopyNewer,
	}
	for scenario, want := range cases {
		t.Run(scenario, func(t *testing.T) {
			g, r, dir := setup(t, scenario)
			st, err := CheckRefCopy(context.Background(), g, r, fixtures.Abs(dir, "refs/heads/main 2"), "refs/heads/main")
			if err != nil || st.Relation != want {
				t.Fatalf("got %+v %v, want %s", st, err, want)
			}
		})
	}
}

func TestCheckRefCopyOddContent(t *testing.T) {
	g, r, dir := setup(t, "clean")
	fr := &fixtures.Repo{Dir: dir, GitDir: filepath.Join(dir, ".git")}
	cases := map[string]RefRelation{
		"ref: refs/heads/main\n":                     RelSymbolic,
		"not an oid\n":                               RelInvalid,
		"0123456789012345678901234567890123456789\n": RelMissingObject,
	}
	for content, want := range cases {
		if err := fr.WriteFile("refs/heads/main 2", content); err != nil {
			t.Fatal(err)
		}
		st, err := CheckRefCopy(context.Background(), g, r, fixtures.Abs(dir, "refs/heads/main 2"), "refs/heads/main")
		if err != nil || st.Relation != want {
			t.Errorf("content %q: got %+v %v, want %s", content, st, err, want)
		}
	}
	first, err := fr.Git("rev-parse", "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if err := fr.WriteFile("refs/heads/gone 2", first+"\n"); err != nil {
		t.Fatal(err)
	}
	st, err := CheckRefCopy(context.Background(), g, r, fixtures.Abs(dir, "refs/heads/gone 2"), "refs/heads/gone")
	if err != nil || st.Relation != RelNoOriginal {
		t.Fatalf("got %+v %v, want no-original", st, err)
	}
}

func TestOperationStateAndHasCommits(t *testing.T) {
	g, r, _ := setup(t, "merge-in-progress")
	if ops, stale := OperationState(r.GitDir); len(ops) != 1 || ops[0] != "merge" || len(stale) != 0 {
		t.Fatalf("ops = %v, stale = %v", ops, stale)
	}
	if ok, err := HasCommits(context.Background(), g, r); err != nil || !ok {
		t.Fatalf("HasCommits = %v %v", ok, err)
	}
}

func TestStaleRebaseDirectory(t *testing.T) {
	_, r, _ := setup(t, "stale-rebase")
	ops, stale := OperationState(r.GitDir)
	if len(ops) != 0 || len(stale) != 1 || stale[0] != "rebase-merge" {
		t.Fatalf("ops = %v, stale = %v", ops, stale)
	}
}
