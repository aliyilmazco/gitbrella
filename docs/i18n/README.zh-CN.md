<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="gitbrella 标志：一把以 git 分支为伞柄的伞">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  找出云同步对 git 仓库造成的损坏，<br>
  不修改任何东西，也不下载任何东西。
</p>

<p align="center">
  <a href="https://github.com/aliyilmazco/gitbrella/actions/workflows/ci.yml"><img src="https://github.com/aliyilmazco/gitbrella/actions/workflows/ci.yml/badge.svg" alt="ci"></a>
  <a href="https://github.com/aliyilmazco/gitbrella/releases"><img src="https://img.shields.io/github/v/release/aliyilmazco/gitbrella" alt="release"></a>
  <a href="https://pkg.go.dev/github.com/aliyilmazco/gitbrella"><img src="https://pkg.go.dev/badge/github.com/aliyilmazco/gitbrella.svg" alt="Go Reference"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/github/license/aliyilmazco/gitbrella" alt="license"></a>
</p>

<p align="center">
  <a href="../../README.md">English</a> ·
  <b>简体中文</b> ·
  <a href="README.zh-TW.md">繁體中文</a> ·
  <a href="README.ja.md">日本語</a> ·
  <a href="README.ko.md">한국어</a> ·
  <a href="README.es.md">Español</a> ·
  <a href="README.pt-BR.md">Português</a> ·
  <a href="README.fr.md">Français</a> ·
  <a href="README.de.md">Deutsch</a> ·
  <a href="README.it.md">Italiano</a> ·
  <a href="README.ru.md">Русский</a> ·
  <a href="README.uk.md">Українська</a> ·
  <a href="README.pl.md">Polski</a> ·
  <a href="README.tr.md">Türkçe</a> ·
  <a href="README.ar.md">العربية</a> ·
  <a href="README.hi.md">हिन्दी</a> ·
  <a href="README.id.md">Bahasa Indonesia</a> ·
  <a href="README.vi.md">Tiếng Việt</a>
</p>

<p align="center">
  <a href="#usage">用法</a> ·
  <a href="#repairing">修复</a> ·
  <a href="#moving-git-out-of-the-synced-folder">移出 .git</a> ·
  <a href="#what-it-reports">报告内容</a> ·
  <a href="#install">安装</a> ·
  <a href="#limits">局限</a>
</p>

iCloud Drive、Dropbox、OneDrive、Google Drive 和 Syncthing 逐个文件地同步。而 git 期望它的 `.git` 目录作为一个整体变化，因此位于同步文件夹中的仓库最终会出现 `.git/index 2` 或 `refs/heads/main 2` 这样的冲突副本、旁边残留着 `index.lock` 的丢失的 index，以及只存在于云端的对象。git 自己的 FAQ 警告不要把仓库放在同步文件夹里，但默认设置总是把仓库放到那里：macOS 的 iCloud 桌面与文稿、Windows 的 OneDrive 文件夹备份、`Documents/GitHub`。

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="同步客户端在设备之间逐个文件地复制 .git，留下 index 2 和 refs/heads/main 2 之类的冲突副本、丢失的 index 旁边的 index.lock，以及只存在于云端的对象">
</p>

`gitbrella check` 通常在一秒之内告诉你仓库是否有这种损坏。它的设计用途是在 AI 代理或 git 钩子接触仓库之前运行。`gitbrella fix` 修复那些只有一种正确修法的损坏，`gitbrella move-out` 则一劳永逸地终结损坏：它把 `.git` 移出同步文件夹，并在原处留下一个指向它的 `.git` 文件，这样同步客户端搬运你的文件，git 则把历史留给自己。

| 命令 | 作用 |
| --- | --- |
| `gitbrella check` | 报告损坏；只读 |
| `gitbrella fix` | 修复只有一个正确答案的问题 |
| `gitbrella move-out` | 把 `.git` 永久移出同步文件夹 |
| `gitbrella undo` | 撤销最近一次修复或移动 |
| `gitbrella history` | 列出本设备上做过的更改 |
| `gitbrella prune-backups` | 删除已无法撤销的更改的旧备份 |
| `gitbrella hook --for claude\|codex` | 守护 AI 代理的工具调用 |

## 快速开始

**1. 查看。** 只读，不下载，不联网：

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # 退出码 0：健康，1：有损坏，2：无法运行
```

**2. 修复**只有一个正确答案的问题。不加 `--apply` 时只显示计划：

```sh
gitbrella fix                 # 它会修复什么，以及留给你处理什么
gitbrella fix --apply         # 修复；`gitbrella undo --apply` 可以还原
```

**3. 彻底解决。** 把 `.git` 移出同步文件夹：

```sh
gitbrella move-out            # 会发生什么，以及什么会阻止它
gitbrella move-out --apply
```

**持续守护。** 在损坏的仓库中阻止提交：

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

AI 代理请参阅[面向代理](#for-agents)。

## 它的承诺

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` 是只读的。** 它从不写入仓库。git 以 `GIT_OPTIONAL_LOCKS=0` 运行，关闭自动 gc 和维护，并且只执行读取类命令。针对高风险情形（index 副本、缺失的 index、部分克隆、拆分 index、移动过的 git 目录）的端到端测试会在检查前后记录每个文件的大小、mtime 和 inode，并要求它们一致。
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **不从云端下载。** 在 macOS 上，它在接触磁盘之前就为自身进程（以及它启动的 git）关闭按需下载；如果 macOS 拒绝，检查不会运行（退出码 2）。只存在于 iCloud 中的文件只会被报告，不会被下载。
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **不联网。** 部分克隆从不拉取缺失的对象（`GIT_NO_LAZY_FETCH`）。在早于 2.45 的 git 上，则改为跳过对象检查。
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **对健康仓库保持安静。** 只有当一个锁已经很旧，并且没有任何正在该仓库（任一工作树）上工作的 git 可能写下它时，它才算损坏：git 在获取锁时创建锁，所以较晚启动的 git 不可能持有它。git 的临时文件以及其他工具放在 `.git` 中的文件只作为信息。在 89 个健康的、正在使用的仓库上运行，它没有报告任何损坏。

<a name="usage"></a>

## 用法

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

退出状态：**0** 无损坏，**1** 发现损坏，**2** 检查无法运行。会做出更改的命令（`move-out`、`fix`、`undo`）的退出状态为：**0** 已计划、已完成或无事可做（`already`）；**1** 被拒绝（附原因）、完成但有警告，或完成但留有需要你处理的损坏（`fix` 的 MANUAL 列表，`check` 仍会报告它们）；**2** 失败（已回滚或已停止；JSON 保留已恢复的内容和日志条目）或无法运行。

`--deep` 还会运行完整的 `git fsck`（它会读取每个对象，因此当对象只存在于云端时会被跳过）。`--provider` 强制指定同步客户端，而不是根据路径检测：`auto`（默认）、`icloud`、`icloud-desktop-documents`、`dropbox`、`onedrive`、`google-drive`、`syncthing`、`nextcloud`、`cloud-storage` 或 `none`。Syncthing 和 Nextcloud（或 ownCloud）文件夹无论在哪里，都通过其客户端在其中保存的标记（`.stfolder`、`.sync_*.db`）识别。`fix` 和 `move-out` 接受同样的 `--provider`：请给它们传入与 `check` 相同的值，因为某些副本名称只对特定客户端有效。此时 JSON 中会显示 `"providerForced": true`。

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="gitbrella check 的输出：可由 gitbrella fix 修复的 DAMAGE INDEX_COPY index 2，以及 INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>同样的输出（文本）</summary>

```
$ gitbrella check
gitbrella v1.0.0
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

</details>

### 作为 git 钩子

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit 运行 pre-commit 钩子，钩子运行 gitbrella check：退出码 0 允许提交，1 或 2 阻止提交">
</p>

损坏的仓库会阻止提交；健康的仓库照常提交。该工具知道是 git 在运行钩子，不会把这次提交自己的锁误认为陈旧的锁。

**检查的是哪个仓库。** 对于提交钩子（git 会传给它们 `GIT_INDEX_FILE` 或绝对路径的 `GIT_DIR`），它通过运行钩子的 git 进程找到仓库，因此钩子可以先切换目录：切到 git 目录、其 hooks 目录，或任何仓库之外的目录。当 git 用绝对路径的 `GIT_DIR` 指明仓库时（链接工作树、子模块、独立的 git 目录），无论钩子切到哪里，钩子中的每条 git 命令都作用于该仓库，检查也一样。否则，它会查看 git 自己从当前目录会定位到哪里（那里的相对 `GIT_DIR` 所指的 git 目录，或从那里找到的仓库）：如果那是另一个仓库，或同一仓库的另一个工作树（切换进去的钩子，或提交信息编辑器），它会拒绝（退出码 2），而不是为错误的仓库作答。

不带路径运行时，它检查 git 导出给钩子的仓库（`GIT_DIR`），因此 `--git-dir` 形式的 dotfiles 配置以及位于自身工作树内的 git 目录同样可用。不带路径时，它也会遵循你在 shell 中设置的 `GIT_DIR`；与 git 不同，它会忽略不存在的 `GIT_DIR`，并拒绝无法追溯来源的链接工作树，而不是把当前目录当作工作树。

**提交所记录的 index。** 当 `GIT_INDEX_FILE` 指向 git 目录内的文件时（git 为提交钩子就是这样设置的），它会读取该文件：如果提交所记录的 index 缺失或为空，或者不再列出仍在工作树中的某个 HEAD 文件，那么在提交把这些文件记录为删除之前就会被阻止。你自己把 `GIT_INDEX_FILE` 指向别处的 index 文件不会被读取：在钩子中改为检查仓库自己的 index，而在设置了该变量时手动运行的检查会拒绝（退出码 2）。

**有意的删除。** 要停止跟踪一个仍保留在磁盘上的文件（`git rm --cached`，取消跟踪一个你现在要忽略的目录），或在文件仍在磁盘上时有意提交"全部删除"（`git rm -r --cached .`），请用 `git commit --no-verify` 跳过钩子。

### 搭配钩子管理器

- **pre-commit**（pre-commit.com）：以钩子 id `gitbrella`（pre-commit 用 Go 构建它）或 `gitbrella-system`（使用 PATH 中的二进制文件）添加本仓库；参见 `.pre-commit-hooks.yaml`。
- **lefthook**：通过 `remotes` 引入 `hooks/lefthook.yml`。
- **husky**：内容为 `gitbrella check` 的 `.husky/pre-commit`。
- **Overcommit**：`PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`，然后运行 `overcommit --sign`。

在所有这些工具中，任何非零退出都会阻止提交，因此无法运行的检查（退出码 2）也会阻止提交。除非设置了 `git-commit-verify: true`，Aider 会用 `--no-verify` 提交，此时根本不会运行任何提交钩子。Obsidian Git（桌面版）使用系统中的 git 提交并运行钩子；请把 `gitbrella` 所在的文件夹加入它的 "Additional PATH" 设置，因为从 Dock 启动的应用拿不到 shell 的 PATH。移动端的 Obsidian Git（isomorphic-git）不运行任何钩子。

<a name="for-agents"></a>

### 面向代理

`--json` 以稳定、带版本的文档形式输出同一份报告（`schemaVersion: 1`，`findings` 始终是列表）。根据退出状态做决定，把发现展示给用户。

代理自己的钩子只在退出码 2 时阻止工具调用：退出码 1（"发现损坏"）会让调用继续。`gitbrella hook --for claude|codex` 能对接它们的钩子。它读取代理通过 stdin 传入的事件：

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="代理的工具调用经过 gitbrella hook：读取和编辑照常进行；在损坏的仓库中，git 写操作和写入 .git 以退出码 2 被阻止">
</p>

- **工具调用之前**（`PreToolUse`）：在损坏的仓库中，会写入的 git 命令（commit、add、reset、rebase、push，以及任何未知是否只读的命令）和写入 `.git` 的操作会阻止该调用（退出码 2，原因写在 stderr 中供代理查看）。读取、编辑你的文件、`git restore` 以及 `gitbrella` 本身照常进行，以便查看和修复损坏。
  - *不可用的仓库。* 当 git 的文件只存在于云端，或 git 完全无法使用该仓库时（被重命名的 `HEAD`、指向不存在位置的 `.git` 文件），每条 git 命令都会被阻止：它会卡住，或作用于外层仓库。此时写入 `.git` 的操作照常进行：那种修复要像原因中说明的那样在 `.git` 内手动完成（`mv '.git/HEAD 2' .git/HEAD`）。`core.worktree` 提示中给出的补救措施（`git config --file … --unset core.worktree`）也照常进行。
  - *哪个仓库。* 包含该调用所处理文件的仓库（Codex 补丁会写明自己的文件；`.git` 内的路径算作其上方的工作树）。对于 shell 命令，是会话的文件夹以及其中 git 命令运行的每个文件夹（`cd DIR`、`pushd`、`git -C DIR`、`--git-dir`、`--work-tree`、`GIT_DIR`、`env -C`、`bash -c '…'`），写入的优先，最多五个；当某个写入的放不下时，调用会带着提示放行。每个文件夹按该调用在那里做的事判断：从损坏的会话文件夹执行 `git -C ../sound commit` 会照常进行。不是仓库的文件夹和裸仓库会静默通过。
  - *当检查无法运行时。* 任何使检查无法运行的情况（包括 20 秒的 `--timeout`，它是整个调用的唯一截止时间）都会让调用带着提示放行；`--fail-open` 让 git 无法使用仓库的情况也这样处理。
  - *如何解读命令。* 只读取，从不执行：引号、注释、子 shell、替换（也包括未加引号的 heredoc 中的替换）、heredoc、它设置的变量、`&&`、`||`、`if`、`case` 和循环（在 `cd a || cd b` 之后，或 `cd` 到不存在的文件夹之后，shell 可能所在的每个文件夹都计入；`for` 循环按每个值读取一次）、包装命令（`timeout`、`sudo`、`nice`、`xargs`、`env`），以及交给 shell 的脚本（`-c`、`eval`、`watch`、`source`、文件、heredoc、`bash < f`、通过管道传给 `sh`）。命令中定义的函数和别名在被调用处读取。通过路径运行或交给 shell 的 shell 脚本（`./deploy.sh`、`bash deploy.sh`），如果是本机上的小文件就会被读取（经由符号链接也可以；即使它运行自身也只读一次）。`make`、`npm run` 之类不会被读取。
  - *什么算写入。* 其他程序写入 `.git`（`rm .git/index.lock`、`> .git/HEAD`、`cd .git && rm index.lock`、对 `.git/config` 执行 `sed -i`、对包含 `.git` 的文件夹执行 `rm -rf` 或 `find -delete`、通过 shell 应用的 Codex 补丁）算作写入 `.git`；把 `.git` 复制走、`tar` 打包或读取它都不算。`gh pr checkout` 和 `git-lfs` 算作会写入的 git 命令。
  - *拿不准时。* 以下每一种都算作会写入的 git 命令：读取超过 5000 步或超过 `--timeout` 的一半（在它无法判断的文件夹中）；出现在读取无法跟进之处的 `git` 一词；从读取无法跟进的程序通过管道传给 shell 的脚本（`curl … | sh`），在它运行的地方；交给 shell 的、存在但无法读取的脚本（太大、只在云端），在它运行的地方。当无法判断某条 git 命令写入哪个文件夹时（`cd "$(…)"`、`for d in */`），该命令在每个被检查的文件夹中都计入，调用会收到提示。
- **会话开始时**（`SessionStart`）：在 stdout 输出一行，代理会把它当作上下文读取，说明仓库位于同步文件夹中（除非其 git 目录已经在外面）以及哪些地方损坏了。它从不阻止。

该钩子不修改任何东西。Claude Code，写在 `.claude/settings.json` 中：

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex，写在 `.codex/hooks.json` 中（只有在你于 `/hooks` 中信任某个钩子之后，Codex 才会运行它）：

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

检查对上述每个文件夹各运行一次，通常每次 0.1 到 0.4 秒，因此分布在一个仓库多个文件夹中的补丁会多次检查该仓库；`--timeout` 限制整个调用的时间。

<a name="repairing"></a>

## 修复

```sh
gitbrella fix                 # 它会修复什么，以及留给你处理什么
gitbrella fix --apply         # 修复
gitbrella undo --apply        # 全部还原
gitbrella history             # 在此做过的更改、它们的备份及大小
gitbrella prune-backups       # 哪些备份已不再被任何 undo 需要
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix 显示计划；fix --apply 记录每一步，然后再次检查：完成，或回滚；undo --apply 撤销最近一次更改">
</p>

`check` 会在每项损坏下注明 `fix` 是否能修复它（`repair:`，在 JSON 中为 `"repair": "fix"` 或 `"manual"`）；如果检查时指定了 provider，还会写出 `fix --provider NAME`。在提交钩子内部它不会询问。

`fix` 只在只有一个正确答案、且不会覆盖任何你可能有意保留的内容时才修复：

| 损坏 | 修复 |
| --- | --- |
| `.git` 中位于原件旁边的冲突副本（`HEAD 2`、`refs/heads/main 2`、`refs 2`、`packed-refs 2`、Dropbox 和 Syncthing 的副本，以及 ref 锁的副本，如 `refs/heads/main 2.lock`）；index 副本仅当它与 index 完全相同，或是与 HEAD 相等的有效 index 时 | 移入备份；只有该副本指向的对象（commit、tag、tree 或文件）会先保存为 `refs/gcd-rescue/<name>` |
| 残留的 rebase 目录、`desktop.ini`、`Icon` 以及其他非 git 创建的类似文件（除非 git 把它读作分支，或它的名字是某个对象的名字）、工作树中其提交仓库都已拥有的 `.git` 副本 | 移入备份 |
| index 缺失，且任何地方都没有它的副本或锁 | 从 HEAD 生成（`git read-tree HEAD`：不触碰文件）；仅当有可用于构建的提交（如果 HEAD 的提交也缺失，会先按下文方式拉取）且不在稀疏检出中时 |
| 不列出任何文件的 index（稀疏检出之外的有效、非拆分 index） | 移入备份并从 HEAD 生成：其中没有暂存任何内容 |
| 作为与 HEAD 相等的有效 index 的孤立 `index.lock` | 它成为 index |
| 空的分支文件 | 其 reflog 最后记录的提交 |
| git 会加入提交的 `.git.nosync` | 在 `.git/info/exclude` 中加入 `/.git.nosync/` 这一行 |
| HEAD 的提交缺失，或空的松散对象文件（旁边没有副本） | 空文件被移入备份，并重新拉取对象（`git fetch --refetch`，需要 git 2.36 或更新）：优先 `--remote`，否则分支的远程，否则 `origin`，否则唯一的远程；远程的分支和标签被拉取到临时的 `refs/gcd-fetch/` 命名空间，之后删除，因此没有任何 ref 改变。如果远程没能带回该对象，修复会回滚 |
| 使用 `fix --deep`：`git fsck` 报告损坏的对象 | 以同样方式重新拉取远程分支和标签的每个对象；之后再次运行 `git fsck`，若仍报告错误（没有任何远程拥有的对象、仍然存在的损坏文件），修复会回滚 |
| 你自己文件的冲突副本（`notes 2.md`），git 未跟踪且与其所复制的文件字节完全相同 | 移入备份（`git add -A` 会提交它们）；内容不同、被跟踪、是链接、只在云端或位于子模块中的副本会保留 |
| 其他机器上超过一小时的 `gc.pid`；`gc.log` | 移入备份，使 git 的自动 gc 在此处重新运行 |
| `.git` 或替代对象存储中只存在于云端的文件 | 下载，仅此而已：在此之前检查无法读取对象，因此其余损坏会作为 MANUAL 列出，留待下次运行 `fix` |

其余一切都作为 MANUAL 列出，并附上要运行的命令。其中包括任何可能保存着别处没有的工作的东西：
- 损坏的 index（git 可能仍能读取它，连同已暂存的工作）；
- 与 HEAD 不同的 index 副本或 `index.lock`，以及 `index.lock` 的副本；
- 原件缺失的副本（它可能是唯一的一份：请放回原处；如果它是锁的副本，则会被保护并移到一旁）；
- 本机上没有任何 git 持有的锁（另一台设备或容器中的 git 可能持有；其建议会写出其中的提交）；
- 指向超过 100 个对象的副本（很长的 reflog）；
- 某个 HEAD 所指分支的副本、其 reflog 的副本，或包含该分支的目录的副本（无论名字看起来像什么，git 都会读取它）。

还包括 fix 必须在两个版本之间做选择，或无法保住副本中提交的任何情况：
- 对象存储中的一切：副本、以对象命名的文件（请检查哪个与其哈希匹配）、没有远程可拉取的空对象文件；
- 子模块 git 目录中的一切（请在子模块中运行 `fix`）；
- 指向尚未到达此处的提交的分支副本（它可能还会到来）；
- 工作树中含有仓库缺少的提交的 `.git` 副本（哪一个是最新的由你决定）；
- 旁边有它自身或其目录副本的空文件；
- 损坏的链接工作树管理目录（`git worktree repair`）；
- 丢失了仍在磁盘上的文件的 index（有意取消跟踪看起来也一样；当没有其他暂存内容时，建议会说明 `git read-tree HEAD` 能把它们找回且不会丢失任何东西）；
- 未使用 `--deep` 或没有远程时，`git fsck` 报告损坏的对象。

`fix` 完全不处理 reftable 仓库；在那里，`check` 的 `REFTABLE_TABLE_UNLISTED` 会说明如何保住另一台设备的 ref。备份必须位于仓库所在的磁盘上，这样 fix 移到一旁的东西才能一步移动完成；否则 `fix` 会拒绝（只做下载的修复不移动任何东西，也不需要它）。

每一步在执行之前都会被记录，最后会再次运行检查：如果它修复过的东西仍被报告，或出现了新的损坏，这些步骤会被撤销。被终止的 fix 会由该仓库中下一个带 `--apply` 的更改命令回滚，该命令随后停止；请再运行一次。30 秒后（`--watch`），`fix` 会再检查一次：重新出现的损坏是同步客户端带回来的，持久的解决办法是 `move-out`。

`undo` 先撤销最近的更改，即一次修复或移动。某一步的结果如果在修复之后发生了变化，就保持原样并列出，绝不覆盖：git 重写了重建的 index、同步客户端带回了副本、救援 ref 移动了、备份被删除，或 `.git` 被移动。只要其保住的副本还在备份中，救援 ref 就会保留。已下载的文件留在磁盘上，重新拉取的对象留在对象存储中：两者都不替换任何东西。其余部分会被撤销，undo 完成，这样之前的那次更改就可以接着撤销。

`history` 列出本设备对该仓库做过的更改（修复、移动、撤销）、每项保留的备份及其大小，以及 `undo` 下一次会撤销哪一项。`prune-backups` 仅在加 `--apply` 时，删除那些已撤销、已回滚或从来不可撤销（undo、prepare、attach）且最后一步早于 `--older-than`（默认 `720h`）的更改的备份。仍可撤销的修复或移动会保留其备份，即使其后还有更新的更改。

<a name="moving-git-out-of-the-synced-folder"></a>

## 把 .git 移出同步文件夹

```sh
gitbrella move-out            # 会发生什么，以及什么会阻止它
gitbrella move-out --apply    # 执行
gitbrella undo --apply        # 移回去
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="之前：.git 在同步文件夹内。move-out 之后：同步文件夹中的 .git 文件指向文件夹外的 ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d">
</p>

`move-out` 把 `.git` 复制到 `~/.local/share/gitbrella/gitdirs/<name>`（在 macOS 上是 APFS 克隆：不占用空间），逐个文件证明副本完全相同，把原件移入 `~/Library/Application Support/gitbrella`（Linux：`$XDG_STATE_HOME/gitbrella`）下的备份，并写入一个 `.git` 文件，例如 `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`。该路径是相对的，因此每台把该文件夹放在主目录下相同位置的设备都会写出相同的 `.git` 文件。然后它检查 git 是否找到相同的 HEAD、状态、配置和 hooks 目录，并在 30 秒内（`--watch`）确认同步客户端没有动 `.git`，且在 iCloud 下上传了新文件。原件在最后一次比较之前被移到一旁，因此复制期间混入的写入会被发现，移动会被撤销。在 iCloud 下，当 iCloud 仍在上传该仓库时，`move-out` 会拒绝；`--wait-upload 10m` 则改为最多等待这么久。

每项更改在发生之前都会记录到日志中，连同它创建的每样东西的设备号和 inode（以及 `.git` 文件的字节）；回滚不会触碰任何其他东西。如果进程中途终止，下一次 `move-out --apply`（或 `undo --apply`）会完成或回滚它并停在那里（再次运行命令以继续）；即使路径写法不同、被重命名过，或位于另一个仓库内，也能通过文件夹的 inode 找到它。在此期间，`check` 会把未完成的更改报告为损坏。一旦 git 使用过移动后的目录，就不会再在其上回滚：如果它包含原件没有的内容，命令会停止并指出两者。不会删除任何东西：原件保留在备份中，直到你删除它。`undo` 会带回当前正在使用的 git 目录，连同移动之后做出的提交；移动之前的备份保持不动。如果同步客户端已经驱逐了 `.git` 文件（在这台 Mac 上只剩下它的名字），`undo` 和恢复操作会要求你先下载它：读取它就会触发下载，而该工具从不这样做。

如果回滚同时发现了原件（在备份中）和一个新的 `.git`（在移动停止后由你或同步客户端创建），它不会覆盖任何一方，并会告诉你：比较两者，把你不想要的 `.git` 移出文件夹，然后再次运行命令。

在以下情况下它会拒绝（退出码 1，附原因）：git 或持有 git 锁的东西正在操作该仓库时、合并或变基期间、`.git` 的文件只存在于云端时、iCloud 仍在上传该文件夹时、`.git` 中有套接字（正在运行的 `git fsmonitor--daemon`）时、移动后存在两个 git 目录时，以及链接会断裂的布局：链接工作树、子模块、借用的对象（`alternates`）、本身是符号链接或并非 git 目录本身的 `.git`。`check` 发现的其他损坏会原样移动并作为"随带"列出：之后请再次运行 `check`。

### 其他设备

同步客户端会在同步该文件夹的其他每台设备上删除 `.git`，并带去 `.git` 文件。要保住它们的历史：

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1：在其他每台设备上运行 move-out --prepare --apply。2：在一台设备上运行 move-out --apply。3：在其他每台设备上运行 move-out --attach --apply">
</p>

1. 在移动之前，在其他每台设备上：`gitbrella move-out --prepare --apply` 把该设备的 `.git` 复制到 `.git` 文件将要指向的位置。
2. 在一台设备上：`gitbrella move-out --apply`。
3. 在 `.git` 文件到达之后，在其他每台设备上：`gitbrella move-out --attach --apply`。未做准备的设备则改为克隆历史：`--attach --from <remote URL> --apply`。无论哪种方式，工作树都不会被触碰。

`.git` 文件是通过同步客户端到达的，因此 attach 只在本设备记录允许的范围内信任它：它必须指向本设备的 `~/.local/share/gitbrella/gitdirs`，并且那里已存在的目录只有在 `--prepare` 为这个仓库创建了它时才会被使用。

从此以后，同步客户端只搬运文件而不搬运历史。请通过远程共享提交。在文件已由同步客户端更新过的设备上，`git pull` 会拒绝（"would be overwritten"）：请使用 `git fetch` 和 `git reset --mixed @{u}`，它会移动分支和 index，并保持已同步的文件不变。

<a name="what-it-reports"></a>

## 报告内容

损坏（退出码 1）：

| 代码 | 含义 |
| --- | --- |
| `INDEX_COPY` | index 的冲突副本（`index 2`；Dropbox、Syncthing、Nextcloud/ownCloud、Proton Drive 和 Seafile 的命名；Google Drive 的 `index (1)`、OneDrive 文件夹中 OneDrive 的 `index-COMPUTER`，以及旧版 Nextcloud、Synology Drive、pCloud 和 Resilio 的命名，这些只针对 git 自己的文件名匹配）；说明它是否为有效 index 以及是否与 HEAD 一致 |
| `REF_COPY` | 分支或标签 ref 的冲突副本；说明该副本是更旧、更新、已分叉，还是指向缺失的提交 |
| `COPY_IN_GITDIR` | `.git` 中的任何其他冲突副本 |
| `INDEX_MISSING` | index 不见了；git 会把每个文件都显示为已删除（在提交钩子中：这次提交会删除每个文件） |
| `INDEX_EMPTY` | HEAD 的文件仍在工作树中，但 index 不列出任何文件，就像同步客户端删除 index 后 git 重建了它一样；提交会删除每个文件 |
| `INDEX_SHRUNK` | index 不再列出仍在工作树中的 HEAD 文件（在稀疏检出中：任何 HEAD 文件），就像同步客户端删除 index 后 git 用已暂存的文件重建了它一样；提交会删除它们。重命名、变成符号链接或变成 `.gitmodules` 声明的子模块的目录，以及也从磁盘上删除的文件不计入；当无法列出 HEAD 的文件时，报告为未验证 |
| `HEAD_COMMIT_MISSING` | HEAD 指向仓库中不存在的提交（ref 先于其对象到达） |
| `ORPHAN_INDEX_LOCK` | index 不见了，留下一个旧的 `index.lock`（往往是完整的 index） |
| `INDEX_INVALID` | index 未通过自身的校验和 |
| `STALE_LOCK` | 没有任何正在运行的 git 能持有的 git 锁：没有 git 在操作该仓库，或所有 git 都在写锁之后才启动。只能看到本机上的 git 进程：删除锁之前，请确认没有 git 在容器中、在共享该文件夹的另一台机器上，或在内嵌 git 的应用中使用该仓库 |
| `DATALESS_IN_GITDIR` | `.git` 中的文件只存在于云端；已跳过对象检查 |
| `FOREIGN_FILE_IN_GITDIR` | `refs/` 或 `objects/` 中非 git 创建的文件（`desktop.ini`、`objects/` 中的 `.tmp` 文件） |
| `EMPTY_FILE_IN_GITDIR` | 零字节的对象或 ref |
| `GITDIR_COPY` | 工作树中的 git 目录副本（`.git 2`） |
| `GIT_DIR_NOT_IGNORED` | git 目录以另一个名字位于工作树内（`.git.nosync`），`git add -A` 会提交它 |
| `WORKTREE_ADMIN_BROKEN` | 链接工作树丢失了其管理文件 |
| `STALE_OPERATION_STATE` | 残留的 rebase 目录让 git 以为变基正在进行 |
| `FSCK_ERRORS` | 仅 `--deep`：`git fsck` 报告了错误（未运行完成时，同一代码作为信息） |
| `REFTABLE_TABLE_UNLISTED` | reftable 仓库中存在其列表未列出的 ref 表：是同步客户端带来的另一台设备的表，而那台设备的列表变成了 git 从不读取的副本。下一次 `git pack-refs` 或维护会删除它们，连同那台设备的分支；请先把 git 目录复制到安全的地方 |
| `CHANGE_UNFINISHED` | gitbrella 的某次 `fix`、`undo` 或 `move-out` 没有完成（被终止）；在那里使用 git 之前，请用 `--apply` 运行它指出的命令以完成或回滚 |
| `GIT_DIR_RETURNED` | `move-out` 之后 `.git` 又变回了目录（同步客户端把旧的带了回来），于是存在两个 git 目录；提交前请比较并保留其中一个 |
| `STAGED_CONFLICT_COPY` | index 添加了一个它同时也列出的文件的同步冲突副本（`notes.md` 旁的 `notes 2.md`、复制出的目录 `src 2/`、Syncthing 的命名）；提交会把副本写入历史。只看名字判断；重命名为这种名字的情况，以及名字像副本但没有原件的文件不计入；拆分 index 下不检查 |

信息（不改变退出状态）：`IN_CLOUD_FOLDER`、`GIT_DIR_NOT_SYNCED`（git 目录已被移出同步文件夹：安全布局）、`WORKTREE_COPIES`（你自己文件的冲突副本）、`GIT_RUNNING`、`LOCK_IN_USE`、`LOCK_RECENT`、`IN_PROGRESS_OPERATION`、`UNKNOWN_IN_GITDIR`、`NO_CHECKOUT`、`PARTIAL_CLONE_UNVERIFIED`、`WORKTREE_SCAN_TRUNCATED`、`DEEP_CHECK_SKIPPED`、`REFTABLE_UNSUPPORTED`、`UNREADABLE_IN_GITDIR`、`PROCESS_CHECK_FAILED`、`GIT_DIR_IGNORE_UNCHECKED`、`GIT_TEMP_FILE`、`DATALESS_IN_ALTERNATE`（替代对象存储中有只在云端的文件，因此跳过了对象检查）、`GC_PID_FOREIGN`（另一台机器的 `gc.pid`：git 会在 12 小时内跳过自动 gc）、`GC_LOG_FRESH`（不到一天的 `gc.log` 会阻止自动 gc）、`HOOKS_IN_SYNCED_FOLDER`（`core.hooksPath` 或 `core.fsmonitor` 命令位于 git 目录之外的同步文件夹中，因此另一台设备上的更改会在这里运行）、`WORKTREE_GUESSED`（有 `GIT_DIR` 但没有工作树，且当前目录中没有任何 HEAD 的文件：未检查工作树）。

当路径根本不是仓库时，如果原因在磁盘上，它会说明原因：被重命名的 `.git 2`、git 自己的文件只在云端、没有 HEAD 的 git 目录、指向已不存在的 git 目录的 `.git` 文件，或指向本机没有的文件夹的 `core.worktree`（同步客户端带来的另一台机器的设置）。如果路径中有一个 git 无法使用的 `.git`，它会停在那里（退出码 2），而不是像 git 自己那样为外层仓库作答。

<a name="install"></a>

## 安装

需要 git 2.31 或更新版本。支持 macOS 和 Linux，amd64 和 arm64，不使用 cgo。

```
brew install aliyilmazco/tap/gitbrella
```

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

或者从 releases 页面下载压缩包。

### 验证发布版本

每个发布版本都附带 `checksums.txt`（每个压缩包和 SBOM 的 SHA-256）、由发布工作流生成的无密钥 cosign 签名 `checksums.txt.sigstore.json`、每个压缩包的 SPDX SBOM，以及 GitHub 构建来源证明（build-provenance attestation）。先检查签名，再对照已签名的列表检查压缩包（示例为 Apple 芯片的 v1.2.3）。请使用 cosign v3.1.3 或更新版本（v2 系列为 v2.6.5）：旧版本可能被伪造的包欺骗（GHSA-fx35-mq7g-6g98）。

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

发布版本由 `.github/workflows/release.yml` 根据推送的标签、使用 `.goreleaser.yaml` 构建，并在有人发布之前一直保持草稿状态。二进制文件使用 `-trimpath` 和提交时间构建，因此相同的标签和相同的 Go 版本（工作流使用最新的 1.26 补丁版本）会构建出相同的二进制文件；而包裹它们的压缩包和 SBOM 带有各自的构建时间。

<a name="limits"></a>

## 局限

- `fix` 只在只有一个正确答案时修复，其余的连同要运行的命令留给你；持久的解决办法是 `move-out`，它让 `.git` 留在同步文件夹之外。
- iCloud 本身在两台设备之间产生的冲突副本没有在测试中复现；检测是用遵循相同命名方式的副本测试的。
- Codex 不会把其 shell 工具的 `workdir` 传给钩子：Codex 在另一个文件夹中运行的命令会在会话的文件夹中检查。
- Windows 以及 Windows 上的 OneDrive 未经测试。reftable 仓库获得文件级检查和未列出表的检查；在那里不会把 ref 副本与其分支比较。
- 对于名字可能有意带有的标记的副本名，只在 git 自己创建名字的地方匹配，从不在工作树中匹配：Google Drive 的 `index (1)` 针对 git 自己的文件及 `refs/` 中的文件；OneDrive 的 `index-COMPUTER` 针对 OneDrive 文件夹顶层的 git 自己的文件，要求设备名中有大写字母、不是人们用来结尾备份名的词（`index-old`、`config-OLD`、`config-Backup`、`index-BAK` 属于你），且最多 15 个字符（更长的 Mac 名字会漏掉）；旧版 Nextcloud 的 `_conflict-YYYYMMDD-HHMMSS`、Synology Drive 的命名、pCloud 的 `.conflicted` 和 Resilio 的 `.Conflict` 针对顶层的 git 自己的文件（旧版 Nextcloud 的命名也适用于 Nextcloud 文件夹中 `refs/` 和 `logs/` 里、位于原件旁的文件）。某个 HEAD 所指的分支（以任意空白写成 `ref:`，或作为符号链接）、其 reflog 以及包含它的目录，无论名字如何，都永远不会被移到一旁。因此 `report (1).pdf`、名为 `fix.Conflict` 的分支、名为 `v1.0.conflicted` 的标签或 `config-backup` 都是普通名字。
- 子模块要在子模块内部检查。从父项目出发，`.git/modules/<name>/` 中的冲突副本只有在原件就在旁边时才会被报告，而名字像副本的子模块（`lib 2`）会被报告为副本。
- 链接工作树缺失的 index 会在检查该工作树时报告，而不是在检查主工作树时。
- 当 `.git` 以符号链接方式指向同步文件夹之外的存储时，请从链接的工作树运行 git 和检查。从不包含任何文件的存储目录看，被清空的 index 就像有意的"全部删除"，不会阻止提交。
- 有些有意的提交需要 `--no-verify`：停止跟踪仍留在磁盘上的文件、在不得读取对象的地方删除所有文件、对有其他硬链接的文件只改大小写或只改 Unicode 形式的重命名，以及在稀疏检出（无论是否为 cone 模式）中删除或重命名已跟踪文件、或通过类型改变移动它（文件变成目录、目录变成符号链接或子模块）的任何提交。在那里由 git 的稀疏模式决定哪些文件在磁盘上，因此有意的删除无法与丢失的 index 区分；不会读取这些模式。
- 如果同步客户端连同 index 一起丢失了分支 ref 本身（其松散文件在新的 `packed-refs` 到达之前被删除），HEAD 看起来像一个新分支，提交不会被阻止：它会开始一段只包含已暂存文件的新历史。磁盘上没有任何东西被删除，旧的历史仍在 reflog 中（`git reflog`）。
- 在拆分 index 下，或在不得读取对象的地方（早于 2.45 的 git 上的部分克隆、只在云端的替代存储），不会逐个文件比较 index 与 HEAD：只能发现缺失或为空的 index。
- 启用 `core.splitIndex` 时，不列出任何文件的 index 无法被识别（其头部只统计拆分部分），因此从它进行的提交不会被阻止。
- 钩子：只有提交钩子能通过运行它们的 git 进程找到仓库。其他钩子（`pre-push`、`pre-applypatch`）必须从工作树运行检查，否则会被拒绝（退出码 2）。在看不到 git 进程的地方（自行运行钩子的客户端，或进程列表被阻止），检查会退回到当前目录：此时如果 git 的目录不回指工作树（`--separate-git-dir`、`.git.nosync`、`--work-tree`），不含文件的 index 总会阻止提交（有意清空请使用 `--no-verify`），不包含任何 HEAD 文件的当前目录不会被当作工作树检查（`WORKTREE_GUESSED`），而切换到另一个仓库或 git 目录的钩子会被拒绝（退出码 2）。接收钩子（`pre-receive`）不是受支持的关卡。
- `move-out`：并非所有工具都理解 `.git` 文件。git 和运行 git 的工具可以；isomorphic-git（移动端的 Obsidian Git）以及把 `.git` 当作目录打开的工具不行。
- `move-out`：重命名或移动同步文件夹会破坏相对的 `.git` 文件。此时 `check` 会指出缺失的目录；请把文件夹放回原处，或手动把 `.git` 文件指向 git 目录。
- `move-out` 要求同步文件夹在每台设备上都位于主目录下的相同位置（iCloud Drive 总是如此；Dropbox 在一台设备上可能是 `~/Dropbox`，在另一台上是 `~/Library/CloudStorage/Dropbox`）。不满足时，那里的 `.git` 文件会指向别处，`--attach` 会拒绝（`TARGET_UNEXPECTED`）。
- 移动后的 git 目录位于主目录所在的文件系统上；没有选项把它放到别处。
- `move-out --prepare` 复制的是设备在那一刻的历史：准备后请尽快移动，期间不要有新的提交。
- `move-out` 和 `fix` 是在 iCloud Drive 自己的文件夹中试验的，而不是在某个应用的 iCloud 文件夹中（Obsidian 的 `iCloud~md~obsidian`）。

## 贡献

参见 [CONTRIBUTING.md](../../CONTRIBUTING.md)。请按照 [SECURITY.md](../../SECURITY.md) 中的说明报告安全问题，不要在公开 issue 中报告。

## 许可证

MIT
