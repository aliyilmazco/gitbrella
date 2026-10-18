package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"gitbrella": func() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) },
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Setenv("HOME", env.WorkDir) // keeps provider detection at "none"
			env.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			env.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			// no detached "git maintenance" after commits, still writing
			// in .git while the work directory is removed
			env.Setenv("GIT_CONFIG_COUNT", "1")
			env.Setenv("GIT_CONFIG_KEY_0", "maintenance.auto")
			env.Setenv("GIT_CONFIG_VALUE_0", "false")
			return nil
		},
		Condition: func(cond string) (bool, error) {
			if cond == "caseinsensitive" { // the disk takes "NOTES" for "notes"
				d, err := os.MkdirTemp("", "case")
				if err != nil {
					return false, err
				}
				defer os.RemoveAll(d)
				os.WriteFile(filepath.Join(d, "a"), nil, 0o644)
				_, err = os.Stat(filepath.Join(d, "A"))
				return err == nil, nil
			}
			// gitNNN: git 2.NNN or newer (git245, git240)
			if minor, err := strconv.Atoi(strings.TrimPrefix(cond, "git2")); strings.HasPrefix(cond, "git2") && err == nil {
				return fixtures.GitAtLeast(2, minor)
			}
			return false, errors.New("unknown condition " + cond)
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// fixture <scenario> <dir>
			"fixture": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 2 {
					ts.Fatalf("usage: fixture <scenario> <dir>")
				}
				ts.Check(fixtures.Build(args[0], ts.MkAbs(args[1])))
			},
			// snapshot save <file> <dir> | snapshot same <file> <dir>
			"snapshot": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 3 {
					ts.Fatalf("usage: snapshot save|same <file> <dir>")
				}
				snap, err := fixtures.TakeSnapshot(ts.MkAbs(args[2]), nil)
				ts.Check(err)
				switch args[0] {
				case "save":
					ts.Check(snap.Save(ts.MkAbs(args[1])))
				case "same":
					before, err := fixtures.LoadSnapshot(ts.MkAbs(args[1]))
					ts.Check(err)
					if d := before.Diff(snap); len(d) > 0 {
						ts.Fatalf("tree changed: %v", d)
					}
				default:
					ts.Fatalf("unknown snapshot action %q", args[0])
				}
			},
			// exitcode <n> <args...> runs gitbrella in the script's
			// directory and checks the exact exit status; stdout and stderr stay
			// available to the following stdout/stderr assertions.
			"exitcode": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) < 2 {
					ts.Fatalf("usage: exitcode <n> <args...>")
				}
				want, err := strconv.Atoi(args[0])
				ts.Check(err)
				got := 0
				if err := ts.Exec("gitbrella", args[1:]...); err != nil {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) {
						ts.Fatalf("running gitbrella: %v", err)
					}
					got = exitErr.ExitCode()
				}
				if got != want {
					ts.Fatalf("exit status %d, want %d", got, want)
				}
			},
		},
	})
}

func TestUsageAndVersion(t *testing.T) {
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	if got := run(nil, devnull, devnull); got != 2 {
		t.Fatalf("no args: exit %d", got)
	}
	if got := run([]string{"version"}, devnull, devnull); got != 0 {
		t.Fatalf("version: exit %d", got)
	}
	if got := run([]string{"bogus"}, devnull, devnull); got != 2 {
		t.Fatalf("bogus: exit %d", got)
	}
	if got := run([]string{"check", "--provider", "box", filepath.Join(t.TempDir())}, devnull, devnull); got != 2 {
		t.Fatalf("bad provider: exit %d", got)
	}
}

func TestRefusesToRunWithoutDownloadProtection(t *testing.T) {
	saved := preventMaterialization
	preventMaterialization = func() error { return errors.New("simulated") }
	defer func() { preventMaterialization = saved }()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	if got := run([]string{"check", dir}, io.Discard, &stderr); got != 2 || !strings.Contains(stderr.String(), "cloud downloads") {
		t.Fatalf("exit %d, stderr %q", got, stderr.String())
	}
}

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		set  string
		bi   *debug.BuildInfo
		want string
	}{
		{"dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, "v0.1.0"},
		{"dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"dev", nil, "dev"},
		{"0.2.0", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, "0.2.0"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.set, c.bi); got != c.want {
			t.Errorf("resolveVersion(%q, %v) = %q, want %q", c.set, c.bi, got, c.want)
		}
	}
}

func TestFlagsMayFollowThePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	if err := fixtures.Build("clean", dir); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if got := run([]string{"check", dir, "--json"}, &out, io.Discard); got != 0 || !strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Fatalf("exit %d, stdout %q", got, out.String())
	}
}

func TestJSONErrorOnExit2(t *testing.T) {
	var out strings.Builder
	if got := run([]string{"check", "--json", t.TempDir()}, &out, io.Discard); got != 2 {
		t.Fatalf("exit %d", got)
	}
	var v struct {
		SchemaVersion int    `json:"schemaVersion"`
		Error         string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out.String()), &v); err != nil || v.SchemaVersion != 1 || !strings.Contains(v.Error, "not a git repository") {
		t.Fatalf("stdout %q: %v %+v", out.String(), err, v)
	}
}
