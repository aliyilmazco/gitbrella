// Package check combines repository location, scanning and verification into
// a report. It never writes to the repository.
package check

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/cloud"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/shq"
	"github.com/aliyilmazco/gitbrella/internal/state"
	"github.com/aliyilmazco/gitbrella/internal/verify"
)

// Severity of a finding. Only Damage makes the exit status 1.
type Severity string

const (
	Damage Severity = "damage"
	Info   Severity = "info"
)

// Finding codes.
const (
	CodeCopy              = "COPY_IN_GITDIR"
	CodeIndexCopy         = "INDEX_COPY"
	CodeRefCopy           = "REF_COPY"
	CodeIndexMissing      = "INDEX_MISSING"
	CodeOrphanIndexLock   = "ORPHAN_INDEX_LOCK"
	CodeIndexInvalid      = "INDEX_INVALID"
	CodeStaleLock         = "STALE_LOCK"
	CodeDataless          = "DATALESS_IN_GITDIR"
	CodeInCloudFolder     = "IN_CLOUD_FOLDER"
	CodeGitRunning        = "GIT_RUNNING"
	CodeLockInUse         = "LOCK_IN_USE"
	CodeLockRecent        = "LOCK_RECENT"
	CodeInProgress        = "IN_PROGRESS_OPERATION"
	CodeReftable          = "REFTABLE_UNSUPPORTED"
	CodeUnknown           = "UNKNOWN_IN_GITDIR"
	CodeUnreadable        = "UNREADABLE_IN_GITDIR"
	CodeProcessCheckFail  = "PROCESS_CHECK_FAILED"
	CodeForeign           = "FOREIGN_FILE_IN_GITDIR"
	CodeEmpty             = "EMPTY_FILE_IN_GITDIR"
	CodeWorktreeAdmin     = "WORKTREE_ADMIN_BROKEN"
	CodeStaleOperation    = "STALE_OPERATION_STATE"
	CodeGitDirCopy        = "GITDIR_COPY"
	CodeWorktreeCopies    = "WORKTREE_COPIES"
	CodeFsck              = "FSCK_ERRORS"
	CodeDeepSkipped       = "DEEP_CHECK_SKIPPED"
	CodeNoCheckout        = "NO_CHECKOUT"
	CodeGitDirNotSynced   = "GIT_DIR_NOT_SYNCED"
	CodeGitDirNotIgnored  = "GIT_DIR_NOT_IGNORED"
	CodePartialClone      = "PARTIAL_CLONE_UNVERIFIED"
	CodeWorktreeTruncated = "WORKTREE_SCAN_TRUNCATED"
	CodeIgnoreUnchecked   = "GIT_DIR_IGNORE_UNCHECKED"
	CodeGitTempFile       = "GIT_TEMP_FILE"
	CodeIndexEmpty        = "INDEX_EMPTY"
	CodeHEADCommitMissing = "HEAD_COMMIT_MISSING"
	CodeAlternateDataless = "DATALESS_IN_ALTERNATE"
	CodeWorktreeGuessed   = "WORKTREE_GUESSED"
	CodeIndexShrunk       = "INDEX_SHRUNK"
	CodeGitDirReturned    = "GIT_DIR_RETURNED"
	CodeChangeUnfinished  = "CHANGE_UNFINISHED"
	CodeStagedCopy        = "STAGED_CONFLICT_COPY"
	CodeReftableUnlisted  = "REFTABLE_TABLE_UNLISTED"
	CodeGCPIDForeign      = "GC_PID_FOREIGN"
	CodeGCLogFresh        = "GC_LOG_FRESH"
	CodeSyncedHooks       = "HOOKS_IN_SYNCED_FOLDER"
)

const (
	// holderSlack: a git that started up to this long after a lock's mtime
	// still counts as its possible holder. It absorbs ps's whole seconds and
	// file servers that stamp mtimes with their own, slower clock; stale
	// locks are at least StaleAfter (a minute) old, far beyond it.
	holderSlack = 30 * time.Second
	// futureSlack: a lock dated further ahead than this was written by a
	// device whose clock runs ahead.
	futureSlack = 2 * time.Second
)

// ErrNotTheHooksRepository: run from a commit hook (GIT_INDEX_FILE is set), but
// no git works on the repository found from the current directory, so the
// hook changed into another repository.
var ErrNotTheHooksRepository = errors.New("this commit hook's repository is not the one found here")

// lazyFetchSafe reports whether git honours GIT_NO_LAZY_FETCH (2.45+). On
// older git, reading a missing object in a partial clone fetches it from the
// remote and writes a pack, so object reads are skipped there.
var lazyFetchSafe = func(ctx context.Context, g *gitx.Runner) bool {
	major, minor, err := g.Version(ctx)
	return err == nil && gitx.AtLeast(major, minor, 2, 45)
}

// WorktreeLimit caps how many working-tree entries are visited.
const WorktreeLimit = 200000

// Finding is one observation about the repository.
type Finding struct {
	Code     string            `json:"code"`
	Severity Severity          `json:"severity"`
	Path     string            `json:"path,omitempty"`
	Message  string            `json:"message"`
	Details  map[string]string `json:"details,omitempty"`
	Repair   string            `json:"repair,omitempty"` // damage only, set by the check command: "fix" repairs it, "manual" lists what to run
}

// RepoInfo describes the checked repository.
type RepoInfo struct {
	Worktree  string `json:"worktree"`
	GitDir    string `json:"gitDir"`
	CommonDir string `json:"commonDir"`
	Provider  string `json:"provider"`
	// ProviderForced: the provider was given (--provider), not detected from the path
	ProviderForced bool   `json:"providerForced,omitempty"`
	ObjectFormat   string `json:"objectFormat"`
	RefStorage     string `json:"refStorage"`
}

// Report is the result of a check. SchemaVersion changes only on breaking changes.
type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	ToolVersion   string    `json:"toolVersion"`
	Repo          RepoInfo  `json:"repo"`
	Findings      []Finding `json:"findings"`
}

// Options configures Run. Every field except Path has a usable zero value
// except Flags and Procs, which the caller must set.
type Options struct {
	Path          string
	Provider      string // "auto" (or "") detects from the path
	Home          string
	GOOS          string
	Now           time.Time
	Flags         scan.FlagReader
	Procs         func(context.Context) ([]procs.Proc, error)
	StaleAfter    time.Duration // a git lock older than this with no git running is stale
	Version       string
	Deep          bool          // also run a full git fsck (reads every object)
	FsckTimeout   time.Duration // limit for the deep check; default 10 minutes
	WorktreeLimit int           // entries the working-tree scan visits; default 200000
	IndexFile     string        // the index git is about to use: GIT_INDEX_FILE inside a hook
	GitEnv        []string      // GIT_DIR / GIT_WORK_TREE that git exported to a hook, used to find the repository
	HookCwd       string        // directory of the git that ran this process as a hook (procs.HookGit), "" otherwise
	JournalDir    string        // the journal of gitbrella's changes on this device; "" skips those findings
}

// ExitCode is 1 when the report contains damage, 0 otherwise.
func ExitCode(r Report) int {
	for _, f := range r.Findings {
		if f.Severity == Damage {
			return 1
		}
	}
	return 0
}

// Run checks the repository at o.Path.
func Run(ctx context.Context, g *gitx.Runner, o Options) (Report, error) {
	if o.StaleAfter == 0 {
		o.StaleAfter = time.Minute
	}
	r, indexFile, err := locateRepo(ctx, g, o)
	if err != nil {
		return Report{}, err
	}
	o.IndexFile = indexFile
	provider := cloud.Provider(o.Provider)
	if o.Provider == "" || o.Provider == "auto" {
		provider = cloud.DetectAt(r.Worktree, o.Home, o.GOOS)
	}
	if (provider == cloud.ICloud || provider == cloud.ICloudDesktopDocuments) && cloud.IsNoSync(r.Worktree) {
		provider = cloud.None // iCloud syncs nothing under a *.nosync folder
	}
	rep := Report{SchemaVersion: 1, ToolVersion: o.Version, Repo: RepoInfo{
		Worktree: r.Worktree, GitDir: r.GitDir, CommonDir: r.CommonDir,
		Provider: string(provider), ProviderForced: o.Provider != "" && o.Provider != "auto",
		ObjectFormat: r.ObjectFormat, RefStorage: r.RefStorage,
	}}
	add := func(f Finding) { rep.Findings = append(rep.Findings, f) }

	switch {
	case provider == cloud.None:
	case gitDirSynced(r, provider, o.Home, o.GOOS):
		add(Finding{Code: CodeInCloudFolder, Severity: Info, Path: r.Worktree,
			Message: fmt.Sprintf("repository is inside a folder synced by %s; moving .git out of it prevents this damage", provider)})
	default:
		add(Finding{Code: CodeGitDirNotSynced, Severity: Info, Path: r.CommonDir,
			Message: fmt.Sprintf("the working tree is synced by %s but the git directory is not; this is the safe layout", provider)})
	}
	for _, f := range journalFindings(o, r) {
		add(f)
	}
	// innerGitDir is the git directory's path inside the working tree when it
	// lives there under a name other than .git (for example .git.nosync).
	innerGitDir := ""
	if rel := relTo(r.Worktree, r.CommonDir); rel != r.CommonDir && !slices.Contains(strings.Split(rel, "/"), ".git") {
		innerGitDir = rel // git never adds a directory named .git, so those need no ignore rule
	}

	// List git processes before scanning: a lock released in between is then
	// seen as gone rather than as left behind.
	procsKnown := true
	ps, err := o.Procs(ctx)
	if err != nil {
		procsKnown = false // unknown: never call a lock stale
		add(Finding{Code: CodeProcessCheckFail, Severity: Info, Message: "could not list git processes; locks are not judged stale: " + err.Error()})
	}
	dirs := worktreeDirs(r, o.Flags)
	// A git dir inside its own worktree (core.worktree) is not found by
	// discovery from the worktree top, where git runs its commands.
	topCommon, _, topOK := commonDirOf(r.Worktree, o.Flags)
	discoverable := topOK && sameFile(topCommon, r.CommonDir)
	var repoProcs []procs.Proc
	unknownCwd := 0
	for _, p := range ps {
		switch {
		case p.Cwd == "":
			unknownCwd++ // could be working here
			repoProcs = append(repoProcs, p)
		case processInRepo(p, r, o.Flags, dirs, discoverable):
			repoProcs = append(repoProcs, p)
			add(Finding{Code: CodeGitRunning, Severity: Info, Message: "a git process is running in this repository", Details: map[string]string{"pid": strconv.Itoa(p.PID)}})
		}
	}
	if unknownCwd > 0 {
		add(Finding{Code: CodeProcessCheckFail, Severity: Info,
			Message: fmt.Sprintf("could not read where %d git process(es) work; they count as possibly working here", unknownCwd)})
	}
	// holds reports whether a git working on this repository could hold a lock
	// written at t. git creates a lock when it takes it, so a git that started
	// later (a hook's own git, an alias, a pager left open) cannot own it.
	holds := func(t time.Time) bool {
		if !procsKnown {
			return true
		}
		for _, p := range repoProcs {
			if p.Start.IsZero() || !p.Start.After(t.Add(holderSlack)) {
				return true
			}
		}
		return false
	}
	hookIndex := hookIndexPath(o.IndexFile, r)
	// A commit hook whose index is not this repository's, while no git works
	// on this repository: the hook changed into another repository. (A client
	// that runs hooks itself, without a git process, still passes its own
	// repository's index.)
	if o.IndexFile != "" && hookIndex == "" && procsKnown && len(repoProcs) == 0 {
		return Report{}, fmt.Errorf("%w: no git is working on %s; the hook probably changed into another repository (run the check from the working tree, or give its path)", ErrNotTheHooksRepository, r.Worktree)
	}

	sr, err := scan.Scan(r.CommonDir, o.Flags, o.Now, provider)
	if err != nil {
		return Report{}, err
	}
	rel := func(p string) string { return relTo(r.CommonDir, p) }

	ops, stale := verify.OperationState(r.GitDir)
	if len(ops) > 0 {
		add(Finding{Code: CodeInProgress, Severity: Info, Message: "an unfinished git operation is recorded: " + strings.Join(ops, ", ")})
	}
	for _, st := range stale {
		t := o.Now
		if info, err := os.Lstat(filepath.Join(r.GitDir, st)); err == nil {
			t = info.ModTime() // only a git that started by then can be writing it
		}
		if holds(t) {
			// git creates the directory before the files in it
			add(Finding{Code: CodeInProgress, Severity: Info, Path: rel(filepath.Join(r.GitDir, st)),
				Message: "operation directory without the files git needs, while git is running; run the check again when it is done"})
			continue
		}
		add(Finding{Code: CodeStaleOperation, Severity: Damage, Path: rel(filepath.Join(r.GitDir, st)),
			Message: "leftover operation directory without the files git needs; git will claim an operation is in progress"})
	}
	if r.RefStorage == "reftable" {
		add(Finding{Code: CodeReftable, Severity: Info, Message: "reftable repositories are not supported; ref copies are not verified"})
	}
	for _, u := range sr.Unknown {
		add(Finding{Code: CodeUnknown, Severity: Info, Path: u, Message: "not created by git; left untouched"})
	}
	for _, tmp := range sr.Temp {
		msg := "temporary file git names after a process; no other git waits on it"
		if m := tempPIDRe.FindStringSubmatch(tmp); m != nil {
			if pid, _ := strconv.Atoi(m[1]); procs.Alive(pid) {
				msg += "; process " + m[1] + " is running, so leave it"
			} else {
				msg += "; no process " + m[1] + " runs on this machine, so it is safe to delete unless git on another machine shares this folder"
			}
		}
		add(Finding{Code: CodeGitTempFile, Severity: Info, Path: tmp, Message: msg})
	}
	for _, u := range sr.Unreadable {
		add(Finding{Code: CodeUnreadable, Severity: Info, Path: u, Message: "could not be read"})
	}
	for _, f := range sr.Foreign {
		add(Finding{Code: CodeForeign, Severity: Damage, Path: f,
			Message: "file git did not create inside refs/ or objects/; git reads it as a ref or object and fails"})
	}
	for _, f := range sr.Empty {
		add(Finding{Code: CodeEmpty, Severity: Damage, Path: f,
			Message: "zero-byte file where git expects data, usually an interrupted sync"})
	}
	for _, w := range sr.BrokenWorktrees {
		add(Finding{Code: CodeWorktreeAdmin, Severity: Damage, Path: w,
			Message: "linked-worktree admin directory is missing HEAD, gitdir or commondir"})
	}
	if r.RefStorage == "reftable" && len(sr.Dataless) == 0 {
		if f, ok := reftableUnlisted(r); ok {
			add(f)
		}
	}
	for _, f := range gcFindings(r, o.Now) {
		add(f)
	}
	for _, f := range syncedHooks(ctx, g, r, o.Home, o.GOOS) {
		add(f)
	}

	// A working tree taken from the current directory (GIT_DIR without one)
	// is checked only where HEAD's files are: elsewhere it may be any
	// directory, and its findings would be about the wrong place.
	worktreeKnown := !r.WorktreeGuessed || worktreeConfirmed(ctx, g, r, o.Flags, len(sr.Dataless) > 0)
	if !worktreeKnown {
		add(Finding{Code: CodeWorktreeGuessed, Severity: Info, Path: r.Worktree,
			Message: "GIT_DIR names no working tree and none of HEAD's files are in the current directory, so it may not be this repository's; the working tree was not checked"})
	}

	limit := o.WorktreeLimit
	if limit == 0 {
		limit = WorktreeLimit
	}
	var skip []string
	if innerGitDir != "" {
		skip = append(skip, innerGitDir)
	}
	var wt scan.WorktreeResult
	if worktreeKnown {
		wt, err = scan.ScanWorktree(r.Worktree, o.Flags, limit, skip...)
		if err != nil {
			return Report{}, err
		}
	}
	if wt.Truncated {
		add(Finding{Code: CodeWorktreeTruncated, Severity: Info,
			Message: fmt.Sprintf("the working-tree scan stopped after %d entries; conflict copies further down were not looked for", limit)})
	}
	for _, c := range wt.GitDirs {
		add(Finding{Code: CodeGitDirCopy, Severity: Damage, Path: c,
			Message: "copy of the git directory inside the working tree; 'git add -A' would commit it"})
	}
	if len(wt.Copies) > 0 {
		sample := wt.Copies
		if len(sample) > 10 {
			sample = sample[:10]
		}
		add(Finding{Code: CodeWorktreeCopies, Severity: Info, Path: wt.Copies[0],
			Message: fmt.Sprintf("%d file(s) look like sync conflict copies of files next to them; auto-commit or 'git add -A' would commit them", len(wt.Copies)),
			Details: map[string]string{"count": strconv.Itoa(len(wt.Copies)), "sample": strings.Join(sample, ", "), "truncated": strconv.FormatBool(wt.Truncated)}})
	}

	blocked := len(sr.Dataless) > 0
	if blocked {
		add(Finding{Code: CodeDataless, Severity: Damage, Path: sr.Dataless[0],
			Message: "files inside the git directory exist only in the cloud; checks that read objects were skipped so nothing gets downloaded",
			Details: map[string]string{"count": strconv.Itoa(len(sr.Dataless))}})
	}
	dataless := func(p string) bool { d, _ := o.Flags.IsDataless(p); return d }

	// noObjects: reading objects could download them (dataless files) or
	// fetch them from a promisor remote (partial clone on git < 2.45).
	noObjects := blocked
	if alt := cloudOnlyAlternate(r, o.Flags); alt != "" && !blocked {
		noObjects = true
		add(Finding{Code: CodeAlternateDataless, Severity: Info, Path: alt,
			Message: "an alternate object store holds files that exist only in the cloud; checks that read objects were skipped so nothing gets downloaded"})
	}
	if !noObjects && partialClone(ctx, g, r) && !lazyFetchSafe(ctx, g) {
		noObjects = true
		add(Finding{Code: CodePartialClone, Severity: Info,
			Message: "partial clone on git older than 2.45: checks that read objects were skipped so git does not fetch from the remote"})
	}
	split := splitIndex(r)

	if innerGitDir != "" && !blocked && worktreeKnown {
		// git lists the directory as untracked unless an ignore rule covers it.
		// check-ignore --no-index answers from the rules alone and never reads
		// the index (reading a split index would touch sharedindex.*).
		if f := cloudOnlyIgnoreFile(ctx, g, r, innerGitDir, o.Home, o.Flags); f != "" {
			// git cannot read the rule here, but a real 'git add' would download and honour it.
			add(Finding{Code: CodeIgnoreUnchecked, Severity: Info, Path: innerGitDir,
				Message: "could not tell whether the git directory inside the working tree is ignored: " + f + " exists only in the cloud"})
		} else if res, err := g.Run(ctx, r.Worktree, r.Env(), "check-ignore", "--no-index", "-q", "--", innerGitDir+"/"); err != nil ||
			(res.ExitCode != 0 && res.ExitCode != 1) || (res.ExitCode == 1 && strings.TrimSpace(res.Stderr) != "") {
			// Exit 1 with a warning means git skipped an ignore file it could not read.
			reason := strings.TrimSpace(res.Stderr)
			if err != nil {
				reason = err.Error()
			}
			add(Finding{Code: CodeIgnoreUnchecked, Severity: Info, Path: innerGitDir,
				Message: "could not tell whether the git directory inside the working tree is ignored: " + reason})
		} else if res.ExitCode == 1 {
			add(Finding{Code: CodeGitDirNotIgnored, Severity: Damage, Path: innerGitDir,
				Message: fmt.Sprintf("the git directory is inside the working tree and not ignored; 'git add -A' would commit git's own files. Add '/%s/' to %s", innerGitDir, innerGitDir+"/info/exclude")})
		}
	}

	indexPath := filepath.Join(r.GitDir, "index")
	_, statErr := os.Lstat(indexPath)
	indexExists := statErr == nil
	switch {
	case indexExists && !dataless(indexPath):
		if st := verify.CheckIndexFile(indexPath, r.ObjectFormat); !st.Valid && !st.Unverifiable {
			add(Finding{Code: CodeIndexInvalid, Severity: Damage, Path: rel(indexPath), Message: "the index file is damaged: " + st.Reason})
		}
	case !indexExists:
		lockPath := indexPath + ".lock"
		_, lockErr := os.Lstat(lockPath)
		age := lockAge(lockPath, o.Now)
		switch {
		case hookIndex != "":
			// A commit is running, but the index was already gone when it started:
			// it would record every tracked file as deleted.
			f := Finding{Code: CodeIndexMissing, Severity: Damage, Path: rel(indexPath), Details: map[string]string{},
				Message: "the index was missing when this commit started; committing now records every tracked file as deleted"}
			if noObjects {
				f.Details["verified"] = "false"
				add(f)
			} else if has, err := verify.HasCommits(ctx, g, r); err != nil {
				f.Details["verified"] = "false"
				f.Details["reason"] = err.Error()
				add(f)
			} else if has {
				add(f)
			}
		case lockErr == nil && !dataless(lockPath) && holds(o.Now.Add(-age)):
			add(Finding{Code: CodeLockInUse, Severity: Info, Path: rel(lockPath),
				Message: "the index is missing and index.lock exists while git is running, as during a checkout or a first add"})
		case lockErr == nil && !dataless(lockPath) && age < o.StaleAfter && age >= -futureSlack:
			add(Finding{Code: CodeLockRecent, Severity: Info, Path: rel(lockPath),
				Message: "the index is missing and index.lock was written moments ago; run the check again if it stays"})
		case lockErr == nil && !dataless(lockPath):
			f := indexCandidate(ctx, g, r, CodeOrphanIndexLock, lockPath, rel(lockPath),
				"the index is missing but index.lock was left behind", noObjects || split)
			markUnchecked(f, noObjects, split)
			add(f)
		case worktreeEmpty(r.Worktree):
			add(Finding{Code: CodeNoCheckout, Severity: Info, Path: rel(indexPath),
				Message: "no index and an empty working tree, as after 'git clone --no-checkout'"})
		case noObjects:
			// Resolving HEAD reads objects that may exist only in the cloud or the remote.
			add(Finding{Code: CodeIndexMissing, Severity: Damage, Path: rel(indexPath),
				Message: "the index is missing; git will show every tracked file as deleted",
				Details: map[string]string{"verified": "false"}})
		default:
			hasCommits, err := verify.HasCommits(ctx, g, r)
			switch {
			case err != nil:
				add(Finding{Code: CodeIndexMissing, Severity: Damage, Path: rel(indexPath),
					Message: "the index is missing; git will show every tracked file as deleted",
					Details: map[string]string{"verified": "false", "reason": err.Error()}})
			case hasCommits:
				add(Finding{Code: CodeIndexMissing, Severity: Damage, Path: rel(indexPath),
					Message: "the index is missing; git will show every tracked file as deleted"})
			default:
				// HEAD may name a commit that is not here (HEAD_COMMIT_MISSING):
				// the branch has history, so the missing index is damage too.
				if res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "-q", "HEAD"); err == nil && res.ExitCode == 0 {
					add(Finding{Code: CodeIndexMissing, Severity: Damage, Path: rel(indexPath),
						Message: "the index is missing; git will show every tracked file as deleted", Details: map[string]string{"headCommit": "missing"}})
				}
			}
		}
	}

	// An index that lists nothing while HEAD has files: git rebuilds an empty
	// index when the sync client deleted it, and a commit from it deletes
	// everything. Not when HEAD's files are gone from the working tree too
	// (git rm -r .), and not for the split part of a split index (a link
	// extension), whose header counts only that part. Without a checksum
	// (index.skipHash) the header is read all the same.
	emptyCand := hookIndex
	if emptyCand == "" && indexExists {
		emptyCand = indexPath
	}
	// In a sparse checkout, files outside the sparse set are missing from
	// disk on purpose: their absence is no sign of a deliberate deletion.
	sparse := sparseCheckout(ctx, g, r)
	if emptyCand != "" && !dataless(emptyCand) && !hasCode(rep.Findings, CodeIndexMissing) && indexIsEmpty(emptyCand, r.ObjectFormat) {
		msg := "the index lists no files while HEAD has some; committing now records every tracked file as deleted"
		if noObjects {
			// HEAD's tree cannot be read here; a branch that has commits is
			// enough (reading the ref reads no object).
			if res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "-q", "HEAD"); err == nil && res.ExitCode == 0 {
				add(Finding{Code: CodeIndexEmpty, Severity: Damage, Path: rel(emptyCand), Message: msg,
					Details: map[string]string{"verified": "false"}})
			}
		} else {
			res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "-q", "HEAD^{tree}")
			if tree := strings.TrimSpace(res.Stdout); err == nil && res.ExitCode == 0 && tree != emptyTree(r.ObjectFormat) && (r.WorktreeGuessed || sparse || headFilesPresent(ctx, g, r)) {
				add(Finding{Code: CodeIndexEmpty, Severity: Damage, Path: rel(emptyCand), Message: msg})
			}
		}
	}

	// An index that lacks HEAD files still in the working tree: git rebuilt
	// it from the files staged since the sync client deleted it, and a commit
	// from it deletes the rest.
	if emptyCand != "" && !noObjects && !dataless(emptyCand) && !hasCode(rep.Findings, CodeIndexMissing) && !hasCode(rep.Findings, CodeIndexEmpty) && !hasCode(rep.Findings, CodeIndexInvalid) {
		if f, ok := indexDrops(ctx, g, r, emptyCand, rel(emptyCand), worktreeKnown, sparse); ok {
			add(f)
		}
		if f, ok := stagedCopies(ctx, g, r, emptyCand, rel(emptyCand)); ok {
			add(f)
		}
	}

	// HEAD's commit must exist: a sync client can deliver a branch ref before
	// the objects it names.
	if !noObjects {
		if res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "-q", "HEAD"); err == nil && res.ExitCode == 0 {
			oid := strings.TrimSpace(res.Stdout)
			if res, err := g.Run(ctx, r.Worktree, r.Env(), "cat-file", "-e", oid); err == nil {
				switch res.ExitCode {
				case 0:
				case 1:
					add(Finding{Code: CodeHEADCommitMissing, Severity: Damage, Details: map[string]string{"commit": oid},
						Message: "HEAD names a commit that is not in the repository; git status and git log fail"})
				default:
					add(Finding{Code: CodeHEADCommitMissing, Severity: Info, Details: map[string]string{"commit": oid, "verified": "false", "reason": strings.TrimSpace(res.Stderr)},
						Message: "could not tell whether HEAD's commit is present"})
				}
			}
		}
	}

	for _, c := range sr.Copies {
		abs := filepath.Join(r.CommonDir, filepath.FromSlash(c.Rel))
		base := map[string]string{"kind": string(c.Kind), "original": c.OriginalRel, "originalExists": strconv.FormatBool(c.OriginalExists)}
		switch {
		case !c.IsDir && isIndexRel(c.OriginalRel) && !dataless(abs):
			ours := strings.TrimSuffix(c.OriginalRel, ".lock") == relTo(r.CommonDir, indexPath)
			f := indexCandidate(ctx, g, r, CodeIndexCopy, abs, c.Rel, fmt.Sprintf("%s conflict copy of %s", c.Kind, c.OriginalRel), noObjects || split || !ours)
			if !ours {
				f.Details["otherWorktree"] = "true"
			}
			markUnchecked(f, noObjects, split)
			for k, v := range base {
				f.Details[k] = v
			}
			add(f)
		case !c.IsDir && strings.HasPrefix(c.OriginalRel, "refs/") && r.RefStorage == "files" && !dataless(abs):
			f := Finding{Code: CodeRefCopy, Severity: Damage, Path: c.Rel, Details: base,
				Message: fmt.Sprintf("%s conflict copy of %s", c.Kind, c.OriginalRel)}
			if noObjects {
				f.Details["verified"] = "false"
			} else if st, err := verify.CheckRefCopy(ctx, g, r, abs, c.OriginalRel); err != nil {
				// One unreadable commit must not hide every other finding.
				f.Details["relation"] = "unverifiable"
				f.Details["reason"] = err.Error()
			} else {
				f.Details["relation"] = string(st.Relation)
				f.Details["copyCommit"] = st.CopyOID
				f.Details["refCommit"] = st.OriginalOID
				switch st.Relation {
				case verify.RelCopyNewer, verify.RelDiverged:
					f.Message += "; the copy points to commits the branch does not have"
				case verify.RelNoOriginal:
					f.Message += "; the branch itself is missing, so this copy may be the only pointer to its commits"
				}
			}
			add(f)
		default:
			what := "file"
			if c.IsDir {
				what = "directory"
			}
			add(Finding{Code: CodeCopy, Severity: Damage, Path: c.Rel, Details: base,
				Message: fmt.Sprintf("%s conflict copy of %s %s", c.Kind, what, c.OriginalRel)})
		}
	}

	for _, l := range sr.Locks {
		if l.Rel == relTo(r.CommonDir, indexPath)+".lock" && !indexExists {
			continue // reported as ORPHAN_INDEX_LOCK
		}
		age := l.Age.Round(time.Second).String()
		switch {
		case holds(o.Now.Add(-l.Age)):
			add(Finding{Code: CodeLockInUse, Severity: Info, Path: l.Rel, Message: "lock file while git is running", Details: map[string]string{"age": age}})
		case l.Age >= o.StaleAfter || l.Age < -futureSlack: // a future mtime: another device's clock
			add(Finding{Code: CodeStaleLock, Severity: Damage, Path: l.Rel, Details: map[string]string{"age": age},
				Message: "git lock that no process named git on this machine can hold; before removing it, make sure no git uses this repository in a container, on another machine sharing the folder, under another name, or inside an app (libgit2, JGit)"})
		default:
			add(Finding{Code: CodeLockRecent, Severity: Info, Path: l.Rel, Message: "recent lock file", Details: map[string]string{"age": age}})
		}
	}

	switch {
	case o.Deep && noObjects:
		add(Finding{Code: CodeDeepSkipped, Severity: Info, Message: "deep check skipped: it would download files that exist only in the cloud"})
	case o.Deep && fsckWrites(ctx, g, r, o.Flags) != "":
		add(Finding{Code: CodeDeepSkipped, Severity: Info,
			Message: "deep check skipped: git fsck reads every worktree's index, and here that would write (" + fsckWrites(ctx, g, r, o.Flags) + ")"})
	case o.Deep:
		if f, ok := fsck(ctx, g, r, o.FsckTimeout); ok {
			add(f)
		}
	}

	sort.SliceStable(rep.Findings, func(i, j int) bool {
		a, b := rep.Findings[i], rep.Findings[j]
		if a.Severity != b.Severity {
			return a.Severity == Damage
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Path < b.Path
	})
	return rep, nil
}

// journalFindings reads this device's journal of changes for r: a change that
// did not finish, or a move-out whose .git came back as a directory (the sync
// client restoring it, so that two git directories exist). A journal that
// cannot be read gives no finding here; the change commands refuse on it.
func journalFindings(o Options, r repo.Repo) []Finding {
	if o.JournalDir == "" {
		return nil
	}
	store := journal.Store{Dir: o.JournalDir}
	id := state.RepoID(r.Worktree, o.Home)
	var out []Finding
	// Unfinished changes: of this repository, and of any folder from the
	// checked path up (matched by inode): a change that left a folder without
	// .git makes git find an enclosing repository instead.
	open, _ := store.Unfinished(id)
	if abs, err := filepath.Abs(o.Path); err == nil {
		more, _, _ := store.UnfinishedAt(abs)
		open = append(open, more...)
	}
	seen := map[string]bool{}
	for _, e := range open {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		command := map[string]string{"move-in": "undo", "unfix": "undo", "fix": "fix"}[e.Kind]
		if command == "" {
			command = "move-out"
		}
		out = append(out, Finding{Code: CodeChangeUnfinished, Severity: Damage, Path: e.Worktree,
			Details: map[string]string{"journal": e.ID, "change": e.Kind, "phase": string(e.Phase)},
			Message: fmt.Sprintf("a gitbrella %s did not finish; run 'gitbrella %s --apply' in %s to finish or roll it back before using git there", e.Kind, command, e.Worktree)})
	}
	// A move (or an attach to a moved repository) whose .git is a directory
	// again: the sync client brought the old one back, two now exist.
	var last journal.Entry
	for _, kind := range []string{"move-out", "attach"} {
		if e, ok, err := store.LastDone(id, kind); err == nil && ok && e.Started.After(last.Started) {
			last = e
		}
	}
	if last.ID == "" {
		return out
	}
	dotgit := filepath.Join(r.Worktree, ".git")
	if info, err := os.Lstat(dotgit); err == nil && info.IsDir() {
		out = append(out, Finding{Code: CodeGitDirReturned, Severity: Damage, Path: dotgit,
			Details: map[string]string{"movedTo": last.Data["target"], "journal": last.ID, "movedAt": last.Started.Format(time.RFC3339)},
			Message: fmt.Sprintf("this repository's git directory lives at %s, but .git is a directory again (the sync client may have brought it back), so two git directories exist; do not commit until you have compared them (git --git-dir=%s log --all) and kept one", last.Data["target"], shq.Quote(last.Data["target"]))})
	}
	return out
}

// stagedCopies finds files the index at p adds to HEAD that are sync
// conflict copies of tracked files: the name, or the name of a directory
// above it, follows a client's copy scheme ("notes 2.md", "src 2/a.go",
// "a.sync-conflict-...") and the original path is still in the index (a
// deliberate rename to such a name drops the original). A commit now would
// add them, which is how sync copies end up in history.
// Only names are compared. A split index lists only some entries: skipped.
func stagedCopies(ctx context.Context, g *gitx.Runner, r repo.Repo, p, relPath string) (Finding, bool) {
	ix, err := verify.ParseIndexFile(p, r.ObjectFormat)
	if err != nil || ix.Split() {
		return Finding{}, false
	}
	tracked := map[string]bool{}
	res, err := g.Run(ctx, r.Worktree, r.Env(), "ls-tree", "-r", "-z", "--name-only", "--full-tree", "HEAD")
	if err == nil && res.ExitCode == 0 {
		for _, q := range strings.Split(strings.TrimRight(res.Stdout, "\x00"), "\x00") {
			tracked[q] = true
		}
	}
	added := []string{}
	staged := map[string]bool{}
	for _, q := range ix.Paths {
		if strings.HasSuffix(q, "/") { // a sparse directory entry
			continue
		}
		staged[q] = true
		if !tracked[q] && !ix.IntentToAdd[q] { // git add -N: a commit adds no content
			added = append(added, q)
		}
	}
	var copies []string
	for _, q := range added {
		parts := strings.Split(q, "/")
		for i, part := range parts {
			m, ok := cloud.MatchCopyName(part)
			if !ok || m.Original == part {
				continue
			}
			at := func(name string) string {
				return strings.Join(append(append(append([]string{}, parts[:i]...), name), parts[i+1:]...), "/")
			}
			// iCloud numbers from 2; with a " 1" twin present the numbers
			// are an app's ("Untitled 1.md", "Untitled 2.md"), not iCloud's.
			if m.Kind == cloud.KindICloud && staged[at(icloudNumbered(part, 1))] {
				continue
			}
			if staged[at(m.Original)] { // still listed: a copy beside it, not a rename
				copies = append(copies, q)
				break
			}
		}
	}
	if len(copies) == 0 {
		return Finding{}, false
	}
	sample := copies
	if len(sample) > 10 {
		sample = sample[:10]
	}
	return Finding{Code: CodeStagedCopy, Severity: Damage, Path: copies[0],
		Details: map[string]string{"count": strconv.Itoa(len(copies)), "sample": strings.Join(sample, ", "), "index": relPath},
		Message: fmt.Sprintf("%d staged file(s) look like sync conflict copies of tracked files; a commit now adds them to history. Unstage them (git restore --staged <file>), or commit with --no-verify if they are meant", len(copies))}, true
}

// icloudNumbered turns an iCloud-style name ("notes 2.md") into the same name
// with another number ("notes 1.md").
func icloudNumbered(name string, n int) string {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if i := strings.LastIndexByte(base, ' '); i >= 0 {
		base = base[:i]
	}
	return fmt.Sprintf("%s %d%s", base, n, ext)
}

// reftableUnlisted finds ref tables in .git/reftable that tables.list does
// not name: a sync client carried them in from another device whose list
// lost the conflict (its list is a copy git never reads). The next git
// pack-refs or maintenance deletes every unlisted table older than the
// stack, and with it that device's branches. Not while git writes the list.
func reftableUnlisted(r repo.Repo) (Finding, bool) {
	stacks := []string{"reftable"} // and each linked worktree's own
	if ws, err := os.ReadDir(filepath.Join(r.CommonDir, "worktrees")); err == nil {
		for _, w := range ws {
			if w.IsDir() {
				stacks = append(stacks, "worktrees/"+w.Name()+"/reftable")
			}
		}
	}
	var unlisted []string
	for _, st := range stacks {
		unlisted = append(unlisted, unlistedTables(filepath.Join(r.CommonDir, filepath.FromSlash(st)), st)...)
	}
	if len(unlisted) == 0 {
		return Finding{}, false
	}
	return Finding{Code: CodeReftableUnlisted, Severity: Damage, Path: unlisted[0],
		Details: map[string]string{"count": strconv.Itoa(len(unlisted)), "sample": strings.Join(unlisted, ", ")},
		Message: "ref tables git does not list, carried in by a sync client from another device; the next git pack-refs or maintenance deletes them and that device's branches with them. Copy .git/reftable somewhere safe before running git here"}, true
}

// unlistedTables lists the tables in one reftable stack (dir, shown as rel)
// that its tables.list does not name. Nothing while git writes the stack:
// its list is locked, the list changed while the directory was read, or a
// table is locked (a compaction drops old tables from the list before it
// removes them, each still holding its lock).
func unlistedTables(dir, rel string) []string {
	if _, err := os.Lstat(filepath.Join(dir, "tables.list.lock")); err == nil {
		return nil
	}
	b, err := readSmall(filepath.Join(dir, "tables.list"))
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	if again, err := readSmall(filepath.Join(dir, "tables.list")); err != nil || !bytes.Equal(again, b) {
		return nil
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		listed[strings.TrimSpace(line)] = true
	}
	present := map[string]bool{}
	for _, e := range entries {
		present[e.Name()] = true
	}
	var out []string
	for _, e := range entries {
		if n := e.Name(); strings.HasSuffix(n, ".ref") && !listed[n] && !present[n+".lock"] && e.Type().IsRegular() {
			out = append(out, rel+"/"+n)
		}
	}
	return out
}

// gcFindings says why git's automatic gc may not run: a gc.pid another
// machine wrote (git takes it for a gc running there, for 12 hours, without
// checking), or a gc.log less than a day old (git waits until it is older).
func gcFindings(r repo.Repo, now time.Time) []Finding {
	var out []Finding
	if info, err := os.Lstat(filepath.Join(r.CommonDir, "gc.pid")); err == nil && now.Sub(info.ModTime()) < 12*time.Hour {
		if b, err := readSmall(filepath.Join(r.CommonDir, "gc.pid")); err == nil {
			f := strings.Fields(string(b))
			// git reads "<pid> <host>" and ignores a file it cannot parse
			if host, _ := os.Hostname(); len(f) == 2 && isPID(f[0]) && f[1] != host {
				out = append(out, Finding{Code: CodeGCPIDForeign, Severity: Info, Path: "gc.pid", Details: map[string]string{"host": f[1]},
					Message: fmt.Sprintf("gc.pid names another machine (%s): git takes it for a gc running there and skips its own automatic gc until the file is 12 hours old", f[1])})
			}
		}
	}
	if info, err := os.Lstat(filepath.Join(r.CommonDir, "gc.log")); err == nil && info.Size() > 0 && now.Sub(info.ModTime()) < 24*time.Hour {
		out = append(out, Finding{Code: CodeGCLogFresh, Severity: Info, Path: "gc.log",
			Message: "gc.log holds the last automatic gc's errors and is less than a day old, so git skips automatic gc until it is older or removed; read it first"})
	}
	return out
}

// isPID reports whether s is a process id as git writes it.
func isPID(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

// syncedHooks reports code git runs from a synced folder outside the git
// directory: core.hooksPath, or a core.fsmonitor command. A change made
// there on another device runs on this one.
func syncedHooks(ctx context.Context, g *gitx.Runner, r repo.Repo, home, goos string) []Finding {
	var out []Finding
	for _, key := range []string{"core.hooksPath", "core.fsmonitor"} {
		res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--get", key)
		if err != nil || res.ExitCode != 0 {
			continue
		}
		v := strings.TrimSpace(res.Stdout)
		if key == "core.fsmonitor" {
			switch strings.ToLower(v) {
			case "true", "false", "yes", "no", "on", "off", "1", "0", "":
				continue // the built-in daemon or none, not a command
			}
			v, _, _ = strings.Cut(v, " ")
		}
		if v == "" {
			continue
		}
		p := v
		if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		} else if !filepath.IsAbs(p) {
			p = filepath.Join(r.Worktree, p)
		}
		if within(p, r.CommonDir) {
			continue // the git directory's own hooks: IN_CLOUD_FOLDER covers a synced one
		}
		if prov := cloud.DetectAt(p, home, goos); prov != cloud.None {
			out = append(out, Finding{Code: CodeSyncedHooks, Severity: Info, Path: p, Details: map[string]string{"config": key, "provider": string(prov)},
				Message: fmt.Sprintf("git runs code from %s (%s), a folder %s syncs: a change there on another device runs on this one", p, key, prov)})
		}
	}
	return out
}

// within reports whether p is dir or below it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// tempPIDRe takes the process id from git's per-process temporary names.
var tempPIDRe = regexp.MustCompile(`(?:next-index-|index\.stash\.)([0-9]+)`)

// indexDrops compares the index at p with HEAD: a HEAD file the index no
// longer lists while it is still in the working tree is recorded as deleted
// by a commit, as after git rebuilt the index from the files staged since a
// sync client deleted it. Not a drop: the file is gone from disk too (deleted
// on purpose), it is the same file under a new name (case, Unicode form), a
// directory above it is now an entry (a symlink, a submodule), or it is now a
// directory of entries. A sparse directory entry covers HEAD's files below it
// when it names HEAD's tree there; an older tree drops the files it lacks.
// In a sparse checkout every file counts without the disk check: git's
// patterns decide what is on disk there, so absence proves nothing. So does
// every file when the worktree is only a guess.
func indexDrops(ctx context.Context, g *gitx.Runner, r repo.Repo, p, relPath string, worktreeKnown, sparse bool) (Finding, bool) {
	checkDisk := worktreeKnown && !sparse
	ix, err := verify.ParseIndexFile(p, r.ObjectFormat)
	if err != nil {
		return Finding{Code: CodeIndexInvalid, Severity: Damage, Path: relPath, Message: "the index could not be read: " + err.Error()}, true
	}
	if ix.Split() {
		return Finding{}, false // the split part lists only some entries
	}
	res, err := g.Run(ctx, r.Worktree, r.Env(), "ls-tree", "-r", "-t", "-z", "--full-tree", "HEAD")
	if err != nil || res.ExitCode != 0 {
		// An unborn branch has no files to lose. Otherwise HEAD's files
		// cannot be listed (its tree is missing, git timed out): no answer
		// is not "no problems".
		// Only a clean "no such ref" (exit 1) is an unborn branch; an error
		// or a timeout here is no answer either.
		h, herr := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "-q", "HEAD")
		if herr == nil && h.ExitCode == 1 {
			return Finding{}, false
		}
		if herr == nil && h.ExitCode == 0 {
			// HEAD's commit is not here: HEAD_COMMIT_MISSING says so, and
			// nothing can be compared until it is fetched again.
			if c, cerr := g.Run(ctx, r.Worktree, r.Env(), "cat-file", "-e", strings.TrimSpace(h.Stdout)); cerr == nil && c.ExitCode == 1 {
				return Finding{}, false
			}
		}
		reason := strings.TrimSpace(res.Stderr)
		if err != nil {
			reason = err.Error()
		}
		if herr != nil {
			reason += "; " + herr.Error()
		}
		return Finding{Code: CodeIndexShrunk, Severity: Damage, Path: relPath,
			Details: map[string]string{"verified": "false", "reason": reason},
			Message: "HEAD's files could not be listed, so the index was not compared with them; a commit may record files as deleted"}, true
	}
	var headFiles []string
	headTrees := map[string]string{}
	gitlinks := map[string]bool{} // submodules: a directory on disk is the entry
	for _, rec := range strings.Split(strings.TrimRight(res.Stdout, "\x00"), "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) == 3 && f[1] == "tree" {
			headTrees[path] = f[2]
			continue
		}
		if len(f) == 3 && f[1] == "commit" {
			gitlinks[path] = true
		}
		headFiles = append(headFiles, path)
	}
	listed := make(map[string]bool, len(ix.Paths))
	entryDirs := map[string]bool{} // directories holding index entries
	for _, q := range ix.Paths {
		if strings.HasSuffix(q, "/") {
			continue
		}
		listed[q] = true
		for d := path.Dir(q); d != "."; d = path.Dir(d) {
			if entryDirs[d] {
				break
			}
			entryDirs[d] = true
		}
	}
	// HEAD files a sparse directory entry stands for: all of them when it
	// names HEAD's tree, else those its own (older) tree has.
	covered := map[string]bool{}
	for d, oid := range ix.SparseDirs {
		dir := strings.TrimSuffix(d, "/")
		if headTrees[dir] == oid {
			for _, q := range headFiles {
				if strings.HasPrefix(q, d) {
					covered[q] = true
				}
			}
			continue
		}
		old, err := g.Run(ctx, r.Worktree, r.Env(), "ls-tree", "-r", "-z", "--name-only", oid)
		if err != nil || old.ExitCode != 0 {
			continue // tree unreadable: its files count as dropped
		}
		for _, q := range strings.Split(strings.TrimRight(old.Stdout, "\x00"), "\x00") {
			if q != "" {
				covered[d+q] = true
			}
		}
	}
	// The same file under another name (a case-only rename, NFD to NFC on
	// APFS) is the same file on disk.
	renamed := map[[2]uint64]bool{}
	if checkDisk {
		inHead := make(map[string]bool, len(headFiles))
		for _, q := range headFiles {
			inHead[q] = true
		}
		for q := range listed {
			if !inHead[q] {
				if id, ok := fileID(filepath.Join(r.Worktree, filepath.FromSlash(q))); ok && id != ([2]uint64{}) {
					renamed[id] = true
				}
			}
		}
	}
	// A type change is excused only when the disk agrees: an older index can
	// list as a file what is a directory of files now, and git add -A takes a
	// repository created inside a tracked directory for a submodule.
	var submodules map[string]bool
	typeChanged := func(q string) bool {
		if !checkDisk {
			return false
		}
		abs := func(p string) string { return filepath.Join(r.Worktree, filepath.FromSlash(p)) }
		if entryDirs[q] {
			fi, err := os.Lstat(abs(q))
			return err == nil && fi.IsDir() // a file that became a directory of entries
		}
		a := listedAncestor(q, listed)
		if a == "" {
			return false
		}
		fi, err := os.Lstat(abs(a))
		switch {
		case err != nil, !fi.IsDir():
			return true // a symlink or a file now: HEAD's files below it cannot be on disk
		default:
			if submodules == nil {
				submodules = submodulePaths(ctx, g, r)
			}
			_, gerr := os.Lstat(filepath.Join(abs(a), ".git"))
			return gerr == nil && submodules[a] // a submodule, not just a repository inside
		}
	}
	var gone []string
	for _, q := range headFiles {
		if listed[q] || covered[q] || typeChanged(q) {
			continue
		}
		if checkDisk {
			abs := filepath.Join(r.Worktree, filepath.FromSlash(q))
			if fi, err := os.Lstat(abs); err == nil && fi.IsDir() && !gitlinks[q] {
				continue // a directory under its name (or its name in another case): the file is gone
			}
			id, ok := fileID(abs)
			if !ok || (id != ([2]uint64{}) && renamed[id]) {
				continue // deleted on purpose, or renamed
			}
		}
		gone = append(gone, q)
	}
	if len(gone) == 0 {
		return Finding{}, false
	}
	sample := gone
	if len(sample) > 10 {
		sample = sample[:10]
	}
	msg := fmt.Sprintf("the index no longer lists %d of HEAD's %d files that are still in the working tree; committing now records them as deleted. To stop tracking them on purpose, commit with --no-verify", len(gone), len(headFiles))
	if sparse {
		msg = fmt.Sprintf("the index no longer lists %d of HEAD's %d files; in a sparse checkout a file missing from disk is no sign it was deleted on purpose, so committing now may record lost files as deleted. To remove them on purpose, commit with --no-verify", len(gone), len(headFiles))
	}
	return Finding{Code: CodeIndexShrunk, Severity: Damage, Path: relPath,
		Details: map[string]string{"dropped": strconv.Itoa(len(gone)), "headFiles": strconv.Itoa(len(headFiles)), "sample": strings.Join(sample, ", ")},
		Message: msg}, true
}

// listedAncestor returns the nearest directory above q that the index lists
// as an entry of its own, or "".
func listedAncestor(q string, listed map[string]bool) string {
	for d := path.Dir(q); d != "."; d = path.Dir(d) {
		if listed[d] {
			return d
		}
	}
	return ""
}

// submodulePaths lists the submodule paths .gitmodules in the working tree
// declares. When it cannot be read, none is declared.
func submodulePaths(ctx context.Context, g *gitx.Runner, r repo.Repo) map[string]bool {
	paths := map[string]bool{}
	res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--file", ".gitmodules", "--get-regexp", `^submodule\..*\.path$`)
	if err != nil || res.ExitCode != 0 {
		return paths
	}
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if _, v, ok := strings.Cut(line, " "); ok {
			paths[strings.Trim(v, "/")] = true
		}
	}
	return paths
}

// sparseCheckout reports whether the worktree is a sparse checkout. Its
// patterns are not read: modelling git's matching let lost files through.
func sparseCheckout(ctx context.Context, g *gitx.Runner, r repo.Repo) bool {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--type=bool", "--get", "core.sparseCheckout")
	return err == nil && strings.TrimSpace(res.Stdout) == "true"
}

// indexIsEmpty reports whether the index at p lists no files: its header
// counts none (checksum verified, or disabled by index.skipHash) and it is not
// the split part of a split index.
func indexIsEmpty(p, objectFormat string) bool {
	st := verify.CheckIndexFile(p, objectFormat)
	if !(st.Valid || st.Unverifiable) || st.Entries != 0 {
		return false
	}
	ix, err := verify.ParseIndexFile(p, objectFormat)
	return err != nil || !ix.Split() // unreadable extensions: block rather than pass
}

// indexCandidate validates an index-like file and, unless skipHEAD is set,
// compares it with HEAD of the checked worktree.
func indexCandidate(ctx context.Context, g *gitx.Runner, r repo.Repo, code, abs, relPath, msg string, skipHEAD bool) Finding {
	f := Finding{Code: code, Severity: Damage, Path: relPath, Message: msg, Details: map[string]string{}}
	st := verify.CheckIndexFile(abs, r.ObjectFormat)
	f.Details["validIndex"] = strconv.FormatBool(st.Valid)
	if st.Unverifiable {
		f.Details["validIndex"] = "unverified" // no checksum to verify (index.skipHash)
	}
	if st.Reason != "" {
		f.Details["reason"] = st.Reason
	}
	switch {
	case skipHEAD:
		f.Details["equalsHEAD"] = "not checked"
	case st.Valid || st.Unverifiable:
		eq, err := verify.IndexEqualsHEAD(ctx, g, r, abs)
		if err != nil {
			f.Details["equalsHEAD"] = "unknown: " + err.Error()
		} else {
			f.Details["equalsHEAD"] = strconv.FormatBool(eq)
		}
	}
	return f
}

// fsckWrites says why git fsck, which reads the index of every worktree,
// would write here, or "". Reading a split index refreshes its sharedindex
// file; reading a sparse index can write tree objects. Any doubt counts.
func fsckWrites(ctx context.Context, g *gitx.Runner, r repo.Repo, fr scan.FlagReader) string {
	for _, key := range []string{"core.splitIndex", "index.sparse"} {
		if res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--type=bool", "--get", key); err == nil && strings.TrimSpace(res.Stdout) == "true" {
			return key + " is set"
		}
	}
	dirs := []string{r.CommonDir, r.GitDir}
	admins, _ := os.ReadDir(filepath.Join(r.CommonDir, "worktrees"))
	for _, a := range admins {
		dirs = append(dirs, filepath.Join(r.CommonDir, "worktrees", a.Name()))
	}
	for _, d := range dirs {
		if m, _ := filepath.Glob(filepath.Join(d, "sharedindex.*")); len(m) > 0 {
			return "a split index is in use"
		}
		idx := filepath.Join(d, "index")
		if _, err := os.Lstat(idx); err != nil {
			continue
		}
		if dl, _ := fr.IsDataless(idx); dl {
			return "an index exists only in the cloud"
		}
		ix, err := verify.ParseIndexFile(idx, r.ObjectFormat)
		switch {
		case err != nil:
			return "an index could not be read"
		case ix.Split():
			return "a split index is in use"
		case ix.Sparse():
			return "a sparse index is in use"
		}
	}
	return ""
}

// fsck runs a full git fsck (reads every object) and returns a finding when it fails.
func fsck(ctx context.Context, g *gitx.Runner, r repo.Repo, timeout time.Duration) (Finding, bool) {
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	long := *g
	long.Timeout = timeout
	res, err := long.Run(ctx, r.Worktree, r.Env(), "fsck", "--no-dangling", "--no-progress")
	if err != nil {
		return Finding{Code: CodeFsck, Severity: Info, Message: "git fsck did not finish: " + err.Error()}, true
	}
	if res.ExitCode == 0 {
		return Finding{}, false
	}
	lines := strings.Split(strings.TrimSpace(res.Stderr+"\n"+res.Stdout), "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	return Finding{Code: CodeFsck, Severity: Damage, Message: "git fsck reported errors",
		Details: map[string]string{"exitCode": strconv.Itoa(res.ExitCode), "first": strings.Join(lines, " | ")}}, true
}

// isIndexRel reports whether rel names an index file: the main worktree's
// "index" or a linked worktree's "worktrees/<name>/index" (lock suffix ignored).
func isIndexRel(rel string) bool {
	rel = strings.TrimSuffix(rel, ".lock")
	if rel == "index" {
		return true
	}
	parts := strings.Split(rel, "/")
	return len(parts) == 3 && parts[0] == "worktrees" && parts[2] == "index"
}

// worktreeDirs lists every directory where a git process works on this
// repository: the checked worktree, the git directory, the main worktree and
// each linked worktree recorded in worktrees/*/gitdir.
func worktreeDirs(r repo.Repo, fr scan.FlagReader) []string {
	dirs := []string{r.Worktree, r.CommonDir}
	if filepath.Base(r.CommonDir) == ".git" {
		dirs = append(dirs, filepath.Dir(r.CommonDir))
	}
	admins, _ := os.ReadDir(filepath.Join(r.CommonDir, "worktrees"))
	for _, a := range admins {
		p := filepath.Join(r.CommonDir, "worktrees", a.Name(), "gitdir")
		if d, _ := fr.IsDataless(p); d {
			continue
		}
		if b, err := os.ReadFile(p); err == nil {
			dirs = append(dirs, filepath.Dir(strings.TrimSpace(string(b))))
		}
	}
	return dirs
}

// partialClone reports whether objects may be missing on purpose and fetched
// on demand from a promisor remote.
func partialClone(ctx context.Context, g *gitx.Runner, r repo.Repo) bool {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--get-regexp", `^(extensions\.partialclone|remote\..*\.promisor)$`)
	return err == nil && res.ExitCode == 0
}

// splitIndex reports whether the repository uses a split index. git refreshes
// sharedindex.* timestamps whenever it reads such an index, so index copies
// are not compared with HEAD there.
func splitIndex(r repo.Repo) bool {
	for _, dir := range []string{r.GitDir, r.CommonDir} {
		if m, _ := filepath.Glob(filepath.Join(dir, "sharedindex.*")); len(m) > 0 {
			return true
		}
	}
	return false
}

// markUnchecked records why an index candidate was not compared with HEAD.
func markUnchecked(f Finding, noObjects, split bool) {
	switch {
	case noObjects:
		f.Details["verified"] = "false"
	case split:
		f.Details["equalsHEAD"] = "not checked: split index"
	}
}

// lockAge returns how long ago path was last written.
func lockAge(path string, now time.Time) time.Duration {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	return now.Sub(info.ModTime())
}

// gitDirSynced reports whether the sync client also syncs the git directory.
// It does when the directory is inside the working tree or another synced
// folder, unless iCloud skips it because of a ".nosync" name.
func gitDirSynced(r repo.Repo, provider cloud.Provider, home, goos string) bool {
	if (provider == cloud.ICloud || provider == cloud.ICloudDesktopDocuments) && cloud.IsNoSync(r.CommonDir) {
		return false
	}
	return relTo(r.Worktree, r.CommonDir) != r.CommonDir || cloud.DetectAt(r.CommonDir, home, goos) != cloud.None
}

func relTo(base, p string) string {
	if r, err := filepath.Rel(base, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return p
}

// worktreeEmpty reports whether dir holds nothing but the .git entry.
func worktreeEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() != ".git" && e.Name() != ".DS_Store" && !cloud.IsSyncMetadata(e.Name()) {
			return false
		}
	}
	return true
}

// GitDirCopyHint lists entries in dir that look like a renamed or copied git
// directory. It explains "not a git repository" when iCloud renamed .git.
func GitDirCopyHint(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if cloud.IsGitDirCopy(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

// NotRepoHints explains why path is not a git repository when the cause is
// visible on disk: a renamed git directory, git's own files evicted to the
// cloud, or a .git file that points at a directory that is gone. It reads
// metadata only, plus a .git file's single line when that file is local.
func NotRepoHints(path string, fr scan.FlagReader) []string {
	var out []string
	for _, name := range GitDirCopyHint(path) {
		out = append(out, fmt.Sprintf("%q here looks like a renamed or copied git directory; sync clients rename .git when they see a conflict", name))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return out
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real // judge the place git looks at, not where a symlink sits
	}
	sep := string(filepath.Separator)
	if i := strings.Index(abs+sep, sep+".git"+sep); i >= 0 {
		if info, err := os.Lstat(abs[:i] + sep + ".git"); err == nil && info.IsDir() {
			return append(out, fmt.Sprintf("%s is inside the git directory; run the check from the working tree (%s)", abs, abs[:i]))
		}
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		dotgit := filepath.Join(dir, ".git")
		if info, err := os.Lstat(dotgit); err == nil {
			if h := dotGitHint(dotgit, info, fr); h != "" {
				out = append(out, h)
			}
			return out
		}
		if filepath.Dir(dir) == dir {
			return out
		}
	}
}

func dotGitHint(dotgit string, info os.FileInfo, fr scan.FlagReader) string {
	const download = "download it (Finder: Download Now) and run the check again"
	real := dotgit
	if p, err := filepath.EvalSymlinks(dotgit); err == nil {
		real = p
	}
	if d, _ := fr.IsDataless(real); d {
		return fmt.Sprintf("%s exists only in the cloud, so git cannot read it; %s", real, download)
	}
	gitDir := dotgit
	if info.Mode()&os.ModeSymlink != 0 {
		if st, err := os.Stat(dotgit); err == nil {
			info = st // a .git symlink to a directory is that directory
		}
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Sprintf("%s is neither a directory nor a regular file, so git ignores it", dotgit)
	}
	if !info.IsDir() {
		b, err := readSmall(dotgit)
		if err != nil {
			return fmt.Sprintf("%s cannot be read: %v", dotgit, err)
		}
		line, _, _ := strings.Cut(strings.TrimRight(string(b), "\r\n"), "\n")
		target, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "gitdir: ")
		if !ok || target == "" {
			return fmt.Sprintf("%s is not a valid .git file (git expects one line \"gitdir: <path>\")", dotgit)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(dotgit), target)
		}
		if d, _ := fr.IsDataless(target); d {
			return fmt.Sprintf("%s, the git directory .git points to, exists only in the cloud; %s", target, download)
		}
		_, err = os.Stat(target)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return fmt.Sprintf(".git points to %s, which does not exist; the git directory was moved or removed (for example a deleted linked worktree). If another device moved this repository out with gitbrella, run 'gitbrella move-out --attach --apply' here (with --from <remote URL> unless this device was prepared)", target)
		case errors.Is(err, syscall.EDEADLK):
			return fmt.Sprintf("%s, the git directory .git points to, is inside a folder that exists only in the cloud; %s", target, download)
		case err != nil:
			return fmt.Sprintf(".git points to %s, which cannot be read: %v", target, err)
		}
		gitDir = target
	}
	// A linked worktree keeps HEAD in its own git directory and config in the common one.
	commonDir := gitDir
	cloudOnly := []string{}
	if d, _ := fr.IsDataless(filepath.Join(gitDir, "commondir")); d {
		cloudOnly = append(cloudOnly, "commondir")
	} else if b, err := readSmall(filepath.Join(gitDir, "commondir")); err == nil {
		commonDir = strings.TrimSpace(string(b))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(gitDir, commonDir)
		}
	}
	for _, f := range []struct{ name, dir string }{{"HEAD", gitDir}, {"config", commonDir}} {
		if d, _ := fr.IsDataless(filepath.Join(f.dir, f.name)); d {
			cloudOnly = append(cloudOnly, f.name)
		}
	}
	if len(cloudOnly) > 0 {
		where := gitDir
		if commonDir != gitDir {
			where += " and " + commonDir
		}
		return fmt.Sprintf("git's own files (%s) in %s exist only in the cloud, so git cannot read the repository; download them (Finder: Download Now) and run the check again", strings.Join(cloudOnly, ", "), where)
	}
	if info, err := os.Lstat(filepath.Join(gitDir, "HEAD")); errors.Is(err, fs.ErrNotExist) {
		return fmt.Sprintf("%s has no HEAD, so git does not treat it as a repository; a sync client may have renamed or removed it (look for \"HEAD 2\" next to it)", gitDir)
	} else if err == nil && info.Mode().IsRegular() && info.Size() == 0 {
		return fmt.Sprintf("%s/HEAD is empty, so git does not treat it as a repository; a sync client may have emptied it (look for \"HEAD 2\" next to it, or write \"ref: refs/heads/<branch>\" into it)", gitDir)
	}
	// core.worktree in the repository's config, or (extensions.worktreeConfig)
	// in config.worktree
	for _, cfg := range []string{filepath.Join(commonDir, "config"), filepath.Join(gitDir, "config.worktree")} {
		wt := configWorktree(cfg)
		if wt == "" {
			continue
		}
		target := wt
		if !filepath.IsAbs(target) {
			target = filepath.Join(gitDir, target) // relative to the git directory
		}
		if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
			// git config --file with no repository around: git --git-dir would
			// fail on the very folder that is missing
			return fmt.Sprintf("core.worktree in %s points at %s, which does not exist on this machine, so every git command fails; a sync client may have carried another machine's setting. If this folder is the working tree, remove it from outside any repository: cd / && git config --file %s --unset core.worktree", cfg, target, shq.Quote(cfg))
		}
	}
	return ""
}

// configWorktree reads core.worktree from the config file at p, without a
// repository (git config --file reads that file only); "" when unset or
// unreadable.
func configWorktree(p string) string {
	// only a regular file: git would wait on a FIFO for ever
	if info, err := os.Lstat(p); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "config", "--file", p, "--get", "core.worktree")
	// git looks for a repository even here; from inside this one it would
	// fail on the very setting being read
	cmd.Dir = string(filepath.Separator)
	cmd.Env = append(gitx.Env(os.Environ()), "GIT_CEILING_DIRECTORIES="+string(filepath.Separator))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// cloudOnlyIgnoreFile returns the first ignore file that could hold a rule for
// dir and exists only in the cloud: .gitignore files from the working tree root
// down to dir, the repository's info/exclude and core.excludesFile.
func cloudOnlyIgnoreFile(ctx context.Context, g *gitx.Runner, r repo.Repo, dir, home string, fr scan.FlagReader) string {
	files := []string{filepath.Join(r.CommonDir, "info", "exclude")}
	parts := strings.Split(dir, "/")
	for i := 0; i < len(parts); i++ {
		files = append(files, filepath.Join(r.Worktree, filepath.Join(parts[:i]...), ".gitignore"))
	}
	if res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--path", "--get", "core.excludesFile"); err == nil && res.ExitCode == 0 {
		files = append(files, strings.TrimSpace(res.Stdout))
	} else if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		files = append(files, filepath.Join(xdg, "git", "ignore"))
	} else if home != "" {
		files = append(files, filepath.Join(home, ".config", "git", "ignore"))
	}
	for _, f := range files {
		if real, err := filepath.EvalSymlinks(f); err == nil {
			f = real // git reads through symlinks (dotfiles kept in a synced folder)
		}
		if d, _ := fr.IsDataless(f); d {
			return f
		}
	}
	return ""
}

// readSmall reads at most 4 KiB: a .git file is one short line.
func readSmall(path string) ([]byte, error) {
	// only a regular file (a symlink to one is fine): a FIFO would hold the
	// check for ever
	if info, err := os.Stat(path); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is no regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 4096))
}

// processInRepo reports whether git process p works on repository r or on a
// submodule whose git dir lives inside r's (its locks are scanned with r's).
// It finds the repository the way git does, walking up from the process's
// directory to a usable .git, and compares common directories by file
// identity; so relative or moved worktrees, a common directory not named .git
// and differently spelled paths all match. When r cannot be discovered from
// its own worktree (core.worktree), a git in that worktree counts unless it
// works on a repository nested below it. With nothing found it falls back to
// the directories r is known by.
func processInRepo(p procs.Proc, r repo.Repo, fr scan.FlagReader, dirs []string, discoverable bool) bool {
	common, top, ok := commonDirOf(p.Cwd, fr)
	if !ok {
		return procs.Within(p, dirs...)
	}
	if sameFile(common, r.CommonDir) || inside(common, r.CommonDir) {
		return true
	}
	if !discoverable && procs.Within(p, r.Worktree) {
		return !inside(top, r.Worktree) // a repository nested below the worktree is another one
	}
	return false
}

// inside reports whether p lies strictly below dir. Ancestors are compared by
// file identity, so a differently spelled path (case on APFS, a symlink, a
// bind mount) still counts.
func inside(p, dir string) bool {
	d, err := os.Stat(dir)
	if err != nil {
		return false
	}
	p = filepath.Clean(p)
	for q := filepath.Dir(p); ; q = filepath.Dir(q) {
		if a, err := os.Stat(q); err == nil && os.SameFile(a, d) {
			return true
		}
		if filepath.Dir(q) == q {
			return false
		}
	}
}

// commonDirOf finds the common git directory of the repository that a git
// started in dir works on, and the directory where discovery stopped.
// Metadata only, plus .git and commondir files of a few bytes that are not
// cloud-only.
func commonDirOf(dir string, fr scan.FlagReader) (common, top string, ok bool) {
	for d := dir; ; d = filepath.Dir(d) {
		if gd, ok := gitDirAt(d, fr); ok {
			return commonOf(gd, fr), d, true
		}
		if looksLikeGitDir(d) {
			return commonOf(d, fr), d, true // git working inside a git directory
		}
		if filepath.Dir(d) == d {
			return "", "", false
		}
	}
}

// gitDirAt resolves d/.git (directory, symlink or "gitdir:" file) to a git
// directory that has HEAD, the way git's discovery accepts it.
func gitDirAt(d string, fr scan.FlagReader) (string, bool) {
	return gitDirFrom(filepath.Join(d, ".git"), fr)
}

// gitDirFrom resolves p (a directory, a symlink or a "gitdir:" file) to a git
// directory that has HEAD.
func gitDirFrom(p string, fr scan.FlagReader) (string, bool) {
	info, err := os.Stat(p)
	if err != nil {
		return "", false
	}
	gd := p
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return "", false
		}
		if dl, _ := fr.IsDataless(p); dl {
			return "", false
		}
		b, err := readSmall(p)
		if err != nil {
			return "", false
		}
		target, ok := strings.CutPrefix(strings.TrimRight(strings.SplitN(string(b), "\n", 2)[0], "\r"), "gitdir: ")
		if !ok {
			return "", false
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(p), target)
		}
		gd = target
	}
	if !looksLikeGitDir(gd) {
		return "", false
	}
	return gd, true
}

func looksLikeGitDir(d string) bool {
	_, err := os.Lstat(filepath.Join(d, "HEAD"))
	if err != nil {
		return false
	}
	for _, n := range []string{"objects", "commondir"} {
		if _, err := os.Lstat(filepath.Join(d, n)); err == nil {
			return true
		}
	}
	return false
}

// commonOf follows a linked worktree's commondir file.
func commonOf(gd string, fr scan.FlagReader) string {
	p := filepath.Join(gd, "commondir")
	if dl, _ := fr.IsDataless(p); dl {
		return gd
	}
	b, err := readSmall(p)
	if err != nil {
		return gd
	}
	c := strings.TrimSpace(string(b))
	if !filepath.IsAbs(c) {
		c = filepath.Join(gd, c)
	}
	return c
}

func sameFile(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

// hookIndexPath accepts GIT_INDEX_FILE only when it names a file inside this
// repository's git directory, as git sets it for commit hooks.
func hookIndexPath(p string, r repo.Repo) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return ""
	}
	abs = filepath.Join(dir, filepath.Base(abs))
	for _, d := range []string{r.GitDir, r.CommonDir} {
		if relTo(d, abs) != abs {
			return abs
		}
	}
	return ""
}

func hasCode(fs []Finding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

// emptyTree is the id of the tree with no entries.
func emptyTree(objectFormat string) string {
	if objectFormat == "sha256" {
		return "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321"
	}
	return "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
}

// headFilesPresent reports whether any file of HEAD's tree is still in the
// working tree. After a sync client deleted the index the files are all
// there; after 'git rm -r .' they are gone, even where an ignored file
// (.DS_Store, __pycache__) keeps a directory alive. It stops at the first hit.
func headFilesPresent(ctx context.Context, g *gitx.Runner, r repo.Repo) bool {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "ls-tree", "-r", "-z", "--name-only", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return false
	}
	for _, name := range strings.Split(strings.TrimRight(res.Stdout, "\x00"), "\x00") {
		if name == "" {
			continue
		}
		if _, err := os.Lstat(filepath.Join(r.Worktree, name)); err == nil {
			return true
		}
	}
	return false
}

// worktreeConfirmed reports whether a working tree taken from the current
// directory holds HEAD's files. It reads HEAD's tree only where that cannot
// download objects or fetch them; otherwise the guess stays unconfirmed.
func worktreeConfirmed(ctx context.Context, g *gitx.Runner, r repo.Repo, fr scan.FlagReader, dataless bool) bool {
	if dataless || cloudOnlyAlternate(r, fr) != "" || (partialClone(ctx, g, r) && !lazyFetchSafe(ctx, g)) {
		return false
	}
	return headFilesPresent(ctx, g, r)
}

// cloudOnlyAlternate returns an alternate object directory (objects/info/
// alternates, followed to the depth git allows) that holds cloud-only files:
// reading objects through it would fail rather than download, and must not
// read as missing objects. Metadata only, plus each alternates file.
func cloudOnlyAlternate(r repo.Repo, fr scan.FlagReader) string {
	return cloudOnlyAlternateFrom(filepath.Join(r.CommonDir, "objects"), fr, 0)
}

// cloudOnlyAlternateFrom walks alternates once each (git deduplicates them
// too): stores that list each other must not multiply the walk.
func cloudOnlyAlternateFrom(objects string, fr scan.FlagReader, depth int, seen ...*[]os.FileInfo) string {
	var visited *[]os.FileInfo
	if len(seen) > 0 {
		visited = seen[0]
	} else {
		visited = &[]os.FileInfo{}
	}
	if info, err := os.Stat(objects); err == nil {
		for _, v := range *visited {
			if os.SameFile(v, info) {
				return ""
			}
		}
		*visited = append(*visited, info)
	}
	if depth > 5 { // git's own nesting limit
		return ""
	}
	alt := filepath.Join(objects, "info", "alternates")
	if d, _ := fr.IsDataless(alt); d {
		return alt
	}
	b, err := readSmall(alt)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		dir := strings.TrimSpace(line)
		if dir == "" || strings.HasPrefix(dir, "#") {
			continue
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(objects, dir)
		}
		if seenDir(dir, *visited) {
			continue
		}
		if cloudOnlyObjectDir(dir, fr) {
			return dir
		}
		if nested := cloudOnlyAlternateFrom(dir, fr, depth+1, visited); nested != "" {
			return nested
		}
	}
	return ""
}

// cloudOnlyObjectDir checks an object directory, its pack directory and
// files, and its 256 loose-object fan-out directories.
func cloudOnlyObjectDir(dir string, fr scan.FlagReader) bool {
	pack := filepath.Join(dir, "pack")
	for _, p := range []string{dir, pack} {
		if d, _ := fr.IsDataless(p); d {
			return true
		}
	}
	entries, _ := os.ReadDir(pack)
	for _, e := range entries {
		if d, _ := fr.IsDataless(filepath.Join(pack, e.Name())); d {
			return true
		}
	}
	for i := 0; i < 256; i++ {
		if d, _ := fr.IsDataless(filepath.Join(dir, fmt.Sprintf("%02x", i))); d {
			return true
		}
	}
	return false
}

func seenDir(dir string, visited []os.FileInfo) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	for _, v := range visited {
		if os.SameFile(v, info) {
			return true
		}
	}
	return false
}

// locateRepo finds the repository to check. In a hook, git's own directory
// (HookCwd, the working tree top) is tried first, with GIT_DIR, GIT_WORK_TREE
// and GIT_INDEX_FILE resolved against it as git meant them; it is used only if
// it agrees with GIT_DIR and the hook's index. Otherwise the path and the
// environment are used as given, relative values against the current directory.
func locateRepo(ctx context.Context, g *gitx.Runner, o Options) (repo.Repo, string, error) {
	o.Path = physical(o.Path)
	cwd := physical(".")
	if o.HookCwd != "" {
		env := resolveGitEnv(o.GitEnv, o.HookCwd, true)
		idx := resolveAgainst(o.IndexFile, o.HookCwd)
		if r, err := repo.Locate(ctx, g, o.HookCwd, env...); err == nil && anchorAgrees(r, env, idx) {
			// An absolute GIT_DIR from git (linked worktrees, submodules,
			// separate git dirs) names the repository for every git command
			// in the hook, wherever it changes directory. Without one (a main
			// worktree's .git/index) the current directory is part of the question.
			if !gitDirNamed(o.GitEnv) && !cwdAgrees(ctx, g, r, o.Path, o.HookCwd, relativeGitDir(o.GitEnv), o.Flags) {
				return repo.Repo{}, "", fmt.Errorf("%w: the current directory %s is in another repository than the one git is committing in (%s); run the check from that working tree, or give its path", ErrNotTheHooksRepository, o.Path, r.Worktree)
			}
			// git's worktree is its own directory; one found elsewhere
			// (a .git's parent) stays a guess.
			r.WorktreeGuessed = r.WorktreeGuessed && !sameFile(r.Worktree, o.HookCwd)
			return r, idx, nil
		}
	}
	r, err := repo.Locate(ctx, g, o.Path, resolveGitEnv(o.GitEnv, cwd, false)...)
	return r, o.IndexFile, err
}

// physical returns p as chdir reaches it, which is where git works: relative
// to the physical working directory (not the shell's $PWD), with symlinks
// resolved left to right, so ".." after a symlinked directory is the parent
// of its target. A path that does not resolve is returned as given.
func physical(p string) string {
	q := p
	if !filepath.IsAbs(q) {
		wd, err := os.Getwd()
		if err != nil {
			return p
		}
		if wd, err = filepath.EvalSymlinks(wd); err != nil {
			return p
		}
		q = wd + string(filepath.Separator) + q // not Join: ".." must follow the links
	}
	if r, err := filepath.EvalSymlinks(q); err == nil {
		return r
	}
	return p
}

// resolveGitEnv makes GIT_DIR absolute against base; GIT_WORK_TREE too when
// base is trusted (git's own directory), and is kept as given otherwise, so
// that a relative value stays recognisable as a guess.
func resolveGitEnv(env []string, base string, trusted bool) []string {
	var out []string
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if k == "GIT_DIR" || (k == "GIT_WORK_TREE" && trusted) {
			v = resolveAgainst(v, base)
		}
		out = append(out, k+"="+v)
	}
	return out
}

// cwdAgrees reports whether the current directory may run the check for the
// anchored repository. A relative GIT_INDEX_FILE (.git/index) fits any
// repository, and git passes it on to the message editor and whatever that
// starts, so a check run where git itself would find another repository (or
// another worktree of this one, with its own index) is asking about that one.
// The hook's own places are fine: git's directory, the git directory, the
// hooks directory (which may sit in another repository), or no repository.
// relGitDir is a relative GIT_DIR as git passed it on, or "".
func cwdAgrees(ctx context.Context, g *gitx.Runner, r repo.Repo, cwd, hookCwd, relGitDir string, fr scan.FlagReader) bool {
	if sameFile(cwd, hookCwd) {
		return true
	}
	for _, d := range []string{r.GitDir, r.CommonDir} {
		if sameFile(cwd, d) || inside(cwd, d) {
			return true
		}
	}
	if gitFindsHere(cwd, relGitDir, r, fr) {
		return true
	}
	res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--git-path", "hooks")
	if err != nil || res.ExitCode != 0 {
		return false
	}
	hooks := resolveAgainst(strings.TrimSpace(res.Stdout), r.Worktree)
	return sameFile(cwd, hooks) || inside(cwd, hooks)
}

// gitFindsHere reports whether git, run in cwd, would find r or no
// repository at all. With a relative GIT_DIR that is the git directory it
// names from cwd (a gitfile followed), with no discovery; without one, it is
// discovery from cwd.
func gitFindsHere(cwd, relGitDir string, r repo.Repo, fr scan.FlagReader) bool {
	if relGitDir != "" {
		p, err := filepath.EvalSymlinks(cwd + string(filepath.Separator) + relGitDir)
		if err != nil {
			return true // nothing there: git finds no repository
		}
		if gd, ok := gitDirFrom(p, fr); ok {
			return sameFile(gd, r.GitDir)
		}
		dl, _ := fr.IsDataless(p)
		return !dl // a cloud-only gitfile git would download and follow
	}
	common, top, ok := commonDirOf(cwd, fr)
	if !ok {
		return true
	}
	gd, found := gitDirAt(top, fr)
	return found && sameFile(common, r.CommonDir) && sameFile(gd, r.GitDir)
}

// relativeGitDir returns GIT_DIR from env when it is relative.
func relativeGitDir(env []string) string {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GIT_DIR="); ok && !filepath.IsAbs(v) {
			return v
		}
	}
	return ""
}

// gitDirNamed reports whether the environment names the repository with an
// absolute GIT_DIR, as git exports it to hooks outside a main worktree.
func gitDirNamed(env []string) bool {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GIT_DIR="); ok {
			return filepath.IsAbs(v)
		}
	}
	return false
}

func resolveAgainst(p, base string) string {
	if p == "" || filepath.IsAbs(p) || base == "" {
		return p
	}
	return filepath.Join(base, p)
}

// anchorAgrees reports whether the repository found from git's directory is
// the one the hook was told about: the same git dir as GIT_DIR, and the
// hook's index inside it. At least one of them must be there to compare.
func anchorAgrees(r repo.Repo, env []string, index string) bool {
	gd := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GIT_DIR="); ok {
			gd = v
		}
	}
	if gd == "" && index == "" {
		return false
	}
	if gd != "" && !sameFile(r.GitDir, gd) {
		return false
	}
	if index != "" && !ownIndex(index, r) {
		return false
	}
	return true
}

// ownIndex reports whether index lies in r's own git directory, where git
// puts a commit's index, and not in another worktree's or a submodule's
// directory below it.
func ownIndex(index string, r repo.Repo) bool {
	p := hookIndexPath(index, r)
	if p == "" {
		return false
	}
	rel := relTo(r.GitDir, p)
	if rel == p {
		return false
	}
	first, _, _ := strings.Cut(rel, "/")
	return first != "worktrees" && first != "modules"
}
