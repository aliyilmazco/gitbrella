// Package change holds what the commands that change something (move-out,
// undo, fix) share: the reasons they refuse, the check they run before
// acting, and moves across the edge of an iCloud Drive folder.
package change

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/scan"
)

// Refusal is a reason a change cannot be made now.
type Refusal struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RefuseBusy: git, or something holding a git lock, works on the repository.
const RefuseBusy = "REPOSITORY_BUSY"

// Blocking are the check findings that forbid touching the git directory:
// git at work, a lock someone may hold, an operation half done, files only in
// the cloud, entries that cannot be read, a change of this tool not finished,
// or a git directory the sync client brought back after a move (two exist:
// acting on one would bury the other's history).
var Blocking = map[string]bool{
	check.CodeGitRunning: true, check.CodeLockInUse: true, check.CodeLockRecent: true,
	check.CodeInProgress: true, check.CodeDataless: true, check.CodeAlternateDataless: true,
	check.CodeReftable: true, check.CodeProcessCheckFail: true, check.CodeUnreadable: true,
	check.CodeGitTempFile: true, check.CodeWorktreeGuessed: true,
	check.CodeChangeUnfinished: true, check.CodeGitDirReturned: true,
}

// Env is what the check needs to look at a repository on this device.
type Env struct {
	Home       string // with symlinks resolved
	GOOS       string
	Flags      scan.FlagReader
	Procs      func(context.Context) ([]procs.Proc, error)
	Version    string
	JournalDir string // this device's journal: unfinished changes and returned git directories
	Deep       bool   // also run git fsck (fix --deep: before the fix and after it)
	Provider   string // the sync client given with --provider; "" detects it from the path
}

// RunCheck runs the read-only check on worktree. The time is taken now: a
// second check must judge locks written after the first.
func RunCheck(ctx context.Context, g *gitx.Runner, env Env, worktree string) (check.Report, error) {
	return check.Run(ctx, g, check.Options{Path: worktree, Provider: env.Provider, Home: env.Home, GOOS: env.GOOS,
		Now: time.Now(), Flags: env.Flags, Procs: env.Procs, StaleAfter: time.Minute, Version: env.Version,
		JournalDir: env.JournalDir, Deep: env.Deep})
}

// Busy runs the check and returns the findings that forbid a change now.
func Busy(ctx context.Context, g *gitx.Runner, env Env, worktree string) ([]Refusal, error) {
	rep, err := RunCheck(ctx, g, env, worktree)
	if err != nil {
		return nil, err
	}
	var out []Refusal
	for _, f := range rep.Findings {
		if Blocking[f.Code] {
			out = append(out, Refusal{Code: RefuseBusy, Message: f.Code + ": " + f.Message})
		}
	}
	return out, nil
}

// Rename moves a file or directory. Moves out of and into an iCloud Drive
// folder need the file provider, which the process's no-download policy
// stops (EDEADLK); callers move only what they checked to be local
// (LocalOnly), so the policy is lifted for that thread alone.
func Rename(from, to string) error {
	return scan.AllowMaterialization(func() error { return os.Rename(from, to) })
}

// ErrNotLocal: something under the path exists only in the cloud.
var ErrNotLocal = errors.New("exists only in the cloud")

// LocalOnly fails if path, or anything under it, is a cloud-only (dataless)
// file or folder: moving it with the policy lifted would download it.
func LocalOnly(path string, fr scan.FlagReader) error {
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dl, err := fr.IsDataless(p); err != nil {
			return err
		} else if dl {
			return fmt.Errorf("%s %w; download it first (Finder: Download Now)", p, ErrNotLocal)
		}
		return nil
	})
}
