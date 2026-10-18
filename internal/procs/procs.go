// Package procs finds running git processes, their working directories and
// start times. A lock file cannot tell whether it is live (git closes it
// before it renames it), so liveness is judged from the git processes working
// on the repository and from when they started.
package procs

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Proc is a running git process.
type Proc struct {
	PID   int
	Cwd   string    // "" when it could not be read
	Start time.Time // zero when unknown
}

// PSEntry is one git process from `ps -axo pid=,etime=,stat=,comm=`.
type PSEntry struct {
	PID     int
	Elapsed time.Duration // -1 when ps printed something unreadable
}

// ParsePS returns the git processes in `ps -axo pid=,etime=,stat=,comm=`
// output, leaving out zombies (exited, holding nothing). comm is the rest of
// the line and may contain spaces.
func ParsePS(out string) []PSEntry {
	var es []PSEntry
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || strings.HasPrefix(f[2], "Z") || !IsGitCommand(strings.Join(f[3:], " ")) {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		el, ok := parseETime(f[1])
		if !ok {
			el = -1 // unknown: keep the process, it may hold a lock
		}
		es = append(es, PSEntry{PID: pid, Elapsed: el})
	}
	return es
}

// parseETime reads ps's elapsed time, "[[dd-]hh:]mm:ss".
func parseETime(s string) (time.Duration, bool) {
	var days int
	if d, rest, ok := strings.Cut(s, "-"); ok {
		n, err := strconv.Atoi(d)
		if err != nil {
			return 0, false
		}
		days, s = n, rest
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var total int
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, false
		}
		total = total*60 + n
	}
	return time.Duration(days)*24*time.Hour + time.Duration(total)*time.Second, true
}

// ParseProcStatStart returns field 22 (starttime, in clock ticks since boot)
// of a /proc/<pid>/stat line. comm may hold spaces and parentheses, so the
// fields are counted after the last ')'.
func ParseProcStatStart(stat string) (uint64, bool) {
	i := strings.LastIndex(stat, ")")
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(stat[i+1:])
	if len(f) < 20 {
		return 0, false
	}
	n, err := strconv.ParseUint(f[19], 10, 64)
	return n, err == nil
}

// IsGitCommand reports whether a process name belongs to git.
func IsGitCommand(comm string) bool {
	base := filepath.Base(strings.TrimSpace(comm))
	return base == "git" || strings.HasPrefix(base, "git-")
}

// withoutSelf drops this process.
func withoutSelf(ps []Proc, self int) []Proc {
	out := ps[:0]
	for _, p := range ps {
		if p.PID != self {
			out = append(out, p)
		}
	}
	return out
}

// Within reports whether p runs in one of dirs or below it. Paths are
// compared after resolving symlinks (/tmp versus /private/tmp on macOS).
func Within(p Proc, dirs ...string) bool {
	cwd := canonical(p.Cwd)
	for _, d := range dirs {
		d = canonical(d)
		if cwd == d || strings.HasPrefix(cwd, d+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func canonical(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// ParsePSParent reads one `ps -o ppid=,comm= -p PID` line; comm may contain spaces.
func ParsePSParent(line string) (ppid int, comm string, ok bool) {
	f := strings.Fields(line)
	if len(f) < 2 {
		return 0, "", false
	}
	n, err := strconv.Atoi(f[0])
	if err != nil {
		return 0, "", false
	}
	_, rest, _ := strings.Cut(strings.TrimSpace(line), f[0])
	return n, strings.TrimSpace(rest), true
}

// ParseProcStatPPID returns field 4 (ppid) of a /proc/<pid>/stat line,
// counting fields after the last ')' because comm may contain one.
func ParseProcStatPPID(stat string) (int, bool) {
	i := strings.LastIndex(stat, ")")
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(stat[i+1:])
	if len(f) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(f[1])
	return n, err == nil
}

// maxHookDepth bounds the walk up the parent chain to the git running a hook
// (hook shell, a wrapper or two, then git).
const maxHookDepth = 10
