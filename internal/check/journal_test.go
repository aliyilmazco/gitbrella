package check

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// journalFor writes, before the check runs, a journal entry for the checked
// repository: kind and phase as a change command would leave them.
func journalFor(t *testing.T, kind string, phase journal.Phase) func(*Options) {
	return func(o *Options) {
		dir := filepath.Join(t.TempDir(), "journal")
		o.JournalDir = dir
		real, err := filepath.EvalSymlinks(o.Path)
		if err != nil {
			t.Fatal(err)
		}
		s := journal.Store{Dir: dir}
		e, err := s.Begin(kind, state.RepoID(real, o.Home), real, func(string) map[string]string {
			return map[string]string{"target": "/elsewhere/notes"}
		})
		if err != nil {
			t.Fatal(err)
		}
		if phase != journal.Planned {
			if err := s.Advance(e, phase); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestGitDirReturnedAfterAMove(t *testing.T) {
	rep := run(t, "clean", journalFor(t, "move-out", journal.Done))
	f := find(t, rep, CodeGitDirReturned)
	if f.Severity != Damage || f.Details["movedTo"] != "/elsewhere/notes" {
		t.Fatalf("%+v", f)
	}
	if ExitCode(rep) != 1 {
		t.Fatal("not damage")
	}
}

func TestUnfinishedChangeBlocks(t *testing.T) {
	rep := run(t, "clean", journalFor(t, "move-out", journal.Copied))
	if f := find(t, rep, CodeChangeUnfinished); f.Severity != Damage {
		t.Fatalf("%+v", f)
	}
}

func TestNoJournalNoFinding(t *testing.T) {
	rep := run(t, "clean", journalFor(t, "move-out", journal.RolledBack))
	if slices.Contains(codes(rep), CodeGitDirReturned) || slices.Contains(codes(rep), CodeChangeUnfinished) {
		t.Fatalf("%v", codes(rep))
	}
	if rep := run(t, "clean", nil); len(codes(rep)) != 0 {
		t.Fatalf("without a journal directory: %v", codes(rep))
	}
}
