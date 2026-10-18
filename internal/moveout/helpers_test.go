package moveout

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/icloud"
	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/state"
)

// lab is a fake home with an iCloud Drive folder holding one repository.
type lab struct {
	t        *testing.T
	home     string
	worktree string
	opts     Options
	store    journal.Store
	g        *gitx.Runner
}

func newLab(t *testing.T) *lab {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	noMaintenance(t)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "my notes")
	if err := os.MkdirAll(filepath.Join(wt, "sub dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	dirs, err := state.Resolve(home, "darwin", func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	g, err := gitx.New()
	if err != nil {
		t.Fatal(err)
	}
	l := &lab{t: t, home: home, worktree: wt, g: g,
		opts: Options{Path: wt, Dirs: dirs,
			Env: change.Env{Home: home, GOOS: "darwin", Flags: scan.DefaultFlagReader(), Version: "test",
				JournalDir: dirs.Journal(), Procs: func(context.Context) ([]procs.Proc, error) { return nil, nil }},
			Probe: func(string) (icloud.Status, error) { return icloud.Status{Known: true}, nil }},
		store: journal.Store{Dir: dirs.Journal()},
	}
	l.git("init", "-q", "-b", "main")
	l.write("a.md", "a\n")
	l.write("sub dir/b.md", "b\n")
	l.git("add", "-A")
	l.git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "one")
	l.write("a.md", "a\nchanged\n") // an uncommitted change must survive the move
	return l
}

func (l *lab) git(args ...string) string {
	l.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = l.worktree
	out, err := cmd.CombinedOutput()
	if err != nil {
		l.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (l *lab) write(rel, body string) {
	l.t.Helper()
	if err := os.WriteFile(filepath.Join(l.worktree, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
		l.t.Fatal(err)
	}
}

func (l *lab) plan() Plan {
	l.t.Helper()
	p, err := MakePlan(context.Background(), l.g, l.opts)
	if err != nil {
		l.t.Fatal(err)
	}
	return p
}

func codes(rs []Refusal) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Code)
	}
	return strings.Join(out, ",")
}

// noMaintenance stops git >= 2.47 from starting a detached "git maintenance
// run --auto" after commits, which can still write in .git while the test's
// temporary directory is removed.
func noMaintenance(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "maintenance.auto")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
}
