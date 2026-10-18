//go:build darwin

package procs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAStuckPSDoesNotHang(t *testing.T) {
	// The shell waits for sleep, which keeps the output pipe open after the
	// shell is killed.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte("#!/bin/sh\nsleep 15\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	start := time.Now()
	if _, err := GitProcesses(context.Background()); err == nil {
		t.Error("GitProcesses: no error from a ps that never answered")
	}
	if d := time.Since(start); d > psTimeout+3*time.Second {
		t.Errorf("GitProcesses took %v", d)
	}
	start = time.Now()
	if !Alive(os.Getpid()) {
		t.Error("this process counts as exited")
	}
	if d := time.Since(start); d > psTimeout+3*time.Second {
		t.Errorf("Alive took %v", d)
	}
	start = time.Now()
	if _, ok := HookGit(context.Background()); ok {
		t.Error("HookGit found a git without ps")
	}
	if d := time.Since(start); d > psTimeout+3*time.Second {
		t.Errorf("HookGit took %v", d)
	}
}

func TestAZombieHasExited(t *testing.T) {
	// A git that exits between the ps snapshot and the cwd lookup stays a
	// zombie until its parent reaps it; kill(pid, 0) still succeeds on it.
	cmd := exec.Command("/usr/bin/true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	pid := cmd.Process.Pid
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		if strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			break
		}
		if time.Now().After(deadline) {
			t.Skip("the child did not become a zombie")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !exited(context.Background(), pid) {
		t.Error("a zombie counts as running")
	}
	if exited(context.Background(), os.Getpid()) {
		t.Error("this process counts as exited")
	}
}

// A process can be reaped between the kill(pid, 0) look and the ps look, and
// one the kernel is ending shows as E: neither may count as a possible lock
// holder (under load that made the check report "could not read where 1 git
// process(es) work" and move-out refuse).
func TestExitedAcrossTheReapRace(t *testing.T) {
	esrch := error(syscall.ESRCH)
	for _, tc := range []struct {
		name  string
		kills []error
		ps    string
		psErr error
		want  bool
	}{
		{"gone", []error{esrch}, "", nil, true},
		{"zombie", []error{nil}, "Z", nil, true},
		{"being ended by the kernel", []error{nil}, "?E", nil, true},
		{"reaped between the two looks", []error{nil, esrch}, "", errors.New("exit status 1"), true},
		{"running", []error{nil}, "S", nil, false},
		{"running, session leader in the foreground", []error{nil}, "Ss+", nil, false},
		{"ps failed and it is still there", []error{nil, nil}, "", errors.New("signal: killed"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			kill := func(int) error {
				err := tc.kills[min(calls, len(tc.kills)-1)]
				calls++
				return err
			}
			ps := func(int) (string, error) { return tc.ps, tc.psErr }
			if got := exitedWith(kill, ps, 42); got != tc.want {
				t.Fatalf("exited = %v, want %v", got, tc.want)
			}
		})
	}
}
