package verify

import (
	"context"
	"os"
	"path/filepath"

	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/repo"
)

var inProgressMarkers = []struct{ rel, op string }{
	{"MERGE_HEAD", "merge"},
	{"rebase-merge", "rebase"},
	{"rebase-apply", "rebase"},
	{"CHERRY_PICK_HEAD", "cherry-pick"},
	{"REVERT_HEAD", "revert"},
	{"BISECT_LOG", "bisect"},
	{"sequencer", "sequencer"},
}

// rebaseNeeds lists a file every live rebase directory contains.
var rebaseNeeds = map[string]string{"rebase-merge": "head-name", "rebase-apply": "next"}

// OperationState lists unfinished git operations recorded in gitDir.
// stale holds rebase directories that lost the files git needs, the leftover
// a sync client can resurrect after the rebase finished.
func OperationState(gitDir string) (inProgress, stale []string) {
	seen := map[string]bool{}
	for _, m := range inProgressMarkers {
		if _, err := os.Lstat(filepath.Join(gitDir, m.rel)); err != nil {
			continue
		}
		if need, ok := rebaseNeeds[m.rel]; ok {
			if _, err := os.Lstat(filepath.Join(gitDir, m.rel, need)); err != nil {
				stale = append(stale, m.rel)
				continue
			}
		}
		if !seen[m.op] {
			inProgress = append(inProgress, m.op)
			seen[m.op] = true
		}
	}
	return inProgress, stale
}

// HasCommits reports whether HEAD resolves to a commit.
func HasCommits(ctx context.Context, g *gitx.Runner, r repo.Repo) (bool, error) {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return false, err
	}
	return res.ExitCode == 0, nil
}
