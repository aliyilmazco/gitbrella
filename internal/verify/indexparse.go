package verify

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// Index is what an index file records, read without git: git never touches
// the file (reading a split index through git refreshes sharedindex.*).
type Index struct {
	Version     uint32
	Paths       []string          // entry paths in index order; sparse directory entries end in "/"
	SparseDirs  map[string]string // sparse directory entry ("dir/") -> its tree id (hex)
	IntentToAdd map[string]bool   // entries added with git add -N: listed, but a commit adds no content
	Extensions  []string          // extension signatures in file order
}

// Split reports whether the index is the split part of a split index (a
// "link" extension): the shared index holds the other entries.
func (ix Index) Split() bool { return ix.has("link") }

// Sparse reports whether the index is a sparse index ("sdir" extension):
// directory entries stand for every path below them.
func (ix Index) Sparse() bool { return ix.has("sdir") }

func (ix Index) has(sig string) bool {
	for _, e := range ix.Extensions {
		if e == sig {
			return true
		}
	}
	return false
}

// ParseIndexFile reads the entries and extensions of an index file. Callers
// must make sure the file is not dataless.
func ParseIndexFile(path, objectFormat string) (Index, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Index{}, err
	}
	return ParseIndexBytes(b, objectFormat)
}

var errTruncated = errors.New("index entries run past the end of the file")

// ParseIndexBytes reads the entries (versions 2, 3 and 4, including v4 path
// compression and sparse directory entries) and the extensions of an index.
// It does not verify the checksum; CheckIndexBytes does.
func ParseIndexBytes(b []byte, objectFormat string) (Index, error) {
	hashLen := sha1.Size
	if objectFormat == "sha256" {
		hashLen = sha256.Size
	}
	if len(b) < 12+hashLen || string(b[:4]) != "DIRC" {
		return Index{}, errors.New("not an index file")
	}
	ix := Index{Version: binary.BigEndian.Uint32(b[4:8])}
	if ix.Version < 2 || ix.Version > 4 {
		return Index{}, fmt.Errorf("unsupported index version %d", ix.Version)
	}
	count := binary.BigEndian.Uint32(b[8:12])
	end := len(b) - hashLen // extensions and entries stop at the trailer
	nameOff := 40 + hashLen + 2
	// Never trust the header for an allocation, and bound what v4 prefix
	// compression can expand to: a real name is far below either limit.
	ix.Paths = make([]string, 0, min(int(count), end/nameOff))
	const maxName = 1 << 16
	budget := 64*len(b) + 1<<20
	off := 12
	var prev []byte
	for i := uint32(0); i < count; i++ {
		start := off
		if end-off < nameOff {
			return Index{}, errTruncated
		}
		flags := binary.BigEndian.Uint16(b[off+nameOff-2 : off+nameOff])
		oid := b[off+40 : off+40+hashLen]
		off += nameOff
		intentToAdd := false
		if flags&0x4000 != 0 { // extended flags (v3 and later)
			if ix.Version < 3 {
				return Index{}, errors.New("extended flags in a version 2 index")
			}
			if end-off < 2 {
				return Index{}, errTruncated
			}
			intentToAdd = binary.BigEndian.Uint16(b[off:off+2])&0x2000 != 0
			off += 2
		}
		var name []byte
		if ix.Version == 4 {
			strip, n, ok := decodeVarint(b[off:end])
			if !ok {
				return Index{}, errTruncated
			}
			off += n
			if strip > uint64(len(prev)) {
				return Index{}, errors.New("v4 path strips more than the previous path")
			}
			nul := bytes.IndexByte(b[off:end], 0)
			if nul < 0 {
				return Index{}, errTruncated
			}
			name = append(append([]byte{}, prev[:len(prev)-int(strip)]...), b[off:off+nul]...)
			off += nul + 1
		} else {
			nul := bytes.IndexByte(b[off:end], 0)
			if nul < 0 {
				return Index{}, errTruncated
			}
			name = b[off : off+nul]
			// The entry is NUL-padded to a multiple of eight bytes, with at
			// least one NUL after the name.
			size := (off - start + nul + 8) &^ 7
			if end-start < size {
				return Index{}, errTruncated
			}
			off = start + size
		}
		if len(name) > maxName {
			return Index{}, errors.New("index path too long")
		}
		if budget -= len(name); budget < 0 {
			return Index{}, errors.New("index paths expand past any real index")
		}
		ix.Paths = append(ix.Paths, string(name))
		if intentToAdd {
			if ix.IntentToAdd == nil {
				ix.IntentToAdd = map[string]bool{}
			}
			ix.IntentToAdd[string(name)] = true
		}
		if len(name) > 0 && name[len(name)-1] == '/' {
			if ix.SparseDirs == nil {
				ix.SparseDirs = map[string]string{}
			}
			ix.SparseDirs[string(name)] = hex.EncodeToString(oid)
		}
		prev = name
	}
	for off < end {
		if end-off < 8 {
			return Index{}, errors.New("truncated extension header")
		}
		sig := string(b[off : off+4])
		size := binary.BigEndian.Uint32(b[off+4 : off+8])
		off += 8
		if uint64(size) > uint64(end-off) {
			return Index{}, fmt.Errorf("extension %q runs past the end of the file", sig)
		}
		ix.Extensions = append(ix.Extensions, sig)
		off += int(size)
	}
	return ix, nil
}

// decodeVarint reads git's offset varint (varint.c): each continuation adds
// one before shifting, so encodings are unique.
func decodeVarint(b []byte) (uint64, int, bool) {
	if len(b) == 0 {
		return 0, 0, false
	}
	c := b[0]
	val := uint64(c & 127)
	n := 1
	for c&128 != 0 {
		if n >= len(b) || n > 9 {
			return 0, 0, false
		}
		val++
		c = b[n]
		n++
		val = val<<7 | uint64(c&127)
	}
	return val, n, true
}
