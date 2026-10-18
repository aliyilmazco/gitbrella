// Package gitx runs git with settings that keep every call read-only and quiet.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SafeConfig is prepended to every git invocation so that a check can never
// trigger background maintenance or run a configured fsmonitor command.
var SafeConfig = []string{
	"-c", "gc.auto=0",
	"-c", "maintenance.auto=false",
	"-c", "core.fsmonitor=false",
	"-c", "advice.graftFileDeprecated=false", // the empty graft file in Env
}

// ErrGitNotFound is returned when no git binary is on PATH.
var ErrGitNotFound = errors.New("git executable not found in PATH")

// Runner executes git.
type Runner struct {
	Bin     string
	Timeout time.Duration
}

// Result holds the output of one git call. A non-zero exit status is data,
// not an error.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// New finds git on PATH.
func New() (*Runner, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, ErrGitNotFound
	}
	return &Runner{Bin: bin, Timeout: 30 * time.Second}, nil
}

// Run executes git in dir with extraEnv appended to the safe environment.
// err is set only when git could not be started or timed out.
func (r *Runner) Run(ctx context.Context, dir string, extraEnv []string, args ...string) (Result, error) {
	return r.RunInput(ctx, dir, extraEnv, "", args...)
}

// RunInput is Run with stdin.
func (r *Runner) RunInput(ctx context.Context, dir string, extraEnv []string, stdin string, args ...string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	full := append(append([]string{}, SafeConfig...), args...)
	cmd := exec.CommandContext(ctx, r.Bin, full...)
	cmd.Dir = dir
	cmd.Env = append(Env(os.Environ()), extraEnv...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	res := Result{Stdout: out.String(), Stderr: errb.String()}
	if ctx.Err() != nil {
		return res, fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return res, nil
}

// Env drops variables that would point git at another repository or index
// and adds the variables that keep git read-only, non-interactive and in C locale.
func Env(base []string) []string {
	drop := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true,
		"GIT_COMMON_DIR": true, "GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	}
	env := make([]string, 0, len(base)+5)
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if drop[k] || strings.HasPrefix(k, "GIT_TRACE") {
			continue // trace output on stderr would read as git complaining
		}
		env = append(env, kv)
	}
	// GIT_NO_LAZY_FETCH stops a partial clone from fetching missing objects
	// (and writing packs) when a check reads them; git < 2.45 ignores it.
	// Replace refs and info/grafts rewrite history; GIT_NO_REPLACE_OBJECTS
	// covers only the first, an empty graft file the second.
	return append(env, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="+os.DevNull)
}

// Version returns git's major and minor version numbers.
func (r *Runner) Version(ctx context.Context) (int, int, error) {
	res, err := r.Run(ctx, "", nil, "version")
	if err != nil {
		return 0, 0, err
	}
	return ParseVersion(res.Stdout)
}

// ParseVersion parses output such as "git version 2.39.5 (Apple Git-154)".
func ParseVersion(s string) (int, int, error) {
	f := strings.Fields(s)
	if len(f) < 3 || f[0] != "git" || f[1] != "version" {
		return 0, 0, fmt.Errorf("unexpected git version output %q", strings.TrimSpace(s))
	}
	parts := strings.SplitN(f[2], ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected git version %q", f[2])
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("unexpected git version %q", f[2])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("unexpected git version %q", f[2])
	}
	return major, minor, nil
}

// AtLeast reports whether major.minor >= wantMajor.wantMinor.
func AtLeast(major, minor, wantMajor, wantMinor int) bool {
	return major > wantMajor || (major == wantMajor && minor >= wantMinor)
}
