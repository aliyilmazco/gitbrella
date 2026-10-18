package treecopy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// tree builds a small git-directory-like tree with the kinds of entries a
// copy must keep: nested files, an executable hook, a symlink, old times.
func tree(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "src with space")
	for rel, body := range map[string]string{
		"HEAD":                       "ref: refs/heads/main\n",
		"objects/ab/cdef":            "object",
		"refs/heads/main":            "0123\n",
		"hooks/pre-commit":           "#!/bin/sh\n",
		"info/exclude":               "",
		"logs/refs/heads/main 🙂.log": "log",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.Chmod(filepath.Join(dir, "hooks", "pre-commit"), 0o755)
	os.Symlink("../HEAD", filepath.Join(dir, "refs", "link"))
	old := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(filepath.Join(dir, "objects", "ab", "cdef"), old, old)
	return dir
}

func TestCloneKeepsEverything(t *testing.T) {
	src := tree(t)
	dst := filepath.Join(t.TempDir(), "dst")
	if err := Clone(context.Background(), src, dst, runtime.GOOS); err != nil {
		t.Fatal(err)
	}
	diffs, err := Compare(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Fatalf("copy differs: %v", diffs)
	}
}

func TestCloneRefusesExistingDestination(t *testing.T) {
	src := tree(t)
	dst := t.TempDir()
	if err := Clone(context.Background(), src, dst, runtime.GOOS); err == nil {
		t.Fatal("copied over an existing directory")
	}
}

func TestCompareFindsEachKindOfDifference(t *testing.T) {
	for name, change := range map[string]func(dir string){
		"content": func(d string) { os.WriteFile(filepath.Join(d, "HEAD"), []byte("ref: refs/heads/mian\n"), 0o644) },
		"mode":    func(d string) { os.Chmod(filepath.Join(d, "hooks", "pre-commit"), 0o644) },
		"missing": func(d string) { os.Remove(filepath.Join(d, "info", "exclude")) },
		"extra":   func(d string) { os.WriteFile(filepath.Join(d, "index 2"), nil, 0o644) },
		"symlink": func(d string) {
			os.Remove(filepath.Join(d, "refs", "link"))
			os.Symlink("../config", filepath.Join(d, "refs", "link"))
		},
		"time": func(d string) { now := time.Now(); os.Chtimes(filepath.Join(d, "objects", "ab", "cdef"), now, now) },
		"kind": func(d string) {
			os.Remove(filepath.Join(d, "info", "exclude"))
			os.Mkdir(filepath.Join(d, "info", "exclude"), 0o755)
		},
	} {
		t.Run(name, func(t *testing.T) {
			src := tree(t)
			dst := filepath.Join(t.TempDir(), "dst")
			if err := Clone(context.Background(), src, dst, runtime.GOOS); err != nil {
				t.Fatal(err)
			}
			change(dst)
			diffs, err := Compare(src, dst)
			if err != nil {
				t.Fatal(err)
			}
			if len(diffs) == 0 {
				t.Fatalf("%s change not found", name)
			}
		})
	}
}

func TestSize(t *testing.T) {
	n, err := Size(tree(t))
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(len("ref: refs/heads/main\n") + len("object") + len("0123\n") + len("#!/bin/sh\n") + len("log")); n != want {
		t.Fatalf("size %d, want %d", n, want)
	}
}

func TestCompareReportsUnreadable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads everything")
	}
	src := tree(t)
	dst := filepath.Join(t.TempDir(), "dst")
	Clone(context.Background(), src, dst, runtime.GOOS)
	for _, d := range []string{src, dst} { // same mode on both: only the read can fail
		os.Chmod(filepath.Join(d, "HEAD"), 0o000)
		t.Cleanup(func() { os.Chmod(filepath.Join(d, "HEAD"), 0o644) })
	}
	if _, err := Compare(src, dst); err == nil || !strings.Contains(err.Error(), "HEAD") {
		t.Fatalf("err = %v, want an error naming HEAD", err)
	}
}

func TestCompareWantsDirectories(t *testing.T) {
	d := t.TempDir()
	a, b := filepath.Join(d, "a"), filepath.Join(d, "b")
	os.WriteFile(a, []byte("one\n"), 0o644)
	os.WriteFile(b, []byte("two\n"), 0o644)
	if _, err := Compare(a, b); err == nil {
		t.Fatal("two files compared as trees")
	}
	if _, err := Compare(d, a); err == nil {
		t.Fatal("a directory compared with a file")
	}
}

// A failed copy reports what cp said and leaves no partial copy behind.
func TestCloneFailureLeavesNothing(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst")
	err := Clone(context.Background(), filepath.Join(t.TempDir(), "missing"), dst, runtime.GOOS)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err = %v, want an error naming the source", err)
	}
	if _, err := os.Lstat(dst); err == nil {
		t.Fatal("a partial copy was left behind")
	}
}

// Every kind of difference counts toward the cap of 20, so a copy that went
// badly wrong still yields a short list.
func TestCompareListsAtMostTwentyDifferences(t *testing.T) {
	for name, change := range map[string]func(src, dst, name string){
		"missing": func(_, d, n string) { os.Remove(filepath.Join(d, n)) },
		"mode":    func(_, d, n string) { os.Chmod(filepath.Join(d, n), 0o600) },
		"size":    func(_, d, n string) { os.WriteFile(filepath.Join(d, n), []byte("longer body"), 0o644) },
		"content": func(s, d, n string) {
			os.WriteFile(filepath.Join(d, n), []byte("y"), 0o644)
			info, _ := os.Stat(filepath.Join(s, n))
			os.Chtimes(filepath.Join(d, n), info.ModTime(), info.ModTime())
		},
		"symlink": func(_, d, n string) {
			os.Remove(filepath.Join(d, n+"-link"))
			os.Symlink("elsewhere", filepath.Join(d, n+"-link"))
		},
		"extra": func(_, d, n string) { os.WriteFile(filepath.Join(d, n+" 2"), nil, 0o644) },
	} {
		t.Run(name, func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "src")
			os.Mkdir(src, 0o755)
			var names []string
			for i := range 30 {
				n := fmt.Sprintf("f%02d", i)
				names = append(names, n)
				os.WriteFile(filepath.Join(src, n), []byte("x"), 0o644)
				os.Symlink(n, filepath.Join(src, n+"-link"))
			}
			dst := filepath.Join(t.TempDir(), "dst")
			if err := Clone(context.Background(), src, dst, runtime.GOOS); err != nil {
				t.Fatal(err)
			}
			for _, n := range names {
				change(src, dst, n)
			}
			diffs, err := Compare(src, dst)
			if err != nil {
				t.Fatal(err)
			}
			if len(diffs) != 20 {
				t.Fatalf("%d differences listed, want 20: %v", len(diffs), diffs)
			}
		})
	}
}
