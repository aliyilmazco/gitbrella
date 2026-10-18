package moveout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// Refusal codes of move-in.
const (
	RefuseNothingToUndo  = "NOTHING_TO_UNDO"
	RefuseChangedSince   = "CHANGED_SINCE"
	RefuseGitfileInCloud = "GITFILE_IN_CLOUD"
)

// UndoPlan is what move-in would do.
type UndoPlan struct {
	Worktree string    `json:"worktree"`
	RepoID   string    `json:"repoId"`
	Undoes   string    `json:"undoes"` // the move-out's journal id
	GitDir   string    `json:"gitDir"` // the git directory that comes back (with every commit since the move)
	Backup   string    `json:"backup"` // the original from before the move: kept, not used
	Refusals []Refusal `json:"refusals"`
	out      journal.Entry
}

// PlanMoveIn says what undoing the last move-out of the repository at o.Path
// would do: the git directory in use now comes back as .git (so commits made
// since the move stay); the backup of the original is left where it is. Only
// what the move-out left, by inode, is undone: its .git file and its copy.
func PlanMoveIn(ctx context.Context, g *gitx.Runner, o Options, store journal.Store) (UndoPlan, error) {
	wt, ok := FindWorktree(o.Path)
	if !ok {
		return UndoPlan{}, fmt.Errorf("no repository at %s", o.Path)
	}
	wt = state.OnDisk(wt)
	u := UndoPlan{Worktree: wt, RepoID: state.RepoID(wt, o.Home), Refusals: []Refusal{}}
	refuse := func(code, format string, args ...any) {
		u.Refusals = append(u.Refusals, Refusal{Code: code, Message: fmt.Sprintf(format, args...)})
	}
	last, ok, err := store.LastDone(u.RepoID, "move-out")
	if err != nil {
		return UndoPlan{}, err
	}
	if !ok {
		refuse(RefuseNothingToUndo, "this device has no move-out of %s to undo", wt)
		return u, nil
	}
	u.out = last
	u.Undoes, u.GitDir, u.Backup = last.ID, last.Data["target"], last.Data["backup"]
	if o.Flags != nil {
		if dl, _ := o.Flags.IsDataless(filepath.Join(wt, ".git")); dl {
			refuse(RefuseGitfileInCloud, "%s/.git exists only in the cloud; download it first (Finder: Download Now), then run undo again", wt)
			return u, nil
		}
	}
	if !ourGitfile(filepath.Join(wt, ".git"), last.Data) {
		refuse(RefuseChangedSince, "%s/.git is not the .git file move-out wrote; undo only what this tool left as it was", wt)
		return u, nil
	}
	if state.FileID(u.GitDir) != last.Data["copyID"] {
		refuse(RefuseChangedSince, "%s is not the git directory move-out made (gone, replaced or a link); nothing to bring back safely", u.GitDir)
		return u, nil
	}
	if !sameDevice(u.GitDir, wt) {
		refuse(RefuseOtherFilesystem, "%s is on another filesystem than %s; it cannot come back in one step", u.GitDir, wt)
	}
	refusals, err := change.Busy(ctx, g, o.Env, wt)
	if err != nil {
		return UndoPlan{}, err
	}
	u.Refusals = append(u.Refusals, refusals...)
	return u, nil
}

// MoveIn carries out u: the .git file goes into this change's backup
// directory and the git directory comes back as .git; git must then see what
// it saw before.
func MoveIn(ctx context.Context, g *gitx.Runner, u UndoPlan, o Options, store journal.Store) (*journal.Entry, error) {
	if len(u.Refusals) > 0 {
		return nil, errors.New("move-in: the plan has refusals")
	}
	dotgit := filepath.Join(u.Worktree, ".git")
	e, err := store.Begin("move-in", u.RepoID, u.Worktree, func(id string) map[string]string {
		return map[string]string{"undoes": u.Undoes, "target": u.GitDir, "backup": filepath.Join(o.Dirs.Backups(), id, "gitfile"),
			"gitfileID": u.out.Data["gitfileID"], "gitfile": u.out.Data["gitfile"], "gitdirID": u.out.Data["copyID"]}
	})
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*journal.Entry, error) {
		if err := rollBackMoveIn(e, store); err != nil {
			return e, fmt.Errorf("%w; rolling back also failed: %v (journal %s)", cause, err, e.ID)
		}
		return e, cause
	}
	quiet := o.Env
	quiet.JournalDir = ""
	if refusals, err := change.Busy(ctx, g, quiet, u.Worktree); err != nil {
		return fail(err)
	} else if len(refusals) > 0 {
		return fail(fmt.Errorf("the repository became busy: %s", refusals[0].Message))
	}
	before, err := look(ctx, g, u.Worktree)
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(e.Data["backup"]), 0o700); err != nil {
		return fail(err)
	}
	if err := rename(dotgit, e.Data["backup"]); err != nil {
		return fail(err)
	}
	stepHook("gitfile-moved")
	if err := store.Advance(e, journal.GitfileMoved); err != nil {
		return fail(err)
	}
	if err := rename(u.GitDir, dotgit); err != nil {
		return fail(err)
	}
	stepHook("gitdir-returned")
	if err := store.Advance(e, journal.GitDirReturned); err != nil {
		return fail(err)
	}
	if err := uses(ctx, g, u.Worktree, dotgit); err != nil {
		return fail(err)
	}
	after, err := look(ctx, g, u.Worktree)
	if err != nil {
		return fail(err)
	}
	if d := before.diff(after); d != "" {
		return fail(fmt.Errorf("git sees the repository differently after moving it back: %s", d))
	}
	return e, finishMoveIn(e, store)
}

// RecoverMoveIn finishes an interrupted move-in whose git directory is back
// (by inode) and works, and rolls back any other.
func RecoverMoveIn(ctx context.Context, g *gitx.Runner, e *journal.Entry, store journal.Store) (string, error) {
	if ctx.Err() != nil {
		return "", fmt.Errorf("stopped (%v); run 'gitbrella undo --apply' again", ctx.Err())
	}
	dotgit := filepath.Join(e.Worktree, ".git")
	if state.FileID(dotgit) == e.Data["gitdirID"] && uses(ctx, g, e.Worktree, dotgit) == nil {
		return "finished", finishMoveIn(e, store)
	}
	if err := rollBackMoveIn(e, store); err != nil {
		return "", err
	}
	return "rolled back", nil
}

func finishMoveIn(e *journal.Entry, store journal.Store) error {
	out, err := store.Get(e.Data["undoes"])
	if err != nil {
		return err
	}
	out.UndoneBy = e.ID
	if err := store.Save(out); err != nil {
		return err
	}
	return store.Advance(e, journal.Done)
}

// rollBackMoveIn puts the git directory back where move-out had it and the
// .git file back in the worktree, moving only what it recognizes (by inode,
// and the .git file also by its bytes).
func rollBackMoveIn(e *journal.Entry, store journal.Store) error {
	if !filepath.IsAbs(e.Data["backup"]) || !filepath.IsAbs(e.Worktree) {
		return fmt.Errorf("journal %s has no usable backup path; leaving everything as it is", e.ID)
	}
	dotgit := filepath.Join(e.Worktree, ".git")
	if inCloud(dotgit) {
		return fmt.Errorf("%s exists only in the cloud; download it (Finder: Download Now) and run 'gitbrella undo --apply' again", dotgit)
	}
	if state.FileID(dotgit) == e.Data["gitdirID"] && !exists(e.Data["target"]) {
		if err := rename(dotgit, e.Data["target"]); err != nil {
			return err
		}
	}
	if !exists(dotgit) && ourGitfile(e.Data["backup"], e.Data) {
		if err := rename(e.Data["backup"], dotgit); err != nil {
			return fmt.Errorf("restoring the .git file from %s: %w", e.Data["backup"], err)
		}
	}
	if !ourGitfile(dotgit, e.Data) {
		return fmt.Errorf("%s is not the .git file move-out wrote, and the git directory is at %s; nothing was overwritten: put the .git you want in place and run 'gitbrella undo --apply' again", dotgit, e.Data["target"])
	}
	return store.Advance(e, journal.RolledBack)
}
