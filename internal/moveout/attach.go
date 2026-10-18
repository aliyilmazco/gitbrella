package moveout

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/gitfile"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/state"
	"github.com/aliyilmazco/gitbrella/internal/treecopy"
)

// More refusal codes, for the other devices.
const (
	RefuseNotMoved         = "NOT_MOVED"
	RefuseNeedSource       = "NEEDS_SOURCE"
	RefuseUnknownCopy      = "PREPARED_COPY_UNKNOWN"
	RefuseUnexpectedTarget = "TARGET_UNEXPECTED"
)

// Prepare copies this device's git directory to where the .git file will
// point, without switching: run it on every other device before one device
// moves out. When the .git file arrives from that device, git here finds the
// copy. p is the move-out plan of this device and must have no refusals.
func Prepare(ctx context.Context, p Plan, o Options, store journal.Store) (*journal.Entry, error) {
	if len(p.Refusals) > 0 || p.Already {
		return nil, errors.New("prepare: the plan has refusals")
	}
	dotgit := filepath.Join(p.Repo.Worktree, ".git")
	e, err := store.Begin("prepare", p.RepoID, p.Repo.Worktree, func(id string) map[string]string {
		return map[string]string{"gitDir": dotgit, "target": p.Target, "head": p.Head,
			"partial": p.Target + ".partial-" + id, "backup": filepath.Join(o.Dirs.Backups(), id, "unused")}
	})
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*journal.Entry, error) {
		if err := setAside(e, store); err != nil {
			return e, fmt.Errorf("%w; moving the copy aside also failed: %v", cause, err)
		}
		return e, cause
	}
	if err := os.MkdirAll(filepath.Dir(p.Target), 0o700); err != nil {
		return fail(err)
	}
	if err := treecopy.Clone(ctx, dotgit, e.Data["partial"], o.GOOS); err != nil {
		return fail(err)
	}
	e.Data["copyID"] = state.FileID(e.Data["partial"])
	if err := store.Save(e); err != nil {
		return fail(err)
	}
	if err := same(dotgit, e.Data["partial"]); err != nil {
		return fail(err)
	}
	if err := rename(e.Data["partial"], p.Target); err != nil {
		return fail(err)
	}
	if err := store.Advance(e, journal.Copied); err != nil {
		return fail(err)
	}
	return e, store.Advance(e, journal.Done)
}

// AttachPlan is what attach would do on this device.
type AttachPlan struct {
	Worktree string    `json:"worktree"`
	RepoID   string    `json:"repoId"`
	Target   string    `json:"target"`
	Source   string    `json:"source"` // "prepared", or the remote URL to clone
	Branch   string    `json:"branch,omitempty"`
	Refusals []Refusal `json:"refusals"`
}

// PlanAttach says how this device would get the git directory its .git file
// points at, after another device moved the repository out: the copy Prepare
// made here, or a clone of from (a remote URL). The .git file arrives through
// the sync client, so it is not trusted: it must point into this device's
// git directories, and an existing directory there is used only if Prepare
// made it for this repository. branch picks the branch HEAD points at ("":
// the remote's default).
func PlanAttach(ctx context.Context, g *gitx.Runner, o Options, from, branch string, store journal.Store) (AttachPlan, error) {
	a := AttachPlan{Branch: branch, Refusals: []Refusal{}}
	refuse := func(code, format string, args ...any) (AttachPlan, error) {
		a.Refusals = append(a.Refusals, Refusal{Code: code, Message: fmt.Sprintf(format, args...)})
		return a, nil
	}
	wt, ok := FindWorktree(o.Path)
	if !ok {
		return refuse(RefuseNotMoved, "no .git here or above %s", o.Path)
	}
	wt = state.OnDisk(wt)
	a.Worktree, a.RepoID = wt, state.RepoID(wt, o.Home)
	target, isFile, err := gitfile.Read(wt)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return refuse(RefuseNotPlain, "%v", err)
	}
	if !isFile {
		return refuse(RefuseNotMoved, "%s/.git is not a .git file; this repository has not been moved out (use move-out, or --prepare before another device moves it)", wt)
	}
	a.Target = target
	if filepath.Dir(state.OnDisk(target)) != state.OnDisk(o.Dirs.GitDirs) {
		return refuse(RefuseUnexpectedTarget, "the .git file points at %s, not into this device's %s: the folder sits at another place under home here than on the device that moved it, or the .git file was not written by gitbrella", target, o.Dirs.GitDirs)
	}
	if exists(target) {
		prep, ok, err := store.LastDone(a.RepoID, "prepare")
		if err != nil {
			return a, err
		}
		if !ok || prep.Data["target"] != target {
			return refuse(RefuseUnknownCopy, "%s exists but was not prepared for this repository on this device; move it away, then attach with --from <remote URL>", target)
		}
		if err := uses(ctx, g, wt, target); err != nil {
			return refuse(RefuseUnknownCopy, "%s was prepared here but git cannot use it: %v", target, err)
		}
		a.Source = "prepared"
		return a, nil
	}
	if from == "" {
		return refuse(RefuseNeedSource, "this device has no copy of the git directory at %s; give --from <remote URL> to clone it (or prepare devices before the move next time)", target)
	}
	a.Source = from
	return a, nil
}

// Attach carries out a: with a prepared copy it only records that; otherwise
// it clones without a checkout, moves the clone's git directory to the target
// and fills the index from HEAD. The working tree is never touched.
func Attach(ctx context.Context, g *gitx.Runner, a AttachPlan, o Options, store journal.Store) (*journal.Entry, error) {
	if len(a.Refusals) > 0 {
		return nil, errors.New("attach: the plan has refusals")
	}
	target := a.Target
	e, err := store.Begin("attach", a.RepoID, a.Worktree, func(id string) map[string]string {
		return map[string]string{"target": target, "source": a.Source, "branch": a.Branch,
			"partial": target + ".clone-" + id, "backup": filepath.Join(o.Dirs.Backups(), id, "unused")}
	})
	if err != nil {
		return nil, err
	}
	if a.Source == "prepared" {
		return e, store.Advance(e, journal.Done)
	}
	fail := func(cause error) (*journal.Entry, error) {
		if err := setAside(e, store); err != nil {
			return e, fmt.Errorf("%w; moving the clone aside also failed: %v", cause, err)
		}
		return e, cause
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fail(err)
	}
	long := *g
	long.Timeout = 30 * time.Minute // a clone over the network
	args := []string{"clone", "--quiet", "--no-checkout"}
	if a.Branch != "" {
		args = append(args, "--branch", a.Branch)
	}
	var env []string
	if os.Getenv("GIT_SSH_COMMAND") == "" && os.Getenv("GIT_SSH") == "" {
		env = []string{"GIT_SSH_COMMAND=ssh -o BatchMode=yes"} // never wait for a password or a host key
	}
	res, err := long.Run(ctx, filepath.Dir(target), env, append(args, "--", a.Source, e.Data["partial"])...)
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git clone %s: %s", a.Source, strings.TrimSpace(res.Stderr))
	}
	if err != nil {
		return fail(err)
	}
	e.Data["copyID"] = state.FileID(filepath.Join(e.Data["partial"], ".git"))
	if err := store.Save(e); err != nil {
		return fail(err)
	}
	if err := rename(filepath.Join(e.Data["partial"], ".git"), target); err != nil {
		return fail(err)
	}
	if err := os.Remove(e.Data["partial"]); err != nil { // empty: --no-checkout
		return fail(err)
	}
	if err := store.Advance(e, journal.Copied); err != nil {
		return fail(err)
	}
	if err := finishAttach(ctx, g, a.Worktree, target); err != nil {
		return fail(err)
	}
	return e, store.Advance(e, journal.Done)
}

// finishAttach fills the index from HEAD. The working tree is not touched:
// the sync client already brought the files, and git status then shows how
// they differ from the commit.
func finishAttach(ctx context.Context, g *gitx.Runner, wt, target string) error {
	if err := uses(ctx, g, wt, target); err != nil {
		return err
	}
	res, err := g.Run(ctx, wt, nil, "reset", "--quiet", "--mixed")
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git reset --mixed: %s", strings.TrimSpace(res.Stderr))
	}
	return err
}

// RecoverAttach finishes or sets aside an interrupted prepare or attach. An
// attach to a prepared copy made nothing, so it only closes its entry.
func RecoverAttach(ctx context.Context, g *gitx.Runner, e *journal.Entry, store journal.Store) (string, error) {
	switch {
	case e.Kind == "attach" && e.Data["source"] == "prepared":
		return "finished (nothing to undo)", store.Advance(e, journal.Done)
	case e.Kind == "attach" && e.Phase == journal.Copied && finishAttach(ctx, g, e.Worktree, e.Data["target"]) == nil:
		return "finished", store.Advance(e, journal.Done)
	case e.Kind == "prepare" && e.Phase == journal.Copied:
		return "finished", store.Advance(e, journal.Done)
	}
	if err := setAside(e, store); err != nil {
		return "", err
	}
	return "rolled back; set aside in " + filepath.Dir(e.Data["backup"]), nil
}

// setAside moves what a prepare or attach made into its backup directory:
// its partial copy (a name only it uses) and the target if its inode is the
// one it made. Nothing else.
func setAside(e *journal.Entry, store journal.Store) error {
	if !filepath.IsAbs(e.Data["backup"]) {
		return fmt.Errorf("journal %s has no usable backup path; leaving everything as it is", e.ID)
	}
	aside := filepath.Dir(e.Data["backup"])
	if err := os.MkdirAll(aside, 0o700); err != nil {
		return err
	}
	if p := e.Data["partial"]; p != "" && exists(p) {
		if err := rename(p, filepath.Join(aside, "abandoned-partial")); err != nil {
			return err
		}
	}
	if t := e.Data["target"]; e.Data["copyID"] != "" && state.FileID(t) == e.Data["copyID"] {
		if err := rename(t, filepath.Join(aside, "abandoned-copy")); err != nil {
			return err
		}
	}
	return store.Advance(e, journal.RolledBack)
}
