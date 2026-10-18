package cloud

import "testing"

func TestDetect(t *testing.T) {
	home := "/Users/jane"
	cases := []struct {
		path, goos string
		want       Provider
	}{
		{"/Users/jane/Library/Mobile Documents/iCloud~md~obsidian/Documents/Notes", "darwin", ICloud},
		{"/Users/jane/Library/Mobile Documents", "darwin", ICloud},
		{"/Users/jane/Library/CloudStorage/Dropbox/code", "darwin", Dropbox},
		{"/Users/jane/Library/CloudStorage/OneDrive-Personal/x", "darwin", OneDrive},
		{"/Users/jane/Library/CloudStorage/GoogleDrive-jane@example.com/My Drive/r", "darwin", GoogleDrive},
		{"/Users/jane/Library/CloudStorage/Box-Box/r", "darwin", OtherCloudStorage},
		{"/Users/jane/Dropbox/r", "linux", Dropbox},
		{"/Users/jane/Documents/proj", "darwin", ICloudDesktopDocuments},
		{"/Users/jane/Documents/proj", "linux", None},
		{"/Users/jane/DocumentsArchive/proj", "darwin", None},
		{"/Users/jane/codes/proj", "darwin", None},
	}
	for _, c := range cases {
		if got := Detect(c.path, home, c.goos); got != c.want {
			t.Errorf("Detect(%q, %s) = %s, want %s", c.path, c.goos, got, c.want)
		}
	}
}

func TestMatchCopyName(t *testing.T) {
	cases := []struct {
		name     string
		ok       bool
		kind     CopyKind
		original string
	}{
		{"index 2", true, KindICloud, "index"},
		{"index 10", true, KindICloud, "index"},
		{"main 2", true, KindICloud, "main"},
		{"pack-1a2b 2.pack", true, KindICloud, "pack-1a2b.pack"},
		{"index 2.lock", true, KindICloud, "index.lock"},
		{"refs 2", true, KindICloud, "refs"},
		{"HEAD (Jane's MacBook Pro's conflicted copy 2026-10-08)", true, KindDropbox, "HEAD"},
		{"config (Jane's conflicted copy 2026-10-08 (1))", true, KindDropbox, "config"},
		{"notes (Case Conflict).md", true, KindDropbox, "notes.md"},
		{"refs (Selective Sync Conflict)", true, KindDropbox, "refs"},
		{"index.sync-conflict-20180827-123456-ABCDEF1", true, KindSyncthing, "index"},
		{"note.sync-conflict-20260101-090000-Z1Y2X3W.md", true, KindSyncthing, "note.md"},
		{"index 1", false, "", ""},
		{"index", false, "", ""},
		{"index2", false, "", ""},
		{"main-2", false, "", ""},
		{"ORIG_HEAD", false, "", ""},
	}
	for _, c := range cases {
		m, ok := MatchCopyName(c.name)
		if ok != c.ok || m.Kind != c.kind || m.Original != c.original {
			t.Errorf("MatchCopyName(%q) = %+v, %v; want %s %q %v", c.name, m, ok, c.kind, c.original, c.ok)
		}
	}
}

func TestIsSyncMetadata(t *testing.T) {
	for _, n := range []string{"desktop.ini", "Desktop.ini", "Icon\r", "Thumbs.db", ".dropbox.attr"} {
		if !IsSyncMetadata(n) {
			t.Errorf("IsSyncMetadata(%q) = false", n)
		}
	}
	for _, n := range []string{"Icon", "main", "index", ".DS_Store"} {
		if IsSyncMetadata(n) {
			t.Errorf("IsSyncMetadata(%q) = true", n)
		}
	}
}

func TestIsGitDirCopy(t *testing.T) {
	for _, n := range []string{".git 2", "git 2", ".git (Jane's conflicted copy 2026-10-08)", ".git.sync-conflict-20260101-090000-ABCDEF1"} {
		if !IsGitDirCopy(n) {
			t.Errorf("IsGitDirCopy(%q) = false", n)
		}
	}
	for _, n := range []string{".git", ".github 2", "gitignore 2"} {
		if IsGitDirCopy(n) {
			t.Errorf("IsGitDirCopy(%q) = true", n)
		}
	}
}

func TestIsNoSync(t *testing.T) {
	cases := map[string]bool{
		"/Users/a/Library/Mobile Documents/com~apple~CloudDocs/p/.git.nosync":     true,
		"/Users/a/Library/Mobile Documents/com~apple~CloudDocs/p.nosync/.git":     true,
		"/Users/a/Library/Mobile Documents/com~apple~CloudDocs/p/.git":            false,
		"/Users/a/Library/Mobile Documents/com~apple~CloudDocs/nosync/.git":       false,
		"/Users/a/Library/Mobile Documents/com~apple~CloudDocs/p/.git.nosync.bak": false,
	}
	for p, want := range cases {
		if got := IsNoSync(p); got != want {
			t.Errorf("IsNoSync(%q) = %v, want %v", p, got, want)
		}
	}
}
