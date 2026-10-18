package verify

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
)

// indexRepo builds a repository with paths of many lengths (to exercise the
// v2/v3 padding) in nested directories.
func indexRepo(t *testing.T, initArgs ...string) *fixtures.Repo {
	t.Helper()
	if _, err := gitx.New(); err != nil {
		t.Skip("git not installed")
	}
	r, err := fixtures.New(filepath.Join(t.TempDir(), "repo"), initArgs...)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 17; i++ {
		p := filepath.Join(r.Dir, "src", strings.Repeat("d", i%5), strings.Repeat("f", i)+".txt")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(r.Dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Dir, "docs", "Ünïcode name.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, r, "add", "-A")
	mustGit(t, r, "commit", "-q", "-m", "files")
	return r
}

func mustGit(t *testing.T, r *fixtures.Repo, args ...string) string {
	t.Helper()
	out, err := r.Git(args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// gitPaths lists the index entries the way git reads them.
func gitPaths(t *testing.T, r *fixtures.Repo) []string {
	t.Helper()
	args := []string{"-c", "core.quotePath=false", "ls-files", "-s", "-z"}
	if ok, _ := fixtures.GitAtLeast(2, 35); ok {
		args = append(args, "--sparse") // keeps a sparse index's directory entries as they are
	}
	out := mustGit(t, r, args...)
	var paths []string
	for _, rec := range strings.Split(strings.TrimRight(out, "\x00"), "\x00") {
		if rec == "" {
			continue
		}
		_, p, _ := strings.Cut(rec, "\t")
		paths = append(paths, p)
	}
	return paths
}

func parsed(t *testing.T, r *fixtures.Repo, format string) Index {
	t.Helper()
	ix, err := ParseIndexFile(filepath.Join(r.GitDir, "index"), format)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func TestParseIndexMatchesGit(t *testing.T) {
	for _, c := range []struct {
		name    string
		init    []string
		format  string
		prepare func(t *testing.T, r *fixtures.Repo)
		version uint32
	}{
		{"v2", nil, "sha1", func(*testing.T, *fixtures.Repo) {}, 2},
		{"v3 intent-to-add", nil, "sha1", func(t *testing.T, r *fixtures.Repo) {
			if err := os.WriteFile(filepath.Join(r.Dir, "new.txt"), []byte("n"), 0o644); err != nil {
				t.Fatal(err)
			}
			mustGit(t, r, "add", "-N", "new.txt")
		}, 3},
		{"v4", nil, "sha1", func(t *testing.T, r *fixtures.Repo) {
			mustGit(t, r, "update-index", "--index-version", "4")
		}, 4},
		{"sha256", []string{"--object-format=sha256"}, "sha256", func(*testing.T, *fixtures.Repo) {}, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := indexRepo(t, c.init...)
			c.prepare(t, r)
			ix := parsed(t, r, c.format)
			if ix.Version != c.version {
				t.Errorf("version %d, want %d", ix.Version, c.version)
			}
			if want := gitPaths(t, r); !slices.Equal(ix.Paths, want) {
				t.Errorf("paths\n got %q\nwant %q", ix.Paths, want)
			}
			if ix.Split() || ix.Sparse() {
				t.Errorf("extensions %q", ix.Extensions)
			}
		})
	}
}

func TestParseSparseIndex(t *testing.T) {
	if ok, _ := fixtures.GitAtLeast(2, 37); !ok {
		t.Skip("needs git 2.37")
	}
	r := indexRepo(t)
	mustGit(t, r, "sparse-checkout", "set", "--cone", "--sparse-index", "docs")
	ix := parsed(t, r, "sha1")
	if !ix.Sparse() {
		t.Fatalf("no sdir extension: %q", ix.Extensions)
	}
	if !slices.Contains(ix.Paths, "src/") {
		t.Errorf("no sparse directory entry for src/: %q", ix.Paths)
	}
	if want := gitPaths(t, r); !slices.Equal(ix.Paths, want) {
		t.Errorf("paths\n got %q\nwant %q", ix.Paths, want)
	}
}

func TestParseSplitAndSkipHashIndex(t *testing.T) {
	r := indexRepo(t)
	mustGit(t, r, "update-index", "--split-index")
	if ix := parsed(t, r, "sha1"); !ix.Split() {
		t.Errorf("split index without link extension: %q", ix.Extensions)
	}
	mustGit(t, r, "update-index", "--no-split-index")
	if ix := parsed(t, r, "sha1"); ix.Split() {
		t.Errorf("unsplit index still split: %q", ix.Extensions)
	}
	if ok, _ := fixtures.GitAtLeast(2, 40); !ok {
		t.Skip("index.skipHash needs git 2.40")
	}
	mustGit(t, r, "-c", "index.skipHash=true", "update-index", "--force-write-index")
	b, err := os.ReadFile(filepath.Join(r.GitDir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	if st := CheckIndexBytes(b, "sha1"); !st.Unverifiable {
		t.Fatalf("not written with skipHash: %+v", st)
	}
	if ix := parsed(t, r, "sha1"); !slices.Equal(ix.Paths, gitPaths(t, r)) {
		t.Errorf("skipHash paths differ from git's: %q", ix.Paths)
	}
}

func TestParseIndexRejectsTruncation(t *testing.T) {
	r := indexRepo(t)
	b, err := os.ReadFile(filepath.Join(r.GitDir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{13, 40, 100, len(b) / 2} {
		if _, err := ParseIndexBytes(append(b[:n:n], make([]byte, 20)...), "sha1"); err == nil {
			t.Errorf("truncated at %d: no error", n)
		}
	}
}

func TestDecodeVarint(t *testing.T) {
	for _, c := range []struct {
		in   []byte
		want uint64
		n    int
	}{
		{[]byte{0}, 0, 1},
		{[]byte{127}, 127, 1},
		{[]byte{0x80, 0}, 128, 2},
		{[]byte{0x80, 0x7f}, 255, 2},
		{[]byte{0x81, 0}, 256, 2},
	} {
		v, n, ok := decodeVarint(c.in)
		if !ok || v != c.want || n != c.n {
			t.Errorf("decodeVarint(%x) = %d,%d,%v want %d,%d", c.in, v, n, ok, c.want, c.n)
		}
	}
	if _, _, ok := decodeVarint([]byte{0x80}); ok {
		t.Error("unterminated varint accepted")
	}
}

func TestParseSparseDirectoryTreeIDs(t *testing.T) {
	if ok, _ := fixtures.GitAtLeast(2, 37); !ok {
		t.Skip("needs git 2.37")
	}
	r := indexRepo(t)
	mustGit(t, r, "sparse-checkout", "set", "--cone", "--sparse-index", "docs")
	ix := parsed(t, r, "sha1")
	if want := mustGit(t, r, "rev-parse", "HEAD:src"); ix.SparseDirs["src/"] != want {
		t.Fatalf("src/ tree %q, want %q", ix.SparseDirs["src/"], want)
	}
}

func TestParseIndexBoundsExpansion(t *testing.T) {
	// A v4 index whose every entry keeps the previous name and adds to it:
	// names grow without bound unless capped.
	var b []byte
	b = append(b, "DIRC\x00\x00\x00\x04"...)
	const n = 2000
	b = binary.BigEndian.AppendUint32(b, n)
	for i := 0; i < n; i++ {
		b = append(b, make([]byte, 40+20)...)
		b = append(b, 0x0f, 0xff) // flags: name length 0xfff
		b = append(b, 0)          // strip nothing from the previous name
		b = append(b, bytes.Repeat([]byte("x"), 500)...)
		b = append(b, 0)
	}
	b = append(b, make([]byte, 20)...)
	if _, err := ParseIndexBytes(b, "sha1"); err == nil {
		t.Fatal("unbounded v4 expansion accepted")
	}
}
