package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aliyilmazco/gitbrella/internal/check"
)

func sample() check.Report {
	return check.Report{SchemaVersion: 1, ToolVersion: "0.1.0", Repo: check.RepoInfo{Worktree: "/r", CommonDir: "/r/.git", Provider: "icloud"},
		Findings: []check.Finding{
			{Code: check.CodeIndexCopy, Severity: check.Damage, Path: "index 2", Message: "icloud conflict copy of index", Details: map[string]string{"validIndex": "true", "equalsHEAD": "true"}},
			{Code: check.CodeInCloudFolder, Severity: check.Info, Path: "/r", Message: "repository is inside a folder synced by icloud"},
		}}
}

func TestWriteText(t *testing.T) {
	var b bytes.Buffer
	if err := WriteText(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"DAMAGE INDEX_COPY  index 2", "equalsHEAD: true", "INFO   IN_CLOUD_FOLDER", "1 damage, 1 info. gitbrella changed nothing."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Index(out, "equalsHEAD") > strings.Index(out, "validIndex") {
		t.Error("details must be sorted by key")
	}
}

func TestWriteTextClean(t *testing.T) {
	var b bytes.Buffer
	if err := WriteText(&b, check.Report{ToolVersion: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "No problems found.") {
		t.Fatal(b.String())
	}
}

func TestWriteJSON(t *testing.T) {
	var b bytes.Buffer
	if err := WriteJSON(&b, check.Report{SchemaVersion: 1}); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if f, ok := decoded["findings"].([]any); !ok || len(f) != 0 {
		t.Fatalf("findings must be an empty array, got %v", decoded["findings"])
	}
	if decoded["schemaVersion"].(float64) != 1 {
		t.Fatal("schemaVersion missing")
	}
}
