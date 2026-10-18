package verify

import "testing"

// Sync clients leave truncated and mixed-up index files behind; reading one
// must never panic, whatever its bytes.
func FuzzCheckIndexBytes(f *testing.F) {
	f.Add([]byte("DIRC\x00\x00\x00\x02\x00\x00\x00\x00"), "sha1")
	f.Add([]byte("DIRC\x00\x00\x00\x04\x00\x00\x00\x01"), "sha256")
	f.Add([]byte{}, "sha1")
	f.Fuzz(func(t *testing.T, b []byte, format string) {
		CheckIndexBytes(b, format)
	})
}

// The same for the entry parser: it walks lengths and offsets taken from the
// file, so a hostile file must give an error, not a panic or a huge allocation.
func FuzzParseIndexBytes(f *testing.F) {
	f.Add([]byte("DIRC\x00\x00\x00\x02\x00\x00\x00\x00"+string(make([]byte, 20))), "sha1")
	f.Add([]byte("DIRC\x00\x00\x00\x04\xff\xff\xff\xff"+string(make([]byte, 80))), "sha1")
	f.Add([]byte("DIRC\x00\x00\x00\x02\x00\x00\x00\x00link\x00\x00\x00\x14"+string(make([]byte, 40))), "sha1")
	f.Fuzz(func(t *testing.T, b []byte, format string) {
		ix, err := ParseIndexBytes(b, format)
		if err == nil && len(ix.Paths) > len(b) {
			t.Fatalf("%d paths from %d bytes", len(ix.Paths), len(b))
		}
	})
}
