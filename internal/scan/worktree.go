package scan

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/cloud"
)

// errLimit stops the worktree walk once enough entries were visited.
var errLimit = errors.New("entry limit reached")

// WorktreeResult describes conflict copies found in the working tree.
type WorktreeResult struct {
	Copies    []string // slash-separated paths relative to the worktree root
	GitDirs   []string // copies of the git directory itself, e.g. ".git 2"
	Truncated bool     // the walk stopped at the entry limit
}

// skipDirs are never descended into: they are huge, generated, or not ours.
var skipDirs = map[string]bool{".git": true, "node_modules": true}

// looksLikeGitDir tells a copied git directory (it has HEAD and objects/) or a
// copied gitfile (".git 2") from an ordinary folder that happens to be called
// "git 2". Metadata only.
func looksLikeGitDir(p string, d fs.DirEntry) bool {
	if !d.IsDir() {
		return strings.HasPrefix(d.Name(), ".git")
	}
	for _, name := range []string{"HEAD", "objects"} {
		if _, err := os.Lstat(filepath.Join(p, name)); err != nil {
			return false
		}
	}
	return true
}

// ScanWorktree walks root (metadata only) and lists files that look like
// conflict copies of a file that still exists next to them, without entering
// the directories in skip (slash-separated, relative to root). Copies of the
// git directory are listed separately. Dataless directories are skipped.
func ScanWorktree(root string, fr FlagReader, limit int, skip ...string) (WorktreeResult, error) {
	var res WorktreeResult
	visited := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if p == root {
			return walkErr
		}
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		visited++
		if limit > 0 && visited > limit {
			res.Truncated = true
			return errLimit
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() && slices.Contains(skip, rel) {
			return fs.SkipDir
		}
		if filepath.Dir(p) == root && cloud.IsGitDirCopy(d.Name()) && looksLikeGitDir(p, d) {
			res.GitDirs = append(res.GitDirs, rel)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			if dl, _ := fr.IsDataless(p); dl {
				return fs.SkipDir
			}
			return nil
		}
		if m, ok := cloud.MatchCopyName(d.Name()); ok {
			if _, err := os.Lstat(filepath.Join(filepath.Dir(p), m.Original)); err == nil {
				res.Copies = append(res.Copies, rel)
			}
		}
		return nil
	})
	if errors.Is(err, errLimit) {
		err = nil
	}
	return res, err
}
