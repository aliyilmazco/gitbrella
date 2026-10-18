package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// hookAgents are the agents whose hooks runHook speaks: both block a tool
// call on exit 2 with the reason on stderr, and both add a session start
// hook's plain stdout to the model's context.
var hookAgents = []string{"claude", "codex"}

// hookEvent is the part of an agent's hook input runHook reads.
type hookEvent struct {
	Cwd   string `json:"cwd"`
	Event string `json:"hook_event_name"`
	Tool  string `json:"tool_name"`
	Input struct {
		FilePath     string       `json:"file_path"`
		NotebookPath string       `json:"notebook_path"`
		Command      shellCommand `json:"command"` // a shell command, or Codex's apply_patch body
	} `json:"tool_input"`
}

// shellCommand is a tool's command: a string, or an argument list as some
// tools pass it (["bash", "-lc", "git status"] is the script it runs).
type shellCommand string

func (c *shellCommand) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*c = shellCommand(s)
		return nil
	}
	var argv []string
	if err := json.Unmarshal(b, &argv); err != nil {
		return err
	}
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	*c = shellCommand(strings.Join(quoted, " ")) // the reading follows a shell's -c as in any command
	return nil
}

// runHook adapts check to an agent's hooks. The agent passes the event as
// JSON on stdin. Before a tool call (PreToolUse) in a damaged repository, a
// git command that writes, or a write into the git directory, is stopped:
// exit 2 with the reason on stderr, which is how these agents block (an exit
// 1, check's "damage", would let the call through). Reading, editing the
// user's files and gitbrella itself go on, so the damage can be
// looked at and repaired. Where git cannot use the repository at all (its
// files only in the cloud, a renamed HEAD) every git command is stopped: it
// would hang, or act on an enclosing repository. Anything else that keeps
// the check from running only warns, and a folder that is no repository is
// left alone. At a session start (SessionStart) a line of context on stdout
// says the repository is in a synced folder and what is wrong; it never
// blocks. Other events pass. The hook writes nothing.
func runHook(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	fs.SetOutput(stderr)
	setUsage(fs)
	agent := fs.String("for", "", "the agent whose hook calls this: "+strings.Join(hookAgents, ", "))
	failOpen := fs.Bool("fail-open", false, "never stop a tool call when the check cannot run, not even where git cannot use the repository")
	timeout := fs.Duration("timeout", 20*time.Second, "give up checking after this long and let the call through")
	// A usage mistake must not stop every call of the agent: it is a notice,
	// exit 1 in Claude Code and 0 in Codex, as every other warning.
	if err := fs.Parse(args); err != nil {
		return usageExit(args)
	}
	if fs.NArg() > 0 || !slices.Contains(hookAgents, *agent) {
		fmt.Fprintf(stderr, "gitbrella: hook needs --for %s and no path (the agent passes the event on stdin); letting the call through\n", strings.Join(hookAgents, "|"))
		return usageExit(args)
	}
	// warn lets the call through with a notice: Claude Code shows a hook's
	// stderr for exit codes other than 0 and 2, Codex documents only 0 and 2.
	warn := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "gitbrella: "+format+"\n", a...)
		if *agent == "claude" {
			return 1
		}
		return 0
	}
	var ev hookEvent
	if err := json.NewDecoder(stdin).Decode(&ev); err != nil {
		return warn("could not read the hook's input (%v); not checking", err)
	}
	session := ev.Event == "SessionStart"
	if !session && ev.Event != "PreToolUse" {
		return 0
	}
	if err := preventMaterialization(); err != nil {
		return warn("could not disable cloud downloads (%v); not checking", err)
	}
	// one deadline for the whole call, however many folders it touches
	deadline := time.Now().Add(*timeout)
	left := func() time.Duration { return max(time.Until(deadline), 0) }
	targets := hookTargets(ev)
	runs := shellRunsWithin(ev, left()/2)
	dirs, dropped, unplaced := hookDirs(ev, targets, runs)
	call := describeCall(ev, targets, runs)
	var notes []string // session context
	for _, d := range dirs {
		dir := d.path
		rep, err := hookCheck(dir, left())
		switch {
		case errors.Is(err, repo.ErrNotRepo):
			// Hooks run in folders that are no repository all the time; a
			// .git that git cannot use (its cause on disk) is another matter.
			hints, ok := hintsWithin(dir, left())
			if !ok {
				if session {
					continue
				}
				return warn("the check took longer than %s; letting %s through", *timeout, call.name)
			}
			if len(hints) == 0 {
				continue
			}
			if session {
				notes = append(notes, "git cannot use the repository here: "+hints[0]+".")
				continue
			}
			if !d.git {
				continue // what the call does here runs no git
			}
			if *failOpen {
				return warn("git cannot use the repository at %s: %s", dir, hints[0])
			}
			fmt.Fprintf(stderr, "gitbrella: stopped before %s: git cannot use the repository at %s: %s.\n", call.name, dir, hints[0])
			return 2
		case errors.Is(err, context.DeadlineExceeded):
			if !session {
				return warn("the check took longer than %s; letting %s through", *timeout, call.name)
			}
		case errors.Is(err, repo.ErrBare):
			continue // no working tree, nothing a sync client damages in it here
		case err != nil:
			if session {
				fmt.Fprintf(stderr, "gitbrella: the check could not run: %v\n", err)
				continue
			}
			return warn("the check could not run (%v); letting %s through", err, call.name)
		default:
			damage := damageOf(rep)
			if session {
				if line := sessionContext(rep, damage); line != "" {
					notes = append(notes, line)
				}
				continue
			}
			if len(damage) == 0 {
				continue
			}
			hangs := false // git reading cloud-only files hangs or fails
			for _, f := range damage {
				hangs = hangs || f.Code == check.CodeDataless
			}
			if !(d.writes || d.into || hangs && d.git) {
				continue // reading and editing files go on; so does gitbrella
			}
			fmt.Fprintf(stderr, "gitbrella: stopped before %s: the repository at %s has damage a sync client causes:\n", call.name, rep.Repo.Worktree)
			for i, f := range damage {
				if i == 3 {
					fmt.Fprintf(stderr, "  and %d more\n", len(damage)-3)
					break
				}
				fmt.Fprintf(stderr, "  %s %s: %s\n", f.Code, f.Path, f.Message)
			}
			fmt.Fprintf(stderr, "Run 'gitbrella check' there to see it all and 'gitbrella fix' to repair what has one right answer; git commands that write, and writes into .git, are stopped until then.\n")
			return 2
		}
	}
	if session && len(notes) > 0 {
		fmt.Fprintln(stdout, "gitbrella: "+strings.Join(notes, " "))
	}
	if dropped && !session {
		return warn("%s touches more than %d folders; only the first %d were checked", call.name, maxHookDirs, maxHookDirs)
	}
	if unplaced && !session {
		return warn("%s may write to a repository the hook cannot tell from the command (a folder in a variable, a function); the ones it can tell were checked", call.name)
	}
	return 0
}

// notRepoHints is check.NotRepoHints; a test replaces it.
var notRepoHints = check.NotRepoHints

// hintsWithin reads why git cannot use dir, giving up after timeout: the
// hints read git's own files, and one may block (a FIFO, a stalled disk).
func hintsWithin(dir string, timeout time.Duration) ([]string, bool) {
	ch := make(chan []string, 1)
	go func() { ch <- notRepoHints(dir, scan.DefaultFlagReader()) }()
	select {
	case h := <-ch:
		return h, true
	case <-time.After(timeout):
		return nil, false
	}
}

// usageExit is the exit status of a usage mistake: 0 when the arguments name
// Codex (which documents only 0 and 2), else 1, a notice in Claude Code.
func usageExit(args []string) int {
	for i, a := range args {
		if a == "--for=codex" || a == "-for=codex" || (a == "--for" || a == "-for") && i+1 < len(args) && args[i+1] == "codex" {
			return 0
		}
	}
	return 1
}

// hookTargets are the files the tool call works on, absolute: the path of an
// edit or write, or the files a Codex patch adds, updates, deletes or moves
// to. Relative paths are the session folder's.
func hookTargets(ev hookEvent) []string {
	var out []string
	add := func(p string) {
		if p = strings.TrimSpace(p); p == "" {
			return
		}
		if !filepath.IsAbs(p) && ev.Cwd != "" {
			p = filepath.Join(ev.Cwd, p)
		}
		if filepath.IsAbs(p) {
			out = append(out, filepath.Clean(p))
		}
	}
	add(ev.Input.FilePath)
	add(ev.Input.NotebookPath)
	if ev.Tool == "apply_patch" && strings.Contains(string(ev.Input.Command), "*** Begin Patch") {
		for _, line := range strings.Split(string(ev.Input.Command), "\n") {
			for _, mark := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
				if p, ok := strings.CutPrefix(strings.TrimSpace(line), mark); ok {
					add(p)
				}
			}
		}
	}
	return out
}

// maxHookDirs bounds the folders one call checks.
const maxHookDirs = 5

// hookDir is a folder the hook checks and what the call does there.
type hookDir struct {
	path   string
	git    bool // runs git there
	writes bool // runs a git command there that may change the repository
	into   bool // writes a file inside that repository's git directory
}

// hookDirs are the folders to check, at most maxHookDirs: those holding the
// targets (each may be in another repository than the session's), then those
// the shell command's writing git commands (and writes into a git directory)
// run in, the session's folder, and those its other git commands run in.
// Symlinks are resolved and a folder inside a git directory is taken at the
// working tree above it. A git command whose folder the reading cannot tell
// counts in every folder, and unplaced says the call writes so. dropped says a
// folder with something to check did not fit.
func hookDirs(ev hookEvent, targets []string, runs []gitRun) (out []hookDir, dropped, unplaced bool) {
	at := map[string]int{}
	add := func(d hookDir, must bool) {
		if d.path == "" {
			return
		}
		if i, ok := at[d.path]; ok {
			out[i].git, out[i].writes, out[i].into = out[i].git || d.git, out[i].writes || d.writes, out[i].into || d.into
			return
		}
		if len(out) == maxHookDirs {
			dropped = dropped || must
			return
		}
		at[d.path] = len(out)
		out = append(out, d)
	}
	norm := func(d string) string {
		if real, err := filepath.EvalSymlinks(d); err == nil {
			d = real // a link into another repository: that one is written
		}
		if top, ok := aboveGitDir(d); ok {
			d = top
		}
		return resolved(nearestDir(d))
	}
	for _, t := range targets {
		into := inGitDir(t)
		if real, err := filepath.EvalSymlinks(t); err == nil {
			t = real
		} else if parent, err := filepath.EvalSymlinks(filepath.Dir(t)); err == nil {
			t = filepath.Join(parent, filepath.Base(t)) // a new file under a linked folder
		}
		add(hookDir{path: norm(filepath.Dir(t)), into: into || inGitDir(t)}, true)
	}
	if len(targets) > 0 {
		return out, dropped, false
	}
	var anywhere hookDir // what the reading cannot place
	for _, r := range runs {
		switch {
		case r.IntoGit && r.Dir == "":
			anywhere.into = true
		case r.IntoGit:
			add(hookDir{path: norm(r.Dir), into: true}, true)
		case !r.Unknown && isRemedy(r.Args):
		case r.Dir == "":
			anywhere.git, anywhere.writes = true, anywhere.writes || writes(r)
		case writes(r):
			add(hookDir{path: norm(r.Dir), git: true, writes: true}, true)
		}
	}
	cwd := ev.Cwd
	if cwd == "" {
		cwd = "."
	}
	add(hookDir{path: norm(cwd)}, false)
	for _, r := range runs {
		if r.Dir != "" && !r.IntoGit && (r.Unknown || !isRemedy(r.Args)) {
			add(hookDir{path: norm(r.Dir), git: true}, true)
		}
	}
	for i := range out {
		out[i].git = out[i].git || anywhere.git
		out[i].writes = out[i].writes || anywhere.writes
		out[i].into = out[i].into || anywhere.into
	}
	return out, dropped, anywhere.writes || anywhere.into
}

// inGitDir reports whether path is inside a git directory, or is one.
func inGitDir(path string) bool {
	_, ok := aboveGitDir(filepath.Dir(path))
	return ok || isGitDirName(filepath.Base(path))
}

// resolved is d with its symlinks resolved, or d.
func resolved(d string) string {
	if real, err := filepath.EvalSymlinks(d); err == nil {
		return real
	}
	return d
}

// nearestDir is d, or for a file about to be created in folders that do not
// exist yet, the nearest folder above that does.
func nearestDir(d string) string {
	for ; ; d = filepath.Dir(d) {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
		if filepath.Dir(d) == d {
			return d
		}
	}
}

// aboveGitDir returns the folder holding the .git that d is in, if it is.
func aboveGitDir(d string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(d), "/")
	for i, p := range parts {
		if isGitDirName(p) {
			return filepath.FromSlash(strings.Join(parts[:i], "/")), true
		}
	}
	return "", false
}

// toolCall is what the hook needs to know of a tool call.
type toolCall struct {
	name       string
	git        bool // runs git
	writesGit  bool // runs a git command that changes the repository
	intoGitDir bool // writes a file inside a git directory
}

// shellRuns are the git commands of the call's shell command, read from the
// session's folder; none for a Codex patch.
func shellRuns(ev hookEvent) []gitRun {
	cmd := string(ev.Input.Command)
	if cmd == "" || ev.Tool == "apply_patch" {
		return nil
	}
	cwd := ev.Cwd
	if cwd == "" {
		cwd = "."
	}
	return hookReadShell(cmd, cwd)
}

// hookReadShell is readShell; a test replaces it.
var hookReadShell = readShell

// shellRunsWithin is shellRuns, giving up after timeout: a reading that goes
// on longer counts as a git command that writes, in a folder it cannot tell.
func shellRunsWithin(ev hookEvent, timeout time.Duration) []gitRun {
	ch := make(chan []gitRun, 1)
	go func() { ch <- shellRuns(ev) }()
	select {
	case r := <-ch:
		return r
	case <-time.After(timeout):
		return []gitRun{{Unknown: true}}
	}
}

// writes reports whether a git command may change the repository: an unknown
// one does, and so does anything not known to only read.
func writes(r gitRun) bool {
	return r.Unknown || r.IntoGit || len(r.Args) == 0 || !isRemedy(r.Args) && !readOnlyGit(r.Args)
}

// isRemedy is the repair check's core.worktree hint names, which must run
// where git cannot use the repository: git config --file F --unset core.worktree.
func isRemedy(args []string) bool {
	if len(args) < 4 || args[0] != "config" {
		return false
	}
	file, unset, key := false, false, false
	for i, a := range args[1:] {
		switch {
		case (a == "--file" || a == "-f") && i+2 < len(args), strings.HasPrefix(a, "--file="):
			file = true
		case a == "--unset" || a == "--unset-all":
			unset = true
		case a == "core.worktree":
			key = true
		}
	}
	return file && unset && key
}

// describeCall sorts the call: whether its shell command runs git and whether
// a git command may write (the remedy for a foreign core.worktree counts as
// neither), and whether an edit or patch writes inside a git directory.
func describeCall(ev hookEvent, targets []string, runs []gitRun) toolCall {
	c := toolCall{name: ev.Tool}
	if c.name == "" {
		c.name = "the tool call"
	}
	for _, t := range targets {
		c.intoGitDir = c.intoGitDir || inGitDir(t)
	}
	for _, r := range runs {
		switch {
		case r.IntoGit:
			c.intoGitDir = true
		case !r.Unknown && isRemedy(r.Args):
		default:
			c.git = true
			c.writesGit = c.writesGit || writes(r)
		}
	}
	return c
}

// readOnlyGit reports whether a git subcommand (with its arguments) only
// reads the repository. git restore counts: it is how a staged copy is taken
// back out of the index, and it touches no history.
func readOnlyGit(w []string) bool {
	args := w[1:]
	for _, a := range args { // diff --output=F writes F; fsck --lost-found writes .git/lost-found
		if a == "--output" || strings.HasPrefix(a, "--output=") || a == "--lost-found" {
			return false
		}
	}
	only := func(allowed ...string) bool { // every argument a listing option
		for _, a := range args {
			if !slices.Contains(allowed, a) && !strings.HasPrefix(a, "--format") && !strings.HasPrefix(a, "--sort") {
				return false
			}
		}
		return true
	}
	switch w[0] {
	case "status", "diff", "log", "show", "ls-files", "ls-tree", "ls-remote", "rev-parse", "rev-list", "cat-file",
		"fsck", "check-ignore", "check-attr", "check-ref-format", "describe", "blame", "grep", "shortlog",
		"help", "version", "--version", "--help", "var", "show-ref", "for-each-ref", "name-rev", "merge-base",
		"diff-index", "diff-files", "diff-tree", "count-objects", "verify-pack", "restore", "clone", "whatchanged":
		return true
	case "branch":
		return only("-l", "--list", "-a", "--all", "-r", "--remotes", "-v", "-vv", "--verbose", "--show-current", "--contains", "--merged", "--no-merged")
	case "tag":
		return only("-l", "--list", "-n", "--contains", "--merged", "--no-merged", "--points-at")
	case "remote":
		for len(args) > 0 && (args[0] == "-v" || args[0] == "--verbose") {
			args = args[1:]
		}
		return len(args) == 0 || args[0] == "show" || args[0] == "get-url"
	case "config":
		for _, a := range args {
			if strings.HasPrefix(a, "--get") || a == "--list" || a == "-l" {
				return true
			}
		}
		return false
	case "stash", "worktree":
		return len(args) > 0 && args[0] == "list" || w[0] == "stash" && len(args) > 0 && args[0] == "show"
	case "reflog":
		return len(args) == 0 || args[0] == "show"
	case "submodule":
		return len(args) > 0 && args[0] == "status"
	case "lfs":
		return len(args) > 0 && slices.Contains([]string{"ls-files", "status", "env", "version", "logs"}, args[0]) &&
			!(args[0] == "logs" && slices.Contains(args, "clear"))
	}
	return false
}

// hookCheck runs the check on the repository at dir the way 'check' does
// without flags; a hook of an agent is no git hook, so no git process or
// index is taken from the environment.
func hookCheck(dir string, timeout time.Duration) (check.Report, error) {
	if timeout <= 0 {
		return check.Report{}, context.DeadlineExceeded
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	g, err := gitx.New()
	if err != nil {
		return check.Report{}, err
	}
	major, minor, err := g.Version(ctx)
	if err != nil {
		return check.Report{}, err
	}
	if !gitx.AtLeast(major, minor, 2, 31) {
		return check.Report{}, fmt.Errorf("git 2.31 or newer is required (found %d.%d)", major, minor)
	}
	home, _ := resolvedHome()
	journalDir := ""
	if dirs, err := state.Resolve(home, runtime.GOOS, os.Getenv); err == nil {
		journalDir = dirs.Journal()
	}
	type result struct {
		rep check.Report
		err error
	}
	done := make(chan result, 1)
	go func() { // a read that blocks (a FIFO, a stalled disk) must not hold the agent
		rep, err := hookRunCheck(ctx, g, check.Options{
			Path: dir, Provider: "auto", Home: home, GOOS: runtime.GOOS, Now: time.Now(),
			Flags: scan.DefaultFlagReader(), Procs: procs.GitProcesses, StaleAfter: time.Minute, Version: version,
			JournalDir: journalDir,
		})
		done <- result{rep, err}
	}()
	select {
	case r := <-done:
		if ctx.Err() != nil {
			return check.Report{}, context.DeadlineExceeded // a check cut short: no answer, never "no damage"
		}
		return r.rep, r.err
	case <-ctx.Done():
		return check.Report{}, context.DeadlineExceeded
	}
}

// hookRunCheck is check.Run; a test replaces it.
var hookRunCheck = check.Run

func damageOf(rep check.Report) []check.Finding {
	var out []check.Finding
	for _, f := range rep.Findings {
		if f.Severity == check.Damage {
			out = append(out, f)
		}
	}
	return out
}

// sessionContext is what an agent gets at a session start about one
// repository: where it is synced (unless its git directory already is not)
// and what is damaged; "" when there is nothing to say.
func sessionContext(rep check.Report, damage []check.Finding) string {
	var parts []string
	moved := false
	for _, f := range rep.Findings {
		moved = moved || f.Code == check.CodeGitDirNotSynced
	}
	if p := rep.Repo.Provider; p != "" && p != "none" && !moved {
		parts = append(parts, fmt.Sprintf("this repository is in a folder %s syncs (%s); sync clients damage git directories (copies like 'index 2', files left only in the cloud). The lasting fix is 'gitbrella move-out'.", p, rep.Repo.Worktree))
	}
	if len(damage) > 0 {
		var codes []string
		for _, f := range damage {
			codes = append(codes, f.Code+" "+f.Path)
		}
		parts = append(parts, fmt.Sprintf("It has damage now (%s): do not run git commands that write until 'gitbrella fix' or the user repairs it.", strings.Join(codes, ", ")))
	}
	return strings.Join(parts, " ")
}
