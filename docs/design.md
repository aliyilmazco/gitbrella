# Design

## Goal

Tell, quickly and without side effects, whether a git repository has the
damage that file-by-file cloud sync causes, so that agents and hooks can stop
before they make it worse.

## Rules

1. **Nothing is written.** Every git call goes through one runner
   (`internal/gitx`) that drops `GIT_DIR`-style redirects and adds
   `GIT_OPTIONAL_LOCKS=0`, `GIT_TERMINAL_PROMPT=0`, `GIT_NO_LAZY_FETCH=1`,
   `GIT_NO_REPLACE_OBJECTS=1`, `-c gc.auto=0 -c maintenance.auto=false
   -c core.fsmonitor=false`, an empty graft file and
   `advice.graftFileDeprecated=false`. Index files are read by the tool's own
   parser, never by git. Index copies are compared with HEAD through a private
   temporary copy outside the repository; under a split index they are not
   compared at all, because reading one refreshes `sharedindex.*` timestamps.
   For the same reason `--deep` skips `git fsck`, which reads every
   worktree's index, wherever an index is split or sparse.
2. **Nothing is downloaded.** On macOS the process calls
   `setiopolicy_np(IOPOL_TYPE_VFS_MATERIALIZE_DATALESS_FILES, OFF)` (through
   purego, no cgo) before touching the disk; child git processes inherit it and
   fail fast instead of downloading. The scan reads `st_flags` for
   `SF_DATALESS` and never opens a cloud-only file. When `.git` holds such
   files, every check that reads objects is skipped. If the policy cannot be set,
   the check does not run.
3. **Liveness from processes and their start times.** Git closes a lock file
   before it renames it, so the file alone says nothing. A lock counts as held
   when a git process works on the repository and started before the lock was
   written (git creates a lock when it takes it). Which repository a process
   works on is found the way git finds it: walking up from its working
   directory (read with `proc_pidinfo` on macOS, `/proc` on Linux) to a usable
   `.git` and comparing common directories by file identity. A process whose
   directory or start time cannot be read counts as a possible holder; if
   processes cannot be listed at all, no lock is called stale.
4. **Names before content.** Conflict copies are recognised by the naming
   schemes of iCloud (`name 2`), Dropbox (`conflicted copy`) and Syncthing
   (`.sync-conflict-…`). Under `modules/`, `hooks/` and `info/`, where users
   choose names, a copy also needs its original next to it.
5. **One bad object does not hide the rest.** A ref copy whose history cannot
   be read is reported as `unverifiable`; the run continues.
6. **A hook is checked where git is committing.** Run from a commit hook
   without a path, the tool walks up its parent processes to the git that ran
   the hook and takes that git's working directory, the working-tree top, as the repository, if `GIT_DIR`
   and `GIT_INDEX_FILE` agree with it. When git names the repository with an
   absolute `GIT_DIR` (linked worktrees, submodules, separate git
   directories), that is the answer wherever the hook changed directory.
   Otherwise (a main worktree, whose `.git/index` fits any repository) the
   tool asks what git would find from the current directory, physically: the
   git directory a relative `GIT_DIR` names there, or discovery. Another
   repository or worktree is refused (exit 2); the git directory, the hooks
   directory and no repository at all are the hook's own. A worktree that git
   would not use (a `.git`'s parent found from elsewhere) is a guess. Where no
   git is visible (a client that runs hooks itself), the
   current directory is taken, as git would take it: an index with no files
   then always blocks the commit, and a guessed working tree that holds none
   of HEAD's files is not checked.

## Changes

`move-out`, `fix`, `undo` and `prune-backups` change things. They share these rules:

1. **Plan first.** Without `--apply` a change command reads, plans and prints
   what it would do and what stops it; it writes nothing anywhere. Exit status
   0 planned or done, 1 refused (with codes), done with a warning or, for
   `fix`, with damage left to the user (MANUAL), 2 failed (rolled back or
   stopped) or could not run.
2. **Journal before acting, found by inode.** Each change is a journal entry in the state
   directory (`~/Library/Application Support/gitbrella/journal`, Linux
   `$XDG_STATE_HOME/gitbrella/journal`), saved atomically before every
   step. The next run of any change command finishes or rolls back an
   unfinished change first, judging by what is on disk, and `check` reports it
   as damage meanwhile. One change per repository at a time (`flock`). A
   repository is named by its path as stored on disk, and unfinished changes
   are found by the folder's inode first, so another spelling, a rename or
   an enclosing repository cannot hide one.
3. **Nothing is deleted, and only what a change made is moved.** Originals
   and anything set aside go to the change's backup directory; the user
   deletes them. Each entry records the device and inode of what it made (the
   copy, the `.git` file) and of the original, and its first save is complete;
   a roll-back moves only what matches. The `.git` file must match its bytes
   too: a file rewritten in place keeps its inode, and Linux gives a freed
   inode to the next new file at once. Once git used a moved directory, no
   roll-back goes over it unless it is still identical to the original.
4. **Prove, then switch.** A copy is compared file by file (kind, mode, size,
   time, symlink target, content) before anything is switched; the check runs
   again right before the switch, and git is asked afterwards whether it sees
   the same HEAD and status.
5. **Downloads stay off, but for a download step.** Moves across an iCloud
   Drive folder's edge need the file provider; they run on a thread of their
   own with the no-download policy lifted, only for files checked to be
   local. `fix`'s download step lifts it the same way to read cloud-only
   files of `.git` through, and then plans nothing else: only after it can
   the check read the objects and see all the damage.
6. **No state in synced folders.** The state directory and the moved git
   directories are refused if a sync client would sync them.
7. **A reversal always ends.** Rolling back or undoing a fix judges each step
   by what is on disk: a step whose result changed since (git used it, the
   sync client brought a copy back, its backup is gone, `.git` moved) stays
   as it is, the reason is recorded, and the older steps are still reversed;
   a rescue ref stays while the copy it keeps alive does. Downloaded files
   and refetched objects stay: they replace nothing. No entry is left
   unfinished for every later run to trip on.
8. **A refetch changes no ref.** `fix` fetches the remote's branches and tags
   into `refs/gcd-fetch/<change>/` with `--refmap=` (no remote-tracking ref
   moves) and deletes them before the step ends; a crash between is cleaned
   up by the reversal. The objects it was for must be there afterwards, or
   the fix is rolled back; after `fix --deep`, `git fsck` must pass too.
9. **A backup goes only when no undo can need it.** `prune-backups` deletes
   the backups of finished changes that are undone, rolled back or of a kind
   undo never reverses, and marks the entry `pruned`; a change still undoable
   keeps its backup whatever its age, since undo reaches it after newer ones.

## Layout

| Package | Job |
| --- | --- |
| `cmd/gitbrella` | command line, exit codes, hints when the path is not a repository |
| `internal/gitx` | safe git runner, version check |
| `internal/repo` | worktree, git dir, common dir, object format, ref storage |
| `internal/cloud` | sync provider detection, conflict-copy names, `.nosync` |
| `internal/scan` | metadata-only scans of `.git` and of the working tree |
| `internal/verify` | index checksum, index-versus-HEAD, ref-copy relation, operation state |
| `internal/procs` | running git processes, their working directories and start times, the git that ran a hook (macOS `ps` and `proc_pidinfo`, Linux `/proc`) |
| `internal/check` | turns all of the above into findings |
| `internal/report` | text and JSON output |
| `internal/fixtures` | builds damaged repositories for tests |
| `internal/state` | state and git-dir locations, repository id, per-repository lock |
| `internal/journal` | the write-ahead journal of changes |
| `internal/treecopy` | clone a directory keeping metadata, prove two trees identical |
| `internal/gitfile` | read and write `.git` files |
| `internal/icloud` | iCloud upload state (macOS, through the Objective-C runtime) |
| `internal/moveout` | plan, apply, recover, prepare, attach and undo a move |
| `internal/change` | what the change commands share: refusals, the check before acting, moves across the iCloud edge |
| `internal/fix` | findings to repairs, steps applied and reversed from what is on disk |
| `internal/shq` | shell quoting for the commands the tool suggests |
| `tools/snapshot` | development tool, not shipped: records file metadata under a directory and proves a run changed nothing |
