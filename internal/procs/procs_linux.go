//go:build linux

package procs

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GitProcesses lists running git processes with their working directories.
func GitProcesses(ctx context.Context) ([]Proc, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	boot := bootTime()
	var procs []Proc
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil || !IsGitCommand(string(comm)) {
			continue
		}
		p := Proc{PID: pid}
		cwd, err := os.Readlink(filepath.Join("/proc", e.Name(), "cwd"))
		switch {
		case err == nil && strings.HasSuffix(cwd, " (deleted)"):
			continue // its directory was deleted: it works on no repository
		case err == nil:
			p.Cwd = cwd
		case errors.Is(err, fs.ErrNotExist):
			continue // exited meanwhile
		default:
			// another user's process: unknown, so it counts as a possible holder
		}
		if stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat")); err == nil && boot > 0 {
			if ticks, ok := ParseProcStatStart(string(stat)); ok {
				// USER_HZ is 100 on amd64 and arm64, the architectures built here.
				p.Start = time.Unix(boot, 0).Add(time.Duration(ticks) * 10 * time.Millisecond)
			}
		}
		procs = append(procs, p)
	}
	return withoutSelf(procs, os.Getpid()), nil
}

// bootTime reads btime (seconds since the epoch) from /proc/stat.
func bootTime() int64 {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "btime "); ok {
			n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			return n
		}
	}
	return 0
}

// HookGit finds the git process that ran this process as a hook: the nearest
// ancestor that is git. git runs commit hooks from the working tree top, so
// its directory says where the repository is even when the hook changed
// directory.
func HookGit(ctx context.Context) (Proc, bool) {
	pid := os.Getppid()
	for i := 0; i < maxHookDepth && pid > 1; i++ {
		dir := filepath.Join("/proc", strconv.Itoa(pid))
		stat, err := os.ReadFile(filepath.Join(dir, "stat"))
		if err != nil {
			return Proc{}, false
		}
		ppid, ok := ParseProcStatPPID(string(stat))
		if !ok {
			return Proc{}, false
		}
		comm, _ := os.ReadFile(filepath.Join(dir, "comm"))
		if IsGitCommand(string(comm)) {
			cwd, err := os.Readlink(filepath.Join(dir, "cwd"))
			if err != nil || strings.HasSuffix(cwd, " (deleted)") {
				return Proc{}, false
			}
			return Proc{PID: pid, Cwd: cwd}, true
		}
		pid = ppid
	}
	return Proc{}, false
}

// Alive reports whether a process with this id runs (a zombie does not).
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return !errors.Is(err, fs.ErrNotExist)
	}
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && i+2 < len(s) {
		return s[i+2] != 'Z'
	}
	return true
}
