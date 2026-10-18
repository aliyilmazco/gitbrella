// Package report renders a check.Report for people and for machines.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/check"
)

// WriteJSON writes the report as indented JSON. findings is never null.
func WriteJSON(w io.Writer, r check.Report) error {
	if r.Findings == nil {
		r.Findings = []check.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteText writes a human-readable report.
func WriteText(w io.Writer, r check.Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "gitbrella %s\n", r.ToolVersion)
	fmt.Fprintf(&b, "repository: %s\n", r.Repo.Worktree)
	fmt.Fprintf(&b, "git dir:    %s\n", r.Repo.CommonDir)
	fmt.Fprintf(&b, "sync:       %s\n\n", r.Repo.Provider)
	damage, info := 0, 0
	for _, f := range r.Findings {
		label := "INFO  "
		if f.Severity == check.Damage {
			label = "DAMAGE"
			damage++
		} else {
			info++
		}
		fmt.Fprintf(&b, "%s %s", label, f.Code)
		if f.Path != "" {
			fmt.Fprintf(&b, "  %s", f.Path)
		}
		fmt.Fprintf(&b, "\n       %s\n", f.Message)
		fixCmd := "gitbrella fix"
		if r.Repo.ProviderForced {
			fixCmd += " --provider " + r.Repo.Provider // fix finds the same damage only with the same provider
		}
		switch f.Repair {
		case "fix":
			fmt.Fprintf(&b, "       repair: '%s' repairs it\n", fixCmd)
		case "manual":
			fmt.Fprintf(&b, "       repair: by hand; '%s' says what to run\n", fixCmd)
		}
		keys := make([]string, 0, len(f.Details))
		for k := range f.Details {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "       %s: %s\n", k, f.Details[k])
		}
	}
	if damage+info == 0 {
		b.WriteString("No problems found.\n")
	}
	fmt.Fprintf(&b, "\n%d damage, %d info. gitbrella changed nothing.\n", damage, info)
	_, err := io.WriteString(w, b.String())
	return err
}

// WriteJSONError writes the JSON document for a check that could not run
// (exit status 2), so a caller parsing stdout always gets JSON.
func WriteJSONError(w io.Writer, toolVersion, msg string, hints []string) error {
	if hints == nil {
		hints = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		SchemaVersion int      `json:"schemaVersion"`
		ToolVersion   string   `json:"toolVersion"`
		Error         string   `json:"error"`
		Hints         []string `json:"hints"`
	}{1, toolVersion, msg, hints})
}
