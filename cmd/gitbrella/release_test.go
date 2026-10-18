package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestReleaseConfig reads .goreleaser.yaml as a release would use it: the four
// targets CI builds, no cgo, and one ldflags line that really sets the version
// a release prints (with a renamed variable a release would print the module
// version Go stamps, or "dev"). GoReleaser's own validation runs in the
// release workflow (goreleaser check).
func TestReleaseConfig(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{"main: ./cmd/gitbrella", "- CGO_ENABLED=0", "goos: [darwin, linux]", "goarch: [amd64, arm64]", "- -trimpath"} {
		if !strings.Contains(cfg, want) {
			t.Errorf(".goreleaser.yaml lacks %q", want)
		}
	}
	if n := strings.Count(cfg, "main.version="); n != 1 {
		t.Fatalf(".goreleaser.yaml sets main.version %d times; a later one would win", n)
	}
	m := regexp.MustCompile(`(?m)^\s*-\s*(-s -w -X main\.version=.+?)\s*$`).FindStringSubmatch(cfg)
	if m == nil {
		t.Fatal(".goreleaser.yaml sets no main.version in its ldflags")
	}
	ldflags := strings.ReplaceAll(m[1], "{{ .Version }}", "1.2.3")
	bin := filepath.Join(t.TempDir(), "gitbrella")
	build := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build -ldflags %q: %v\n%s", ldflags, err, out)
	}
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "gitbrella v1.2.3" {
		t.Fatalf("a release built with %q prints %q", ldflags, got)
	}
}
