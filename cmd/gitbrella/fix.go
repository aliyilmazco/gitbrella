package main

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/fix"
	"github.com/aliyilmazco/gitbrella/internal/journal"
)

func fixOptions(env *changeEnv, remote string, deep bool) fix.Options {
	o := fix.Options{Env: env.opts.Env, Path: env.opts.Path, Dirs: env.opts.Dirs, Remote: remote}
	o.Deep = deep
	return o
}

func runFix(args []string, stdout, stderr io.Writer) int {
	c := newChangeFlags("fix", stderr).withProvider()
	watchFor := c.fs.Duration("watch", 30*time.Second, "after the fix, check again after this long whether the sync client brought the damage back (0: do not wait)")
	remote := c.fs.String("remote", "", "the remote to fetch lost objects from (default: the branch's remote, else origin, else the only one)")
	deep := c.fs.Bool("deep", false, "also run git fsck, before and after: damaged objects it finds are fetched again from the remote")
	if !c.parse(args) {
		return 2
	}
	env, done, err := start("fix", c, stderr)
	if err != nil {
		return changeFail(stdout, stderr, *c.asJSON, err)
	}
	defer done()
	res := &env.res
	res.Mode = "fix"
	if res.Result == "refused" || stoppedAfterRecovery(res) {
		return printChange(stdout, *c.asJSON, *res)
	}
	p, err := fix.MakePlan(env.ctx, env.g, fixOptions(env, *remote, *deep))
	if err != nil {
		return failedChange(stdout, stderr, *c.asJSON, res, err)
	}
	res.Plan, res.Refusals, res.Manual = p, p.Refusals, p.Manual
	switch {
	case len(p.Refusals) > 0:
		res.Result = "refused"
	case len(p.Actions) == 0:
		res.Result = "already"
		if len(p.Manual) == 0 {
			res.Next = append(res.Next, "No damage fix can repair.")
		} else {
			res.Next = append(res.Next, "Nothing fix repairs on its own; the damage listed as MANUAL needs you.")
		}
	case !*c.apply:
		res.Result = "planned"
		res.Next = append(res.Next, "Nothing was changed. Run again with --apply to repair; 'gitbrella undo --apply' reverses it.")
	default:
		e, err := fix.Apply(env.ctx, env.g, p, fixOptions(env, *remote, *deep), env.store)
		if err != nil {
			if e != nil {
				res.Journal = e.ID
			}
			return failedChange(stdout, stderr, *c.asJSON, res, err) // rolled back: see the error
		}
		res.Result, res.Journal = "done", e.ID
		res.Backup = env.opts.Dirs.Backups() + "/" + e.ID
		moved := false
		for _, a := range p.Actions {
			switch a.Kind {
			case fix.RescueRef:
				res.Next = append(res.Next, "Commits only a conflict copy pointed at are kept as "+a.Ref+"; merge or delete it (git branch -D is not needed: git update-ref -d "+a.Ref+").")
			case fix.Download:
				res.Next = append(res.Next, "The files of "+a.Path+" that existed only in the cloud are on the disk now; run 'gitbrella fix"+c.providerArg()+"' again: the check reads the objects now and may find more.")
				continue
			case fix.Refetch:
				res.Next = append(res.Next, "The objects fetched again from "+a.Value+" stay in the object store; undo leaves them.")
				continue
			}
			moved = true
		}
		if moved {
			res.Next = append(res.Next, "Everything moved aside is kept under "+res.Backup+"; 'gitbrella undo --apply' puts it back.")
		}
		res.Warnings = append(res.Warnings, watchFix(env.ctx, env, p, *watchFor)...)
		if len(res.Warnings) == 0 && p.Repo.Provider != "none" {
			res.Next = append(res.Next, "The lasting cure is moving .git out of the synced folder: 'gitbrella move-out'.")
		}
	}
	return printChange(stdout, *c.asJSON, *res)
}

// watchFix waits d and runs the check again: damage the fix repaired that
// is back was brought back by the sync client.
func watchFix(ctx context.Context, env *changeEnv, p fix.Plan, d time.Duration) []string {
	if d <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return []string{"watch interrupted; run 'gitbrella check' later"}
	case <-time.After(d):
	}
	rep, err := change.RunCheck(ctx, env.g, env.opts.Env, p.Repo.Worktree)
	if err != nil {
		return []string{"the check after the fix could not run: " + err.Error()}
	}
	var back []string
	for _, f := range rep.Findings {
		if f.Severity == check.Damage && p.Repaired(f) {
			back = append(back, f.Code+" "+f.Path)
		}
	}
	if len(back) == 0 {
		return nil
	}
	return []string{"the sync client brought back what fix repaired (" + strings.Join(back, ", ") + "); move .git out of the synced folder: 'gitbrella move-out'"}
}

// undoFix plans and, with apply, carries out the undo of the fix entry last.
func undoFix(env *changeEnv, apply bool, last journal.Entry) (int, error) {
	res := &env.res
	res.Mode = "unfix"
	u, err := fix.PlanUndo(env.ctx, env.g, fixOptions(env, "", false), last)
	if err != nil {
		return 0, err
	}
	res.Plan, res.Refusals = u, u.Refusals
	switch {
	case len(u.Refusals) > 0:
		res.Result = "refused"
	case !apply:
		res.Result = "planned"
		res.Next = append(res.Next, "Nothing was changed. Run again with --apply to put back what the fix moved aside and remove what it added.")
		for _, k := range u.Kept {
			res.Next = append(res.Next, "Would stay as it is, it changed after the fix: "+k)
		}
	default:
		e, kept, err := fix.Undo(env.ctx, env.g, u, env.store)
		if err != nil {
			if e != nil {
				res.Journal = e.ID
			}
			return 0, err
		}
		res.Result, res.Journal = "done", e.ID
		if len(kept) == 0 {
			res.Next = append(res.Next, "The repository is as it was before the fix.")
		}
		for _, k := range kept {
			res.Warnings = append(res.Warnings, "left as it is, it changed after the fix: "+k)
		}
	}
	return res.exitCode(), nil
}
