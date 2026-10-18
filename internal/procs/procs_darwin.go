//go:build darwin

package procs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// GitProcesses lists running git processes with their working directories
// and start times. The working directory comes from proc_pidinfo, which
// returns the path as bytes (lsof escapes non-ASCII names as \xNN).
func GitProcesses(ctx context.Context) ([]Proc, error) {
	ctx, cancel := context.WithTimeout(ctx, psTimeout)
	defer cancel()
	out, err := psOutput(ctx, "-axo", "pid=,etime=,stat=,comm=")
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	now := time.Now()
	var procs []Proc
	for _, e := range ParsePS(string(out)) {
		// etime is truncated to whole seconds: take the earliest possible start,
		// which errs towards "this git could hold the lock".
		p := Proc{PID: e.PID}
		if e.Elapsed >= 0 {
			p.Start = now.Add(-e.Elapsed - time.Second)
		}
		cwd, err := processCwd(e.PID)
		switch {
		case err == nil:
			p.Cwd = cwd
		case exited(ctx, e.PID):
			continue // exited meanwhile
		default:
			// another user's process: unknown, so it counts as a possible holder
		}
		procs = append(procs, p)
	}
	return withoutSelf(procs, os.Getpid()), nil
}

// From <sys/proc_info.h>: PROC_PIDVNODEPATHINFO returns struct
// proc_vnodepathinfo { vnode_info_path pvi_cdir, pvi_rdir }, where
// vnode_info_path is a 152-byte vnode_info followed by char vip_path[1024].
const (
	procPIDVnodePathInfo  = 9
	vnodeInfoSize         = 152
	maxPathLen            = 1024
	procVnodePathInfoSize = 2 * (vnodeInfoSize + maxPathLen)
)

var (
	pidinfoOnce sync.Once
	pidinfo     func(pid, flavor int32, arg uint64, buf unsafe.Pointer, size int32) int32
	pidinfoErr  error
)

func processCwd(pid int) (string, error) {
	pidinfoOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			pidinfoErr = err
			return
		}
		sym, err := purego.Dlsym(lib, "proc_pidinfo")
		if err != nil {
			pidinfoErr = err
			return
		}
		purego.RegisterFunc(&pidinfo, sym)
	})
	if pidinfoErr != nil {
		return "", pidinfoErr
	}
	buf := make([]byte, procVnodePathInfoSize)
	n := pidinfo(int32(pid), procPIDVnodePathInfo, 0, unsafe.Pointer(&buf[0]), int32(len(buf)))
	runtime.KeepAlive(buf)
	if n != procVnodePathInfoSize {
		return "", fmt.Errorf("proc_pidinfo(%d): returned %d", pid, n)
	}
	path := buf[vnodeInfoSize : vnodeInfoSize+maxPathLen]
	if i := strings.IndexByte(string(path), 0); i >= 0 {
		path = path[:i]
	}
	if len(path) == 0 {
		return "", fmt.Errorf("proc_pidinfo(%d): no working directory", pid)
	}
	return string(path), nil
}

// HookGit finds the git process that ran this process as a hook: the nearest
// ancestor that is git. git runs commit hooks from the working tree top, so
// its directory says where the repository is even when the hook changed
// directory.
func HookGit(ctx context.Context) (Proc, bool) {
	ctx, cancel := context.WithTimeout(ctx, psTimeout)
	defer cancel()
	pid := os.Getppid()
	for i := 0; i < maxHookDepth && pid > 1; i++ {
		out, err := psOutput(ctx, "-o", "ppid=,comm=", "-p", strconv.Itoa(pid))
		if err != nil {
			return Proc{}, false
		}
		ppid, comm, ok := ParsePSParent(string(out))
		if !ok {
			return Proc{}, false
		}
		if IsGitCommand(comm) {
			cwd, err := processCwd(pid)
			if err != nil {
				return Proc{}, false
			}
			return Proc{PID: pid, Cwd: cwd}, true
		}
		pid = ppid
	}
	return Proc{}, false
}

// psTimeout bounds the ps calls of one listing or one hook walk: a ps that
// hangs must not hang a commit. On expiry the callers' safe paths apply
// (processes unknown, no anchor).
const psTimeout = 5 * time.Second

func psOutput(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ps", args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.WaitDelay = time.Second // a child of a killed ps may hold the pipe open
	return cmd.Output()
}

// exited reports whether pid has exited: it is gone, or a zombie its parent
// has not reaped yet (kill(pid, 0) still succeeds on those).
func exited(ctx context.Context, pid int) bool {
	ps := func(pid int) (string, error) {
		out, err := psOutput(ctx, "-o", "stat=", "-p", strconv.Itoa(pid))
		return string(out), err
	}
	return exitedWith(func(pid int) error { return syscall.Kill(pid, 0) }, ps, pid)
}

// exitedWith is exited with the two looks replaceable: kill(pid, 0) and ps's
// state letters. A process reaped between the two makes ps find nothing (it
// fails); the second kill then says it is gone. ps marks a process the kernel
// is ending with E: its user code has finished and holds no lock.
func exitedWith(kill func(int) error, ps func(int) (string, error), pid int) bool {
	if kill(pid) == syscall.ESRCH {
		return true
	}
	stat, err := ps(pid)
	if err != nil {
		return kill(pid) == syscall.ESRCH
	}
	stat = strings.TrimSpace(stat)
	return strings.HasPrefix(stat, "Z") || strings.Contains(stat, "E")
}

// Alive reports whether a process with this id runs (a zombie does not).
func Alive(pid int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	return pid > 0 && !exited(ctx, pid)
}
