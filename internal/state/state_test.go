package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestResolve(t *testing.T) {
	for _, tc := range []struct {
		name, home, goos string
		env              map[string]string
		state, gitdirs   string
	}{
		{"darwin", "/Users/a", "darwin", nil,
			"/Users/a/Library/Application Support/gitbrella", "/Users/a/.local/share/gitbrella/gitdirs"},
		{"linux default", "/home/a", "linux", nil,
			"/home/a/.local/state/gitbrella", "/home/a/.local/share/gitbrella/gitdirs"},
		{"linux XDG", "/home/a", "linux", map[string]string{"XDG_STATE_HOME": "/srv/state"},
			"/srv/state/gitbrella", "/home/a/.local/share/gitbrella/gitdirs"},
		{"linux relative XDG ignored", "/home/a", "linux", map[string]string{"XDG_STATE_HOME": "state"},
			"/home/a/.local/state/gitbrella", "/home/a/.local/share/gitbrella/gitdirs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := Resolve(tc.home, tc.goos, env(tc.env))
			if err != nil {
				t.Fatal(err)
			}
			if d.State != tc.state || d.GitDirs != tc.gitdirs {
				t.Fatalf("got %+v, want state %s gitdirs %s", d, tc.state, tc.gitdirs)
			}
		})
	}
}

func TestResolveRefusesSyncedState(t *testing.T) {
	_, err := Resolve("/home/a", "linux", env(map[string]string{"XDG_STATE_HOME": "/home/a/Dropbox/state"}))
	if !errors.Is(err, ErrSynced) {
		t.Fatalf("err = %v, want ErrSynced", err)
	}
	if _, err := Resolve("relative", "linux", env(nil)); err == nil {
		t.Fatal("a relative home was accepted")
	}
}

func TestSubdirectories(t *testing.T) {
	d := Dirs{State: "/s"}
	if d.Journal() != "/s/journal" || d.Backups() != "/s/backups" || d.Locks() != "/s/locks" {
		t.Fatalf("got %s %s %s", d.Journal(), d.Backups(), d.Locks())
	}
}

func TestRepoID(t *testing.T) {
	a := RepoID("/Users/a/Library/Mobile Documents/com~apple~CloudDocs/My Notes", "/Users/a")
	b := RepoID("/Users/b/Library/Mobile Documents/com~apple~CloudDocs/My Notes", "/Users/b")
	if a != b {
		t.Fatalf("the same folder under two homes got %s and %s", a, b)
	}
	if !strings.HasPrefix(a, "My-Notes-") || len(a) != len("My-Notes-")+8 {
		t.Fatalf("id %q: want My-Notes-<8 hex>", a)
	}
	if c := RepoID("/Users/a/Library/Mobile Documents/com~apple~CloudDocs/Other/My Notes", "/Users/a"); c == a {
		t.Fatalf("two folders share id %s", c)
	}
	if d := RepoID("/srv/repos/ğüş", "/Users/a"); !strings.HasPrefix(d, "repo-") {
		t.Fatalf("a name with no safe characters: %q", d)
	}
}

func TestLockExcludesSecondHolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	release, err := Lock(dir, "notes-12345678")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(dir, "notes-12345678"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second lock: err = %v, want ErrBusy", err)
	}
	if _, err := Lock(dir, "other-12345678"); err != nil {
		t.Fatalf("another repository's lock: %v", err)
	}
	release()
	again, err := Lock(dir, "notes-12345678")
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again()
}

func TestFileIDFollowsTheFileNotItsName(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "Çalışma")
	if err := os.Mkdir(a, 0o755); err != nil {
		t.Fatal(err)
	}
	id := FileID(a)
	if id == "" || FileID(filepath.Join(dir, "missing")) != "" {
		t.Fatalf("id %q", id)
	}
	b := filepath.Join(dir, "renamed")
	os.Rename(a, b)
	if FileID(b) != id {
		t.Fatal("a rename changed the id")
	}
}

func TestOnDiskAndRepoIDIgnoreTheSpelling(t *testing.T) {
	home, _ := filepath.EvalSymlinks(t.TempDir())
	stored := "\u00c7al\u0131s\u0327ma Notes" // "Çalışma" with an NFD ş
	if err := os.MkdirAll(filepath.Join(home, "Library", stored), 0o755); err != nil {
		t.Fatal(err)
	}
	typed := filepath.Join(home, "LIBRARY", "\u00c7al\u0131\u015fma notes") // other case, NFC ş
	if _, err := os.Stat(typed); err != nil {
		t.Skip("case- and normalization-sensitive filesystem")
	}
	if got := OnDisk(typed); got != filepath.Join(home, "Library", stored) {
		t.Fatalf("OnDisk(%q) = %q", typed, got)
	}
	if RepoID(typed, home) != RepoID(filepath.Join(home, "Library", stored), home) {
		t.Fatal("two spellings, two ids")
	}
	if got := OnDisk(filepath.Join(home, "Library", "missing", "x")); got != filepath.Join(home, "Library", "missing", "x") {
		t.Fatalf("a missing part: %q", got)
	}
}
