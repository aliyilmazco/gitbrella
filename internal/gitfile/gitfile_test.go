package gitfile

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestContent(t *testing.T) {
	for _, tc := range []struct{ worktree, gitDir, home, want string }{
		{"/Users/a/Library/Mobile Documents/x/notes", "/Users/a/.local/share/gitbrella/gitdirs/notes-1a2b3c4d", "/Users/a",
			"gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d\n"},
		{"/Users/a/Dropbox/notes", "/Volumes/ext/notes.git", "/Users/a", "gitdir: /Volumes/ext/notes.git\n"},
		{"/srv/notes", "/Users/a/.local/share/g/notes", "/Users/a", "gitdir: /Users/a/.local/share/g/notes\n"},
	} {
		if got := Content(tc.worktree, tc.gitDir, tc.home); got != tc.want {
			t.Errorf("Content(%s, %s) = %q, want %q", tc.worktree, tc.gitDir, got, tc.want)
		}
	}
}

func TestWriteReadAndGitAgrees(t *testing.T) {
	home := t.TempDir()
	home, _ = filepath.EvalSymlinks(home)
	wt := filepath.Join(home, "Library", "Mobile Documents", "my notes")
	gd := filepath.Join(home, ".local", "share", "gitdirs", "my notes-1")
	os.MkdirAll(wt, 0o755)
	os.MkdirAll(filepath.Dir(gd), 0o755)
	if out, err := exec.Command("git", "init", "-q", wt).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if err := os.Rename(filepath.Join(wt, ".git"), gd); err != nil {
		t.Fatal(err)
	}
	if err := Write(wt, Content(wt, gd, home)); err != nil {
		t.Fatal(err)
	}
	target, isFile, err := Read(wt)
	if err != nil || !isFile || target != gd {
		t.Fatalf("Read = %q %v %v, want %q", target, isFile, err, gd)
	}
	out, err := exec.Command("git", "-C", filepath.Join(wt), "rev-parse", "--absolute-git-dir").Output()
	if err != nil || strings.TrimSpace(string(out)) != gd {
		t.Fatalf("git found %q (%v), want %q", out, err, gd)
	}
}

func TestWriteNeverReplaces(t *testing.T) {
	wt := t.TempDir()
	os.Mkdir(filepath.Join(wt, ".git"), 0o755)
	if err := Write(wt, "gitdir: /x\n"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("over a directory: %v", err)
	}
	wt2 := t.TempDir()
	os.WriteFile(filepath.Join(wt2, ".git"), []byte("gitdir: /old\n"), 0o644)
	if err := Write(wt2, "gitdir: /new\n"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("over a file: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(wt2, ".git")); string(b) != "gitdir: /old\n" {
		t.Fatalf("file replaced: %q", b)
	}
}

func TestRead(t *testing.T) {
	wt := t.TempDir()
	if _, _, err := Read(wt); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("no .git: %v", err)
	}
	os.Mkdir(filepath.Join(wt, ".git"), 0o755)
	if _, isFile, err := Read(wt); err != nil || isFile {
		t.Fatalf("directory: %v %v", isFile, err)
	}
	wt2 := t.TempDir()
	os.WriteFile(filepath.Join(wt2, ".git"), []byte("not a gitfile\n"), 0o644)
	if _, _, err := Read(wt2); err == nil {
		t.Fatal("garbage accepted")
	}
	wt3 := t.TempDir()
	os.WriteFile(filepath.Join(wt3, ".git"), []byte("gitdir: ../x\r\n"), 0o644)
	if target, isFile, err := Read(wt3); err != nil || !isFile || target != filepath.Join(filepath.Dir(wt3), "x") {
		t.Fatalf("relative with CRLF: %q %v %v", target, isFile, err)
	}
}
