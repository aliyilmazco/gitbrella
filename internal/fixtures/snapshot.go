package fixtures

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// FileMeta is what a snapshot records about one path. Contents are hashed only
// for small files outside objects/, so taking a snapshot never reads objects.
type FileMeta struct {
	Size    int64       `json:"size"`
	ModTime time.Time   `json:"modTime"`
	Mode    fs.FileMode `json:"mode"`
	Inode   uint64      `json:"inode"`
	Hash    string      `json:"hash,omitempty"`
}

// Snapshot maps slash-separated relative paths to their metadata.
type Snapshot map[string]FileMeta

const maxHashed = 1 << 20

// TakeSnapshot records every path under root. skipContent reports paths whose
// contents must not be read (for example dataless cloud files).
func TakeSnapshot(root string, skipContent func(path string) bool) (Snapshot, error) {
	snap := Snapshot{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		m := FileMeta{Size: info.Size(), ModTime: info.ModTime().UTC(), Mode: info.Mode()}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			m.Inode = uint64(st.Ino)
		}
		inObjects := strings.Contains("/"+rel+"/", "/objects/")
		if info.Mode().IsRegular() && info.Size() <= maxHashed && !inObjects && (skipContent == nil || !skipContent(p)) {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			m.Hash = hex.EncodeToString(sum[:])
		}
		snap[rel] = m
		return nil
	})
	return snap, err
}

// Diff lists every path that was added, removed or changed between a and b.
func (a Snapshot) Diff(b Snapshot) []string {
	var out []string
	for p, ma := range a {
		mb, ok := b[p]
		switch {
		case !ok:
			out = append(out, "removed: "+p)
		case !ma.ModTime.Equal(mb.ModTime) || ma.Size != mb.Size || ma.Mode != mb.Mode || ma.Inode != mb.Inode || ma.Hash != mb.Hash:
			out = append(out, "changed: "+p)
		}
	}
	for p := range b {
		if _, ok := a[p]; !ok {
			out = append(out, "added: "+p)
		}
	}
	sort.Strings(out)
	return out
}

// Save writes the snapshot as JSON.
func (a Snapshot) Save(path string) error {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadSnapshot reads a snapshot written by Save.
func LoadSnapshot(path string) (Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}
