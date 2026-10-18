<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="gitbrella のロゴ：持ち手が git ブランチになった傘">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  クラウド同期が git リポジトリに与えた損傷を見つけます。<br>
  何も変更せず、何もダウンロードしません。
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
  <b>日本語</b> ·
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
  <a href="#usage">使い方</a> ·
  <a href="#repairing">修復</a> ·
  <a href="#moving-git-out-of-the-synced-folder">.git の移動</a> ·
  <a href="#what-it-reports">報告内容</a> ·
  <a href="#install">インストール</a> ·
  <a href="#limits">制限事項</a>
</p>

iCloud Drive、Dropbox、OneDrive、Google Drive、Syncthing はファイルを 1 つずつ同期します。一方 git は `.git` ディレクトリが全体としてまとまって変化することを前提にしているため、同期フォルダ内のリポジトリには `.git/index 2` や `refs/heads/main 2` のような競合コピー、残された `index.lock` の横で消えた index、クラウドにしか存在しないオブジェクトが生じます。git 自身の FAQ は同期フォルダにリポジトリを置かないよう警告していますが、既定の設定がリポジトリをそこへ置き続けます。macOS の iCloud の「デスクトップ」と「書類」、Windows の OneDrive へのフォルダバックアップ、`Documents/GitHub` などです。

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="同期クライアントは .git をデバイス間で 1 ファイルずつコピーし、index 2 や refs/heads/main 2 のような競合コピー、失われた index の横の index.lock、クラウドにしかないオブジェクトを残す">
</p>

`gitbrella check` は、通常 1 秒以内に、リポジトリにその損傷があるかどうかを教えてくれます。AI エージェントや git フックがリポジトリに触れる前に実行することを想定しています。`gitbrella fix` は正しい修復方法が 1 つしかないものを修復し、`gitbrella move-out` は損傷を根本から終わらせます。`.git` を同期フォルダの外に移し、そこを指す `.git` ファイルを残すので、同期クライアントはファイルを運び、git は履歴を自分の手元に置いておけます。

| コマンド | 内容 |
| --- | --- |
| `gitbrella check` | 損傷を報告する（読み取り専用） |
| `gitbrella fix` | 正しい答えが 1 つしかないものを修復する |
| `gitbrella move-out` | `.git` を同期フォルダの外へ恒久的に移す |
| `gitbrella undo` | 直近の修復または移動を元に戻す |
| `gitbrella history` | このデバイスで行った変更を一覧表示する |
| `gitbrella prune-backups` | もう元に戻せない変更の古いバックアップを削除する |
| `gitbrella hook --for claude\|codex` | AI エージェントのツール呼び出しを守る |

## クイックスタート

**1. 確認する。** 読み取り専用で、ダウンロードもネットワーク通信もしません。

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # 終了コード 0: 正常、1: 損傷あり、2: 実行できなかった
```

**2. 修復する。** 正しい答えが 1 つしかないものだけです。`--apply` を付けなければ計画を表示するだけです。

```sh
gitbrella fix                 # 何を修復し、何をあなたに任せるか
gitbrella fix --apply         # 修復する。`gitbrella undo --apply` で元に戻せる
```

**3. 根本から解決する。** `.git` を同期フォルダの外へ移します。

```sh
gitbrella move-out            # 何が起きるか、何がそれを妨げるか
gitbrella move-out --apply
```

**守り続ける。** 損傷したリポジトリでのコミットをブロックします。

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

AI エージェント向けには[エージェント向け](#for-agents)を参照してください。

## 約束していること

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` は読み取り専用です。** リポジトリに書き込むことはありません。git は `GIT_OPTIONAL_LOCKS=0`、自動 gc とメンテナンスを無効にした状態で、読み取りのコマンドだけを実行します。危険なケース（index のコピー、index の欠落、部分クローン、分割 index、移動された git ディレクトリ）のエンドツーエンドテストでは、チェックの前後ですべてのファイルのサイズ、mtime、inode を記録し、一致することを確認しています。
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **クラウドからダウンロードしません。** macOS では、ディスクに触れる前に自分のプロセス（と起動する git）のオンデマンドダウンロードを無効にします。macOS がこれを拒否した場合、チェックは実行されません（終了コード 2）。iCloud にしか存在しないファイルは報告されるだけで、取得はされません。
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **ネットワークを使いません。** 部分クローンが欠けたオブジェクトを取得することはありません（`GIT_NO_LAZY_FETCH`）。2.45 より古い git では、代わりにオブジェクトのチェックを省略します。
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **正常なリポジトリでは静かです。** ロックが損傷と見なされるのは、それが古く、かつそのリポジトリ（いずれかのワークツリー）で動作中のどの git もそれを書いた可能性がない場合だけです。git はロックを取得するときに作成するので、後から起動した git がそれを保持していることはありえません。git の一時ファイルや `.git` 内にある他のツールのファイルは情報として扱われます。使用中の正常なリポジトリ 89 個で実行したところ、損傷は 1 件も報告されませんでした。

<a name="usage"></a>

## 使い方

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

終了コード：**0** 損傷なし、**1** 損傷あり、**2** チェックを実行できなかった。何かを変更するコマンド（`move-out`、`fix`、`undo`）の終了コードは、**0** 計画済み・完了・何もすることがない（`already`）、**1** 拒否された（理由付き）、警告付きで完了した、またはあなたに任された損傷が残っている（`fix` の MANUAL リスト。`check` は引き続きこれらを報告します）、**2** 失敗した（ロールバックまたは停止。JSON には回復できたものとジャーナルのエントリが残ります）または実行できなかった、です。

`--deep` は完全な `git fsck` も実行します（すべてのオブジェクトを読むため、オブジェクトがクラウドにしかない場合は省略されます）。`--provider` は、パスから検出する代わりに同期クライアントを強制指定します：`auto`（既定）、`icloud`、`icloud-desktop-documents`、`dropbox`、`onedrive`、`google-drive`、`syncthing`、`nextcloud`、`cloud-storage`、`none`。Syncthing と Nextcloud（または ownCloud）のフォルダは、どこにあっても、クライアントがその中に置いているマーカー（`.stfolder`、`.sync_*.db`）で認識されます。`fix` と `move-out` も同じ `--provider` を受け付けます。一部のコピー名は特定のクライアントでしか有効にならないため、`check` に渡したものと同じ値を渡してください。その場合 JSON には `"providerForced": true` と表示されます。

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="gitbrella check の出力：gitbrella fix で修復可能な DAMAGE INDEX_COPY index 2 と INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>同じ出力（テキスト）</summary>

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

### git フックとして

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit が pre-commit フックを実行し、フックが gitbrella check を実行する：終了コード 0 ならコミットは進み、1 または 2 ならブロックされる">
</p>

損傷したリポジトリではコミットがブロックされ、正常なリポジトリでは通常どおりコミットされます。このツールは git がフックを実行していることを把握しており、コミット自身のロックを古いロックと取り違えることはありません。

**どのリポジトリをチェックするか。** コミットフック（git はこれらに `GIT_INDEX_FILE` か絶対パスの `GIT_DIR` を渡します）では、フックを実行した git プロセスからリポジトリを見つけます。そのため、フックは先にディレクトリを移動してもかまいません。git ディレクトリ、その hooks ディレクトリ、あるいはどのリポジトリにも属さないディレクトリへ移っても大丈夫です。git が絶対パスの `GIT_DIR` でリポジトリを指定している場合（リンクされたワークツリー、サブモジュール、分離された git ディレクトリ）、フックがどこへ移動しても、フック内のすべての git コマンドはそのリポジトリに作用し、チェックも同様です。そうでなければ、git 自身が現在のディレクトリからどこに行き着くか（そこで相対パスの `GIT_DIR` が指す git ディレクトリ、またはそこから見つかるリポジトリ）を調べます。それが別のリポジトリや同じリポジトリの別のワークツリー（そこへ移動するフック、またはコミットメッセージのエディタ）であれば、誤ったリポジトリについて答える代わりに拒否します（終了コード 2）。

パスを指定せずに実行すると、git がフックにエクスポートしたリポジトリ（`GIT_DIR`）をチェックします。そのため、`--git-dir` を使った dotfiles の構成や、自身のワークツリー内にある git ディレクトリでも動作します。パスを指定しない場合は、シェルで設定された `GIT_DIR` にも従います。git とは異なり、存在しない `GIT_DIR` は無視し、たどれないリンクされたワークツリーは、現在のディレクトリを作業ツリーとみなす代わりに拒否します。

**コミットが記録する index。** `GIT_INDEX_FILE` が git ディレクトリ内のファイルを指している場合（git はコミットフックに対してこのように設定します）、それを読みます。コミットが記録する index が欠けているか空である場合、あるいは作業ツリーにまだ存在する HEAD のファイルを index が列挙しなくなっている場合、コミットがそれらのファイルを削除として記録する前にブロックされます。あなた自身が `GIT_INDEX_FILE` で別の場所の index ファイルを指定した場合、それは読まれません。フック内では代わりにリポジトリ自身の index がチェックされ、この変数を設定したまま手動で実行したチェックは拒否されます（終了コード 2）。

**意図的な削除。** ディスク上に残すファイルの追跡をやめる場合（`git rm --cached`、今後無視するディレクトリの追跡をやめる場合）や、ファイルがまだディスク上にある状態で意図的に「すべて削除」をコミットする場合（`git rm -r --cached .`）は、`git commit --no-verify` でフックをスキップしてください。

### フックマネージャーと使う

- **pre-commit**（pre-commit.com）：フック ID `gitbrella`（pre-commit が Go でビルドします）または `gitbrella-system`（PATH 上のバイナリ）でこのリポジトリを追加します。`.pre-commit-hooks.yaml` を参照してください。
- **lefthook**：`remotes` で `hooks/lefthook.yml` を取り込みます。
- **husky**：`gitbrella check` を書いた `.husky/pre-commit`。
- **Overcommit**：`PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }` を設定し、`overcommit --sign` を実行します。

どのツールでも 0 以外の終了コードはコミットを止めるため、実行できなかったチェック（終了コード 2）でもコミットは止まります。Aider は `git-commit-verify: true` を設定しない限り `--no-verify` でコミットし、その場合コミットフックは一切実行されません。Obsidian Git（デスクトップ版）はシステムの git でコミットしてフックを実行します。Dock から起動したアプリはシェルの PATH を受け取らないため、`gitbrella` のあるフォルダを「Additional PATH」設定に追加してください。モバイル版の Obsidian Git（isomorphic-git）はフックを実行しません。

<a name="for-agents"></a>

### エージェント向け

`--json` は同じレポートを、安定したバージョン付きのドキュメントとして出力します（`schemaVersion: 1`、`findings` は常にリスト）。終了コードで判断し、検出結果をユーザーに示してください。

エージェント自身のフックは、終了コード 2 のときだけツール呼び出しを止めます。終了コード 1（「損傷あり」）では呼び出しはそのまま進みます。`gitbrella hook --for claude|codex` はそれらのフックの作法に合わせて動作します。エージェントが stdin で渡すイベントを読み取ります。

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="エージェントのツール呼び出しは gitbrella hook を通る：読み取りと編集は続行される。損傷したリポジトリでは、git の書き込みと .git への書き込みが終了コード 2 で止められる">
</p>

- **ツール呼び出しの前**（`PreToolUse`）：損傷したリポジトリでは、書き込みを行う git コマンド（commit、add、reset、rebase、push、および読み取り専用だと分かっていないものすべて）と `.git` 内への書き込みが呼び出しを止めます（終了コード 2。理由はエージェント向けに stderr に出力されます）。読み取り、ファイルの編集、`git restore`、そして `gitbrella` 自体は続行されるので、損傷を調べて修復できます。
  - *使えないリポジトリ。* git のファイルがクラウドにしかない場合や、git がリポジトリをまったく使えない場合（名前が変わった `HEAD`、どこも指していない `.git` ファイル）は、あらゆる git コマンドが止められます。それらはハングするか、外側のリポジトリに作用してしまうからです。この場合、`.git` 内への書き込みは続行されます。理由にあるとおり、その修復は `.git` 内で手作業で行うものだからです（`mv '.git/HEAD 2' .git/HEAD`）。`core.worktree` のヒントが示す対処（`git config --file … --unset core.worktree`）も続行されます。
  - *どのリポジトリか。* 呼び出しが扱うファイルを含むリポジトリです（Codex のパッチは自分のファイルを明示します。`.git` 内のパスはその上の作業ツリーとして扱われます）。シェルコマンドの場合は、セッションのフォルダと、その git コマンドが実行されるすべてのフォルダ（`cd DIR`、`pushd`、`git -C DIR`、`--git-dir`、`--work-tree`、`GIT_DIR`、`env -C`、`bash -c '…'`）で、書き込むものを優先して最大 5 つです。書き込むものが収まらない場合、呼び出しは通知付きで通されます。各フォルダは、呼び出しがそこで何をするかで判断されます。損傷したセッションフォルダから実行した `git -C ../sound commit` は続行されます。リポジトリでないフォルダと bare リポジトリは何も言わずに通ります。
  - *チェックを実行できない場合。* チェックの実行を妨げるもの（20 秒の `--timeout` を含む。これは呼び出し全体に対する 1 つの期限です）があれば、呼び出しは通知付きで通されます。`--fail-open` を指定すると、git がリポジトリを使えない場合も同様に扱います。
  - *コマンドの読み方。* コマンドは読むだけで、決して実行しません。引用符、コメント、サブシェル、置換（引用符なしのヒアドキュメント内も含む）、ヒアドキュメント、コマンドが設定する変数、`&&`、`||`、`if`、`case`、ループ（`cd a || cd b` の後や、存在しないフォルダへの `cd` の後は、シェルがいる可能性のあるすべてのフォルダが対象になります。`for` ループは値ごとに 1 回読まれます）、ラッパー（`timeout`、`sudo`、`nice`、`xargs`、`env`）、そしてシェルに渡されるスクリプト（`-c`、`eval`、`watch`、`source`、ファイル、ヒアドキュメント、`bash < f`、`sh` へのパイプ）を読みます。コマンドが定義した関数やエイリアスは、呼び出された場所で読まれます。パスで実行されるシェルスクリプトやシェルに渡されるシェルスクリプト（`./deploy.sh`、`bash deploy.sh`）は、このマシン上の小さなファイルであれば読まれます（シンボリックリンク経由でも、また自分自身を実行する場合でも 1 回だけ）。`make` や `npm run` などは読まれません。
  - *書き込みとみなされるもの。* 別のプログラムによる `.git` 内への書き込み（`rm .git/index.lock`、`> .git/HEAD`、`cd .git && rm index.lock`、`.git/config` への `sed -i`、`.git` を含むフォルダへの `rm -rf` や `find -delete`、シェル経由の Codex パッチ）は `.git` への書き込みとみなされます。`.git` を別の場所へコピーすること、`tar` でまとめること、読むことは該当しません。`gh pr checkout` と `git-lfs` は書き込みを行う git コマンドとみなされます。
  - *判断がつかない場合。* 次のそれぞれは書き込みを行う git コマンドとみなされます。5000 ステップまたは `--timeout` の半分を超える読み取り（判別できないフォルダにおいて）、読み取りが追跡しない場所にある `git` という単語、読み取りが追跡しないプログラムからシェルへパイプされるスクリプト（`curl … | sh`。実行される場所において）、存在するが読めない（大きすぎる、クラウドにしかない）シェルに渡されるスクリプト（実行される場所において）。git コマンドが書き込むフォルダを判別できない場合（`cd "$(…)"`、`for d in */`）、そのコマンドはチェック対象のすべてのフォルダで考慮され、呼び出しには通知が付きます。
- **セッション開始時**（`SessionStart`）：stdout に 1 行を出力し、エージェントはそれをコンテキストとして読みます。リポジトリが同期フォルダ内にあること（git ディレクトリがすでに外にある場合を除く）と、何が損傷しているかを伝えます。ブロックすることはありません。

フックは何も変更しません。Claude Code では `.claude/settings.json` に次のように書きます。

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex では `.codex/hooks.json` に書きます（Codex は `/hooks` でフックを信頼した後でないと実行しません）。

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

チェックはこれらのフォルダごとに 1 回、通常それぞれ 0.1〜0.4 秒で実行されます。そのため、1 つのリポジトリの複数フォルダにまたがるパッチは、そのリポジトリを複数回チェックします。`--timeout` は呼び出し全体の時間を制限します。

<a name="repairing"></a>

## 修復

```sh
gitbrella fix                 # 何を修復し、何をあなたに任せるか
gitbrella fix --apply         # 修復する
gitbrella undo --apply        # すべて元に戻す
gitbrella history             # ここで行った変更、そのバックアップとサイズ
gitbrella prune-backups       # どの undo にももう必要ないバックアップ
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix は計画を表示する。fix --apply は各ステップを記録し、その後チェックを再実行する：完了するか、ロールバックされる。undo --apply は直近の変更を元に戻す">
</p>

`check` は各損傷の下に `fix` で修復できるかどうかを示します（`repair:`、JSON では `"repair": "fix"` または `"manual"`）。チェックに provider が指定されていれば `fix --provider NAME` も示します。コミットフック内では確認を求めません。

`fix` が修復するのは、正しい答えが 1 つしかなく、あなたが意図していたかもしれないものを何も上書きしない場合だけです。

| 損傷 | 修復 |
| --- | --- |
| `.git` 内で元のファイルの横にある競合コピー（`HEAD 2`、`refs/heads/main 2`、`refs 2`、`packed-refs 2`、Dropbox と Syncthing のコピー、および `refs/heads/main 2.lock` のような ref のロックのコピー）。index のコピーは、index と完全に同一であるか、HEAD と等しい有効な index である場合のみ | バックアップへ移動。コピーだけが指していたオブジェクト（コミット、タグ、ツリー、ファイル）は、先に `refs/gcd-rescue/<name>` として保全される |
| 残された rebase ディレクトリ、`desktop.ini`、`Icon` など git が作っていない類似ファイル（git がブランチとして読むもの、またはオブジェクト名と同じ名前のものを除く）、リポジトリがそのコミットを持っている作業ツリー内の `.git` のコピー | バックアップへ移動 |
| index が欠けていて、そのコピーもロックもどこにもない | HEAD から作成（`git read-tree HEAD`：ファイルには触れない）。元にするコミットがあり（HEAD のコミットも欠けている場合は、下記のように先に取得する）、スパースチェックアウトでない場合のみ |
| ファイルを 1 つも列挙しない index（スパースチェックアウト外の、有効で分割されていない index） | バックアップへ移動し、HEAD から作成：ステージされていたものは何もない |
| HEAD と等しい有効な index である孤立した `index.lock` | それを index にする |
| 空のブランチファイル | その reflog が最後に記録したコミット |
| git がコミットに含めてしまう `.git.nosync` | `.git/info/exclude` に `/.git.nosync/` という行を追加 |
| HEAD のコミットが欠けている、または空のルーズオブジェクトファイル（横にコピーがない） | 空のファイルをバックアップへ移動し、オブジェクトを再取得する（`git fetch --refetch`、git 2.36 以降）。取得元は `--remote`、なければブランチのリモート、なければ `origin`、なければ唯一のリモート。リモートのブランチとタグは一時的な `refs/gcd-fetch/` 名前空間に取得され、後で削除されるので、ref は何も変わらない。リモートがオブジェクトを取り戻せなければ修復はロールバックされる |
| `fix --deep` 指定時：`git fsck` が損傷を報告したオブジェクト | リモートのブランチとタグのすべてのオブジェクトを同じ方法で再取得する。その後 `git fsck` を再実行し、まだエラーが報告される（どのリモートにもないオブジェクト、残り続ける破損ファイル）場合、修復はロールバックされる |
| あなた自身のファイルの競合コピー（`notes 2.md`）で、git が追跡しておらず、コピー元と同じバイト内容のもの | バックアップへ移動（`git add -A` でコミットされてしまうため）。内容が異なる、追跡されている、リンクである、クラウドにしかない、サブモジュール内にあるコピーはそのまま残す |
| 1 時間以上前の別マシンの `gc.pid`、`gc.log` | バックアップへ移動し、git の自動 gc がここで再び動くようにする |
| `.git` または代替オブジェクトストアの、クラウドにしかないファイル | ダウンロードするだけで、他には何もしない。それまではチェックがオブジェクトを読めないため、残りの損傷は次回の `fix` 実行に向けて MANUAL として表示される |

それ以外はすべて、実行すべきコマンドとともに MANUAL として表示されます。これには、他のどこにもない作業を含んでいるかもしれないものがすべて含まれます。
- 損傷した index（git はまだそれを、ステージされた作業ごと読めるかもしれません）
- HEAD と異なる index のコピーや `index.lock`、および `index.lock` のコピー
- 元のファイルが欠けているコピー（それが唯一のものかもしれません。元に戻してください。ただしロックのコピーの場合は保全され、脇に移されます）
- このマシンのどの git も保持していないロック（別のデバイスやコンテナ内の git が保持しているかもしれません。アドバイスにはその中のコミットが示されます）
- 100 を超えるオブジェクトを指すコピー（長い reflog）
- いずれかの HEAD が指すブランチ、その reflog、またはそのブランチを含むディレクトリのコピー（名前がどう見えようと git はそれを読みます）

また、fix が 2 つのバージョンのどちらかを選ばなければならない場合や、コピーのコミットを保持できない場合もすべて含まれます。
- オブジェクトストア内のすべて：コピー、オブジェクトの名前を持つファイル（どれがハッシュと一致するか確認してください）、取得元のリモートがない空のオブジェクトファイル
- サブモジュールの git ディレクトリ内のすべて（サブモジュール内で `fix` を実行してください）
- まだここに届いていないコミットを指すブランチのコピー（まだ届くかもしれません）
- リポジトリにないコミットを含む、作業ツリー内の `.git` のコピー（どちらが最新かはあなたが判断します）
- 自分自身またはそのディレクトリのコピーが横にある空のファイル
- 壊れたリンクワークツリーの管理ディレクトリ（`git worktree repair`）
- まだディスク上にあるファイルを失った index（意図的に追跡をやめた場合も同じに見えます。他に何もステージされていなければ、アドバイスは `git read-tree HEAD` で何も失わずに取り戻せると伝えます）
- `--deep` なし、またはリモートなしの場合に `git fsck` が損傷を報告したオブジェクト

`fix` は reftable リポジトリにはまったく触れません。その場合は `check` の `REFTABLE_TABLE_UNLISTED` が、別デバイスの ref を守る方法を示します。バックアップはリポジトリと同じディスク上になければなりません。fix が脇に移すものを 1 ステップで移動するためです。そうでない場合 `fix` は拒否します（ダウンロードだけの修復は何も移動しないので不要です）。

各ステップは実行前に記録され、最後にチェックが再実行されます。修復したものがまだ報告される場合や新たな損傷が現れた場合、ステップは元に戻されます。中断された fix は、そのリポジトリで次に `--apply` 付きで実行された変更コマンドによってロールバックされ、そのコマンドはそこで停止します。もう一度実行してください。30 秒後（`--watch`）に `fix` はもう一度チェックします。戻ってきた損傷は同期クライアントが持ち帰ったもので、恒久的な解決策は `move-out` です。

`undo` は最新の変更（修復または移動）から元に戻します。修復後に結果が変わったステップは、そのまま残して一覧に示し、決して上書きしません。たとえば、git が再構築された index を書き換えた、同期クライアントがコピーを持ち帰った、レスキュー ref が動いた、バックアップが削除された、`.git` が移動された、といった場合です。レスキュー ref は、それが保持しているコピーがバックアップ内にある限り残ります。ダウンロードしたファイルはディスクに残り、再取得したオブジェクトはオブジェクトストアに残ります。どちらも何かを置き換えることはありません。残りは元に戻され、undo は完了するので、その前の変更を続けて元に戻せます。

`history` は、このデバイスがリポジトリに行った変更（修復、移動、undo）、それぞれが保持するバックアップとそのサイズ、そして `undo` が次にどれを元に戻すかを表示します。`prune-backups` は `--apply` 付きの場合に限り、元に戻された、ロールバックされた、あるいは元々元に戻せない（undo、prepare、attach）変更のうち、最後のステップが `--older-than`（既定 `720h`）より古いもののバックアップを削除します。まだ元に戻せる修復や移動は、その上に新しい変更があってもバックアップを保持します。

<a name="moving-git-out-of-the-synced-folder"></a>

## .git を同期フォルダの外へ移す

```sh
gitbrella move-out            # 何が起きるか、何がそれを妨げるか
gitbrella move-out --apply    # 実行する
gitbrella undo --apply        # 元に戻す
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="移動前：.git は同期フォルダの中。move-out 後：同期フォルダ内の .git ファイルが外部の ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d を指す">
</p>

`move-out` は `.git` を `~/.local/share/gitbrella/gitdirs/<name>` にコピーし（macOS では APFS クローン：容量を消費しません）、コピーが同一であることをファイルごとに検証し、元のものを `~/Library/Application Support/gitbrella`（Linux：`$XDG_STATE_HOME/gitbrella`）以下のバックアップへ移し、`gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d` のような `.git` ファイルを書き込みます。パスは相対パスなので、ホーム以下の同じ場所にフォルダを置いているデバイスはどれも同じ `.git` ファイルを書き込みます。その後、git が同じ HEAD、ステータス、設定、hooks ディレクトリを見つけられることを確認し、30 秒間（`--watch`）同期クライアントが `.git` に手を出さないこと、そして iCloud の場合は新しいファイルをアップロードすることを確認します。元のものは最後の比較の前に脇へ移されるので、コピー中に紛れ込んだ書き込みは検出され、移動は元に戻されます。iCloud では、iCloud がまだリポジトリをアップロード中の間 `move-out` は拒否します。`--wait-upload 10m` を指定すると、代わりにその時間まで待ちます。

すべての変更は、実行前に、作成するすべてのもののデバイスと inode（および `.git` ファイルのバイト列）とともにジャーナルに記録されます。ロールバックはそれ以外のものには触れません。プロセスが途中で終了した場合、次の `move-out --apply`（または `undo --apply`）がそれを完了またはロールバックしてそこで停止します（続けるにはコマンドをもう一度実行してください）。フォルダの inode で見つけるので、パスの表記が違っていても、名前が変更された後でも、別のリポジトリの中にあっても見つかります。その間 `check` は未完了の変更を損傷として報告します。git が移動先のディレクトリを使い始めた後は、その上にロールバックすることはありません。元のものにない内容が含まれていれば、コマンドは停止して両方を示します。何も削除されません。元のものはあなたが削除するまでバックアップに残ります。`undo` は、現在使用中の git ディレクトリを、移動後に行われたコミットとともに戻します。移動前のバックアップには手を付けません。同期クライアントが `.git` ファイルを退避させている場合（この Mac には名前だけがある状態）、`undo` と復旧は、先にそれをダウンロードするよう求めます。読めばダウンロードが発生しますが、このツールは決してそれを行わないからです。

ロールバック時に元のもの（バックアップ内）と新しい `.git`（移動が止まった後に、あなたか同期クライアントが作成したもの）の両方が見つかった場合、どちらも上書きせず、その旨を伝えます。両者を比較し、不要な方の `.git` をフォルダの外へ移してから、コマンドをもう一度実行してください。

次の場合は拒否します（終了コード 1、理由付き）。git や git のロックを保持する何かがリポジトリで動作している間、merge や rebase の最中、`.git` のファイルがクラウドにしかない間、iCloud がまだフォルダをアップロードしている間、`.git` 内にソケット（実行中の `git fsmonitor--daemon`）がある間、移動後に git ディレクトリが 2 つ存在する場合、そしてリンクが壊れてしまう構成の場合です：リンクされたワークツリー、サブモジュール、借用オブジェクト（`alternates`）、シンボリックリンクである `.git` や git ディレクトリそのものではない `.git`。`check` が見つけたその他の損傷はそのまま移動され、持ち越しとして表示されます。後で `check` を再実行してください。

### 他のデバイス

同期クライアントは、そのフォルダを同期している他のすべてのデバイスで `.git` を削除し、代わりに `.git` ファイルを届けます。それらのデバイスの履歴を守るには：

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1：他の各デバイスで move-out --prepare --apply。2：1 台のデバイスで move-out --apply。3：他の各デバイスで move-out --attach --apply">
</p>

1. 移動の前に、他の各デバイスで：`gitbrella move-out --prepare --apply` が、そのデバイスの `.git` を `.git` ファイルが指すことになる場所へコピーします。
2. 1 台のデバイスで：`gitbrella move-out --apply`。
3. `.git` ファイルが届いた後、他の各デバイスで：`gitbrella move-out --attach --apply`。準備をしていないデバイスは、代わりに履歴をクローンします：`--attach --from <remote URL> --apply`。どちらの場合も作業ツリーには触れません。

`.git` ファイルは同期クライアント経由で届くため、attach はこのデバイスの記録が許す範囲でしかそれを信頼しません。それはこのデバイスの `~/.local/share/gitbrella/gitdirs` の中を指していなければならず、そこにすでにあるディレクトリは、`--prepare` がこのリポジトリのために作成したものである場合にのみ使われます。

それ以降、同期クライアントはファイルを運び、履歴は運びません。コミットはリモート経由で共有してください。同期クライアントがすでにファイルを更新したデバイスでは、`git pull` は拒否されます（"would be overwritten"）。`git fetch` と `git reset --mixed @{u}` を使ってください。これはブランチと index を動かし、同期されたファイルはそのまま残します。

<a name="what-it-reports"></a>

## 報告内容

損傷（終了コード 1）：

| コード | 意味 |
| --- | --- |
| `INDEX_COPY` | index の競合コピー（`index 2`。Dropbox、Syncthing、Nextcloud/ownCloud、Proton Drive、Seafile の名前。Google Drive の `index (1)`、OneDrive フォルダ内の OneDrive の `index-COMPUTER`、旧 Nextcloud、Synology Drive、pCloud、Resilio の名前は git 自身のファイル名に対してのみ照合される）。有効な index かどうか、HEAD と一致するかどうかを示す |
| `REF_COPY` | ブランチまたはタグの ref の競合コピー。コピーが古いか、新しいか、分岐しているか、存在しないコミットを指しているかを示す |
| `COPY_IN_GITDIR` | `.git` 内のその他の競合コピー |
| `INDEX_MISSING` | index がない。git はすべてのファイルを削除済みとして表示する（コミットフック内では：コミットがすべてのファイルを削除してしまう） |
| `INDEX_EMPTY` | HEAD のファイルがまだ作業ツリーにあるのに、index がファイルを 1 つも列挙していない。同期クライアントが index を削除した後に git が再構築した場合に起きる。コミットするとすべてのファイルが削除される |
| `INDEX_SHRUNK` | まだ作業ツリーにある HEAD のファイルを index が列挙しなくなっている（スパースチェックアウトでは：HEAD のいずれかのファイル）。同期クライアントが index を削除した後に git がステージ済みのファイルから再構築した場合に起きる。コミットするとそれらが削除される。名前の変更、シンボリックリンクや `.gitmodules` で宣言されたサブモジュールになったディレクトリ、ディスクからも削除されたファイルは数えない。HEAD のファイルを列挙できない場合は未検証として報告する |
| `HEAD_COMMIT_MISSING` | HEAD がリポジトリにないコミットを指している（ref がオブジェクトより先に届いた） |
| `ORPHAN_INDEX_LOCK` | index がなく、古い `index.lock` が残っている（多くの場合、完全な index） |
| `INDEX_INVALID` | index が自身のチェックサムに合わない |
| `STALE_LOCK` | 実行中のどの git も保持しえない git のロック：リポジトリで動作中の git がない、またはすべての git がロックが書かれた後に起動した。見えるのはこのマシン上の git プロセスだけなので、ロックを削除する前に、コンテナ内、フォルダを共有する別のマシン、git を組み込んだアプリでリポジトリを使っている git がないことを確認すること |
| `DATALESS_IN_GITDIR` | `.git` 内のファイルがクラウドにしかない。オブジェクトのチェックは省略された |
| `FOREIGN_FILE_IN_GITDIR` | `refs/` または `objects/` 内にある git が作っていないファイル（`desktop.ini`、`objects/` 内の `.tmp` ファイル） |
| `EMPTY_FILE_IN_GITDIR` | 0 バイトのオブジェクトまたは ref |
| `GITDIR_COPY` | 作業ツリー内の git ディレクトリのコピー（`.git 2`） |
| `GIT_DIR_NOT_IGNORED` | git ディレクトリが別の名前で作業ツリー内にあり（`.git.nosync`）、`git add -A` でコミットされてしまう |
| `WORKTREE_ADMIN_BROKEN` | リンクされたワークツリーが管理ファイルを失った |
| `STALE_OPERATION_STATE` | 残された rebase ディレクトリのせいで、git が rebase の途中だと思い込んでいる |
| `FSCK_ERRORS` | `--deep` のみ：`git fsck` がエラーを報告した（完了しなかった場合、同じコードが情報になる） |
| `REFTABLE_TABLE_UNLISTED` | reftable リポジトリに、そのリストが名前を挙げていない ref テーブルがある：同期クライアントが持ち込んだ別デバイスのテーブルで、そのデバイスのリストは git が決して読まないコピーになってしまった。次の `git pack-refs` やメンテナンスでそれらは削除され、そのデバイスのブランチも一緒に消える。先に git ディレクトリを安全な場所へコピーすること |
| `CHANGE_UNFINISHED` | gitbrella の `fix`、`undo`、`move-out` が完了しなかった（中断された）。そこで git を使う前に、示されたコマンドを `--apply` 付きで実行して、完了またはロールバックすること |
| `GIT_DIR_RETURNED` | `move-out` の後、`.git` が再びディレクトリになっている（同期クライアントが古いものを持ち帰った）ため、git ディレクトリが 2 つ存在する。コミットする前に比較して 1 つを残すこと |
| `STAGED_CONFLICT_COPY` | index が、自身も列挙しているファイルの同期競合コピーを追加している（`notes.md` の横の `notes 2.md`、コピーされたディレクトリ `src 2/`、Syncthing の名前）。コミットするとコピーが履歴に入る。名前だけで判断する。そうした名前への名前変更や、元のファイルがないコピー風の名前のファイルは数えない。分割 index の場合はチェックしない |

情報（終了コードは変わらない）：`IN_CLOUD_FOLDER`、`GIT_DIR_NOT_SYNCED`（git ディレクトリが同期フォルダの外へ移されている：安全な構成）、`WORKTREE_COPIES`（あなた自身のファイルの競合コピー）、`GIT_RUNNING`、`LOCK_IN_USE`、`LOCK_RECENT`、`IN_PROGRESS_OPERATION`、`UNKNOWN_IN_GITDIR`、`NO_CHECKOUT`、`PARTIAL_CLONE_UNVERIFIED`、`WORKTREE_SCAN_TRUNCATED`、`DEEP_CHECK_SKIPPED`、`REFTABLE_UNSUPPORTED`、`UNREADABLE_IN_GITDIR`、`PROCESS_CHECK_FAILED`、`GIT_DIR_IGNORE_UNCHECKED`、`GIT_TEMP_FILE`、`DATALESS_IN_ALTERNATE`（代替オブジェクトストアにクラウドにしかないファイルがあるため、オブジェクトのチェックを省略した）、`GC_PID_FOREIGN`（別マシンの `gc.pid`：git は 12 時間自動 gc を省略する）、`GC_LOG_FRESH`（1 日未満の `gc.log` が自動 gc を止める）、`HOOKS_IN_SYNCED_FOLDER`（`core.hooksPath` または `core.fsmonitor` のコマンドが git ディレクトリ外の同期フォルダにあるため、別デバイスでの変更がここで実行される）、`WORKTREE_GUESSED`（作業ツリーなしの `GIT_DIR` で、現在のディレクトリに HEAD のファイルが 1 つもない：作業ツリーはチェックされなかった）。

パスがそもそもリポジトリでない場合、原因がディスク上にあればその理由を示します。名前が変わった `.git 2`、クラウドにしかない git 自身のファイル、HEAD のない git ディレクトリ、もう存在しない git ディレクトリを指す `.git` ファイル、このマシンにないフォルダを指す `core.worktree`（同期クライアントが持ち込んだ別マシンの設定）などです。パスに git が使えない `.git` がある場合は、git 自身のように外側のリポジトリについて答える代わりに、そこで停止します（終了コード 2）。

<a name="install"></a>

## インストール

git 2.31 以降が必要です。macOS と Linux、amd64 と arm64 に対応し、cgo は使いません。

```
brew install aliyilmazco/tap/gitbrella
```

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

または、releases ページからアーカイブを入手してください。

### リリースの検証

各リリースには、`checksums.txt`（各アーカイブと SBOM の SHA-256）、リリースワークフローが作成したキーレス cosign 署名 `checksums.txt.sigstore.json`、各アーカイブの SPDX SBOM、GitHub のビルド来歴証明（build-provenance attestation）が付属します。まず署名を確認し、次に署名済みリストに照らしてアーカイブを確認します（例は Apple シリコン用の v1.2.3）。cosign v3.1.3 以降（v2 系では v2.6.5）を使ってください。古いバージョンは偽造されたバンドルにだまされる可能性があります（GHSA-fx35-mq7g-6g98）。

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

リリースは、プッシュされたタグから `.github/workflows/release.yml` が `.goreleaser.yaml` を使ってビルドし、誰かが公開するまで下書きのままです。バイナリは `-trimpath` とコミットの時刻でビルドされるため、同じタグと同じ Go のリリース（ワークフローは最新の 1.26 パッチを使います）からは同じバイナリができます。それらを包むアーカイブと SBOM にはビルド時刻が含まれます。

<a name="limits"></a>

## 制限事項

- `fix` は正しい答えが 1 つしかない場合にのみ修復し、残りは実行すべきコマンドとともにあなたに任せます。恒久的な解決策は、`.git` を同期フォルダの外に置き続ける `move-out` です。
- iCloud 自身が 2 台のデバイス間で作成する競合コピーは、テストで再現できていません。検出は、同じ命名規則に従うコピーでテストしています。
- Codex はシェルツールの `workdir` をフックに渡しません。Codex が別のフォルダで実行したコマンドは、セッションのフォルダでチェックされます。
- Windows および Windows 上の OneDrive はテストしていません。reftable リポジトリでは、ファイルレベルのチェックと未列挙テーブルのチェックが行われますが、ref のコピーをブランチと比較することはしません。
- 名前に意図的に含まれうる印を持つコピー名は、git 自身が名前を作る場所でのみ照合し、作業ツリーでは照合しません。Google Drive の `index (1)` は git 自身のファイルと `refs/` 内のファイルに対して。OneDrive の `index-COMPUTER` は OneDrive フォルダ最上位にある git 自身のファイルに対して、デバイス名に大文字を含み、バックアップ名の末尾によく使われる語でなく（`index-old`、`config-OLD`、`config-Backup`、`index-BAK` はあなたのもの）、15 文字以内の場合（それより長い Mac の名前は見逃されます）。旧 Nextcloud の `_conflict-YYYYMMDD-HHMMSS`、Synology Drive の名前、pCloud の `.conflicted`、Resilio の `.Conflict` は最上位にある git 自身のファイルに対して（旧 Nextcloud のものは、Nextcloud フォルダの `refs/` と `logs/` 内で元のファイルの横にある場合も）。いずれかの HEAD が指すブランチ（任意の空白を含む `ref:` として、またはシンボリックリンクとして）、その reflog、それを含むディレクトリは、名前にかかわらず決して脇へ移されません。したがって `report (1).pdf`、`fix.Conflict` というブランチ、`v1.0.conflicted` というタグ、`config-backup` は普通の名前です。
- サブモジュールはサブモジュールの中からチェックします。親プロジェクトからは、`.git/modules/<name>/` 内の競合コピーは元のファイルが横にある場合にのみ報告され、コピーのような名前のサブモジュール（`lib 2`）はコピーとして報告されます。
- リンクされたワークツリーの index の欠落は、メインのワークツリーではなく、そのワークツリーをチェックしたときに報告されます。
- `.git` を同期フォルダ外の保存場所へシンボリックリンクしている場合は、リンクされた作業ツリーから git とチェックを実行してください。ファイルを 1 つも含まない保存場所のディレクトリからだと、空になった index は意図的な「すべて削除」に見え、コミットはブロックされません。
- 意図的なコミットの中には `--no-verify` が必要なものがあります。ディスク上に残るファイルの追跡をやめる場合、オブジェクトを読んではならない場所ですべてのファイルを削除する場合、別のハードリンクを持つファイルの大文字小文字や Unicode 形式だけの名前変更、そしてスパースチェックアウト（cone モードかどうかにかかわらず）で、追跡中のファイルを削除・名前変更する、または型の変更によって移動する（ファイルがディレクトリになる、ディレクトリがシンボリックリンクやサブモジュールになる）コミットです。そこでは git のスパースパターンがディスク上のファイルを決めるため、意図的な削除と失われた index を区別できません。パターンは読みません。
- 同期クライアントが index と一緒にブランチの ref 自体も失った場合（新しい `packed-refs` が届く前にそのルーズファイルが削除された場合）、HEAD は新しいブランチのように見え、コミットはブロックされません。ステージ済みのファイルだけを含む新しい履歴が始まります。ディスク上のものは何も削除されず、古い履歴は reflog（`git reflog`）に残っています。
- 分割 index の場合や、オブジェクトを読んではならない場所（2.45 より古い git での部分クローン、クラウドにしかない代替ストア）では、index と HEAD をファイルごとに比較しません。検出できるのは欠落した index と空の index だけです。
- `core.splitIndex` を有効にしている場合、ファイルを 1 つも列挙しない index は認識されません（ヘッダーは分割部分しか数えないため）。そのため、そこからのコミットはブロックされません。
- フック：フックを実行した git プロセスから見つけられるのはコミットフックだけです。その他のフック（`pre-push`、`pre-applypatch`）は作業ツリーからチェックを実行する必要があり、そうでなければ拒否されます（終了コード 2）。git プロセスが見えない場合（フックを自分で実行するクライアント、プロセス一覧の取得がブロックされている場合）、チェックは現在のディレクトリにフォールバックします。このとき、git のディレクトリが作業ツリーを指し返していなければ（`--separate-git-dir`、`.git.nosync`、`--work-tree`）、ファイルのない index は常にコミットをブロックし（意図的に空にする場合は `--no-verify` を使ってください）、HEAD のファイルを 1 つも含まない現在のディレクトリは作業ツリーとしてチェックされず（`WORKTREE_GUESSED`）、別のリポジトリや git ディレクトリへ移動したフックは拒否されます（終了コード 2）。受信フック（`pre-receive`）は、サポートされたゲートではありません。
- `move-out`：すべてのツールが `.git` ファイルを理解するわけではありません。git と git を実行するツールは動作しますが、isomorphic-git（モバイル版 Obsidian Git）や `.git` をディレクトリとして開くツールは動作しません。
- `move-out`：同期フォルダの名前を変えたり移動したりすると、相対パスの `.git` ファイルが壊れます。その場合 `check` は見つからないディレクトリを示します。フォルダを元に戻すか、`.git` ファイルを手作業で git ディレクトリに向けてください。
- `move-out` では、すべてのデバイスで同期フォルダがホーム以下の同じ場所にある必要があります（iCloud Drive は常にそうです。Dropbox は、あるデバイスでは `~/Dropbox`、別のデバイスでは `~/Library/CloudStorage/Dropbox` にあることがあります）。そうでない場合、そのデバイスでは `.git` ファイルが別の場所を指し、`--attach` は拒否します（`TARGET_UNEXPECTED`）。
- 移動した git ディレクトリはホームのファイルシステム上に置かれます。別の場所に置くオプションはありません。
- `move-out --prepare` はその時点のデバイスの履歴をコピーします。準備した後は、間に新しいコミットを挟まず、すぐに移動してください。
- `move-out` と `fix` は iCloud Drive 自身のフォルダで試したもので、アプリの iCloud フォルダ（Obsidian の `iCloud~md~obsidian`）の中では試していません。

## コントリビューション

[CONTRIBUTING.md](../../CONTRIBUTING.md) を参照してください。セキュリティ上の問題は、公開 issue ではなく [SECURITY.md](../../SECURITY.md) に書かれた方法で報告してください。

## ライセンス

MIT
