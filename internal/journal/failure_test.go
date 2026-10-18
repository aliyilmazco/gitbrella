package journal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A journal that cannot be written stops the change before it starts, and a
// save that fails leaves the saved entry as it was: never half a file.
func TestUnwritableJournal(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes everywhere")
	}
	blocker := filepath.Join(t.TempDir(), "file")
	os.WriteFile(blocker, nil, 0o644)
	under := Store{Dir: filepath.Join(blocker, "journal")} // a directory inside a file
	if _, err := under.Begin("fix", "r", "/w", nil); err == nil {
		t.Fatal("Begin wrote under a regular file")
	}
	if err := under.Save(&Entry{ID: "x"}); err == nil {
		t.Fatal("Save wrote under a regular file")
	}

	s := store(t)
	e, err := s.Begin("fix", "r", "/w", nil)
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(s.Dir, 0o500)
	t.Cleanup(func() { os.Chmod(s.Dir, 0o700) })
	if _, err := s.Begin("fix", "r", "/w", nil); err == nil {
		t.Fatal("Begin wrote into a read-only journal")
	}
	if err := s.Advance(e, Done); err == nil {
		t.Fatal("Advance wrote into a read-only journal")
	}
	got, err := s.Get(e.ID)
	if err != nil || got.Phase != Planned {
		t.Fatalf("the saved entry changed: %+v %v", got, err)
	}
	names, _ := os.ReadDir(s.Dir)
	if len(names) != 1 {
		t.Fatalf("left behind: %v", names)
	}
}

// Files that are not entries (temporary files, anything else) are not read.
func TestAllSkipsWhatIsNotAnEntry(t *testing.T) {
	s := store(t)
	e, err := s.Begin("fix", "r", "/w", nil)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(s.Dir, ".tmp-x.json"), []byte("half"), 0o600)
	os.WriteFile(filepath.Join(s.Dir, "notes.txt"), []byte("hi"), 0o600)
	all, err := s.All()
	if err != nil || len(all) != 1 || all[0].ID != e.ID {
		t.Fatalf("%+v %v", all, err)
	}
}

// Nothing to undo or recover: the lookups say so without an error.
func TestNothingFound(t *testing.T) {
	s := store(t)
	wt := t.TempDir()
	e, err := s.Begin("move-out", "r", wt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.LastUndoable("r"); ok || err != nil {
		t.Fatalf("an unfinished move-out is undoable: %v %v", ok, err)
	}
	if es, d, err := s.UnfinishedAt(t.TempDir()); len(es) != 0 || d != "" || err != nil {
		t.Fatalf("another folder: %+v %q %v", es, d, err)
	}
	s.Advance(e, Done)
	if es, _, err := s.UnfinishedAt(wt); len(es) != 0 || err != nil {
		t.Fatalf("a finished change needs recovery: %+v %v", es, err)
	}
}

// A store without a clock uses the time of day.
func TestStoreWithoutAClock(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	before := time.Now()
	e, err := s.Begin("fix", "r", "/w", nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.Started.Before(before) || time.Since(e.Started) > time.Minute {
		t.Fatalf("started %v", e.Started)
	}
}
