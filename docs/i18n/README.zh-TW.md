<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="gitbrella 標誌：一把以 git 分支為傘柄的傘">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  找出雲端同步對 git 儲存庫造成的損壞，<br>
  不修改任何東西，也不下載任何東西。
</p>

<p align="center">
  <a href="https://github.com/aliyilmazco/gitbrella/actions/workflows/ci.yml"><img src="https://github.com/aliyilmazco/gitbrella/actions/workflows/ci.yml/badge.svg" alt="ci"></a>
  <a href="https://github.com/aliyilmazco/gitbrella/releases"><img src="https://img.shields.io/github/v/release/aliyilmazco/gitbrella" alt="release"></a>
  <a href="https://pkg.go.dev/github.com/aliyilmazco/gitbrella"><img src="https://pkg.go.dev/badge/github.com/aliyilmazco/gitbrella.svg" alt="Go Reference"></a>
  <a href="https://app.codecov.io/gh/aliyilmazco/gitbrella"><img src="https://codecov.io/gh/aliyilmazco/gitbrella/graph/badge.svg" alt="coverage"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/github/license/aliyilmazco/gitbrella" alt="license"></a>
</p>

<p align="center">
  <a href="../../README.md">English</a> ·
  <a href="README.zh-CN.md">简体中文</a> ·
  <b>繁體中文</b> ·
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
  <a href="#repairing">修復</a> ·
  <a href="#moving-git-out-of-the-synced-folder">移出 .git</a> ·
  <a href="#what-it-reports">回報內容</a> ·
  <a href="#install">安裝</a> ·
  <a href="#limits">限制</a>
</p>

iCloud Drive、Dropbox、OneDrive、Google Drive 和 Syncthing 是逐一檔案同步的。而 git 預期它的 `.git` 目錄會作為一個整體變化，因此位於同步資料夾中的儲存庫最後會出現 `.git/index 2` 或 `refs/heads/main 2` 這類衝突副本、旁邊殘留著 `index.lock` 的遺失 index，以及只存在於雲端的物件。git 自己的 FAQ 警告不要把儲存庫放在同步資料夾裡，但預設設定總是把儲存庫放到那裡：macOS 的 iCloud 桌面與文件、Windows 的 OneDrive 資料夾備份、`Documents/GitHub`。

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="同步用戶端在裝置之間逐一檔案複製 .git，留下 index 2 和 refs/heads/main 2 之類的衝突副本、遺失的 index 旁邊的 index.lock，以及只存在於雲端的物件">
</p>

`gitbrella check` 通常在一秒內告訴你儲存庫是否有這類損壞。它的設計用途是在 AI 代理或 git 鉤子碰到儲存庫之前執行。`gitbrella fix` 修復只有一種正確修法的損壞，`gitbrella move-out` 則從根本終結損壞：它把 `.git` 移出同步資料夾，並在原處留下一個指向它的 `.git` 檔案，這樣同步用戶端搬運你的檔案，git 則自己保管歷史。

| 指令 | 作用 |
| --- | --- |
| `gitbrella check` | 回報損壞；唯讀 |
| `gitbrella fix` | 修復只有一個正確答案的問題 |
| `gitbrella move-out` | 把 `.git` 永久移出同步資料夾 |
| `gitbrella undo` | 復原最近一次修復或移動 |
| `gitbrella history` | 列出在本裝置上做過的變更 |
| `gitbrella prune-backups` | 刪除已無法復原的變更的舊備份 |
| `gitbrella hook --for claude\|codex` | 守護 AI 代理的工具呼叫 |

## 快速開始

**1. 檢查。** 唯讀，不下載，不連網：

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # 結束碼 0：健康，1：有損壞，2：無法執行
```

**2. 修復**只有一個正確答案的問題。不加 `--apply` 時只顯示計畫：

```sh
gitbrella fix                 # 它會修復什麼，以及留給你處理什麼
gitbrella fix --apply         # 修復；`gitbrella undo --apply` 可以還原
```

**3. 徹底解決。** 把 `.git` 移出同步資料夾：

```sh
gitbrella move-out            # 會發生什麼，以及什麼會阻止它
gitbrella move-out --apply
```

**持續守護。** 在損壞的儲存庫中阻擋提交：

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

AI 代理請參閱[給代理使用](#for-agents)。

## 它的承諾

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` 是唯讀的。** 它從不寫入儲存庫。git 以 `GIT_OPTIONAL_LOCKS=0` 執行，關閉自動 gc 與維護，並且只執行讀取類指令。針對高風險情境（index 副本、遺失的 index、部分複製、分割 index、移動過的 git 目錄）的端對端測試，會在檢查前後記錄每個檔案的大小、mtime 和 inode，並要求兩者一致。
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **不從雲端下載。** 在 macOS 上，它在碰觸磁碟之前就為自身行程（以及它啟動的 git）關閉隨選下載；如果 macOS 拒絕，檢查就不會執行（結束碼 2）。只存在於 iCloud 的檔案只會被回報，不會被下載。
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **不連網。** 部分複製從不抓取缺少的物件（`GIT_NO_LAZY_FETCH`）。在早於 2.45 的 git 上，則改為略過物件檢查。
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **對健康的儲存庫保持安靜。** 只有當一個鎖已經很舊，而且沒有任何正在該儲存庫（任一工作樹）上運作的 git 可能寫下它時，它才算損壞：git 在取得鎖時建立鎖，所以較晚啟動的 git 不可能持有它。git 的暫存檔以及其他工具放在 `.git` 中的檔案只作為資訊。在 89 個健康、正在使用的儲存庫上執行，它沒有回報任何損壞。

<a name="usage"></a>

## 用法

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

結束狀態：**0** 無損壞，**1** 發現損壞，**2** 檢查無法執行。會做出變更的指令（`move-out`、`fix`、`undo`）的結束狀態為：**0** 已規劃、已完成或無事可做（`already`）；**1** 被拒絕（附原因）、完成但有警告，或完成但留有需要你處理的損壞（`fix` 的 MANUAL 清單，`check` 仍會回報它們）；**2** 失敗（已回溯或已停止；JSON 保留已復原的內容與日誌項目）或無法執行。

`--deep` 還會執行完整的 `git fsck`（它會讀取每個物件，因此當物件只存在於雲端時會被略過）。`--provider` 強制指定同步用戶端，而不是依路徑偵測：`auto`（預設）、`icloud`、`icloud-desktop-documents`、`dropbox`、`onedrive`、`google-drive`、`syncthing`、`nextcloud`、`cloud-storage` 或 `none`。Syncthing 與 Nextcloud（或 ownCloud）資料夾無論位於何處，都透過其用戶端在裡面存放的標記（`.stfolder`、`.sync_*.db`）辨識。`fix` 和 `move-out` 接受同樣的 `--provider`：請給它們傳入與 `check` 相同的值，因為某些副本名稱只對特定用戶端有效。此時 JSON 中會顯示 `"providerForced": true`。

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="gitbrella check 的輸出：可由 gitbrella fix 修復的 DAMAGE INDEX_COPY index 2，以及 INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>同樣的輸出（文字）</summary>

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

### 作為 git 鉤子

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit 執行 pre-commit 鉤子，鉤子執行 gitbrella check：結束碼 0 允許提交，1 或 2 阻擋提交">
</p>

損壞的儲存庫會阻擋提交；健康的儲存庫照常提交。這個工具知道是 git 在執行鉤子，不會把這次提交自己的鎖誤認為過期的鎖。

**檢查的是哪個儲存庫。** 對於提交鉤子（git 會傳給它們 `GIT_INDEX_FILE` 或絕對路徑的 `GIT_DIR`），它透過執行鉤子的 git 行程找到儲存庫，因此鉤子可以先切換目錄：切到 git 目錄、其 hooks 目錄，或任何儲存庫之外的目錄。當 git 用絕對路徑的 `GIT_DIR` 指明儲存庫時（連結的工作樹、子模組、獨立的 git 目錄），無論鉤子切到哪裡，鉤子中的每個 git 指令都作用於該儲存庫，檢查也一樣。否則，它會查看 git 自己從目前目錄會找到哪裡（那裡的相對 `GIT_DIR` 所指的 git 目錄，或從那裡找到的儲存庫）：如果那是另一個儲存庫，或同一儲存庫的另一個工作樹（切換進去的鉤子，或提交訊息編輯器），它會拒絕（結束碼 2），而不是替錯誤的儲存庫作答。

不帶路徑執行時，它檢查 git 匯出給鉤子的儲存庫（`GIT_DIR`），因此 `--git-dir` 形式的 dotfiles 設定，以及位於自身工作樹內的 git 目錄同樣可用。不帶路徑時，它也會遵循你在 shell 中設定的 `GIT_DIR`；與 git 不同，它會忽略不存在的 `GIT_DIR`，並拒絕無法追溯的連結工作樹，而不是把目前目錄當作工作樹。

**提交所記錄的 index。** 當 `GIT_INDEX_FILE` 指向 git 目錄內的檔案時（git 為提交鉤子就是這樣設定的），它會讀取該檔案：如果提交所記錄的 index 遺失或為空，或者不再列出仍在工作樹中的某個 HEAD 檔案，那麼在提交把這些檔案記錄為刪除之前就會被阻擋。你自己把 `GIT_INDEX_FILE` 指向別處的 index 檔案不會被讀取：在鉤子中改為檢查儲存庫自己的 index，而在設定了該變數時手動執行的檢查會拒絕（結束碼 2）。

**有意的刪除。** 要停止追蹤仍保留在磁碟上的檔案（`git rm --cached`，取消追蹤一個你現在要忽略的目錄），或在檔案仍在磁碟上時有意提交「全部刪除」（`git rm -r --cached .`），請用 `git commit --no-verify` 略過鉤子。

### 搭配鉤子管理工具

- **pre-commit**（pre-commit.com）：以鉤子 id `gitbrella`（pre-commit 用 Go 建置它）或 `gitbrella-system`（使用 PATH 中的執行檔）加入本儲存庫；參見 `.pre-commit-hooks.yaml`。
- **lefthook**：透過 `remotes` 引入 `hooks/lefthook.yml`。
- **husky**：內容為 `gitbrella check` 的 `.husky/pre-commit`。
- **Overcommit**：`PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`，然後執行 `overcommit --sign`。

在這些工具中，任何非零結束都會阻擋提交，因此無法執行的檢查（結束碼 2）也會阻擋提交。除非設定了 `git-commit-verify: true`，Aider 會用 `--no-verify` 提交，此時根本不會執行任何提交鉤子。Obsidian Git（桌面版）使用系統的 git 提交並執行鉤子；請把 `gitbrella` 所在的資料夾加入它的「Additional PATH」設定，因為從 Dock 啟動的 App 拿不到 shell 的 PATH。行動版的 Obsidian Git（isomorphic-git）不執行任何鉤子。

<a name="for-agents"></a>

### 給代理使用

`--json` 以穩定、帶版本的文件形式輸出同一份報告（`schemaVersion: 1`，`findings` 永遠是清單）。依結束狀態做決定，並把發現項目顯示給使用者。

代理自己的鉤子只在結束碼 2 時阻擋工具呼叫：結束碼 1（「發現損壞」）會讓呼叫繼續。`gitbrella hook --for claude|codex` 能對接它們的鉤子。它讀取代理透過 stdin 傳入的事件：

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="代理的工具呼叫經過 gitbrella hook：讀取與編輯照常進行；在損壞的儲存庫中，git 寫入操作和寫入 .git 以結束碼 2 被阻擋">
</p>

- **工具呼叫之前**（`PreToolUse`）：在損壞的儲存庫中，會寫入的 git 指令（commit、add、reset、rebase、push，以及任何無法確定為唯讀的指令）和寫入 `.git` 的操作會阻擋該呼叫（結束碼 2，原因寫在 stderr 供代理查看）。讀取、編輯你的檔案、`git restore` 以及 `gitbrella` 本身照常進行，以便檢視並修復損壞。
  - *無法使用的儲存庫。* 當 git 的檔案只存在於雲端，或 git 完全無法使用該儲存庫時（被重新命名的 `HEAD`、指向不存在位置的 `.git` 檔案），每個 git 指令都會被阻擋：它會卡住，或作用於外層儲存庫。此時寫入 `.git` 的操作照常進行：那種修復要依原因說明在 `.git` 內手動完成（`mv '.git/HEAD 2' .git/HEAD`）。`core.worktree` 提示中給出的補救措施（`git config --file … --unset core.worktree`）也照常進行。
  - *哪個儲存庫。* 包含該呼叫所處理檔案的儲存庫（Codex 修補會寫明自己的檔案；`.git` 內的路徑算作其上方的工作樹）。對於 shell 指令，是工作階段的資料夾以及其中 git 指令執行的每個資料夾（`cd DIR`、`pushd`、`git -C DIR`、`--git-dir`、`--work-tree`、`GIT_DIR`、`env -C`、`bash -c '…'`），寫入的優先，最多五個；當某個寫入的放不下時，呼叫會帶著提示放行。每個資料夾依該呼叫在那裡做的事判斷：從損壞的工作階段資料夾執行 `git -C ../sound commit` 會照常進行。不是儲存庫的資料夾和 bare 儲存庫會靜默通過。
  - *當檢查無法執行時。* 任何使檢查無法執行的情況（包括 20 秒的 `--timeout`，它是整個呼叫唯一的期限）都會讓呼叫帶著提示放行；`--fail-open` 讓 git 無法使用儲存庫的情況也這樣處理。
  - *如何解讀指令。* 只讀取，從不執行：引號、註解、子 shell、替換（也包括未加引號的 heredoc 中的替換）、heredoc、它設定的變數、`&&`、`||`、`if`、`case` 與迴圈（在 `cd a || cd b` 之後，或 `cd` 到不存在的資料夾之後，shell 可能所在的每個資料夾都計入；`for` 迴圈依每個值各讀取一次）、包裝指令（`timeout`、`sudo`、`nice`、`xargs`、`env`），以及交給 shell 的指令稿（`-c`、`eval`、`watch`、`source`、檔案、heredoc、`bash < f`、以管線傳給 `sh`）。指令中定義的函式與別名在被呼叫處讀取。以路徑執行或交給 shell 的 shell 指令稿（`./deploy.sh`、`bash deploy.sh`），若是本機上的小檔案就會被讀取（經由符號連結也可以；即使它執行自身也只讀一次）。`make`、`npm run` 之類不會被讀取。
  - *什麼算寫入。* 其他程式寫入 `.git`（`rm .git/index.lock`、`> .git/HEAD`、`cd .git && rm index.lock`、對 `.git/config` 執行 `sed -i`、對包含 `.git` 的資料夾執行 `rm -rf` 或 `find -delete`、透過 shell 套用的 Codex 修補）算作寫入 `.git`；把 `.git` 複製到別處、用 `tar` 打包或讀取它都不算。`gh pr checkout` 與 `git-lfs` 算作會寫入的 git 指令。
  - *拿不準時。* 以下每一種都算作會寫入的 git 指令：讀取超過 5000 步或超過 `--timeout` 的一半（在它無法判斷的資料夾中）；出現在讀取無法追蹤之處的 `git` 一詞；從讀取無法追蹤的程式以管線傳給 shell 的指令稿（`curl … | sh`），在它執行的地方；交給 shell、存在但無法讀取的指令稿（太大、只在雲端），在它執行的地方。當無法判斷某個 git 指令寫入哪個資料夾時（`cd "$(…)"`、`for d in */`），該指令在每個被檢查的資料夾中都計入，呼叫會收到提示。
- **工作階段開始時**（`SessionStart`）：在 stdout 輸出一行，代理會把它當作脈絡讀取，說明儲存庫位於同步資料夾中（除非其 git 目錄已經在外面）以及哪些地方損壞了。它從不阻擋。

該鉤子不修改任何東西。Claude Code，寫在 `.claude/settings.json` 中：

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex，寫在 `.codex/hooks.json` 中（只有在你於 `/hooks` 中信任某個鉤子之後，Codex 才會執行它）：

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

檢查對上述每個資料夾各執行一次，通常每次 0.1 到 0.4 秒，因此分布在同一個儲存庫多個資料夾中的修補會多次檢查該儲存庫；`--timeout` 限制整個呼叫的時間。

<a name="repairing"></a>

## 修復

```sh
gitbrella fix                 # 它會修復什麼，以及留給你處理什麼
gitbrella fix --apply         # 修復
gitbrella undo --apply        # 全部還原
gitbrella history             # 在此做過的變更、它們的備份與大小
gitbrella prune-backups       # 哪些備份已不再被任何 undo 需要
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix 顯示計畫；fix --apply 記錄每一步，然後再次檢查：完成，或回溯；undo --apply 復原最近一次變更">
</p>

`check` 會在每項損壞下註明 `fix` 是否能修復它（`repair:`，在 JSON 中為 `"repair": "fix"` 或 `"manual"`）；如果檢查時指定了 provider，還會寫出 `fix --provider NAME`。在提交鉤子內部它不會詢問。

`fix` 只在只有一個正確答案、且不會覆寫任何你可能有意保留的內容時才修復：

| 損壞 | 修復 |
| --- | --- |
| `.git` 中位於原件旁邊的衝突副本（`HEAD 2`、`refs/heads/main 2`、`refs 2`、`packed-refs 2`、Dropbox 與 Syncthing 的副本，以及 ref 鎖的副本，如 `refs/heads/main 2.lock`）；index 副本僅在它與 index 完全相同，或是與 HEAD 相等的有效 index 時 | 移入備份；只有該副本指向的物件（commit、tag、tree 或檔案）會先保存為 `refs/gcd-rescue/<name>` |
| 殘留的 rebase 目錄、`desktop.ini`、`Icon` 以及其他非 git 建立的類似檔案（除非 git 把它讀作分支，或它的名稱是某個物件的名稱）、工作樹中其提交儲存庫都已擁有的 `.git` 副本 | 移入備份 |
| index 遺失，且任何地方都沒有它的副本或鎖 | 從 HEAD 產生（`git read-tree HEAD`：不碰觸檔案）；僅在有可用來建置的提交（若 HEAD 的提交也遺失，會先依下文方式重新抓取）且不在稀疏簽出中時 |
| 不列出任何檔案的 index（稀疏簽出之外的有效、非分割 index） | 移入備份並從 HEAD 產生：其中沒有暫存任何內容 |
| 作為與 HEAD 相等之有效 index 的孤立 `index.lock` | 它成為 index |
| 空的分支檔案 | 其 reflog 最後記錄的提交 |
| git 會加入提交的 `.git.nosync` | 在 `.git/info/exclude` 中加入 `/.git.nosync/` 這一行 |
| HEAD 的提交遺失，或空的鬆散物件檔案（旁邊沒有副本） | 空檔案移入備份，並重新抓取物件（`git fetch --refetch`，需要 git 2.36 或更新版本）：優先 `--remote`，否則分支的遠端，否則 `origin`，否則唯一的遠端；遠端的分支與標籤會抓取到暫時的 `refs/gcd-fetch/` 命名空間，之後刪除，因此沒有任何 ref 改變。如果遠端沒能帶回該物件，修復會回溯 |
| 使用 `fix --deep`：`git fsck` 回報損壞的物件 | 以同樣方式重新抓取遠端分支與標籤的每個物件；之後再次執行 `git fsck`，若仍回報錯誤（沒有任何遠端擁有的物件、仍殘留的損壞檔案），修復會回溯 |
| 你自己檔案的衝突副本（`notes 2.md`），git 未追蹤且與其所複製的檔案逐位元組相同 | 移入備份（`git add -A` 會提交它們）；內容不同、被追蹤、是連結、只在雲端或位於子模組中的副本會保留 |
| 其他機器上超過一小時的 `gc.pid`；`gc.log` | 移入備份，讓 git 的自動 gc 在此處重新執行 |
| `.git` 或替代物件儲存區中只存在於雲端的檔案 | 下載，僅此而已：在此之前檢查無法讀取物件，因此其餘損壞會列為 MANUAL，留待下次執行 `fix` |

其餘一切都列為 MANUAL，並附上要執行的指令。其中包括任何可能保存著別處沒有之工作的東西：
- 損壞的 index（git 可能仍能讀取它，連同已暫存的工作）；
- 與 HEAD 不同的 index 副本或 `index.lock`，以及 `index.lock` 的副本；
- 原件遺失的副本（它可能是唯一的一份：請放回原處；若它是鎖的副本，則會被保護並移到一旁）；
- 本機上沒有任何 git 持有的鎖（其他裝置或容器中的 git 可能持有；建議中會寫出其中的提交）；
- 指向超過 100 個物件的副本（很長的 reflog）；
- 某個 HEAD 所指分支的副本、其 reflog 的副本，或包含該分支之目錄的副本（無論名稱看起來像什麼，git 都會讀取它）。

還包括 fix 必須在兩個版本之間做選擇，或無法保住副本中提交的任何情況：
- 物件儲存區中的一切：副本、以物件命名的檔案（請檢查哪個與其雜湊相符）、沒有遠端可重新抓取的空物件檔案；
- 子模組 git 目錄中的一切（請在子模組中執行 `fix`）；
- 指向尚未抵達此處之提交的分支副本（它可能還會到來）；
- 工作樹中含有儲存庫缺少之提交的 `.git` 副本（哪一個是最新的由你決定）；
- 旁邊有它自身或其目錄副本的空檔案；
- 損壞的連結工作樹管理目錄（`git worktree repair`）；
- 遺失了仍在磁碟上之檔案的 index（有意取消追蹤看起來也一樣；當沒有其他暫存內容時，建議會說明 `git read-tree HEAD` 能把它們找回且不會遺失任何東西）；
- 未使用 `--deep` 或沒有遠端時，`git fsck` 回報損壞的物件。

`fix` 完全不處理 reftable 儲存庫；在那裡，`check` 的 `REFTABLE_TABLE_UNLISTED` 會說明如何保住另一台裝置的 ref。備份必須位於儲存庫所在的磁碟上，這樣 fix 移到一旁的東西才能一步移動完成；否則 `fix` 會拒絕（只做下載的修復不移動任何東西，也不需要這點）。

每一步在執行之前都會被記錄，最後會再次執行檢查：如果它修復過的東西仍被回報，或出現了新的損壞，這些步驟會被復原。被中斷的 fix 會由該儲存庫中下一個帶 `--apply` 的變更指令回溯，該指令隨後停止；請再執行一次。30 秒後（`--watch`），`fix` 會再檢查一次：重新出現的損壞是同步用戶端帶回來的，長久的解決方法是 `move-out`。

`undo` 先復原最近的變更，也就是一次修復或移動。某一步的結果若在修復之後發生了變化，就保持原樣並列出，絕不覆寫：git 重寫了重建的 index、同步用戶端帶回了副本、救援 ref 移動了、備份被刪除，或 `.git` 被移動。只要其保住的副本還在備份中，救援 ref 就會保留。已下載的檔案留在磁碟上，重新抓取的物件留在物件儲存區中：兩者都不取代任何東西。其餘部分會被復原，undo 完成，這樣先前的那次變更就可以接著復原。

`history` 列出本裝置對該儲存庫做過的變更（修復、移動、復原）、每項保留的備份及其大小，以及 `undo` 下一次會復原哪一項。`prune-backups` 僅在加上 `--apply` 時，刪除那些已復原、已回溯或本來就不可復原（undo、prepare、attach）且最後一步早於 `--older-than`（預設 `720h`）之變更的備份。仍可復原的修復或移動會保留其備份，即使其後還有較新的變更。

<a name="moving-git-out-of-the-synced-folder"></a>

## 把 .git 移出同步資料夾

```sh
gitbrella move-out            # 會發生什麼，以及什麼會阻止它
gitbrella move-out --apply    # 執行
gitbrella undo --apply        # 移回去
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="之前：.git 在同步資料夾內。move-out 之後：同步資料夾中的 .git 檔案指向資料夾外的 ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d">
</p>

`move-out` 把 `.git` 複製到 `~/.local/share/gitbrella/gitdirs/<name>`（在 macOS 上是 APFS 複製：不占用空間），逐一檔案證明副本完全相同，把原件移入 `~/Library/Application Support/gitbrella`（Linux：`$XDG_STATE_HOME/gitbrella`）下的備份，並寫入一個 `.git` 檔案，例如 `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`。該路徑是相對的，因此每台把該資料夾放在家目錄下相同位置的裝置，都會寫出相同的 `.git` 檔案。接著它檢查 git 是否找到相同的 HEAD、狀態、設定和 hooks 目錄，並在 30 秒內（`--watch`）確認同步用戶端沒有動到 `.git`，且在 iCloud 下上傳了新檔案。原件在最後一次比對之前被移到一旁，因此複製期間混入的寫入會被發現，移動會被取消。在 iCloud 下，當 iCloud 仍在上傳該儲存庫時，`move-out` 會拒絕；`--wait-upload 10m` 則改為最多等待這麼久。

每項變更在發生之前都會記錄到日誌，連同它建立的每樣東西的裝置號碼與 inode（以及 `.git` 檔案的位元組）；回溯不會碰觸其他任何東西。如果行程中途中斷，下一次 `move-out --apply`（或 `undo --apply`）會完成或回溯它並停在那裡（再次執行指令以繼續）；即使路徑寫法不同、被重新命名過，或位於另一個儲存庫內，也能透過資料夾的 inode 找到它。在此期間，`check` 會把未完成的變更回報為損壞。一旦 git 使用過移動後的目錄，就不會再在其上回溯：如果它包含原件沒有的內容，指令會停止並指出兩者。不會刪除任何東西：原件保留在備份中，直到你刪除它。`undo` 會帶回目前正在使用的 git 目錄，連同移動之後做出的提交；移動之前的備份保持不動。如果同步用戶端已經移出了 `.git` 檔案的本機內容（在這台 Mac 上只剩下它的名稱），`undo` 與復原作業會要求你先下載它：讀取它就會觸發下載，而這個工具從不這樣做。

如果回溯同時發現了原件（在備份中）和一個新的 `.git`（在移動停止後由你或同步用戶端建立），它不會覆寫任何一方，並會告訴你：比對兩者，把你不要的 `.git` 移出資料夾，然後再次執行指令。

在以下情況它會拒絕（結束碼 1，附原因）：git 或持有 git 鎖的東西正在操作該儲存庫時、合併或 rebase 期間、`.git` 的檔案只存在於雲端時、iCloud 仍在上傳該資料夾時、`.git` 中有通訊端（正在執行的 `git fsmonitor--daemon`）時、移動後存在兩個 git 目錄時，以及連結會斷掉的配置：連結的工作樹、子模組、借用的物件（`alternates`）、本身是符號連結或並非 git 目錄本身的 `.git`。`check` 發現的其他損壞會原樣移動並列為「隨附」：之後請再次執行 `check`。

### 其他裝置

同步用戶端會在同步該資料夾的其他每台裝置上刪除 `.git`，並帶去 `.git` 檔案。要保住它們的歷史：

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1：在其他每台裝置上執行 move-out --prepare --apply。2：在一台裝置上執行 move-out --apply。3：在其他每台裝置上執行 move-out --attach --apply">
</p>

1. 在移動之前，於其他每台裝置上：`gitbrella move-out --prepare --apply` 把該裝置的 `.git` 複製到 `.git` 檔案將要指向的位置。
2. 在一台裝置上：`gitbrella move-out --apply`。
3. 在 `.git` 檔案抵達之後，於其他每台裝置上：`gitbrella move-out --attach --apply`。未做準備的裝置則改為複製歷史：`--attach --from <remote URL> --apply`。無論哪種方式，工作樹都不會被碰觸。

`.git` 檔案是透過同步用戶端抵達的，因此 attach 只在本裝置記錄允許的範圍內信任它：它必須指向本裝置的 `~/.local/share/gitbrella/gitdirs` 之內，而且那裡已存在的目錄只有在 `--prepare` 為這個儲存庫建立了它時才會被使用。

從此以後，同步用戶端只搬運檔案而不搬運歷史。請透過遠端分享提交。在檔案已由同步用戶端更新過的裝置上，`git pull` 會拒絕（「would be overwritten」）：請使用 `git fetch` 和 `git reset --mixed @{u}`，它會移動分支與 index，並保持已同步的檔案不變。

<a name="what-it-reports"></a>

## 回報內容

損壞（結束碼 1）：

| 代碼 | 意義 |
| --- | --- |
| `INDEX_COPY` | index 的衝突副本（`index 2`；Dropbox、Syncthing、Nextcloud/ownCloud、Proton Drive 與 Seafile 的命名；Google Drive 的 `index (1)`、OneDrive 資料夾中 OneDrive 的 `index-COMPUTER`，以及舊版 Nextcloud、Synology Drive、pCloud 與 Resilio 的命名，這些只針對 git 自己的檔名比對）；說明它是否為有效 index 以及是否與 HEAD 一致 |
| `REF_COPY` | 分支或標籤 ref 的衝突副本；說明該副本是較舊、較新、已分歧，還是指向遺失的提交 |
| `COPY_IN_GITDIR` | `.git` 中任何其他衝突副本 |
| `INDEX_MISSING` | index 不見了；git 會把每個檔案都顯示為已刪除（在提交鉤子中：這次提交會刪除每個檔案） |
| `INDEX_EMPTY` | HEAD 的檔案仍在工作樹中，但 index 不列出任何檔案，就像同步用戶端刪除 index 後 git 重建了它一樣；提交會刪除每個檔案 |
| `INDEX_SHRUNK` | index 不再列出仍在工作樹中的 HEAD 檔案（在稀疏簽出中：任何 HEAD 檔案），就像同步用戶端刪除 index 後 git 用已暫存的檔案重建了它一樣；提交會刪除它們。重新命名、變成符號連結或變成 `.gitmodules` 宣告之子模組的目錄，以及也已從磁碟刪除的檔案不計入；當無法列出 HEAD 的檔案時，回報為未驗證 |
| `HEAD_COMMIT_MISSING` | HEAD 指向儲存庫中不存在的提交（ref 比其物件先抵達） |
| `ORPHAN_INDEX_LOCK` | index 不見了，留下一個舊的 `index.lock`（往往是完整的 index） |
| `INDEX_INVALID` | index 未通過自身的校驗和 |
| `STALE_LOCK` | 沒有任何執行中的 git 能持有的 git 鎖：沒有 git 在操作該儲存庫，或所有 git 都是在鎖寫入之後才啟動。只看得到本機上的 git 行程：刪除鎖之前，請確認沒有 git 在容器中、在共用該資料夾的另一台機器上，或在內嵌 git 的 App 中使用該儲存庫 |
| `DATALESS_IN_GITDIR` | `.git` 中的檔案只存在於雲端；已略過物件檢查 |
| `FOREIGN_FILE_IN_GITDIR` | `refs/` 或 `objects/` 中非 git 建立的檔案（`desktop.ini`、`objects/` 中的 `.tmp` 檔案） |
| `EMPTY_FILE_IN_GITDIR` | 零位元組的物件或 ref |
| `GITDIR_COPY` | 工作樹中的 git 目錄副本（`.git 2`） |
| `GIT_DIR_NOT_IGNORED` | git 目錄以另一個名稱位於工作樹內（`.git.nosync`），`git add -A` 會提交它 |
| `WORKTREE_ADMIN_BROKEN` | 連結的工作樹遺失了其管理檔案 |
| `STALE_OPERATION_STATE` | 殘留的 rebase 目錄讓 git 以為 rebase 正在進行 |
| `FSCK_ERRORS` | 僅 `--deep`：`git fsck` 回報了錯誤（未執行完成時，同一代碼改為資訊） |
| `REFTABLE_TABLE_UNLISTED` | reftable 儲存庫中存在其清單未列出的 ref 表：是同步用戶端帶來的另一台裝置的表，而那台裝置的清單變成了 git 從不讀取的副本。下一次 `git pack-refs` 或維護會刪除它們，連同那台裝置的分支；請先把 git 目錄複製到安全的地方 |
| `CHANGE_UNFINISHED` | gitbrella 的某次 `fix`、`undo` 或 `move-out` 沒有完成（被中斷）；在那裡使用 git 之前，請用 `--apply` 執行它指出的指令以完成或回溯 |
| `GIT_DIR_RETURNED` | `move-out` 之後 `.git` 又變回了目錄（同步用戶端把舊的帶了回來），於是存在兩個 git 目錄；提交前請比對並保留其中一個 |
| `STAGED_CONFLICT_COPY` | index 加入了一個它同時也列出之檔案的同步衝突副本（`notes.md` 旁的 `notes 2.md`、複製出的目錄 `src 2/`、Syncthing 的命名）；提交會把副本寫入歷史。只看名稱判斷；重新命名為這種名稱的情況，以及名稱像副本但沒有原件的檔案不計入；分割 index 下不檢查 |

資訊（不改變結束狀態）：`IN_CLOUD_FOLDER`、`GIT_DIR_NOT_SYNCED`（git 目錄已移出同步資料夾：安全配置）、`WORKTREE_COPIES`（你自己檔案的衝突副本）、`GIT_RUNNING`、`LOCK_IN_USE`、`LOCK_RECENT`、`IN_PROGRESS_OPERATION`、`UNKNOWN_IN_GITDIR`、`NO_CHECKOUT`、`PARTIAL_CLONE_UNVERIFIED`、`WORKTREE_SCAN_TRUNCATED`、`DEEP_CHECK_SKIPPED`、`REFTABLE_UNSUPPORTED`、`UNREADABLE_IN_GITDIR`、`PROCESS_CHECK_FAILED`、`GIT_DIR_IGNORE_UNCHECKED`、`GIT_TEMP_FILE`、`DATALESS_IN_ALTERNATE`（替代物件儲存區中有只在雲端的檔案，因此略過了物件檢查）、`GC_PID_FOREIGN`（另一台機器的 `gc.pid`：git 會在 12 小時內略過自動 gc）、`GC_LOG_FRESH`（不到一天的 `gc.log` 會阻止自動 gc）、`HOOKS_IN_SYNCED_FOLDER`（`core.hooksPath` 或 `core.fsmonitor` 指令位於 git 目錄之外的同步資料夾中，因此另一台裝置上的變更會在這裡執行）、`WORKTREE_GUESSED`（有 `GIT_DIR` 但沒有工作樹，且目前目錄中沒有任何 HEAD 的檔案：未檢查工作樹）。

當路徑根本不是儲存庫時，若原因在磁碟上，它會說明原因：被重新命名的 `.git 2`、git 自己的檔案只在雲端、沒有 HEAD 的 git 目錄、指向已不存在之 git 目錄的 `.git` 檔案，或指向本機沒有的資料夾的 `core.worktree`（同步用戶端帶來的另一台機器的設定）。如果路徑中有一個 git 無法使用的 `.git`，它會停在那裡（結束碼 2），而不是像 git 自己那樣替外層儲存庫作答。

<a name="install"></a>

## 安裝

需要 git 2.31 或更新版本。支援 macOS 與 Linux，amd64 與 arm64，不使用 cgo。

```
brew install aliyilmazco/tap/gitbrella
```

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

或從 releases 頁面下載壓縮檔。

### 驗證發行版本

每個發行版本都附帶 `checksums.txt`（每個壓縮檔與 SBOM 的 SHA-256）、由發行工作流程產生的無金鑰 cosign 簽章 `checksums.txt.sigstore.json`、每個壓縮檔的 SPDX SBOM，以及 GitHub 建置來源證明（build-provenance attestation）。先驗證簽章，再對照已簽署的清單驗證壓縮檔（範例為 Apple 晶片的 v1.2.3）。請使用 cosign v3.1.3 或更新版本（v2 系列為 v2.6.5）：舊版本可能被偽造的套件欺騙（GHSA-fx35-mq7g-6g98）。

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

發行版本由 `.github/workflows/release.yml` 依推送的標籤、使用 `.goreleaser.yaml` 建置，並在有人發布之前一直維持草稿狀態。執行檔以 `-trimpath` 和提交時間建置，因此相同的標籤與相同的 Go 版本（工作流程使用最新的 1.26 修補版本）會建置出相同的執行檔；而包裹它們的壓縮檔與 SBOM 帶有各自的建置時間。

<a name="limits"></a>

## 限制

- `fix` 只在只有一個正確答案時修復，其餘的連同要執行的指令留給你；長久的解決方法是 `move-out`，它讓 `.git` 留在同步資料夾之外。
- iCloud 本身在兩台裝置之間產生的衝突副本沒有在測試中重現；偵測是用遵循相同命名方式的副本測試的。
- Codex 不會把其 shell 工具的 `workdir` 傳給鉤子：Codex 在另一個資料夾中執行的指令，會在工作階段的資料夾中檢查。
- Windows 以及 Windows 上的 OneDrive 未經測試。reftable 儲存庫會進行檔案層級的檢查與未列出表的檢查；在那裡不會把 ref 副本與其分支比對。
- 對於名稱可能有意帶有之標記的副本名稱，只在 git 自己建立名稱的地方比對，從不在工作樹中比對：Google Drive 的 `index (1)` 針對 git 自己的檔案及 `refs/` 中的檔案；OneDrive 的 `index-COMPUTER` 針對 OneDrive 資料夾最上層的 git 自己的檔案，要求裝置名稱含有大寫字母、不是人們用來結尾備份名稱的字（`index-old`、`config-OLD`、`config-Backup`、`index-BAK` 屬於你），且最多 15 個字元（更長的 Mac 名稱會漏掉）；舊版 Nextcloud 的 `_conflict-YYYYMMDD-HHMMSS`、Synology Drive 的命名、pCloud 的 `.conflicted` 和 Resilio 的 `.Conflict` 針對最上層的 git 自己的檔案（舊版 Nextcloud 的命名也適用於 Nextcloud 資料夾中 `refs/` 和 `logs/` 裡、位於原件旁的檔案）。某個 HEAD 所指的分支（以任意空白寫成 `ref:`，或作為符號連結）、其 reflog 以及包含它的目錄，無論名稱如何，都永遠不會被移到一旁。因此 `report (1).pdf`、名為 `fix.Conflict` 的分支、名為 `v1.0.conflicted` 的標籤或 `config-backup` 都是一般名稱。
- 子模組要在子模組內部檢查。從父專案出發，`.git/modules/<name>/` 中的衝突副本只有在原件就在旁邊時才會被回報，而名稱像副本的子模組（`lib 2`）會被回報為副本。
- 連結工作樹遺失的 index 會在檢查該工作樹時回報，而不是在檢查主要工作樹時。
- 當 `.git` 以符號連結指向同步資料夾之外的儲存位置時，請從連結的工作樹執行 git 與檢查。從不包含任何檔案的儲存目錄來看，被清空的 index 就像有意的「全部刪除」，不會阻擋提交。
- 有些有意的提交需要 `--no-verify`：停止追蹤仍留在磁碟上的檔案、在不得讀取物件的地方刪除所有檔案、對有其他硬連結的檔案只改大小寫或只改 Unicode 形式的重新命名，以及在稀疏簽出（無論是否為 cone 模式）中刪除或重新命名已追蹤檔案、或透過類型改變移動它（檔案變成目錄、目錄變成符號連結或子模組）的任何提交。在那裡由 git 的稀疏模式決定哪些檔案在磁碟上，因此有意的刪除無法與遺失的 index 區分；不會讀取這些模式。
- 如果同步用戶端連同 index 一起遺失了分支 ref 本身（其鬆散檔案在新的 `packed-refs` 抵達之前被刪除），HEAD 看起來像一個新分支，提交不會被阻擋：它會開始一段只包含已暫存檔案的新歷史。磁碟上沒有任何東西被刪除，舊的歷史仍在 reflog 中（`git reflog`）。
- 在分割 index 下，或在不得讀取物件的地方（早於 2.45 的 git 上的部分複製、只在雲端的替代儲存區），不會逐一檔案比對 index 與 HEAD：只能發現遺失或為空的 index。
- 啟用 `core.splitIndex` 時，不列出任何檔案的 index 無法被辨識（其標頭只計算分割部分），因此從它進行的提交不會被阻擋。
- 鉤子：只有提交鉤子能透過執行它們的 git 行程找到儲存庫。其他鉤子（`pre-push`、`pre-applypatch`）必須從工作樹執行檢查，否則會被拒絕（結束碼 2）。在看不到 git 行程的地方（自行執行鉤子的用戶端，或行程清單被封鎖），檢查會退回使用目前目錄：此時如果 git 的目錄不回指工作樹（`--separate-git-dir`、`.git.nosync`、`--work-tree`），不含檔案的 index 總會阻擋提交（有意清空請使用 `--no-verify`），不包含任何 HEAD 檔案的目前目錄不會被當作工作樹檢查（`WORKTREE_GUESSED`），而切換到另一個儲存庫或 git 目錄的鉤子會被拒絕（結束碼 2）。接收鉤子（`pre-receive`）不是受支援的防線。
- `move-out`：並非所有工具都理解 `.git` 檔案。git 和執行 git 的工具可以；isomorphic-git（行動版的 Obsidian Git）以及把 `.git` 當作目錄開啟的工具不行。
- `move-out`：重新命名或移動同步資料夾會破壞相對的 `.git` 檔案。此時 `check` 會指出遺失的目錄；請把資料夾放回原處，或手動把 `.git` 檔案指向 git 目錄。
- `move-out` 要求同步資料夾在每台裝置上都位於家目錄下的相同位置（iCloud Drive 一定如此；Dropbox 在一台裝置上可能是 `~/Dropbox`，在另一台上則是 `~/Library/CloudStorage/Dropbox`）。不符合時，那裡的 `.git` 檔案會指向別處，`--attach` 會拒絕（`TARGET_UNEXPECTED`）。
- 移動後的 git 目錄位於家目錄所在的檔案系統上；沒有選項能把它放到別處。
- `move-out --prepare` 複製的是裝置在那一刻的歷史：準備後請盡快移動，期間不要有新的提交。
- `move-out` 與 `fix` 是在 iCloud Drive 自己的資料夾中試驗的，而不是在某個 App 的 iCloud 資料夾中（Obsidian 的 `iCloud~md~obsidian`）。

## 貢獻

參見 [CONTRIBUTING.md](../../CONTRIBUTING.md)。請依照 [SECURITY.md](../../SECURITY.md) 中的說明回報安全問題，不要在公開 issue 中回報。

## 授權條款

MIT
