// Package moveout moves a repository's git directory out of the synced
// folder and leaves a .git file pointing at it. It also prepares and attaches
// the other devices that sync the folder, and moves the git directory back.
package moveout

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/cloud"
	"github.com/aliyilmazco/gitbrella/internal/gitfile"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/icloud"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/state"
	"github.com/aliyilmazco/gitbrella/internal/treecopy"
)

// Options configures every operation of this package. The git directory
// always goes to Dirs.GitDirs/<repo id>: the same place under home on every
// device, on the filesystem of home, so it can always come back in one step.
type Options struct {
	change.Env
	Path  string
	Probe func(string) (icloud.Status, error)
	Dirs  state.Dirs
}

// Refusal is a reason a change cannot be made now.
type Refusal = change.Refusal

// Refusal codes.
const (
	RefuseNotSynced       = "NOT_IN_SYNCED_FOLDER"
	RefuseGitDirMissing   = "GIT_DIR_MISSING"
	RefuseSeparateGitDir  = "SEPARATE_GIT_DIR"
	RefuseBusy            = change.RefuseBusy
	RefuseLinkedWorktrees = "LINKED_WORKTREES"
	RefuseSubmodules      = "SUBMODULES"
	RefuseAlternates      = "ALTERNATES"
	RefuseTargetExists    = "TARGET_EXISTS"
	RefuseTargetSynced    = "TARGET_IN_SYNCED_FOLDER"
	RefuseTargetInside    = "TARGET_INSIDE_WORKTREE"
	RefuseOtherFilesystem = "OTHER_FILESYSTEM"
	RefuseNoSpace         = "NOT_ENOUGH_SPACE"
	RefuseICloudUploading = "ICLOUD_UPLOADING"
	RefuseNotPlain        = "GIT_DIR_NOT_PLAIN"
	RefuseSocket          = "SOCKET_IN_GITDIR"
)

// Plan is what move-out would do, and what stops it.
type Plan struct {
	Repo     check.RepoInfo  `json:"repo"`
	RepoID   string          `json:"repoId"`
	Target   string          `json:"target"`
	Gitfile  string          `json:"gitfile"`
	Head     string          `json:"head"`
	Bytes    int64           `json:"bytes"`
	Already  bool            `json:"alreadyMoved"`
	Refusals []Refusal       `json:"refusals"`
	Carried  []check.Finding `json:"carried"`
}

// MakePlan inspects the repository at o.Path and says what move-out would do.
// It writes nothing. An error means the question could not be answered (not a
// repository, git failed); refusals are answers.
func MakePlan(ctx context.Context, g *gitx.Runner, o Options) (Plan, error) {
	// A .git file whose directory is not on this device: git finds no
	// repository at all, but the answer is known (attach this device).
	if wt, ok := FindWorktree(o.Path); ok {
		if target, isFile, err := gitfile.Read(wt); err == nil && isFile && !exists(target) {
			return Plan{RepoID: state.RepoID(wt, o.Home), Target: target, Carried: []check.Finding{},
				Repo: check.RepoInfo{Worktree: wt, Provider: string(cloud.DetectAt(wt, o.Home, o.GOOS))},
				Refusals: []Refusal{{Code: RefuseGitDirMissing, Message: fmt.Sprintf(
					"%s points at %s, which does not exist on this device; run move-out --attach", filepath.Join(wt, ".git"), target)}}}, nil
		}
	}
	r, err := repo.Locate(ctx, g, o.Path)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{RepoID: state.RepoID(r.Worktree, o.Home), Refusals: []Refusal{}, Carried: []check.Finding{}}
	refuse := func(code, format string, args ...any) {
		p.Refusals = append(p.Refusals, Refusal{Code: code, Message: fmt.Sprintf(format, args...)})
	}
	provider := cloud.DetectAt(r.Worktree, o.Home, o.GOOS)
	p.Repo = check.RepoInfo{Worktree: r.Worktree, GitDir: r.GitDir, CommonDir: r.CommonDir,
		Provider: string(provider), ObjectFormat: r.ObjectFormat, RefStorage: r.RefStorage}
	dotgit := filepath.Join(r.Worktree, ".git")

	target, isFile, err := gitfile.Read(r.Worktree)
	switch {
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		refuse(RefuseNotPlain, "%v; only a plain .git directory is moved", err)
		return p, nil
	case isFile:
		p.Target = target
		p.Already = state.NotSynced(target, o.Home, o.GOOS) == nil
		if !p.Already {
			refuse(RefuseSeparateGitDir, "%s points at %s, which is also in a synced folder; move it back first", dotgit, target)
		}
		return p, nil
	}
	if !sameFile(r.GitDir, dotgit) || !sameFile(r.CommonDir, dotgit) {
		refuse(RefuseSeparateGitDir, "the git directory is not %s (core.worktree, GIT_DIR or a separate git directory); only a plain .git directory is moved", dotgit)
		return p, nil
	}
	if provider == cloud.None || cloud.IsNoSync(r.Worktree) {
		refuse(RefuseNotSynced, "%s is not in a folder a sync client syncs; there is nothing to move it out of", r.Worktree)
	}

	rep, err := change.RunCheck(ctx, g, o.Env, r.Worktree)
	if err != nil {
		return Plan{}, err
	}
	for _, f := range rep.Findings {
		switch {
		case change.Blocking[f.Code]: // a copy now would be inconsistent
			refuse(RefuseBusy, "%s: %s", f.Code, f.Message)
		case f.Severity == check.Damage:
			p.Carried = append(p.Carried, f)
		}
	}
	if names, _ := os.ReadDir(filepath.Join(dotgit, "worktrees")); len(names) > 0 {
		refuse(RefuseLinkedWorktrees, "the repository has linked worktrees (git worktree list); their links would break")
	}
	if _, err := os.Lstat(filepath.Join(dotgit, "modules")); err == nil {
		refuse(RefuseSubmodules, "the repository has submodules in .git/modules; their .git files point into .git and would break")
	}
	if _, err := os.Lstat(filepath.Join(dotgit, "objects", "info", "alternates")); err == nil {
		refuse(RefuseAlternates, "the repository borrows objects from another (objects/info/alternates)")
	}
	if s := socketIn(dotgit); s != "" {
		refuse(RefuseSocket, "%s is a socket (a running 'git fsmonitor--daemon', or one left behind), which cannot be copied; stop it with 'git fsmonitor--daemon stop' or remove the socket", s)
	}

	p.Target = filepath.Join(o.Dirs.GitDirs, p.RepoID)
	switch {
	case exists(p.Target):
		refuse(RefuseTargetExists, "%s already exists; this device may have been prepared (then use --attach)", p.Target)
	case within(p.Target, r.Worktree):
		refuse(RefuseTargetInside, "%s is inside the working tree", p.Target)
	case state.NotSynced(p.Target, o.Home, o.GOOS) != nil:
		refuse(RefuseTargetSynced, "%s is in a synced folder", p.Target)
	}
	for _, d := range []string{o.Dirs.Backups(), p.Target} {
		if !sameDevice(dotgit, d) {
			refuse(RefuseOtherFilesystem, "%s is on another filesystem than %s; the git directory could not be moved (or moved back) in one step", d, dotgit)
		}
	}
	if p.Bytes, err = treecopy.Size(dotgit); err != nil {
		return Plan{}, err
	}
	// An APFS clone on the same volume takes no space; anything else copies.
	if !(o.GOOS == "darwin" && sameDevice(dotgit, p.Target)) {
		if free, ok := freeBytes(p.Target); ok && free < p.Bytes+p.Bytes/10 {
			refuse(RefuseNoSpace, "%s needs %d bytes, %d are free", p.Target, p.Bytes, free)
		}
	}
	if provider == cloud.ICloud || provider == cloud.ICloudDesktopDocuments {
		for _, q := range []string{r.Worktree, dotgit, filepath.Join(dotgit, "HEAD"), filepath.Join(dotgit, "index")} {
			if !exists(q) {
				continue
			}
			s, err := o.Probe(q)
			if err != nil {
				refuse(RefuseICloudUploading, "iCloud's state of %s could not be read: %v", q, err)
				break
			}
			if s.Pending() {
				refuse(RefuseICloudUploading, "iCloud has not finished uploading %s; wait until it has and try again", q)
				break
			}
		}
	}

	p.Gitfile = gitfile.Content(r.Worktree, p.Target, o.Home)
	if p.Head, err = head(ctx, g, r.Worktree); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// PlanWaiting plans like MakePlan, but while iCloud's upload is the only
// refusal it plans again every `every` until wait has passed.
func PlanWaiting(ctx context.Context, g *gitx.Runner, o Options, wait, every time.Duration) (Plan, error) {
	deadline := time.Now().Add(wait)
	for {
		p, err := MakePlan(ctx, g, o)
		if err != nil || !onlyUploading(p.Refusals) || time.Now().Add(every).After(deadline) {
			return p, err
		}
		select {
		case <-ctx.Done():
			return p, nil
		case <-time.After(every):
		}
	}
}

func onlyUploading(rs []Refusal) bool {
	for _, r := range rs {
		if r.Code != RefuseICloudUploading {
			return false
		}
	}
	return len(rs) > 0
}

// socketIn names the first socket under dir, "" if there is none.
func socketIn(dir string) string {
	found := ""
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type()&fs.ModeSocket != 0 {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// head is HEAD's commit, "" on an unborn branch.
func head(ctx context.Context, g *gitx.Runner, worktree string) (string, error) {
	res, err := g.Run(ctx, worktree, nil, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		return "", err
	}
	if res.ExitCode == 1 {
		return "", nil
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("git rev-parse HEAD: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout), nil
}

// status is git's view of the working tree (without refreshing the index).
func status(ctx context.Context, g *gitx.Runner, worktree string) (string, error) {
	res, err := g.Run(ctx, worktree, nil, "status", "--porcelain=v1", "-z")
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("git status: %s", strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// FindWorktree walks up from path to the first directory holding a .git
// entry, the way git starts its search, without asking git.
func FindWorktree(path string) (string, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		if exists(filepath.Join(dir, ".git")) {
			return dir, true
		}
		if filepath.Dir(dir) == dir {
			return "", false
		}
	}
}

func sameFile(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// existingAncestor is p or its nearest existing parent.
func existingAncestor(p string) string {
	for {
		if exists(p) || filepath.Dir(p) == p {
			return p
		}
		p = filepath.Dir(p)
	}
}

// sameDevice reports whether a and b (or their nearest existing parents)
// are on one filesystem, where a rename between them is one step.
func sameDevice(a, b string) bool {
	var x, y syscall.Stat_t
	if syscall.Stat(existingAncestor(a), &x) != nil || syscall.Stat(existingAncestor(b), &y) != nil {
		return false
	}
	return x.Dev == y.Dev
}

func freeBytes(p string) (int64, bool) {
	var s syscall.Statfs_t
	if syscall.Statfs(existingAncestor(p), &s) != nil {
		return 0, false
	}
	return int64(s.Bavail) * int64(s.Bsize), true
}

// rename moves across the iCloud folder edge (see change.Rename); every move
// here is of a git directory the check found local.
var rename = change.Rename
