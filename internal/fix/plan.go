package fix

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// Options configures fix.
type Options struct {
	change.Env
	Path   string
	Dirs   state.Dirs
	Remote string // where lost objects are fetched from; "" picks one
}

// Plan is what fix would do, what it leaves to the user, and what stops it.
type Plan struct {
	Repo     check.RepoInfo   `json:"repo"`
	RepoID   string           `json:"repoId"`
	Actions  []Action         `json:"actions"`
	Manual   []Manual         `json:"manual"`
	Refusals []change.Refusal `json:"refusals"`
	before   map[string]bool  // damage findings ("code path") before the fix
	targets  map[string]bool  // the findings the actions repair
	r        repo.Repo
}

// MakePlan checks the repository at o.Path and says what fix would do. It
// writes nothing. While files of the repository exist only in the cloud the
// check cannot read its objects and may not see all the damage, so the plan
// is then to download them, and nothing else.
func MakePlan(ctx context.Context, g *gitx.Runner, o Options) (Plan, error) {
	r, err := repo.Locate(ctx, g, o.Path)
	if err != nil {
		return Plan{}, err
	}
	rep, err := change.RunCheck(ctx, g, o.Env, r.Worktree)
	if err != nil {
		return Plan{}, err
	}
	return PlanFor(ctx, g, o, r, rep)
}

// PlanFor says what fix would do about rep, the check of r just run. It
// writes nothing.
func PlanFor(ctx context.Context, g *gitx.Runner, o Options, r repo.Repo, rep check.Report) (Plan, error) {
	p := Plan{Repo: rep.Repo, RepoID: state.RepoID(r.Worktree, o.Home), Actions: []Action{}, Manual: []Manual{},
		Refusals: []change.Refusal{}, before: map[string]bool{}, targets: map[string]bool{}, r: r}
	var clouds []check.Finding // files only in the cloud: fix downloads them first
	for _, f := range rep.Findings {
		if f.Code == check.CodeDataless || f.Code == check.CodeAlternateDataless {
			clouds = append(clouds, f)
		} else if change.Blocking[f.Code] {
			p.Refusals = append(p.Refusals, change.Refusal{Code: change.RefuseBusy, Message: f.Code + ": " + f.Message})
		}
		if f.Severity == check.Damage {
			p.before[key(f.Code, f.Path)] = true
		}
	}
	if len(p.Refusals) > 0 {
		for _, f := range clouds {
			p.Refusals = append(p.Refusals, change.Refusal{Code: change.RefuseBusy, Message: f.Code + ": " + f.Message})
		}
		return p, nil
	}
	if len(clouds) > 0 {
		return downloadPlan(p, r, rep, clouds), nil
	}
	if !sameDevice(r.CommonDir, o.Dirs.Backups()) || !sameDevice(r.Worktree, o.Dirs.Backups()) {
		p.Refusals = append(p.Refusals, change.Refusal{Code: "OTHER_FILESYSTEM",
			Message: fmt.Sprintf("the backup directory %s is on another filesystem than the repository; what fix moves aside could not be moved in one step", o.Dirs.Backups())})
		return p, nil
	}
	res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--type=bool", "--get", "core.sparseCheckout")
	if err != nil {
		return Plan{}, err
	}
	sparse := strings.TrimSpace(res.Stdout) == "true"
	if p.Actions, p.Manual, err = actionsFor(ctx, g, r, rep, sparse, o.Flags, o.Remote); err != nil {
		return Plan{}, err
	}
	if p.Actions == nil {
		p.Actions = []Action{}
	}
	if p.Manual == nil {
		p.Manual = []Manual{}
	}
	for _, f := range rep.Findings {
		if f.Severity != check.Damage {
			continue
		}
		repaired := true
		for _, m := range p.Manual {
			if m.Code == f.Code && m.Path == f.Path {
				repaired = false
			}
		}
		if repaired {
			p.targets[key(f.Code, f.Path)] = true
		}
	}
	return p, nil
}

// downloadPlan downloads the cloud-only files the findings clouds point at:
// those of the git directory, or an alternate object store (a path in it).
// The other damage waits for the next run, when the check sees it all.
func downloadPlan(p Plan, r repo.Repo, rep check.Report, clouds []check.Finding) Plan {
	for _, f := range clouds {
		root, why := r.CommonDir, "files of the git directory exist only in the cloud; the check cannot read the objects until they are on the disk"
		if f.Code == check.CodeAlternateDataless {
			root, why = f.Path, "an alternate object store holds files that exist only in the cloud; the check cannot read the objects until they are on the disk"
			if filepath.Base(root) == "alternates" { // the list of stores itself
				root = filepath.Dir(filepath.Dir(root))
			}
		}
		p.Actions = append(p.Actions, Action{Kind: Download, Path: root, Code: f.Code, Reason: why})
		if f.Severity == check.Damage {
			p.targets[key(f.Code, f.Path)] = true
		}
	}
	for _, f := range rep.Findings {
		if f.Severity == check.Damage && !p.targets[key(f.Code, f.Path)] {
			p.Manual = append(p.Manual, Manual{Code: f.Code, Path: f.Path,
				Do: "after the download, run gitbrella fix again: the check then reads the objects and sees all the damage"})
		}
	}
	return p
}

func key(code, path string) string { return code + " " + path }

// sameDevice reports whether a and b (or their nearest existing parents) are
// on one filesystem.
func sameDevice(a, b string) bool {
	for !exists(b) && filepath.Dir(b) != b {
		b = filepath.Dir(b)
	}
	x, y := state.FileID(a), state.FileID(b)
	return x != "" && y != "" && strings.SplitN(x, ":", 2)[0] == strings.SplitN(y, ":", 2)[0]
}

// Repaired reports whether f is damage this plan repairs.
func (p Plan) Repaired(f check.Finding) bool { return p.targets[key(f.Code, f.Path)] }

// Annotate marks each damage finding of rep with what fix does about it:
// "fix" repairs it, "manual" lists it with what to run. While the plan is
// refused nothing is marked: fix does nothing then.
func (p Plan) Annotate(rep *check.Report) {
	if len(p.Refusals) > 0 {
		return
	}
	for i, f := range rep.Findings {
		switch {
		case f.Severity != check.Damage:
		case p.Repaired(f):
			rep.Findings[i].Repair = "fix"
		default:
			rep.Findings[i].Repair = "manual"
		}
	}
}
