package cloud

import (
	"os"
	"path/filepath"
	"testing"
)

// The copy schemes of the clients beyond iCloud, Dropbox and Syncthing, from
// their documentation or source; each gives back the original name. Only the
// schemes whose mark no name carries on purpose (a space and a parenthesis
// around a date) match here.
func TestMatchCopyNameMoreClients(t *testing.T) {
	for name, want := range map[string]CopyMatch{
		"notes (conflicted copy jane 2026-10-14 120000).md":          {KindNextcloud, "notes.md"},
		"index (conflicted copy 2026-10-14 120000)":                  {KindNextcloud, "index"},
		"notes (case clash from 2026-10-14 120000).md":               {KindNextcloud, "notes.md"},
		"notes (# Edit conflict 2026-10-14 X1Y2Z3A #).md":            {KindProton, "notes.md"},
		"main (# Name clash 2026-10-14 X1Y2Z3C #)":                   {KindProton, "main"},
		"notes (SFConflict jane@example.com 2026-10-14-12-00-00).md": {KindSeafile, "notes.md"},
	} {
		got, ok := MatchCopyName(name)
		if !ok || got != want {
			t.Errorf("%q: got %+v %v, want %+v", name, got, ok, want)
		}
	}
	// names that only look alike, and names that carry an unmarked scheme's
	// letters on purpose: a branch, a tag, a worktree, a source file
	for _, name := range []string{"release-V2", "notes (1).md", "index-DESKTOP-1A2B", "conflicted.md", "a_conflict.md", "notes (draft).md",
		"fix.Conflict", "resolve.conflicted", "merge_conflict-20240101-120000", "v1.0.Conflict", "wt.Conflict", "Foo.Conflict.cs",
		"notes_conflict-20261014-120000.md", "a_Andy-PC_Jan-03-0901-2013_CaseConflict_1.txt", "index.conflicted"} {
		if m, ok := MatchCopyName(name); ok {
			t.Errorf("%q matched as %+v", name, m)
		}
	}
}

// The schemes whose mark is ordinary letters (Nextcloud's older "_conflict-",
// Synology, pCloud, Resilio) match only here; callers use it for git's own
// names.
func TestMatchUnmarkedCopyName(t *testing.T) {
	for name, want := range map[string]CopyMatch{
		"notes_conflict-20261014-120000.md":             {KindNextcloud, "notes.md"},
		"a_Andy-PC_Jan-03-0901-2013_CaseConflict_1.txt": {KindSynology, "a.txt"},
		"index.conflicted":                              {KindPCloud, "index"},
		"notes.Conflict.md":                             {KindResilio, "notes.md"},
		"notes.Conflict2.md":                            {KindResilio, "notes.md"},
	} {
		got, ok := MatchUnmarkedCopyName(name)
		if !ok || got != want {
			t.Errorf("%q: got %+v %v, want %+v", name, got, ok, want)
		}
	}
	for _, name := range []string{"index", "notes.md", "notes (conflicted copy 2026-10-14 120000).md"} {
		if m, ok := MatchUnmarkedCopyName(name); ok {
			t.Errorf("%q matched as %+v", name, m)
		}
	}
}

// Google Drive's "(1)" matches only through the loose matcher, which callers
// use for git's own names.
func TestMatchLooseCopyName(t *testing.T) {
	for name, want := range map[string]CopyMatch{
		"index (1)":       {KindGoogleDrive, "index"},
		"packed-refs (2)": {KindGoogleDrive, "packed-refs"},
	} {
		got, ok := MatchLooseCopyName(name)
		if !ok || got != want {
			t.Errorf("%q: got %+v %v, want %+v", name, got, ok, want)
		}
	}
	for _, name := range []string{"index", "index-DESKTOP-1A2B"} {
		if m, ok := MatchLooseCopyName(name); ok {
			t.Errorf("%q matched as %+v", name, m)
		}
	}
}

// OneDrive's device suffix is tried at every hyphen, for an original the
// caller knows, and only with a capital in it, as device names have: a
// lowercase "-old" or "-backup" is the user's own name.
func TestMatchOneDriveCopyName(t *testing.T) {
	known := map[string]bool{"index": true, "HEAD": true, "packed-refs": true, "rebase-merge": true, "config": true, "description": true, "hooks": true}
	isGit := func(s string) bool { return known[s] }
	for name, want := range map[string]string{
		"index-DESKTOP-1A2B":          "index",
		"HEAD-LAPTOP":                 "HEAD",
		"packed-refs-ALIPC":           "packed-refs",
		"packed-refs-DESKTOP-1A2B3C4": "packed-refs",
		"rebase-merge-WORK":           "rebase-merge",
		"description-LAPTOP":          "description",
	} {
		got, ok := MatchOneDriveCopyName(name, isGit)
		if !ok || got != (CopyMatch{KindOneDrive, want}) {
			t.Errorf("%q: got %+v %v, want %s", name, got, ok, want)
		}
	}
	for _, name := range []string{"config-backup", "index-old", "description-old", "hooks-disabled", "index", "notes-MACBOOK", "index-2",
		"config-OLD", "config-Backup", "index-BAK", "index-Copy", "packed-refs-TEMP", "HEAD-ORIG",
		"config-BKP", "config-OLD2", "config-Old-1", "config-BACKUP-2024", "config-Backups", "config-Copy2", "index-Before-Rebase"} {
		if m, ok := MatchOneDriveCopyName(name, isGit); ok {
			t.Errorf("%q matched as %+v", name, m)
		}
	}
}

// A Syncthing or Nextcloud folder is known by its marker, in the folder or
// above it; anywhere else there is none.
func TestDetectMarker(t *testing.T) {
	root := t.TempDir()
	st := filepath.Join(root, "sync [1]")
	os.MkdirAll(filepath.Join(st, ".stfolder"), 0o755)
	os.MkdirAll(filepath.Join(st, "proj", "src"), 0o755)
	nc := filepath.Join(root, "Nextcloud")
	os.MkdirAll(filepath.Join(nc, "proj"), 0o755)
	os.WriteFile(filepath.Join(nc, ".sync_ab12cd34ef56.db"), nil, 0o644)
	plain := filepath.Join(root, "plain")
	os.MkdirAll(plain, 0o755)
	for dir, want := range map[string]Provider{
		filepath.Join(st, "proj", "src"): Syncthing,
		filepath.Join(nc, "proj"):        Nextcloud,
		plain:                            None,
	} {
		if got := DetectMarker(dir); got != want {
			t.Errorf("%s: %s, want %s", dir, got, want)
		}
	}
}
