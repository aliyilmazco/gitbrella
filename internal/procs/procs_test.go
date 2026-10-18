package procs

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestParsePS(t *testing.T) {
	out := "  1  10-00:00:01 Ss /sbin/launchd\n 42 05:07 S git\n 43    1:02:03 S+ /opt/homebrew/bin/git\n 44 2-03:04:05 S git-remote-https\n 45 00:01 S gitk\n 46 00:01 S /Applications/My App/helper\n 47 00:02 R /Applications/My App/git\n 48 ?? S git\n 49 00:03 Z git\n"
	want := []PSEntry{
		{PID: 42, Elapsed: 5*time.Minute + 7*time.Second},
		{PID: 43, Elapsed: time.Hour + 2*time.Minute + 3*time.Second},
		{PID: 44, Elapsed: 2*24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second},
		{PID: 47, Elapsed: 2 * time.Second},
		{PID: 48, Elapsed: -1}, // unreadable etime: kept, start unknown
		// 49 is a zombie: it has exited and holds nothing
	}
	if got := ParsePS(out); !slices.Equal(got, want) {
		t.Fatalf("ParsePS = %+v", got)
	}
}

func TestParseProcStatStart(t *testing.T) {
	// comm may hold spaces and parentheses; fields are counted after the last ')'
	stat := "4242 (git (x) y) S 1 4242 4242 0 -1 4194304 100 0 0 0 1 2 0 0 20 0 1 0 98765 1000 10"
	if got, ok := ParseProcStatStart(stat); !ok || got != 98765 {
		t.Fatalf("ParseProcStatStart = %d %v", got, ok)
	}
	if _, ok := ParseProcStatStart("garbage"); ok {
		t.Fatal("garbage parsed")
	}
}

func TestWithinResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	real, _ := filepath.EvalSymlinks(dir)
	if !Within(Proc{Cwd: filepath.Join(real, "sub")}, dir) && !Within(Proc{Cwd: real}, dir) {
		t.Fatal("Within must match a cwd inside dir")
	}
	if Within(Proc{Cwd: real + "-other"}, dir) {
		t.Fatal("prefix without separator must not match")
	}
}

func TestGitProcessesFindsARunningGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); cmd.Wait() }()
	time.Sleep(300 * time.Millisecond)
	procs, err := GitProcesses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range procs {
		if p.PID == cmd.Process.Pid && Within(p, dir) {
			return
		}
	}
	t.Fatalf("running git (pid %d) in %s not found in %+v", cmd.Process.Pid, dir, procs)
}

func TestOwnToolIsNotAGitProcess(t *testing.T) {
	cases := map[string]bool{
		"git": true, "/usr/bin/git": true, "git-upload-pack": true, "git-remote-https": true,
		"gitbrella": false, "/usr/local/bin/gitbrella": false,
	}
	for comm, want := range cases {
		if got := IsGitCommand(comm); got != want {
			t.Errorf("IsGitCommand(%q) = %v, want %v", comm, got, want)
		}
	}
}

func TestGitProcessesReportCwdAndStartInATurkishPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(t.TempDir(), "Projeler Çalışma Ğİş")
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	before := time.Now()
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); cmd.Wait() }()
	time.Sleep(1500 * time.Millisecond)
	ps, err := GitProcesses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	for _, p := range ps {
		if p.PID != cmd.Process.Pid {
			continue
		}
		if got, _ := filepath.EvalSymlinks(p.Cwd); got != want {
			t.Fatalf("cwd = %q, want %q", p.Cwd, want)
		}
		if p.Start.IsZero() || p.Start.Before(before.Add(-2*time.Second)) || p.Start.After(time.Now()) {
			t.Fatalf("start = %v, started at about %v", p.Start, before)
		}
		return
	}
	t.Fatalf("running git (pid %d) not found in %+v", cmd.Process.Pid, ps)
}

func TestParsePSParent(t *testing.T) {
	ppid, comm, ok := ParsePSParent("  4242 /opt/homebrew/bin/git\n")
	if !ok || ppid != 4242 || comm != "/opt/homebrew/bin/git" {
		t.Fatalf("ParsePSParent = %d %q %v", ppid, comm, ok)
	}
	if _, comm, ok := ParsePSParent("17 /Applications/My App/git"); !ok || comm != "/Applications/My App/git" {
		t.Fatalf("comm with spaces: %q %v", comm, ok)
	}
	if _, _, ok := ParsePSParent(""); ok {
		t.Fatal("empty parsed")
	}
}

func TestParseProcStatPPID(t *testing.T) {
	if got, ok := ParseProcStatPPID("4242 (git (x) y) S 17 4242 4242 0"); !ok || got != 17 {
		t.Fatalf("ParseProcStatPPID = %d %v", got, ok)
	}
}
