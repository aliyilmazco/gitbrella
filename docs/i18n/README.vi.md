<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="Logo gitbrella: một chiếc ô có cán là một nhánh git">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Tìm những hư hỏng mà đồng bộ đám mây gây ra cho kho git.<br>
  Không thay đổi gì. Không tải xuống gì.
</p>

<p align="center">
  <a href="https://github.com/aliyilmazco/gitbrella/actions/workflows/ci.yml"><img src="https://github.com/aliyilmazco/gitbrella/actions/workflows/ci.yml/badge.svg" alt="ci"></a>
  <a href="https://github.com/aliyilmazco/gitbrella/releases"><img src="https://img.shields.io/github/v/release/aliyilmazco/gitbrella" alt="release"></a>
  <a href="https://pkg.go.dev/github.com/aliyilmazco/gitbrella"><img src="https://pkg.go.dev/badge/github.com/aliyilmazco/gitbrella.svg" alt="Go Reference"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/github/license/aliyilmazco/gitbrella" alt="license"></a>
</p>

<p align="center">
  <a href="../../README.md">English</a> ·
  <a href="README.zh-CN.md">简体中文</a> ·
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
  <b>Tiếng Việt</b>
</p>

<p align="center">
  <a href="#usage">Cách dùng</a> ·
  <a href="#repairing">Sửa chữa</a> ·
  <a href="#moving-git-out-of-the-synced-folder">Chuyển .git ra ngoài</a> ·
  <a href="#what-it-reports">Những gì được báo cáo</a> ·
  <a href="#install">Cài đặt</a> ·
  <a href="#limits">Giới hạn</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive và Syncthing đồng bộ từng tệp một. Git lại cho rằng thư mục `.git` thay đổi như một khối thống nhất, nên kho nằm trong thư mục được đồng bộ sẽ xuất hiện các bản sao xung đột như `.git/index 2` và `refs/heads/main 2`, index bị mất bên cạnh một `index.lock` bị bỏ lại, và các đối tượng chỉ có trên đám mây. Chính FAQ của git khuyên không nên đặt kho trong thư mục được đồng bộ, nhưng các thiết lập mặc định vẫn đặt chúng ở đó: Desktop và Documents trên iCloud ở macOS, sao lưu thư mục OneDrive trên Windows, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Trình đồng bộ sao chép .git giữa các thiết bị từng tệp một và để lại các bản sao xung đột như index 2 và refs/heads/main 2, một index.lock bên cạnh index bị mất, và các đối tượng chỉ có trên đám mây">
</p>

`gitbrella check` cho biết một kho có những hư hỏng này hay không, thường trong chưa đầy một giây. Nó được tạo ra để chạy trước khi một tác nhân AI hoặc git hook chạm vào kho. `gitbrella fix` sửa những gì có đúng một cách sửa đúng, còn `gitbrella move-out` loại bỏ hư hỏng tận gốc: chuyển `.git` ra khỏi thư mục được đồng bộ và để lại một tệp `.git` trỏ tới đó, để trình đồng bộ mang theo các tệp của bạn còn git tự giữ lịch sử.

| Lệnh | Chức năng |
| --- | --- |
| `gitbrella check` | báo cáo hư hỏng (chỉ đọc) |
| `gitbrella fix` | sửa những gì có một câu trả lời đúng |
| `gitbrella move-out` | chuyển `.git` ra khỏi thư mục được đồng bộ vĩnh viễn |
| `gitbrella undo` | hoàn tác lần sửa hoặc lần chuyển gần nhất |
| `gitbrella history` | liệt kê các thay đổi đã thực hiện trên thiết bị này |
| `gitbrella prune-backups` | xóa bản sao lưu cũ của các thay đổi không còn hoàn tác được |
| `gitbrella hook --for claude\|codex` | canh giữ các lệnh gọi công cụ của tác nhân AI |

## Bắt đầu nhanh

**1. Kiểm tra.** Chỉ đọc, không tải xuống, không dùng mạng.

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # mã 0: ổn, 1: hư hỏng, 2: không chạy được
```

**2. Sửa.** Chỉ những gì có một câu trả lời đúng. Không có `--apply` thì chỉ hiển thị kế hoạch.

```sh
gitbrella fix                 # những gì sẽ được sửa và những gì để lại cho bạn
gitbrella fix --apply         # sửa; `gitbrella undo --apply` đưa về như cũ
```

**3. Giải quyết vĩnh viễn.** Chuyển `.git` ra khỏi thư mục được đồng bộ.

```sh
gitbrella move-out            # điều gì sẽ xảy ra và điều gì có thể cản trở
gitbrella move-out --apply
```

**Luôn được bảo vệ.** Chặn commit trong kho bị hư hỏng:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Với tác nhân AI, xem [Dành cho tác nhân](#for-agents).

## Những gì được cam kết

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` chỉ đọc.** Nó không bao giờ ghi vào kho. Git chỉ chạy các lệnh đọc, với `GIT_OPTIONAL_LOCKS=0`, đồng thời tắt gc tự động và bảo trì. Các bài kiểm thử đầu-cuối cho những trường hợp rủi ro (bản sao index, index bị mất, partial clone, split index, thư mục git đã chuyển) ghi lại kích thước, mtime và inode của mọi tệp trước và sau khi kiểm tra rồi so sánh chúng.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Không tải gì từ đám mây.** Trên macOS, nó tắt tải xuống theo yêu cầu cho chính tiến trình của mình (và git mà nó chạy) trước khi chạm vào đĩa; nếu macOS từ chối, việc kiểm tra không chạy (mã 2). Tệp chỉ có trên iCloud được báo cáo chứ không được tải xuống.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Không truy cập mạng.** Partial clone không bao giờ tải các đối tượng bị thiếu (`GIT_NO_LAZY_FETCH`); với git cũ hơn 2.45, các kiểm tra đối tượng sẽ bị bỏ qua.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Im lặng với kho lành mạnh.** Một lock chỉ bị coi là hư hỏng khi nó đã cũ và không git nào đang làm việc với kho đó (ở bất kỳ worktree nào) có thể đã ghi nó: git tạo lock khi giành lấy nó, nên git khởi động sau không thể đang giữ nó. Tệp tạm của git và tệp của các công cụ khác bên trong `.git` chỉ mang tính thông tin. Chạy trên 89 kho đang làm việc lành mạnh không phát hiện hư hỏng nào.

<a name="usage"></a>

## Cách dùng

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Mã thoát: **0** không có hư hỏng, **1** phát hiện hư hỏng, **2** không thể thực hiện kiểm tra. Với các lệnh thay đổi thứ gì đó (`move-out`, `fix`, `undo`): **0** đã lập kế hoạch, đã xong, hoặc không có gì phải làm (`already`), **1** bị từ chối (kèm lý do), xong nhưng có cảnh báo, hoặc còn hư hỏng để lại cho bạn (danh sách MANUAL của `fix`; `check` vẫn tiếp tục báo cáo chúng), **2** thất bại (đã hoàn nguyên hoặc dừng lại; JSON giữ lại những gì khôi phục được và các mục nhật ký) hoặc không chạy được.

`--deep` chạy thêm `git fsck` đầy đủ (lệnh này đọc mọi đối tượng, nên bị bỏ qua khi có đối tượng chỉ trên đám mây). `--provider` chỉ định trình đồng bộ thay vì nhận diện từ đường dẫn: `auto` (mặc định), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage`, `none`. Thư mục Syncthing và Nextcloud (hoặc ownCloud) được nhận ra ở bất kỳ đâu nhờ dấu hiệu mà trình đồng bộ đặt vào bên trong (`.stfolder`, `.sync_*.db`). `fix` và `move-out` nhận cùng `--provider`; hãy truyền giá trị giống như cho `check`, vì một số tên bản sao chỉ được tính cho một trình đồng bộ nhất định. Khi đó JSON có `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Đầu ra của gitbrella check: DAMAGE INDEX_COPY index 2, được gitbrella fix sửa, và INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>Cùng đầu ra đó dưới dạng văn bản</summary>

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

### Dùng như git hook

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit chạy hook pre-commit, hook này chạy gitbrella check: mã 0 thì commit tiếp tục, mã 1 hoặc 2 thì commit bị chặn">
</p>

Trong kho bị hư hỏng, commit bị chặn; trong kho lành mạnh, commit diễn ra như bình thường. Công cụ biết hook do git chạy, nên không nhầm lock của chính commit với một lock cũ.

**Kho nào được kiểm tra.** Trong các hook commit (git truyền cho chúng `GIT_INDEX_FILE` hoặc `GIT_DIR` tuyệt đối), kho được tìm từ tiến trình git đã chạy hook, nên hook có thể đổi thư mục trước: sang thư mục git, thư mục hooks của nó, hoặc một thư mục nằm ngoài mọi kho. Khi git chỉ định kho bằng `GIT_DIR` tuyệt đối (linked worktree, submodule, thư mục git tách riêng), mọi lệnh git trong hook đều làm việc trên kho đó dù hook chuyển đến đâu, và việc kiểm tra cũng vậy. Ngoài ra, thứ được kiểm tra là nơi mà thư mục hiện tại dẫn tới theo chính git (thư mục git mà một `GIT_DIR` tương đối ở đó trỏ tới, hoặc kho được tìm thấy từ đó); nếu đó là một kho khác, hoặc một worktree khác của cùng kho (hook chuyển sang đó, hoặc trình soạn thông điệp commit), việc kiểm tra từ chối (mã 2) thay vì trả lời thay cho kho khác.

Không có đường dẫn, kho được kiểm tra là kho mà git xuất cho hook (`GIT_DIR`), nên cấu hình dotfiles dùng `--git-dir` và thư mục git nằm trong chính worktree của nó cũng hoạt động. Không có đường dẫn, `GIT_DIR` được đặt trong shell cũng được dùng; khác với git, một `GIT_DIR` không tồn tại sẽ bị bỏ qua, và linked worktree không thể lần theo sẽ bị từ chối thay vì coi thư mục hiện tại là worktree của nó.

**Index mà commit ghi lại.** Nếu `GIT_INDEX_FILE` trỏ tới một tệp bên trong thư mục git (như git làm với hook commit), tệp đó được đọc. Nếu index mà commit sẽ ghi lại bị mất hoặc trống, hoặc không còn liệt kê các tệp HEAD vẫn nằm trong worktree, commit bị chặn trước khi ghi lại việc xóa chúng. Một tệp index ở nơi khác mà bạn chỉ tới bằng `GIT_INDEX_FILE` sẽ không được đọc: trong hook, index của chính kho được kiểm tra thay thế, còn kiểm tra thủ công với biến đó bị từ chối (mã 2).

**Xóa có chủ ý.** Để ngừng theo dõi các tệp vẫn ở lại trên đĩa (`git rm --cached`, ngừng theo dõi một thư mục giờ đã bị ignore), hoặc cố ý commit "xóa tất cả" khi các tệp vẫn còn trên đĩa (`git rm -r --cached .`), hãy bỏ qua hook bằng `git commit --no-verify`.

### Với trình quản lý hook

- **pre-commit** (pre-commit.com): thêm kho này với id hook `gitbrella` (pre-commit build nó bằng Go) hoặc `gitbrella-system` (tệp thực thi trong PATH); xem `.pre-commit-hooks.yaml`.
- **lefthook**: đưa `hooks/lefthook.yml` vào qua `remotes`.
- **husky**: `.husky/pre-commit` chứa `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, sau đó `overcommit --sign`.

Ở tất cả các trình này, bất kỳ mã khác 0 nào cũng dừng commit, nên một lần kiểm tra không chạy được (mã 2) cũng dừng nó. Aider commit với `--no-verify` trừ khi đặt `git-commit-verify: true`, và khi đó không có hook commit nào chạy cả. Obsidian Git (máy tính) commit bằng git hệ thống và chạy hook; hãy thêm thư mục chứa `gitbrella` vào thiết lập "Additional PATH" của nó, vì ứng dụng mở từ Dock không nhận PATH của shell. Obsidian Git trên di động (isomorphic-git) không chạy hook.

<a name="for-agents"></a>

### Dành cho tác nhân

`--json` in cùng báo cáo đó dưới dạng tài liệu ổn định có phiên bản (`schemaVersion: 1`, `findings` luôn là một danh sách). Hãy quyết định dựa trên mã thoát và hiển thị các phát hiện cho người dùng.

Hook riêng của các tác nhân chỉ dừng lệnh gọi công cụ khi gặp mã 2: mã 1 ("phát hiện hư hỏng") để lệnh gọi tiếp tục. `gitbrella hook --for claude|codex` nói đúng ngôn ngữ hook của chúng. Nó đọc sự kiện mà tác nhân gửi qua stdin:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="Lệnh gọi công cụ của tác nhân đi qua gitbrella hook: đọc và chỉnh sửa vẫn tiếp tục; trong kho bị hư hỏng, thao tác ghi của git và ghi vào .git bị dừng với mã 2">
</p>

- **Trước lệnh gọi công cụ** (`PreToolUse`): trong kho bị hư hỏng, lệnh git có ghi (commit, add, reset, rebase, push và mọi thứ không được biết là chỉ đọc) và thao tác ghi vào `.git` sẽ dừng lệnh gọi (mã 2, kèm lý do cho tác nhân trong stderr). Đọc, chỉnh sửa tệp của bạn, `git restore` và chính `gitbrella` vẫn tiếp tục, để hư hỏng có thể được xem xét và sửa.
  - *Kho không dùng được.* Khi các tệp git chỉ có trên đám mây, hoặc git hoàn toàn không dùng được kho (`HEAD` bị đổi tên, tệp `.git` trỏ tới nơi không tồn tại), mọi lệnh git đều bị dừng: nó sẽ treo hoặc tác động lên một kho bên ngoài. Ghi vào `.git` vẫn tiếp tục ở đó: cách sửa là thao tác thủ công bên trong `.git` như lý do nêu (`mv '.git/HEAD 2' .git/HEAD`). Biện pháp `core.worktree` được nêu trong gợi ý (`git config --file … --unset core.worktree`) cũng tiếp tục.
  - *Kho nào.* Kho chứa các tệp mà lệnh gọi làm việc với (bản vá Codex nêu tên các tệp của nó; một đường dẫn bên trong `.git` thuộc về worktree bên trên nó). Với lệnh shell, đó là thư mục phiên và mọi thư mục mà các lệnh git của nó chạy trong đó (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), ưu tiên các thư mục có ghi, tối đa năm; nếu một thư mục có ghi không vừa, lệnh gọi được cho qua kèm cảnh báo. Mỗi thư mục được đánh giá theo việc lệnh gọi làm gì ở đó: `git -C ../sound commit` từ một thư mục phiên bị hư hỏng vẫn tiếp tục. Thư mục không phải kho và kho bare được cho qua mà không có thông báo.
  - *Khi không thể chạy kiểm tra.* Bất kỳ điều gì ngăn việc kiểm tra chạy (kể cả `--timeout` 20 giây, một thời hạn duy nhất cho toàn bộ lệnh gọi) đều cho lệnh gọi qua kèm cảnh báo; `--fail-open` làm điều tương tự ở nơi git không dùng được kho.
  - *Cách đọc lệnh.* Lệnh được đọc, không bao giờ được chạy: dấu nháy, chú thích, subshell, phép thay thế (kể cả bên trong heredoc không có dấu nháy), heredoc, biến mà lệnh đặt, `&&`, `||`, `if`, `case` và vòng lặp (sau `cd a || cd b`, hoặc `cd` vào thư mục không tồn tại, mọi thư mục mà shell có thể ở đều được tính; vòng lặp `for` được đọc một lần cho mỗi giá trị), các lệnh bao (`timeout`, `sudo`, `nice`, `xargs`, `env`), và các script được đưa cho shell (`-c`, `eval`, `watch`, `source`, tệp, heredoc, `bash < f`, pipe vào `sh`). Hàm và alias do lệnh định nghĩa được đọc tại nơi chúng được gọi. Script shell được chạy theo đường dẫn hoặc đưa cho shell (`./deploy.sh`, `bash deploy.sh`) được đọc nếu là tệp nhỏ trên máy này (kể cả qua symlink; một lần, ngay cả khi nó tự chạy chính nó); `make`, `npm run` và tương tự thì không được đọc.
  - *Thế nào là ghi.* Việc một chương trình khác ghi vào `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` trên `.git/config`, `rm -rf` hoặc `find -delete` trên thư mục chứa `.git`, bản vá Codex qua shell) được tính là ghi vào `.git`; sao chép `.git` sang nơi khác, đóng gói bằng `tar` hay đọc nó thì không. `gh pr checkout` và `git-lfs` được tính là lệnh git có ghi.
  - *Khi không chắc chắn.* Mỗi trường hợp sau được tính là lệnh git có ghi: việc đọc vượt quá 5000 bước hoặc một nửa `--timeout` (tại thư mục không xác định được); từ `git` ở chỗ mà việc đọc không lần theo được; script được pipe vào shell từ một chương trình mà việc đọc không lần theo (`curl … | sh`), tại nơi nó chạy; script tồn tại nhưng không đọc được đưa cho shell (quá lớn, chỉ trên đám mây), tại nơi nó chạy. Nếu không xác định được lệnh git ghi vào thư mục nào (`cd "$(…)"`, `for d in */`), nó được tính ở mọi thư mục được kiểm tra, và lệnh gọi nhận cảnh báo.
- **Khi bắt đầu phiên** (`SessionStart`): một dòng ra stdout mà tác nhân đọc như ngữ cảnh: rằng kho nằm trong thư mục được đồng bộ (nếu thư mục git của nó chưa được chuyển ra ngoài) và cái gì bị hư hỏng. Không bao giờ chặn.

Hook không thay đổi gì. Với Claude Code, trong `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Với Codex, trong `.codex/hooks.json` (Codex chỉ chạy hook sau khi bạn tin cậy nó trong `/hooks`):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

Việc kiểm tra chạy một lần cho mỗi thư mục như vậy, thường mất 0,1–0,4 giây mỗi lần, nên một bản vá chạm vào nhiều thư mục của cùng một kho sẽ kiểm tra kho đó nhiều lần; `--timeout` giới hạn toàn bộ lệnh gọi.

<a name="repairing"></a>

## Sửa chữa

```sh
gitbrella fix                 # những gì sẽ được sửa và những gì để lại cho bạn
gitbrella fix --apply         # sửa
gitbrella undo --apply        # đưa mọi thứ về như cũ
gitbrella history             # các thay đổi đã làm ở đây, bản sao lưu và kích thước của chúng
gitbrella prune-backups       # các bản sao lưu không còn undo nào cần đến
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix hiển thị kế hoạch; fix --apply ghi lại từng bước rồi chạy lại kiểm tra: hoàn tất hoặc hoàn nguyên; undo --apply hoàn tác thay đổi gần nhất">
</p>

`check` ghi chú dưới mỗi hư hỏng liệu `fix` có sửa được không (`repair:`, và `"repair": "fix"` hoặc `"manual"` trong JSON), và nêu `fix --provider NAME` nếu việc kiểm tra được chỉ định provider. Bên trong hook commit, nó không hỏi gì cả.

`fix` chỉ sửa khi có một câu trả lời đúng và không ghi đè lên thứ gì bạn có thể đã chủ ý:

| Hư hỏng | Cách sửa |
| --- | --- |
| bản sao xung đột trong `.git` bên cạnh bản gốc (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, bản sao của Dropbox và Syncthing, và bản sao của lock ref như `refs/heads/main 2.lock`); bản sao index chỉ khi nó giống hệt index hoặc là một index hợp lệ bằng với HEAD | chuyển vào bản sao lưu; đối tượng (commit, tag, tree hoặc tệp) chỉ được bản sao trỏ tới sẽ được giữ trước dưới dạng `refs/gcd-rescue/<name>` |
| thư mục rebase bị bỏ lại, `desktop.ini`, `Icon` và các tệp tương tự không do git tạo (trừ khi git đọc chúng như một nhánh hoặc tên của chúng là tên đối tượng), bản sao `.git` trong worktree mà các commit đã có trong kho | chuyển vào bản sao lưu |
| index bị mất, không có bản sao và không có lock ở đâu cả | dựng lại từ HEAD (`git read-tree HEAD`: không đụng đến tệp), chỉ khi có commit để dựng (nếu commit HEAD cũng bị mất, nó được tải lại trước như bên dưới) và nằm ngoài sparse checkout |
| index không liệt kê tệp nào (index hợp lệ, không tách, ngoài sparse checkout) | chuyển vào bản sao lưu và dựng lại từ HEAD: không có gì được stage trong đó |
| `index.lock` mồ côi là index hợp lệ bằng với HEAD | trở thành index |
| tệp nhánh trống | commit mà reflog của nó ghi lại gần nhất |
| `.git.nosync` mà git sẽ commit | dòng `/.git.nosync/` trong `.git/info/exclude` |
| commit HEAD bị mất hoặc tệp loose object trống (không có bản sao bên cạnh) | tệp trống được chuyển vào bản sao lưu và các đối tượng được tải lại (`git fetch --refetch`, git 2.36 trở lên) từ `--remote`, nếu không thì từ remote của nhánh, nếu không thì từ `origin`, nếu không thì từ remote duy nhất; các nhánh và tag của remote được tải vào không gian tên tạm `refs/gcd-fetch/` rồi bị xóa, nên không ref nào thay đổi. Nếu remote không gửi đối tượng đó, việc sửa bị hoàn nguyên |
| với `fix --deep`: các đối tượng mà `git fsck` báo là hỏng | mọi đối tượng của các nhánh và tag remote được tải lại theo cùng cách; rồi `git fsck` chạy lại, và nếu vẫn báo lỗi (đối tượng không có trên remote nào, tệp hỏng bị bỏ lại), việc sửa bị hoàn nguyên |
| bản sao xung đột của chính các tệp của bạn (`notes 2.md`) mà git không theo dõi và giống từng byte với tệp được sao chép | chuyển vào bản sao lưu (`git add -A` sẽ commit chúng); bản sao khác biệt, được theo dõi, là liên kết, chỉ trên đám mây hoặc nằm trong submodule thì được giữ nguyên |
| `gc.pid` của máy khác cũ hơn một giờ; `gc.log` | chuyển vào bản sao lưu để gc tự động của git chạy lại ở đây |
| các tệp `.git` hoặc kho đối tượng alternate chỉ có trên đám mây | được tải xuống, và chỉ vậy thôi: trước đó việc kiểm tra không đọc được đối tượng, nên các hư hỏng khác được liệt kê là MANUAL cho đến lần chạy `fix` tiếp theo |

Mọi thứ khác được liệt kê là MANUAL kèm lệnh cần chạy. Trong đó có mọi thứ có thể chứa công việc không còn ở đâu khác:
- index bị hỏng (git có thể vẫn đọc được nó cùng công việc đã stage);
- bản sao index hoặc `index.lock` khác với HEAD, và bản sao của `index.lock`;
- bản sao không có bản gốc (có thể đó là bản duy nhất: hãy đặt nó về chỗ cũ; nếu là bản sao lock, nó được giữ lại và để sang một bên);
- lock không do git nào trên máy này giữ (có thể do git trên thiết bị khác hoặc trong container giữ; lời khuyên nêu commit bên trong);
- bản sao trỏ tới hơn 100 đối tượng (reflog dài);
- bản sao của nhánh mà bất kỳ HEAD nào trỏ tới, reflog của nó, hoặc thư mục chứa nó (git đọc nó, dù tên trông thế nào).

Và mọi thứ mà fix sẽ phải chọn giữa hai phiên bản hoặc không giữ được các commit của bản sao:
- mọi thứ trong kho đối tượng: bản sao, tệp có tên đối tượng (kiểm tra xem cái nào khớp với hash của nó), tệp đối tượng trống không có remote để tải lại;
- mọi thứ trong thư mục git của submodule (chạy `fix` trong submodule);
- bản sao nhánh trỏ tới commit chưa có ở đây (nó có thể vẫn đang đến);
- bản sao `.git` trong worktree có commit không có trong kho (bạn quyết định cái nào là hiện hành);
- tệp trống có bản sao của chính nó hoặc của thư mục chứa nó bên cạnh;
- thư mục quản trị bị hỏng của linked worktree (`git worktree repair`);
- index đã mất các tệp vẫn còn trên đĩa (ngừng theo dõi có chủ ý trông giống hệt; nếu không còn gì khác được stage, lời khuyên cho biết `git read-tree HEAD` đưa chúng về mà không mất gì);
- không có `--deep` hoặc không có remote, các đối tượng mà `git fsck` báo là hỏng.

`fix` hoàn toàn không động đến kho reftable; ở đó phát hiện `REFTABLE_TABLE_UNLISTED` của `check` cho biết cách giữ lại các ref của thiết bị khác. Bản sao lưu phải nằm trên cùng ổ đĩa với kho để những gì fix để sang một bên được di chuyển trong một bước; nếu không, `fix` từ chối (cách sửa chỉ gồm tải xuống không di chuyển gì nên không cần điều này).

Mỗi bước được ghi lại trước khi thực hiện, và cuối cùng việc kiểm tra chạy lại: nếu thứ đã sửa vẫn được phát hiện hoặc xuất hiện hư hỏng mới, các bước bị hoàn tác. Một lần fix bị gián đoạn sẽ được hoàn nguyên bởi lệnh thay đổi tiếp theo chạy với `--apply` trong kho đó; sau đó lệnh ấy dừng lại — hãy chạy lại. Sau 30 giây (`--watch`), `fix` kiểm tra thêm một lần: hư hỏng quay lại là do trình đồng bộ mang về, và giải pháp lâu dài là `move-out`.

`undo` hoàn tác thay đổi mới nhất trước, dù đó là lần sửa hay lần chuyển. Một bước mà kết quả đã thay đổi kể từ lần sửa được để nguyên và liệt kê, nhưng không bao giờ bị ghi đè: git đã ghi lại index được dựng lại, trình đồng bộ đã mang bản sao trở lại, rescue ref đã dịch chuyển, bản sao lưu đã bị xóa, hoặc `.git` đã được chuyển đi. Rescue ref còn lại chừng nào bản sao mà nó giữ còn nằm trong bản sao lưu. Tệp đã tải xuống vẫn ở trên đĩa và đối tượng đã tải lại vẫn ở trong kho đối tượng: không cái nào thay thế thứ gì. Phần còn lại được hoàn tác và undo hoàn tất, nên thay đổi trước đó có thể được hoàn tác tiếp theo.

`history` liệt kê các thay đổi mà thiết bị này đã thực hiện với kho (sửa, chuyển, hoàn tác), bản sao lưu của từng thay đổi cùng kích thước, và thay đổi nào `undo` sẽ hoàn tác tiếp theo. `prune-backups`, chỉ với `--apply`, xóa bản sao lưu của những thay đổi đã được hoàn tác, đã hoàn nguyên, hoặc vốn không thể hoàn tác (undo, prepare, attach), và có bước cuối cũ hơn `--older-than` (mặc định `720h`). Lần sửa hoặc lần chuyển vẫn còn hoàn tác được thì giữ bản sao lưu của nó, kể cả khi có những thay đổi mới hơn phía trên.

<a name="moving-git-out-of-the-synced-folder"></a>

## Chuyển .git ra khỏi thư mục được đồng bộ

```sh
gitbrella move-out            # điều gì sẽ xảy ra và điều gì có thể cản trở
gitbrella move-out --apply    # thực hiện
gitbrella undo --apply        # đưa về như cũ
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Trước: .git nằm trong thư mục được đồng bộ. Sau move-out: tệp .git trong thư mục được đồng bộ trỏ tới ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d ở bên ngoài">
</p>

`move-out` sao chép `.git` tới `~/.local/share/gitbrella/gitdirs/<name>` (trên macOS là một bản clone APFS không tốn dung lượng), chứng minh từng tệp rằng bản sao giống hệt, chuyển bản gốc vào bản sao lưu trong `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`), và ghi một tệp `.git` như `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. Đường dẫn là tương đối, nên mọi thiết bị có thư mục nằm ở cùng vị trí trong thư mục home sẽ ghi cùng một tệp `.git`. Sau đó nó xác nhận git tìm thấy cùng HEAD, trạng thái, cấu hình và thư mục hooks, và trong 30 giây (`--watch`) xác nhận trình đồng bộ để yên `.git` và, với iCloud, tải tệp mới lên. Bản gốc được để sang một bên trước lần so sánh cuối cùng, nên một thao tác ghi lọt vào trong lúc sao chép sẽ bị phát hiện và việc chuyển bị hủy. Trên iCloud, `move-out` từ chối khi iCloud vẫn đang tải kho lên; `--wait-upload 10m` thay vào đó sẽ chờ tối đa khoảng thời gian đó.

Mỗi thay đổi được ghi vào nhật ký trước khi thực hiện, cùng thiết bị và inode của mọi thứ nó tạo ra (và các byte của tệp `.git`); việc hoàn nguyên không đụng tới thứ gì khác. Nếu tiến trình bị gián đoạn giữa chừng, lần `move-out --apply` tiếp theo (hoặc `undo --apply`) sẽ hoàn tất hoặc hoàn nguyên nó rồi dừng lại (chạy lại để tiếp tục); thay đổi dang dở được tìm thấy nhờ inode của thư mục, kể cả khi đường dẫn được viết khác, sau khi thư mục bị đổi tên, hoặc bên trong một kho khác. Trong lúc đó, `check` báo cáo thay đổi dang dở là hư hỏng. Không có gì được hoàn nguyên đè lên thư mục đã chuyển sau khi git bắt đầu dùng nó: nếu nó chứa thứ gì không có trong bản gốc, lệnh dừng lại và nêu tên cả hai. Không có gì bị xóa: bản gốc nằm trong bản sao lưu cho đến khi bạn xóa nó. `undo` đưa về thư mục git đang được dùng, bao gồm các commit tạo ra từ sau khi chuyển; bản sao lưu từ trước khi chuyển không bị đụng tới. Nếu trình đồng bộ đã giải phóng tệp `.git` (chỉ còn tên của nó trên máy Mac này), `undo` và quá trình khôi phục sẽ yêu cầu bạn tải nó xuống trước: đọc nó sẽ kích hoạt tải xuống, và công cụ này không bao giờ làm vậy.

Nếu việc hoàn nguyên thấy cả bản gốc (trong bản sao lưu) lẫn một `.git` mới (do bạn hoặc trình đồng bộ tạo ra sau khi việc chuyển dừng), không cái nào bị ghi đè và điều đó được báo cáo: hãy so sánh chúng, chuyển `.git` mà bạn không muốn ra khỏi thư mục, rồi chạy lại lệnh.

Từ chối (mã 1, kèm lý do) khi git hoặc thứ gì đang giữ lock git đang làm việc với kho, trong lúc merge hoặc rebase, khi có tệp `.git` chỉ trên đám mây, khi iCloud vẫn đang tải thư mục lên, khi `.git` chứa socket (`git fsmonitor--daemon` đang chạy), khi sau lần chuyển tồn tại hai thư mục git, và với các cấu hình mà liên kết sẽ bị gãy: linked worktree, submodule, đối tượng mượn (`alternates`), `.git` là symlink hoặc không phải chính thư mục git. Các hư hỏng khác mà `check` tìm thấy được chuyển theo nguyên trạng và liệt kê là đã mang theo: hãy chạy lại `check` sau đó.

### Các thiết bị khác

Trình đồng bộ xóa `.git` trên mọi thiết bị khác đồng bộ thư mục đó và đưa tệp `.git` tới đó. Để giữ lịch sử của chúng:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: move-out --prepare --apply trên mỗi thiết bị khác. 2: move-out --apply trên một thiết bị. 3: move-out --attach --apply trên mỗi thiết bị khác">
</p>

1. Trên mỗi thiết bị khác, trước khi chuyển: `gitbrella move-out --prepare --apply` sao chép `.git` của thiết bị đó tới nơi tệp `.git` sẽ trỏ tới.
2. Trên một thiết bị: `gitbrella move-out --apply`.
3. Trên mỗi thiết bị khác, sau khi tệp `.git` tới nơi: `gitbrella move-out --attach --apply`. Thiết bị chưa được chuẩn bị thì clone lịch sử: `--attach --from <remote URL> --apply`. Cả hai đều không đụng tới worktree.

Tệp `.git` đến qua trình đồng bộ, nên attach chỉ tin nó trong phạm vi các bản ghi của thiết bị này cho phép: nó phải trỏ vào bên trong `~/.local/share/gitbrella/gitdirs` của thiết bị này, và thư mục đã có sẵn ở đó chỉ được dùng nếu do `--prepare` tạo cho kho này.

Từ đó trở đi, trình đồng bộ mang theo tệp, không mang lịch sử. Hãy chia sẻ commit qua remote. Trên thiết bị mà trình đồng bộ đã cập nhật tệp, `git pull` từ chối ("would be overwritten"): hãy dùng `git fetch` rồi `git reset --mixed @{u}`, lệnh này di chuyển nhánh và index, giữ nguyên các tệp đã đồng bộ.

<a name="what-it-reports"></a>

## Những gì được báo cáo

Hư hỏng (mã 1):

| Mã | Ý nghĩa |
| --- | --- |
| `INDEX_COPY` | bản sao xung đột của index (`index 2`; tên của Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive và Seafile; `index (1)` của Google Drive, `index-COMPUTER` của OneDrive trong thư mục OneDrive, cùng tên của Nextcloud cũ, Synology Drive, pCloud và Resilio chỉ được so khớp với tên tệp riêng của git); cho biết đó có phải index hợp lệ không và có bằng HEAD không |
| `REF_COPY` | bản sao xung đột của ref nhánh hoặc tag; cho biết bản sao cũ hơn, mới hơn, rẽ nhánh, hay trỏ tới commit bị mất |
| `COPY_IN_GITDIR` | bất kỳ bản sao xung đột nào khác trong `.git` |
| `INDEX_MISSING` | index bị mất; git hiển thị mọi tệp là đã xóa (trong hook commit: commit sẽ xóa tất cả tệp) |
| `INDEX_EMPTY` | index không liệt kê tệp nào dù các tệp HEAD vẫn trong worktree, như khi git dựng lại index sau khi trình đồng bộ xóa nó; commit sẽ xóa tất cả tệp |
| `INDEX_SHRUNK` | index không còn liệt kê các tệp HEAD vẫn trong worktree (trong sparse checkout: bất kỳ tệp HEAD nào), như khi git dựng lại nó từ các tệp đã stage sau khi trình đồng bộ xóa index; commit sẽ xóa chúng. Đổi tên, thư mục đã thành symlink hoặc submodule được khai báo trong `.gitmodules`, và tệp cũng đã bị xóa khỏi đĩa không được tính; nếu không liệt kê được các tệp HEAD, nó được báo là chưa xác minh |
| `HEAD_COMMIT_MISSING` | HEAD trỏ tới commit không có trong kho (ref đến trước các đối tượng của nó) |
| `ORPHAN_INDEX_LOCK` | index bị mất và một `index.lock` cũ còn lại (thường là một index hoàn chỉnh) |
| `INDEX_INVALID` | index không vượt qua checksum của chính nó |
| `STALE_LOCK` | lock git mà không git nào đang chạy có thể giữ: không git nào đang làm việc với kho, hoặc tất cả đều khởi động sau khi lock được ghi. Chỉ thấy được các tiến trình git trên máy này: trước khi xóa lock, hãy chắc chắn không git nào đang dùng kho trong container, trên máy khác dùng chung thư mục, hoặc trong ứng dụng có git tích hợp |
| `DATALESS_IN_GITDIR` | tệp trong `.git` chỉ có trên đám mây; các kiểm tra đối tượng bị bỏ qua |
| `FOREIGN_FILE_IN_GITDIR` | tệp trong `refs/` hoặc `objects/` không do git tạo (`desktop.ini`, tệp `.tmp` trong `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | đối tượng hoặc ref có kích thước 0 byte |
| `GITDIR_COPY` | bản sao của thư mục git trong worktree (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | thư mục git nằm trong worktree dưới tên khác (`.git.nosync`) và `git add -A` sẽ commit nó |
| `WORKTREE_ADMIN_BROKEN` | linked worktree đã mất các tệp quản trị của nó |
| `STALE_OPERATION_STATE` | thư mục rebase bị bỏ lại khiến git tưởng đang có rebase |
| `FSCK_ERRORS` | chỉ với `--deep`: `git fsck` báo lỗi (nếu nó không chạy xong, cùng mã đó trở thành thông tin) |
| `REFTABLE_TABLE_UNLISTED` | kho reftable có các bảng ref không được nêu trong danh sách của nó: bảng của thiết bị khác do trình đồng bộ mang tới, mà danh sách của chúng đã thành một bản sao git không bao giờ đọc. Lần `git pack-refs` hoặc bảo trì tiếp theo sẽ xóa chúng cùng các nhánh của thiết bị đó; hãy sao chép thư mục git tới nơi an toàn trước |
| `CHANGE_UNFINISHED` | `fix`, `undo` hoặc `move-out` của gitbrella chưa hoàn tất (bị gián đoạn); trước khi dùng git ở đó, hãy chạy lệnh được nêu với `--apply` để hoàn tất hoặc hoàn nguyên nó |
| `GIT_DIR_RETURNED` | sau `move-out`, `.git` lại thành thư mục (trình đồng bộ đã mang cái cũ trở lại), nên tồn tại hai thư mục git; hãy so sánh và giữ một cái trước khi commit |
| `STAGED_CONFLICT_COPY` | index thêm một bản sao xung đột đồng bộ của một tệp mà nó cũng liệt kê (`notes 2.md` bên cạnh `notes.md`, thư mục sao chép `src 2/`, tên của Syncthing); commit sẽ đưa bản sao vào lịch sử. Chỉ tên là yếu tố quyết định; đổi tên sang một tên như vậy và tệp mang tên bản sao mà không có bản gốc không được tính; không kiểm tra khi dùng split index |

Thông tin (mã thoát không đổi): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (thư mục git đã được chuyển ra khỏi thư mục được đồng bộ: cấu hình an toàn), `WORKTREE_COPIES` (bản sao xung đột của chính các tệp của bạn), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (kho đối tượng alternate có tệp chỉ trên đám mây, nên các kiểm tra đối tượng bị bỏ qua), `GC_PID_FOREIGN` (`gc.pid` của máy khác: git bỏ qua gc tự động trong 12 giờ), `GC_LOG_FRESH` (`gc.log` chưa tới một ngày tuổi làm dừng gc tự động), `HOOKS_IN_SYNCED_FOLDER` (lệnh `core.hooksPath` hoặc `core.fsmonitor` nằm trong thư mục được đồng bộ, bên ngoài thư mục git, nên một thay đổi trên thiết bị khác sẽ chạy ở đây), `WORKTREE_GUESSED` (`GIT_DIR` không có worktree và thư mục hiện tại không chứa tệp HEAD nào: worktree không được kiểm tra).

Nếu đường dẫn hoàn toàn không phải kho, lý do được báo cáo khi nguyên nhân nằm trên đĩa: `.git 2` bị đổi tên, các tệp riêng của git chỉ trên đám mây, thư mục git không có HEAD, tệp `.git` trỏ tới thư mục git không còn tồn tại, hoặc `core.worktree` trỏ tới thư mục không có trên máy này (thiết lập của máy khác do trình đồng bộ mang tới). Nếu trên đường dẫn có một `.git` mà git không dùng được, việc kiểm tra dừng ở đó (mã 2) thay vì trả lời thay cho một kho bên ngoài như chính git sẽ làm.

<a name="install"></a>

## Cài đặt

Cần git 2.31 trở lên. macOS và Linux, amd64 và arm64, không cgo.

```
brew install aliyilmazco/tap/gitbrella
```

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

hoặc tải một tệp nén từ trang releases.

### Xác minh bản phát hành

Mỗi bản phát hành kèm theo `checksums.txt` (SHA-256 của mọi tệp nén và SBOM), chữ ký cosign không khóa `checksums.txt.sigstore.json` do workflow phát hành tạo ra, SBOM SPDX cho mỗi tệp nén, và chứng thực nguồn gốc bản dựng (build provenance attestation) của GitHub. Hãy xác minh chữ ký trước, rồi xác minh tệp nén với danh sách đã ký (ví dụ là v1.2.3 cho Apple silicon). Dùng cosign v3.1.3 trở lên (v2.6.5 ở dòng v2): các phiên bản cũ có thể bị đánh lừa bằng bundle giả mạo (GHSA-fx35-mq7g-6g98).

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

Bản phát hành được build từ tag đã push bởi workflow `.github/workflows/release.yml` bằng `.goreleaser.yaml`, và vẫn ở dạng bản nháp cho đến khi có người xuất bản. Các tệp thực thi được build với `-trimpath` và thời điểm commit, nên cùng một tag với cùng một bản phát hành Go (workflow dùng bản vá 1.26 mới nhất) cho ra các tệp thực thi giống hệt nhau; các tệp nén và SBOM bao quanh chúng mang thời điểm build.

<a name="limits"></a>

## Giới hạn

- `fix` chỉ sửa khi có một câu trả lời đúng và để phần còn lại cho bạn kèm lệnh cần chạy; giải pháp lâu dài là `move-out`, giữ `.git` bên ngoài thư mục được đồng bộ.
- Các bản sao xung đột do chính iCloud tạo ra giữa hai thiết bị chưa được tái hiện trong kiểm thử; việc phát hiện được kiểm thử trên các bản sao có cùng cách đặt tên.
- Codex không truyền `workdir` của công cụ shell cho hook: một lệnh mà Codex chạy ở thư mục khác được kiểm tra tại thư mục phiên.
- Windows và OneDrive trên Windows chưa được kiểm thử. Kho reftable được kiểm tra ở mức tệp và kiểm tra bảng không được liệt kê; bản sao ref ở đó không được so sánh với nhánh của chúng.
- Tên bản sao có những dấu hiệu mà một cái tên có thể cố ý mang theo chỉ được so khớp ở nơi chính git tạo ra tên đó, và không bao giờ trong worktree: `index (1)` của Google Drive với tệp riêng của git và trong `refs/`; `index-COMPUTER` của OneDrive với tệp riêng của git ở cấp cao nhất của thư mục OneDrive, với tên thiết bị có chữ in hoa, không phải từ thường dùng để kết thúc tên bản sao lưu (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` là của bạn), và tối đa 15 ký tự (tên Mac dài hơn sẽ bị bỏ sót); `_conflict-YYYYMMDD-HHMMSS` của Nextcloud cũ, tên của Synology Drive, `.conflicted` của pCloud và `.Conflict` của Resilio với tệp riêng của git ở cấp cao nhất (tên của Nextcloud cũ cũng áp dụng trong `refs/` và `logs/` bên cạnh bản gốc trong thư mục Nextcloud). Nhánh mà bất kỳ HEAD nào trỏ tới (dưới dạng `ref:` với khoảng trắng bất kỳ, hoặc dưới dạng symlink), reflog của nó và các thư mục chứa nó không bao giờ bị để sang một bên, dù tên là gì. Vì vậy `report (1).pdf`, nhánh `fix.Conflict`, tag `v1.0.conflicted` hay `config-backup` đều là tên bình thường.
- Submodule được kiểm tra từ bên trong submodule. Từ dự án cha, bản sao xung đột trong `.git/modules/<name>/` chỉ được phát hiện khi bản gốc nằm bên cạnh, và submodule mang tên bản sao (`lib 2`) được báo cáo là bản sao.
- Index bị mất của linked worktree được phát hiện khi kiểm tra worktree đó, không phải worktree chính.
- Nếu `.git` là symlink tới kho lưu trữ bên ngoài thư mục được đồng bộ, hãy chạy git và việc kiểm tra từ linked worktree. Từ thư mục kho lưu trữ, nơi không chứa tệp nào, một index bị làm trống trông như "xóa tất cả" có chủ ý và không chặn commit.
- Một số commit có chủ ý cần `--no-verify`: ngừng theo dõi tệp vẫn ở lại trên đĩa, xóa tất cả tệp ở nơi không đọc được đối tượng, đổi tên chỉ thay đổi chữ hoa/thường hoặc dạng Unicode của tệp có hard link khác, và bất kỳ commit nào trong sparse checkout (chế độ cone hay không) xóa hoặc đổi tên tệp được theo dõi, hoặc di chuyển nó qua thay đổi kiểu (tệp thành thư mục, thư mục thành symlink hoặc submodule). Ở đó, các mẫu sparse của git quyết định tệp nào có trên đĩa, nên việc xóa có chủ ý không phân biệt được với index bị mất; các mẫu không được đọc.
- Nếu trình đồng bộ cũng làm mất chính ref của nhánh cùng với index (tệp loose của nó bị xóa trước khi `packed-refs` mới tới), HEAD trông như một nhánh mới và commit không bị chặn: nó bắt đầu lịch sử mới chỉ với các tệp đã stage. Không có gì trên đĩa bị xóa, và lịch sử cũ vẫn còn trong reflog (`git reflog`).
- Với split index, hoặc ở nơi không đọc được đối tượng (partial clone với git cũ hơn 2.45, kho alternate chỉ trên đám mây), index không được so sánh với HEAD theo từng tệp: chỉ index bị mất hoặc trống được phát hiện.
- Với `core.splitIndex`, index không liệt kê tệp nào sẽ không được nhận ra (header của nó chỉ đếm phần split), nên commit từ nó không bị chặn.
- Hook: chỉ hook commit được tìm từ tiến trình git chạy chúng. Các hook khác (`pre-push`, `pre-applypatch`) phải chạy việc kiểm tra từ worktree, nếu không sẽ bị từ chối (mã 2). Ở nơi không thấy được tiến trình git (trình khách tự chạy hook, hoặc việc xem tiến trình bị chặn), việc kiểm tra dùng thư mục hiện tại: khi đó, nếu thư mục git không trỏ ngược về worktree (`--separate-git-dir`, `.git.nosync`, `--work-tree`), index không có tệp nào luôn chặn commit (dùng `--no-verify` cho việc làm trống có chủ ý), thư mục hiện tại không chứa tệp HEAD nào không được kiểm tra như worktree (`WORKTREE_GUESSED`), và hook đã chuyển sang kho hoặc thư mục git khác bị từ chối (mã 2). Hook nhận (`pre-receive`) không phải là ranh giới được hỗ trợ.
- `move-out`: không phải công cụ nào cũng hiểu tệp `.git`. Git và các công cụ chạy git hoạt động; isomorphic-git (Obsidian Git trên di động) và các công cụ mở `.git` như một thư mục thì không.
- `move-out`: đổi tên hoặc di chuyển thư mục được đồng bộ sẽ làm hỏng tệp `.git` tương đối. Khi đó `check` nêu tên thư mục bị mất; hãy đưa thư mục về chỗ cũ, hoặc tự trỏ tệp `.git` tới thư mục git.
- `move-out` yêu cầu thư mục được đồng bộ nằm ở cùng vị trí trong thư mục home trên mọi thiết bị (iCloud Drive luôn như vậy; Dropbox có thể là `~/Dropbox` trên một thiết bị và `~/Library/CloudStorage/Dropbox` trên thiết bị khác). Nếu không, tệp `.git` ở đó trỏ tới nơi khác và `--attach` từ chối (`TARGET_UNEXPECTED`).
- Thư mục git đã chuyển nằm trên hệ thống tệp của thư mục home; không có tùy chọn đặt nó ở nơi khác.
- `move-out --prepare` sao chép lịch sử của thiết bị tại thời điểm đó: hãy chuyển ngay sau khi chuẩn bị, không có commit mới ở giữa.
- `move-out` và `fix` đã được thử trong thư mục riêng của iCloud Drive, không phải thư mục iCloud của một ứng dụng (`iCloud~md~obsidian` của Obsidian).

## Đóng góp

Xem [CONTRIBUTING.md](../../CONTRIBUTING.md). Hãy báo cáo vấn đề bảo mật như mô tả trong [SECURITY.md](../../SECURITY.md), không qua issue công khai.

## Giấy phép

MIT
