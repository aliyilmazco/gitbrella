package main

// history lists the changes this device made to a repository and the
// backups they keep; prune-backups deletes the backups no undo can use.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/journal"
	"github.com/aliyilmazco/gitbrella/internal/state"
	"github.com/aliyilmazco/gitbrella/internal/treecopy"
)

// historyEntry is one change with its backup.
type historyEntry struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Phase    string    `json:"phase"`
	Started  time.Time `json:"started"`
	Last     time.Time `json:"last"` // the newest phase's time
	UndoneBy string    `json:"undoneBy,omitempty"`
	Backup   string    `json:"backup,omitempty"` // "" when there is none (any more)
	Bytes    int64     `json:"bytes"`
	Undo     bool      `json:"undo"`     // what 'undo' reverses next
	Prunable bool      `json:"prunable"` // no undo or recovery can need its backup
	Pruned   bool      `json:"pruned,omitempty"`
}

type historyResult struct {
	SchemaVersion int            `json:"schemaVersion"`
	ToolVersion   string         `json:"toolVersion"`
	Command       string         `json:"command"`
	Worktree      string         `json:"worktree"`
	Entries       []historyEntry `json:"entries"`
	Freed         int64          `json:"freed,omitempty"`
	Warnings      []string       `json:"warnings"`
}

// prunable reports whether e's backup can go: e finished and is undone,
// rolled back, or of a kind undo never reverses. A change still undoable
// keeps its backup, even under newer ones: undo reaches it after them.
func prunable(e journal.Entry) bool {
	if !e.Finished() {
		return false
	}
	return e.UndoneBy != "" || e.Phase == journal.RolledBack || (e.Kind != "move-out" && e.Kind != "fix")
}

func runHistory(args []string, stdout, stderr io.Writer) int {
	return listOrPrune("history", args, stdout, stderr)
}

func runPruneBackups(args []string, stdout, stderr io.Writer) int {
	return listOrPrune("prune-backups", args, stdout, stderr)
}

func listOrPrune(command string, args []string, stdout, stderr io.Writer) int {
	var c *changeFlags
	var olderThan *time.Duration
	if command == "prune-backups" {
		c = newChangeFlags(command, stderr)
		olderThan = c.fs.Duration("older-than", 30*24*time.Hour, "delete only backups of changes whose last step is older than this")
	} else { // history only reads: no --apply
		fs := flag.NewFlagSet(command, flag.ContinueOnError)
		fs.SetOutput(stderr)
		setUsage(fs)
		c = &changeFlags{fs: fs, apply: new(bool), asJSON: fs.Bool("json", false, "print the result as JSON")}
	}
	if !c.parse(args) {
		return 2
	}
	env, done, err := start(command, c, stderr)
	if err != nil {
		return changeFail(stdout, stderr, *c.asJSON, err)
	}
	defer done()
	res := historyResult{SchemaVersion: 1, ToolVersion: version, Command: command, Worktree: env.res.Worktree,
		Entries: []historyEntry{}, Warnings: []string{}}
	for _, f := range env.res.Refusals {
		res.Warnings = append(res.Warnings, f.Code+": "+f.Message)
	}
	res.Warnings = append(res.Warnings, env.res.Warnings...)
	repoID := state.RepoID(res.Worktree, env.opts.Home)
	all, err := env.store.List(repoID)
	if err != nil {
		return changeFail(stdout, stderr, *c.asJSON, err)
	}
	next, hasNext, _ := env.store.LastUndoable(repoID)
	now := time.Now()
	for i := range all {
		e := &all[i]
		h := historyEntry{ID: e.ID, Kind: e.Kind, Phase: string(e.Phase), Started: e.Started, Last: e.Started,
			UndoneBy: e.UndoneBy, Undo: hasNext && next.ID == e.ID, Prunable: prunable(*e), Pruned: e.Data["pruned"] != ""}
		if n := len(e.History); n > 0 {
			h.Last = e.History[n-1].At
		}
		dir := filepath.Join(env.opts.Dirs.Backups(), e.ID)
		if _, err := os.Lstat(dir); err == nil {
			h.Backup = dir
			if h.Bytes, err = treecopy.Size(dir); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("the size of %s: %v", dir, err))
			}
		}
		if olderThan != nil && now.Sub(h.Last) <= *olderThan {
			h.Prunable = false // not old enough for this run
		}
		if h.Backup != "" && h.Prunable && *c.apply {
			if err := os.RemoveAll(dir); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("deleting %s: %v", dir, err))
				continue
			}
			if e.Data == nil {
				e.Data = map[string]string{}
			}
			e.Data["pruned"] = now.UTC().Format(time.RFC3339)
			if err := env.store.Save(e); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("journal %s: %v", e.ID, err))
			}
			res.Freed += h.Bytes
			h.Backup, h.Bytes, h.Pruned = "", 0, true
		}
		res.Entries = append(res.Entries, h)
	}
	code := 0
	if len(res.Warnings) > 0 {
		code = 1
	}
	if *c.asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return 2
		}
		return code
	}
	var b strings.Builder
	fmt.Fprintf(&b, "gitbrella %s\n%s: %s\n", version, command, res.Worktree)
	if len(res.Entries) == 0 {
		b.WriteString("this device has made no change to it\n")
	}
	for _, h := range res.Entries {
		state := h.Phase
		if h.UndoneBy != "" {
			state = "undone by " + h.UndoneBy
		}
		fmt.Fprintf(&b, "%s  %-8s %s  %s", h.Last.Local().Format("2006-01-02 15:04"), h.Kind, h.ID, state)
		if h.Undo {
			b.WriteString("  <- undo reverses this next")
		}
		b.WriteString("\n")
		switch {
		case h.Backup != "":
			mark := ""
			if h.Prunable {
				mark = " (prune-backups may delete it)"
			}
			fmt.Fprintf(&b, "    backup %s, %s%s\n", h.Backup, size(h.Bytes), mark)
		case h.Pruned:
			b.WriteString("    backup deleted by prune-backups\n")
		}
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(&b, "WARNING %s\n", w)
	}
	switch {
	case olderThan == nil:
	case *c.apply:
		fmt.Fprintf(&b, "\nfreed %s.\n", size(res.Freed))
	default:
		var would int64
		for _, h := range res.Entries {
			if h.Prunable && h.Backup != "" {
				would += h.Bytes
			}
		}
		fmt.Fprintf(&b, "\nNothing was deleted. Run again with --apply to delete the backups marked above (%s).\n", size(would))
	}
	io.WriteString(stdout, b.String())
	return code
}

func size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
