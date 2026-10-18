// Package scan walks a git directory and finds what cloud sync left behind:
// conflict copies, leftover locks, dataless files and foreign entries.
// It only reads metadata; it never opens a file.
package scan

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/cloud"
)

// FlagReader tells whether a path is a dataless cloud placeholder without
// reading or downloading it.
type FlagReader interface {
	IsDataless(path string) (bool, error)
}

// Copy is a conflict copy found inside the git directory.
type Copy struct {
	Rel            string // slash-separated, relative to the scanned directory
	IsDir          bool
	Kind           cloud.CopyKind
	OriginalRel    string
	OriginalExists bool
}

// Lock is a git-owned lock file.
type Lock struct {
	Rel string
	Age time.Duration
}

// Result is everything Scan found.
type Result struct {
	Copies          []Copy
	Locks           []Lock
	Dataless        []string // dataless files and directories
	Unknown         []string // entries git does not create, outside refs/ and objects/
	Foreign         []string // files git would misread: sync metadata or unexpected names in refs/ or objects/
	Empty           []string // zero-byte files where git expects content
	BrokenWorktrees []string // worktrees/<name> missing HEAD, gitdir or commondir
	Unreadable      []string
	Temp            []string // temporary files git names after a process id (next-index-<pid>.lock, index.stash.<pid>)
}

var (
	looseObjectRe = regexp.MustCompile(`^objects/[0-9a-f]{2}/([0-9a-f]{38}|[0-9a-f]{62}|tmp_obj_.*)$`)
	packFileRe    = regexp.MustCompile(`^objects/pack/(pack-([0-9a-f]{40}|[0-9a-f]{64})\.(pack|idx|rev|bitmap|mtimes|promisor|keep)|multi-pack-index(-[0-9a-f]+\.(bitmap|rev))?|tmp_.*|\.tmp-[0-9]+-.*|multi-pack-index\.d/(multi-pack-index-chain|multi-pack-index-[0-9a-f]+\.(midx|bitmap|rev)|tmp_midx_.*))$`)
	gitTempRe     = regexp.MustCompile(`^(worktrees/[^/]+/)?(next-index-[0-9]+\.lock|index\.stash\.[0-9]+(\.lock)?)$`)
	objectDirRe   = regexp.MustCompile(`^objects/([0-9a-f]{2}|pack|pack/multi-pack-index\.d|info|incoming-.*|tmp_objdir-.*)$`)
)

// validObjectsEntry reports whether rel (under objects/) is a name git uses.
func validObjectsEntry(rel string, isDir bool) bool {
	parts := strings.Split(rel, "/")
	if len(parts) > 1 && (parts[1] == "info" || strings.HasPrefix(parts[1], "incoming-") || strings.HasPrefix(parts[1], "tmp_objdir-")) {
		return true
	}
	if isDir {
		return objectDirRe.MatchString(rel)
	}
	if strings.HasSuffix(rel, ".lock") {
		return true // git's own locks (maintenance.lock, multi-pack-index.lock); judged as locks
	}
	if strings.HasPrefix(rel, "objects/bitmap-ref-tips_") {
		return true // gc writing a bitmap
	}
	return looseObjectRe.MatchString(rel) || packFileRe.MatchString(rel)
}

// mayBeEmpty reports whether git legitimately writes rel as a zero-byte file.
func mayBeEmpty(rel string) bool {
	return strings.HasSuffix(rel, ".keep") || strings.HasSuffix(rel, ".promisor") || strings.HasSuffix(rel, ".lock") ||
		strings.HasPrefix(rel, "objects/info/") || strings.Contains(rel, "/tmp_obj_") || // a loose object being written
		strings.HasPrefix(rel, "objects/bitmap-ref-tips_") || // gc writing a bitmap
		strings.HasPrefix(rel, "objects/pack/tmp_") || strings.HasPrefix(rel, "objects/pack/.tmp-") // a pack being written
}

// knownTop lists the top-level names git itself creates in a git directory.
var knownTop = map[string]bool{
	"HEAD": true, "ORIG_HEAD": true, "FETCH_HEAD": true, "MERGE_HEAD": true, "MERGE_MSG": true,
	"MERGE_MODE": true, "MERGE_RR": true, "AUTO_MERGE": true, "CHERRY_PICK_HEAD": true,
	"REVERT_HEAD": true, "REBASE_HEAD": true, "COMMIT_EDITMSG": true, "TAG_EDITMSG": true, "SQUASH_MSG": true,
	"fsmonitor--daemon": true, "fsmonitor--daemon.ipc": true,
	"gc.pid": true, "gc.log": true, "MERGE_AUTOSTASH": true,
	"index": true, "sharedindex": true, "packed-refs": true, "config": true, "config.worktree": true,
	"description": true, "shallow": true, "gitk.cache": true, "gitdir": true, "commondir": true,
	"objects": true, "refs": true, "logs": true, "hooks": true, "info": true, "worktrees": true,
	"modules": true, "rebase-merge": true, "rebase-apply": true, "sequencer": true,
	"reftable": true, "lfs": true, "rr-cache": true, "branches": true, "remotes": true,
}

// IsGitName reports whether rel (slash-separated, relative to a git dir) is a
// path git itself creates. Lock suffixes are ignored.
func IsGitName(rel string) bool {
	top, _, nested := strings.Cut(rel, "/")
	if nested {
		return knownTop[top]
	}
	base := strings.TrimSuffix(top, ".lock")
	return knownTop[base] || strings.HasPrefix(base, "BISECT_") || strings.HasPrefix(base, "NOTES_MERGE_") ||
		strings.HasPrefix(base, "sharedindex.") || strings.HasPrefix(base, "next-index-") || strings.HasPrefix(base, "index.stash.")
}

// looseCopy matches the schemes whose mark a name may carry on purpose, where
// it cannot be a name the user picked: Google Drive's " (1)" for git's names
// at the top and anywhere below (no ref name holds a space); OneDrive's
// "-DEVICE" for git's names at the top of a OneDrive folder; the unmarked
// schemes (Nextcloud's older "_conflict-", Synology, Resilio, pCloud) for
// git's names at the top, and Nextcloud's also under refs/ and logs/ in a
// Nextcloud folder when the original is there too (needsOriginal). A branch
// "fix.Conflict" or a file "config-backup" stays a name.
func looseCopy(name, rel string, provider cloud.Provider) (m cloud.CopyMatch, ok, needsOriginal bool) {
	top := !strings.Contains(rel, "/")
	if m, ok := cloud.MatchLooseCopyName(name); ok && (!top || IsGitName(m.Original)) {
		return m, true, false
	}
	// a linked worktree's own files (worktrees/<name>/index) are the top of
	// its git directory
	if parts := strings.SplitN(rel, "/", 3); len(parts) == 3 && parts[0] == "worktrees" {
		rel = parts[2]
		top = !strings.Contains(rel, "/")
	}
	if top && provider == cloud.OneDrive {
		if m, ok := cloud.MatchOneDriveCopyName(name, IsGitName); ok {
			return m, true, false
		}
	}
	if m, ok := cloud.MatchUnmarkedCopyName(name); ok {
		switch {
		case top && IsGitName(m.Original):
			return m, true, false
		case (strings.HasPrefix(rel, "refs/") || strings.HasPrefix(rel, "logs/")) && provider == cloud.Nextcloud && m.Kind == cloud.KindNextcloud:
			return m, true, true
		}
	}
	return cloud.CopyMatch{}, false, false
}

// noOriginalNeeded reports whether a copy of rel counts even when rel itself
// is gone. Names under refs/, logs/, objects/ and worktrees/ follow git's own
// patterns; under modules/, hooks/ or info/ the user picks names, so a name
// ending in " 5" is only a copy when the original sits next to it.
func noOriginalNeeded(rel string) bool {
	if !IsGitName(rel) {
		return false
	}
	top, _, nested := strings.Cut(rel, "/")
	return !nested || top == "refs" || top == "logs" || top == "objects" || top == "worktrees"
}

// Scan walks dir, the git directory of a repository provider syncs. Call
// PreventMaterialization first on macOS.
func Scan(dir string, fr FlagReader, now time.Time, provider cloud.Provider) (Result, error) {
	var res Result
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if p == dir {
			return walkErr
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if walkErr != nil {
			if errors.Is(walkErr, syscall.EDEADLK) {
				res.Dataless = append(res.Dataless, rel)
			} else {
				res.Unreadable = append(res.Unreadable, rel)
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == ".DS_Store" {
			return nil // Finder metadata, harmless
		}
		if dataless, _ := fr.IsDataless(p); dataless {
			res.Dataless = append(res.Dataless, rel)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		m, ok := cloud.MatchCopyName(d.Name())
		needsOriginal := false
		if !ok {
			m, ok, needsOriginal = looseCopy(d.Name(), rel, provider)
		}
		if ok {
			origRel := m.Original
			if parent := path.Dir(rel); parent != "." {
				origRel = parent + "/" + m.Original
			}
			_, statErr := os.Lstat(filepath.Join(dir, filepath.FromSlash(origRel)))
			exists := statErr == nil
			if !strings.Contains(rel, "/") && !IsGitName(origRel) {
				// A copy of another tool's file is that tool's business.
				res.Unknown = append(res.Unknown, rel)
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if exists || !needsOriginal && noOriginalNeeded(origRel) {
				res.Copies = append(res.Copies, Copy{Rel: rel, IsDir: d.IsDir(), Kind: m.Kind, OriginalRel: origRel, OriginalExists: exists})
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		top, _, nested := strings.Cut(rel, "/")
		inRefsOrObjects := nested && (top == "refs" || top == "objects")
		if cloud.IsSyncMetadata(d.Name()) {
			if inRefsOrObjects {
				res.Foreign = append(res.Foreign, rel)
			} else {
				res.Unknown = append(res.Unknown, rel)
			}
			return nil
		}
		if !nested && !IsGitName(rel) {
			res.Unknown = append(res.Unknown, rel)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if nested && top == "objects" && !validObjectsEntry(rel, d.IsDir()) {
			res.Foreign = append(res.Foreign, rel)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() && (inRefsOrObjects || rel == "HEAD") && !mayBeEmpty(rel) {
			if info, err := d.Info(); err == nil && info.Size() == 0 {
				res.Empty = append(res.Empty, rel)
			}
		}
		if !d.IsDir() && gitTempRe.MatchString(rel) {
			res.Temp = append(res.Temp, rel) // per-process names: no other git waits on them
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(rel, ".lock") {
			if info, err := d.Info(); err == nil {
				res.Locks = append(res.Locks, Lock{Rel: rel, Age: now.Sub(info.ModTime())})
			}
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	res.BrokenWorktrees = brokenWorktrees(dir, fr)
	return res, nil
}

// brokenWorktrees lists linked-worktree admin directories that lost files git needs.
func brokenWorktrees(dir string, fr FlagReader) []string {
	entries, err := os.ReadDir(filepath.Join(dir, "worktrees"))
	if err != nil {
		return nil
	}
	var broken []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := cloud.MatchCopyName(e.Name()); ok {
			continue // reported as a copy
		}
		base := filepath.Join(dir, "worktrees", e.Name())
		if d, _ := fr.IsDataless(base); d {
			continue
		}
		for _, f := range []string{"HEAD", "gitdir", "commondir"} {
			if _, err := os.Lstat(filepath.Join(base, f)); err != nil {
				broken = append(broken, "worktrees/"+e.Name())
				break
			}
		}
	}
	return broken
}
