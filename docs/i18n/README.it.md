<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="Logo di gitbrella: un ombrello il cui manico è un ramo git">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Trova i danni che la sincronizzazione cloud ha fatto a un repository git.<br>
  Non cambia nulla. Non scarica nulla.
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
  <b>Italiano</b> ·
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
  <a href="#usage">Uso</a> ·
  <a href="#repairing">Riparazione</a> ·
  <a href="#moving-git-out-of-the-synced-folder">Spostare .git</a> ·
  <a href="#what-it-reports">Cosa segnala</a> ·
  <a href="#install">Installazione</a> ·
  <a href="#limits">Limiti</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive e Syncthing sincronizzano i file uno alla volta. Git invece presume che la directory `.git` cambi come un tutt'uno, così un repository dentro una cartella sincronizzata accumula copie di conflitto come `.git/index 2` e `refs/heads/main 2`, un index sparito accanto a un `index.lock` dimenticato e oggetti che esistono solo nel cloud. La FAQ di git stesso sconsiglia di tenere repository in cartelle sincronizzate, ma le impostazioni predefinite continuano a metterli lì: Scrivania e Documenti di iCloud su macOS, il backup delle cartelle di OneDrive su Windows, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Il client di sincronizzazione copia .git tra i dispositivi file per file e lascia copie di conflitto come index 2 e refs/heads/main 2, un index.lock accanto a un index mancante e oggetti presenti solo nel cloud">
</p>

`gitbrella check` ti dice, di solito in meno di un secondo, se un repository ha quei danni. È pensato per essere eseguito prima che un agente IA o un hook di git tocchi il repository. `gitbrella fix` ripara ciò che ha un solo modo corretto di essere riparato, e `gitbrella move-out` elimina il danno alla radice: sposta `.git` fuori dalla cartella sincronizzata e lascia un file `.git` che punta lì, così il client di sincronizzazione trasporta i tuoi file e git tiene la cronologia per sé.

| Comando | Cosa fa |
| --- | --- |
| `gitbrella check` | segnala i danni (sola lettura) |
| `gitbrella fix` | ripara ciò che ha una sola risposta corretta |
| `gitbrella move-out` | sposta `.git` fuori dalla cartella sincronizzata, per sempre |
| `gitbrella undo` | annulla l'ultima riparazione o l'ultimo spostamento |
| `gitbrella history` | elenca le modifiche fatte su questo dispositivo |
| `gitbrella prune-backups` | elimina i vecchi backup di modifiche che non si possono più annullare |
| `gitbrella hook --for claude\|codex` | protegge le chiamate agli strumenti di un agente IA |

## Avvio rapido

**1. Controlla.** Sola lettura, nessun download, nessuna rete.

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # uscita 0: sano, 1: danni, 2: impossibile eseguire
```

**2. Ripara.** Solo ciò che ha una sola risposta corretta. Senza `--apply` mostra solo il piano.

```sh
gitbrella fix                 # cosa riparerà e cosa lascerà a te
gitbrella fix --apply         # ripara; `gitbrella undo --apply` rimette tutto com'era
```

**3. Risolvi alla radice.** Sposta `.git` fuori dalla cartella sincronizzata.

```sh
gitbrella move-out            # cosa succede e cosa lo impedirebbe
gitbrella move-out --apply
```

**Resta protetto.** Blocca i commit in un repository danneggiato:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Per gli agenti IA, vedi [Per gli agenti](#for-agents).

## Cosa promette

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` è in sola lettura.** Non scrive mai nel repository. Git esegue solo comandi di lettura, con `GIT_OPTIONAL_LOCKS=0` e con gc automatico e manutenzione disattivati. I test end-to-end per i casi rischiosi (copia dell'index, index mancante, clone parziale, index diviso, directory git spostata) registrano dimensione, mtime e inode di ogni file prima e dopo il controllo e li confrontano.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Non scarica nulla dal cloud.** Su macOS disattiva il download su richiesta per il proprio processo (e per il git che avvia) prima di toccare il disco; se macOS rifiuta, il controllo non viene eseguito (uscita 2). Un file presente solo in iCloud viene segnalato, non recuperato.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Non usa la rete.** Un clone parziale non recupera mai gli oggetti mancanti (`GIT_NO_LAZY_FETCH`); con git precedente alla 2.45 i controlli sugli oggetti vengono invece saltati.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Silenzioso sui repository sani.** Un lock conta come danno solo se è vecchio e nessun git in esecuzione su quel repository (su uno qualsiasi dei suoi worktree) può averlo scritto: git crea il lock quando lo acquisisce, quindi un git avviato dopo non può detenerlo. I file temporanei di git e i file di altri strumenti dentro `.git` sono informativi. Eseguito su 89 repository sani in uso, non ha segnalato alcun danno.

<a name="usage"></a>

## Uso

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Codici di uscita: **0** nessun danno, **1** danni trovati, **2** il controllo non è stato eseguito. Per i comandi che modificano qualcosa (`move-out`, `fix`, `undo`): **0** pianificato, completato o niente da fare (`already`), **1** rifiutato (con i motivi), completato con avvisi o con danni lasciati a te (l'elenco MANUAL di `fix`; `check` continua a segnalarli), **2** fallito (annullato o fermato; il JSON conserva ciò che è stato recuperato e le voci del journal) o impossibile eseguire.

`--deep` esegue anche un `git fsck` completo (legge ogni oggetto, quindi viene saltato se gli oggetti sono solo nel cloud). `--provider` impone il client di sincronizzazione invece di rilevarlo dal percorso: `auto` (predefinito), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage`, `none`. Le cartelle Syncthing e Nextcloud (o ownCloud) vengono riconosciute ovunque si trovino grazie al marcatore che il client vi mette dentro (`.stfolder`, `.sync_*.db`). `fix` e `move-out` accettano lo stesso `--provider`; passa lo stesso valore usato con `check`, perché alcuni nomi di copia contano solo per un determinato client. Il JSON mostra allora `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Output di gitbrella check: DAMAGE INDEX_COPY index 2, che gitbrella fix ripara, e INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>Lo stesso output in testo</summary>

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

### Come hook di git

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit esegue l'hook pre-commit, che esegue gitbrella check: con uscita 0 il commit procede, con 1 o 2 viene bloccato">
</p>

Un commit in un repository danneggiato viene bloccato; in uno sano procede normalmente. Lo strumento sa che è git a eseguire l'hook, quindi non scambia il lock del commit stesso per uno vecchio.

**Quale repository controlla.** Negli hook di commit (git passa loro `GIT_INDEX_FILE` o un `GIT_DIR` assoluto) il repository viene trovato a partire dal processo git che ha eseguito l'hook, quindi l'hook può prima cambiare directory: nella directory git, nella sua directory hooks o in una directory esterna a qualsiasi repository. Quando git indica il repository con un `GIT_DIR` assoluto (worktree collegato, submodule, directory git separata), ogni comando git nell'hook agisce su di esso ovunque l'hook vada, e così fa il controllo. Altrimenti verifica dove porta la directory corrente per git stesso (la directory git a cui punta lì un `GIT_DIR` relativo, o il repository trovato da lì); se si tratta di un altro repository o di un altro worktree dello stesso repository (un hook che si sposta lì, o l'editor del messaggio di commit), rifiuta (uscita 2) invece di rispondere per il repository sbagliato.

Senza percorso controlla il repository che git ha esportato all'hook (`GIT_DIR`), quindi funzionano le configurazioni di dotfiles con `--git-dir` e una directory git dentro il proprio worktree. Senza percorso segue anche un `GIT_DIR` impostato nella shell; a differenza di git, ignora un `GIT_DIR` inesistente e rifiuta un worktree collegato che non si può seguire, invece di considerare la directory corrente come worktree.

**L'index che il commit registra.** Quando `GIT_INDEX_FILE` punta a un file dentro la directory git (così git lo imposta per gli hook di commit), viene letto quello. Se l'index che il commit registrerà manca, è vuoto o non elenca più file di HEAD ancora presenti nel worktree, il commit viene bloccato prima che registri quei file come eliminati. Un file index altrove, indicato da te con `GIT_INDEX_FILE`, non viene letto: dentro un hook viene controllato invece l'index del repository, e un controllo manuale con quella variabile impostata viene rifiutato (uscita 2).

**Eliminazioni intenzionali.** Per smettere di tracciare file che restano su disco (`git rm --cached`, smettere di tracciare una directory ora ignorata), o per fare intenzionalmente il commit di un "elimina tutto" mentre i file sono ancora su disco (`git rm -r --cached .`), salta l'hook con `git commit --no-verify`.

### Con un gestore di hook

- **pre-commit** (pre-commit.com): aggiungi questo repository con l'id di hook `gitbrella` (pre-commit lo compila con Go) o `gitbrella-system` (il binario nel PATH); vedi `.pre-commit-hooks.yaml`.
- **lefthook**: importa `hooks/lefthook.yml` tramite `remotes`.
- **husky**: un `.husky/pre-commit` contenente `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, poi `overcommit --sign`.

In tutti, qualsiasi uscita diversa da zero ferma il commit, quindi anche un controllo che non può essere eseguito (uscita 2) lo ferma. Aider fa commit con `--no-verify` a meno che non sia impostato `git-commit-verify: true`, e in quel caso non viene eseguito alcun hook di commit. Obsidian Git (desktop) fa commit con il git del tuo sistema ed esegue l'hook; aggiungi la cartella che contiene `gitbrella` alla sua impostazione "Additional PATH", perché le app avviate dal Dock non ricevono il PATH della shell. Obsidian Git su mobile (isomorphic-git) non esegue hook.

<a name="for-agents"></a>

### Per gli agenti

`--json` stampa lo stesso report come documento stabile e versionato (`schemaVersion: 1`, `findings` è sempre una lista). Decidi in base al codice di uscita e mostra i risultati all'utente.

Gli hook degli agenti fermano una chiamata a uno strumento solo con l'uscita 2: l'uscita 1 ("danni trovati") lascia procedere la chiamata. `gitbrella hook --for claude|codex` parla il linguaggio dei loro hook. Legge l'evento che l'agente passa su stdin:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="La chiamata a uno strumento dell'agente passa per gitbrella hook: letture e modifiche procedono; in un repository danneggiato le scritture git e le scritture in .git si fermano con uscita 2">
</p>

- **Prima di una chiamata a uno strumento** (`PreToolUse`): in un repository danneggiato, un comando git che scrive (commit, add, reset, rebase, push e tutto ciò che non si sa essere di sola lettura) e una scrittura dentro `.git` fermano la chiamata (uscita 2, motivo su stderr per l'agente). Le letture, la modifica dei tuoi file, `git restore` e `gitbrella` stesso procedono, così il danno può essere esaminato e riparato.
  - *Repository inutilizzabili.* Dove i file di git sono solo nel cloud, o git non riesce affatto a usare il repository (un `HEAD` rinominato, un file `.git` che non punta da nessuna parte), ogni comando git si ferma: si bloccherebbe o agirebbe su un repository esterno. Lì le scritture dentro `.git` procedono: quella riparazione si fa a mano dentro `.git`, come dice il motivo (`mv '.git/HEAD 2' .git/HEAD`). Procede anche il rimedio indicato dal suggerimento di `core.worktree` (`git config --file … --unset core.worktree`).
  - *Quale repository.* Quello che contiene i file su cui lavora la chiamata (una patch di Codex nomina i propri file; un percorso dentro `.git` conta per il worktree sopra di esso). Per un comando di shell, la cartella della sessione e ogni cartella in cui girano i suoi comandi git (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), prima quelle che scrivono, al massimo cinque; quando una che scrive non ci sta, la chiamata passa con un avviso. Ogni cartella è giudicata da ciò che la chiamata vi fa: `git -C ../sound commit` da una cartella di sessione danneggiata procede. Una cartella che non è un repository e un repository bare passano in silenzio.
  - *Quando il controllo non può essere eseguito.* Qualsiasi cosa impedisca l'esecuzione del controllo (incluso il `--timeout` di 20 secondi, un'unica scadenza per tutta la chiamata) lascia passare la chiamata con un avviso; `--fail-open` fa lo stesso dove git non può usare il repository.
  - *Come viene letto il comando.* Viene letto, mai eseguito: virgolette, commenti, subshell, sostituzioni (anche dentro un heredoc senza virgolette), heredoc, le variabili che imposta, `&&`, `||`, `if`, `case` e i cicli (dopo `cd a || cd b` o un `cd` verso una cartella inesistente conta ogni cartella in cui la shell potrebbe trovarsi; un ciclo `for` viene letto una volta per ogni valore), i wrapper (`timeout`, `sudo`, `nice`, `xargs`, `env`) e gli script passati a una shell (`-c`, `eval`, `watch`, `source`, un file, un heredoc, `bash < f`, una pipe verso `sh`). Funzioni e alias definiti dal comando vengono letti dove vengono chiamati. Uno script di shell eseguito tramite il suo percorso o passato a una shell (`./deploy.sh`, `bash deploy.sh`) viene letto se è un piccolo file su questa macchina (anche tramite un symlink; una sola volta, anche se esegue se stesso); `make`, `npm run` e simili non vengono letti.
  - *Cosa conta come scrittura.* La scrittura di un altro programma dentro `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` su `.git/config`, `rm -rf` o `find -delete` su una cartella che contiene `.git`, una patch di Codex tramite la shell) conta come scrittura in `.git`; copiare `.git` altrove, archiviarlo con `tar` o leggerlo no. `gh pr checkout` e `git-lfs` contano come comandi git che scrivono.
  - *Nel dubbio.* Ciascuno dei seguenti conta come comando git che scrive: una lettura più lunga di 5000 passi o di metà del `--timeout` (nella cartella che non è riuscito a determinare); la parola `git` in un punto che la lettura non segue; uno script passato in pipe a una shell da un programma che la lettura non segue (`curl … | sh`), dove viene eseguito; uno script passato a una shell che esiste ma non si può leggere (troppo grande, solo nel cloud), dove viene eseguito. Quando non riesce a determinare in quale cartella scrive un comando git (`cd "$(…)"`, `for d in */`), il comando conta in ogni cartella controllata e la chiamata riceve un avviso.
- **All'inizio della sessione** (`SessionStart`): una riga su stdout, che l'agente legge come contesto, dice che il repository è in una cartella sincronizzata (a meno che la sua directory git non sia già fuori) e cosa è danneggiato. Non blocca mai.

L'hook non cambia nulla. Claude Code, in `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex, in `.codex/hooks.json` (Codex esegue un hook solo dopo che gli hai dato fiducia in `/hooks`):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

Il controllo viene eseguito una volta per ciascuna di queste cartelle, di solito 0,1–0,4 secondi ciascuna, quindi una patch distribuita su più cartelle di un repository controlla quel repository più volte; `--timeout` limita l'intera chiamata.

<a name="repairing"></a>

## Riparazione

```sh
gitbrella fix                 # cosa riparerà e cosa lascerà a te
gitbrella fix --apply         # ripara
gitbrella undo --apply        # rimette tutto com'era
gitbrella history             # modifiche fatte qui, i loro backup e dimensioni
gitbrella prune-backups       # backup che nessun undo userà più
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix mostra il piano; fix --apply registra ogni passo, poi il controllo viene rieseguito: completato o annullato; undo --apply annulla l'ultima modifica">
</p>

`check` indica sotto ogni danno se `fix` lo ripara (`repair:`, e `"repair": "fix"` o `"manual"` nel JSON), e nomina `fix --provider NAME` quando al controllo è stato passato un provider. Dentro gli hook di commit non chiede nulla.

`fix` ripara solo dove c'è una sola risposta corretta e niente di ciò che potresti aver voluto viene sovrascritto:

| Danno | Riparazione |
| --- | --- |
| copie di conflitto in `.git` accanto al loro originale (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, copie di Dropbox e Syncthing, e la copia del lock di un ref come `refs/heads/main 2.lock`); una copia dell'index solo se identica all'index o se è un index valido uguale a HEAD | spostate in un backup; un oggetto (commit, tag, tree o file) a cui punta solo la copia viene prima conservato come `refs/gcd-rescue/<name>` |
| una directory di rebase rimasta, `desktop.ini`, `Icon` e file simili non creati da git (a meno che git non ne legga uno come ramo o il suo nome sia quello di un oggetto), una copia di `.git` nel worktree i cui commit sono già nel repository | spostati in un backup |
| un index mancante senza copie né lock da nessuna parte | creato da HEAD (`git read-tree HEAD`: i file non vengono toccati), solo se esiste un commit da cui costruirlo (se manca anche il commit di HEAD, viene prima recuperato come descritto sotto) e fuori da uno sparse checkout |
| un index che non elenca alcun file (un index valido, non diviso, fuori da uno sparse checkout) | spostato in un backup e creato da HEAD: non conteneva nulla di preparato per il commit |
| un `index.lock` orfano che è un index valido uguale a HEAD | diventa l'index |
| un file di ramo vuoto | il commit registrato per ultimo dal suo reflog |
| `.git.nosync`, che git aggiungerebbe a un commit | la riga `/.git.nosync/` in `.git/info/exclude` |
| il commit di HEAD mancante, o un file di oggetto loose vuoto (senza una copia accanto) | il file vuoto va in un backup e gli oggetti vengono recuperati di nuovo (`git fetch --refetch`, git 2.36 o successivo) da `--remote`, altrimenti dal remote del ramo, altrimenti da `origin`, altrimenti dall'unico remote; rami e tag del remote vengono recuperati in uno spazio dei nomi temporaneo `refs/gcd-fetch/` che poi viene eliminato, quindi nessun ref cambia. Se il remote non restituisce l'oggetto, la riparazione viene annullata |
| con `fix --deep`: oggetti che `git fsck` ha segnalato come danneggiati | ogni oggetto dei rami e dei tag del remote viene recuperato di nuovo allo stesso modo; poi `git fsck` viene rieseguito, e se segnala ancora errori (un oggetto che nessun remote ha, un file corrotto rimasto) la riparazione viene annullata |
| copie di conflitto dei tuoi file (`notes 2.md`) che git non traccia e che hanno gli stessi byte del file da cui sono state copiate | spostate in un backup (`git add -A` ne farebbe il commit); una copia diversa, tracciata, che è un link, presente solo nel cloud o dentro un submodule resta |
| il `gc.pid` di un'altra macchina più vecchio di un'ora; un `gc.log` | spostati in un backup, così il gc automatico di git torna a funzionare qui |
| file di `.git` o di un archivio di oggetti alternativo presenti solo nel cloud | scaricati, e nient'altro: fino ad allora il controllo non può leggere gli oggetti, quindi il resto dei danni viene elencato come MANUAL per la prossima esecuzione di `fix` |

Tutto il resto viene elencato come MANUAL, con il comando da eseguire. Questo include tutto ciò che potrebbe contenere lavoro che non esiste altrove:
- un index danneggiato (git potrebbe ancora riuscire a leggerlo, insieme al lavoro preparato);
- una copia dell'index o un `index.lock` diversi da HEAD, e una copia di `index.lock`;
- una copia il cui originale manca (potrebbe essere l'unica: rimettila al suo posto; se è la copia di un lock, viene conservata e messa da parte);
- un lock che nessun git su questa macchina detiene (potrebbe detenerlo un git su un altro dispositivo o in un container; il consiglio nomina il commit al suo interno);
- una copia che nomina più di 100 oggetti (un reflog lungo);
- una copia del ramo nominato da un qualsiasi HEAD, del suo reflog o di una directory che lo contiene (git la legge, qualunque aspetto abbia il nome).

E tutto ciò in cui fix dovrebbe scegliere tra due versioni o non può mantenere vivi i commit di una copia:
- tutto nell'archivio degli oggetti: copie, file con il nome di un oggetto (controlla quale corrisponde al suo hash), file di oggetto vuoti senza un remote da cui recuperarli;
- tutto nella directory git di un submodule (esegui `fix` nel submodule);
- una copia di ramo che nomina un commit non ancora arrivato qui (potrebbe ancora arrivare);
- una copia di `.git` nel worktree con commit che mancano al repository (decidi tu quale è quella attuale);
- un file vuoto con accanto una copia di se stesso o della sua directory;
- una directory di amministrazione di un worktree collegato rotta (`git worktree repair`);
- un index che ha perso file ancora presenti su disco (smettere di tracciarli intenzionalmente sembra uguale; quando nient'altro è preparato, il consiglio dice che `git read-tree HEAD` li recupera senza perdere nulla);
- senza `--deep` o senza un remote, oggetti che `git fsck` ha segnalato come danneggiati.

`fix` non tocca affatto i repository reftable; lì il risultato `REFTABLE_TABLE_UNLISTED` di `check` dice come conservare i ref di un altro dispositivo. Il backup deve trovarsi sul disco del repository, così ciò che fix mette da parte si sposta in un solo passo; altrimenti `fix` rifiuta (una riparazione che consiste solo in un download non sposta nulla e non ne ha bisogno).

Ogni passo viene registrato prima di essere eseguito, e alla fine il controllo viene rieseguito: se qualcosa che ha riparato viene ancora segnalato o compare un nuovo danno, i passi vengono annullati. Un fix interrotto viene annullato dal successivo comando di modifica eseguito con `--apply` in quel repository, che poi si ferma; eseguilo di nuovo. Dopo 30 secondi (`--watch`) `fix` controlla un'altra volta: il danno che ritorna l'ha riportato il client di sincronizzazione, e il rimedio duraturo è `move-out`.

`undo` annulla prima la modifica più recente, una riparazione o uno spostamento. Un passo il cui risultato è cambiato dopo la riparazione viene lasciato com'è ed elencato, mai sovrascritto: git ha riscritto l'index ricreato, il client di sincronizzazione ha riportato una copia, un ref di salvataggio si è spostato, il backup è stato eliminato o `.git` è stato spostato. Un ref di salvataggio resta finché la copia che mantiene viva è nel backup. I file scaricati restano su disco e gli oggetti recuperati di nuovo restano nell'archivio degli oggetti: nessuno dei due sostituisce nulla. Il resto viene annullato e l'undo si completa, così la modifica precedente può essere annullata dopo.

`history` elenca le modifiche che questo dispositivo ha fatto al repository (riparazioni, spostamenti, annullamenti), il backup che ciascuna conserva con la sua dimensione e quale annullerà `undo` per prima. `prune-backups`, solo con `--apply`, elimina i backup delle modifiche annullate, ripristinate o mai annullabili (un undo, un prepare, un attach) il cui ultimo passo è più vecchio di `--older-than` (predefinito `720h`). Una riparazione o uno spostamento che si può ancora annullare conserva il proprio backup, anche se sopra ci sono modifiche più recenti.

<a name="moving-git-out-of-the-synced-folder"></a>

## Spostare .git fuori dalla cartella sincronizzata

```sh
gitbrella move-out            # cosa succede e cosa lo impedirebbe
gitbrella move-out --apply    # fallo
gitbrella undo --apply        # riportalo indietro
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Prima: .git dentro la cartella sincronizzata. Dopo move-out: un file .git nella cartella sincronizzata punta a ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d all'esterno">
</p>

`move-out` copia `.git` in `~/.local/share/gitbrella/gitdirs/<name>` (su macOS un clone APFS: non occupa spazio), dimostra file per file che la copia è identica, sposta l'originale in un backup sotto `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`) e scrive un file `.git` come `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. Il percorso è relativo, quindi ogni dispositivo che tiene la cartella nello stesso punto della home scrive lo stesso file `.git`. Poi verifica che git trovi lo stesso HEAD, stato, configurazione e directory hooks, e per 30 secondi (`--watch`) che il client di sincronizzazione lasci stare `.git` e, con iCloud, carichi il nuovo file. L'originale viene messo da parte prima del confronto finale, così una scrittura che si infila durante la copia viene rilevata e lo spostamento viene annullato. Con iCloud, `move-out` rifiuta finché iCloud sta ancora caricando il repository; `--wait-upload 10m` invece aspetta fino a quel tempo.

Ogni modifica viene registrata in un journal prima di avvenire, con dispositivo e inode di tutto ciò che crea (e i byte del file `.git`); un rollback non tocca nient'altro. Se il processo si interrompe a metà, il successivo `move-out --apply` (o `undo --apply`) lo completa o lo annulla e si ferma lì (riesegui il comando per continuare); la modifica incompiuta viene trovata tramite l'inode della cartella, anche con il percorso scritto in un altro modo, dopo una rinomina o dentro un altro repository. Nel frattempo `check` segnala la modifica incompiuta come danno. Nulla viene annullato sopra la directory spostata dopo che git ha iniziato a usarla: se contiene qualcosa che l'originale non ha, il comando si ferma e li nomina entrambi. Nulla viene eliminato: l'originale resta nel backup finché non lo rimuovi tu. `undo` riporta indietro la directory git attualmente in uso, con i commit fatti dopo lo spostamento; il backup precedente allo spostamento resta intatto. Se il client di sincronizzazione ha rimosso localmente il file `.git` (su questo Mac ne resta solo il nome), `undo` e il ripristino chiedono di scaricarlo prima: leggerlo lo scaricherebbe, e questo strumento non lo fa mai.

Se un rollback trova sia l'originale (nel backup) sia un nuovo `.git` (creato da te o dal client di sincronizzazione dopo che lo spostamento si è fermato), non sovrascrive nessuno dei due e lo dice: confrontali, sposta fuori dalla cartella il `.git` che non vuoi e riesegui il comando.

Rifiuta (uscita 1, con i motivi) mentre git o qualcosa che detiene un lock di git è in esecuzione sul repository, durante un merge o un rebase, mentre i file di `.git` sono solo nel cloud, mentre iCloud sta ancora caricando la cartella, mentre in `.git` c'è un socket (un `git fsmonitor--daemon` in esecuzione), quando dopo uno spostamento esistono due directory git, e per configurazioni i cui collegamenti si romperebbero: worktree collegati, submodule, oggetti presi in prestito (`alternates`), un `.git` che è un symlink o che non è la directory git stessa. Gli altri danni trovati da `check` vengono spostati così come sono ed elencati come riportati: riesegui `check` dopo.

### Altri dispositivi

Il client di sincronizzazione elimina `.git` su ogni altro dispositivo che sincronizza la cartella e vi consegna il file `.git`. Per conservare la loro cronologia:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: move-out --prepare --apply su ogni altro dispositivo. 2: move-out --apply su un dispositivo. 3: move-out --attach --apply su ogni altro dispositivo">
</p>

1. Su ogni altro dispositivo, prima dello spostamento: `gitbrella move-out --prepare --apply` copia il `.git` di quel dispositivo dove punterà il file `.git`.
2. Su un dispositivo: `gitbrella move-out --apply`.
3. Su ogni altro dispositivo, dopo che è arrivato il file `.git`: `gitbrella move-out --attach --apply`. Un dispositivo non preparato clona invece la cronologia: `--attach --from <remote URL> --apply`. In entrambi i casi il worktree non viene toccato.

Il file `.git` arriva tramite il client di sincronizzazione, quindi attach se ne fida solo quanto consentono i registri di questo dispositivo: deve puntare dentro `~/.local/share/gitbrella/gitdirs` di questo dispositivo, e una directory già presente lì viene usata solo se l'ha creata `--prepare` per questo repository.

Da quel momento il client di sincronizzazione trasporta i file, non la cronologia. Condividi i commit tramite un remote. Su un dispositivo i cui file il client di sincronizzazione ha già aggiornato, `git pull` rifiuta ("would be overwritten"): usa `git fetch` e `git reset --mixed @{u}`, che sposta il ramo e l'index e lascia i file sincronizzati come sono.

<a name="what-it-reports"></a>

## Cosa segnala

Danni (uscita 1):

| Codice | Significato |
| --- | --- |
| `INDEX_COPY` | copia di conflitto dell'index (`index 2`; nomi di Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive e Seafile; `index (1)` di Google Drive, `index-COMPUTER` di OneDrive in una cartella OneDrive e i nomi del vecchio Nextcloud, di Synology Drive, pCloud e Resilio vengono confrontati solo con i nomi dei file di git stesso); indica se è un index valido e se corrisponde a HEAD |
| `REF_COPY` | copia di conflitto del ref di un ramo o di un tag; indica se la copia è più vecchia, più nuova, divergente o se nomina un commit mancante |
| `COPY_IN_GITDIR` | qualsiasi altra copia di conflitto in `.git` |
| `INDEX_MISSING` | l'index manca; git mostra ogni file come eliminato (in un hook di commit: il commit eliminerebbe ogni file) |
| `INDEX_EMPTY` | l'index non elenca alcun file mentre i file di HEAD sono ancora nel worktree, come quando git lo ricrea dopo che il client di sincronizzazione ha eliminato l'index; un commit eliminerebbe ogni file |
| `INDEX_SHRUNK` | l'index non elenca più file di HEAD ancora presenti nel worktree (in uno sparse checkout: qualsiasi file di HEAD), come quando git lo ricrea dai file preparati dopo che il client di sincronizzazione ha eliminato l'index; un commit li eliminerebbe. Rinomine, una directory diventata un symlink o un submodule dichiarato in `.gitmodules` e file eliminati anche dal disco non contano; segnalato come non verificato quando i file di HEAD non si possono elencare |
| `HEAD_COMMIT_MISSING` | HEAD nomina un commit che non è nel repository (il ref è arrivato prima dei suoi oggetti) |
| `ORPHAN_INDEX_LOCK` | l'index manca ed è rimasto un vecchio `index.lock` (spesso un index completo) |
| `INDEX_INVALID` | l'index non supera il proprio checksum |
| `STALE_LOCK` | un lock di git che nessun git in esecuzione può detenere: nessuno è in esecuzione sul repository, o sono partiti tutti dopo che il lock è stato scritto. Si vedono solo i processi git su questa macchina: prima di rimuovere il lock assicurati che nessun git usi il repository in un container, su un'altra macchina che condivide la cartella o in un'app che incorpora git |
| `DATALESS_IN_GITDIR` | file in `.git` presenti solo nel cloud; controlli sugli oggetti saltati |
| `FOREIGN_FILE_IN_GITDIR` | un file in `refs/` o `objects/` non creato da git (`desktop.ini`, file `.tmp` in `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | un oggetto o ref di zero byte |
| `GITDIR_COPY` | una copia della directory git nel worktree (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | la directory git si trova dentro il worktree con un altro nome (`.git.nosync`) e `git add -A` ne farebbe il commit |
| `WORKTREE_ADMIN_BROKEN` | un worktree collegato ha perso i suoi file di amministrazione |
| `STALE_OPERATION_STATE` | una directory di rebase rimasta fa credere a git che un rebase sia in corso |
| `FSCK_ERRORS` | solo `--deep`: `git fsck` ha segnalato errori (quando non si completa, lo stesso codice è informativo) |
| `REFTABLE_TABLE_UNLISTED` | un repository reftable contiene tabelle di ref che il suo elenco non nomina: le tabelle di un altro dispositivo portate dal client di sincronizzazione, il cui elenco è diventato una copia che git non legge mai. Il prossimo `git pack-refs` o la manutenzione le eliminano, e con loro i rami di quel dispositivo; copia prima la directory git in un posto sicuro |
| `CHANGE_UNFINISHED` | un `fix`, `undo` o `move-out` di gitbrella non è stato completato (è stato interrotto); prima di usare git lì, esegui con `--apply` il comando che nomina per completarlo o annullarlo |
| `GIT_DIR_RETURNED` | dopo `move-out`, `.git` è di nuovo una directory (il client di sincronizzazione ha riportato quella vecchia), quindi esistono due directory git; confronta e tienine una prima di fare commit |
| `STAGED_CONFLICT_COPY` | l'index aggiunge una copia di conflitto di sincronizzazione di un file che elenca anche (`notes 2.md` accanto a `notes.md`, una directory copiata `src 2/`, nomi di Syncthing); un commit metterebbe la copia nella cronologia. Decidono solo i nomi; una rinomina verso un nome del genere e un file con nome da copia senza il suo originale non contano; non controllato con un index diviso |

Informazioni (il codice di uscita non cambia): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (la directory git è stata spostata fuori dalla cartella sincronizzata: la configurazione sicura), `WORKTREE_COPIES` (copie di conflitto dei tuoi file), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (un archivio di oggetti alternativo contiene file solo nel cloud, quindi i controlli sugli oggetti sono stati saltati), `GC_PID_FOREIGN` (il `gc.pid` di un'altra macchina: git salta il gc automatico per 12 ore), `GC_LOG_FRESH` (un `gc.log` di meno di un giorno blocca il gc automatico), `HOOKS_IN_SYNCED_FOLDER` (un comando di `core.hooksPath` o `core.fsmonitor` si trova in una cartella sincronizzata fuori dalla directory git, quindi una modifica fatta su un altro dispositivo viene eseguita qui), `WORKTREE_GUESSED` (`GIT_DIR` senza worktree e nessun file di HEAD nella directory corrente: il worktree non è stato controllato).

Quando il percorso non è affatto un repository, dice perché se la causa è sul disco: un `.git 2` rinominato, i file di git solo nel cloud, una directory git senza HEAD, un file `.git` che punta a una directory git che non esiste più, o un `core.worktree` che punta a una cartella che non c'è su questa macchina (l'impostazione di un'altra macchina, portata dal client di sincronizzazione). Se il percorso contiene un `.git` che git non può usare, si ferma lì (uscita 2) invece di rispondere per un repository esterno come farebbe git stesso.

<a name="install"></a>

## Installazione

Richiede git 2.31 o successivo. macOS e Linux, amd64 e arm64, senza cgo.

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

oppure scarica un archivio dalla pagina delle release.

### Verificare una release

Ogni release include `checksums.txt` (lo SHA-256 di ogni archivio e SBOM), una firma cosign keyless `checksums.txt.sigstore.json` prodotta dal workflow di release, un SBOM SPDX per archivio e un'attestazione di provenienza della build di GitHub (build-provenance attestation). Controlla prima la firma, poi l'archivio rispetto all'elenco firmato (l'esempio è v1.2.3 per Apple silicon). Usa cosign v3.1.3 o successivo (v2.6.5 nella linea v2): le versioni più vecchie possono essere ingannate da un bundle contraffatto (GHSA-fx35-mq7g-6g98).

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

Una release viene compilata da un tag inviato tramite `.github/workflows/release.yml` con `.goreleaser.yaml` e resta una bozza finché una persona non la pubblica. I binari vengono compilati con `-trimpath` e l'orario del commit, quindi lo stesso tag e la stessa release di Go (il workflow prende la patch 1.26 più recente) producono gli stessi binari; gli archivi e gli SBOM che li contengono riportano il proprio orario di build.

<a name="limits"></a>

## Limiti

- `fix` ripara solo dove c'è una sola risposta corretta e lascia il resto a te, con il comando da eseguire; il rimedio duraturo è `move-out`, che tiene `.git` fuori dalla cartella sincronizzata.
- Le copie di conflitto che iCloud stesso crea tra due dispositivi non sono state riprodotte nei test; il rilevamento è testato con copie che seguono la stessa denominazione.
- Codex non passa agli hook il `workdir` del suo strumento di shell: un comando che Codex esegue in un'altra cartella viene controllato nella cartella della sessione.
- Windows e OneDrive su Windows non sono testati. I repository reftable ricevono i controlli a livello di file e quello sulle tabelle non elencate; lì le copie dei ref non vengono confrontate con i loro rami.
- I nomi di copia con contrassegni che un nome potrebbe avere di proposito vengono confrontati solo dove è git stesso a creare il nome, mai nel worktree: `index (1)` di Google Drive per i file di git stesso e dentro `refs/`; `index-COMPUTER` di OneDrive per i file di git stesso al livello superiore di una cartella OneDrive, con un nome di dispositivo che contiene una maiuscola, non è una parola con cui si chiude il nome di un backup (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` sono tuoi) ed è lungo al massimo 15 caratteri (un nome di Mac più lungo viene perso); `_conflict-YYYYMMDD-HHMMSS` del vecchio Nextcloud, i nomi di Synology Drive, `.conflicted` di pCloud e `.Conflict` di Resilio per i file di git stesso al livello superiore (quelli del vecchio Nextcloud anche in `refs/` e `logs/` accanto all'originale in una cartella Nextcloud). Il ramo nominato da un qualsiasi HEAD (come `ref:` con qualsiasi spaziatura, o come symlink), il suo reflog e le directory che lo contengono non vengono mai messi da parte, qualunque sia il loro nome. Perciò `report (1).pdf`, un ramo chiamato `fix.Conflict`, un tag chiamato `v1.0.conflicted` o `config-backup` sono nomi normali.
- I submodule vengono controllati dall'interno del submodule. Dal progetto genitore, una copia di conflitto in `.git/modules/<name>/` viene segnalata solo quando il suo originale è accanto, e un submodule con nome da copia (`lib 2`) viene segnalato come copia.
- L'index mancante di un worktree collegato viene segnalato quando si controlla quel worktree, non il worktree principale.
- Quando `.git` è un symlink verso un archivio fuori dalla cartella sincronizzata, esegui git e il controllo dal worktree collegato. Dalla directory dell'archivio, che non contiene file, un index svuotato sembra un "elimina tutto" intenzionale e non blocca il commit.
- Alcuni commit intenzionali richiedono `--no-verify`: smettere di tracciare file che restano su disco, eliminare ogni file dove gli oggetti non devono essere letti, una rinomina che cambia solo le maiuscole o la forma Unicode di un file con un altro hard link, e qualsiasi commit in uno sparse checkout (in modalità cone o no) che rimuove o rinomina un file tracciato o lo sposta con un cambio di tipo (un file diventato directory, una directory diventata symlink o submodule). Lì sono i pattern sparse di git a decidere quali file sono su disco, quindi una rimozione intenzionale non si distingue da un index perso; i pattern non vengono letti.
- Se il client di sincronizzazione ha perso insieme all'index anche il ref del ramo stesso (il suo file loose è stato eliminato prima che arrivasse un nuovo `packed-refs`), HEAD sembra un ramo nuovo e il commit non viene bloccato: inizia una nuova cronologia che contiene solo i file preparati. Nulla su disco viene eliminato e la vecchia cronologia resta nel reflog (`git reflog`).
- Con un index diviso, o dove gli oggetti non devono essere letti (clone parziale con git precedente alla 2.45, alternativi solo nel cloud), l'index non viene confrontato con HEAD file per file: viene rilevato solo un index mancante o vuoto.
- Con `core.splitIndex`, un index che non elenca alcun file non viene riconosciuto (la sua intestazione conta solo la parte divisa), quindi un commit da lì non viene bloccato.
- Hook: solo gli hook di commit vengono trovati dal processo git che li esegue. Gli altri hook (`pre-push`, `pre-applypatch`) devono eseguire il controllo dal worktree, altrimenti vengono rifiutati (uscita 2). Dove il processo git non è visibile (un client che esegue gli hook da sé, o l'elenco dei processi bloccato), il controllo ripiega sulla directory corrente: allora, se la directory di git non punta di nuovo al worktree (`--separate-git-dir`, `.git.nosync`, `--work-tree`), un index senza file blocca sempre il commit (usa `--no-verify` per uno svuotamento intenzionale), una directory corrente che non contiene alcun file di HEAD non viene controllata come worktree (`WORKTREE_GUESSED`) e un hook che si sposta in un altro repository o directory git viene rifiutato (uscita 2). Gli hook di ricezione (`pre-receive`) non sono un punto di controllo supportato.
- `move-out`: non tutti gli strumenti capiscono un file `.git`. Git e gli strumenti che eseguono git funzionano; isomorphic-git (Obsidian Git su mobile) e gli strumenti che aprono `.git` come directory no.
- `move-out`: rinominare o spostare la cartella sincronizzata rompe il file `.git` relativo. Allora `check` nomina la directory mancante; rimetti la cartella al suo posto o fai puntare a mano il file `.git` alla directory git.
- `move-out` richiede che la cartella sincronizzata si trovi nello stesso punto della home su ogni dispositivo (iCloud Drive lo è sempre; Dropbox può essere `~/Dropbox` su un dispositivo e `~/Library/CloudStorage/Dropbox` su un altro). Quando non è così, lì il file `.git` punta altrove e `--attach` rifiuta (`TARGET_UNEXPECTED`).
- La directory git spostata risiede nel file system della home; non c'è un'opzione per metterla altrove.
- `move-out --prepare` copia la cronologia del dispositivo così com'è in quel momento: sposta poco dopo la preparazione, senza nuovi commit nel mezzo.
- `move-out` e `fix` sono stati provati nella cartella di iCloud Drive stessa, non dentro la cartella iCloud di un'app (`iCloud~md~obsidian` di Obsidian).

## Contribuire

Vedi [CONTRIBUTING.md](../../CONTRIBUTING.md). Segnala i problemi di sicurezza come descritto in [SECURITY.md](../../SECURITY.md), non in una issue pubblica.

## Licenza

MIT
