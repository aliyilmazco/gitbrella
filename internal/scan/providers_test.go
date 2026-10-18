package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/cloud"
)

// The newer clients' copies are found in the git directory: the marked
// schemes anywhere; Google Drive's " (1)" for git's names at the top and
// anywhere below (no ref name holds a space); OneDrive's device suffix only
// for git's names at the top of a OneDrive folder; the unmarked schemes only
// for git's names at the top, and Nextcloud's older "_conflict-" also below in
// a Nextcloud folder. Names a user picks with those letters stay names.
func TestCopiesOfMoreClients(t *testing.T) {
	for _, tc := range []struct {
		provider cloud.Provider
		copies   map[string]string // rel -> "kind original"
		not      []string
	}{
		{cloud.OneDrive, map[string]string{
			"index-DESKTOP-1A2B":  "onedrive index",
			"packed-refs-ALIPC":   "onedrive packed-refs",
			"HEAD (1)":            "google-drive HEAD",
			"refs/heads/main (1)": "google-drive refs/heads/main",
			"packed-refs (conflicted copy 2026-10-14 120000)":        "nextcloud packed-refs",
			"refs/heads/main (# Edit conflict 2026-10-14 X1Y2Z3A #)": "proton refs/heads/main",
			"packed-refs_conflict-20261014-120000":                   "nextcloud packed-refs",
		}, []string{"refs/heads/release-V2", "config-backup", "index-old", "hooks-disabled",
			"refs/heads/fix.Conflict", "refs/heads/resolve.conflicted", "refs/heads/merge_conflict-20240101-120000",
			"refs/tags/v1.0.Conflict", "logs/refs/heads/fix.Conflict", "worktrees/wt.Conflict"}},
		{cloud.None, map[string]string{
			"HEAD (1)": "google-drive HEAD",
		}, []string{"index-DESKTOP-1A2B", "refs/heads/main_conflict-20261014-120000"}},
		{cloud.Nextcloud, map[string]string{
			"refs/heads/main_conflict-20261014-120000":      "nextcloud refs/heads/main",
			"logs/refs/heads/main_conflict-20261014-120000": "nextcloud logs/refs/heads/main",
			"worktrees/wt/index_conflict-20261014-120000":   "nextcloud worktrees/wt/index",
			"worktrees/wt/HEAD_conflict-20261014-120000":    "nextcloud worktrees/wt/HEAD",
		}, []string{"index-DESKTOP-1A2B", "refs/heads/fix.Conflict", "refs/heads/merge_conflict-20240101-120000", "hooks/pre-commit_conflict-20261014-120000"}},
	} {
		g := filepath.Join(t.TempDir(), ".git")
		files := map[string]string{
			"HEAD": "ref: refs/heads/main\n", "index": "DIRC", "config": "[core]\n", "packed-refs": "# pack-refs\n",
			"refs/heads/main": "x\n", "logs/refs/heads/main": "x\n", "refs/heads/release": "x\n", "hooks/pre-commit.sample": "x\n", "hooks/pre-commit": "x\n",
			"worktrees/wt/index": "DIRC", "worktrees/wt/HEAD": "x\n", "worktrees/wt/gitdir": "/x/.git\n",
		}
		for rel := range tc.copies {
			files[rel] = "x\n"
		}
		for _, rel := range tc.not {
			files[rel] = "x\n"
		}
		files["worktrees/wt.Conflict/HEAD"] = "x\n"
		files["hooks-disabled/pre-commit"] = "x\n"
		delete(files, "hooks-disabled")
		delete(files, "worktrees/wt.Conflict")
		for rel, content := range files {
			p := filepath.Join(g, filepath.FromSlash(rel))
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte(content), 0o644)
		}
		res, err := Scan(g, fakeFlags{}, time.Now(), tc.provider)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, c := range res.Copies {
			got[c.Rel] = string(c.Kind) + " " + c.OriginalRel
		}
		for rel, want := range tc.copies {
			if got[rel] != want {
				t.Errorf("%s: %s: %q, want %q", tc.provider, rel, got[rel], want)
			}
		}
		for _, rel := range tc.not {
			if k, ok := got[rel]; ok {
				t.Errorf("%s: %s taken for a copy: %s", tc.provider, rel, k)
			}
		}
	}
}
