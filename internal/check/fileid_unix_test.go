//go:build unix

package check

import (
	"os"
	"path/filepath"
	"testing"
)

// A directory always has more than one link ("." and its parent's entry), so
// the hard-link rule must not give it the zero id: a submodule renamed in case
// only is still the same directory. A hard-linked file does get the zero id.
func TestFileIDKeepsDirectoryIdentity(t *testing.T) {
	d := t.TempDir()
	if id, ok := fileID(d); !ok || id == ([2]uint64{}) {
		t.Fatalf("fileID(dir) = %v, %v; want a non-zero id", id, ok)
	}
	f := filepath.Join(d, "f")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, ok := fileID(f); !ok || id == ([2]uint64{}) {
		t.Fatalf("fileID(file) = %v, %v; want a non-zero id", id, ok)
	}
	if err := os.Link(f, filepath.Join(d, "g")); err != nil {
		t.Skip("no hard links here:", err)
	}
	if id, ok := fileID(f); !ok || id != ([2]uint64{}) {
		t.Fatalf("fileID(hard-linked file) = %v, %v; want the zero id", id, ok)
	}
}
