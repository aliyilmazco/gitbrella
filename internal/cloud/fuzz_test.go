package cloud

import "testing"

// File names come from whatever a sync client wrote; matching must never panic.
func FuzzNames(f *testing.F) {
	for _, s := range []string{"index 2", "HEAD (Jane's conflicted copy 2026-10-08)", "x.sync-conflict-20260101-090000-ABCDEF1.md", "", " 2", "(", ".git 2"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		MatchCopyName(name)
		IsSyncMetadata(name)
		IsGitDirCopy(name)
		IsNoSync(name)
	})
}
