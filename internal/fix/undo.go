package fix

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/repo"
)

// UndoPlan is what undoing a fix would do: its steps reversed, newest first,
// except those whose result changed since, which stay as they are.
type UndoPlan struct {
	Worktree string           `json:"worktree"`
	RepoID   string           `json:"repoId"`
	Undoes   string           `json:"undoes"`
	Steps    []journal.Step   `json:"steps"`
	Kept     []string         `json:"kept"`
	Refusals []change.Refusal `json:"refusals"`
}

// PlanUndo says what undoing the fix fx would reverse and what it would
// leave. It refuses only while git works on the repository: a step that
// changed since is kept, never overwritten, and never stops the rest.
func PlanUndo(ctx context.Context, g *gitx.Runner, o Options, fx journal.Entry) (UndoPlan, error) {
	u := UndoPlan{Worktree: fx.Worktree, RepoID: fx.RepoID, Undoes: fx.ID, Steps: fx.Steps,
		Kept: []string{}, Refusals: []change.Refusal{}}
	r, err := repo.Locate(ctx, g, fx.Worktree)
	if err != nil {
		return UndoPlan{}, err
	}
	busy, err := change.Busy(ctx, g, o.Env, r.Worktree)
	if err != nil {
		return UndoPlan{}, err
	}
	u.Refusals = append(u.Refusals, busy...)
	v, stays := newView(), map[string]bool{}
	for i := len(fx.Steps) - 1; i >= 0; i-- {
		st := fx.Steps[i]
		why, err := judged(ctx, g, r, &st, v, nil, stays)
		if err != nil {
			return UndoPlan{}, err
		}
		if why != "" {
			u.Kept = append(u.Kept, fmt.Sprintf("%s %s: %s", st.Action, stepName(st), why))
		}
	}
	return u, nil
}

// Undo reverses the fix u names, as a change of its own ("unfix"). It
// returns what stayed in place.
func Undo(ctx context.Context, g *gitx.Runner, u UndoPlan, store journal.Store) (*journal.Entry, []string, error) {
	if len(u.Refusals) > 0 {
		return nil, nil, errors.New("undo: the plan has refusals")
	}
	e, err := store.Begin("unfix", u.RepoID, u.Worktree, func(string) map[string]string {
		return map[string]string{"undoes": u.Undoes}
	})
	if err != nil {
		return nil, nil, err
	}
	kept, err := finishUndo(ctx, g, e, store)
	return e, kept, err
}

// RecoverUnfix finishes an interrupted undo of a fix: reversing is safe to
// repeat, since each step looks at what is on disk.
func RecoverUnfix(ctx context.Context, g *gitx.Runner, e *journal.Entry, store journal.Store) (string, error) {
	kept, err := finishUndo(ctx, g, e, store)
	if err != nil {
		return "", err
	}
	if len(kept) > 0 {
		return "undone except: " + strings.Join(kept, "; "), nil
	}
	return "undone", nil
}

// finishUndo reverses the fix e undoes and marks it undone, also when some
// steps had to stay: an undo that could never finish would block every
// older change's undo.
func finishUndo(ctx context.Context, g *gitx.Runner, e *journal.Entry, store journal.Store) ([]string, error) {
	fx, err := store.Get(e.Data["undoes"])
	if err != nil {
		return nil, err
	}
	// The undo's own entry follows the folder when it is renamed (start
	// updates it); steps recorded under the old path then stay as they are.
	r, err := repo.Locate(ctx, g, e.Worktree)
	if err != nil {
		return nil, err
	}
	kept, err := reverseAll(ctx, g, r, fx, store)
	if err != nil {
		return nil, fmt.Errorf("undoing fix %s: %w", fx.ID, err)
	}
	fx.UndoneBy = e.ID
	if err := store.Save(fx); err != nil {
		return nil, err
	}
	if len(kept) > 0 {
		e.Data["kept"] = strings.Join(kept, "; ")
	}
	return kept, store.Advance(e, journal.Done)
}
