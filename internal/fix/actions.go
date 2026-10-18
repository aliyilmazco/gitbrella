// Package fix repairs what the check found, where there is one right answer
// and nothing the user may have meant can be overwritten: conflict copies
// next to their originals, leftover operation directories, files git did not
// make, a missing index with nothing staged anywhere to lose, an empty
// branch file, objects a remote still has, files only in the cloud.
// Everything else is listed with the command to run by hand.
package fix

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/change"
	"github.com/aliyilmazco/gitbrella/internal/check"
	"github.com/aliyilmazco/gitbrella/internal/cloud"
	"github.com/aliyilmazco/gitbrella/internal/gitx"
	"github.com/aliyilmazco/gitbrella/internal/repo"
	"github.com/aliyilmazco/gitbrella/internal/scan"
	"github.com/aliyilmazco/gitbrella/internal/shq"
	"github.com/aliyilmazco/gitbrella/internal/verify"
)

// Action kinds.
const (
	Quarantine   = "quarantine"    // move a file or directory into the change's backup
	RescueRef    = "rescue-ref"    // keep an object nothing else points at, under refs/gcd-rescue/
	RestoreIndex = "restore-index" // an orphan index.lock that is a valid index equal to HEAD becomes the index
	RebuildIndex = "rebuild-index" // the index from HEAD (git read-tree HEAD); files untouched
	SetRef       = "set-ref"       // an empty branch file gets the commit its reflog last recorded
	AddExclude   = "add-exclude"   // info/exclude learns the git directory's other name
	Download     = "download"      // files under the path that exist only in the cloud are brought onto the disk; their content does not change
	Refetch      = "refetch"       // the remote's objects are fetched again (git fetch --refetch); no ref changes
)

// Action is one repair.
type Action struct {
	Kind    string   `json:"kind"`
	Path    string   `json:"path,omitempty"`    // absolute; for rescue-ref, the copy whose object it keeps
	Ref     string   `json:"ref,omitempty"`     // rescue-ref, set-ref
	Value   string   `json:"value,omitempty"`   // object id, exclude line, or the remote to fetch from
	Objects []string `json:"objects,omitempty"` // refetch: the objects that must be here afterwards
	Code    string   `json:"code"`              // the finding it repairs
	Reason  string   `json:"reason"`
	seen    string   // the path's fingerprint when planned: apply stops if it changed
}

// Manual is damage fix leaves to the user, with what to do.
type Manual struct {
	Code string `json:"code"`
	Path string `json:"path,omitempty"`
	Do   string `json:"do"`
}

// RescuePrefix is where objects nothing else points at are kept.
const RescuePrefix = "refs/gcd-rescue/"

// FetchPrefix is where a refetch puts the remote's refs while it fetches;
// they are deleted before the step ends.
const FetchPrefix = "refs/gcd-fetch/"

// looseObject matches the path of a loose object file.
var looseObject = regexp.MustCompile(`^objects/([0-9a-f]{2})/([0-9a-f]{38}|[0-9a-f]{62})$`)

var oidTokenRe = regexp.MustCompile(`\b([0-9a-f]{40}|[0-9a-f]{64})\b`)

// gcOver is how old another machine's gc.pid must be for its gc to count as
// over.
const gcOver = time.Hour

// objectName finds what may be an object's file name in a path: a file git
// does not read but named after an object ("<hash>-safeBackup-0001") may hold
// its bytes.
var objectName = regexp.MustCompile(`[0-9a-f]{38}`)

// settled are the relations of a ref copy to its branch where the copy's
// commit is here: it can be kept by a rescue ref before the copy goes.
var settled = map[string]bool{"same": true, "copy-older": true, "copy-newer": true, "diverged": true}

// maxRescues is the most objects one copy may name for fix to check and keep
// them one by one; a copy naming more (a long reflog) is left to the user.
const maxRescues = 100

// actionsFor turns the damage in rep into repairs, in the order to run them:
// rescues first, then empty object files moved aside, the refetch that
// brings their objects (and HEAD's commit) back, then the rest. sparse is
// whether the worktree is a sparse checkout, where no index is touched;
// flags tells cloud-only files, which fix never reads. remote, when given,
// is the remote lost objects are fetched from.
func actionsFor(ctx context.Context, g *gitx.Runner, r repo.Repo, rep check.Report, sparse bool, flags scan.FlagReader, remote string) ([]Action, []Manual, error) {
	var rescues, emptyObjects, rest []Action
	var lost []string // objects the refetch must bring back
	lostCode := ""
	fetchAll := false // git fsck found damaged objects it may not name
	var manual []Manual
	inGit := func(rel string) string { return filepath.Join(r.CommonDir, filepath.FromSlash(rel)) }
	want, noRemote, remoteRead := remote, "", false
	from := func() error { // the remote to fetch lost objects from, read once
		if remoteRead {
			return nil
		}
		remoteRead = true
		var err error
		remote, noRemote, err = fetchRemote(ctx, g, r, want)
		return err
	}
	refetch := func(f check.Finding, oid string) {
		if !slices.Contains(lost, oid) { // an empty HEAD commit is both findings
			lost = append(lost, oid)
		}
		if lostCode == "" {
			lostCode = f.Code
		}
	}
	inTree := func(rel string) string { return filepath.Join(r.Worktree, filepath.FromSlash(rel)) }
	byHand := func(f check.Finding, do string) { manual = append(manual, Manual{Code: f.Code, Path: f.Path, Do: do}) }
	names, err := rescueRefs(ctx, g, r) // taken: the rescue refs there are, and those this plan makes
	if err != nil {
		return nil, nil, err
	}
	// Every path fix may move into the backup. A ref inside one of them does
	// not keep a commit alive (a Syncthing copy "main.sync-conflict-..." is a
	// valid ref name, and git lists it): it goes too.
	var movable []string
	for _, f := range rep.Findings {
		switch f.Code {
		case check.CodeCopy, check.CodeIndexCopy, check.CodeRefCopy, check.CodeStaleOperation, check.CodeForeign:
			movable = append(movable, inGit(f.Path))
		}
	}
	ignore := func(ref string) bool {
		for _, m := range movable {
			if within(inGit(ref), m) {
				return true
			}
		}
		return false
	}
	quarantine := func(f check.Finding, abs, why string) error {
		rs, stays, err := rescuesIn(ctx, g, r, abs, rescueName(f.Path), names, ignore)
		if err != nil {
			return err
		}
		if stays != "" {
			byHand(f, fmt.Sprintf("%s; keep what you want with git branch <name> <commit>, then move it away (mv %s <somewhere>)", stays, shq.Quote(abs)))
			return nil
		}
		rescues = append(rescues, rs...)
		rest = append(rest, Action{Kind: Quarantine, Path: abs, Code: f.Code, Reason: why, seen: fingerprint(abs)})
		return nil
	}
	born, err := headBorn(ctx, g, r)
	if err != nil {
		return nil, nil, err
	}
	checkedOut := headRefs(r)
	// What other findings say about a path: a copy of it, a lock of the index.
	copied, empty, indexCopies, indexLock, headLost := map[string]string{}, map[string]bool{}, false, false, false
	for _, f := range rep.Findings {
		switch f.Code {
		case check.CodeCopy, check.CodeIndexCopy, check.CodeRefCopy:
			copied[f.Details["original"]] = f.Path
		case check.CodeEmpty:
			empty[filepath.ToSlash(f.Path)] = true
		}
		indexCopies = indexCopies || f.Code == check.CodeIndexCopy
		indexLock = indexLock || f.Code == check.CodeOrphanIndexLock
		headLost = headLost || f.Code == check.CodeHEADCommitMissing && f.Severity == check.Damage
	}
	// copyOf is a copy of rel, or of a directory rel is in; emptyIn an empty
	// file at rel or under it.
	copyOf := func(rel string) string {
		for orig, cp := range copied {
			if orig != "" && (rel == orig || strings.HasPrefix(rel, orig+"/")) {
				return cp
			}
		}
		return ""
	}
	emptyIn := func(rel string) string {
		for e := range empty {
			if rel != "" && (e == rel || strings.HasPrefix(e, rel+"/")) {
				return e
			}
		}
		return ""
	}
	for _, f := range rep.Findings {
		if f.Severity != check.Damage {
			continue
		}
		rel := filepath.ToSlash(f.Path)
		switch {
		case strings.HasPrefix(rel, "reftable/"):
			// Another device's ref tables and its list of them: the list copy
			// is all that names those tables, and the next pack-refs deletes
			// tables the live list does not name.
			byHand(f, fmt.Sprintf("reftable: another device's refs are in tables only a copied list names. First copy the whole git directory somewhere safe (cp -R %s <safe place>); in that copy, put the copied list in place of reftable/tables.list and list that device's branches (git --git-dir=<safe place> for-each-ref), then make the ones you want here (git branch <name> <commit>). Do not run git pack-refs or maintenance here until then", shq.Quote(r.CommonDir)))
			continue
		case strings.HasPrefix(rel, "modules/"):
			// A submodule's git directory: the superproject cannot read its
			// objects, so nothing in it could be rescued from here.
			byHand(f, "it is in a submodule's git directory; run gitbrella fix in the submodule's own folder, where its objects can be read")
			continue
		case strings.HasPrefix(rel, "objects") && f.Code == check.CodeEmpty && copyOf(rel) == "":
			// Empty, it holds nothing; moved away, the object is missing,
			// so it goes only when a remote can bring it back, and the
			// refetch fails (and the move is reversed) if it does not.
			if err := from(); err != nil {
				return nil, nil, err
			}
			m := looseObject.FindStringSubmatch(rel)
			if m == nil || remote == "" {
				why := noRemote
				if m == nil {
					why = "it is not a loose object, so fix cannot tell which objects to expect back"
				}
				byHand(f, fmt.Sprintf("an empty object file (%s): if no other file holds that object, move it away (mv %s <somewhere>) and fetch it again (git fetch --refetch <remote>)", why, shq.Quote(inGit(rel))))
				continue
			}
			emptyObjects = append(emptyObjects, Action{Kind: Quarantine, Path: inGit(rel), Code: f.Code, seen: fingerprint(inGit(rel)),
				Reason: "an empty object file; the object is fetched again from " + remote})
			refetch(f, m[1]+m[2])
			continue
		case strings.HasPrefix(rel, "objects") && (f.Code == check.CodeCopy || f.Code == check.CodeEmpty || f.Code == check.CodeForeign && objectName.MatchString(rel)):
			byHand(f, "in the object store, which bytes are good fix does not decide: an object file's name is its hash; check the files with git hash-object (on the decompressed content), keep the one that matches in place and move the other away")
			continue
		}
		var err error
		switch f.Code {
		case check.CodeCopy, check.CodeIndexCopy, check.CodeRefCopy:
			switch {
			case f.Details["verified"] == "false" || f.Details["relation"] == "unverifiable":
				byHand(f, "its history could not be read; run the check again when the repository's objects are readable")
			case (f.Details["originalExists"] == "false" || f.Details["relation"] == "no-original") && strings.HasSuffix(f.Details["original"], ".lock"):
				// A copy of a lock git left behind, not a lost file: never
				// put back (it would stop git). A ref lock's commit is kept
				// as a rescue ref first; an index lock copy may hold staged
				// work and stays with the user.
				if f.Code != check.CodeIndexCopy {
					err = quarantine(f, inGit(f.Path), "a copy of a lock git left behind; what it names is kept first")
					break
				}
				byHand(f, fmt.Sprintf("a copy of a lock git left behind, not a lost file: do not put it back as %s; it may hold staged work: look at it (GIT_INDEX_FILE=%s git diff --cached --stat), then move it away (mv %s <somewhere>)", f.Details["original"], shq.Quote(inGit(f.Path)), shq.Quote(inGit(f.Path))))
			case (f.Details["originalExists"] == "false" || f.Details["relation"] == "no-original") && f.Details["refCommit"] == "":
				// (a branch in packed-refs is there, with no file of its own)
				byHand(f, fmt.Sprintf("the only copy: %s is missing. If it was not removed on purpose, put the copy back (mv %s %s); otherwise move the copy away", f.Details["original"], shq.Quote(inGit(f.Path)), shq.Quote(inGit(f.Details["original"]))))
			case emptyIn(f.Details["original"]) != "":
				byHand(f, fmt.Sprintf("%s is empty; which content is right fix does not decide (see that finding)", emptyIn(f.Details["original"])))
			case f.Code == check.CodeRefCopy && !settled[f.Details["relation"]]:
				byHand(f, fmt.Sprintf("the copy names %q, which is not a commit here (%s): it may still arrive with the rest of the sync, or be something else; look at it before moving it away (mv %s <somewhere>)", f.Details["copyCommit"], f.Details["relation"], shq.Quote(inGit(f.Path))))
			case holdsHead(checkedOut, strings.TrimPrefix(rel, "logs/")):
				// the branch itself, or its reflog
				byHand(f, fmt.Sprintf("it belongs to the branch checked out here (a HEAD names %s), so git does read it, whatever its name looks like; look at it and at %s before renaming either", strings.TrimPrefix(rel, "logs/"), f.Details["original"]))
			case f.Code == check.CodeIndexCopy && !sameFile(inGit(f.Path), inGit(f.Details["original"])) &&
				!(f.Details["validIndex"] == "true" && f.Details["equalsHEAD"] == "true"):
				byHand(f, fmt.Sprintf("the copy differs from the index and from HEAD, so it may hold staged work the index lacks; look at it (GIT_INDEX_FILE=%s git diff --cached --stat) and, if it is the one you want, move it over %s; otherwise move it away", shq.Quote(inGit(f.Path)), shq.Quote(inGit(f.Details["original"]))))
			default:
				err = quarantine(f, inGit(f.Path), "a conflict copy git never reads, next to its original")
			}
		case check.CodeStaleLock:
			holds := ""
			if b, rerr := os.ReadFile(inGit(f.Path)); rerr == nil {
				if oid := oidTokenRe.FindString(string(b)); oid != "" {
					holds = fmt.Sprintf("; it names %s, which may be kept nowhere else: keep it if you want it (git branch <name> %s)", oid, oid)
				}
			}
			byHand(f, fmt.Sprintf("no git on this machine holds it; make sure none in a container, on another machine sharing the folder, or in an app does%s, then move it away (mv %s <somewhere>)", holds, shq.Quote(inGit(f.Path))))
		case check.CodeWorktreeAdmin:
			byHand(f, "repair the link with git worktree repair <path of the linked worktree>, or, if that worktree is gone, git worktree prune")
		case check.CodeStaleOperation, check.CodeForeign:
			if f.Code == check.CodeForeign && strings.HasPrefix(rel, "refs/") {
				res, rerr := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "-q", "--verify", rel)
				if rerr != nil {
					return nil, nil, rerr
				}
				if res.ExitCode == 0 { // git reads it as a ref: it may be a branch someone made
					byHand(f, fmt.Sprintf("git reads it as the ref %s (%s); if it is yours, keep it, otherwise move it away (mv %s <somewhere>)", rel, strings.TrimSpace(res.Stdout), shq.Quote(inGit(rel))))
					continue
				}
			}
			err = quarantine(f, inGit(f.Path), "left behind; git misreads it")
		case check.CodeGitDirCopy:
			// A whole copy of the git directory keeps its own objects in the
			// backup, but which of the two is current is the user's to say
			// when the copy has commits the repository does not reach.
			why, cerr := copyIsBehind(ctx, g, r, inTree(f.Path), flags)
			switch {
			case cerr != nil:
				err = cerr
			case why != "":
				byHand(f, fmt.Sprintf("%s; which git directory is current fix does not decide: compare them (git --git-dir=%s log --all --oneline) and move away the one you do not want", why, shq.Quote(inTree(f.Path))))
			default:
				rest = append(rest, Action{Kind: Quarantine, Path: inTree(f.Path), Code: f.Code, seen: fingerprint(inTree(f.Path)),
					Reason: "a copy of the git directory in the working tree whose commits the repository has; it is kept whole in the backup"})
			}
		case check.CodeGitDirNotIgnored:
			rest = append(rest, Action{Kind: AddExclude, Path: inGit("info/exclude"), Value: excludeLine(f.Path),
				Code: f.Code, Reason: "git would add its own directory to a commit", seen: fingerprint(inGit("info/exclude"))})
		case check.CodeEmpty:
			switch {
			case copyOf(rel) != "":
				byHand(f, fmt.Sprintf("it is empty and a copy of it exists (%s); which content is right fix does not decide: look at the copy and, if it is the one, move it over %q", copyOf(rel), inGit(rel)))
			case strings.HasPrefix(rel, "refs/"):
				oid, ok, rerr := lastReflogValue(ctx, g, r, rel)
				if rerr != nil {
					return nil, nil, rerr
				}
				if !ok {
					byHand(f, "the branch file is empty and its reflog names no commit; recreate the branch with git branch -f <name> <commit>")
					continue
				}
				rest = append(rest, Action{Kind: SetRef, Path: inGit(rel), Ref: rel, Value: oid, Code: f.Code, seen: fingerprint(inGit(rel)),
					Reason: "the branch file is empty; its reflog's last commit is " + oid})
			default:
				byHand(f, "an empty "+rel+"; restore it from a backup or another clone")
			}
		case check.CodeOrphanIndexLock:
			switch {
			case f.Details["validIndex"] == "true" && f.Details["equalsHEAD"] == "true" && !sparse:
				rest = append(rest, Action{Kind: RestoreIndex, Path: inGit(f.Path), Code: f.Code, seen: fingerprint(inGit(f.Path)),
					Reason: "the lock is a complete, valid index equal to HEAD: it becomes the index"})
			default:
				byHand(f, fmt.Sprintf("the index is missing and the lock may hold staged work that is not in HEAD; look at it (GIT_INDEX_FILE=%s git diff --cached --stat) and, if you want it, move it to %s; otherwise move it away and run git read-tree HEAD", shq.Quote(inGit(f.Path)), shq.Quote(inGit(strings.TrimSuffix(f.Path, ".lock")))))
			}
		case check.CodeIndexMissing:
			// No index, no copy of one, no lock: nothing staged survives
			// anywhere, so building the index from HEAD loses nothing. When
			// HEAD's commit is missing, the refetch (which runs first)
			// brings it.
			if headLost {
				if err := from(); err != nil {
					return nil, nil, err
				}
			}
			switch {
			case f.Details["verified"] == "false":
				byHand(f, "HEAD could not be read; run the check again when the repository's objects are readable")
			case sparse:
				byHand(f, "sparse checkout: rebuild the index yourself (git reset --mixed, then git sparse-checkout reapply)")
			case !born && headLost && remote == "":
				byHand(f, "HEAD's commit is missing: fetch it again first (see HEAD_COMMIT_MISSING), then rebuild the index with git read-tree HEAD")
			case !born && !headLost:
				byHand(f, "no commit yet: make the index again with git add")
			case indexCopies || indexLock:
				byHand(f, "a copy or lock of the index exists and may hold staged work: look at it (its finding) before rebuilding with git read-tree HEAD")
			default:
				rest = append(rest, Action{Kind: RebuildIndex, Path: inGit(f.Path), Code: f.Code,
					Reason: "there is no index and no copy of one: it is made from HEAD (git read-tree HEAD); the files are not touched"})
			}
		case check.CodeIndexInvalid:
			byHand(f, fmt.Sprintf("the index is damaged and may still hold staged work (git may read it anyway: try git status). If it does, commit or note what is staged, then rebuild with git read-tree HEAD; if it does not, move it away (mv %s <somewhere>) and run git read-tree HEAD: what was staged must be staged again", shq.Quote(inGit(f.Path))))
		case check.CodeIndexEmpty:
			// An index that lists nothing holds nothing staged: the backup
			// keeps it, and the index is made from HEAD.
			ix, perr := verify.ParseIndexFile(inGit(f.Path), r.ObjectFormat)
			switch {
			case f.Details["verified"] == "false" || sparse || perr != nil || ix.Split() || len(ix.Paths) > 0:
				byHand(f, "the index lists no files while HEAD has some; look at git status, then rebuild it from HEAD with git read-tree HEAD")
			default:
				rest = append(rest,
					Action{Kind: Quarantine, Path: inGit(f.Path), Code: f.Code, seen: fingerprint(inGit(f.Path)),
						Reason: "the index lists no files, so nothing is staged in it; it is kept in the backup"},
					Action{Kind: RebuildIndex, Path: inGit(f.Path), Code: f.Code,
						Reason: "the index is made from HEAD (git read-tree HEAD); the files are not touched"})
			}
		case check.CodeIndexShrunk:
			do := "the index lost files that are still on disk, which cannot be told from untracking them on purpose; look at git status, then either stage them again or rebuild the index from HEAD with git reset --mixed (staged changes must be staged again)"
			if f.Details["verified"] != "false" && !sparse {
				only, err := onlyDeletions(ctx, g, r)
				if err != nil {
					return nil, nil, err
				}
				if only {
					do = "the index lost files that are still on disk, which cannot be told from untracking them on purpose (git rm --cached); nothing else is staged, so if you did not untrack them, git read-tree HEAD gives them back and loses nothing"
				}
			}
			byHand(f, do)
		case check.CodeHEADCommitMissing:
			if err := from(); err != nil {
				return nil, nil, err
			}
			if remote == "" {
				byHand(f, fmt.Sprintf("HEAD's commit is not here (%s); fetch it again: git fetch --refetch <remote>", noRemote))
				continue
			}
			refetch(f, f.Details["commit"])
		case check.CodeFsck:
			if err := from(); err != nil {
				return nil, nil, err
			}
			if remote == "" {
				byHand(f, fmt.Sprintf("git fsck found damaged objects and there is no remote to fetch them from (%s); restore them from another clone", noRemote))
				continue
			}
			fetchAll = true
			if lostCode == "" {
				lostCode = f.Code
			}
		default:
			byHand(f, "not repaired by fix: "+f.Message)
		}
		if err != nil {
			return nil, nil, err
		}
	}
	for _, f := range rep.Findings {
		switch f.Code {
		case check.CodeWorktreeCopies:
			cs, err := worktreeCopies(ctx, g, r, flags)
			if err != nil {
				return nil, nil, err
			}
			rest = append(rest, cs...)
		case check.CodeGCPIDForeign:
			// git takes it for a gc on that machine for 12 hours; one that
			// started over an hour ago is over
			p := inGit(f.Path)
			if info, err := os.Lstat(p); err == nil && time.Since(info.ModTime()) > gcOver {
				rest = append(rest, Action{Kind: Quarantine, Path: p, Code: f.Code, seen: fingerprint(p),
					Reason: fmt.Sprintf("another machine's (%s) gc.pid from %s: that gc is over, and the file stops git's automatic gc here", f.Details["host"], info.ModTime().Format(time.RFC3339))})
			}
		case check.CodeGCLogFresh:
			p := inGit(f.Path)
			first := ""
			if b, err := os.ReadFile(p); err == nil {
				first, _, _ = strings.Cut(strings.TrimSpace(string(b)), "\n")
			}
			rest = append(rest, Action{Kind: Quarantine, Path: p, Code: f.Code, seen: fingerprint(p),
				Reason: fmt.Sprintf("the last automatic gc's errors (%q); kept in the backup, so git runs automatic gc again", first)})
		}
	}
	out := append(rescues, emptyObjects...)
	switch {
	case len(lost) > 0:
		out = append(out, Action{Kind: Refetch, Value: remote, Objects: lost, Code: lostCode,
			Reason: fmt.Sprintf("objects the repository lost (%s) are fetched again from %s; no ref changes", strings.Join(lost, ", "), remote)})
	case fetchAll:
		out = append(out, Action{Kind: Refetch, Value: remote, Code: lostCode,
			Reason: fmt.Sprintf("git fsck found damaged objects: every object of %s's branches and tags is fetched again, no ref changes; git fsck runs again after it", remote)})
	}
	return append(out, rest...), manual, nil
}

// worktreeCopies moves into the backup the conflict copies in the working
// tree that git does not track and that hold the same bytes as the file
// they copy: nothing is lost, and 'git add -A' would commit them. A copy
// that differs, is tracked, is a link or only in the cloud, or lies in a
// nested repository (a submodule) or in the git directory stays.
func worktreeCopies(ctx context.Context, g *gitx.Runner, r repo.Repo, flags scan.FlagReader) ([]Action, error) {
	var skip []string
	for _, d := range []string{r.CommonDir, r.GitDir} {
		if rel, err := filepath.Rel(r.Worktree, d); err == nil && within(d, r.Worktree) {
			skip = append(skip, filepath.ToSlash(rel))
		}
	}
	wt, err := scan.ScanWorktree(r.Worktree, flags, check.WorktreeLimit, skip...)
	if err != nil || len(wt.Copies) == 0 {
		return nil, err
	}
	tracked := map[string]bool{}
	for rest := wt.Copies; len(rest) > 0; {
		n := min(len(rest), 500)
		res, err := g.Run(ctx, r.Worktree, r.Env(), append([]string{"--literal-pathspecs", "ls-files", "-z", "--cached", "--"}, rest[:n]...)...)
		if err == nil && res.ExitCode != 0 {
			err = fmt.Errorf("git ls-files: %s", strings.TrimSpace(res.Stderr))
		}
		if err != nil {
			return nil, err
		}
		for _, p := range strings.Split(res.Stdout, "\x00") {
			tracked[p] = true
		}
		rest = rest[n:]
	}
	var out []Action
	for _, rel := range wt.Copies {
		abs := filepath.Join(r.Worktree, filepath.FromSlash(rel))
		m, ok := cloud.MatchCopyName(filepath.Base(abs))
		if !ok || tracked[rel] || inNestedRepo(r.Worktree, filepath.Dir(abs)) {
			continue
		}
		orig := filepath.Join(filepath.Dir(abs), m.Original)
		if !regularLocal(abs, flags) || !regularLocal(orig, flags) || !sameFile(abs, orig) {
			continue
		}
		out = append(out, Action{Kind: Quarantine, Path: abs, Code: check.CodeWorktreeCopies, seen: fingerprint(abs),
			Reason: "an untracked conflict copy with the same bytes as " + m.Original + "; 'git add -A' would commit it"})
	}
	return out, nil
}

// inNestedRepo reports whether dir, below the working tree top, is in
// another repository (a submodule, a nested clone): a directory on the way
// up holds a .git of its own.
func inNestedRepo(top, dir string) bool {
	for ; dir != top && within(dir, top); dir = filepath.Dir(dir) {
		if exists(filepath.Join(dir, ".git")) {
			return true
		}
	}
	return false
}

// regularLocal reports whether p is a regular file whose bytes are on disk.
func regularLocal(p string, flags scan.FlagReader) bool {
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	dl, err := flags.IsDataless(p)
	return err == nil && !dl
}

// onlyDeletions reports whether the index differs from HEAD only by files
// it no longer lists: nothing added or changed is staged. Reading the index
// with GIT_OPTIONAL_LOCKS=0 writes nothing.
func onlyDeletions(ctx context.Context, g *gitx.Runner, r repo.Repo) (bool, error) {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "diff-index", "--cached", "--no-renames", "--name-status", "-z", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return false, err
	}
	f := strings.Split(strings.TrimRight(res.Stdout, "\x00"), "\x00")
	if len(f) < 2 {
		return false, nil
	}
	for i := 0; i+1 < len(f); i += 2 {
		if f[i] != "D" {
			return false, nil
		}
	}
	return true, nil
}

// fetchRemote is the remote fix fetches lost objects from: want when given
// (it must be a configured remote), else the remote of the branch HEAD
// names, else origin, else the only remote. When there is none, why says
// so.
func fetchRemote(ctx context.Context, g *gitx.Runner, r repo.Repo, want string) (remote, why string, err error) {
	major, minor, err := g.Version(ctx)
	if err != nil {
		return "", "", err
	}
	if !gitx.AtLeast(major, minor, 2, 36) {
		return "", "git fetch --refetch needs git 2.36 or newer", nil
	}
	if want != "" {
		res, err := g.Run(ctx, r.Worktree, r.Env(), "remote", "get-url", "--", want)
		if err != nil {
			return "", "", err
		}
		if res.ExitCode != 0 {
			return "", fmt.Sprintf("there is no remote %q", want), nil
		}
		return want, "", nil
	}
	res, err := g.Run(ctx, r.Worktree, r.Env(), "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		return "", "", err
	}
	if b := strings.TrimSpace(res.Stdout); res.ExitCode == 0 && b != "" {
		res, err := g.Run(ctx, r.Worktree, r.Env(), "config", "--get", "branch."+b+".remote")
		if err != nil {
			return "", "", err
		}
		if name := strings.TrimSpace(res.Stdout); res.ExitCode == 0 && name != "" && name != "." { // "." is this repository
			return name, "", nil
		}
	}
	if res, err = g.Run(ctx, r.Worktree, r.Env(), "remote"); err != nil {
		return "", "", err
	}
	switch names := strings.Fields(res.Stdout); {
	case slices.Contains(names, "origin"):
		return "origin", "", nil
	case len(names) == 1:
		return names[0], "", nil
	case len(names) == 0:
		return "", "the repository has no remote", nil
	}
	return "", "there are several remotes and none is the branch's or origin", nil
}

// copyIsBehind says why the copy of a git directory at dir may be ahead of
// the repository: a ref or HEAD of it names a commit (or other object) no
// ref of the repository reaches, it cannot be read, parts of it exist only
// in the cloud, or it names too many objects to check. "" means everything
// it points at is in the repository.
func copyIsBehind(ctx context.Context, g *gitx.Runner, r repo.Repo, dir string, flags scan.FlagReader) (string, error) {
	if flags != nil {
		if err := change.LocalOnly(dir, flags); errors.Is(err, change.ErrNotLocal) {
			return "parts of the copy exist only in the cloud", nil
		} else if err != nil {
			return "", err
		}
	}
	in := []string{"GIT_DIR=" + dir}
	res, err := g.Run(ctx, r.Worktree, in, "for-each-ref", "--format=%(objectname)")
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "the copy cannot be read as a git directory", nil
	}
	oids := strings.Fields(res.Stdout)
	if head, err := g.Run(ctx, r.Worktree, in, "rev-parse", "-q", "--verify", "HEAD"); err != nil {
		return "", err
	} else if head.ExitCode == 0 {
		oids = append(oids, strings.TrimSpace(head.Stdout))
	}
	if len(oids) > maxRescues {
		return fmt.Sprintf("the copy has more than %d refs to compare", maxRescues), nil
	}
	for _, oid := range oids {
		_, ok, err := reach(ctx, g, r, oid, nil)
		if err != nil {
			return "", err
		}
		if !ok {
			return "the copy points at " + oid + ", which no ref of the repository reaches", nil
		}
	}
	return "", nil
}

// reach says what oid is in the repository (kind "" when it is not there)
// and whether a ref reaches it: contains it (a commit) or points at it
// (anything else). Refs ignore names (a copy about to move away) do not
// count.
func reach(ctx context.Context, g *gitx.Runner, r repo.Repo, oid string, ignore func(string) bool) (kind string, reached bool, err error) {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "cat-file", "-t", oid)
	if err != nil || res.ExitCode != 0 {
		return "", false, err
	}
	kind = strings.TrimSpace(res.Stdout)
	args := []string{"for-each-ref", "--points-at", oid, "--format=%(refname)"}
	if kind == "commit" {
		args = []string{"for-each-ref", "--contains", oid, "--format=%(refname)"}
	}
	res, err = g.Run(ctx, r.Worktree, r.Env(), args...)
	if err != nil || res.ExitCode != 0 {
		return kind, false, err
	}
	for _, ref := range strings.Fields(res.Stdout) {
		if ignore == nil || !ignore(ref) {
			return kind, true, nil
		}
	}
	return kind, false, nil
}

// fingerprint is what apply compares to see whether a path changed since the
// plan: a file's content hash, a directory's list of entries with their
// sizes, times and modes ("" when the path does not exist).
func fingerprint(p string) string {
	info, err := os.Lstat(p)
	if err != nil {
		return ""
	}
	if !info.IsDir() {
		return fmt.Sprintf("%v %s", info.Mode(), hashPath(p))
	}
	h := sha256.New()
	filepath.WalkDir(p, func(q string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(h, "error %v\n", err)
			return nil
		}
		i, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(p, q)
		fmt.Fprintf(h, "%s\x00%d %d %v\n", rel, i.Size(), i.ModTime().UnixNano(), i.Mode())
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

// excludeLine is the info/exclude line matching the directory dir at the top
// of the working tree, its glob characters taken literally.
func excludeLine(dir string) string {
	var b strings.Builder
	for _, c := range strings.Trim(filepath.ToSlash(dir), "/") {
		if strings.ContainsRune(`\*?[`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	line := b.String()
	if strings.HasSuffix(line, " ") { // a trailing space is dropped unless escaped
		line = strings.TrimSuffix(line, " ") + `\ `
	}
	return "/" + line + "/"
}

// sameFile reports whether the files a and b have the same bytes.
func sameFile(a, b string) bool {
	x, errA := os.ReadFile(a)
	y, errB := os.ReadFile(b)
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

// rescueRefs returns the refs under refs/gcd-rescue/ that exist.
func rescueRefs(ctx context.Context, g *gitx.Runner, r repo.Repo) (map[string]bool, error) {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "for-each-ref", "--format=%(refname)", RescuePrefix)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git for-each-ref: %s", strings.TrimSpace(res.Stderr))
	}
	out := map[string]bool{}
	for _, ref := range strings.Fields(res.Stdout) {
		out[ref] = true
	}
	return out, nil
}

// rescuesIn returns a rescue for every object named in the file (or the
// files under the directory) at abs that would lose its last pointer when the
// copy goes into the backup: a commit no ref contains, or a tag, tree or blob
// no ref points at. Each gets a free, valid name under refs/gcd-rescue/.
// When the copy cannot go safely, stays says why and nothing is planned: it
// names more than maxRescues objects, or an object that is not here (yet:
// the sync may still bring it).
func rescuesIn(ctx context.Context, g *gitx.Runner, r repo.Repo, abs, name string, taken map[string]bool, ignore func(string) bool) (out []Action, stays string, err error) {
	var oids []string
	seen := map[string]bool{}
	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			for _, m := range oidTokenRe.FindAllString(sc.Text(), -1) {
				if !seen[m] {
					seen[m] = true
					oids = append(oids, m)
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		return nil, "", err
	}
	if len(oids) > maxRescues {
		return nil, fmt.Sprintf("it names more than %d objects that may be kept nowhere else (git fsck --unreachable shows which)", maxRescues), nil
	}
	var keep []string
	kinds := map[string]string{}
	for _, oid := range oids {
		if strings.Trim(oid, "0") == "" {
			continue // the null id: a reflog's "created from nothing", no object
		}
		kind, ok, err := reach(ctx, g, r, oid, ignore)
		if err != nil {
			return nil, "", err
		}
		if kind == "" {
			return nil, fmt.Sprintf("it names %s, which is not in the repository (yet: the sync may still bring it)", oid), nil
		}
		if !ok {
			keep, kinds[oid] = append(keep, oid), kind
		}
	}
	for _, oid := range keep {
		ref, err := freeRef(ctx, g, r, RescuePrefix+name, len(oids) > 1, taken)
		if err != nil {
			return nil, "", err
		}
		out = append(out, Action{Kind: RescueRef, Path: abs, Ref: ref, Value: oid, Code: "RESCUE",
			Reason: fmt.Sprintf("%s %s is reached only from %s", kinds[oid], oid, abs)})
	}
	return out, "", nil
}

// freeRef picks a ref name from base that git accepts and that clashes with
// no rescue ref in taken (those there are and those this plan makes): not the
// same name, and neither one a directory of the other (refs/x and refs/x/1
// cannot both exist). It tries base, base-2, base-3... or, when one copy
// holds several objects, base/1, base/2... under the first base-N that is
// not itself a ref.
func freeRef(ctx context.Context, g *gitx.Runner, r repo.Repo, base string, numbered bool, taken map[string]bool) (string, error) {
	ref, err := freeRefFrom(ctx, g, r, base, numbered, taken)
	if flat := RescuePrefix + strings.ReplaceAll(strings.TrimPrefix(base, RescuePrefix), "/", "-"); err != nil && flat != base {
		// a rescue ref is a directory of every name tried (heads/x-2 for
		// heads/x-2/login): one level, without directories
		return freeRefFrom(ctx, g, r, flat, numbered, taken)
	}
	return ref, err
}

func freeRefFrom(ctx context.Context, g *gitx.Runner, r repo.Repo, base string, numbered bool, taken map[string]bool) (string, error) {
	clashes := func(ref string) bool {
		for t := range taken {
			if t == ref || strings.HasPrefix(t, ref+"/") || strings.HasPrefix(ref, t+"/") {
				return true
			}
		}
		return false
	}
	valid := func(ref string) (bool, error) {
		res, err := g.Run(ctx, r.Worktree, r.Env(), "check-ref-format", ref)
		return err == nil && res.ExitCode == 0, err
	}
	for v := 1; v <= 100; v++ {
		b := base
		if v > 1 {
			b = base + "-" + strconv.Itoa(v)
		}
		if ok, err := valid(b); err != nil || !ok {
			return "", fmt.Errorf("no valid rescue name from %q (%v)", base, err)
		}
		if !numbered {
			if !clashes(b) {
				taken[b] = true
				return b, nil
			}
			continue
		}
		if taken[b] {
			continue // a ref of that name: nothing can live under it
		}
		for k := 1; k <= maxRescues; k++ {
			if ref := b + "/" + strconv.Itoa(k); !clashes(ref) {
				taken[ref] = true
				return ref, nil
			}
		}
	}
	return "", fmt.Errorf("no free rescue name from %q", base)
}

var refUnsafe = regexp.MustCompile(`[^A-Za-z0-9._/-]+`)

// rescueName turns a copy's path into a ref name part: "refs/heads/main 2"
// becomes "heads/main-2"; a ".lock" ending, dots git refuses and names the
// check reads as a copy or a stray file go.
func rescueName(rel string) string {
	n := strings.TrimPrefix(filepath.ToSlash(rel), "refs/")
	n = refUnsafe.ReplaceAllString(n, "-")
	n = strings.ReplaceAll(n, "..", "-")
	for strings.HasSuffix(n, ".lock") {
		n = strings.TrimSuffix(n, ".lock")
	}
	parts := strings.Split(n, "/")
	for i, p := range parts {
		p = strings.Trim(p, ".-")
		// The check must not take the rescue ref for a copy or a stray file
		// itself ("main.sync-conflict-..." and Nextcloud's older
		// "main_conflict-..." are valid ref names).
		if copyLike(p) {
			p = strings.NewReplacer(".", "-", "_", "-").Replace(p)
		}
		if p == "" {
			p = "copy"
		}
		parts[i] = p
	}
	return strings.Join(parts, "/")
}

// copyLike reports whether the check could read a ref name part as a copy or
// a stray file, by any scheme it knows.
func copyLike(p string) bool {
	_, marked := cloud.MatchCopyName(p)
	_, unmarked := cloud.MatchUnmarkedCopyName(p)
	_, loose := cloud.MatchLooseCopyName(p)
	return marked || unmarked || loose || cloud.IsSyncMetadata(p)
}

// headRefs are the refs the HEADs of the repository and its linked worktrees
// name ("refs/heads/main").
func headRefs(r repo.Repo) map[string]bool {
	out := map[string]bool{}
	heads := []string{filepath.Join(r.CommonDir, "HEAD")}
	if ws, err := os.ReadDir(filepath.Join(r.CommonDir, "worktrees")); err == nil {
		for _, w := range ws {
			heads = append(heads, filepath.Join(r.CommonDir, "worktrees", w.Name(), "HEAD"))
		}
	}
	for _, h := range heads {
		if target, err := os.Readlink(h); err == nil { // core.preferSymlinkRefs
			out[filepath.ToSlash(target)] = true
			continue
		}
		if b, err := os.ReadFile(h); err == nil {
			// git takes any whitespace after "ref:"
			if rest, ok := strings.CutPrefix(string(b), "ref:"); ok {
				if f := strings.Fields(rest); len(f) > 0 {
					out[f[0]] = true
				}
			}
		}
	}
	return out
}

// holdsHead reports whether a HEAD names the ref at rel or one below it (in a
// copied directory of branches).
func holdsHead(heads map[string]bool, rel string) bool {
	for h := range heads {
		if h == rel || strings.HasPrefix(h, rel+"/") {
			return true
		}
	}
	return false
}

// headBorn reports whether HEAD names a commit.
func headBorn(ctx context.Context, g *gitx.Runner, r repo.Repo) (bool, error) {
	res, err := g.Run(ctx, r.Worktree, r.Env(), "rev-parse", "--verify", "-q", "HEAD^{commit}")
	if err != nil {
		return false, err
	}
	return res.ExitCode == 0, nil
}

// lastReflogValue reads the newest commit the reflog of ref (refs/...) recorded.
func lastReflogValue(ctx context.Context, g *gitx.Runner, r repo.Repo, ref string) (string, bool, error) {
	b, err := os.ReadFile(filepath.Join(r.CommonDir, "logs", filepath.FromSlash(ref)))
	if err != nil {
		return "", false, nil
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	f := strings.Fields(lines[len(lines)-1])
	if len(f) < 2 || !oidTokenRe.MatchString(f[1]) {
		return "", false, nil
	}
	res, err := g.Run(ctx, r.Worktree, r.Env(), "cat-file", "-e", f[1]+"^{commit}")
	if err != nil {
		return "", false, err
	}
	return f[1], res.ExitCode == 0, nil
}

// within reports whether p is dir or below it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// hashPath is the sha256 of a file's content, "" when it does not exist or is
// not a regular file.
func hashPath(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
