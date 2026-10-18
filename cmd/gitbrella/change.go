package main

// The commands that change something: move-out (with --prepare and --attach)
// and undo. Without --apply each only says what it would do.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/fix"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/icloud"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/moveout"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/report"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// changeResult is the outcome of a change command, printed as text or JSON.
// Exit status: 0 planned, done or already so; 1 refused, or done with a
// warning; 2 failed (rolled back, or stopped: see error) or could not run.
type changeResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	ToolVersion   string           `json:"toolVersion"`
	Command       string           `json:"command"` // move-out, undo
	Mode          string           `json:"mode"`    // move-out, prepare, attach, move-in
	Result        string           `json:"result"`  // planned, done, already, refused, recovered, failed
	Error         string           `json:"error,omitempty"`
	Worktree      string           `json:"worktree"`
	Recovered     []string         `json:"recovered"`
	Plan          any              `json:"plan"`
	Refusals      []change.Refusal `json:"refusals"`
	Carried       []check.Finding  `json:"carried"`
	Manual        []fix.Manual     `json:"manual"`
	Journal       string           `json:"journal,omitempty"`
	Backup        string           `json:"backup,omitempty"`
	Warnings      []string         `json:"warnings"`
	Next          []string         `json:"next"`
}

func (r changeResult) exitCode() int {
	if r.Result == "failed" {
		return 2
	}
	if r.Result == "refused" || len(r.Warnings) > 0 || len(r.Manual) > 0 { // fix: damage left to the user
		return 1
	}
	return 0
}

// changeFlags are the flags every change command takes.
type changeFlags struct {
	fs       *flag.FlagSet
	apply    *bool
	asJSON   *bool
	provider *string // nil for the commands that run no check of their own
	paths    []string
}

// withProvider adds --provider, as check takes it: the change sees the same
// damage check reported with it.
func (c *changeFlags) withProvider() *changeFlags {
	c.provider = c.fs.String("provider", "auto", "sync provider, as for check: "+strings.Join(providerNames, ", "))
	return c
}

// providerArg is " --provider NAME" when one was given, for the commands the
// output tells to run next: they must see the same damage.
func (c *changeFlags) providerArg() string {
	if c.provider == nil || *c.provider == "auto" {
		return ""
	}
	return " --provider " + *c.provider
}

func newChangeFlags(name string, stderr io.Writer) *changeFlags {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	setUsage(fs)
	return &changeFlags{fs: fs, apply: fs.Bool("apply", false, "make the change (without it, only say what would happen)"),
		asJSON: fs.Bool("json", false, "print the result as JSON")}
}

// parse accepts flags before or after the path.
func (c *changeFlags) parse(args []string) bool {
	for rest := args; ; {
		if err := c.fs.Parse(rest); err != nil {
			return false
		}
		if c.fs.NArg() == 0 {
			return true
		}
		c.paths = append(c.paths, c.fs.Arg(0))
		rest = c.fs.Args()[1:]
	}
}

// changeEnv is what every change command needs once its flags are parsed.
type changeEnv struct {
	ctx   context.Context
	g     *gitx.Runner
	opts  moveout.Options
	store journal.Store
	res   changeResult
}

// start sets up a change command: no cloud downloads, a usable git, the
// state directories, the repository lock (with --apply) and the recovery of
// changes an earlier run left unfinished. done must be called at the end.
func start(command string, c *changeFlags, stderr io.Writer) (env *changeEnv, done func(), err error) {
	if err := preventMaterialization(); err != nil {
		return nil, nil, fmt.Errorf("could not disable cloud downloads (%v); not running, since reading the repository could download files", err)
	}
	if len(c.paths) > 1 {
		return nil, nil, errors.New("give at most one path")
	}
	if c.provider != nil && !slices.Contains(providerNames, *c.provider) {
		return nil, nil, fmt.Errorf("unknown provider %q", *c.provider)
	}
	path := "."
	if len(c.paths) == 1 {
		path = c.paths[0]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	g, err := gitx.New()
	if err != nil {
		stop()
		return nil, nil, err
	}
	if major, minor, err := g.Version(ctx); err != nil || !gitx.AtLeast(major, minor, 2, 31) {
		stop()
		return nil, nil, fmt.Errorf("git 2.31 or newer is required (%d.%d, %v)", major, minor, err)
	}
	home, err := resolvedHome()
	if err != nil {
		stop()
		return nil, nil, err
	}
	dirs, err := state.Resolve(home, runtime.GOOS, os.Getenv)
	if err != nil {
		stop()
		return nil, nil, err
	}
	env = &changeEnv{ctx: ctx, g: g,
		opts: moveout.Options{Path: path, Probe: icloud.Probe, Dirs: dirs,
			Env: change.Env{Home: home, GOOS: runtime.GOOS, Flags: scan.DefaultFlagReader(), Procs: procs.GitProcesses,
				Version: version, JournalDir: dirs.Journal()}},
		store: journal.Store{Dir: dirs.Journal(), AfterSave: failpoint()},
		res: changeResult{SchemaVersion: 1, ToolVersion: version, Command: command, Recovered: []string{},
			Refusals: []change.Refusal{}, Carried: []check.Finding{}, Manual: []fix.Manual{}, Warnings: []string{}, Next: []string{}},
	}
	if c.provider != nil && *c.provider != "auto" {
		env.opts.Env.Provider = *c.provider
	}
	// Which repository: first an unfinished change of a folder from path up
	// (by inode: another spelling, a renamed folder, or a repository around
	// it cannot hide it), then the folder git finds, as stored on disk.
	abs, err := filepath.Abs(path)
	if err != nil {
		stop()
		return nil, nil, err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	open, wt, err := env.store.UnfinishedAt(abs)
	if err != nil {
		stop()
		return nil, nil, err
	}
	if wt == "" {
		var ok bool
		if wt, ok = worktreeOf(ctx, g, abs); !ok {
			stop()
			return nil, nil, fmt.Errorf("not a git repository: %s", path)
		}
	}
	wt = state.OnDisk(wt)
	env.res.Worktree = wt
	repoID := state.RepoID(wt, home)
	release := func() {}
	if *c.apply {
		if release, err = state.Lock(dirs.Locks(), repoID); err != nil {
			stop()
			if errors.Is(err, state.ErrBusy) {
				env.res.Result = "refused"
				env.res.Refusals = append(env.res.Refusals, change.Refusal{Code: "TOOL_BUSY", Message: err.Error()})
				return env, func() {}, nil
			}
			return nil, nil, err
		}
	}
	done = func() { release(); stop() }
	more, err := env.store.Unfinished(repoID)
	if err != nil {
		done()
		return nil, nil, err
	}
	seen := map[string]bool{}
	for _, e := range open {
		seen[e.ID] = true
	}
	for _, e := range more {
		if !seen[e.ID] {
			open = append(open, e)
		}
	}
	for i := range open {
		e := &open[i]
		if !*c.apply {
			env.res.Result = "refused"
			env.res.Refusals = append(env.res.Refusals, change.Refusal{Code: "UNFINISHED_CHANGE",
				Message: fmt.Sprintf("a %s (journal %s) did not finish; run again with --apply to finish or roll it back first", e.Kind, e.ID)})
			continue
		}
		if e.Worktree != wt { // the folder was renamed since: its paths follow
			e.Worktree = wt
			if err := env.store.Save(e); err != nil {
				done()
				return nil, nil, err
			}
		}
		var did string
		switch e.Kind {
		case "move-out":
			did, err = moveout.Recover(ctx, g, e, env.store)
		case "move-in":
			did, err = moveout.RecoverMoveIn(ctx, g, e, env.store)
		case "prepare", "attach":
			did, err = moveout.RecoverAttach(ctx, g, e, env.store)
		case "fix":
			did, err = fix.Recover(ctx, g, e, fixOptions(env, "", false), env.store)
		case "unfix":
			did, err = fix.RecoverUnfix(ctx, g, e, env.store)
		default:
			err = fmt.Errorf("journal %s: unknown change %q", e.ID, e.Kind)
		}
		if err != nil {
			done()
			return nil, nil, fmt.Errorf("recovering the unfinished %s (journal %s): %w", e.Kind, e.ID, err)
		}
		env.res.Recovered = append(env.res.Recovered, fmt.Sprintf("%s %s: %s", e.Kind, e.ID, did))
		if strings.Contains(did, " except: ") {
			env.res.Warnings = append(env.res.Warnings, fmt.Sprintf("the %s (journal %s) was reversed only in part: what changed after it stays as it is (see RECOVERED)", e.Kind, e.ID))
		}
	}
	return env, done, nil
}

// resolvedHome is the home directory with symlinks resolved: paths under it
// are compared with resolved paths.
func resolvedHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	return home, nil
}

// worktreeOf is the working tree git finds from dir, or, where git finds
// none because the .git file points at a directory not on this device, the
// folder holding that .git file.
func worktreeOf(ctx context.Context, g *gitx.Runner, dir string) (string, bool) {
	if res, err := g.Run(ctx, dir, nil, "rev-parse", "--show-toplevel"); err == nil && res.ExitCode == 0 {
		if top := strings.TrimSpace(res.Stdout); top != "" {
			return top, true
		}
	}
	return moveout.FindWorktree(dir)
}

// stoppedAfterRecovery: when start() finished or rolled back an earlier
// change, this run ends there. The state the user asked about is not the one
// they see now, so they run the command again (an undo used to recover must
// not undo one change more).
func stoppedAfterRecovery(res *changeResult) bool {
	if len(res.Recovered) == 0 {
		return false
	}
	res.Result = "recovered"
	res.Next = append(res.Next, "An unfinished change was finished or rolled back first; nothing else was done. Check the repository, then run the command again.")
	return true
}

// failpoint makes the process exit right after the named journal phase is
// saved (GITBRELLA_FAILPOINT, for tests of crash recovery).
func failpoint() func(journal.Phase) {
	fp := os.Getenv("GITBRELLA_FAILPOINT")
	if fp == "" {
		return nil
	}
	return func(p journal.Phase) {
		if string(p) == fp {
			os.Exit(99)
		}
	}
}

func runMoveOut(args []string, stdout, stderr io.Writer) int {
	c := newChangeFlags("move-out", stderr).withProvider()
	prepare := c.fs.Bool("prepare", false, "only copy the git directory to where the .git file will point (run on the other devices first)")
	attach := c.fs.Bool("attach", false, "use the git directory the .git file points at on this device (after another device moved out)")
	from := c.fs.String("from", "", "with --attach: the remote URL to clone when this device was not prepared")
	branch := c.fs.String("branch", "", "with --attach --from: the branch HEAD points at (default: the remote's)")
	watchFor := c.fs.Duration("watch", 30*time.Second, "after the move, watch this long that the sync client leaves .git alone (0: do not wait)")
	waitUpload := c.fs.Duration("wait-upload", 0, "while iCloud is still uploading the repository, wait up to this long instead of refusing")
	if !c.parse(args) {
		return 2
	}
	if *prepare && *attach {
		return changeFail(stdout, stderr, *c.asJSON, errors.New("--prepare and --attach exclude each other"))
	}
	env, done, err := start("move-out", c, stderr)
	if err != nil {
		return changeFail(stdout, stderr, *c.asJSON, err)
	}
	defer done()
	res := &env.res
	res.Mode = "move-out"
	switch {
	case *attach:
		res.Mode = "attach"
	case *prepare:
		res.Mode = "prepare"
	}
	if res.Result == "refused" || stoppedAfterRecovery(res) {
		return printChange(stdout, *c.asJSON, *res)
	}
	if *attach {
		a, err := moveout.PlanAttach(env.ctx, env.g, env.opts, *from, *branch, env.store)
		if err != nil {
			return failedChange(stdout, stderr, *c.asJSON, res, err)
		}
		res.Plan, res.Refusals = a, a.Refusals
		switch {
		case len(a.Refusals) > 0:
			res.Result = "refused"
		case !*c.apply:
			res.Result = "planned"
			res.Next = append(res.Next, "Nothing was changed. Run again with --apply to attach this device.")
		default:
			e, err := moveout.Attach(env.ctx, env.g, a, env.opts, env.store)
			if err != nil {
				return failedChange(stdout, stderr, *c.asJSON, res, err)
			}
			res.Result, res.Journal = "done", e.ID
			res.Next = append(res.Next, "git status shows how the synced files differ from the last commit.",
				"To follow the other devices: git fetch, then git reset --mixed @{u} (not git pull: the sync client already brought the files).")
		}
		return printChange(stdout, *c.asJSON, *res)
	}

	p, err := moveout.PlanWaiting(env.ctx, env.g, env.opts, *waitUpload, 5*time.Second)
	if err != nil {
		return failedChange(stdout, stderr, *c.asJSON, res, err)
	}
	res.Plan, res.Refusals, res.Carried = p, p.Refusals, p.Carried
	switch {
	case p.Already && !*prepare:
		res.Result = "already"
		res.Next = append(res.Next, fmt.Sprintf("The git directory is already outside the synced folder: %s.", p.Target))
		return printChange(stdout, *c.asJSON, *res)
	case len(p.Refusals) > 0 || p.Already:
		res.Result = "refused"
		return printChange(stdout, *c.asJSON, *res)
	case !*c.apply:
		res.Result = "planned"
		if *prepare {
			res.Next = append(res.Next, "Nothing was changed. Run again with --apply to copy the git directory to "+p.Target+".")
		} else {
			res.Next = append(res.Next,
				"Nothing was changed. If other devices sync this folder, run 'gitbrella move-out --prepare --apply' on each of them first.",
				"Then run 'gitbrella move-out --apply' here.")
		}
		return printChange(stdout, *c.asJSON, *res)
	}
	if *prepare {
		e, err := moveout.Prepare(env.ctx, p, env.opts, env.store)
		if err != nil {
			return failedChange(stdout, stderr, *c.asJSON, res, err)
		}
		res.Result, res.Journal = "done", e.ID
		res.Next = append(res.Next, "Now move the repository out on one device. When its .git file arrives here, run 'gitbrella move-out --attach --apply'.")
		return printChange(stdout, *c.asJSON, *res)
	}
	e, err := moveout.Apply(env.ctx, env.g, p, env.opts, env.store)
	if err != nil {
		return failedChange(stdout, stderr, *c.asJSON, res, err)
	}
	res.Result, res.Journal, res.Backup = "done", e.ID, e.Data["backup"]
	res.Warnings = append(res.Warnings, watch(env.ctx, *watchFor, p, res.Worktree)...)
	res.Next = append(res.Next,
		"Other devices: when the .git file arrives there, run 'gitbrella move-out --attach --apply' (add --from <remote URL> where you did not prepare).",
		"The sync client now carries your files but not your history: share commits through a remote; on the other devices use git fetch and git reset --mixed @{u} instead of git pull.",
		"Run 'gitbrella check"+c.providerArg()+"' again in ten minutes to confirm the sync client left .git alone.",
		"The original git directory is kept at "+e.Data["backup"]+"; delete it when you no longer need it.")
	return printChange(stdout, *c.asJSON, *res)
}

// watch samples, for d, that .git is still the file the move wrote and the
// git directory is still there; under iCloud it also waits for the .git file
// to be uploaded. It returns warnings.
func watch(ctx context.Context, d time.Duration, p moveout.Plan, worktree string) []string {
	if d <= 0 {
		return nil
	}
	dotgit := filepath.Join(worktree, ".git")
	icloudSynced := p.Repo.Provider == "icloud" || p.Repo.Provider == "icloud-desktop-documents"
	uploaded := false
	deadline := time.Now().Add(d)
	for {
		if b, err := os.ReadFile(dotgit); err != nil || string(b) != p.Gitfile {
			return []string{"GIT_DIR_RETURNED: .git changed after the move (the sync client may have brought the old directory back); do not commit, and run 'gitbrella check'"}
		}
		if _, err := os.Stat(filepath.Join(p.Target, "HEAD")); err != nil {
			return []string{"the moved git directory " + p.Target + " disappeared"}
		}
		if icloudSynced && !uploaded {
			if s, err := icloud.Probe(dotgit); err == nil && s.Known && !s.Pending() {
				uploaded = true
			}
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return []string{"watch interrupted; run 'gitbrella check' later"}
		case <-time.After(5 * time.Second):
		}
	}
	if icloudSynced && !uploaded {
		return []string{"iCloud has not uploaded the .git file yet; run 'gitbrella check' again later"}
	}
	return nil
}

func runUndo(args []string, stdout, stderr io.Writer) int {
	c := newChangeFlags("undo", stderr)
	if !c.parse(args) {
		return 2
	}
	env, done, err := start("undo", c, stderr)
	if err != nil {
		return changeFail(stdout, stderr, *c.asJSON, err)
	}
	defer done()
	res := &env.res
	res.Mode = "move-in"
	if res.Result == "refused" || stoppedAfterRecovery(res) {
		return printChange(stdout, *c.asJSON, *res)
	}
	// undo reverses the newest change that is not undone yet: a fix or a move.
	last, ok, err := env.store.LastUndoable(state.RepoID(res.Worktree, env.opts.Home))
	if err != nil {
		return failedChange(stdout, stderr, *c.asJSON, res, err)
	}
	if ok && last.Kind == "fix" {
		if _, err := undoFix(env, *c.apply, last); err != nil {
			return failedChange(stdout, stderr, *c.asJSON, res, err)
		}
		return printChange(stdout, *c.asJSON, *res)
	}
	u, err := moveout.PlanMoveIn(env.ctx, env.g, env.opts, env.store)
	if err != nil {
		return failedChange(stdout, stderr, *c.asJSON, res, err)
	}
	for i, f := range u.Refusals {
		if f.Code == moveout.RefuseNothingToUndo { // no fix to undo either
			u.Refusals[i].Message = fmt.Sprintf("this device has no fix or move-out of %s to undo", res.Worktree)
		}
	}
	res.Plan, res.Refusals = u, u.Refusals
	switch {
	case len(u.Refusals) > 0:
		res.Result = "refused"
	case !*c.apply:
		res.Result = "planned"
		res.Next = append(res.Next, "Nothing was changed. Run again with --apply to move the git directory back into the working tree.")
	default:
		e, err := moveout.MoveIn(env.ctx, env.g, u, env.opts, env.store)
		if err != nil {
			return failedChange(stdout, stderr, *c.asJSON, res, err)
		}
		res.Result, res.Journal = "done", e.ID
		res.Next = append(res.Next, "The git directory is back in the synced folder.",
			"The original from before the move is still at "+u.Backup+"; delete it when you no longer need it.")
	}
	return printChange(stdout, *c.asJSON, *res)
}

// failedChange reports a change that failed after it started: what was
// recovered first and which journal entry holds it stay in the result.
func failedChange(stdout, stderr io.Writer, asJSON bool, res *changeResult, err error) int {
	fmt.Fprintf(stderr, "gitbrella: %v\n", err)
	res.Result, res.Error = "failed", err.Error()
	return printChange(stdout, asJSON, *res)
}

func changeFail(stdout, stderr io.Writer, asJSON bool, err error) int {
	fmt.Fprintf(stderr, "gitbrella: %v\n", err)
	if asJSON {
		report.WriteJSONError(stdout, version, err.Error(), nil)
	}
	return 2
}

func printChange(w io.Writer, asJSON bool, r changeResult) int {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			return 2
		}
		return r.exitCode()
	}
	var b strings.Builder
	head := r.Command
	if r.Mode != "" && r.Mode != r.Command {
		head += " (" + r.Mode + ")"
	}
	fmt.Fprintf(&b, "gitbrella %s\n%s: %s\n", r.ToolVersion, head, r.Worktree)
	for _, s := range r.Recovered {
		fmt.Fprintf(&b, "RECOVERED %s\n", s)
	}
	switch p := r.Plan.(type) {
	case moveout.Plan:
		if p.Target != "" {
			fmt.Fprintf(&b, "git directory: %s\n.git file:     %s", p.Target, p.Gitfile)
		}
	case moveout.AttachPlan:
		if p.Target != "" {
			fmt.Fprintf(&b, "git directory: %s\nsource:        %s\n", p.Target, p.Source)
		}
	case moveout.UndoPlan:
		if p.GitDir != "" {
			fmt.Fprintf(&b, "git directory: %s comes back as .git\n", p.GitDir)
		}
	case fix.Plan:
		for _, a := range p.Actions {
			what := a.Path
			switch {
			case a.Ref != "":
				what = a.Ref
			case a.Kind == fix.Refetch:
				what = a.Value // the remote
			}
			fmt.Fprintf(&b, "%-13s %s\n              %s: %s\n", strings.ToUpper(a.Kind), what, a.Code, a.Reason)
		}
	case fix.UndoPlan:
		if p.Undoes != "" {
			fmt.Fprintf(&b, "reverses fix %s (%d steps)\n", p.Undoes, len(p.Steps))
		}
	}
	for _, f := range r.Refusals {
		fmt.Fprintf(&b, "REFUSED %s\n        %s\n", f.Code, f.Message)
	}
	for _, m := range r.Manual {
		fmt.Fprintf(&b, "MANUAL  %s  %s\n        %s\n", m.Code, m.Path, m.Do)
	}
	for _, f := range r.Carried {
		fmt.Fprintf(&b, "CARRIED %s  %s\n        damage that moves as it is; run 'gitbrella check' afterwards\n", f.Code, f.Path)
	}
	for _, s := range r.Warnings {
		fmt.Fprintf(&b, "WARNING %s\n", s)
	}
	fmt.Fprintf(&b, "\n%s.\n", r.Result)
	for _, s := range r.Next {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	io.WriteString(w, b.String())
	return r.exitCode()
}
