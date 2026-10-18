//go:build darwin

package scan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAllowMaterializationRunsOnceAndReturnsItsError(t *testing.T) {
	if err := PreventMaterialization(); err != nil {
		t.Fatal(err)
	}
	calls := 0
	want := errors.New("from fn")
	if err := AllowMaterialization(func() error { calls++; return want }); err != want || calls != 1 {
		t.Fatalf("err %v, calls %d", err, calls)
	}
}

// TestAllowMaterializationMovesOutOfICloud needs a real iCloud Drive folder
// (GCD_ICLOUD_DIR, a real-iCloud acceptance run): there a rename out of the
// folder fails under the process policy and works inside AllowMaterialization.
func TestAllowMaterializationMovesOutOfICloud(t *testing.T) {
	dir := os.Getenv("GCD_ICLOUD_DIR")
	if dir == "" {
		t.Skip("GCD_ICLOUD_DIR not set")
	}
	if err := PreventMaterialization(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "allow-test")
	os.RemoveAll(src)
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(src, "sub", "f"), []byte("x"), 0o644)
	dst := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(src, dst); err == nil {
		t.Log("plain rename worked here (no file provider involved?)")
		os.Rename(dst, src)
	}
	if err := AllowMaterialization(func() error { return os.Rename(src, dst) }); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "sub", "f")); err != nil || string(b) != "x" {
		t.Fatalf("%q %v", b, err)
	}
}
