<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="gitbrella-Logo: ein Regenschirm, dessen Griff ein git-Branch ist">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Findet den Schaden, den Cloud-Synchronisation an git-Repositories anrichtet,<br>
  ohne etwas zu verändern und ohne etwas herunterzuladen.
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
  <b>Deutsch</b> ·
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
  <a href="#usage">Verwendung</a> ·
  <a href="#repairing">Reparieren</a> ·
  <a href="#moving-git-out-of-the-synced-folder">.git auslagern</a> ·
  <a href="#what-it-reports">Meldungen</a> ·
  <a href="#install">Installation</a> ·
  <a href="#limits">Grenzen</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive und Syncthing synchronisieren Dateien einzeln. Git erwartet dagegen, dass sich sein `.git`-Verzeichnis als Ganzes ändert; deshalb landen Repositories in synchronisierten Ordnern bei Konfliktkopien wie `.git/index 2` oder `refs/heads/main 2`, bei verschwundenen Indizes neben zurückgelassenen `index.lock`-Dateien und bei Objekten, die nur in der Cloud existieren. Gits eigene FAQ rät davon ab, ein Repository in einem synchronisierten Ordner abzulegen, doch Voreinstellungen legen sie immer wieder dort ab: Schreibtisch und Dokumente von macOS in iCloud, die Ordnersicherung von Windows in OneDrive, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Sync-Clients kopieren .git Datei für Datei zwischen Geräten und hinterlassen Konfliktkopien wie index 2 und refs/heads/main 2, ein index.lock neben einem verlorenen Index und Objekte, die nur in der Cloud existieren">
</p>

`gitbrella check` sagt dir, meist in unter einer Sekunde, ob ein Repository diesen Schaden hat. Es ist dafür gedacht, zu laufen, bevor ein KI-Agent oder ein git-Hook das Repository anfasst. `gitbrella fix` repariert, was genau eine richtige Reparatur hat, und `gitbrella move-out` beendet den Schaden dauerhaft: Es verschiebt `.git` aus dem synchronisierten Ordner und hinterlässt eine `.git`-Datei, die darauf zeigt, sodass der Sync-Client deine Dateien trägt und git seine Historie für sich behält.

| Befehl | Was er tut |
| --- | --- |
| `gitbrella check` | meldet den Schaden; nur lesend |
| `gitbrella fix` | repariert, was genau eine richtige Antwort hat |
| `gitbrella move-out` | verschiebt `.git` dauerhaft aus dem synchronisierten Ordner |
| `gitbrella undo` | macht die letzte Reparatur oder Verschiebung rückgängig |
| `gitbrella history` | listet die auf diesem Gerät vorgenommenen Änderungen |
| `gitbrella prune-backups` | löscht alte Sicherungen von Änderungen, die nicht mehr rückgängig gemacht werden können |
| `gitbrella hook --for claude\|codex` | schützt die Tool-Aufrufe von KI-Agenten |

## Schnellstart

**1. Nachsehen.** Nur lesend, keine Downloads, kein Netzwerk:

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # Exit 0: gesund, 1: Schaden, 2: konnte nicht laufen
```

**2. Reparieren**, was genau eine richtige Antwort hat. Ohne `--apply` zeigt es nur den Plan:

```sh
gitbrella fix                 # was es reparieren würde und was es dir überlässt
gitbrella fix --apply         # reparieren; `gitbrella undo --apply` stellt alles wieder her
```

**3. Dauerhaft beenden.** `.git` aus dem synchronisierten Ordner verschieben:

```sh
gitbrella move-out            # was passieren würde und was es verhindert
gitbrella move-out --apply
```

**Geschützt halten.** Commits in einem beschädigten Repository blockieren:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Für KI-Agenten siehe [Für Agenten](#for-agents).

## Was es verspricht

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` ist nur lesend.** Es schreibt niemals in das Repository. Git läuft mit `GIT_OPTIONAL_LOCKS=0`, abgeschaltetem automatischem gc und abgeschalteter Wartung und nur mit lesenden Befehlen. End-to-End-Tests für die riskanten Fälle (Indexkopien, ein fehlender Index, partielle Klone, geteilte Indizes, verschobene git-Verzeichnisse) erfassen Größe, mtime und Inode jeder Datei vor und nach der Prüfung und verlangen, dass sie übereinstimmen.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Keine Cloud-Downloads.** Unter macOS schaltet es das Herunterladen bei Bedarf für den eigenen Prozess (und das git, das es startet) ab, bevor es die Festplatte anfasst; verweigert macOS das, läuft die Prüfung nicht (Exit 2). Dateien, die nur in iCloud existieren, werden gemeldet, nicht geholt.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Kein Netzwerk.** Partielle Klone holen niemals fehlende Objekte (`GIT_NO_LAZY_FETCH`). Mit git älter als 2.45 werden die Objektprüfungen stattdessen übersprungen.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Ruhig bei gesunden Repositories.** Ein Lock ist nur dann ein Schaden, wenn er alt ist und kein git, das am Repository arbeitet (in einem seiner Worktrees), ihn geschrieben haben kann: git legt einen Lock an, wenn es ihn nimmt, also kann ein später gestartetes git ihn nicht halten. Gits temporäre Dateien und Dateien anderer Werkzeuge in `.git` sind Information. Auf 89 gesunden, aktiv genutzten Repositories ausgeführt, meldete es keinen Schaden.

<a name="usage"></a>

## Verwendung

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Exit-Status: **0** kein Schaden, **1** Schaden gefunden, **2** die Prüfung konnte nicht laufen. Die Befehle, die etwas verändern (`move-out`, `fix`, `undo`), beenden mit **0** geplant, erledigt oder nichts zu tun (`already`); **1** abgelehnt (mit den Gründen), erledigt mit einer Warnung oder mit Schaden, der dir überlassen bleibt (die MANUAL-Liste von `fix`, die `check` weiterhin melden würde); **2** fehlgeschlagen (zurückgerollt oder angehalten; das JSON bewahrt das Wiederhergestellte und den Journaleintrag) oder konnte nicht laufen.

`--deep` führt zusätzlich ein vollständiges `git fsck` aus (es liest jedes Objekt und wird daher übersprungen, wenn Objekte nur in der Cloud existieren). `--provider` erzwingt den Sync-Client, statt ihn aus dem Pfad zu erkennen: `auto` (Standard), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage` oder `none`. Syncthing- und Nextcloud-Ordner (oder ownCloud) werden an der Markierung erkannt, die ihr Client darin ablegt (`.stfolder`, `.sync_*.db`), wo auch immer sie liegen. `fix` und `move-out` nehmen dasselbe `--provider`: gib ihnen denselben Wert wie `check`, da manche Kopienamen nur für einen bestimmten Client zählen. Das JSON sagt dann `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Ausgabe von gitbrella check: DAMAGE INDEX_COPY index 2, reparierbar mit gitbrella fix, und INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>Dieselbe Ausgabe als Text</summary>

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

### Als git-Hook

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit startet den pre-commit-Hook, der gitbrella check startet: Exit 0 lässt den Commit durch, 1 oder 2 blockiert ihn">
</p>

Ein beschädigtes Repository blockiert den Commit; ein gesundes committet wie gewohnt. Das Werkzeug weiß, dass git den Hook ausführt, und verwechselt den Lock des Commits selbst nicht mit einem veralteten.

**Welches Repository geprüft wird.** Bei Commit-Hooks (git übergibt ihnen `GIT_INDEX_FILE` oder ein absolutes `GIT_DIR`) findet es das Repository über den git-Prozess, der den Hook gestartet hat; der Hook darf also vorher das Verzeichnis wechseln: in das git-Verzeichnis, sein Hooks-Verzeichnis oder ein Verzeichnis außerhalb jedes Repositorys. Wenn git das Repository mit einem absoluten `GIT_DIR` benennt (verknüpfte Worktrees, Submodule, separate git-Verzeichnisse), wirkt jeder git-Befehl im Hook darauf, wohin der Hook auch geht, und die Prüfung ebenso. Andernfalls sieht es nach, wo git selbst vom aktuellen Verzeichnis aus landen würde (das git-Verzeichnis, das ein relatives `GIT_DIR` dort benennt, oder das von dort gefundene Repository): Ist das ein anderes Repository oder ein anderer Worktree desselben (ein Hook, der hineinwechselt, oder der Editor für die Commit-Nachricht), lehnt es ab (Exit 2), statt für das falsche zu antworten.

Ohne Pfad gestartet, prüft es das Repository, das git an den Hook exportiert hat (`GIT_DIR`); Dotfiles-Setups mit `--git-dir` und git-Verzeichnisse innerhalb ihres eigenen Worktrees funktionieren also ebenfalls. Es beachtet auch ein in deiner Shell gesetztes `GIT_DIR`, wenn kein Pfad angegeben ist; anders als git ignoriert es ein `GIT_DIR`, das nicht existiert, und lehnt einen verknüpften Worktree ab, den es nicht zurückverfolgen kann, statt das aktuelle Verzeichnis als Arbeitsverzeichnis zu nehmen.

**Der Index, den der Commit festhält.** Es liest `GIT_INDEX_FILE`, wenn es eine Datei innerhalb des git-Verzeichnisses benennt, wie git es für Commit-Hooks setzt: Fehlte der Index, den der Commit festhält, oder war er leer, oder listet er eine HEAD-Datei nicht mehr, die noch im Arbeitsverzeichnis liegt, wird der Commit blockiert, bevor er diese Dateien als gelöscht festhalten kann. Eine Indexdatei an anderer Stelle, auf die du `GIT_INDEX_FILE` selbst zeigen lässt, wird nicht gelesen: In einem Hook wird stattdessen der eigene Index des Repositorys geprüft, und eine von Hand gestartete Prüfung mit gesetzter Variable lehnt ab (Exit 2).

**Absichtliche Entfernungen.** Um eine Datei, die du auf der Platte behältst, nicht mehr zu verfolgen (`git rm --cached`, ein Verzeichnis, das du jetzt ignorierst, nicht mehr verfolgen), oder um absichtlich ein „alles löschen“ zu committen, während deine Dateien noch auf der Platte liegen (`git rm -r --cached .`), überspringe den Hook mit `git commit --no-verify`.

### Mit einem Hook-Manager

- **pre-commit** (pre-commit.com): Füge das Repository mit der Hook-ID `gitbrella` (pre-commit baut es mit Go) oder `gitbrella-system` (die Binärdatei im PATH) hinzu; siehe `.pre-commit-hooks.yaml`.
- **lefthook**: Binde `hooks/lefthook.yml` über `remotes` ein.
- **husky**: `.husky/pre-commit` mit `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, danach `overcommit --sign`.

Jeder Exit-Code ungleich null stoppt den Commit in allen von ihnen; eine Prüfung, die nicht laufen kann (Exit 2), stoppt ihn also ebenfalls. Aider committet mit `--no-verify`, sofern nicht `git-commit-verify: true` gesetzt ist, und dann läuft überhaupt kein Commit-Hook. Obsidian Git (Desktop) committet mit dem git deines Systems und führt den Hook aus; trage den Ordner mit `gitbrella` in dessen Einstellung „Additional PATH“ ein, da aus dem Dock gestartete Apps den PATH der Shell nicht erhalten. Obsidian Git auf Mobilgeräten (isomorphic-git) führt keine Hooks aus.

<a name="for-agents"></a>

### Für Agenten

`--json` gibt denselben Bericht als stabiles, versioniertes Dokument aus (`schemaVersion: 1`, `findings` ist immer eine Liste). Entscheide anhand des Exit-Status, zeige dem Benutzer die Befunde.

Die eigenen Hooks von Agenten stoppen einen Tool-Aufruf nur bei Exit 2: ein Exit 1 („Schaden gefunden“) lässt den Aufruf weiterlaufen. `gitbrella hook --for claude|codex` spricht die Sprache ihrer Hooks. Es liest das Ereignis, das der Agent über stdin übergibt:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="Der Tool-Aufruf eines Agenten läuft durch gitbrella hook: Lesen und Bearbeiten gehen weiter; in einem beschädigten Repository stoppen schreibende git-Befehle und Schreibzugriffe in .git mit Exit 2">
</p>

- **Vor einem Tool-Aufruf** (`PreToolUse`): In einem beschädigten Repository stoppen ein schreibender git-Befehl (commit, add, reset, rebase, push und alles, von dem nicht bekannt ist, dass es nur liest) und ein Schreibzugriff in `.git` den Aufruf (Exit 2, der Grund auf stderr für den Agenten). Lesen, das Bearbeiten deiner Dateien, `git restore` und `gitbrella` selbst gehen weiter, damit der Schaden untersucht und repariert werden kann.
  - *Unbrauchbare Repositories.* Wo gits Dateien nur in der Cloud existieren oder git das Repository überhaupt nicht nutzen kann (ein umbenanntes `HEAD`, eine `.git`-Datei, die ins Leere zeigt), stoppt jeder git-Befehl: er würde hängen oder auf ein umschließendes Repository wirken. Schreibzugriffe in `.git` gehen dort weiter: Diese Reparatur erfolgt von Hand innerhalb von `.git`, wie der Grund besagt (`mv '.git/HEAD 2' .git/HEAD`). Auch die Abhilfe, die der Hinweis zu `core.worktree` nennt (`git config --file … --unset core.worktree`), geht weiter.
  - *Welches Repository.* Dasjenige, das die Dateien enthält, mit denen der Aufruf arbeitet (ein Codex-Patch nennt seine eigenen; ein Pfad innerhalb von `.git` zählt für das Arbeitsverzeichnis darüber). Für einen Shell-Befehl sind es der Ordner der Sitzung und jeder Ordner, in dem seine git-Befehle laufen (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), die schreibenden zuerst, höchstens fünf; passt ein schreibender nicht mehr hinein, geht der Aufruf mit einem Hinweis durch. Jeder Ordner wird danach beurteilt, was der Aufruf dort tut: Aus einem beschädigten Sitzungsordner geht `git -C ../sound commit` weiter. Ein Ordner, der kein Repository ist, und ein Bare-Repository passieren still.
  - *Wenn die Prüfung nicht laufen kann.* Alles, was die Prüfung am Laufen hindert (einschließlich des `--timeout` von 20 Sekunden, einer einzigen Frist für den ganzen Aufruf), lässt den Aufruf mit einem Hinweis durch; `--fail-open` tut das auch dort, wo git das Repository nicht nutzen kann.
  - *Wie der Befehl gelesen wird.* Er wird gelesen, nie ausgeführt: Anführungszeichen, Kommentare, Subshells, Ersetzungen (auch in einem Heredoc ohne Anführungszeichen), Heredocs, Variablen, die er setzt, `&&`, `||`, `if`, `case` und Schleifen (nach `cd a || cd b` oder einem `cd` in einen nicht vorhandenen Ordner zählt jeder Ordner, in dem die Shell sein kann; eine `for`-Schleife wird einmal pro Wert gelesen), Wrapper (`timeout`, `sudo`, `nice`, `xargs`, `env`) und die Skripte, die einer Shell übergeben werden (`-c`, `eval`, `watch`, `source`, eine Datei, ein Heredoc, `bash < f`, eine Pipe in `sh`). Funktionen und Aliase, die der Befehl definiert, werden dort gelesen, wo sie aufgerufen werden. Ein Shell-Skript, das über seinen Pfad ausgeführt oder einer Shell übergeben wird (`./deploy.sh`, `bash deploy.sh`), wird gelesen, wenn es eine kleine Datei auf diesem Rechner ist (auch über einen Symlink, und nur einmal, selbst wenn es sich selbst ausführt). `make`, `npm run` und dergleichen werden nicht gelesen.
  - *Was als Schreibzugriff zählt.* Ein anderes Programm, das in `.git` schreibt (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` auf `.git/config`, `rm -rf` oder `find -delete` auf einen Ordner, der `.git` enthält, ein Codex-Patch über die Shell), zählt als Schreibzugriff in `.git`; `.git` wegzukopieren, mit `tar` zu packen oder zu lesen nicht. `gh pr checkout` und `git-lfs` zählen als schreibende git-Befehle.
  - *Im Zweifel.* Jedes davon zählt als schreibender git-Befehl: ein Lesen, das mehr als 5000 Schritte oder die Hälfte des `--timeout` dauert (in einem Ordner, den es nicht bestimmen kann); das Wort `git` an einer Stelle, der das Lesen nicht folgt; ein Skript, das aus einem Programm, dem das Lesen nicht folgt, in eine Shell gepipt wird (`curl … | sh`), dort, wo es läuft; ein einer Shell übergebenes Skript, das vorhanden, aber nicht lesbar ist (zu groß, nur in der Cloud), dort, wo es läuft. Wo es den Ordner, in den ein git-Befehl schreibt, nicht bestimmen kann (`cd "$(…)"`, `for d in */`), zählt der Befehl in jedem geprüften Ordner und der Aufruf erhält einen Hinweis.
- **Beim Start einer Sitzung** (`SessionStart`): Eine Zeile auf stdout, die der Agent als Kontext liest, sagt, dass das Repository in einem synchronisierten Ordner liegt (es sei denn, sein git-Verzeichnis ist bereits ausgelagert) und was beschädigt ist. Sie blockiert nie.

Der Hook verändert nichts. Claude Code, in `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex, in `.codex/hooks.json` (Codex führt einen Hook erst aus, nachdem du ihm in `/hooks` vertraut hast):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

Die Prüfung läuft einmal für jeden dieser Ordner, typischerweise 0,1 bis 0,4 Sekunden pro Ordner; ein Patch, der sich über mehrere Ordner eines Repositorys verteilt, prüft es also mehrmals; `--timeout` begrenzt den gesamten Aufruf.

<a name="repairing"></a>

## Reparieren

```sh
gitbrella fix                 # was es reparieren würde und was es dir überlässt
gitbrella fix --apply         # reparieren
gitbrella undo --apply        # alles zurücksetzen
gitbrella history             # die hier vorgenommenen Änderungen, ihre Sicherungen und Größen
gitbrella prune-backups       # welche Sicherungen kein undo mehr brauchen kann
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix zeigt den Plan; fix --apply protokolliert jeden Schritt, dann läuft die Prüfung erneut: erledigt oder zurückgerollt; undo --apply macht die letzte Änderung rückgängig">
</p>

`check` sagt unter jedem Schaden, ob `fix` ihn repariert (`repair:` und `"repair": "fix"` oder `"manual"` im JSON), und nennt `fix --provider NAME`, wenn der Prüfung einer übergeben wurde; innerhalb von Commit-Hooks fragt es nicht.

`fix` repariert nur, wo es genau eine richtige Antwort gibt und nichts überschrieben wird, was du gemeint haben könntest:

| Schaden | Reparatur |
| --- | --- |
| Konfliktkopien in `.git` neben ihrem Original (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, Dropbox- und Syncthing-Kopien sowie eine Kopie des Locks einer Ref wie `refs/heads/main 2.lock`); eine Indexkopie nur, wenn sie mit dem Index identisch oder ein gültiger Index gleich HEAD ist | in die Sicherung verschoben; ein Objekt, auf das nur die Kopie zeigte (ein Commit, ein Tag, ein Tree oder eine Datei), wird zuerst als `refs/gcd-rescue/<name>` aufbewahrt |
| ein zurückgelassenes Rebase-Verzeichnis, `desktop.ini`, `Icon` und ähnliche Dateien, die git nicht angelegt hat (es sei denn, git liest eine davon als Branch oder ihr Name ist der eines Objekts), eine Kopie von `.git` im Arbeitsverzeichnis, deren Commits das Repository besitzt | in die Sicherung verschoben |
| Index fehlt, ohne Kopie oder Lock davon irgendwo | aus HEAD erzeugt (`git read-tree HEAD`: die Dateien werden nicht angefasst); nur mit einem Commit, aus dem gebaut werden kann (fehlt auch der Commit von HEAD, wird er zuerst geholt, wie unten), und außerhalb eines Sparse-Checkouts |
| ein Index, der keine Dateien listet (ein gültiger, nicht geteilter Index außerhalb eines Sparse-Checkouts) | in die Sicherung verschoben und aus HEAD erzeugt: darin war nichts gestaged |
| ein verwaistes `index.lock`, das ein gültiger Index gleich HEAD ist | es wird zum Index |
| eine leere Branch-Datei | der Commit, den ihr Reflog zuletzt festgehalten hat |
| `.git.nosync`, das git einem Commit hinzufügen würde | die Zeile `/.git.nosync/` in `.git/info/exclude` |
| Commit von HEAD fehlt, oder eine leere lose Objektdatei (ohne Kopie daneben) | die leere Datei wird in die Sicherung verschoben und die Objekte werden erneut geholt (`git fetch --refetch`, git 2.36 oder neuer) von `--remote`, sonst vom Remote des Branches, sonst von `origin`, sonst vom einzigen Remote; die Branches und Tags des Remotes werden in einen temporären Namensraum `refs/gcd-fetch/` geholt, der danach gelöscht wird, sodass sich keine Ref ändert. Bringt das Remote das Objekt nicht zurück, wird die Reparatur zurückgerollt |
| mit `fix --deep`: Objekte, die `git fsck` als beschädigt meldet | jedes Objekt der Branches und Tags des Remotes wird auf dieselbe Weise erneut geholt; danach läuft `git fsck` erneut, und meldet es immer noch Fehler (ein Objekt, das kein Remote hat, eine beschädigte Datei, die bleibt), wird die Reparatur zurückgerollt |
| Konfliktkopien deiner eigenen Dateien (`notes 2.md`), die git nicht verfolgt und die dieselben Bytes enthalten wie die Datei, die sie kopieren | in die Sicherung verschoben (`git add -A` würde sie committen); eine Kopie, die abweicht, verfolgt wird, ein Link ist, nur in der Cloud oder in einem Submodul liegt, bleibt |
| das `gc.pid` eines anderen Rechners, älter als eine Stunde; ein `gc.log` | in die Sicherung verschoben, damit gits automatisches gc hier wieder läuft |
| Dateien von `.git` oder eines alternativen Objektspeichers, die nur in der Cloud existieren | heruntergeladen, sonst nichts: bis dahin kann die Prüfung die Objekte nicht lesen, daher wird der übrige Schaden für den nächsten `fix`-Lauf als MANUAL gelistet |

Alles andere wird als MANUAL gelistet, mit dem, was auszuführen ist. Dazu gehört alles, was Arbeit enthalten könnte, die es nirgendwo sonst gibt:
- ein beschädigter Index (git kann ihn eventuell noch lesen, samt gestagter Arbeit);
- eine Indexkopie oder ein `index.lock`, die von HEAD abweichen, sowie eine Kopie von `index.lock`;
- eine Kopie, deren Original fehlt (sie könnte die einzige sein: lege sie zurück, es sei denn, sie ist die Kopie eines Locks, die gesichert und beiseitegelegt wird);
- ein Lock, den kein git auf diesem Rechner hält (git auf einem anderen Gerät oder in einem Container könnte es; sein Hinweis nennt den Commit darin);
- eine Kopie, die mehr als 100 Objekte benennt (ein langes Reflog);
- eine Kopie des Branches, den ein HEAD benennt, seines Reflogs oder eines Verzeichnisses, das diesen Branch enthält (git liest sie, wie auch immer ihr Name aussieht).

Dazu gehört auch alles, wo fix zwischen zwei Versionen wählen müsste oder die Commits einer Kopie nicht am Leben halten könnte:
- alles im Objektspeicher: Kopien, Dateien mit dem Namen eines Objekts (prüfe, welche zu ihrem Hash passt), leere Objektdateien ohne Remote, von dem geholt werden kann;
- alles im git-Verzeichnis eines Submoduls (führe `fix` im Submodul aus);
- eine Branch-Kopie, die einen Commit benennt, der noch nicht hier ist (er könnte noch ankommen);
- eine Kopie von `.git` im Arbeitsverzeichnis mit Commits, die dem Repository fehlen (welche der beiden aktuell ist, entscheidest du);
- eine leere Datei neben einer Kopie von ihr oder ihrem Verzeichnis;
- ein defektes Verwaltungsverzeichnis eines verknüpften Worktrees (`git worktree repair`);
- ein Index, der Dateien verloren hat, die noch auf der Platte liegen (absichtliches Nicht-mehr-Verfolgen sieht genauso aus; wenn sonst nichts gestaged ist, sagt der Hinweis, dass `git read-tree HEAD` sie zurückbringt, ohne etwas zu verlieren);
- Objekte, die `git fsck` als beschädigt meldet, ohne `--deep` oder ohne Remote.

`fix` fasst reftable-Repositories überhaupt nicht an; dort sagt das `REFTABLE_TABLE_UNLISTED` von `check`, wie die Refs eines anderen Geräts erhalten bleiben. Die Sicherung muss auf der Platte des Repositorys liegen, damit das, was fix beiseitelegt, in einem Schritt verschoben wird; andernfalls lehnt `fix` ab (eine Reparatur, die nur herunterlädt, verschiebt nichts und braucht sie nicht).

Jeder Schritt wird protokolliert, bevor er ausgeführt wird, und am Ende läuft die Prüfung erneut: Wird etwas, das repariert wurde, noch gemeldet, oder ist neuer Schaden aufgetaucht, werden die Schritte rückgängig gemacht. Ein abgebrochenes fix wird vom nächsten mit `--apply` gestarteten Änderungsbefehl in diesem Repository zurückgerollt, der danach anhält; starte ihn erneut. Nach 30 Sekunden (`--watch`) prüft `fix` noch einmal: Schaden, der zurückgekehrt ist, hat der Sync-Client zurückgebracht, und die dauerhafte Abhilfe ist `move-out`.

`undo` macht zuerst die neueste Änderung rückgängig, eine Reparatur oder eine Verschiebung. Ein Schritt, dessen Ergebnis sich seit der Reparatur geändert hat, bleibt, wie er ist, und wird gelistet, nie überschrieben: git hat den neu gebauten Index umgeschrieben, der Sync-Client hat eine Kopie zurückgebracht, eine Rettungs-Ref hat sich bewegt, die Sicherung wurde gelöscht oder `.git` wurde verschoben. Eine Rettungs-Ref bleibt, solange die Kopie, die sie am Leben hält, in der Sicherung bleibt. Heruntergeladene Dateien bleiben auf der Platte und erneut geholte Objekte bleiben im Objektspeicher: keines von beiden ersetzt etwas. Der Rest wird rückgängig gemacht, und das undo wird abgeschlossen, sodass die Änderung davor als Nächstes rückgängig gemacht werden kann.

`history` listet die Änderungen, die dieses Gerät am Repository vorgenommen hat (Reparaturen, Verschiebungen, undos), die Sicherung, die jede davon behält, samt Größe, und welche `undo` als Nächstes rückgängig macht. `prune-backups` löscht, nur mit `--apply`, die Sicherungen von Änderungen, die rückgängig gemacht, zurückgerollt oder nie rückgängig zu machen waren (ein undo, ein prepare, ein attach) und deren letzter Schritt älter als `--older-than` ist (Standard `720h`). Eine Reparatur oder Verschiebung, die noch rückgängig gemacht werden kann, behält ihre Sicherung, auch unter neueren Änderungen.

<a name="moving-git-out-of-the-synced-folder"></a>

## .git aus dem synchronisierten Ordner verschieben

```sh
gitbrella move-out            # was passieren würde und was es verhindert
gitbrella move-out --apply    # ausführen
gitbrella undo --apply        # zurückverschieben
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Vorher: .git im synchronisierten Ordner. Nach move-out: eine .git-Datei im synchronisierten Ordner zeigt auf ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d außerhalb davon">
</p>

`move-out` kopiert `.git` nach `~/.local/share/gitbrella/gitdirs/<name>` (unter macOS ein APFS-Klon: kein Platzverbrauch), weist Datei für Datei nach, dass die Kopie identisch ist, verschiebt das Original in eine Sicherung unter `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`) und schreibt eine `.git`-Datei wie `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. Der Pfad ist relativ, daher schreibt jedes Gerät, das den Ordner an derselben Stelle unter seinem Home hat, dieselbe `.git`-Datei. Danach prüft es, dass git denselben HEAD, Status, dieselbe Konfiguration und dasselbe Hooks-Verzeichnis findet, und 30 Sekunden lang (`--watch`), dass der Sync-Client `.git` in Ruhe lässt und unter iCloud die neue Datei hochlädt. Das Original wird vor dem letzten Vergleich beiseitegelegt, sodass ein Schreibzugriff, der sich während des Kopierens eingeschlichen hat, bemerkt und die Verschiebung rückgängig gemacht wird. Unter iCloud lehnt `move-out` ab, solange iCloud das Repository noch hochlädt; `--wait-upload 10m` wartet stattdessen bis zu dieser Dauer.

Jede Änderung wird vor ihrer Ausführung in einem Journal festgehalten, mit Gerät und Inode von allem, was sie anlegt (und den Bytes der `.git`-Datei); ein Zurückrollen fasst nichts anderes an. Stirbt der Prozess mittendrin, schließt das nächste `move-out --apply` (oder `undo --apply`) die Änderung ab oder rollt sie zurück und hält dort an (starte den Befehl erneut, um fortzufahren); es findet sie über die Inode des Ordners, selbst unter anderer Schreibweise, nach einer Umbenennung oder innerhalb eines anderen Repositorys, und `check` meldet die unfertige Änderung in der Zwischenzeit als Schaden. Sobald git das verschobene Verzeichnis benutzt hat, wird nichts mehr darüber zurückgerollt: Enthält es etwas, das dem Original fehlt, hält der Befehl an und nennt beide. Nichts wird gelöscht: Das Original bleibt in der Sicherung, bis du es entfernst. `undo` bringt das jetzt benutzte git-Verzeichnis zurück, mit den seit der Verschiebung gemachten Commits; die Sicherung von vor der Verschiebung bleibt unberührt. Hat der Sync-Client die `.git`-Datei ausgelagert (nur ihr Name liegt auf diesem Mac), bitten `undo` und die Wiederherstellung dich, sie zuerst herunterzuladen: Sie zu lesen würde sie herunterladen, und das tut das Werkzeug nie.

Findet ein Zurückrollen sowohl das Original (in der Sicherung) als auch ein neues `.git` (angelegt, nachdem die Verschiebung angehalten hat, von dir oder vom Sync-Client), überschreibt es keines von beiden und sagt das: Vergleiche sie, verschiebe das `.git`, das du nicht willst, aus dem Ordner und starte den Befehl erneut.

Es lehnt ab (Exit 1, mit den Gründen), solange git oder etwas, das einen git-Lock hält, am Repository arbeitet, während eines Merges oder Rebases, solange Dateien von `.git` nur in der Cloud existieren, solange iCloud den Ordner noch hochlädt, solange ein Socket (ein laufender `git fsmonitor--daemon`) in `.git` liegt, wenn nach einer Verschiebung zwei git-Verzeichnisse existieren, und bei Layouts, deren Verknüpfungen brechen würden: verknüpfte Worktrees, Submodule, geliehene Objekte (`alternates`), ein `.git`, das ein Symlink oder nicht das git-Verzeichnis selbst ist. Anderer Schaden, den `check` findet, wird unverändert mitverschoben und als mitgenommen gelistet: Führe `check` danach erneut aus.

### Andere Geräte

Der Sync-Client löscht `.git` auf jedem anderen Gerät, das den Ordner synchronisiert, und bringt stattdessen die `.git`-Datei. Um deren Historie zu behalten:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: auf jedem anderen Gerät move-out --prepare --apply. 2: auf einem Gerät move-out --apply. 3: auf jedem anderen Gerät move-out --attach --apply">
</p>

1. Auf jedem anderen Gerät, vor der Verschiebung: `gitbrella move-out --prepare --apply` kopiert dessen `.git` dorthin, wohin die `.git`-Datei zeigen wird.
2. Auf einem Gerät: `gitbrella move-out --apply`.
3. Auf jedem anderen Gerät, sobald die `.git`-Datei angekommen ist: `gitbrella move-out --attach --apply`. Ein Gerät, das nicht vorbereitet wurde, klont die Historie stattdessen: `--attach --from <remote URL> --apply`. In beiden Fällen wird das Arbeitsverzeichnis nicht angefasst.

Die `.git`-Datei kommt über den Sync-Client an, daher vertraut attach ihr nur so weit, wie die Aufzeichnungen dieses Geräts reichen: Sie muss in das `~/.local/share/gitbrella/gitdirs` dieses Geräts zeigen, und ein dort bereits vorhandenes Verzeichnis wird nur verwendet, wenn `--prepare` es für dieses Repository angelegt hat.

Von da an trägt der Sync-Client die Dateien und nicht die Historie. Teile Commits über ein Remote. Auf einem Gerät, dessen Dateien der Sync-Client bereits aktualisiert hat, lehnt `git pull` ab („would be overwritten“): Verwende `git fetch` und `git reset --mixed @{u}`, das den Branch und den Index bewegt und die synchronisierten Dateien so lässt, wie sie sind.

<a name="what-it-reports"></a>

## Was es meldet

Schaden (Exit 1):

| Code | Bedeutung |
| --- | --- |
| `INDEX_COPY` | Konfliktkopie des Index (`index 2`; Namen von Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive und Seafile; Google Drives `index (1)`, OneDrives `index-COMPUTER` in einem OneDrive-Ordner sowie die Namen des älteren Nextcloud, von Synology Drive, pCloud und Resilio, die nur für gits eigene Dateinamen erkannt werden); sagt, ob es ein gültiger Index ist und ob er HEAD entspricht |
| `REF_COPY` | Konfliktkopie einer Branch- oder Tag-Ref; sagt, ob die Kopie älter, neuer, divergiert ist oder einen fehlenden Commit benennt |
| `COPY_IN_GITDIR` | jede andere Konfliktkopie in `.git` |
| `INDEX_MISSING` | der Index fehlt; git würde jede Datei als gelöscht anzeigen (in einem Commit-Hook: der Commit würde jede Datei löschen) |
| `INDEX_EMPTY` | der Index listet keine Dateien, während die Dateien von HEAD noch im Arbeitsverzeichnis liegen, wie wenn git ihn neu gebaut hat, nachdem der Sync-Client ihn gelöscht hatte; ein Commit würde jede Datei löschen |
| `INDEX_SHRUNK` | der Index listet HEAD-Dateien nicht mehr, die noch im Arbeitsverzeichnis liegen (in einem Sparse-Checkout: jede HEAD-Datei), wie wenn git ihn aus den gestagten Dateien neu gebaut hat, nachdem der Sync-Client ihn gelöscht hatte; ein Commit würde sie löschen. Umbenennungen, ein Verzeichnis, das zu einem Symlink oder zu einem in `.gitmodules` deklarierten Submodul wurde, und auch von der Platte gelöschte Dateien zählen nicht; können die Dateien von HEAD nicht gelistet werden, wird es als unverifiziert gemeldet |
| `HEAD_COMMIT_MISSING` | HEAD benennt einen Commit, der nicht im Repository ist (die Ref kam vor ihren Objekten an) |
| `ORPHAN_INDEX_LOCK` | der Index fehlt und ein altes `index.lock` blieb zurück (oft ein vollständiger Index) |
| `INDEX_INVALID` | der Index besteht seine eigene Prüfsumme nicht |
| `STALE_LOCK` | ein git-Lock, den kein laufendes git halten kann: keines arbeitet am Repository, oder alle starteten, nachdem der Lock geschrieben wurde. Nur git-Prozesse auf diesem Rechner sind sichtbar: Stelle vor dem Entfernen des Locks sicher, dass kein git das Repository in einem Container, auf einem anderen Rechner, der den Ordner teilt, oder in einer App mit eingebettetem git benutzt |
| `DATALESS_IN_GITDIR` | Dateien in `.git` existieren nur in der Cloud; Objektprüfungen wurden übersprungen |
| `FOREIGN_FILE_IN_GITDIR` | eine Datei, die git nicht angelegt hat, in `refs/` oder `objects/` (`desktop.ini`, `.tmp`-Dateien in `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | ein Objekt oder eine Ref mit null Bytes |
| `GITDIR_COPY` | eine Kopie des git-Verzeichnisses im Arbeitsverzeichnis (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | das git-Verzeichnis liegt unter einem anderen Namen im Arbeitsverzeichnis (`.git.nosync`) und `git add -A` würde es committen |
| `WORKTREE_ADMIN_BROKEN` | ein verknüpfter Worktree hat seine Verwaltungsdateien verloren |
| `STALE_OPERATION_STATE` | ein zurückgelassenes Rebase-Verzeichnis lässt git glauben, dass ein Rebase läuft |
| `FSCK_ERRORS` | nur mit `--deep`: `git fsck` meldete Fehler (wenn es nicht fertig wird, ist derselbe Code Information) |
| `REFTABLE_TABLE_UNLISTED` | ein reftable-Repository enthält Ref-Tabellen, die seine Liste nicht nennt: die eines anderen Geräts, vom Sync-Client hereingetragen, während dessen Liste zu einer Kopie wurde, die git nie liest. Das nächste `git pack-refs` oder die nächste Wartung löscht sie und mit ihnen die Branches dieses Geräts; kopiere zuerst das git-Verzeichnis an einen sicheren Ort |
| `CHANGE_UNFINISHED` | ein `fix`, `undo` oder `move-out` von gitbrella wurde nicht abgeschlossen (es wurde abgebrochen); führe den genannten Befehl mit `--apply` aus, um es abzuschließen oder zurückzurollen, bevor du git dort benutzt |
| `GIT_DIR_RETURNED` | nach `move-out` ist `.git` wieder ein Verzeichnis (der Sync-Client hat das alte zurückgebracht), es gibt also zwei git-Verzeichnisse; vergleiche sie und behalte eines, bevor du committest |
| `STAGED_CONFLICT_COPY` | der Index fügt eine Sync-Konfliktkopie einer Datei hinzu, die er ebenfalls listet (`notes 2.md` neben `notes.md`, ein kopiertes Verzeichnis `src 2/`, Syncthing-Namen); ein Commit würde die Kopie in die Historie bringen. Nur die Namen entscheiden; eine Umbenennung in einen solchen Namen und eine wie eine Kopie benannte Datei ohne ihr Original zählen nicht; unter einem geteilten Index nicht geprüft |

Information (Exit-Status unverändert): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (das git-Verzeichnis wurde aus dem synchronisierten Ordner verschoben: das sichere Layout), `WORKTREE_COPIES` (Konfliktkopien deiner eigenen Dateien), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (ein alternativer Objektspeicher enthält Dateien nur in der Cloud, daher wurden Objektprüfungen übersprungen), `GC_PID_FOREIGN` (das `gc.pid` eines anderen Rechners: git überspringt sein automatisches gc für 12 Stunden), `GC_LOG_FRESH` (ein `gc.log`, das weniger als einen Tag alt ist, stoppt das automatische gc), `HOOKS_IN_SYNCED_FOLDER` (`core.hooksPath` oder ein `core.fsmonitor`-Befehl liegt in einem synchronisierten Ordner außerhalb des git-Verzeichnisses, sodass eine Änderung auf einem anderen Gerät hier ausgeführt wird), `WORKTREE_GUESSED` (`GIT_DIR` ohne Arbeitsverzeichnis, und keine der Dateien von HEAD liegt im aktuellen Verzeichnis: das Arbeitsverzeichnis wurde nicht geprüft).

Ist der Pfad gar kein Repository, sagt es warum, wenn die Ursache auf der Platte liegt: ein umbenanntes `.git 2`, gits eigene Dateien nur in der Cloud, ein git-Verzeichnis ohne HEAD, eine `.git`-Datei, die auf ein verschwundenes git-Verzeichnis zeigt, oder ein `core.worktree`, das auf einen Ordner zeigt, den dieser Rechner nicht hat (die Einstellung eines anderen Rechners, die der Sync-Client hereingetragen hat). Hat der Pfad ein `.git`, das git nicht nutzen kann, hält es dort an (Exit 2), statt für ein umschließendes Repository zu antworten, was git selbst tun würde.

<a name="install"></a>

## Installation

Erfordert git 2.31 oder neuer. macOS und Linux, amd64 und arm64, kein cgo.

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

oder lade ein Archiv von der Releases-Seite.

### Ein Release verifizieren

Jedes Release enthält `checksums.txt` (die SHA-256 jedes Archivs und jeder SBOM), dessen schlüssellose cosign-Signatur `checksums.txt.sigstore.json`, erstellt vom Release-Workflow, eine SPDX-SBOM für jedes Archiv und eine GitHub-Build-Provenance-Attestation. Prüfe zuerst die Signatur, dann das Archiv gegen die signierte Liste (das Beispiel ist v1.2.3 für Apple Silicon). Verwende cosign v3.1.3 oder neuer (v2.6.5 auf der v2-Linie): ältere Versionen lassen sich durch ein gefälschtes Bundle täuschen (GHSA-fx35-mq7g-6g98).

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

Ein Release wird von `.github/workflows/release.yml` aus einem gepushten Tag mit `.goreleaser.yaml` gebaut und bleibt ein Entwurf, bis eine Person es veröffentlicht. Die Binärdateien werden mit `-trimpath` und der Zeit des Commits gebaut, sodass derselbe Tag und dasselbe Go-Release (der Workflow nimmt den neuesten 1.26-Patch) dieselben Binärdateien bauen; die Archive und SBOMs um sie herum tragen ihre Build-Zeit.

<a name="limits"></a>

## Grenzen

- `fix` repariert nur, wo es genau eine richtige Antwort gibt, und überlässt dir den Rest mit dem auszuführenden Befehl; die dauerhafte Lösung ist `move-out`, das `.git` außerhalb des synchronisierten Ordners hält.
- Konfliktkopien, die iCloud selbst zwischen zwei Geräten erzeugt, wurden in Tests nicht nachgestellt; die Erkennung wird mit Kopien getestet, die derselben Benennung folgen.
- Codex übergibt das `workdir` seines Shell-Tools nicht an Hooks: Ein Befehl, den Codex in einem anderen Ordner ausführt, wird im Ordner der Sitzung geprüft.
- Windows und OneDrive unter Windows sind ungetestet. reftable-Repositories erhalten die Prüfungen auf Dateiebene und die Prüfung auf nicht gelistete Tabellen; Ref-Kopien werden dort nicht mit ihren Branches verglichen.
- Kopienamen, deren Kennzeichen ein Name absichtlich tragen könnte, werden nur dort erkannt, wo git den Namen selbst erzeugt, nie im Arbeitsverzeichnis: Google Drives `index (1)` für gits eigene Dateien und in `refs/`; OneDrives `index-COMPUTER` für gits eigene Dateien auf oberster Ebene eines OneDrive-Ordners, mit einem Großbuchstaben im Gerätenamen und ohne ein Wort, mit dem man den Namen einer Sicherung beendet (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` gehören dir), mit höchstens 15 Zeichen (ein längerer Mac-Name wird übersehen); das `_conflict-YYYYMMDD-HHMMSS` des älteren Nextcloud, das von Synology Drive, pClouds `.conflicted` und Resilios `.Conflict` für gits eigene Dateien auf oberster Ebene (das des älteren Nextcloud auch in `refs/` und `logs/` eines Nextcloud-Ordners, neben seinem Original). Ein Branch, den ein HEAD benennt (als `ref:` mit beliebigen Leerzeichen oder als Symlink), sein Reflog und ein Verzeichnis, das ihn enthält, werden nie beiseitegelegt, wie auch immer sie heißen. Daher sind `report (1).pdf`, ein Branch `fix.Conflict`, ein Tag `v1.0.conflicted` oder `config-backup` gewöhnliche Namen.
- Submodule werden von innerhalb des Submoduls geprüft. Vom übergeordneten Projekt aus wird eine Konfliktkopie in `.git/modules/<name>/` nur gemeldet, wenn ihr Original daneben liegt, und ein wie eine Kopie benanntes Submodul (`lib 2`) wird als Kopie gemeldet.
- Der fehlende Index eines verknüpften Worktrees wird gemeldet, wenn dieser Worktree geprüft wird, nicht wenn der Haupt-Worktree geprüft wird.
- Ist `.git` per Symlink mit einem Speicher außerhalb des synchronisierten Ordners verbunden, führe git und die Prüfung vom verknüpften Arbeitsverzeichnis aus. Vom Speicherverzeichnis aus, das keine der Dateien enthält, sieht ein geleerter Index wie ein absichtliches „alles löschen“ aus und blockiert den Commit nicht.
- Manche absichtlichen Commits brauchen `--no-verify`: das Nicht-mehr-Verfolgen von Dateien, die auf der Platte bleiben, das Löschen aller Dateien dort, wo Objekte nicht gelesen werden dürfen, eine Umbenennung nur in der Groß-/Kleinschreibung oder nur in der Unicode-Form einer Datei, die einen weiteren Hardlink hat, und in einem Sparse-Checkout (im Cone-Modus oder nicht) jeder Commit, der eine verfolgte Datei entfernt oder umbenennt oder sie durch eine Typänderung verschiebt (eine Datei, die zu einem Verzeichnis wird, ein Verzeichnis, das zu einem Symlink oder Submodul wird). Dort entscheiden gits Sparse-Muster, welche Dateien auf der Platte liegen, sodass eine absichtliche Entfernung nicht von einem verlorenen Index zu unterscheiden ist; die Muster werden nicht gelesen.
- Verliert der Sync-Client zusammen mit dem Index auch die Branch-Ref selbst (ihre lose Datei wird gelöscht, bevor ein neues `packed-refs` ankommt), sieht HEAD wie ein neuer Branch aus und der Commit wird nicht blockiert: Er beginnt eine neue Historie, die nur die gestagten Dateien enthält. Nichts auf der Platte wird gelöscht, und die alte Historie ist noch im Reflog (`git reflog`).
- Unter einem geteilten Index oder dort, wo Objekte nicht gelesen werden dürfen (partieller Klon mit git älter als 2.45, Alternates nur in der Cloud), wird der Index nicht Datei für Datei mit HEAD verglichen: nur ein fehlender oder leerer Index wird erkannt.
- Mit `core.splitIndex` wird ein Index, der keine Dateien listet, nicht erkannt (sein Header zählt nur den geteilten Teil), daher wird ein Commit daraus nicht blockiert.
- Hooks: Nur Commit-Hooks werden über den git-Prozess gefunden, der sie gestartet hat. Andere Hooks (`pre-push`, `pre-applypatch`) müssen die Prüfung vom Arbeitsverzeichnis aus starten, sonst werden sie abgelehnt (Exit 2). Wo der git-Prozess nicht sichtbar ist (ein Client, der Hooks selbst ausführt, oder blockierte Prozessauflistung), fällt die Prüfung auf das aktuelle Verzeichnis zurück: Zeigt gits Verzeichnis dann nicht zurück auf das Arbeitsverzeichnis (`--separate-git-dir`, `.git.nosync`, `--work-tree`), blockiert ein Index ohne Dateien den Commit immer (verwende `--no-verify` für ein absichtliches Leeren), ein aktuelles Verzeichnis ohne eine der Dateien von HEAD wird nicht als Arbeitsverzeichnis geprüft (`WORKTREE_GUESSED`), und ein Hook, der in ein anderes Repository oder in das git-Verzeichnis gewechselt ist, wird abgelehnt (Exit 2). Receive-Hooks (`pre-receive`) sind keine unterstützte Schranke.
- `move-out`: Nicht jedes Werkzeug versteht eine `.git`-Datei. Git und Werkzeuge, die git ausführen, funktionieren; isomorphic-git (Obsidian Git auf Mobilgeräten) und Werkzeuge, die `.git` als Verzeichnis öffnen, nicht.
- `move-out`: Das Umbenennen oder Verschieben des synchronisierten Ordners bricht die relative `.git`-Datei. `check` nennt dann das fehlende Verzeichnis; stelle den Ordner zurück oder lass die `.git`-Datei von Hand auf das git-Verzeichnis zeigen.
- `move-out` braucht den synchronisierten Ordner auf jedem Gerät an derselben Stelle unter dem Home (iCloud Drive liegt immer so; Dropbox kann auf einem Gerät unter `~/Dropbox` und auf einem anderen unter `~/Library/CloudStorage/Dropbox` liegen). Wo das nicht so ist, zeigt die `.git`-Datei dort woandershin und `--attach` lehnt ab (`TARGET_UNEXPECTED`).
- Das verschobene git-Verzeichnis bleibt auf dem Dateisystem des Home; es gibt keine Option, es woanders abzulegen.
- `move-out --prepare` kopiert die Historie des Geräts zu diesem Zeitpunkt: Verschiebe bald nach dem Vorbereiten, ohne neue Commits dazwischen.
- `move-out` und `fix` wurden im eigenen Ordner von iCloud Drive ausprobiert, nicht innerhalb des iCloud-Ordners einer App (Obsidians `iCloud~md~obsidian`).

## Mitwirken

Siehe [CONTRIBUTING.md](../../CONTRIBUTING.md). Melde Sicherheitsprobleme wie in [SECURITY.md](../../SECURITY.md) beschrieben, nicht in einem öffentlichen Issue.

## Lizenz

MIT
