package change

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/procs"
	"github.com/aliyilmazco/gitbrella/internal/scan"
)

type someDataless struct{ suffix string }

func (s someDataless) IsDataless(p string) (bool, error) { return strings.HasSuffix(p, s.suffix), nil }

func TestLocalOnly(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "b", "cloud.pack"), []byte("x"), 0o644)
	if err := LocalOnly(dir, scan.DefaultFlagReader()); err != nil {
		t.Fatalf("local tree: %v", err)
	}
	if err := LocalOnly(dir, someDataless{"cloud.pack"}); !errors.Is(err, ErrNotLocal) || !strings.Contains(err.Error(), "cloud.pack") {
		t.Fatalf("cloud-only file: %v", err)
	}
}

func TestRenameMovesAndReportsErrors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o644)
	if err := Rename(filepath.Join(dir, "a"), filepath.Join(dir, "b")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b")); err != nil {
		t.Fatal(err)
	}
	if err := Rename(filepath.Join(dir, "missing"), filepath.Join(dir, "c")); err == nil {
		t.Fatal("renaming nothing succeeded")
	}
}

func TestBusy(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_COUNT", "1") // no detached "git maintenance" after commits
	t.Setenv("GIT_CONFIG_KEY_0", "maintenance.auto")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	g, err := gitx.New()
	if err != nil {
		t.Skip("git not installed")
	}
	wt, _ := filepath.EvalSymlinks(t.TempDir())
	if out, err := exec.Command("git", "init", "-q", wt).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	env := Env{Home: t.TempDir(), GOOS: "linux", Flags: scan.DefaultFlagReader(), Version: "test",
		Procs: func(context.Context) ([]procs.Proc, error) { return nil, nil }}
	if rs, err := Busy(context.Background(), g, env, wt); err != nil || len(rs) != 0 {
		t.Fatalf("idle repository: %+v %v", rs, err)
	}
	env.Procs = func(context.Context) ([]procs.Proc, error) {
		return []procs.Proc{{PID: 4242, Cwd: wt, Start: time.Now().Add(-time.Hour)}}, nil
	}
	if rs, err := Busy(context.Background(), g, env, wt); err != nil || len(rs) != 1 || rs[0].Code != RefuseBusy {
		t.Fatalf("git running: %+v %v", rs, err)
	}
}
