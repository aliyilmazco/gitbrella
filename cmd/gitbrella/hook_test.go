package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/scan"
)

// Which git commands a shell command runs, and whether they write: wrappers,
// environment assignments, git's own options, chains and shell keywords are
// seen through; git where the reading does not follow it counts as a write;
// the core.worktree remedy counts as neither.
func TestDescribeCall(t *testing.T) {
	for cmd, want := range map[string]string{ // "git writes" as y/n
		"ls -la":                                           "nn",
		"git status":                                       "yn",
		"git -C ../other log --oneline | head":             "yn",
		"GIT_PAGER=cat git diff && git add -A":             "yy",
		"cd sub && sudo git commit -m 'a b'":               "yy",
		"git restore --staged 'notes 2.md'":                "yn",
		"git branch":                                       "yn",
		"git branch -D old":                                "yy",
		"git config --get user.name":                       "yn",
		"git config user.name x":                           "yy",
		"gitbrella fix --apply":                            "nn",
		"echo $(git rev-parse HEAD)":                       "yn",
		"git stash list; git stash":                        "yy",
		"npm test":                                         "nn",
		"/usr/bin/git -c core.pager=cat push":              "yy",
		"if ! git diff --quiet; then git commit -am x; fi": "yy",
		`for f in a.txt; do git add "$f"; done`:            "yy",
		"! git commit -m x":                                "yy",
		"timeout 30 git status":                            "yn",
		`\git status`:                                      "yn",
		"sudo -u bob git status":                           "yn",
		"ls | xargs git add":                               "yy",
		`echo "(cd x && git commit)" > notes.txt`:          "nn",
		"git commit -m 'wip; cd ../dmg && git push'":       "yy",
		"bash -c 'git status'":                             "yn",
		"cd / && git config --file /r/.git/config --unset core.worktree":       "nn",
		"git config --file /r/.git/config --unset core.worktree && git commit": "yy",
		"git commit -m 'oops":        "yy",
		"git remote -v":              "yn",
		"git remote -v add x /tmp/x": "yy",
		"git diff --output=/tmp/x":   "yy",
		"git fsck --lost-found":      "yy",
		"gh pr checkout 12":          "yy",
		"git-lfs ls-files":           "yn",
		"git-lfs install":            "yy",
		"git lfs logs":               "yn",
		"git lfs logs clear":         "yy",
	} {
		ev := hookEvent{Tool: "Bash"}
		ev.Input.Command = shellCommand(cmd)
		c := describeCall(ev, nil, shellRuns(ev))
		got := map[bool]string{true: "y", false: "n"}[c.git] + map[bool]string{true: "y", false: "n"}[c.writesGit]
		if got != want {
			t.Errorf("%q: git/writes %s, want %s", cmd, got, want)
		}
	}
}

// The files a call works on: relative paths are the session's, a Codex patch
// names its own; a write inside a git directory is seen as such and checked
// from the working tree above it.
func TestHookTargetsAndDirs(t *testing.T) {
	root := t.TempDir()
	ev := hookEvent{Cwd: root}
	ev.Input.FilePath = "a/b.txt"
	if got := hookTargets(ev); len(got) != 1 || got[0] != filepath.Join(root, "a", "b.txt") {
		t.Fatalf("relative: %q", got)
	}
	os.MkdirAll(filepath.Join(root, "repo", ".git", "info"), 0o755)
	ev.Input.FilePath = filepath.Join(root, "repo", ".git", "info", "exclude")
	targets := hookTargets(ev)
	if c := describeCall(ev, targets, nil); !c.intoGitDir {
		t.Fatal("a write into .git not seen")
	}
	real, _ := filepath.EvalSymlinks(root)
	if d, _, _ := hookDirs(ev, targets, nil); len(d) != 1 || d[0].path != filepath.Join(real, "repo") || !d[0].into {
		t.Fatalf("dir of a write into .git: %+v", d)
	}
	patch := hookEvent{Cwd: root, Tool: "apply_patch"}
	patch.Input.Command = shellCommand("*** Begin Patch\n  *** Update File: x/y.go\n@@\n*** Add File: z.md\r\n*** End Patch\n")
	if got := hookTargets(patch); len(got) != 2 || !strings.HasSuffix(got[0], "x/y.go") || !strings.HasSuffix(got[1], "z.md") {
		t.Fatalf("patch: %q", got)
	}
	if c := describeCall(patch, hookTargets(patch), shellRuns(patch)); c.git {
		t.Fatal("a patch taken for a git command")
	}
}

// The context names the sync client and the damage, says nothing once the
// git directory is out of the synced folder and nothing is damaged.
func TestSessionContext(t *testing.T) {
	synced := check.Report{Repo: check.RepoInfo{Worktree: "/w", Provider: "icloud"}}
	if got := sessionContext(synced, nil); !strings.Contains(got, "icloud syncs") || !strings.Contains(got, "move-out") {
		t.Fatalf("%q", got)
	}
	moved := synced
	moved.Findings = []check.Finding{{Code: check.CodeGitDirNotSynced, Severity: check.Info}}
	if got := sessionContext(moved, nil); got != "" {
		t.Fatalf("moved out: %q", got)
	}
	damage := []check.Finding{{Code: check.CodeIndexCopy, Path: "index 2", Severity: check.Damage}}
	if got := sessionContext(moved, damage); !strings.Contains(got, "INDEX_COPY index 2") {
		t.Fatalf("damage: %q", got)
	}
	if got := sessionContext(check.Report{Repo: check.RepoInfo{Provider: "none"}}, nil); got != "" {
		t.Fatalf("not synced: %q", got)
	}
}

// The hook's --timeout covers the hints too: where git cannot use the
// repository, a hint that hangs (a FIFO for a config, a stalled disk) lets
// the call through with a notice instead of holding the agent.
func TestHookTimeoutCoversTheHints(t *testing.T) {
	dir := t.TempDir() // no repository: the hints are read
	block := make(chan struct{})
	defer close(block)
	defer func(f func(string, scan.FlagReader) []string) { notRepoHints = f }(notRepoHints)
	entered := make(chan struct{}, 1)
	notRepoHints = func(string, scan.FlagReader) []string { entered <- struct{}{}; <-block; return nil }
	ev := fmt.Sprintf(`{"cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status"}}`, dir)
	done := make(chan int, 1)
	var stderr strings.Builder
	go func() {
		done <- runHook([]string{"--for", "claude", "--timeout", "1s"}, strings.NewReader(ev), io.Discard, &stderr)
	}()
	select {
	case code := <-done:
		if code != 1 || !strings.Contains(stderr.String(), "longer than 1s") {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the hook waits on the hints past its --timeout")
	}
	<-entered // the abandoned reading has read notRepoHints before the deferred restore writes it
}

// The folders checked for a shell command, and what the call does in each:
// the folders its writing git commands run in first, then the session's,
// then the rest; a folder inside a git directory counts as the working tree
// above it; a git command whose folder the reading cannot tell counts in
// every one; a writing command's folder that does not fit is reported.
func TestHookDirsOfAShellCommand(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"inner", "sound/.git/hooks", "dmg", "r1", "r2", "r3", "r4", "r5", "r6"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	real, _ := filepath.EvalSymlinks(root)
	for _, tc := range []struct {
		cwd, cmd, want string // want: "dir:flags", flags w writes, g reads, - nothing
		dropped        bool
	}{
		{"", "git -C inner commit -m x", "inner:w .:-", false},
		{"", "git -C inner status", ".:- inner:g", false},
		{"sound/.git", "git log -1", "sound:g", false},
		{"", "cd sound/.git/hooks && git status", ".:- sound:g", false},
		{"dmg", "git -C ../inner commit", "inner:w dmg:-", false},
		{"", "(cd dmg && git status) && git -C inner commit", "inner:w .:- dmg:g", false},
		{"", "cd $HOME && git commit", ".:w", false},
		{"", "cd / && git config --file /x/.git/config --unset core.worktree", ".:-", false},
		{"", "git -C r1 status; git -C r2 status; git -C r3 status; git -C r4 status; git -C r6 commit", "r6:w .:- r1:g r2:g r3:g", true},
		{"", "git -C r1 add .; git -C r2 add .; git -C r3 add .; git -C r4 add .; git -C r5 add .; git -C r6 add .", "r1:w r2:w r3:w r4:w r5:w", true},
	} {
		ev := hookEvent{Cwd: filepath.Join(root, tc.cwd), Tool: "Bash"}
		ev.Input.Command = shellCommand(tc.cmd)
		dirs, dropped, _ := hookDirs(ev, nil, shellRuns(ev))
		var got []string
		for _, d := range dirs {
			rel, _ := filepath.Rel(real, d.path)
			f := "-"
			switch {
			case d.writes:
				f = "w"
			case d.git:
				f = "g"
			}
			got = append(got, rel+":"+f)
		}
		if g := strings.Join(got, " "); g != tc.want || dropped != tc.dropped {
			t.Errorf("%s in %q: %q dropped=%v, want %q dropped=%v", tc.cmd, tc.cwd, g, dropped, tc.want, tc.dropped)
		}
	}
}

// Some tools pass the command as an argument list: it is read as the words
// it is, a shell's -c script included.
func TestShellCommandAsAList(t *testing.T) {
	for in, want := range map[string]string{
		`["bash","-lc","git commit -am x"]`:               "/w:commit",
		`["bash","-e","-c","cd dmg && git commit -am x"]`: "/w/dmg:commit",
		`["git","-C","sp dmg","commit"]`:                  "/w/sp dmg:commit",
		`["git","status"]`:                                "/w:status",
	} {
		var ev hookEvent
		if err := json.Unmarshal([]byte(`{"cwd":"/w","tool_name":"Bash","tool_input":{"command":`+in+`}}`), &ev); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		var got []string
		for _, r := range shellRuns(ev) {
			got = append(got, r.Dir+":"+r.Args[0])
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// The --timeout covers the check as well, even one stuck on a read that does
// not return (a FIFO, a stalled disk): the call goes through with a notice.
func TestHookTimeoutCoversTheCheck(t *testing.T) {
	dir := t.TempDir()
	block := make(chan struct{})
	defer close(block)
	defer func(f func(context.Context, *gitx.Runner, check.Options) (check.Report, error)) { hookRunCheck = f }(hookRunCheck)
	entered := make(chan struct{}, 1)
	hookRunCheck = func(context.Context, *gitx.Runner, check.Options) (check.Report, error) {
		entered <- struct{}{}
		<-block
		return check.Report{}, nil
	}
	ev := fmt.Sprintf(`{"cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m x"}}`, dir)
	done := make(chan int, 1)
	var stderr strings.Builder
	go func() {
		done <- runHook([]string{"--for", "claude", "--timeout", "1s"}, strings.NewReader(ev), io.Discard, &stderr)
	}()
	select {
	case code := <-done:
		if code != 1 || !strings.Contains(stderr.String(), "longer than 1s") {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the hook waits on the check past its --timeout")
	}
	<-entered // the abandoned check has read hookRunCheck before the deferred restore writes it
}

// A reading of the command that goes on past half the --timeout gives up:
// the command counts as an unknown write, so a damaged session folder still
// stops it.
func TestHookTimeoutCoversTheReading(t *testing.T) {
	dir := t.TempDir()
	block := make(chan struct{})
	defer close(block)
	defer func(f func(string, string) []gitRun) { hookReadShell = f }(hookReadShell)
	entered := make(chan struct{}, 1)
	hookReadShell = func(string, string) []gitRun { entered <- struct{}{}; <-block; return nil }
	defer func(f func(context.Context, *gitx.Runner, check.Options) (check.Report, error)) { hookRunCheck = f }(hookRunCheck)
	hookRunCheck = func(context.Context, *gitx.Runner, check.Options) (check.Report, error) {
		return check.Report{Findings: []check.Finding{{Code: check.CodeIndexMissing, Severity: check.Damage, Path: "index"}}}, nil
	}
	ev := fmt.Sprintf(`{"cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m x"}}`, dir)
	done := make(chan int, 1)
	var stderr strings.Builder
	go func() {
		done <- runHook([]string{"--for", "claude", "--timeout", "2s"}, strings.NewReader(ev), io.Discard, &stderr)
	}()
	select {
	case code := <-done:
		if code != 2 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the hook waits on the reading past its --timeout")
	}
	<-entered // the abandoned reading has read hookReadShell before the deferred restore writes it
}
