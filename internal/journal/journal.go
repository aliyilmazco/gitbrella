// Package journal records each change gitbrella makes, phase by
// phase and before it acts, so that the next run can finish or roll back an
// interrupted change and a finished one can be undone later.
package journal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/state"
)

// Phase is how far a change got. Every phase is saved before the next step.
type Phase string

const (
	Planned        Phase = "planned"
	Copied         Phase = "copied"          // the git directory is copied and verified
	OriginalMoved  Phase = "original-moved"  // move-out: the original is in the backup, the worktree has no .git
	GitfileWritten Phase = "gitfile-written" // move-out: the new .git is in place, not yet checked
	GitfileMoved   Phase = "gitfile-moved"   // move-in: the .git file is in the backup, the worktree has no .git
	GitDirReturned Phase = "gitdir-returned" // move-in: the git directory is back, not yet checked
	Applied        Phase = "applied"         // fix: every step done, the check not yet run again
	Done           Phase = "done"
	RolledBack     Phase = "rolled-back"
)

// Mark is one phase with its time.
type Mark struct {
	Phase Phase     `json:"phase"`
	At    time.Time `json:"at"`
}

// Step is one action of a fix, recorded before it is done (Done false) and
// again after (Done true). Recovery and undo reverse a step whose effect is
// on disk, whatever Done says: a crash can fall between the two saves.
type Step struct {
	Action string `json:"action"`           // quarantine, rescue-ref, restore-index, rebuild-index, set-ref, add-exclude, download, refetch
	Path   string `json:"path,omitempty"`   // the file acted on (absolute); for rescue-ref, the copy whose object it keeps
	Backup string `json:"backup,omitempty"` // where it, or a copy of it, went
	Ref    string `json:"ref,omitempty"`    // rescue-ref, set-ref; for refetch, the namespace it fetches into and empties
	Before string `json:"before,omitempty"` // sha256 of the file, or a ref's commit, before
	After  string `json:"after,omitempty"`  // the same after
	Done   bool   `json:"done"`
	Kept   string `json:"kept,omitempty"` // why a reversal left this step's result in place
	Made   string `json:"made,omitempty"` // the outermost directory the step made, removed on reversal while empty
}

// Entry is one change.
type Entry struct {
	SchemaVersion int               `json:"schemaVersion"`
	ID            string            `json:"id"`
	Kind          string            `json:"kind"` // move-out, move-in, prepare, attach
	RepoID        string            `json:"repoId"`
	Worktree      string            `json:"worktree"`
	WorktreeID    string            `json:"worktreeId,omitempty"` // device:inode of the working tree, whatever its name
	Started       time.Time         `json:"started"`
	Phase         Phase             `json:"phase"`
	History       []Mark            `json:"history"`
	Data          map[string]string `json:"data"`
	Steps         []Step            `json:"steps,omitempty"`
	UndoneBy      string            `json:"undoneBy,omitempty"`
}

// Finished reports whether the change needs no recovery.
func (e Entry) Finished() bool { return e.Phase == Done || e.Phase == RolledBack }

// Store keeps entries as one JSON file each in Dir.
type Store struct {
	Dir       string
	Now       func() time.Time
	AfterSave func(Phase) // test hook: called after each saved phase
}

// Begin records a new change in the Planned phase. data receives the change's
// id and returns everything the change records about itself (where its copy
// and backup go): the first save is complete, so a crash right after it leaves
// recovery nothing to guess. The first save never replaces an entry.
func (s Store) Begin(kind, repoID, worktree string, data func(id string) map[string]string) (*Entry, error) {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return nil, err
	}
	for {
		now := s.now()
		var r [6]byte
		if _, err := rand.Read(r[:]); err != nil {
			return nil, err
		}
		e := &Entry{SchemaVersion: 1, ID: now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(r[:]),
			Kind: kind, RepoID: repoID, Worktree: worktree, WorktreeID: state.FileID(worktree), Started: now,
			Phase: Planned, History: []Mark{{Phase: Planned, At: now}}, Data: map[string]string{}}
		if data != nil {
			if d := data(e.ID); d != nil {
				e.Data = d
			}
		}
		tmp, err := s.write(e)
		if err != nil {
			return nil, err
		}
		err = os.Link(tmp, filepath.Join(s.Dir, e.ID+".json"))
		os.Remove(tmp)
		if errors.Is(err, os.ErrExist) {
			continue // the same id: draw another
		}
		if err != nil {
			return nil, err
		}
		if err := syncDir(s.Dir); err != nil {
			return nil, err
		}
		if s.AfterSave != nil {
			s.AfterSave(Planned)
		}
		return e, nil
	}
}

// Advance moves e to phase p and saves it.
func (s Store) Advance(e *Entry, p Phase) error {
	e.Phase = p
	e.History = append(e.History, Mark{Phase: p, At: s.now()})
	if err := s.Save(e); err != nil {
		return err
	}
	if s.AfterSave != nil {
		s.AfterSave(p)
	}
	return nil
}

// Save writes e atomically: a synced temporary file renamed over the entry.
func (s Store) Save(e *Entry) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	tmp, err := s.write(e)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(s.Dir, e.ID+".json")); err != nil {
		os.Remove(tmp)
		return err
	}
	return syncDir(s.Dir)
}

// write puts e in a synced temporary file in the journal and returns its name.
func (s Store) write(e *Entry) (string, error) {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(s.Dir, ".tmp-"+e.ID+"-*")
	if err != nil {
		return "", err
	}
	_, werr := f.Write(append(b, '\n'))
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil || serr != nil || cerr != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("journal %s: write %v, sync %v, close %v", e.ID, werr, serr, cerr)
	}
	return f.Name(), nil
}

// Get reads one entry.
func (s Store) Get(id string) (*Entry, error) {
	b, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, fmt.Errorf("journal entry %s is damaged: %w", id, err)
	}
	return &e, nil
}

// All returns every entry, oldest first. An entry that cannot be read is an
// error: recovery must not guess.
func (s Store) All() ([]Entry, error) {
	names, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, n := range names {
		id, ok := strings.CutSuffix(n.Name(), ".json")
		if !ok || strings.HasPrefix(id, ".") {
			continue
		}
		e, err := s.Get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Started.Equal(out[j].Started) {
			return out[i].Started.Before(out[j].Started)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// List returns the entries of repoID, oldest first.
func (s Store) List(repoID string) ([]Entry, error) {
	all, err := s.All()
	var out []Entry
	for _, e := range all {
		if e.RepoID == repoID {
			out = append(out, e)
		}
	}
	return out, err
}

// Unfinished returns the entries of repoID that need recovery.
func (s Store) Unfinished(repoID string) ([]Entry, error) {
	all, err := s.List(repoID)
	var open []Entry
	for _, e := range all {
		if !e.Finished() {
			open = append(open, e)
		}
	}
	return open, err
}

// UnfinishedAt returns the unfinished changes whose working tree is dir or a
// directory above it, matched by device and inode: another spelling of the
// path, or a folder renamed since, still finds them, and an enclosing
// repository cannot hide them. It also returns the matching directory as it
// is named now.
func (s Store) UnfinishedAt(dir string) ([]Entry, string, error) {
	all, err := s.All()
	if err != nil {
		return nil, "", err
	}
	byID := map[string][]Entry{}
	for _, e := range all {
		if !e.Finished() && e.WorktreeID != "" {
			byID[e.WorktreeID] = append(byID[e.WorktreeID], e)
		}
	}
	if len(byID) == 0 {
		return nil, "", nil
	}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if es := byID[state.FileID(d)]; len(es) > 0 {
			return es, d, nil
		}
		if filepath.Dir(d) == d {
			return nil, "", nil
		}
	}
}

// LastUndoable returns the newest finished change of repoID that undo can
// reverse (a move-out or a fix) and is not undone yet.
func (s Store) LastUndoable(repoID string) (Entry, bool, error) {
	all, err := s.List(repoID)
	if err != nil {
		return Entry{}, false, err
	}
	for i := len(all) - 1; i >= 0; i-- {
		e := all[i]
		if (e.Kind == "move-out" || e.Kind == "fix") && e.Phase == Done && e.UndoneBy == "" {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// LastDone returns the newest finished, not undone change of kind.
func (s Store) LastDone(repoID, kind string) (Entry, bool, error) {
	all, err := s.List(repoID)
	if err != nil {
		return Entry{}, false, err
	}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Kind == kind && all[i].Phase == Done && all[i].UndoneBy == "" {
			return all[i], true, nil
		}
	}
	return Entry{}, false, nil
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
