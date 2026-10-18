// Command gitbrella diagnoses git repositories damaged by cloud sync.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/fix"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/report"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usageText = `gitbrella diagnoses git repositories damaged by cloud sync (iCloud Drive,
Dropbox, OneDrive, Google Drive, Syncthing, Nextcloud and others).

Usage:
  gitbrella check [--json] [--deep] [--provider NAME] [path]
  gitbrella move-out [--apply] [--watch DURATION] [--wait-upload DURATION] [--provider NAME] [--json] [path]
  gitbrella move-out --prepare [--apply] [--json] [path]
  gitbrella move-out --attach [--from URL] [--branch NAME] [--apply] [--json] [path]
  gitbrella fix [--apply] [--watch DURATION] [--remote NAME] [--deep] [--provider NAME] [--json] [path]
  gitbrella undo [--apply] [--json] [path]
  gitbrella history [--json] [path]
  gitbrella prune-backups [--older-than DURATION] [--apply] [--json] [path]
  gitbrella hook --for claude|codex [--fail-open] [--timeout DURATION]   (reads the agent's hook event on stdin)
  gitbrella version

check only reads; it never changes the repository. --deep also runs a full
git fsck, which reads every object and can take minutes on large repositories.
--provider forces the sync client instead of detecting it from the path: auto
(default), icloud, icloud-desktop-documents, dropbox, onedrive, google-drive,
syncthing, nextcloud, cloud-storage or none.
Exit status: 0 no damage, 1 damage found, 2 the check could not run.

move-out moves .git out of the synced folder and leaves a .git file pointing
at it; --prepare readies the other devices before, --attach after. fix
repairs the damage check finds where there is one right answer and lists the
rest. undo reverses the last fix or move. Without --apply they only say what
they would do. fix and move-out take --provider as check does: give the same
one, so they see the damage check reported with it. Exit status: 0 planned,
done or nothing to do, 1 refused (the reasons are listed), done with a
warning, or damage left to you (fix's MANUAL list), 2 failed (rolled back or
stopped; see the error) or could not run.

history lists the changes this device made to the repository, their backups
and sizes, and which one undo reverses next. prune-backups deletes the
backups of changes that are undone, rolled back or never undoable, and whose
last step is older than --older-than (default 720h); others stay.

hook speaks the hooks of Claude Code and Codex. Before a tool call
(PreToolUse) in a damaged repository, a git command that writes, or a write
into .git, is stopped (exit 2, the reason on stderr); where git cannot use the
repository, every git command is. At a session start (SessionStart) one line
of context says the repository is synced and what is wrong. It changes
nothing; --fail-open lets the call through whenever the check cannot run.
`

func main() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		version = resolveVersion(version, bi)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 2
	}
	switch args[0] {
	case "version", "--version":
		fmt.Fprintf(stdout, "gitbrella %s\n", version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return 0
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "move-out":
		return runMoveOut(args[1:], stdout, stderr)
	case "fix":
		return runFix(args[1:], stdout, stderr)
	case "undo":
		return runUndo(args[1:], stdout, stderr)
	case "history":
		return runHistory(args[1:], stdout, stderr)
	case "prune-backups":
		return runPruneBackups(args[1:], stdout, stderr)
	case "hook":
		return runHook(args[1:], os.Stdin, stdout, stderr)
	}
	fmt.Fprintf(stderr, "gitbrella: unknown command %q\n\n%s", args[0], usageText)
	return 2
}

// providerNames are the values --provider accepts.
var providerNames = []string{"auto", "icloud", "icloud-desktop-documents", "dropbox", "onedrive", "google-drive", "syncthing", "nextcloud", "cloud-storage", "none"}

// setUsage makes fs print the command's lines of usageText, then its flags,
// when a flag is wrong or -h is given.
func setUsage(fs *flag.FlagSet) {
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, "usage:")
		for _, l := range strings.Split(usageText, "\n") {
			if strings.HasPrefix(l, "  gitbrella "+fs.Name()+" ") {
				fmt.Fprintln(out, l)
			}
		}
		fs.PrintDefaults()
	}
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	setUsage(fs)
	asJSON := fs.Bool("json", false, "print the report as JSON")
	provider := fs.String("provider", "auto", "sync provider: "+strings.Join(providerNames, ", "))
	deep := fs.Bool("deep", false, "also run a full git fsck (reads every object; can be slow)")
	// Flags may come before or after the path.
	var paths []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		paths = append(paths, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	// fail reports an error that stops the check: on stderr, and as a JSON
	// object on stdout when --json was asked for, so an agent always gets JSON.
	fail := func(msg string, hints []string) int {
		fmt.Fprintf(stderr, "gitbrella: %s\n", msg)
		for _, h := range hints {
			fmt.Fprintf(stderr, "gitbrella: hint: %s\n", h)
		}
		if *asJSON {
			report.WriteJSONError(stdout, version, msg, hints)
		}
		return 2
	}
	// Turn off cloud downloads before touching the filesystem at all.
	if err := preventMaterialization(); err != nil {
		return fail(fmt.Sprintf("could not disable cloud downloads (%v); not running, since reading the repository could download files", err), nil)
	}
	if len(paths) > 1 {
		return fail("give at most one path", nil)
	}
	if !slices.Contains(providerNames, *provider) {
		return fail(fmt.Sprintf("unknown provider %q", *provider), nil)
	}
	path := "."
	var gitEnv []string
	if len(paths) == 1 {
		path = paths[0]
	} else {
		// Without a path, use the repository git itself points at: it exports
		// GIT_DIR (and GIT_WORK_TREE) to hooks of --git-dir and core.worktree
		// setups, where discovery from the working directory would miss it.
		// Values are kept as given; they are resolved against git's own
		// directory when the hook's git is found, else the current directory.
		for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE"} {
			if v := os.Getenv(k); v != "" {
				gitEnv = append(gitEnv, k+"="+v)
			}
		}
	}
	// A signal cancels the context: git is stopped and deferred clean-up (the
	// temporary index copy) still runs before the exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	g, err := gitx.New()
	if err != nil {
		return fail(err.Error(), nil)
	}
	major, minor, err := g.Version(ctx)
	if err != nil {
		return fail(err.Error(), nil)
	}
	if !gitx.AtLeast(major, minor, 2, 31) {
		return fail(fmt.Sprintf("git 2.31 or newer is required (found %d.%d)", major, minor), nil)
	}
	// A commit hook (GIT_INDEX_FILE) or a hook given an absolute GIT_DIR:
	// find the git that ran it. (A relative GIT_DIR alone, as in
	// `submodule foreach`, is relative to the command's own directory.)
	var hookCwd string
	if len(paths) == 0 && (os.Getenv("GIT_INDEX_FILE") != "" || filepath.IsAbs(os.Getenv("GIT_DIR"))) {
		if p, ok := hookGit(ctx); ok {
			hookCwd = p.Cwd
		}
	}
	home, _ := resolvedHome()
	journalDir := "" // where this device's changes are recorded; unusable state means none
	dirs, dirsErr := state.Resolve(home, runtime.GOOS, os.Getenv)
	if dirsErr == nil {
		journalDir = dirs.Journal()
	}
	rep, err := check.Run(ctx, g, check.Options{
		Path: path, Provider: *provider, Home: home, GOOS: runtime.GOOS, Now: time.Now(),
		Flags: scan.DefaultFlagReader(), Procs: procs.GitProcesses, StaleAfter: time.Minute, Version: version,
		Deep: *deep, IndexFile: os.Getenv("GIT_INDEX_FILE"), // set by git for commit hooks
		GitEnv: gitEnv, HookCwd: hookCwd, JournalDir: journalDir,
	})
	if ctx.Err() != nil {
		// Checks after the signal did not run: no answer, never "no damage".
		return fail("interrupted by a signal; the check did not finish", nil)
	}
	if err != nil {
		var hints []string
		if errors.Is(err, repo.ErrNotRepo) && len(gitEnv) == 0 {
			// with GIT_DIR from a hook, the current directory is not the question
			hints = check.NotRepoHints(path, scan.DefaultFlagReader())
			if abs, err := filepath.Abs(path); err == nil && journalDir != "" {
				if open, wt, _ := (journal.Store{Dir: journalDir}).UnfinishedAt(abs); len(open) > 0 {
					hints = append(hints, fmt.Sprintf("a gitbrella %s of %s did not finish, so .git is missing; run 'gitbrella move-out --apply' (or 'undo --apply') there to finish or roll it back", open[0].Kind, wt))
				}
			}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			hints = append(hints, "a git command timed out")
		}
		return fail(err.Error(), hints)
	}
	// Outside commit hooks (which must stay fast), say which damage fix
	// repairs: planning reads, it never writes.
	if check.ExitCode(rep) == 1 && dirsErr == nil && os.Getenv("GIT_INDEX_FILE") == "" && hookCwd == "" {
		annotateRepairs(ctx, g, &rep, fix.Options{Dirs: dirs, Env: change.Env{Home: home, GOOS: runtime.GOOS,
			Flags: scan.DefaultFlagReader(), Procs: procs.GitProcesses, Version: version, JournalDir: journalDir, Provider: *provider}})
	}
	if *asJSON {
		err = report.WriteJSON(stdout, rep)
	} else {
		err = report.WriteText(stdout, rep)
	}
	if err != nil {
		fmt.Fprintf(stderr, "gitbrella: %v\n", err)
		return 2
	}
	return check.ExitCode(rep)
}

// annotateRepairs marks the damage in rep with what fix does about it. A
// plan that cannot be made leaves the report as it is.
func annotateRepairs(ctx context.Context, g *gitx.Runner, rep *check.Report, o fix.Options) {
	hasDamage := false
	for _, f := range rep.Findings {
		hasDamage = hasDamage || f.Severity == check.Damage
	}
	if !hasDamage {
		return
	}
	r, err := repo.Locate(ctx, g, rep.Repo.Worktree)
	if err != nil {
		return
	}
	o.Path = r.Worktree
	if p, err := fix.PlanFor(ctx, g, o, r, *rep); err == nil {
		p.Annotate(rep)
	}
}

// hookGit is replaceable in tests.
var hookGit = procs.HookGit

// preventMaterialization is replaceable in tests.
var preventMaterialization = scan.PreventMaterialization

// resolveVersion prefers the version set at build time and otherwise takes
// the module version recorded in the build: vX.Y.Z for 'go install ...@vX.Y.Z',
// a pseudo-version (possibly +dirty) for a local build with Go 1.24 or newer.
func resolveVersion(set string, bi *debug.BuildInfo) string {
	if set != "dev" || bi == nil || bi.Main.Version == "" || bi.Main.Version == "(devel)" {
		return set
	}
	return bi.Main.Version
}
