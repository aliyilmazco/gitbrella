package moveout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/gitfile"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/shq"
	"github.com/aliyilmazco/gitbrella/internal/state"
	"github.com/aliyilmazco/gitbrella/internal/treecopy"
)

// stepHook is called at named points between changes on disk and their
// saves: tests stop or interfere there.
var stepHook = func(step string) {}

// Apply carries out p as one journaled change:
//  1. copy the git directory under a name only this change uses, and prove
//     the copy identical; it becomes the target;
//  2. ask again whether anything works on the repository, and take HEAD,
//     status, config and the hooks directory as git sees them now;
//  3. move the original into the backup, so nothing reaches it by path any
//     more, and prove the backup and the target still identical (a write that
//     slipped in before the move is caught here, and the move is undone);
//  4. write the .git file, and check that git sees what it saw in step 2.
//
// The entry records the device and inode of the original, the copy and the
// .git file, so a roll-back touches only what this change made. A failure
// rolls back; after the switch only if the target holds nothing the backup
// lacks. p must have no refusals; the caller holds the repository lock.
func Apply(ctx context.Context, g *gitx.Runner, p Plan, o Options, store journal.Store) (*journal.Entry, error) {
	if len(p.Refusals) > 0 || p.Already {
		return nil, errors.New("move-out: the plan has refusals")
	}
	wt := p.Repo.Worktree
	dotgit := filepath.Join(wt, ".git")
	e, err := store.Begin("move-out", p.RepoID, wt, func(id string) map[string]string {
		return map[string]string{"gitDir": dotgit, "target": p.Target, "partial": p.Target + ".partial-" + id,
			"backup": filepath.Join(o.Dirs.Backups(), id, "dotgit"), "gitfile": p.Gitfile, "head": p.Head,
			"originalID": state.FileID(dotgit)}
	})
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*journal.Entry, error) {
		if _, err := settle(ctx, g, e, store, false); err != nil {
			return e, fmt.Errorf("%w; %v", cause, err)
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
	stepHook("copied")
	if err := store.Advance(e, journal.Copied); err != nil {
		return fail(err)
	}

	quiet := o.Env
	quiet.JournalDir = "" // this change's own entry is unfinished, and expected to be
	if refusals, err := change.Busy(ctx, g, quiet, wt); err != nil {
		return fail(err)
	} else if len(refusals) > 0 {
		return fail(fmt.Errorf("the repository became busy: %s", refusals[0].Message))
	}
	before, err := look(ctx, g, wt)
	if err != nil {
		return fail(err)
	}

	stepHook("compared")
	if err := os.MkdirAll(filepath.Dir(e.Data["backup"]), 0o700); err != nil {
		return fail(err)
	}
	if err := rename(dotgit, e.Data["backup"]); err != nil {
		return fail(err)
	}
	stepHook("original-moved")
	if err := store.Advance(e, journal.OriginalMoved); err != nil {
		return fail(err)
	}
	if err := same(e.Data["backup"], p.Target); err != nil {
		return fail(fmt.Errorf("the git directory changed while it was copied: %w", err))
	}
	if err := gitfile.Write(wt, p.Gitfile); err != nil {
		return fail(err)
	}
	e.Data["gitfileID"] = state.FileID(dotgit)
	stepHook("gitfile-written")
	if err := store.Advance(e, journal.GitfileWritten); err != nil {
		return fail(err)
	}
	if err := uses(ctx, g, wt, p.Target); err != nil {
		return fail(err)
	}
	after, err := look(ctx, g, wt)
	if err != nil {
		return fail(err)
	}
	if d := before.diff(after); d != "" {
		return fail(fmt.Errorf("git sees the repository differently after the move: %s", d))
	}
	return e, store.Advance(e, journal.Done)
}

// Recover finishes or rolls back an interrupted move-out, judging by what is
// on disk: when git follows this change's .git file to the copy, the move is
// finished; otherwise it is rolled back if that loses nothing.
func Recover(ctx context.Context, g *gitx.Runner, e *journal.Entry, store journal.Store) (string, error) {
	if e.Kind != "move-out" {
		return "", fmt.Errorf("journal %s: cannot recover a %s here", e.ID, e.Kind)
	}
	return settle(ctx, g, e, store, true)
}

// settle ends a move-out that stopped half way. A cancelled context stops it
// with nothing done. Once the .git file is this change's (by inode, or by
// content in the one window where it can be written but not yet recorded),
// the copy is the git directory in use: finishIfWorks finishes when git uses
// it, and a roll-back happens only when the copy holds nothing the backup
// lacks. Before that, it rolls back.
func settle(ctx context.Context, g *gitx.Runner, e *journal.Entry, store journal.Store, finishIfWorks bool) (string, error) {
	if ctx.Err() != nil {
		return "", fmt.Errorf("stopped (%v); run 'gitbrella move-out --apply' again to finish or roll it back", ctx.Err())
	}
	dotgit := filepath.Join(e.Worktree, ".git")
	if inCloud(dotgit) {
		return "", fmt.Errorf("%s exists only in the cloud; download it (Finder: Download Now) and run 'gitbrella move-out --apply' again", dotgit)
	}
	if e.Data["gitfileID"] == "" && e.Phase == journal.OriginalMoved && backupIsOurs(e) {
		if b, err := os.ReadFile(dotgit); err == nil && string(b) == e.Data["gitfile"] {
			e.Data["gitfileID"] = state.FileID(dotgit) // written just before the crash: under the lock, ours
			if err := store.Save(e); err != nil {
				return "", err
			}
		}
	}
	if gitfileIsOurs(e) {
		if finishIfWorks && uses(ctx, g, e.Worktree, e.Data["target"]) == nil {
			return "finished", store.Advance(e, journal.Done)
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("stopped (%v); run 'gitbrella move-out --apply' again", ctx.Err())
		}
		if diffs, err := treecopy.Compare(e.Data["backup"], e.Data["target"]); err != nil || len(diffs) > 0 {
			return "", fmt.Errorf("git already used the moved directory %s, which now differs from the original kept at %s; nothing was rolled back. Compare them (git --git-dir=%s log --all), keep the one you want as the target and run 'gitbrella move-out --apply' again", e.Data["target"], e.Data["backup"], shq.Quote(e.Data["target"]))
		}
	}
	if err := rollBack(e, store); err != nil {
		return "", err
	}
	return "rolled back; set aside in " + filepath.Dir(e.Data["backup"]), nil
}

// rollBack puts the original back and sets aside what this change made, in
// the change's own backup directory. It moves only what it recorded: the
// copy under its own name or inode, the original by inode, the .git file by
// inode. Nothing is deleted.
func rollBack(e *journal.Entry, store journal.Store) error {
	aside := filepath.Dir(e.Data["backup"])
	if !filepath.IsAbs(e.Data["backup"]) || !filepath.IsAbs(e.Worktree) {
		return fmt.Errorf("journal %s has no usable backup path; leaving everything as it is", e.ID)
	}
	if err := os.MkdirAll(aside, 0o700); err != nil {
		return err
	}
	if p := e.Data["partial"]; p != "" && exists(p) { // a name only this change uses
		if err := rename(p, filepath.Join(aside, "abandoned-partial")); err != nil {
			return err
		}
	}
	if t := e.Data["target"]; e.Data["copyID"] != "" && state.FileID(t) == e.Data["copyID"] {
		if err := rename(t, filepath.Join(aside, "abandoned-copy")); err != nil {
			return err
		}
	}
	if backupIsOurs(e) {
		dotgit := filepath.Join(e.Worktree, ".git")
		if gitfileIsOurs(e) {
			if err := rename(dotgit, filepath.Join(aside, "gitfile")); err != nil {
				return err
			}
		}
		if exists(dotgit) {
			return fmt.Errorf("the original git directory is kept at %s, and %s exists too (made after the move stopped); nothing was overwritten. Compare them, move the .git you do not want out of the folder, and run 'gitbrella move-out --apply' again", e.Data["backup"], dotgit)
		}
		if err := rename(e.Data["backup"], dotgit); err != nil {
			return fmt.Errorf("restoring %s from %s: %w", dotgit, e.Data["backup"], err)
		}
	}
	return store.Advance(e, journal.RolledBack)
}

func backupIsOurs(e *journal.Entry) bool {
	return e.Data["originalID"] != "" && state.FileID(e.Data["backup"]) == e.Data["originalID"]
}

// inCloud reports whether p exists only in the cloud: reading it would
// download it, which this tool never does. Tests replace it.
var inCloud = func(p string) bool {
	dl, _ := scan.DefaultFlagReader().IsDataless(p)
	return dl
}

func gitfileIsOurs(e *journal.Entry) bool {
	return ourGitfile(filepath.Join(e.Worktree, ".git"), e.Data)
}

// ourGitfile reports whether p is the .git file a move-out wrote, by its
// inode (gitfileID) and its bytes (gitfile). The inode alone is not enough:
// a file rewritten in place keeps it, and Linux gives a freed inode to the
// next new file at once.
func ourGitfile(p string, data map[string]string) bool {
	if data["gitfileID"] == "" || state.FileID(p) != data["gitfileID"] {
		return false
	}
	b, err := os.ReadFile(p)
	return err == nil && string(b) == data["gitfile"]
}

// uses checks that git follows worktree/.git to target.
func uses(ctx context.Context, g *gitx.Runner, worktree, target string) error {
	res, err := g.Run(ctx, worktree, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	got, want := strings.TrimSpace(res.Stdout), target
	if real, err := filepath.EvalSymlinks(got); err == nil {
		got = real
	}
	if real, err := filepath.EvalSymlinks(want); err == nil {
		want = real
	}
	if res.ExitCode != 0 || got != want {
		return fmt.Errorf("git uses %q, not %s: %s", got, want, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// view is how git sees a repository: what must not change with a move.
type view struct{ head, status, config, hooks string }

// look takes git's view of the repository at worktree.
func look(ctx context.Context, g *gitx.Runner, worktree string) (view, error) {
	var v view
	var err error
	if v.head, err = head(ctx, g, worktree); err != nil {
		return v, err
	}
	if v.status, err = status(ctx, g, worktree); err != nil {
		return v, err
	}
	res, err := g.Run(ctx, worktree, nil, "config", "--list", "--includes")
	if err != nil {
		return v, err
	}
	v.config = res.Stdout
	// The hooks directory relative to the git directory: the same wherever
	// the git directory is, unless a path relative to .git stops resolving.
	res, err = g.Run(ctx, worktree, nil, "rev-parse", "--path-format=absolute", "--absolute-git-dir", "--git-path", "hooks")
	if err != nil {
		return v, err
	}
	if lines := strings.Split(strings.TrimSpace(res.Stdout), "\n"); res.ExitCode == 0 && len(lines) == 2 {
		if rel, err := filepath.Rel(lines[0], lines[1]); err == nil {
			v.hooks = rel
		}
	}
	return v, nil
}

// diff names what differs between two views, "" when nothing does.
func (a view) diff(b view) string {
	var out []string
	if a.head != b.head {
		out = append(out, fmt.Sprintf("HEAD %q became %q", a.head, b.head))
	}
	if a.status != b.status {
		out = append(out, "git status")
	}
	if a.config != b.config {
		out = append(out, "the configuration (a path relative to .git, such as core.hooksPath or include.path, no longer resolves)")
	}
	if a.hooks != b.hooks {
		out = append(out, "the hooks directory")
	}
	return strings.Join(out, ", ")
}

func same(a, b string) error {
	diffs, err := treecopy.Compare(a, b)
	if err != nil {
		return err
	}
	if len(diffs) > 0 {
		return fmt.Errorf("the copy differs from %s: %s", a, strings.Join(diffs, "; "))
	}
	return nil
}
