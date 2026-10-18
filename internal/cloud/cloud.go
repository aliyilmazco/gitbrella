// Package cloud knows where sync clients keep their folders and how they name
// the conflict copies they create.
package cloud

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Provider names a sync client.
type Provider string

const (
	None                   Provider = "none"
	ICloud                 Provider = "icloud"
	ICloudDesktopDocuments Provider = "icloud-desktop-documents" // ~/Desktop and ~/Documents when that iCloud option is on
	Dropbox                Provider = "dropbox"
	OneDrive               Provider = "onedrive"
	GoogleDrive            Provider = "google-drive"
	Syncthing              Provider = "syncthing" // a folder holding Syncthing's .stfolder marker
	Nextcloud              Provider = "nextcloud" // a folder holding the Nextcloud or ownCloud client's sync database
	OtherCloudStorage      Provider = "cloud-storage"
)

// Detect returns the provider that probably syncs path. home is the user's
// home directory and goos is runtime.GOOS; both are parameters for testing.
func Detect(path, home, goos string) Provider {
	p := filepath.Clean(path)
	in := func(dir string) bool {
		dir = filepath.Clean(dir)
		return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
	}
	cloudStorage := filepath.Join(home, "Library", "CloudStorage")
	switch {
	case in(filepath.Join(home, "Library", "Mobile Documents")):
		return ICloud
	case in(cloudStorage):
		top := strings.SplitN(strings.TrimPrefix(p, cloudStorage+string(filepath.Separator)), string(filepath.Separator), 2)[0]
		switch {
		case strings.HasPrefix(top, "Dropbox"):
			return Dropbox
		case strings.HasPrefix(top, "OneDrive"):
			return OneDrive
		case strings.HasPrefix(top, "GoogleDrive"):
			return GoogleDrive
		}
		return OtherCloudStorage
	case in(filepath.Join(home, "Dropbox")):
		return Dropbox
	case goos == "darwin" && (in(filepath.Join(home, "Documents")) || in(filepath.Join(home, "Desktop"))):
		return ICloudDesktopDocuments
	}
	return None
}

// CopyKind names the client whose naming scheme matched.
type CopyKind string

const (
	KindICloud      CopyKind = "icloud"
	KindDropbox     CopyKind = "dropbox"
	KindSyncthing   CopyKind = "syncthing"
	KindNextcloud   CopyKind = "nextcloud" // and ownCloud
	KindProton      CopyKind = "proton"
	KindSeafile     CopyKind = "seafile"
	KindSynology    CopyKind = "synology"
	KindPCloud      CopyKind = "pcloud"
	KindResilio     CopyKind = "resilio"
	KindOneDrive    CopyKind = "onedrive"     // loose: see MatchLooseCopyName
	KindGoogleDrive CopyKind = "google-drive" // loose: see MatchLooseCopyName
)

// CopyMatch describes a name that looks like a conflict copy.
type CopyMatch struct {
	Kind     CopyKind
	Original string // the name the copy was made from
}

var (
	// "index 2", "main 3", "pack-1a2b 2.pack": iCloud inserts " N" (N >= 2) before the last extension.
	icloudRe = regexp.MustCompile(`^(.+) ([2-9]|[1-9][0-9]+)(\.[^ .]+)?$`)
	// "HEAD (Jane's conflicted copy 2026-10-08)", "notes (Case Conflict).md".
	dropboxRe = regexp.MustCompile(`^(.+?) \((.*'s conflicted copy.*|Case Conflict.*|Selective Sync Conflict.*)\)(\.[^ .()]+)?$`)
	// "index.sync-conflict-20180827-123456-ABCDEF1", "a.sync-conflict-20180827-123456-ABCDEF1.md".
	syncthingRe = regexp.MustCompile(`^(.+?)\.sync-conflict-[0-9]{8}-[0-9]{6}-[A-Z0-9]{7}(\.[^.]+)?$`)
	// Nextcloud "a (conflicted copy jane 2026-10-14 120000).md", ownCloud the
	// same without the user, the older "a_conflict-20261014-120000.md" and
	// "a (case clash from 2026-10-14 120000).md".
	nextcloudRe    = regexp.MustCompile(`^(.+?) \((?:conflicted copy (?:[^()]* )?|case clash from )[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{6}\)(\.[^ .()]+)?$`)
	nextcloudOldRe = regexp.MustCompile(`^(.+?)_conflict-[0-9]{8}-[0-9]{6}(\.[^.]+)?$`)
	// Proton Drive "a (# Edit conflict 2026-10-14 X1Y2Z3A #).md", also "Name
	// clash" and "Delete conflict".
	protonRe = regexp.MustCompile(`^(.+?) \(# (?:Edit conflict|Name clash|Delete conflict) [0-9]{4}-[0-9]{2}-[0-9]{2} [A-Za-z0-9]+ #\)(\.[^ .()]+)?$`)
	// Seafile "a (SFConflict jane@example.com 2026-10-14-12-00-00).md".
	seafileRe = regexp.MustCompile(`^(.+?) \(SFConflict [^ ()]+ [0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{2}\)(\.[^ .()]+)?$`)
	// Synology Drive "a_Andy-PC_Jan-03-0901-2013_CaseConflict_1.txt".
	synologyRe = regexp.MustCompile(`^(.+?)_[^_/]+_[A-Z][a-z]{2}-[0-9]{2}-[0-9]{4}-[0-9]{4}_(?:CaseConflict|WhiteSpaceConflict|TailCharacterConflict|TypeConflict|Conflict)_[0-9]+(\.[^.]+)?$`)
	// pCloud "a.conflicted".
	pcloudRe = regexp.MustCompile(`^(.+)\.conflicted$`)
	// Resilio Sync "a.Conflict.md", "a.Conflict1.md".
	resilioRe = regexp.MustCompile(`^(.+?)\.Conflict[0-9]*(\.[^.]+)?$`)
	// Too loose for names in general, see MatchLooseCopyName: Google Drive
	// "index (1)".
	googleDriveRe = regexp.MustCompile(`^(.+) \(([1-9][0-9]*)\)(\.[^ .()]+)?$`)
	// A OneDrive device name as it ends a copy ("index-DESKTOP-1A2B"): at
	// most 15 letters, digits and hyphens, a Windows computer name.
	deviceRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,14}$`)
)

type copyScheme struct {
	kind CopyKind
	re   *regexp.Regexp
	ext  int // the submatch holding the extension after the copy mark
}

// copySchemes are the names MatchCopyName knows, in the order tried: each
// marks a copy with a space and a parenthesis, or (Syncthing) a timestamp and
// a device id, which no name a user picks carries.
var copySchemes = []copyScheme{
	{KindSyncthing, syncthingRe, 2},
	{KindDropbox, dropboxRe, 3},
	{KindNextcloud, nextcloudRe, 2},
	{KindProton, protonRe, 2},
	{KindSeafile, seafileRe, 2},
	{KindICloud, icloudRe, 3},
}

// unmarkedSchemes mark a copy with ordinary letters a branch, a tag or a
// source file may carry on purpose ("fix.Conflict", "v1.0.conflicted",
// "merge_conflict-20240101-120000", "Foo.Conflict.cs").
var unmarkedSchemes = []copyScheme{
	{KindNextcloud, nextcloudOldRe, 2},
	{KindSynology, synologyRe, 2},
	{KindResilio, resilioRe, 2},
	{KindPCloud, pcloudRe, 0},
}

func matchSchemes(schemes []copyScheme, name string) (CopyMatch, bool) {
	for _, sc := range schemes {
		if m := sc.re.FindStringSubmatch(name); m != nil {
			orig := m[1]
			if sc.ext > 0 {
				orig += m[sc.ext]
			}
			return CopyMatch{Kind: sc.kind, Original: orig}, true
		}
	}
	return CopyMatch{}, false
}

// MatchCopyName reports whether name looks like a conflict copy, by a scheme
// that marks copies unmistakably (iCloud's " 2" holds a space, which no ref
// name and few file names carry next to a numbered twin).
func MatchCopyName(name string) (CopyMatch, bool) {
	return matchSchemes(copySchemes, name)
}

// MatchUnmarkedCopyName matches the schemes whose mark is ordinary letters:
// Nextcloud's older "_conflict-YYYYMMDD-HHMMSS", Synology Drive, Resilio
// Sync, pCloud. They are safe only for the names git itself makes at the top
// of its directory, which is for the caller to check; never for refs, logs,
// worktrees or a working tree.
func MatchUnmarkedCopyName(name string) (CopyMatch, bool) {
	return matchSchemes(unmarkedSchemes, name)
}

// MatchLooseCopyName matches Google Drive's " (1)", too loose for file names
// in general. Inside the git directory it is safe for the names git itself
// makes, and below the top (no ref name holds a space); never for a working
// tree.
func MatchLooseCopyName(name string) (CopyMatch, bool) {
	if m := googleDriveRe.FindStringSubmatch(name); m != nil {
		return CopyMatch{Kind: KindGoogleDrive, Original: m[1] + m[3]}, true
	}
	return CopyMatch{}, false
}

// notDevices are words people end a backup's name with; as a OneDrive device
// name they would make the user's "config-OLD" a sync copy.
var notDevices = []string{"old", "new", "bak", "bkp", "backup", "backups", "copy", "copies", "orig", "original", "save", "saved",
	"tmp", "temp", "test", "final", "prev", "previous", "local", "remote", "draft", "wip", "before", "after", "broken",
	"good", "bad", "latest", "keep", "disabled", "off", "archive", "archived", "rebase", "merge"}

// backupWord reports whether a device name holds a backup word: as any of its
// hyphen parts, with trailing digits off ("OLD2", "Old-1", "BACKUP-2024").
func backupWord(device string) bool {
	for _, part := range strings.Split(device, "-") {
		if w := strings.ToLower(strings.TrimRight(part, "0123456789")); slices.Contains(notDevices, w) {
			return true
		}
	}
	return false
}

// MatchOneDriveCopyName matches OneDrive's "-COMPUTERNAME" copies of a name
// isOriginal accepts, trying every hyphen ("packed-refs-ALIPC" is a copy of
// packed-refs). The device name holds a capital, as computer names do, and is
// no word people end a backup's name with: "-old", "-Backup" or "-BAK" is the
// user's own name.
func MatchOneDriveCopyName(name string, isOriginal func(string) bool) (CopyMatch, bool) {
	stem, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > 0 && !strings.Contains(name[i:], "-") {
		stem, ext = name[:i], name[i:]
	}
	for i := 1; i < len(stem); i++ {
		if stem[i] != '-' {
			continue
		}
		orig, device := stem[:i]+ext, stem[i+1:]
		if deviceRe.MatchString(device) && strings.ContainsAny(device, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") && !backupWord(device) && isOriginal(orig) {
			return CopyMatch{Kind: KindOneDrive, Original: orig}, true
		}
	}
	return CopyMatch{}, false
}

// DetectAt is Detect, then, where no known folder holds path, DetectMarker:
// the clients that sync any folder (Syncthing, Nextcloud) are known by the
// marker they keep in it.
func DetectAt(path, home, goos string) Provider {
	if p := Detect(path, home, goos); p != None {
		return p
	}
	return DetectMarker(path)
}

// DetectMarker returns the sync client whose marker file sits in path or a
// folder above it: Syncthing's ".stfolder", the Nextcloud or ownCloud
// client's ".sync_*.db" ("._sync_*.db" on older versions) or
// ".owncloudsync.log". It reads directory entries only, no file content.
func DetectMarker(path string) Provider {
	for d := filepath.Clean(path); ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".stfolder")); err == nil {
			return Syncthing
		}
		if _, err := os.Lstat(filepath.Join(d, ".owncloudsync.log")); err == nil {
			return Nextcloud
		}
		for _, pat := range []string{".sync_*.db", "._sync_*.db"} {
			if m, _ := filepath.Glob(filepath.Join(globEscape(d), pat)); len(m) > 0 {
				return Nextcloud
			}
		}
		if filepath.Dir(d) == d {
			return None
		}
	}
}

// globEscape quotes the glob characters in a directory name.
func globEscape(dir string) string {
	var b strings.Builder
	for _, c := range dir {
		if strings.ContainsRune(`*?[\`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// IsSyncMetadata reports whether name is a file that sync clients or desktop
// shells drop into folders (Google Drive and Windows "desktop.ini", the macOS
// custom-icon file "Icon\r", "Thumbs.db", Dropbox attribute files). Inside
// .git/refs or .git/objects git reads such a file as a ref or object and fails.
func IsSyncMetadata(name string) bool {
	switch {
	case strings.EqualFold(name, "desktop.ini"), strings.EqualFold(name, "Thumbs.db"):
		return true
	case name == "Icon\r", name == ".dropbox", name == ".dropbox.attr":
		return true
	}
	return false
}

// IsNoSync reports whether any component of path ends in ".nosync", a name
// iCloud Drive leaves out of syncing.
func IsNoSync(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasSuffix(part, ".nosync") {
			return true
		}
	}
	return false
}

// IsGitDirCopy reports whether name looks like a renamed or copied git
// directory, such as ".git 2" or "git 2" (iCloud) or ".git (conflicted copy)".
func IsGitDirCopy(name string) bool {
	m, ok := MatchCopyName(name)
	return ok && (m.Original == ".git" || m.Original == "git")
}
