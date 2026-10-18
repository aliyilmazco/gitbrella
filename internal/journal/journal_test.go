package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func store(t *testing.T) Store {
	t.Helper()
	clock := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	return Store{Dir: filepath.Join(t.TempDir(), "journal"), Now: func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}}
}

func TestBeginAdvanceReload(t *testing.T) {
	s := store(t)
	e, err := s.Begin("move-out", "notes-1a2b3c4d", "/w/notes", func(id string) map[string]string {
		return map[string]string{"target": "/g/notes", "backup": "/b/" + id}
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.Phase != Planned || len(e.History) != 1 {
		t.Fatalf("new entry: %+v", e)
	}
	if err := s.Advance(e, Copied); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != Copied || len(got.History) != 2 || got.Data["target"] != "/g/notes" || got.Data["backup"] != "/b/"+e.ID || got.SchemaVersion != 1 {
		t.Fatalf("reloaded: %+v", got)
	}
	names, _ := os.ReadDir(s.Dir)
	for _, n := range names {
		if !strings.HasSuffix(n.Name(), ".json") {
			t.Fatalf("left behind: %s", n.Name())
		}
	}
}

func TestAfterSaveSeesEveryPhase(t *testing.T) {
	s := store(t)
	var seen []Phase
	s.AfterSave = func(p Phase) { seen = append(seen, p) }
	e, _ := s.Begin("move-out", "r-00000000", "/w", nil)
	s.Advance(e, Copied)
	s.Advance(e, Done)
	if strings.Join([]string{string(seen[0]), string(seen[1]), string(seen[2])}, ",") != "planned,copied,done" {
		t.Fatalf("seen %v", seen)
	}
}

func TestListFiltersAndOrders(t *testing.T) {
	s := store(t)
	a, _ := s.Begin("move-out", "a-00000000", "/w/a", nil)
	s.Begin("move-out", "b-00000000", "/w/b", nil)
	c, _ := s.Begin("move-in", "a-00000000", "/w/a", nil)
	s.Advance(a, Done)
	list, err := s.List("a-00000000")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != c.ID {
		t.Fatalf("list: %+v", list)
	}
	open, err := s.Unfinished("a-00000000")
	if err != nil || len(open) != 1 || open[0].ID != c.ID {
		t.Fatalf("unfinished: %+v %v", open, err)
	}
	last, ok, err := s.LastDone("a-00000000", "move-out")
	if err != nil || !ok || last.ID != a.ID {
		t.Fatalf("last done: %+v %v %v", last, ok, err)
	}
	a.UndoneBy = c.ID
	s.Save(a)
	if _, ok, _ := s.LastDone("a-00000000", "move-out"); ok {
		t.Fatal("an undone change counts as the last one")
	}
}

func TestUnreadableEntryStopsChanges(t *testing.T) {
	s := store(t)
	s.Begin("move-out", "a-00000000", "/w/a", nil)
	os.WriteFile(filepath.Join(s.Dir, "20261014T120000Z-zzzz.json"), []byte("{"), 0o600)
	if _, err := s.List("a-00000000"); err == nil {
		t.Fatal("a damaged journal entry was skipped")
	}
}

func TestListWithoutJournal(t *testing.T) {
	s := store(t)
	list, err := s.List("a-00000000")
	if err != nil || len(list) != 0 {
		t.Fatalf("%v %v", list, err)
	}
}

func TestAllAcrossRepositories(t *testing.T) {
	s := store(t)
	s.Begin("move-out", "a-00000000", "/w/a", nil)
	s.Begin("move-out", "b-00000000", "/w/b", nil)
	all, err := s.All()
	if err != nil || len(all) != 2 || all[0].RepoID != "a-00000000" {
		t.Fatalf("%+v %v", all, err)
	}
}

func TestUnfinishedAtFollowsTheFolder(t *testing.T) {
	s := store(t)
	wt := filepath.Join(t.TempDir(), "notes")
	os.MkdirAll(filepath.Join(wt, "sub dir"), 0o755)
	e, _ := s.Begin("move-out", "notes-00000000", wt, nil)
	if e.WorktreeID == "" {
		t.Fatal("no worktree id")
	}
	if open, dir, err := s.UnfinishedAt(filepath.Join(wt, "sub dir")); err != nil || len(open) != 1 || dir != wt {
		t.Fatalf("from a subdirectory: %v %q %v", open, dir, err)
	}
	renamed := filepath.Join(filepath.Dir(wt), "notes 2")
	os.Rename(wt, renamed)
	if open, dir, err := s.UnfinishedAt(renamed); err != nil || len(open) != 1 || dir != renamed {
		t.Fatalf("after a rename: %v %q %v", open, dir, err)
	}
	s.Advance(e, Done)
	if open, _, _ := s.UnfinishedAt(renamed); len(open) != 0 {
		t.Fatal("a finished change still found")
	}
}

func TestBeginNeverReplacesAnEntry(t *testing.T) {
	s := store(t)
	frozen := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return frozen }
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		e, err := s.Begin("fix", "a-00000000", "/w/a", nil)
		if err != nil || seen[e.ID] {
			t.Fatalf("%v %v", e, err)
		}
		seen[e.ID] = true
	}
	if all, _ := s.All(); len(all) != 50 {
		t.Fatalf("%d entries", len(all))
	}
}

func TestStepsAndLastUndoable(t *testing.T) {
	s := store(t)
	out, _ := s.Begin("move-out", "a-00000000", "/w/a", nil)
	s.Advance(out, Done)
	fx, _ := s.Begin("fix", "a-00000000", "/w/a", nil)
	fx.Steps = append(fx.Steps, Step{Action: "quarantine", Path: "/w/a/.git/index 2", Backup: "/b/index 2", Before: "abc"})
	s.Save(fx)
	if last, ok, _ := s.LastUndoable("a-00000000"); !ok || last.ID != out.ID {
		t.Fatalf("an unfinished fix counted: %+v", last)
	}
	fx.Steps[0].Done = true
	s.Advance(fx, Done)
	last, ok, err := s.LastUndoable("a-00000000")
	if err != nil || !ok || last.ID != fx.ID || len(last.Steps) != 1 || !last.Steps[0].Done {
		t.Fatalf("%+v %v %v", last, ok, err)
	}
	prep, _ := s.Begin("prepare", "a-00000000", "/w/a", nil)
	s.Advance(prep, Done)
	if last, _, _ := s.LastUndoable("a-00000000"); last.ID != fx.ID {
		t.Fatal("a prepare counts as undoable")
	}
}
