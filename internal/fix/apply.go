package fix

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/scan"
)

// stepHook is called after each action's change on disk, before the step is
// saved as done: tests stop there to play a crash between the two.
var stepHook = func(kind string) {}

// reverseHook is called when a step's reversal is saved as begun, before it
// changes the disk: tests stop there to play a crash between the two.
var reverseHook = func(kind string) {}

// Apply carries out p as one journaled change. Each step is saved before it
// acts and again after; before each, no git may work on the repository. At
// the end the check runs again: the repaired findings must be gone and no new
// damage may have appeared, or every step is reversed. After a download,
// damage the check could not see before is no failure: it was there.
func Apply(ctx context.Context, g *gitx.Runner, p Plan, o Options, store journal.Store) (*journal.Entry, error) {
	if len(p.Refusals) > 0 || len(p.Actions) == 0 {
		return nil, errors.New("fix: nothing to do, or the plan has refusals")
	}
	e, err := store.Begin("fix", p.RepoID, p.r.Worktree, func(id string) map[string]string {
		return map[string]string{"backups": filepath.Join(o.Dirs.Backups(), id)}
	})
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*journal.Entry, error) {
		kept, err := Reverse(ctx, g, p.r, e, store)
		switch {
		case err != nil:
			return e, fmt.Errorf("%w; reversing: %v (journal %s)", cause, err, e.ID)
		case len(kept) > 0:
			return e, fmt.Errorf("%w; rolled back except: %s", cause, strings.Join(kept, "; "))
		}
		return e, fmt.Errorf("%w; rolled back", cause)
	}
	for i, a := range p.Actions {
		if err := noGitAt(ctx, o.Procs, p.r); err != nil {
			return fail(err)
		}
		st, err := prepare(a, p.r, filepath.Join(e.Data["backups"], strconv.Itoa(i+1)), o)
		if err != nil {
			return fail(err)
		}
		e.Steps = append(e.Steps, st)
		if err := store.Save(e); err != nil {
			return fail(err)
		}
		if a.Kind == RebuildIndex {
			// git writes the new index beside the old place first; its hash
			// is saved before it moves in, so a crash leaves nothing unknown.
			if err := buildIndex(ctx, g, p.r, &e.Steps[i]); err != nil {
				return fail(err)
			}
			if err := store.Save(e); err != nil {
				return fail(err)
			}
		}
		if err := act(ctx, g, p.r, &e.Steps[i], a, o.Flags); err != nil {
			return fail(fmt.Errorf("%s %s: %w", a.Kind, stepName(e.Steps[i]), err))
		}
		stepHook(a.Kind)
		e.Steps[i].Done = true
		if err := store.Save(e); err != nil {
			return fail(err)
		}
	}
	if err := store.Advance(e, journal.Applied); err != nil {
		return fail(err)
	}
	// The check again, without the journal: this fix's own entry is still
	// unfinished, as it should be, and is not damage.
	downloaded := p.Actions[0].Kind == Download // a download plan holds downloads only
	quiet := o.Env
	quiet.JournalDir = ""
	rep, err := change.RunCheck(ctx, g, quiet, p.r.Worktree)
	if err != nil {
		return fail(err)
	}
	for _, f := range rep.Findings {
		k := key(f.Code, f.Path)
		switch {
		case f.Severity != check.Damage:
		case p.targets[k]:
			return fail(fmt.Errorf("after the fix the check still reports %s %s", f.Code, f.Path))
		case !p.before[k] && !downloaded:
			return fail(fmt.Errorf("the fix would leave new damage: %s %s: %s", f.Code, f.Path, f.Message))
		}
	}
	return e, store.Advance(e, journal.Done)
}

// noGitAt fails when a git process works in the repository, or one whose
// directory cannot be read runs at all.
func noGitAt(ctx context.Context, list func(context.Context) ([]procs.Proc, error), r repo.Repo) error {
	ps, err := list(ctx)
	if err != nil {
		return fmt.Errorf("could not list git processes: %w", err)
	}
	for _, p := range ps {
		if p.Cwd == "" || procs.Within(p, r.Worktree, r.CommonDir, r.GitDir) {
			return fmt.Errorf("a git process (%d) works here; run fix again when it is done", p.PID)
		}
	}
	return nil
}

// prepare fills in the step for a: where things go (under dir, this step's
// own backup directory), what they are now and, where it is known before
// acting, what they will be.
func prepare(a Action, r repo.Repo, dir string, o Options) (journal.Step, error) {
	st := journal.Step{Action: a.Kind, Path: a.Path, Ref: a.Ref}
	if a.seen != "" && fingerprint(a.Path) != a.seen {
		// the sync client (which no process list shows) wrote it after the plan
		return st, fmt.Errorf("%s changed since the plan; run fix again", a.Path)
	}
	switch a.Kind {
	case Quarantine:
		if err := change.LocalOnly(a.Path, o.Flags); err != nil {
			return st, err
		}
		st.Backup = filepath.Join(dir, relName(a.Path, r))
		st.Before = hashPath(a.Path)
	case RescueRef:
		st.After = a.Value
	case Refetch:
		st.Ref = FetchPrefix + filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir) + "/" // the entry's id and the step
	case RestoreIndex:
		st.Path = strings.TrimSuffix(a.Path, ".lock")
		st.Backup = a.Path // the lock goes back there on reversal
		st.Before = hashPath(a.Path)
		st.After = st.Before // the same bytes under the other name
	case RebuildIndex:
		st.Backup = filepath.Join(dir, "index")
	case SetRef, AddExclude:
		st.Backup = filepath.Join(dir, filepath.Base(a.Path))
		st.Before = hashPath(a.Path)
		if a.Kind == SetRef {
			st.After = hashBytes([]byte(a.Value + "\n"))
		} else {
			st.After = hashBytes(excludeContent(a.Path, a.Value))
		}
		for d := filepath.Dir(a.Path); !exists(d) && inRepo(d, r); d = filepath.Dir(d) {
			st.Made = d // the outermost directory the step will make (info/)
		}
	}
	return st, nil
}

// excludeContent is info/exclude with line added.
func excludeContent(p, line string) []byte {
	old, _ := os.ReadFile(p)
	if len(old) > 0 && !bytes.HasSuffix(old, []byte("\n")) {
		old = append(old, '\n')
	}
	return append(old, []byte(line+"\n")...)
}

// newIndex is where buildIndex makes the index before it moves in.
func newIndex(path string) string { return path + ".gitbrella-new" }

// buildIndex makes the index from HEAD at newIndex(st.Path) and records its
// hash. git read-tree writes only the index: unlike git reset it leaves
// ORIG_HEAD and the reflog alone.
func buildIndex(ctx context.Context, g *gitx.Runner, r repo.Repo, st *journal.Step) error {
	tmp := newIndex(st.Path)
	if exists(st.Path) || exists(tmp) || exists(tmp+".lock") {
		return fmt.Errorf("%s, %s or its lock exists; refusing to replace it", st.Path, tmp)
	}
	res, err := g.Run(ctx, r.Worktree, append(r.Env(), "GIT_INDEX_FILE="+tmp), "read-tree", "HEAD")
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git read-tree HEAD: %s", strings.TrimSpace(res.Stderr))
	}
	if err != nil {
		return err
	}
	st.After = hashPath(tmp)
	return nil
}

// fetchTimeout bounds a refetch, which moves the whole repository.
const fetchTimeout = time.Hour

// act does one step on disk.
func act(ctx context.Context, g *gitx.Runner, r repo.Repo, st *journal.Step, a Action, flags scan.FlagReader) error {
	switch a.Kind {
	case Quarantine:
		if err := os.MkdirAll(filepath.Dir(st.Backup), 0o700); err != nil {
			return err
		}
		return change.Rename(a.Path, st.Backup)
	case RescueRef:
		return gitOK(ctx, g, r, "update-ref", "-m", "gitbrella: rescue", a.Ref, a.Value, "")
	case RestoreIndex:
		if exists(st.Path) {
			return fmt.Errorf("%s appeared meanwhile", st.Path)
		}
		return change.Rename(a.Path, st.Path)
	case RebuildIndex:
		if exists(a.Path) {
			return fmt.Errorf("%s appeared meanwhile", a.Path)
		}
		return os.Rename(newIndex(a.Path), a.Path) // same directory: one step
	case SetRef, AddExclude:
		content := []byte(a.Value + "\n")
		if a.Kind == AddExclude {
			content = excludeContent(a.Path, a.Value)
		}
		if hashPath(a.Path) != st.Before || hashBytes(content) != st.After {
			return fmt.Errorf("%s changed since the plan", a.Path)
		}
		if exists(a.Path) {
			if err := copyFile(a.Path, st.Backup); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
			return err
		}
		return writeAtomic(a.Path, content)
	case Download:
		return download(a.Path, flags)
	case Refetch:
		return refetch(ctx, g, r, st.Ref, a)
	}
	return fmt.Errorf("unknown action %q", a.Kind)
}

// download brings every cloud-only file and folder under root onto the disk:
// each is read through on a thread whose policy lets the file provider
// fetch it. Nothing is written; what is still in the cloud afterwards fails.
func download(root string, flags scan.FlagReader) error {
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dl, err := flags.IsDataless(p); err != nil || !dl {
			return err
		}
		// a folder is downloaded before the walk lists it
		return scan.AllowMaterialization(func() error {
			if d.IsDir() {
				_, err := os.ReadDir(p)
				return err
			}
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(io.Discard, f)
			return err
		})
	})
	if err != nil {
		return err
	}
	return change.LocalOnly(root, flags)
}

// refetch fetches the objects of a.Value's branches and tags again, into
// the refs under ns, which it deletes before it returns: no ref of the
// repository changes (--refmap= stops the remote-tracking ones). Then every
// object of a.Objects must be here.
func refetch(ctx context.Context, g *gitx.Runner, r repo.Repo, ns string, a Action) error {
	long := *g
	long.Timeout = fetchTimeout
	res, err := long.Run(ctx, r.Worktree, r.Env(), "fetch", "--refetch", "--quiet", "--no-tags", "--refmap=", "--no-prune",
		"--no-write-fetch-head", "--no-recurse-submodules", "--no-write-commit-graph",
		a.Value, "+refs/heads/*:"+ns+"heads/*", "+refs/tags/*:"+ns+"tags/*")
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git fetch --refetch %s: %s", a.Value, strings.TrimSpace(res.Stderr))
	}
	refs, lerr := refsUnder(ctx, g, r, ns)
	if lerr == nil && len(refs) > 0 {
		lerr = dropRefs(ctx, g, r, refs)
	}
	if err != nil {
		return err
	}
	if lerr != nil {
		return lerr
	}
	for _, oid := range a.Objects {
		res, err := g.Run(ctx, r.Worktree, r.Env(), "cat-file", "-e", oid)
		if err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("%s does not have %s", a.Value, oid)
		}
	}
	return nil
}

// refsUnder lists the refs under prefix ("refs/x/").
func refsUnder(ctx context.Context, g *gitx.Runner, r repo.Repo, prefix string) ([]string, error) {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "for-each-ref", "--format=%(refname)", prefix)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git for-each-ref: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.Fields(res.Stdout), nil
}

// dropRefs deletes the refs a refetch made, and the directories they leave
// empty.
func dropRefs(ctx context.Context, g *gitx.Runner, r repo.Repo, refs []string) error {
	var in strings.Builder
	for _, ref := range refs {
		in.WriteString("delete " + ref + "\n")
	}
	res, err := g.RunInput(ctx, r.Worktree, r.Env(), in.String(), "update-ref", "--stdin")
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git update-ref: %s", strings.TrimSpace(res.Stderr))
	}
	if err != nil {
		return err
	}
	for _, ref := range refs {
		removeEmptyRefDirs(r, ref, FetchPrefix)
	}
	return nil
}

// removeEmptyRefDirs removes the directories of ref up to and including the
// one prefix names, each only while it is empty: git leaves them.
func removeEmptyRefDirs(r repo.Repo, ref, prefix string) {
	top := filepath.Join(r.CommonDir, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
	for d := filepath.Dir(filepath.Join(r.CommonDir, filepath.FromSlash(ref))); within(d, top); d = filepath.Dir(d) {
		if err := os.Remove(d); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return // not empty
		}
	}
}

// Reverse undoes the steps of an interrupted or failed fix (see reverseAll)
// and marks it rolled back.
func Reverse(ctx context.Context, g *gitx.Runner, r repo.Repo, e *journal.Entry, store journal.Store) ([]string, error) {
	kept, err := reverseAll(ctx, g, r, e, store)
	if err == nil && e.Phase != journal.RolledBack {
		err = store.Advance(e, journal.RolledBack)
	}
	return kept, err
}

// reverseAll undoes the steps of a fix, newest first, judging each by what
// is on disk rather than by its Done flag alone (a crash can fall between
// acting and saving). A step whose result changed since (git used the
// rebuilt index, a rescue ref moved, the path is back), whose backup is gone,
// or whose repository is elsewhere now is kept: nothing is overwritten, the
// reason goes into the step, and the older steps are still reversed. kept
// lists what was left in place. Running it again is safe.
func reverseAll(ctx context.Context, g *gitx.Runner, r repo.Repo, e *journal.Entry, store journal.Store) (kept []string, err error) {
	stays := map[string]bool{}
	for i := len(e.Steps) - 1; i >= 0; i-- {
		st := &e.Steps[i]
		// Done is cleared and saved before the step is reversed: a crash
		// after the reversal then finds an undone step, not a missing backup.
		clear := func() error { st.Done = false; return store.Save(e) }
		why, err := judged(ctx, g, r, st, nil, clear, stays)
		if err != nil {
			return kept, err
		}
		st.Kept = why
		if why != "" {
			kept = append(kept, fmt.Sprintf("%s %s: %s", st.Action, stepName(*st), why))
		}
		if err := store.Save(e); err != nil {
			return kept, err
		}
	}
	return kept, nil
}

// view is what the disk would look like after the reversals planned so far:
// a dry run judges each step against it, not against the disk.
type view struct{ gone, made map[string]bool }

func newView() *view { return &view{gone: map[string]bool{}, made: map[string]bool{}} }

// moved records a planned move.
func (v *view) moved(from, to string) {
	v.gone[from], v.made[from] = true, false
	v.made[to], v.gone[to] = true, false
}

// judged reverses one step (see reverseStep), except a rescue ref whose
// copy stays in the backup: the ref may be all that keeps the copy's object
// alive. Steps go newest first, so the quarantines are judged before the
// rescues, which ran first; stays collects the copies left in the backup.
func judged(ctx context.Context, g *gitx.Runner, r repo.Repo, st *journal.Step, v *view, before func() error, stays map[string]bool) (string, error) {
	if st.Action == RescueRef && stays[st.Path] {
		return "the copy whose object it keeps stays in the backup", nil
	}
	why, err := reverseStep(ctx, g, r, st, v, before)
	if st.Action == Quarantine && why != "" {
		stays[st.Path] = true
	}
	return why, err
}

// stepName is what a step acted on: its ref, or its path.
func stepName(st journal.Step) string {
	if st.Ref != "" {
		return st.Ref
	}
	return st.Path
}

// reverseStep reverses one step. It returns "" when it did, or when there
// was nothing to reverse, and otherwise why the step's result stays. With a
// view it only judges, recording in the view what it would move. before runs
// just before anything on disk changes; its error stops the reversal.
func reverseStep(ctx context.Context, g *gitx.Runner, r repo.Repo, st *journal.Step, v *view, before func() error) (string, error) {
	exists := func(p string) bool {
		if v != nil && v.made[p] {
			return true
		}
		if v != nil && v.gone[p] {
			return false
		}
		return exists(p)
	}
	do := func(f func() error, from, to string) (string, error) {
		if v != nil {
			if from != "" {
				v.moved(from, to)
			}
			return "", nil
		}
		if before != nil {
			if err := before(); err != nil {
				return "", err
			}
		}
		reverseHook(st.Action)
		if err := f(); err != nil {
			return "reversing failed: " + err.Error(), nil
		}
		return "", nil
	}
	moveAside := func(from, to string) func() error {
		return func() error {
			if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
				return err
			}
			return change.Rename(from, to)
		}
	}
	switch st.Action {
	case Download:
		// the files are on disk now, with the content they had in the
		// cloud: nothing to reverse, and nothing that changed since
		return "", nil
	case Refetch:
		// the fetched objects stay: they replace nothing. Only refs a
		// fetch that stopped half way left need to go.
		refs, err := refsUnder(ctx, g, r, st.Ref)
		if err != nil {
			return "reading " + st.Ref + " failed: " + err.Error(), nil
		}
		if len(refs) == 0 {
			return "", nil
		}
		return do(func() error { return dropRefs(ctx, g, r, refs) }, "", "")
	}
	if st.Path != "" && !inRepo(st.Path, r) {
		return "the repository is somewhere else now; " + st.Path + " is not in it", nil
	}
	switch st.Action {
	case Quarantine:
		switch {
		case !exists(st.Backup) && st.Done:
			return "its backup " + st.Backup + " is gone", nil
		case !exists(st.Backup):
			return "", nil // never moved, or already moved back
		case exists(st.Path):
			return st.Path + " exists again; the quarantined one stays at " + st.Backup, nil
		}
		return do(func() error {
			if err := os.MkdirAll(filepath.Dir(st.Path), 0o755); err != nil {
				return err
			}
			return change.Rename(st.Backup, st.Path)
		}, st.Backup, st.Path)
	case RescueRef:
		res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "-q", st.Ref)
		if err != nil {
			return "reading " + st.Ref + " failed: " + err.Error(), nil
		}
		switch got := strings.TrimSpace(res.Stdout); {
		case res.ExitCode != 0:
			return "", nil // never made, or already removed
		case got != st.After && !st.Done:
			return "", nil // update-ref with an empty old value never replaces: not this step's
		case got != st.After:
			return st.Ref + " now points at " + got + ", not " + st.After, nil
		}
		return do(func() error {
			if err := gitOK(ctx, g, r, "update-ref", "-d", st.Ref, st.After); err != nil {
				return err
			}
			removeEmptyRefDirs(r, st.Ref, RescuePrefix)
			return nil
		}, "", "")
	case RestoreIndex:
		switch {
		case exists(st.Backup):
			return "", nil // the lock was never moved, or is back
		case !exists(st.Path):
			return "neither " + st.Path + " nor " + st.Backup + " exists", nil
		case hashPath(st.Path) != st.After:
			return st.Path + " changed since the fix (git used it)", nil
		}
		return do(func() error { return change.Rename(st.Path, st.Backup) }, st.Path, st.Backup)
	case RebuildIndex:
		if lock := newIndex(st.Path) + ".lock"; exists(lock) { // git read-tree stopped half way
			if why, err := do(moveAside(lock, st.Backup+"-unused.lock"), lock, st.Backup+"-unused.lock"); why != "" || err != nil {
				return why, err
			}
		}
		if tmp := newIndex(st.Path); exists(tmp) { // built, never moved in
			return do(moveAside(tmp, st.Backup+"-unused"), tmp, st.Backup+"-unused")
		}
		switch {
		case !exists(st.Path):
			return "", nil // never rebuilt
		case st.After == "" || hashPath(st.Path) != st.After:
			return st.Path + " changed since the fix (git used it)", nil
		}
		return do(moveAside(st.Path, st.Backup), st.Path, st.Backup)
	case SetRef, AddExclude:
		switch now := hashPath(st.Path); {
		case now == st.Before:
			if v == nil {
				removeMade(st)
			}
			return "", nil // never written, or already put back
		case now != st.After:
			return st.Path + " changed since the fix", nil
		case st.Before == "": // there was no such file: set the one fix made aside
			return do(func() error {
				if err := moveAside(st.Path, st.Backup+"-added")(); err != nil {
					return err
				}
				removeMade(st)
				return nil
			}, st.Path, st.Backup+"-added")
		case !exists(st.Backup):
			return "its backup " + st.Backup + " is gone", nil
		}
		return do(func() error {
			b, err := os.ReadFile(st.Backup)
			if err != nil {
				return err
			}
			return writeAtomic(st.Path, b)
		}, "", "")
	}
	return "unknown action " + st.Action, nil
}

// removeMade removes the directories the step made (st.Made and those below
// it, up to the step's file), each only while it is empty.
func removeMade(st *journal.Step) {
	if st.Made == "" {
		return
	}
	for d := filepath.Dir(st.Path); within(d, st.Made); d = filepath.Dir(d) {
		if err := os.Remove(d); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return // not empty: something else is in it now
		}
	}
}

// Recover reverses an interrupted fix.
func Recover(ctx context.Context, g *gitx.Runner, e *journal.Entry, o Options, store journal.Store) (string, error) {
	r, err := repo.Locate(ctx, g, e.Worktree)
	if err != nil {
		return "", err
	}
	kept, err := Reverse(ctx, g, r, e, store)
	if err != nil {
		return "", err
	}
	if len(kept) > 0 {
		return "rolled back except: " + strings.Join(kept, "; "), nil
	}
	return "rolled back", nil
}

func gitOK(ctx context.Context, g *gitx.Runner, r repo.Repo, args ...string) error {
	res, err := g.Run(ctx, r.Worktree, r.Env(), args...)
	if err == nil && res.ExitCode != 0 {
		err = fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(res.Stderr))
	}
	return err
}

// relName is p relative to the git directory or the working tree, for its
// place in the backup.
func relName(p string, r repo.Repo) string {
	for _, base := range []string{r.CommonDir, r.Worktree} {
		if within(p, base) {
			rel, _ := filepath.Rel(base, p)
			if base == r.Worktree {
				return filepath.Join("worktree", rel)
			}
			return filepath.Join("gitdir", rel)
		}
	}
	return filepath.Base(p)
}

// inRepo reports whether p is in r's git directory, or in its working tree
// but not under a .git that is no longer the git directory (it was moved out
// since).
func inRepo(p string, r repo.Repo) bool {
	return within(p, r.CommonDir) || within(p, r.Worktree) && !within(p, filepath.Join(r.Worktree, ".git"))
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}

// writeAtomic replaces p with b through a synced temporary file in the same
// directory.
func writeAtomic(p string, b []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(p); err == nil {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".gitbrella-*")
	if err != nil {
		return err
	}
	_, werr := f.Write(b)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil || serr != nil || cerr != nil {
		os.Remove(f.Name())
		return fmt.Errorf("writing %s: %v %v %v", p, werr, serr, cerr)
	}
	if err := os.Chmod(f.Name(), mode); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), p)
}
