package check

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
)

// A commit about to add a sync conflict copy of a tracked file is stopped;
// names alone decide, and only for files the commit adds while the original
// is still staged.
func TestStagedConflictCopies(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	for _, tc := range []struct {
		name   string
		setup  func(r *fixtures.Repo) error
		damage string // the sample, "" for none
	}{
		{"copy beside its original", func(r *fixtures.Repo) error {
			return stage(r, "note 2.md", "two\n")
		}, "note 2.md"},
		{"copy of a directory", func(r *fixtures.Repo) error {
			if err := commitFile(r, "docs/a.md", "a\n"); err != nil {
				return err
			}
			return stage(r, "docs 2/a.md", "a\n")
		}, "docs 2/a.md"},
		{"syncthing copy", func(r *fixtures.Repo) error {
			return stage(r, "note.sync-conflict-20261014-120000-ABCDEF1.md", "x\n")
		}, "note.sync-conflict-20261014-120000-ABCDEF1.md"},
		{"no original", func(r *fixtures.Repo) error {
			return stage(r, "plan 2.md", "x\n")
		}, ""},
		{"a rename to such a name", func(r *fixtures.Repo) error {
			_, err := r.Git("mv", "note.md", "note 2.md")
			return err
		}, ""},
		{"a copy committed before", func(r *fixtures.Repo) error {
			return commitFile(r, "note 2.md", "two\n")
		}, ""},
		{"a copy not staged", func(r *fixtures.Repo) error {
			return os.WriteFile(filepath.Join(r.Dir, "note 2.md"), []byte("x\n"), 0o644)
		}, ""},
		{"intent to add (git add -N)", func(r *fixtures.Repo) error {
			os.WriteFile(filepath.Join(r.Dir, "note 2.md"), []byte("x\n"), 0o644)
			_, err := r.Git("add", "-N", "note 2.md")
			return err
		}, ""},
		{"a name with a client's letters on purpose (Foo.Conflict.cs)", func(r *fixtures.Repo) error {
			if err := commitFile(r, "Foo.cs", "a\n"); err != nil {
				return err
			}
			return stage(r, "Foo.Conflict.cs", "b\n")
		}, ""},
		{"an app's numbering (a \" 1\" twin)", func(r *fixtures.Repo) error {
			if err := commitFile(r, "Untitled.md", "a\n"); err != nil {
				return err
			}
			if err := commitFile(r, "Untitled 1.md", "b\n"); err != nil {
				return err
			}
			return stage(r, "Untitled 2.md", "c\n")
		}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"))
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.setup(r); err != nil {
				t.Fatal(err)
			}
			rep, err := Run(context.Background(), g, Options{Path: r.Dir, Home: t.TempDir(), GOOS: "linux", Now: time.Now(),
				Flags: noDataless{}, Procs: noProcs, Version: "test"})
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			for _, f := range rep.Findings {
				if f.Code == CodeStagedCopy {
					if f.Severity != Damage {
						t.Fatalf("severity %s", f.Severity)
					}
					got = f.Details["sample"]
				}
			}
			if got != tc.damage {
				t.Fatalf("staged copy %q, want %q (findings %v)", got, tc.damage, codes(rep))
			}
		})
	}
}

func stage(r *fixtures.Repo, rel, content string) error {
	p := filepath.Join(r.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return err
	}
	_, err := r.Git("add", "--", rel)
	return err
}

func commitFile(r *fixtures.Repo, rel, content string) error {
	if err := stage(r, rel, content); err != nil {
		return err
	}
	_, err := r.Git("commit", "-q", "-m", "add "+rel)
	return err
}
