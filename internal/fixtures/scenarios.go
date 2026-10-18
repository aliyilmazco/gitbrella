package fixtures

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Scenarios maps a scenario name to the damage it injects into a fresh repo.
var Scenarios = map[string]func(r *Repo) error{
	"clean": func(r *Repo) error { return nil },
	"index-copies": func(r *Repo) error {
		if err := r.CopyFile("index", "index 2"); err != nil {
			return err
		}
		return r.CopyFile("index", "index 3")
	},
	"orphan-index-lock": func(r *Repo) error {
		if err := r.RenameFile("index", "index.lock"); err != nil {
			return err
		}
		return r.Age("index.lock", 10*time.Minute) // left behind, not being written
	},
	"ref-copy-older": func(r *Repo) error {
		return r.WriteFile("refs/heads/main 2", r.Commits[0]+"\n")
	},
	"ref-copy-newer": func(r *Repo) error {
		oid, err := r.Git("commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "only in the copy")
		if err != nil {
			return err
		}
		return r.WriteFile("refs/heads/main 2", oid+"\n")
	},
	"dropbox-copy": func(r *Repo) error {
		return r.CopyFile("HEAD", "HEAD (Fixture's conflicted copy 2026-10-08)")
	},
	"stale-lock": func(r *Repo) error {
		if err := r.WriteFile("refs/heads/main.lock", ""); err != nil {
			return err
		}
		return r.Age("refs/heads/main.lock", 10*time.Minute)
	},
	"merge-in-progress": func(r *Repo) error { return r.WriteFile("MERGE_HEAD", r.Commits[1]+"\n") },
	"corrupt-index":     func(r *Repo) error { return r.FlipByte("index", 20) },
	"third-party": func(r *Repo) error {
		if err := r.WriteFile("notesync.lock", "pid 1\n"); err != nil {
			return err
		}
		if err := r.Age("notesync.lock", time.Hour); err != nil {
			return err
		}
		if err := r.WriteFile(".DS_Store", "finder"); err != nil {
			return err
		}
		return r.WriteFile("notesync-transactions/t1.json", "{}\n")
	},
	"dir-copy": func(r *Repo) error { return r.CopyDir("refs", "refs 2") },
	"syncthing-copy": func(r *Repo) error {
		return r.CopyFile("index", "index.sync-conflict-20180827-123456-ABCDEF1")
	},
	"desktop-ini": func(r *Repo) error {
		if err := r.WriteFile("refs/heads/desktop.ini", "[.ShellClassInfo]\n"); err != nil {
			return err
		}
		return r.WriteFile("objects/desktop.ini", "[.ShellClassInfo]\n")
	},
	"foreign-object": func(r *Repo) error {
		oid, err := r.Git("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		return r.WriteFile("objects/"+oid[:2]+"/"+oid[2:]+"-safeBackup-0001", "x")
	},
	"empty-object": func(r *Repo) error {
		oid, err := r.Git("rev-parse", "HEAD:note.md")
		if err != nil {
			return err
		}
		rel := "objects/" + oid[:2] + "/" + oid[2:]
		if err := os.Chmod(r.path(rel), 0o644); err != nil {
			return err
		}
		return os.WriteFile(r.path(rel), nil, 0o644)
	},
	"empty-ref": func(r *Repo) error { return os.WriteFile(r.path("refs/heads/main"), nil, 0o644) },
	"stale-rebase": func(r *Repo) error {
		return os.MkdirAll(r.path("rebase-merge"), 0o755)
	},
	"gitdir-copy": func(r *Repo) error {
		return r.CopyDir(".", "../.git 2")
	},
	"worktree-copies": func(r *Repo) error {
		if err := copyFile(filepath.Join(r.Dir, "note.md"), filepath.Join(r.Dir, "note 2.md")); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(r.Dir, "Chapter 2.md"), []byte("no original, not a copy\n"), 0o644)
	},
}

// Names returns the scenario names in sorted order.
func Names() []string {
	names := make([]string, 0, len(Scenarios)+2)
	for n := range Scenarios {
		names = append(names, n)
	}
	names = append(names, "linked-worktree", "reftable", "broken-worktree-admin", "no-checkout",
		"gitdir-outside", "nosync-unignored", "nosync-ignored", "partial-clone")
	sort.Strings(names)
	return names
}

// Build creates scenario name at dir. For linked-worktree, dir receives the
// linked worktree and the main repository is created next to it.
func Build(name, dir string) error {
	switch name {
	case "reftable":
		ok, err := GitAtLeast(2, 45)
		if err != nil {
			return err
		}
		if !ok {
			return ErrUnsupported
		}
		_, err = New(dir, "--ref-format=reftable")
		return err
	case "broken-worktree-admin":
		main, err := New(dir)
		if err != nil {
			return err
		}
		if _, err := main.Git("worktree", "add", "-q", "-b", "feature", dir+"-wt"); err != nil {
			return err
		}
		admins, err := os.ReadDir(filepath.Join(main.GitDir, "worktrees"))
		if err != nil {
			return err
		}
		if len(admins) != 1 {
			return fmt.Errorf("want one worktree admin directory, got %d", len(admins))
		}
		return os.Remove(filepath.Join(main.GitDir, "worktrees", admins[0].Name(), "gitdir"))
	case "no-checkout":
		src, err := New(dir + "-src")
		if err != nil {
			return err
		}
		_, err = gitIn(filepath.Dir(dir), "clone", "-q", "--no-checkout", src.Dir, dir)
		return err
	case "partial-clone":
		// A blob:none clone whose ref copy points at a commit only the remote
		// has. Reading it would make git fetch from the promisor remote.
		origin, err := New(dir + "-origin")
		if err != nil {
			return err
		}
		if _, err := origin.Git("config", "uploadpack.allowFilter", "true"); err != nil {
			return err
		}
		if _, err := origin.Git("config", "uploadpack.allowAnySHA1InWant", "true"); err != nil {
			return err
		}
		if _, err := gitIn(filepath.Dir(dir), "clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(origin.Dir), dir); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(origin.Dir, "note.md"), []byte("three\n"), 0o644); err != nil {
			return err
		}
		if _, err := origin.Git("commit", "-q", "-am", "only on the remote"); err != nil {
			return err
		}
		oid, err := origin.Git("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, ".git", "refs", "heads", "main 2"), []byte(oid+"\n"), 0o644)
	case "gitdir-outside":
		// .git moved out of the synced folder; a gitfile points to it.
		_, err := New(dir, "--separate-git-dir="+dir+"-gitdir")
		return err
	case "nosync-unignored", "nosync-ignored":
		// The forum trick: the git directory renamed to .git.nosync inside
		// the working tree, which iCloud skips but git sees as untracked.
		gitDir := filepath.Join(dir, ".git.nosync")
		if _, err := New(dir, "--separate-git-dir="+gitDir); err != nil {
			return err
		}
		if name == "nosync-unignored" {
			return nil
		}
		if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(gitDir, "info", "exclude"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := f.WriteString("/.git.nosync/\n"); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	case "linked-worktree":
		main, err := New(dir + "-main")
		if err != nil {
			return err
		}
		if _, err := main.Git("worktree", "add", "-q", "-b", "feature", dir); err != nil {
			return err
		}
		return main.CopyFile("index", "index 2")
	}
	inject, ok := Scenarios[name]
	if !ok {
		return fmt.Errorf("unknown scenario %q (known: %s)", name, strings.Join(Names(), ", "))
	}
	r, err := New(dir)
	if err != nil {
		return err
	}
	return inject(r)
}

// GitAtLeast reports whether the installed git is at least major.minor.
func GitAtLeast(major, minor int) (bool, error) {
	out, err := exec.Command("git", "version").Output()
	if err != nil {
		return false, err
	}
	f := strings.Fields(string(out))
	if len(f) < 3 {
		return false, fmt.Errorf("unexpected git version output %q", out)
	}
	var maj, min int
	if _, err := fmt.Sscanf(f[2], "%d.%d", &maj, &min); err != nil {
		return false, err
	}
	return maj > major || (maj == major && min >= minor), nil
}

// Abs is a helper for tests that need the git dir of a built scenario.
func Abs(dir, rel string) string { return filepath.Join(dir, ".git", filepath.FromSlash(rel)) }
