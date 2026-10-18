package gitx

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in           string
		major, minor int
		ok           bool
	}{
		{"git version 2.54.0\n", 2, 54, true},
		{"git version 2.39.5 (Apple Git-154)", 2, 39, true},
		{"git version 2.43.0.windows.1", 2, 43, true},
		{"hello", 0, 0, false},
		{"git version x.y", 0, 0, false},
	}
	for _, c := range cases {
		major, minor, err := ParseVersion(c.in)
		if (err == nil) != c.ok || major != c.major || minor != c.minor {
			t.Errorf("ParseVersion(%q) = %d, %d, %v", c.in, major, minor, err)
		}
	}
}

func TestAtLeast(t *testing.T) {
	if !AtLeast(2, 31, 2, 31) || !AtLeast(3, 0, 2, 31) || AtLeast(2, 30, 2, 31) {
		t.Fatal("AtLeast comparison is wrong")
	}
}

func TestEnvDropsRedirectsAndAddsSafety(t *testing.T) {
	env := Env([]string{"HOME=/h", "GIT_DIR=/x", "GIT_INDEX_FILE=/y", "PATH=/bin", "GIT_TRACE=1", "GIT_TRACE2_EVENT=/t", "GIT_TRACE_PACKET=1", "GIT_ALTERNATE_OBJECT_DIRECTORIES=/a"})
	for _, bad := range []string{"GIT_DIR=/x", "GIT_INDEX_FILE=/y", "GIT_TRACE=1", "GIT_TRACE2_EVENT=/t", "GIT_TRACE_PACKET=1", "GIT_ALTERNATE_OBJECT_DIRECTORIES=/a"} {
		if slices.Contains(env, bad) {
			t.Errorf("Env kept %s", bad)
		}
	}
	for _, want := range []string{"HOME=/h", "PATH=/bin", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1"} {
		if !slices.Contains(env, want) {
			t.Errorf("Env missing %s", want)
		}
	}
}

func TestRunReportsExitCodeAsData(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Skip("git not installed")
	}
	res, err := g.Run(context.Background(), t.TempDir(), nil, "rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("Run returned error for a non-zero exit: %v", err)
	}
	if res.ExitCode == 0 || !strings.Contains(res.Stderr, "not a git repository") {
		t.Fatalf("want non-zero exit outside a repo, got %d %q", res.ExitCode, res.Stderr)
	}
}

func TestNewWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := New(); err != ErrGitNotFound {
		t.Fatalf("want ErrGitNotFound, got %v", err)
	}
}
