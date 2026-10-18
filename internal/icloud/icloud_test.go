package icloud

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPending(t *testing.T) {
	for _, tc := range []struct {
		s    Status
		want bool
	}{
		{Status{}, false},
		{Status{Known: true}, false},
		{Status{Known: true, Ubiquitous: true, Uploaded: true}, false},
		{Status{Known: true, Ubiquitous: true}, true},
		{Status{Known: true, Ubiquitous: true, Uploaded: true, Uploading: true}, true},
	} {
		if got := tc.s.Pending(); got != tc.want {
			t.Errorf("%+v.Pending() = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestProbeLocalFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "file with space")
	os.WriteFile(p, []byte("x"), 0o644)
	for i := 0; i < 20; i++ { // repeated: the autorelease pool must not crash
		s, err := Probe(p)
		if err != nil {
			t.Fatal(err)
		}
		if s.Pending() || s.Ubiquitous || s.Known != (runtime.GOOS == "darwin") {
			t.Fatalf("local file: %+v", s)
		}
	}
	if _, err := Probe(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a missing path was probed")
	}
}

// TestProbeICloud runs against a real iCloud Drive item when
// GCD_ICLOUD_ITEM names one (a real-iCloud acceptance run); CI skips it.
func TestProbeICloud(t *testing.T) {
	p := os.Getenv("GCD_ICLOUD_ITEM")
	if p == "" {
		t.Skip("GCD_ICLOUD_ITEM not set")
	}
	s, err := Probe(p)
	if err != nil || !s.Known || !s.Ubiquitous {
		t.Fatalf("%s: %+v %v", p, s, err)
	}
}
