package verify

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/repo"
)

// RefRelation says how a ref copy relates to the ref it was copied from.
type RefRelation string

const (
	RelSame          RefRelation = "same"           // identical commit
	RelCopyOlder     RefRelation = "copy-older"     // copy is an ancestor: nothing would be lost
	RelCopyNewer     RefRelation = "copy-newer"     // copy has commits the ref lacks
	RelDiverged      RefRelation = "diverged"       // both have commits the other lacks
	RelMissingObject RefRelation = "missing-object" // copy names a commit that is not in the repo
	RelInvalid       RefRelation = "invalid"        // copy content is not an object id
	RelSymbolic      RefRelation = "symbolic"       // copy is "ref: ..."
	RelNoOriginal    RefRelation = "no-original"    // the ref itself does not resolve
)

// RefCopyStatus is the result of CheckRefCopy.
type RefCopyStatus struct {
	RefName     string
	CopyOID     string
	OriginalOID string
	Relation    RefRelation
}

var oidRe = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// CheckRefCopy compares the loose ref copy at copyPath with refName.
func CheckRefCopy(ctx context.Context, g *gitx.Runner, r repo.Repo, copyPath, refName string) (RefCopyStatus, error) {
	st := RefCopyStatus{RefName: refName}
	b, err := os.ReadFile(copyPath)
	if err != nil {
		return st, err
	}
	content := strings.TrimSpace(string(b))
	switch {
	case strings.HasPrefix(content, "ref: "):
		st.Relation = RelSymbolic
		return st, nil
	case !oidRe.MatchString(content):
		st.Relation = RelInvalid
		return st, nil
	}
	st.CopyOID = content
	res, err := g.Run(ctx, r.Worktree, r.Env(), "cat-file", "-e", content+"^{commit}")
	if err != nil {
		return st, err
	}
	if res.ExitCode != 0 {
		st.Relation = RelMissingObject
		return st, nil
	}
	res, err = g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "--quiet", refName+"^{commit}")
	if err != nil {
		return st, err
	}
	if res.ExitCode != 0 {
		st.Relation = RelNoOriginal
		return st, nil
	}
	st.OriginalOID = strings.TrimSpace(res.Stdout)
	if st.OriginalOID == st.CopyOID {
		st.Relation = RelSame
		return st, nil
	}
	older, err := isAncestor(ctx, g, r, st.CopyOID, st.OriginalOID)
	if err != nil {
		return st, err
	}
	if older {
		st.Relation = RelCopyOlder
		return st, nil
	}
	newer, err := isAncestor(ctx, g, r, st.OriginalOID, st.CopyOID)
	if err != nil {
		return st, err
	}
	if newer {
		st.Relation = RelCopyNewer
	} else {
		st.Relation = RelDiverged
	}
	return st, nil
}

func isAncestor(ctx context.Context, g *gitx.Runner, r repo.Repo, a, b string) (bool, error) {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "merge-base", "--is-ancestor", a, b)
	if err != nil {
		return false, err
	}
	switch res.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	}
	return false, fmt.Errorf("git merge-base: exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
}
