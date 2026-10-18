// Package state says where gitbrella keeps what outlives one run: the
// journal of its changes, backups, locks, and the git directories it moved.
// None of it may live in a synced folder.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/cloud"
)

// Dirs are the tool's own directories.
type Dirs struct {
	State   string // journal, backups, locks: this device only
	GitDirs string // git directories moved out of synced folders
}

// ErrSynced: a directory the tool would write to is inside a synced folder.
var ErrSynced = errors.New("inside a synced folder")

// Resolve returns the directories for home on goos. On Linux the state
// directory follows an absolute XDG_STATE_HOME. Moved git directories always
// live at ~/.local/share/gitbrella/gitdirs: the same place relative to
// home on every system, so the .git file pointing there reads the same on
// every device that syncs the folder.
func Resolve(home, goos string, getenv func(string) string) (Dirs, error) {
	if !filepath.IsAbs(home) {
		return Dirs{}, fmt.Errorf("home directory %q is not absolute", home)
	}
	d := Dirs{GitDirs: filepath.Join(home, ".local", "share", "gitbrella", "gitdirs")}
	switch xdg := getenv("XDG_STATE_HOME"); {
	case goos == "darwin":
		d.State = filepath.Join(home, "Library", "Application Support", "gitbrella")
	case filepath.IsAbs(xdg):
		d.State = filepath.Join(xdg, "gitbrella")
	default:
		d.State = filepath.Join(home, ".local", "state", "gitbrella")
	}
	for _, p := range []string{d.State, d.GitDirs} {
		if err := NotSynced(p, home, goos); err != nil {
			return Dirs{}, err
		}
	}
	return d, nil
}

// Journal, Backups and Locks are the parts of the state directory.
func (d Dirs) Journal() string { return filepath.Join(d.State, "journal") }
func (d Dirs) Backups() string { return filepath.Join(d.State, "backups") }
func (d Dirs) Locks() string   { return filepath.Join(d.State, "locks") }

// NotSynced fails when path lies in a folder a sync client syncs.
func NotSynced(path, home, goos string) error {
	if p := cloud.DetectAt(path, home, goos); p != cloud.None {
		return fmt.Errorf("%w: %s is synced by %s", ErrSynced, path, p)
	}
	return nil
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// OnDisk spells p as the filesystem stores it. On a case-insensitive or
// Unicode-normalizing disk "NOTES" and "notes", or the NFC and NFD forms of
// "Çalışma", name one folder, and an id must not depend on which was typed:
// each component's stored name is read back from its parent directory by
// device and inode. A part that does not exist is kept as given.
func OnDisk(p string) string {
	p = filepath.Clean(p)
	if !filepath.IsAbs(p) {
		return p
	}
	out := string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(p, out), string(filepath.Separator))
	for i, name := range parts {
		id := FileID(filepath.Join(out, name))
		if id == "" {
			return filepath.Join(append([]string{out}, parts[i:]...)...)
		}
		if entries, err := os.ReadDir(out); err == nil {
			stored := ""
			for _, e := range entries {
				if e.Name() == name {
					stored = name
					break
				}
				if stored == "" && FileID(filepath.Join(out, e.Name())) == id {
					stored = e.Name()
				}
			}
			if stored != "" {
				name = stored
			}
		}
		out = filepath.Join(out, name)
	}
	return out
}

// RepoID names a working tree: its folder name and a hash of its path
// relative to home, both as stored on disk (OnDisk), so the same folder
// synced to another device, under another user name, gets the same id, and
// every spelling of it on this device does too.
func RepoID(worktree, home string) string {
	worktree, home = OnDisk(worktree), OnDisk(home)
	key := worktree
	if rel, err := filepath.Rel(home, worktree); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		key = "~/" + filepath.ToSlash(rel)
	}
	sum := sha256.Sum256([]byte(key))
	name := strings.Trim(unsafeName.ReplaceAllString(filepath.Base(worktree), "-"), "-.")
	if name == "" {
		name = "repo"
	}
	if len(name) > 40 {
		name = name[:40]
	}
	return name + "-" + hex.EncodeToString(sum[:4])
}
