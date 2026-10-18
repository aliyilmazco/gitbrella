package verify

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/repo"
)

func setup(t *testing.T, scenario string) (*gitx.Runner, repo.Repo, string) {
	t.Helper()
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build(scenario, dir); err != nil {
		t.Fatal(err)
	}
	r, err := repo.Locate(context.Background(), g, dir)
	if err != nil {
		t.Fatal(err)
	}
	return g, r, dir
}

func TestCheckIndexFile(t *testing.T) {
	_, r, dir := setup(t, "clean")
	if st := CheckIndexFile(fixtures.Abs(dir, "index"), r.ObjectFormat); !st.Valid || st.Version < 2 || st.Entries != 1 {
		t.Fatalf("clean index: %+v", st)
	}
	_, r, dir = setup(t, "corrupt-index")
	if st := CheckIndexFile(fixtures.Abs(dir, "index"), r.ObjectFormat); st.Valid || st.Reason != "checksum mismatch" {
		t.Fatalf("corrupt index: %+v", st)
	}
}

func TestCheckIndexBytesEdgeCases(t *testing.T) {
	_, r, dir := setup(t, "clean")
	good, err := os.ReadFile(fixtures.Abs(dir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	truncated := good[:len(good)-7]
	if st := CheckIndexBytes(truncated, r.ObjectFormat); st.Valid {
		t.Fatalf("truncated index accepted: %+v", st)
	}
	badSig := append([]byte("XXXX"), good[4:]...)
	if st := CheckIndexBytes(badSig, r.ObjectFormat); st.Valid || st.Reason != "missing DIRC signature" {
		t.Fatalf("bad signature: %+v", st)
	}
	zeroTrailer := append(append([]byte{}, good[:len(good)-20]...), make([]byte, 20)...)
	if st := CheckIndexBytes(zeroTrailer, r.ObjectFormat); st.Valid || !st.Unverifiable {
		t.Fatalf("zero trailer must be unverifiable: %+v", st)
	}
	if st := CheckIndexBytes([]byte("DIRC"), r.ObjectFormat); st.Valid || st.Reason != "file too short" {
		t.Fatalf("short file: %+v", st)
	}
}

func TestCheckIndexSHA256Repo(t *testing.T) {
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "s256")
	if _, err := fixtures.New(dir, "--object-format=sha256"); err != nil {
		t.Fatal(err)
	}
	r, err := repo.Locate(context.Background(), g, dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.ObjectFormat != "sha256" {
		t.Fatalf("object format = %s", r.ObjectFormat)
	}
	if st := CheckIndexFile(fixtures.Abs(dir, "index"), r.ObjectFormat); !st.Valid {
		t.Fatalf("sha256 index: %+v", st)
	}
}

func TestIndexEqualsHEAD(t *testing.T) {
	g, r, dir := setup(t, "orphan-index-lock")
	lock := fixtures.Abs(dir, "index.lock")
	before, _ := os.Stat(lock)
	eq, err := IndexEqualsHEAD(context.Background(), g, r, lock)
	if err != nil || !eq {
		t.Fatalf("lock should equal HEAD: %v %v", eq, err)
	}
	after, _ := os.Stat(lock)
	if !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Fatal("IndexEqualsHEAD modified the candidate")
	}
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(lock, fixtures.Abs(dir, "index")); err != nil {
		t.Fatal(err)
	}
	fr := &fixtures.Repo{Dir: dir, GitDir: filepath.Join(dir, ".git")}
	if _, err := fr.Git("commit", "-q", "-am", "three"); err != nil {
		t.Fatal(err)
	}
	if err := fr.CopyFile("index", "older-index"); err != nil {
		t.Fatal(err)
	}
	if _, err := fr.Git("reset", "-q", "--soft", "HEAD~1"); err != nil {
		t.Fatal(err)
	}
	eq, err = IndexEqualsHEAD(context.Background(), g, r, fixtures.Abs(dir, "older-index"))
	if err != nil || eq {
		t.Fatalf("index from a later commit must differ from HEAD: %v %v", eq, err)
	}
}
