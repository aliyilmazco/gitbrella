package fix

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/procs"
)

// A git that starts in the repository part way through a fix stops it
// before the next step, and every step already done is reversed: the
// repository is given back exactly as it was, for every kind of step.
func TestGitStartingMidFixReversesTheDoneSteps(t *testing.T) {
	for _, name := range repairable {
		g0, r0, _ := scenario(t, name)
		o0, _ := fixOptions(t, r0.Worktree)
		p0, _ := MakePlan(context.Background(), g0, o0)
		for done := range len(p0.Actions) {
			t.Run(name+"/"+kinds(p0.Actions[:done]), func(t *testing.T) {
				g, r, _ := scenario(t, name)
				before := content(t, r.Worktree)
				o, store := fixOptions(t, r.Worktree)
				p, err := MakePlan(context.Background(), g, o)
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				o.Procs = func(context.Context) ([]procs.Proc, error) {
					calls++
					if calls > done {
						return []procs.Proc{{PID: 4242, Cwd: r.Worktree}}, nil
					}
					return nil, nil
				}
				e, err := Apply(context.Background(), g, p, o, store)
				if err == nil || !strings.Contains(err.Error(), "(4242) works here") || !strings.Contains(err.Error(), "rolled back") {
					t.Fatalf("err = %v", err)
				}
				if open, _ := store.Unfinished(e.RepoID); len(open) != 0 {
					t.Fatalf("the fix left its journal entry open: %+v", open)
				}
				sameContent(t, before, content(t, r.Worktree))
			})
		}
	}
}

// When the git processes cannot be listed, nothing is changed.
func TestFixStopsWhenProcessesCannotBeListed(t *testing.T) {
	g, r, _ := scenario(t, "index-copies")
	before := content(t, r.Worktree)
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil {
		t.Fatal(err)
	}
	o.Procs = func(context.Context) ([]procs.Proc, error) { return nil, errors.New("ps failed") }
	if _, err := Apply(context.Background(), g, p, o, store); err == nil || !strings.Contains(err.Error(), "could not list git processes") {
		t.Fatalf("err = %v", err)
	}
	sameContent(t, before, content(t, r.Worktree))
}

// A plan with nothing to do, or with refusals, is never carried out.
func TestApplyRefusesAnEmptyOrRefusedPlan(t *testing.T) {
	g, r, _ := scenario(t, "index-copies")
	o, store := fixOptions(t, r.Worktree)
	p, err := MakePlan(context.Background(), g, o)
	if err != nil {
		t.Fatal(err)
	}
	empty := p
	empty.Actions = nil
	refused := p
	refused.Refusals = []change.Refusal{{Code: change.RefuseBusy}}
	for _, q := range []Plan{empty, refused} {
		if e, err := Apply(context.Background(), g, q, o, store); err == nil || e != nil {
			t.Fatalf("carried out %+v: %v", q, err)
		}
	}
}
