// Package verify checks candidate index files, ref copies and repository
// state. Callers must make sure a file is not dataless before passing it here.
package verify

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/repo"
)

// IndexStatus is the result of validating an index file on its own.
type IndexStatus struct {
	Valid        bool   // header and trailing checksum verified
	Unverifiable bool   // trailer is all zeros (index.skipHash); header looked sane
	Version      uint32 // 2, 3 or 4 when the header was readable
	Entries      uint32
	Reason       string // why the file is not valid
}

// CheckIndexFile validates an index file. git's own ls-files and diff-index
// do not verify the checksum, so this must run before trusting a candidate.
func CheckIndexFile(path, objectFormat string) IndexStatus {
	b, err := os.ReadFile(path)
	if err != nil {
		return IndexStatus{Reason: "read failed: " + err.Error()}
	}
	return CheckIndexBytes(b, objectFormat)
}

// CheckIndexBytes validates the bytes of an index file.
func CheckIndexBytes(b []byte, objectFormat string) IndexStatus {
	hashLen := sha1.Size
	if objectFormat == "sha256" {
		hashLen = sha256.Size
	}
	if len(b) < 12+hashLen {
		return IndexStatus{Reason: "file too short"}
	}
	if string(b[:4]) != "DIRC" {
		return IndexStatus{Reason: "missing DIRC signature"}
	}
	st := IndexStatus{Version: binary.BigEndian.Uint32(b[4:8]), Entries: binary.BigEndian.Uint32(b[8:12])}
	if st.Version < 2 || st.Version > 4 {
		st.Reason = fmt.Sprintf("unsupported index version %d", st.Version)
		return st
	}
	body, trailer := b[:len(b)-hashLen], b[len(b)-hashLen:]
	if bytes.Equal(trailer, make([]byte, hashLen)) {
		st.Unverifiable = true
		st.Reason = "checksum disabled (index.skipHash)"
		return st
	}
	var sum []byte
	if hashLen == sha256.Size {
		s := sha256.Sum256(body)
		sum = s[:]
	} else {
		s := sha1.Sum(body)
		sum = s[:]
	}
	if !bytes.Equal(sum, trailer) {
		st.Reason = "checksum mismatch"
		return st
	}
	st.Valid = true
	return st
}

// IndexEqualsHEAD reports whether the index at indexPath records exactly the
// tree of HEAD. It works on a private copy so git can never touch the original.
func IndexEqualsHEAD(ctx context.Context, g *gitx.Runner, r repo.Repo, indexPath string) (bool, error) {
	root, err := filepath.Abs(os.TempDir()) // git runs elsewhere: a relative TMPDIR would miss
	if err != nil {
		return false, err
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	// Compared by file identity: on a case-insensitive disk another spelling
	// names the same directory.
	var repoDirs []os.FileInfo
	for _, d := range []string{r.Worktree, r.GitDir, r.CommonDir} {
		if fi, err := os.Stat(d); d != "" && err == nil {
			repoDirs = append(repoDirs, fi)
		}
	}
	for d := root; ; d = filepath.Dir(d) {
		if fi, err := os.Stat(d); err == nil {
			for _, rd := range repoDirs {
				if os.SameFile(fi, rd) {
					return false, errors.New("the temporary directory is inside the repository; the copy would be written there")
				}
			}
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	tmp, err := os.MkdirTemp(root, "gitbrella-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)
	candidate := filepath.Join(tmp, "index")
	if err := copyFile(indexPath, candidate); err != nil {
		return false, err
	}
	res, err := g.Run(ctx, r.Worktree, append(r.Env(), "GIT_INDEX_FILE="+candidate), "diff-index", "--cached", "--quiet", "--ignore-submodules=none", "HEAD")
	if err != nil {
		return false, err
	}
	switch res.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	}
	return false, fmt.Errorf("git diff-index: exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
