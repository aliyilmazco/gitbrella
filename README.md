# gitbrella

Finds the damage that cloud sync does to git repositories, without changing
anything and without downloading anything.

iCloud Drive, Dropbox, OneDrive, Google Drive and Syncthing sync files one by
one. Git expects its `.git` directory to change as a whole, so repositories in
synced folders end up with conflict copies such as `.git/index 2` or
`refs/heads/main 2`, missing indexes next to leftover `index.lock` files, and
objects that exist only in the cloud. Git's own FAQ warns against keeping a
repository in a synced folder, but defaults keep putting them there: macOS
Desktop & Documents in iCloud, Windows folder backup to OneDrive,
`Documents/GitHub`.

`gitbrella check` tells you, typically in under a second, whether a
repository has that damage. It is meant to run before an AI agent or a git hook
touches the repository. `gitbrella fix` repairs what has one right
repair, and `gitbrella move-out` ends the damage for good:
it moves `.git` out of the synced folder and leaves a `.git` file pointing at
it, so the sync client carries your files and git keeps its history to itself.

## What it promises

- **`check` is read-only.** It never writes to the repository. Git runs with
  `GIT_OPTIONAL_LOCKS=0`, automatic gc and maintenance off, and only commands
  that read. End-to-end tests for the risky cases (index copies, a missing
  index, partial clones, split indexes, moved git directories) record every
  file's size, mtime and inode before and after the check and require them to
  match.
- **No cloud downloads.** On macOS it turns off on-demand downloading for its
  own process (and the git it starts) before touching the disk; if macOS
  refuses, the check does not run (exit 2). Files that exist only in iCloud are
  reported, not fetched.
- **No network.** Partial clones never fetch missing objects
  (`GIT_NO_LAZY_FETCH`). On git older than 2.45 object checks are skipped
  instead.
- **Quiet on healthy repositories.** A lock is damage only when it is old and no
  git working on the repository (in any of its worktrees) could have written
  it: git creates a lock when it takes it, so a git that started later cannot
  hold it. Git's temporary files and other tools' files in `.git` are
  information. Run against 89 healthy
  working repositories, it reported no damage.

## Usage

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Exit status: **0** no damage, **1** damage found, **2** the check could not run.
The commands that change something (`move-out`, `fix`, `undo`) exit **0**
planned, done or nothing to do (`already`), **1** refused (with the reasons),
done with a warning, or with damage left to you (`fix`'s MANUAL list, which
`check` would still report), **2** failed (rolled back or stopped; the JSON
keeps what was recovered and the journal entry) or could not run.

`--deep` also runs a full `git fsck` (it reads every object, so it is skipped
when objects exist only in the cloud). `--provider` forces the sync client
instead of detecting it from the path: `auto` (the default), `icloud`,
`icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`,
`syncthing`, `nextcloud`, `cloud-storage` or `none`. Syncthing and Nextcloud
(or ownCloud) folders are recognised by the marker their client keeps in them
(`.stfolder`, `.sync_*.db`), wherever they are. `fix` and `move-out` take the
same `--provider`: give them the one you gave `check`, since some copy names
count only for a given client. The JSON says `"providerForced": true` then.

```
$ gitbrella check
gitbrella v0.1.0
repository: /Users/jane/Library/Mobile Documents/com~apple~CloudDocs/notes
git dir:    /Users/jane/Library/Mobile Documents/com~apple~CloudDocs/notes/.git
sync:       icloud

DAMAGE INDEX_COPY  index 2
       icloud conflict copy of index
       repair: 'gitbrella fix' repairs it
       equalsHEAD: true
       kind: icloud
       original: index
       originalExists: true
       validIndex: true
INFO   IN_CLOUD_FOLDER  /Users/jane/Library/Mobile Documents/com~apple~CloudDocs/notes
       repository is inside a folder synced by icloud; moving .git out of it prevents this damage

1 damage, 1 info. gitbrella changed nothing.
```

### As a git hook

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

A damaged repository blocks the commit; a healthy one commits as usual. The
tool knows git is running the hook and does not mistake the commit's own lock
for a stale one. For commit hooks (git passes them `GIT_INDEX_FILE`, or an
absolute `GIT_DIR`) it finds the repository from the git process that ran the
hook, so the hook may change directory first: into the git directory, its
hooks directory, or a directory outside any repository. When git names the
repository with an absolute `GIT_DIR` (linked worktrees, submodules,
separate git directories), every git command in the hook acts on it wherever
the hook goes, and so does the check. Otherwise it looks where git itself
would land from the current directory (the git directory a relative
`GIT_DIR` names there, or the repository found from there): if that is
another repository or another worktree of the same one (a hook that changes
into it, or the commit-message editor), it refuses (exit 2) rather than
answer for the wrong one. It also reads `GIT_INDEX_FILE` when it names a
file inside the git directory, as git sets it for commit hooks: if the index
the commit records was missing or empty, or no longer lists a HEAD file that
is still in the working tree, the commit is blocked before it can record
those files as deleted. An index file you point `GIT_INDEX_FILE` at elsewhere
yourself is not read: in a hook the repository's own index is checked
instead, and a check run by hand with it set refuses (exit 2). To stop tracking a file you keep on disk (`git rm --cached`,
untracking a directory you now ignore), commit with `--no-verify`. Run without a path, it checks the
repository git exported to the hook (`GIT_DIR`), so `--git-dir` dotfiles
setups and git directories inside their own worktree work too (it also
honours a `GIT_DIR` set in your shell when no path is given; unlike git, it
ignores a `GIT_DIR` that does not exist and refuses a linked worktree it
cannot trace back, rather than taking the current directory as the working
tree). To commit a deliberate "delete everything" while your files are still
on disk (`git rm -r --cached .`), skip the hook with `git commit --no-verify`.

### With a hook manager

- **pre-commit** (pre-commit.com): add the repository with the hook id
  `gitbrella` (pre-commit builds it with Go) or
  `gitbrella-system` (the binary on PATH); see `.pre-commit-hooks.yaml`.
- **lefthook**: pull in `hooks/lefthook.yml` with `remotes`.
- **husky**: `.husky/pre-commit` holding `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, then `overcommit --sign`.

Any non-zero exit stops the commit in all of them, so a check that cannot run
(exit 2) stops it too. Aider commits with `--no-verify` unless
`git-commit-verify: true` is set, and then no commit hook runs at all.
Obsidian Git (desktop) commits with the git on your system and runs the hook;
put the folder holding `gitbrella` in its "Additional PATH" setting, as
apps started from the Dock do not get the shell's PATH. Obsidian Git on
mobile (isomorphic-git) runs no hooks.

### For agents

`--json` prints the same report as a stable, versioned document
(`schemaVersion: 1`, `findings` is always a list). Decide on the exit status,
show the findings to the user.

Agents' own hooks stop a tool call only on exit 2: an exit 1 ("damage found")
lets the call go ahead. `gitbrella hook --for claude|codex` speaks
their hooks. It reads the event the agent passes on stdin:

- **Before a tool call** (`PreToolUse`): in a damaged repository, a git
  command that writes (commit, add, reset, rebase, push, and anything not
  known to only read) and a write into `.git` stop the call (exit 2, the
  reason on stderr for the agent). Reading, editing your files, `git restore`
  and `gitbrella` itself go on, so the damage can be looked at and
  repaired. Where git's files exist only in the cloud, or git cannot use the
  repository at all (a renamed `HEAD`, a `.git` file pointing nowhere),
  every git command stops: it would hang, or act on an enclosing repository.
  Writes into `.git` go on there: that repair is done by hand inside `.git`,
  as the reason says (`mv '.git/HEAD 2' .git/HEAD`).
  The repository checked is the one holding the files the call works on (a
  Codex patch names its own; a path inside `.git` counts for the working tree
  above it). For a shell command it is the session's folder and every folder
  its git commands run in (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`,
  `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), the writing ones first,
  at most five; when a writing one does not fit, the call goes through with a
  notice. Each folder is judged by what the call does there: from a damaged
  session folder, `git -C ../sound commit` goes on. A folder that is no repository, and a bare one, pass quietly.
  Anything else that keeps the check from running (including the 20-second
  `--timeout`, one deadline for the whole call) lets the call through with a
  notice; `--fail-open` does that where git cannot use the repository as
  well. The command is read, never run: quotes, comments, subshells,
  substitutions (also in an unquoted heredoc), heredocs, variables it sets,
  `&&`, `||`, `if`, `case` and loops (after `cd a || cd b`, or a `cd` to a
  folder that is not there, each folder the shell may be in counts; a `for`
  loop is read once per value), wrappers (`timeout`, `sudo`, `nice`, `xargs`,
  `env`), and the scripts a shell is given (`-c`, `eval`, `watch`, `source`,
  a file, a heredoc, `bash < f`, a pipe into `sh`). Another program writing
  inside `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm
  index.lock`, `sed -i` on `.git/config`, `rm -rf` or `find -delete` on a
  folder holding `.git`, a Codex patch through the shell) counts as a write
  into `.git`; copying `.git` away, `tar`ring it or reading it does not.
  `gh pr checkout` and `git-lfs` count as git commands that write. A reading
  that takes more than 5000 steps or half the `--timeout` stops, and the
  command counts as a git command that writes in a folder it cannot tell. Where the word `git` appears in a place the reading does not follow,
  the call counts as a git command that writes; where it cannot tell the
  folder a git command writes to (`cd "$(…)"`, `for d in */`), the command
  counts in every folder checked and the call gets a notice. Functions and
  aliases the command defines are read where they are called; a script
  piped into a shell from a program the reading does not follow
  (`curl … | sh`) counts as a git command that writes where it runs. A shell script
  run by its path or given to a shell (`./deploy.sh`, `bash deploy.sh`) is read
  when it is a small file on this machine (through a symlink too, once even
  when it runs itself); one given to a shell that is there but cannot be read
  (too big, only in the cloud) counts as a git command that writes where it
  runs. `make`, `npm run` and the like are
  not read. The remedy the
  `core.worktree` hint names (`git config --file … --unset core.worktree`) goes
  on where every other git command stops.
- **At a session start** (`SessionStart`): one line on stdout, which the agent
  reads as context, says the repository is in a synced folder (unless its git
  directory is already outside it) and what is damaged. It never blocks.

It changes nothing. Claude Code, in `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex, in `.codex/hooks.json` (Codex runs a hook only after you trust it in
`/hooks`):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

The check runs once for each of those folders, typically 0.1 to 0.4 seconds
each, so a patch spread over several folders of one repository checks it
several times; `--timeout` bounds the whole call.

## Repairing

```sh
gitbrella fix                 # what it would repair, and what it leaves to you
gitbrella fix --apply         # repair
gitbrella undo --apply        # put everything back
gitbrella history             # the changes made here, their backups and sizes
gitbrella prune-backups       # which backups no undo can need any more
```

`check` says under each damage whether `fix` repairs it (`repair:`, and
`"repair": "fix"` or `"manual"` in the JSON), naming `fix --provider NAME`
when the check was given one; inside commit hooks it does not ask.

`fix` repairs only where there is one right answer and nothing you may have
meant is overwritten:

| Damage | Repair |
| --- | --- |
| conflict copies in `.git` next to their original (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, Dropbox and Syncthing copies, and a copy of a ref's lock such as `refs/heads/main 2.lock`); an index copy only when it is identical to the index or a valid index equal to HEAD | moved into the backup; an object only the copy pointed at (a commit, a tag, a tree or a file) is first kept as `refs/gcd-rescue/<name>` |
| a leftover rebase directory, `desktop.ini`, `Icon` and similar files git did not make (unless git reads one as a branch, or its name is an object's), a copy of `.git` in the working tree whose commits the repository has | moved into the backup |
| index missing, with no copy or lock of it anywhere | made from HEAD (`git read-tree HEAD`: the files are not touched); only with a commit to build from (when HEAD's commit is missing too, it is fetched first, as below) and outside a sparse checkout |
| an index that lists no files (a valid, not split index outside a sparse checkout) | moved into the backup and made from HEAD: nothing was staged in it |
| an orphan `index.lock` that is a valid index equal to HEAD | it becomes the index |
| an empty branch file | the commit its reflog last recorded |
| `.git.nosync` that git would add to a commit | the line `/.git.nosync/` in `.git/info/exclude` |
| HEAD's commit missing, or an empty loose object file (with no copy next to it) | the empty file is moved into the backup and the objects are fetched again (`git fetch --refetch`, git 2.36 or newer) from `--remote`, else the branch's remote, else `origin`, else the only remote; the remote's branches and tags are fetched into a temporary `refs/gcd-fetch/` namespace that is deleted afterwards, so no ref changes. If the remote does not bring the object back, the fix is rolled back |
| with `fix --deep`: objects `git fsck` reports damaged | every object of the remote's branches and tags is fetched again, the same way; `git fsck` runs again after it, and if it still reports errors (an object no remote has, a corrupt file that stays) the fix is rolled back |
| conflict copies of your own files (`notes 2.md`) that git does not track and that hold the same bytes as the file they copy | moved into the backup (`git add -A` would commit them); a copy that differs, is tracked, a link, only in the cloud or in a submodule stays |
| another machine's `gc.pid` older than an hour; a `gc.log` | moved into the backup, so git's automatic gc runs here again |
| files of `.git`, or of an alternate object store, that exist only in the cloud | downloaded, and nothing else: until then the check cannot read the objects, so the rest of the damage is listed as MANUAL for the next `fix` run |

Everything else is listed as MANUAL with what to run. That includes
anything that may hold work found nowhere else:
- a damaged index (git may still read it, staged work and all);
- an index copy or an `index.lock` that differs from HEAD, and a copy of
  `index.lock`;
- a copy whose original is missing (it may be the only one: put it back,
  unless it is a copy of a lock, which is rescued and moved aside);
- a lock no git on this machine holds (git on another device or in a
  container may; its advice names the commit in it);
- a copy that names more than 100 objects (a long reflog);
- a copy of the branch some HEAD names, of its reflog, or of a directory
  holding that branch (git reads it, whatever its name looks like).

It also includes anything where fix would have to choose between two
versions, or could not keep a copy's commits alive:
- anything in the object store: copies, files named after an object (check
  which matches its hash), empty object files with no remote to fetch from;
- anything in a submodule's git directory (run `fix` in the submodule);
- a branch copy naming a commit that is not here yet (it may still arrive);
- a copy of `.git` in the working tree with commits the repository lacks
  (which of the two is current is yours to say);
- an empty file next to a copy of it, or of its directory;
- a broken linked-worktree admin directory (`git worktree repair`);
- an index that lost files still on disk (untracking them on purpose looks
  the same; when nothing else is staged, the advice says `git read-tree HEAD`
  gives them back and loses nothing);
- objects `git fsck` reports damaged, without `--deep` or without a remote.

`fix` does not touch reftable repositories at all; there `check`'s
`REFTABLE_TABLE_UNLISTED` says how to keep another device's refs. The
backup must be on the repository's disk, so that what fix moves aside moves
in one step; `fix` refuses otherwise (a download-only fix moves nothing and
does not need it).

Each step is recorded before it acts, and the check runs again at the end: if
anything it repaired is still reported, or new damage appeared, the steps are
reversed. A killed fix is rolled back by the next change command run with
`--apply` in that repository, which then stops; run it again. After 30
seconds (`--watch`) `fix` checks once more: damage that came back was brought
back by the sync client, and the lasting cure is `move-out`.

`undo` reverses the newest change first, a fix or a move. A step whose result
changed since the fix is left as it is and listed, never overwritten: git
rewrote the rebuilt index, the sync client brought a copy back, a rescue ref
moved, the backup was deleted, or `.git` moved. A rescue ref stays while the
copy it keeps alive stays in the backup. Downloaded files stay on the disk
and refetched objects stay in the object store: neither replaces anything.
The rest is reversed, and the undo finishes, so the change before it can be
undone next.

`history` lists the changes this device made to the repository (fixes,
moves, undos), the backup each keeps with its size, and which one `undo`
reverses next. `prune-backups` deletes, only with `--apply`, the backups of
changes that are undone, rolled back or never undoable (an undo, a prepare,
an attach), and whose last step is older than `--older-than` (default
`720h`). A fix or move that can still be undone keeps its backup, even under
newer changes.

## Moving .git out of the synced folder

```sh
gitbrella move-out            # what would happen, and what stops it
gitbrella move-out --apply    # do it
gitbrella undo --apply        # move it back
```

`move-out` copies `.git` to `~/.local/share/gitbrella/gitdirs/<name>`
(an APFS clone on macOS: no space is used), proves the copy identical file by
file, moves the original into a backup under
`~/Library/Application Support/gitbrella` (Linux:
`$XDG_STATE_HOME/gitbrella`), and writes a `.git` file such as
`gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`.
The path is relative, so every device that keeps the folder at the same place
under its home writes the same `.git` file. Then it checks that git finds the
same HEAD, status, configuration and hooks directory, and for 30 seconds
(`--watch`) that the sync client leaves `.git` alone and, under iCloud,
uploads the new file. The original is moved aside before the last comparison,
so a write that slipped in while copying is caught and the move undone.
Under iCloud, `move-out` refuses while iCloud is still uploading the
repository; `--wait-upload 10m` waits up to that long instead.

Every change is recorded in a journal before it happens, with the device and
inode of everything it makes (and the `.git` file's bytes); a roll-back
touches nothing else. If the process
dies half way, the next `move-out --apply` (or `undo --apply`) finishes or
rolls it back and stops there (run the command again to go on), found by the
folder's inode even under
another spelling, after a rename, or inside another repository, and `check`
reports the unfinished change as damage in the meantime. Once git has used the
moved directory, nothing rolls back over it: if it holds anything the original
lacks, the command stops and names both. Nothing is deleted: the original
stays in the backup until you remove it. `undo` brings back the git directory
in use now, with the commits made since the move; the backup from before the
move is left alone. If the sync client has evicted the `.git` file (only its
name is on this Mac), `undo` and the recovery ask you to download it first:
reading it would download it, which the tool never does.

If a roll-back finds both the original (in the backup) and a new `.git`
(made after the move stopped, by you or the sync client), it overwrites
neither and says so: compare them, move the `.git` you do not want out of the
folder, and run the command again.

It refuses (exit 1, with the reasons) while git or something holding a git
lock works on the repository, during a merge or rebase, while files of `.git`
exist only in the cloud, while iCloud is still uploading the folder, while a
socket (a running `git fsmonitor--daemon`) is in `.git`, when two git
directories exist after a move, and for layouts whose links would break:
linked worktrees, submodules, borrowed objects (`alternates`), a `.git` that
is a symlink or not the git directory itself. Other damage `check` finds is
moved as it is and listed as carried: run `check` again afterwards.

### Other devices

The sync client deletes `.git` on every other device that syncs the folder
and brings the `.git` file instead. To keep their history:

1. On each other device, before the move: `gitbrella move-out --prepare --apply`
   copies its `.git` to where the `.git` file will point.
2. On one device: `gitbrella move-out --apply`.
3. On each other device, once the `.git` file has arrived:
   `gitbrella move-out --attach --apply`. A device that was not prepared
   clones the history instead: `--attach --from <remote URL> --apply`. Either
   way the working tree is not touched.

The `.git` file arrives through the sync client, so attach trusts it only as
far as this device's records go: it must point into this device's
`~/.local/share/gitbrella/gitdirs`, and a directory already there is
used only if `--prepare` made it for this repository.

From then on the sync client carries the files and not the history. Share
commits through a remote. On a device whose files the sync client already
updated, `git pull` refuses ("would be overwritten"): use `git fetch` and
`git reset --mixed @{u}`, which moves the branch and the index and leaves the
synced files as they are.

## What it reports

Damage (exit 1):

| Code | Meaning |
| --- | --- |
| `INDEX_COPY` | conflict copy of the index (`index 2`; Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive and Seafile names; Google Drive's `index (1)`, OneDrive's `index-COMPUTER` in a OneDrive folder, and the older Nextcloud, Synology Drive, pCloud and Resilio names, which are matched only for git's own file names); tells whether it is a valid index and whether it matches HEAD |
| `REF_COPY` | conflict copy of a branch or tag ref; tells whether the copy is older, newer, diverged or names a missing commit |
| `COPY_IN_GITDIR` | any other conflict copy inside `.git` |
| `INDEX_MISSING` | the index is gone; git would show every file as deleted (inside a commit hook: the commit would delete every file) |
| `INDEX_EMPTY` | the index lists no files while HEAD's files are still in the working tree, as when git rebuilt it after the sync client deleted it; a commit would delete every file |
| `INDEX_SHRUNK` | the index no longer lists HEAD files that are still in the working tree (in a sparse checkout: any HEAD file), as when git rebuilt it from the files staged after the sync client deleted it; a commit would delete them. Renames, a directory turned into a symlink or into a submodule `.gitmodules` declares, and files deleted from disk too are not counted; when HEAD's files cannot be listed it is reported unverified |
| `HEAD_COMMIT_MISSING` | HEAD names a commit that is not in the repository (the ref arrived before its objects) |
| `ORPHAN_INDEX_LOCK` | the index is gone and an old `index.lock` was left behind (often a complete index) |
| `INDEX_INVALID` | the index fails its own checksum |
| `STALE_LOCK` | a git lock that no running git can hold: none works on the repository, or all of them started after the lock was written. Only git processes on this machine are seen: before removing the lock, make sure no git uses the repository in a container, on another machine sharing the folder, or inside an app that embeds git |
| `DATALESS_IN_GITDIR` | files in `.git` exist only in the cloud; object checks were skipped |
| `FOREIGN_FILE_IN_GITDIR` | a file git did not create in `refs/` or `objects/` (`desktop.ini`, `.tmp` files in `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | a zero-byte object or ref |
| `GITDIR_COPY` | a copy of the git directory in the working tree (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | the git directory sits inside the working tree under another name (`.git.nosync`) and `git add -A` would commit it |
| `WORKTREE_ADMIN_BROKEN` | a linked worktree lost its admin files |
| `STALE_OPERATION_STATE` | a leftover rebase directory makes git think a rebase is running |
| `FSCK_ERRORS` | `--deep` only: `git fsck` reported errors (when it does not finish, the same code is information) |
| `REFTABLE_TABLE_UNLISTED` | a reftable repository holds ref tables its list does not name: another device's, carried in by the sync client while its list became a copy git never reads. The next `git pack-refs` or maintenance deletes them, and that device's branches with them; copy the git directory somewhere safe first |
| `CHANGE_UNFINISHED` | a `fix`, `undo` or `move-out` of gitbrella did not finish (it was killed); run the command it names with `--apply` to finish or roll it back before using git there |
| `GIT_DIR_RETURNED` | after `move-out`, `.git` is a directory again (the sync client brought the old one back), so two git directories exist; compare them and keep one before committing |
| `STAGED_CONFLICT_COPY` | the index adds a sync conflict copy of a file it also lists (`notes 2.md` beside `notes.md`, a copied directory `src 2/`, Syncthing names); a commit would put the copy into history. Names alone decide; a rename to such a name, and a file named like a copy without its original, are not counted; not checked under a split index |

Information (exit status unchanged): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED`
(the git directory was moved out of the synced folder: the safe layout),
`WORKTREE_COPIES` (conflict copies of your own files), `GIT_RUNNING`,
`LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`,
`NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`,
`DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`,
`PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`,
`DATALESS_IN_ALTERNATE` (an alternate object store holds cloud-only files, so
object checks were skipped), `GC_PID_FOREIGN` (another machine's `gc.pid`:
git skips its automatic gc for 12 hours), `GC_LOG_FRESH` (a `gc.log` less than
a day old stops automatic gc), `HOOKS_IN_SYNCED_FOLDER` (`core.hooksPath` or a
`core.fsmonitor` command lives in a synced folder outside the git directory,
so a change on another device runs here), `WORKTREE_GUESSED` (`GIT_DIR` without a working
tree, and none of HEAD's files are in the current directory: the working tree
was not checked).

When the path is not a repository at all, it says why if the cause is on disk:
a renamed `.git 2`, git's own files existing only in the cloud, a git directory
without HEAD, a `.git` file pointing at a git directory that is gone, or a
`core.worktree` that points at a folder this machine does not have (another
machine's setting the sync client carried in). If the
path has a `.git` that git cannot use, it stops there (exit 2) instead of
answering for an enclosing repository, which is what git itself would do.

## Install

Requires git 2.31 or newer. macOS and Linux, amd64 and arm64, no cgo.

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

or take an archive from the releases page (both after the first tagged
release).

### Verifying a release

Each release carries `checksums.txt` (the SHA-256 of each archive and SBOM),
its keyless cosign signature `checksums.txt.sigstore.json` made by the release
workflow, an SPDX SBOM for each archive, and a GitHub build-provenance
attestation. Check the signature first, then the archive against the signed
list (the example is v1.2.3 for Apple silicon). Use cosign v3.1.3 or newer
(v2.6.5 on the v2 line): older versions can be fooled by a forged bundle
(GHSA-fx35-mq7g-6g98).

```sh
cosign verify-blob \
  --certificate-identity 'https://github.com/aliyilmazco/gitbrella/.github/workflows/release.yml@refs/tags/v1.2.3' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --bundle checksums.txt.sigstore.json checksums.txt
grep ' gitbrella_1.2.3_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c
gh attestation verify gitbrella_1.2.3_darwin_arm64.tar.gz \
  --repo aliyilmazco/gitbrella \
  --signer-workflow aliyilmazco/gitbrella/.github/workflows/release.yml \
  --source-ref refs/tags/v1.2.3 --deny-self-hosted-runners
```

A release is built by `.github/workflows/release.yml` from a pushed tag with
`.goreleaser.yaml` and stays a draft until a person publishes it. The binaries
are built with `-trimpath` and the commit's time, so the same tag and the same
Go release (the workflow takes the newest 1.26 patch) build the same
binaries; the archives and SBOMs around them carry their build time.

## Limits

- `fix` repairs only where there is one right answer and leaves the rest to
  you with the command to run; the lasting fix is `move-out`, which keeps
  `.git` outside the synced folder.
- Conflict copies made by iCloud itself across two devices were not reproduced
  in tests; the detection is tested with copies that follow the same naming.
- Codex does not pass its shell tool's `workdir` to hooks: a command Codex
  runs in another folder is checked in the session's folder.
- Windows and OneDrive on Windows are untested. reftable repositories get the
  file-level checks and the unlisted-table check; ref copies are not compared
  with their branches there.
- Copy names whose mark a name may carry on purpose are matched only where
  git itself makes the name, never in the working tree: Google Drive's
  `index (1)` for git's own files and in `refs/`; OneDrive's
  `index-COMPUTER` for git's own files at the top of a OneDrive folder, with a
  capital in the device name and no word people end a backup's name with
  (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` are yours), at
  most 15 characters (a longer Mac name is missed); the older Nextcloud
  `_conflict-YYYYMMDD-HHMMSS`, Synology Drive's, pCloud's `.conflicted` and
  Resilio's `.Conflict` for git's own files at the top (the older Nextcloud
  one also in `refs/` and `logs/` of a Nextcloud folder, beside its
  original). A branch some HEAD names (as `ref:` with any spacing, or as a
  symlink), its reflog and a directory holding it are never set aside,
  whatever their names. So `report (1).pdf`, a branch
  `fix.Conflict`, a tag `v1.0.conflicted` or `config-backup` are ordinary
  names.
- Submodules are checked from inside the submodule. From the superproject, a
  conflict copy inside `.git/modules/<name>/` is only reported when its
  original is next to it, and a submodule named like a copy (`lib 2`) is
  reported as one.
- A linked worktree's missing index is reported when that worktree is checked,
  not when the main one is.
- With `.git` symlinked into a store outside the synced folder, run git and the
  check from the linked working tree. From the store directory, which holds
  none of the files, an emptied index looks like a deliberate "delete
  everything" and does not block the commit.
- Some deliberate commits need `--no-verify`: stopping to track files that
  stay on disk, deleting every file where objects must not be read, a case-
  or Unicode-only rename of a file that has another hard link, and in a
  sparse checkout (cone mode or not) any commit that removes or renames a
  tracked file or moves it by a type change (a file becoming a directory, a
  directory becoming a symlink or a submodule). git's sparse patterns decide
  which files are on disk there, so a deliberate removal cannot be told from
  a lost index; the patterns are not read.
- If the sync client loses the branch ref itself (its loose file deleted
  before a new `packed-refs` arrives) along with the index, HEAD looks like a
  new branch and the commit is not blocked: it starts a new history holding
  only the staged files. Nothing on disk is deleted, and the old history is
  still in the reflog (`git reflog`).
- Under a split index, or where objects must not be read (partial clone on
  git older than 2.45, cloud-only alternates), the index is not compared with
  HEAD file by file: only a missing or empty index is caught.
- With `core.splitIndex`, an index that lists no files is not recognised (its
  header counts only the split part), so a commit from it is not blocked.
- Hooks: only commit hooks are found from the git process that ran them. Other
  hooks (`pre-push`, `pre-applypatch`) must run the check from the working
  tree, or they are refused (exit 2). Where the git process cannot be seen (a
  client that runs hooks itself, or process listing blocked), the check falls
  back to the current directory: then, if git's directory does not point back
  to the working tree (`--separate-git-dir`, `.git.nosync`, `--work-tree`), an
  index with no files always blocks the commit (use `--no-verify` for a
  deliberate wipe), a current directory with none of HEAD's files is not
  checked as the working tree (`WORKTREE_GUESSED`), and a hook that changed
  into another repository or into the git directory is refused (exit 2).
  Receive hooks (`pre-receive`) are not a supported gate.

- `move-out`: a `.git` file is not understood by every tool. Git and tools
  that run git work; isomorphic-git (Obsidian Git on mobile) and tools that
  open `.git` as a directory do not.
- `move-out`: renaming or moving the synced folder breaks the relative `.git`
  file. `check` then names the missing directory; put the folder back, or
  point the `.git` file at the git directory by hand.
- `move-out` needs the synced folder at the same place under home on every
  device (iCloud Drive always is; Dropbox may be at `~/Dropbox` on one device
  and `~/Library/CloudStorage/Dropbox` on another). Where it is not, the `.git`
  file points elsewhere there and `--attach` refuses (`TARGET_UNEXPECTED`).
- The moved git directory stays on home's filesystem; there is no option to
  put it elsewhere.
- `move-out --prepare` copies the device's history at that moment: move out
  soon after preparing, without new commits in between.
- `move-out` and `fix` were tried on iCloud Drive's own folder, not inside an
  app's iCloud folder (Obsidian's `iCloud~md~obsidian`).

## License

MIT
