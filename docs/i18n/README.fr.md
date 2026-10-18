<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="logo de gitbrella : un parapluie dont le manche est une branche git">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Trouve les dégâts que la synchronisation cloud cause aux dépôts git,<br>
  sans rien modifier et sans rien télécharger.
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
  <b>Français</b> ·
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
  <a href="#usage">Utilisation</a> ·
  <a href="#repairing">Réparation</a> ·
  <a href="#moving-git-out-of-the-synced-folder">Sortir .git</a> ·
  <a href="#what-it-reports">Ce qu'il signale</a> ·
  <a href="#install">Installation</a> ·
  <a href="#limits">Limites</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive et Syncthing synchronisent les fichiers un par un. Git, lui, s'attend à ce que son répertoire `.git` change d'un seul bloc ; les dépôts situés dans des dossiers synchronisés finissent donc avec des copies de conflit comme `.git/index 2` ou `refs/heads/main 2`, des index disparus à côté de fichiers `index.lock` abandonnés, et des objets qui n'existent que dans le cloud. La FAQ de Git elle-même déconseille de garder un dépôt dans un dossier synchronisé, mais les réglages par défaut continuent de les y placer : Bureau et Documents de macOS dans iCloud, la sauvegarde des dossiers Windows dans OneDrive, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Les clients de synchronisation copient .git d'un appareil à l'autre fichier par fichier et laissent des copies de conflit comme index 2 et refs/heads/main 2, un index.lock à côté d'un index perdu et des objets qui n'existent que dans le cloud">
</p>

`gitbrella check` vous dit, en général en moins d'une seconde, si un dépôt présente ces dégâts. Il est conçu pour s'exécuter avant qu'un agent IA ou un hook git ne touche au dépôt. `gitbrella fix` répare ce qui n'admet qu'une seule bonne réparation, et `gitbrella move-out` met fin aux dégâts pour de bon : il sort `.git` du dossier synchronisé et laisse à sa place un fichier `.git` qui pointe vers lui, de sorte que le client de synchronisation transporte vos fichiers et que git garde son historique pour lui.

| Commande | Ce qu'elle fait |
| --- | --- |
| `gitbrella check` | signale les dégâts ; lecture seule |
| `gitbrella fix` | répare ce qui n'a qu'une seule bonne réponse |
| `gitbrella move-out` | sort définitivement `.git` du dossier synchronisé |
| `gitbrella undo` | annule la dernière réparation ou le dernier déplacement |
| `gitbrella history` | liste les changements faits sur cet appareil |
| `gitbrella prune-backups` | supprime les anciennes sauvegardes de changements qui ne peuvent plus être annulés |
| `gitbrella hook --for claude\|codex` | protège les appels d'outils des agents IA |

## Démarrage rapide

**1. Regardez.** Lecture seule, aucun téléchargement, aucun réseau :

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # sortie 0 : sain, 1 : dégâts, 2 : n'a pas pu s'exécuter
```

**2. Réparez** ce qui n'a qu'une seule bonne réponse. Sans `--apply`, il affiche seulement le plan :

```sh
gitbrella fix                 # ce qu'il réparerait, et ce qu'il vous laisse
gitbrella fix --apply         # réparer ; `gitbrella undo --apply` remet tout en place
```

**3. Réglez le problème pour de bon.** Sortez `.git` du dossier synchronisé :

```sh
gitbrella move-out            # ce qui se passerait, et ce qui l'empêche
gitbrella move-out --apply
```

**Gardez-le protégé.** Bloquez les commits dans un dépôt endommagé :

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Pour les agents IA, voir [Pour les agents](#for-agents).

## Ce qu'il promet

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` est en lecture seule.** Il n'écrit jamais dans le dépôt. Git s'exécute avec `GIT_OPTIONAL_LOCKS=0`, gc automatique et maintenance désactivés, et uniquement des commandes qui lisent. Des tests de bout en bout pour les cas à risque (copies de l'index, index manquant, clones partiels, index fractionnés, répertoires git déplacés) enregistrent la taille, le mtime et l'inode de chaque fichier avant et après la vérification et exigent qu'ils soient identiques.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Aucun téléchargement depuis le cloud.** Sur macOS, il désactive le téléchargement à la demande pour son propre processus (et le git qu'il lance) avant de toucher au disque ; si macOS refuse, la vérification ne s'exécute pas (sortie 2). Les fichiers qui n'existent que dans iCloud sont signalés, pas récupérés.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Aucun réseau.** Les clones partiels ne récupèrent jamais les objets manquants (`GIT_NO_LAZY_FETCH`). Avec un git antérieur à 2.45, les vérifications d'objets sont ignorées à la place.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Silencieux sur les dépôts sains.** Un verrou n'est un dégât que lorsqu'il est ancien et qu'aucun git travaillant sur le dépôt (dans l'un quelconque de ses worktrees) n'a pu l'écrire : git crée un verrou au moment où il le prend, donc un git démarré plus tard ne peut pas le détenir. Les fichiers temporaires de git et les fichiers d'autres outils dans `.git` sont de l'information. Exécuté sur 89 dépôts sains en service, il n'a signalé aucun dégât.

<a name="usage"></a>

## Utilisation

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Code de sortie : **0** aucun dégât, **1** dégâts trouvés, **2** la vérification n'a pas pu s'exécuter. Les commandes qui modifient quelque chose (`move-out`, `fix`, `undo`) sortent avec **0** planifié, fait ou rien à faire (`already`) ; **1** refusé (avec les raisons), fait avec un avertissement, ou avec des dégâts laissés à votre charge (la liste MANUAL de `fix`, que `check` signalerait encore) ; **2** échec (annulé ou arrêté ; le JSON conserve ce qui a été récupéré et l'entrée du journal) ou n'a pas pu s'exécuter.

`--deep` exécute aussi un `git fsck` complet (il lit chaque objet, il est donc ignoré quand des objets n'existent que dans le cloud). `--provider` impose le client de synchronisation au lieu de le détecter d'après le chemin : `auto` (par défaut), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage` ou `none`. Les dossiers Syncthing et Nextcloud (ou ownCloud) sont reconnus grâce au marqueur que leur client y conserve (`.stfolder`, `.sync_*.db`), où qu'ils se trouvent. `fix` et `move-out` acceptent le même `--provider` : donnez-leur celui que vous avez donné à `check`, car certains noms de copie ne comptent que pour un client donné. Le JSON indique alors `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Sortie de gitbrella check : DAMAGE INDEX_COPY index 2, réparable par gitbrella fix, et INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>La même sortie en texte</summary>

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

### Comme hook git

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit lance le hook pre-commit, qui lance gitbrella check : la sortie 0 laisse passer le commit, 1 ou 2 le bloque">
</p>

Un dépôt endommagé bloque le commit ; un dépôt sain commite comme d'habitude. L'outil sait que c'est git qui exécute le hook et ne confond pas le verrou du commit lui-même avec un verrou abandonné.

**Quel dépôt est vérifié.** Pour les hooks de commit (git leur transmet `GIT_INDEX_FILE` ou un `GIT_DIR` absolu), il retrouve le dépôt à partir du processus git qui a lancé le hook ; le hook peut donc d'abord changer de répertoire : vers le répertoire git, son répertoire de hooks, ou un répertoire hors de tout dépôt. Quand git désigne le dépôt par un `GIT_DIR` absolu (worktrees liés, sous-modules, répertoires git séparés), chaque commande git du hook agit sur lui où que le hook aille, et la vérification aussi. Sinon, il regarde où git lui-même aboutirait depuis le répertoire courant (le répertoire git qu'y désigne un `GIT_DIR` relatif, ou le dépôt trouvé à partir de là) : s'il s'agit d'un autre dépôt ou d'un autre worktree du même (un hook qui s'y déplace, ou l'éditeur du message de commit), il refuse (sortie 2) plutôt que de répondre pour le mauvais.

Lancé sans chemin, il vérifie le dépôt que git a exporté vers le hook (`GIT_DIR`) ; les configurations de dotfiles avec `--git-dir` et les répertoires git situés dans leur propre worktree fonctionnent donc aussi. Il respecte également un `GIT_DIR` défini dans votre shell quand aucun chemin n'est donné ; contrairement à git, il ignore un `GIT_DIR` qui n'existe pas et refuse un worktree lié dont il ne peut remonter l'origine, plutôt que de prendre le répertoire courant comme arbre de travail.

**L'index que le commit enregistre.** Il lit `GIT_INDEX_FILE` quand celui-ci désigne un fichier à l'intérieur du répertoire git, comme git le définit pour les hooks de commit : si l'index que le commit enregistre était absent ou vide, ou ne liste plus un fichier de HEAD encore présent dans l'arbre de travail, le commit est bloqué avant de pouvoir enregistrer ces fichiers comme supprimés. Un fichier d'index situé ailleurs vers lequel vous faites pointer vous-même `GIT_INDEX_FILE` n'est pas lu : dans un hook, c'est l'index propre du dépôt qui est vérifié à la place, et une vérification lancée à la main avec cette variable définie refuse (sortie 2).

**Suppressions délibérées.** Pour arrêter de suivre un fichier que vous gardez sur le disque (`git rm --cached`, arrêter de suivre un répertoire que vous ignorez désormais), ou pour commiter délibérément un « tout supprimer » alors que vos fichiers sont encore sur le disque (`git rm -r --cached .`), sautez le hook avec `git commit --no-verify`.

### Avec un gestionnaire de hooks

- **pre-commit** (pre-commit.com) : ajoutez le dépôt avec l'identifiant de hook `gitbrella` (pre-commit le compile avec Go) ou `gitbrella-system` (le binaire du PATH) ; voir `.pre-commit-hooks.yaml`.
- **lefthook** : intégrez `hooks/lefthook.yml` avec `remotes`.
- **husky** : un `.husky/pre-commit` contenant `gitbrella check`.
- **Overcommit** : `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, puis `overcommit --sign`.

Toute sortie non nulle arrête le commit dans chacun d'eux ; une vérification qui ne peut pas s'exécuter (sortie 2) l'arrête donc aussi. Aider commite avec `--no-verify` sauf si `git-commit-verify: true` est défini, et alors aucun hook de commit ne s'exécute. Obsidian Git (bureau) commite avec le git de votre système et exécute le hook ; ajoutez le dossier contenant `gitbrella` à son réglage « Additional PATH », car les apps lancées depuis le Dock ne reçoivent pas le PATH du shell. Obsidian Git sur mobile (isomorphic-git) n'exécute aucun hook.

<a name="for-agents"></a>

### Pour les agents

`--json` affiche le même rapport sous forme de document stable et versionné (`schemaVersion: 1`, `findings` est toujours une liste). Décidez d'après le code de sortie, montrez les constats à l'utilisateur.

Les hooks propres aux agents n'arrêtent un appel d'outil que sur une sortie 2 : une sortie 1 (« dégâts trouvés ») laisse l'appel se poursuivre. `gitbrella hook --for claude|codex` parle le langage de leurs hooks. Il lit l'événement que l'agent lui transmet sur stdin :

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="L'appel d'outil d'un agent passe par gitbrella hook : lecture et édition se poursuivent ; dans un dépôt endommagé, les écritures git et les écritures dans .git s'arrêtent avec la sortie 2">
</p>

- **Avant un appel d'outil** (`PreToolUse`) : dans un dépôt endommagé, une commande git qui écrit (commit, add, reset, rebase, push, et tout ce dont on ne sait pas qu'il se contente de lire) et une écriture dans `.git` arrêtent l'appel (sortie 2, la raison sur stderr pour l'agent). La lecture, la modification de vos fichiers, `git restore` et `gitbrella` lui-même se poursuivent, afin que les dégâts puissent être examinés et réparés.
  - *Dépôts inutilisables.* Là où les fichiers de git n'existent que dans le cloud, ou où git ne peut pas du tout utiliser le dépôt (un `HEAD` renommé, un fichier `.git` qui ne pointe nulle part), toute commande git est arrêtée : elle se bloquerait, ou agirait sur un dépôt englobant. Les écritures dans `.git` s'y poursuivent : cette réparation se fait à la main dans `.git`, comme l'indique la raison (`mv '.git/HEAD 2' .git/HEAD`). Le remède que nomme l'indication sur `core.worktree` (`git config --file … --unset core.worktree`) se poursuit aussi.
  - *Quel dépôt.* Celui qui contient les fichiers sur lesquels porte l'appel (un patch Codex nomme les siens ; un chemin dans `.git` compte pour l'arbre de travail situé au-dessus). Pour une commande shell, c'est le dossier de la session et chaque dossier où s'exécutent ses commandes git (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), ceux qui écrivent d'abord, cinq au plus ; quand un dossier qui écrit n'y trouve pas sa place, l'appel passe avec un avertissement. Chaque dossier est jugé d'après ce que l'appel y fait : depuis un dossier de session endommagé, `git -C ../sound commit` se poursuit. Un dossier qui n'est pas un dépôt, et un dépôt bare, passent sans bruit.
  - *Quand la vérification ne peut pas s'exécuter.* Tout ce qui empêche la vérification de s'exécuter (y compris le `--timeout` de 20 secondes, une seule échéance pour tout l'appel) laisse passer l'appel avec un avertissement ; `--fail-open` fait de même là où git ne peut pas utiliser le dépôt.
  - *Comment la commande est lue.* Elle est lue, jamais exécutée : guillemets, commentaires, sous-shells, substitutions (y compris dans un heredoc sans guillemets), heredocs, variables qu'elle définit, `&&`, `||`, `if`, `case` et boucles (après `cd a || cd b`, ou un `cd` vers un dossier inexistant, chaque dossier où le shell peut se trouver compte ; une boucle `for` est lue une fois par valeur), enveloppes (`timeout`, `sudo`, `nice`, `xargs`, `env`), et les scripts donnés à un shell (`-c`, `eval`, `watch`, `source`, un fichier, un heredoc, `bash < f`, un pipe vers `sh`). Les fonctions et alias que définit la commande sont lus là où ils sont appelés. Un script shell lancé par son chemin ou donné à un shell (`./deploy.sh`, `bash deploy.sh`) est lu quand c'est un petit fichier sur cette machine (via un lien symbolique aussi, et une seule fois même s'il s'exécute lui-même). `make`, `npm run` et consorts ne sont pas lus.
  - *Ce qui compte comme écriture.* Un autre programme qui écrit dans `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` sur `.git/config`, `rm -rf` ou `find -delete` sur un dossier contenant `.git`, un patch Codex passant par le shell) compte comme une écriture dans `.git` ; copier `.git` ailleurs, l'archiver avec `tar` ou le lire, non. `gh pr checkout` et `git-lfs` comptent comme des commandes git qui écrivent.
  - *En cas de doute.* Chacun de ces cas compte comme une commande git qui écrit : une lecture qui prend plus de 5000 étapes ou la moitié du `--timeout` (dans un dossier qu'elle ne peut pas déterminer) ; le mot `git` à un endroit que la lecture ne suit pas ; un script envoyé par pipe dans un shell depuis un programme que la lecture ne suit pas (`curl … | sh`), là où il s'exécute ; un script donné à un shell, présent mais illisible (trop gros, seulement dans le cloud), là où il s'exécute. Quand il ne peut pas déterminer le dossier dans lequel écrit une commande git (`cd "$(…)"`, `for d in */`), la commande compte dans chaque dossier vérifié et l'appel reçoit un avertissement.
- **Au démarrage d'une session** (`SessionStart`) : une ligne sur stdout, que l'agent lit comme contexte, indique que le dépôt se trouve dans un dossier synchronisé (sauf si son répertoire git est déjà dehors) et ce qui est endommagé. Elle ne bloque jamais.

Le hook ne modifie rien. Claude Code, dans `.claude/settings.json` :

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex, dans `.codex/hooks.json` (Codex n'exécute un hook qu'après que vous lui avez fait confiance dans `/hooks`) :

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

La vérification s'exécute une fois pour chacun de ces dossiers, en général de 0,1 à 0,4 seconde chacun ; un patch réparti sur plusieurs dossiers d'un même dépôt le vérifie donc plusieurs fois ; `--timeout` borne l'appel entier.

<a name="repairing"></a>

## Réparation

```sh
gitbrella fix                 # ce qu'il réparerait, et ce qu'il vous laisse
gitbrella fix --apply         # réparer
gitbrella undo --apply        # tout remettre en place
gitbrella history             # les changements faits ici, leurs sauvegardes et leurs tailles
gitbrella prune-backups       # les sauvegardes dont aucun undo ne peut plus avoir besoin
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix affiche le plan ; fix --apply enregistre chaque étape, puis la vérification est relancée : terminé, ou annulé ; undo --apply annule le changement le plus récent">
</p>

`check` indique sous chaque dégât si `fix` le répare (`repair:`, et `"repair": "fix"` ou `"manual"` dans le JSON), en nommant `fix --provider NAME` quand la vérification en a reçu un ; dans les hooks de commit, il ne pose pas la question.

`fix` ne répare que là où il n'y a qu'une seule bonne réponse et où rien de ce que vous avez pu vouloir n'est écrasé :

| Dégât | Réparation |
| --- | --- |
| copies de conflit dans `.git` à côté de leur original (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, copies Dropbox et Syncthing, et une copie du verrou d'une ref comme `refs/heads/main 2.lock`) ; une copie de l'index seulement quand elle est identique à l'index ou est un index valide égal à HEAD | déplacées dans la sauvegarde ; un objet vers lequel seule la copie pointait (un commit, un tag, un arbre ou un fichier) est d'abord conservé sous `refs/gcd-rescue/<name>` |
| un répertoire de rebase abandonné, `desktop.ini`, `Icon` et fichiers semblables que git n'a pas créés (sauf si git en lit un comme branche, ou si son nom est celui d'un objet), une copie de `.git` dans l'arbre de travail dont le dépôt possède les commits | déplacés dans la sauvegarde |
| index manquant, sans copie ni verrou de celui-ci nulle part | recréé à partir de HEAD (`git read-tree HEAD` : les fichiers ne sont pas touchés) ; seulement s'il existe un commit à partir duquel le construire (si le commit de HEAD manque aussi, il est d'abord récupéré, comme ci-dessous) et hors d'un sparse checkout |
| un index qui ne liste aucun fichier (un index valide, non fractionné, hors d'un sparse checkout) | déplacé dans la sauvegarde et recréé à partir de HEAD : rien n'y était indexé |
| un `index.lock` orphelin qui est un index valide égal à HEAD | il devient l'index |
| un fichier de branche vide | le commit que son reflog a enregistré en dernier |
| `.git.nosync` que git ajouterait à un commit | la ligne `/.git.nosync/` dans `.git/info/exclude` |
| commit de HEAD manquant, ou fichier d'objet libre vide (sans copie à côté) | le fichier vide est déplacé dans la sauvegarde et les objets sont récupérés à nouveau (`git fetch --refetch`, git 2.36 ou plus récent) depuis `--remote`, sinon le distant de la branche, sinon `origin`, sinon l'unique distant ; les branches et tags du distant sont récupérés dans un espace de noms temporaire `refs/gcd-fetch/` supprimé ensuite, donc aucune ref ne change. Si le distant ne ramène pas l'objet, la réparation est annulée |
| avec `fix --deep` : objets que `git fsck` signale comme endommagés | chaque objet des branches et tags du distant est récupéré à nouveau, de la même façon ; `git fsck` est relancé ensuite et, s'il signale encore des erreurs (un objet qu'aucun distant ne possède, un fichier corrompu qui reste), la réparation est annulée |
| copies de conflit de vos propres fichiers (`notes 2.md`) que git ne suit pas et qui contiennent les mêmes octets que le fichier qu'elles copient | déplacées dans la sauvegarde (`git add -A` les commiterait) ; une copie qui diffère, est suivie, est un lien, n'existe que dans le cloud ou se trouve dans un sous-module reste en place |
| le `gc.pid` d'une autre machine datant de plus d'une heure ; un `gc.log` | déplacés dans la sauvegarde, pour que le gc automatique de git s'exécute à nouveau ici |
| fichiers de `.git`, ou d'un magasin d'objets alternatif, qui n'existent que dans le cloud | téléchargés, et rien d'autre : d'ici là la vérification ne peut pas lire les objets, donc le reste des dégâts est listé comme MANUAL pour la prochaine exécution de `fix` |

Tout le reste est listé comme MANUAL, avec la commande à lancer. Cela comprend tout ce qui pourrait contenir du travail introuvable ailleurs :
- un index endommagé (git peut encore le lire, travail indexé compris) ;
- une copie de l'index ou un `index.lock` qui diffère de HEAD, et une copie de `index.lock` ;
- une copie dont l'original manque (c'est peut-être la seule : remettez-la en place, sauf s'il s'agit d'une copie de verrou, qui est sauvée et mise de côté) ;
- un verrou qu'aucun git de cette machine ne détient (un git sur un autre appareil ou dans un conteneur le peut ; son conseil nomme le commit qu'il contient) ;
- une copie qui nomme plus de 100 objets (un long reflog) ;
- une copie de la branche que nomme un HEAD, de son reflog, ou d'un répertoire contenant cette branche (git la lit, quelle que soit l'allure de son nom).

Cela comprend aussi tout cas où fix devrait choisir entre deux versions, ou ne pourrait pas garder en vie les commits d'une copie :
- tout ce qui se trouve dans le magasin d'objets : copies, fichiers portant le nom d'un objet (vérifiez lequel correspond à son hash), fichiers d'objet vides sans distant d'où les récupérer ;
- tout ce qui se trouve dans le répertoire git d'un sous-module (lancez `fix` dans le sous-module) ;
- une copie de branche qui nomme un commit pas encore arrivé ici (il peut encore arriver) ;
- une copie de `.git` dans l'arbre de travail avec des commits que le dépôt n'a pas (c'est à vous de dire laquelle des deux est à jour) ;
- un fichier vide à côté d'une copie de lui-même ou de son répertoire ;
- un répertoire d'administration de worktree lié cassé (`git worktree repair`) ;
- un index qui a perdu des fichiers encore présents sur le disque (cesser de les suivre exprès donne le même résultat ; quand rien d'autre n'est indexé, le conseil dit que `git read-tree HEAD` les rend sans rien perdre) ;
- les objets que `git fsck` signale comme endommagés, sans `--deep` ou sans distant.

`fix` ne touche pas du tout aux dépôts reftable ; là, le `REFTABLE_TABLE_UNLISTED` de `check` indique comment conserver les refs d'un autre appareil. La sauvegarde doit se trouver sur le disque du dépôt, pour que ce que fix met de côté soit déplacé en une seule étape ; sinon `fix` refuse (une réparation qui ne fait que télécharger ne déplace rien et n'en a pas besoin).

Chaque étape est enregistrée avant d'agir, et la vérification est relancée à la fin : si quelque chose qu'il a réparé est encore signalé, ou si de nouveaux dégâts sont apparus, les étapes sont annulées. Un fix interrompu est annulé par la prochaine commande de modification lancée avec `--apply` dans ce dépôt, qui s'arrête ensuite ; relancez-la. Au bout de 30 secondes (`--watch`), `fix` vérifie une fois de plus : des dégâts qui reviennent ont été ramenés par le client de synchronisation, et le remède durable est `move-out`.

`undo` annule d'abord le changement le plus récent, une réparation ou un déplacement. Une étape dont le résultat a changé depuis la réparation est laissée telle quelle et listée, jamais écrasée : git a réécrit l'index reconstruit, le client de synchronisation a ramené une copie, une ref de sauvetage a bougé, la sauvegarde a été supprimée, ou `.git` a été déplacé. Une ref de sauvetage reste tant que la copie qu'elle maintient en vie reste dans la sauvegarde. Les fichiers téléchargés restent sur le disque et les objets récupérés à nouveau restent dans le magasin d'objets : ni les uns ni les autres ne remplacent quoi que ce soit. Le reste est annulé, et l'undo se termine, pour que le changement précédent puisse être annulé ensuite.

`history` liste les changements que cet appareil a faits sur le dépôt (réparations, déplacements, annulations), la sauvegarde que chacun conserve avec sa taille, et celui que `undo` annulera ensuite. `prune-backups` supprime, uniquement avec `--apply`, les sauvegardes des changements annulés, revenus en arrière ou jamais annulables (un undo, un prepare, un attach) et dont la dernière étape est plus ancienne que `--older-than` (par défaut `720h`). Une réparation ou un déplacement encore annulable garde sa sauvegarde, même sous des changements plus récents.

<a name="moving-git-out-of-the-synced-folder"></a>

## Sortir .git du dossier synchronisé

```sh
gitbrella move-out            # ce qui se passerait, et ce qui l'empêche
gitbrella move-out --apply    # le faire
gitbrella undo --apply        # le remettre en place
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Avant : .git à l'intérieur du dossier synchronisé. Après move-out : un fichier .git dans le dossier synchronisé pointe vers ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d, à l'extérieur">
</p>

`move-out` copie `.git` vers `~/.local/share/gitbrella/gitdirs/<name>` (un clone APFS sur macOS : aucun espace utilisé), prouve fichier par fichier que la copie est identique, déplace l'original dans une sauvegarde sous `~/Library/Application Support/gitbrella` (Linux : `$XDG_STATE_HOME/gitbrella`), et écrit un fichier `.git` tel que `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. Le chemin est relatif ; chaque appareil qui garde le dossier au même endroit sous son home écrit donc le même fichier `.git`. Il vérifie ensuite que git trouve le même HEAD, le même état, la même configuration et le même répertoire de hooks, et pendant 30 secondes (`--watch`) que le client de synchronisation laisse `.git` tranquille et, sous iCloud, envoie le nouveau fichier. L'original est mis de côté avant la dernière comparaison ; une écriture qui s'est glissée pendant la copie est donc détectée et le déplacement annulé. Sous iCloud, `move-out` refuse tant qu'iCloud envoie encore le dépôt ; `--wait-upload 10m` attend jusqu'à cette durée à la place.

Chaque changement est consigné dans un journal avant d'avoir lieu, avec le périphérique et l'inode de tout ce qu'il crée (et les octets du fichier `.git`) ; un retour en arrière ne touche à rien d'autre. Si le processus meurt en cours de route, le prochain `move-out --apply` (ou `undo --apply`) le termine ou l'annule puis s'arrête là (relancez la commande pour continuer) ; il le retrouve grâce à l'inode du dossier, même sous une autre graphie, après un renommage ou à l'intérieur d'un autre dépôt, et `check` signale entre-temps le changement inachevé comme un dégât. Une fois que git a utilisé le répertoire déplacé, rien n'est annulé par-dessus : s'il contient quelque chose qui manque à l'original, la commande s'arrête et nomme les deux. Rien n'est supprimé : l'original reste dans la sauvegarde jusqu'à ce que vous le retiriez. `undo` ramène le répertoire git utilisé actuellement, avec les commits faits depuis le déplacement ; la sauvegarde d'avant le déplacement reste intacte. Si le client de synchronisation a évincé le fichier `.git` (seul son nom est sur ce Mac), `undo` et la reprise vous demandent d'abord de le télécharger : le lire le téléchargerait, ce que l'outil ne fait jamais.

Si un retour en arrière trouve à la fois l'original (dans la sauvegarde) et un nouveau `.git` (créé après l'arrêt du déplacement, par vous ou par le client de synchronisation), il n'écrase ni l'un ni l'autre et le signale : comparez-les, sortez du dossier le `.git` dont vous ne voulez pas, et relancez la commande.

Il refuse (sortie 1, avec les raisons) tant que git ou quelque chose qui détient un verrou git travaille sur le dépôt, pendant un merge ou un rebase, tant que des fichiers de `.git` n'existent que dans le cloud, tant qu'iCloud envoie encore le dossier, tant qu'un socket (un `git fsmonitor--daemon` en cours d'exécution) se trouve dans `.git`, lorsque deux répertoires git existent après un déplacement, et pour les dispositions dont les liens casseraient : worktrees liés, sous-modules, objets empruntés (`alternates`), un `.git` qui est un lien symbolique ou qui n'est pas le répertoire git lui-même. Les autres dégâts que trouve `check` sont déplacés tels quels et listés comme emportés : relancez `check` ensuite.

### Autres appareils

Le client de synchronisation supprime `.git` sur chaque autre appareil qui synchronise le dossier et y apporte le fichier `.git` à la place. Pour conserver leur historique :

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1 : sur chaque autre appareil, move-out --prepare --apply. 2 : sur un appareil, move-out --apply. 3 : sur chaque autre appareil, move-out --attach --apply">
</p>

1. Sur chaque autre appareil, avant le déplacement : `gitbrella move-out --prepare --apply` copie son `.git` là où pointera le fichier `.git`.
2. Sur un appareil : `gitbrella move-out --apply`.
3. Sur chaque autre appareil, une fois le fichier `.git` arrivé : `gitbrella move-out --attach --apply`. Un appareil qui n'a pas été préparé clone l'historique à la place : `--attach --from <remote URL> --apply`. Dans les deux cas l'arbre de travail n'est pas touché.

Le fichier `.git` arrive par le client de synchronisation ; attach ne lui fait donc confiance que dans la mesure où le permettent les enregistrements de cet appareil : il doit pointer à l'intérieur du `~/.local/share/gitbrella/gitdirs` de cet appareil, et un répertoire qui s'y trouve déjà n'est utilisé que si `--prepare` l'a créé pour ce dépôt.

Dès lors, le client de synchronisation transporte les fichiers et non l'historique. Partagez les commits via un distant. Sur un appareil dont le client de synchronisation a déjà mis à jour les fichiers, `git pull` refuse (« would be overwritten ») : utilisez `git fetch` et `git reset --mixed @{u}`, qui déplace la branche et l'index et laisse les fichiers synchronisés tels quels.

<a name="what-it-reports"></a>

## Ce qu'il signale

Dégâts (sortie 1) :

| Code | Signification |
| --- | --- |
| `INDEX_COPY` | copie de conflit de l'index (`index 2` ; noms Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive et Seafile ; le `index (1)` de Google Drive, le `index-COMPUTER` de OneDrive dans un dossier OneDrive, et les noms de l'ancien Nextcloud, de Synology Drive, pCloud et Resilio, qui ne sont reconnus que pour les noms de fichiers propres à git) ; indique s'il s'agit d'un index valide et s'il correspond à HEAD |
| `REF_COPY` | copie de conflit d'une ref de branche ou de tag ; indique si la copie est plus ancienne, plus récente, divergente ou nomme un commit manquant |
| `COPY_IN_GITDIR` | toute autre copie de conflit dans `.git` |
| `INDEX_MISSING` | l'index a disparu ; git afficherait chaque fichier comme supprimé (dans un hook de commit : le commit supprimerait chaque fichier) |
| `INDEX_EMPTY` | l'index ne liste aucun fichier alors que les fichiers de HEAD sont encore dans l'arbre de travail, comme lorsque git l'a reconstruit après que le client de synchronisation l'a supprimé ; un commit supprimerait chaque fichier |
| `INDEX_SHRUNK` | l'index ne liste plus des fichiers de HEAD encore présents dans l'arbre de travail (dans un sparse checkout : n'importe quel fichier de HEAD), comme lorsque git l'a reconstruit à partir des fichiers indexés après que le client de synchronisation l'a supprimé ; un commit les supprimerait. Les renommages, un répertoire devenu lien symbolique ou sous-module déclaré par `.gitmodules`, et les fichiers aussi supprimés du disque ne sont pas comptés ; quand les fichiers de HEAD ne peuvent pas être listés, c'est signalé comme non vérifié |
| `HEAD_COMMIT_MISSING` | HEAD nomme un commit absent du dépôt (la ref est arrivée avant ses objets) |
| `ORPHAN_INDEX_LOCK` | l'index a disparu et un vieux `index.lock` a été laissé (souvent un index complet) |
| `INDEX_INVALID` | l'index échoue à sa propre somme de contrôle |
| `STALE_LOCK` | un verrou git qu'aucun git en cours d'exécution ne peut détenir : aucun ne travaille sur le dépôt, ou tous ont démarré après l'écriture du verrou. Seuls les processus git de cette machine sont visibles : avant de retirer le verrou, assurez-vous qu'aucun git n'utilise le dépôt dans un conteneur, sur une autre machine partageant le dossier, ou dans une app qui embarque git |
| `DATALESS_IN_GITDIR` | des fichiers de `.git` n'existent que dans le cloud ; les vérifications d'objets ont été ignorées |
| `FOREIGN_FILE_IN_GITDIR` | un fichier que git n'a pas créé dans `refs/` ou `objects/` (`desktop.ini`, fichiers `.tmp` dans `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | un objet ou une ref de zéro octet |
| `GITDIR_COPY` | une copie du répertoire git dans l'arbre de travail (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | le répertoire git se trouve dans l'arbre de travail sous un autre nom (`.git.nosync`) et `git add -A` le commiterait |
| `WORKTREE_ADMIN_BROKEN` | un worktree lié a perdu ses fichiers d'administration |
| `STALE_OPERATION_STATE` | un répertoire de rebase abandonné fait croire à git qu'un rebase est en cours |
| `FSCK_ERRORS` | `--deep` seulement : `git fsck` a signalé des erreurs (quand il ne se termine pas, le même code est une information) |
| `REFTABLE_TABLE_UNLISTED` | un dépôt reftable contient des tables de refs que sa liste ne nomme pas : celles d'un autre appareil, apportées par le client de synchronisation pendant que sa liste devenait une copie que git ne lit jamais. Le prochain `git pack-refs` ou la prochaine maintenance les supprime, et les branches de cet appareil avec elles ; copiez d'abord le répertoire git en lieu sûr |
| `CHANGE_UNFINISHED` | un `fix`, `undo` ou `move-out` de gitbrella ne s'est pas terminé (il a été interrompu) ; lancez avec `--apply` la commande qu'il indique pour le terminer ou l'annuler avant d'utiliser git à cet endroit |
| `GIT_DIR_RETURNED` | après `move-out`, `.git` est redevenu un répertoire (le client de synchronisation a ramené l'ancien), il existe donc deux répertoires git ; comparez-les et gardez-en un avant de commiter |
| `STAGED_CONFLICT_COPY` | l'index ajoute une copie de conflit de synchronisation d'un fichier qu'il liste aussi (`notes 2.md` à côté de `notes.md`, un répertoire copié `src 2/`, noms Syncthing) ; un commit mettrait la copie dans l'historique. Seuls les noms décident ; un renommage vers un tel nom, et un fichier nommé comme une copie sans son original, ne comptent pas ; non vérifié avec un index fractionné |

Information (code de sortie inchangé) : `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (le répertoire git a été sorti du dossier synchronisé : la disposition sûre), `WORKTREE_COPIES` (copies de conflit de vos propres fichiers), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (un magasin d'objets alternatif contient des fichiers uniquement dans le cloud, donc les vérifications d'objets ont été ignorées), `GC_PID_FOREIGN` (le `gc.pid` d'une autre machine : git saute son gc automatique pendant 12 heures), `GC_LOG_FRESH` (un `gc.log` de moins d'un jour arrête le gc automatique), `HOOKS_IN_SYNCED_FOLDER` (`core.hooksPath` ou une commande `core.fsmonitor` se trouve dans un dossier synchronisé hors du répertoire git, donc une modification faite sur un autre appareil s'exécute ici), `WORKTREE_GUESSED` (`GIT_DIR` sans arbre de travail, et aucun des fichiers de HEAD n'est dans le répertoire courant : l'arbre de travail n'a pas été vérifié).

Quand le chemin n'est pas du tout un dépôt, il dit pourquoi si la cause est sur le disque : un `.git 2` renommé, des fichiers propres à git qui n'existent que dans le cloud, un répertoire git sans HEAD, un fichier `.git` pointant vers un répertoire git disparu, ou un `core.worktree` qui pointe vers un dossier que cette machine n'a pas (le réglage d'une autre machine apporté par le client de synchronisation). Si le chemin contient un `.git` que git ne peut pas utiliser, il s'arrête là (sortie 2) au lieu de répondre pour un dépôt englobant, ce que ferait git lui-même.

<a name="install"></a>

## Installation

Nécessite git 2.31 ou plus récent. macOS et Linux, amd64 et arm64, sans cgo.

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

ou prenez une archive sur la page des releases.

### Vérifier une version

Chaque version est accompagnée de `checksums.txt` (le SHA-256 de chaque archive et de chaque SBOM), de sa signature cosign sans clé `checksums.txt.sigstore.json` produite par le workflow de release, d'un SBOM SPDX pour chaque archive, et d'une attestation de provenance de build GitHub. Vérifiez d'abord la signature, puis l'archive par rapport à la liste signée (l'exemple est la v1.2.3 pour Apple silicon). Utilisez cosign v3.1.3 ou plus récent (v2.6.5 sur la branche v2) : les versions plus anciennes peuvent être trompées par un bundle falsifié (GHSA-fx35-mq7g-6g98).

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

Une version est construite par `.github/workflows/release.yml` à partir d'un tag poussé, avec `.goreleaser.yaml`, et reste un brouillon jusqu'à ce qu'une personne la publie. Les binaires sont construits avec `-trimpath` et l'heure du commit ; le même tag et la même version de Go (le workflow prend le correctif 1.26 le plus récent) produisent donc les mêmes binaires ; les archives et SBOM qui les entourent portent leur heure de construction.

<a name="limits"></a>

## Limites

- `fix` ne répare que là où il n'y a qu'une seule bonne réponse et vous laisse le reste avec la commande à lancer ; la solution durable est `move-out`, qui garde `.git` hors du dossier synchronisé.
- Les copies de conflit créées par iCloud lui-même entre deux appareils n'ont pas été reproduites en test ; la détection est testée avec des copies qui suivent la même convention de nommage.
- Codex ne transmet pas aux hooks le `workdir` de son outil shell : une commande que Codex exécute dans un autre dossier est vérifiée dans le dossier de la session.
- Windows et OneDrive sous Windows ne sont pas testés. Les dépôts reftable bénéficient des vérifications au niveau des fichiers et de la vérification des tables non listées ; les copies de refs n'y sont pas comparées à leurs branches.
- Les noms de copie dont la marque pourrait figurer volontairement dans un nom ne sont reconnus que là où git crée lui-même le nom, jamais dans l'arbre de travail : le `index (1)` de Google Drive pour les fichiers propres à git et dans `refs/` ; le `index-COMPUTER` de OneDrive pour les fichiers propres à git à la racine d'un dossier OneDrive, avec une majuscule dans le nom de l'appareil et sans un mot par lequel on termine le nom d'une sauvegarde (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` sont à vous), de 15 caractères au plus (un nom de Mac plus long est manqué) ; le `_conflict-YYYYMMDD-HHMMSS` de l'ancien Nextcloud, celui de Synology Drive, le `.conflicted` de pCloud et le `.Conflict` de Resilio pour les fichiers propres à git à la racine (celui de l'ancien Nextcloud aussi dans `refs/` et `logs/` d'un dossier Nextcloud, à côté de son original). Une branche que nomme un HEAD (sous forme de `ref:` avec n'importe quel espacement, ou de lien symbolique), son reflog et un répertoire qui la contient ne sont jamais mis de côté, quels que soient leurs noms. Ainsi `report (1).pdf`, une branche `fix.Conflict`, un tag `v1.0.conflicted` ou `config-backup` sont des noms ordinaires.
- Les sous-modules sont vérifiés depuis l'intérieur du sous-module. Depuis le super-projet, une copie de conflit dans `.git/modules/<name>/` n'est signalée que lorsque son original est à côté, et un sous-module nommé comme une copie (`lib 2`) est signalé comme tel.
- L'index manquant d'un worktree lié est signalé quand ce worktree est vérifié, pas quand le principal l'est.
- Avec `.git` lié symboliquement à un magasin hors du dossier synchronisé, lancez git et la vérification depuis l'arbre de travail lié. Depuis le répertoire du magasin, qui ne contient aucun des fichiers, un index vidé ressemble à un « tout supprimer » délibéré et ne bloque pas le commit.
- Certains commits délibérés nécessitent `--no-verify` : arrêter de suivre des fichiers qui restent sur le disque, supprimer tous les fichiers là où les objets ne doivent pas être lus, un renommage qui ne change que la casse ou la forme Unicode d'un fichier ayant un autre lien physique, et, dans un sparse checkout (en mode cone ou non), tout commit qui supprime ou renomme un fichier suivi ou le déplace par un changement de type (un fichier devenu répertoire, un répertoire devenu lien symbolique ou sous-module). Ce sont alors les motifs sparse de git qui décident quels fichiers sont sur le disque ; une suppression délibérée ne peut donc pas être distinguée d'un index perdu ; les motifs ne sont pas lus.
- Si le client de synchronisation perd la ref de branche elle-même (son fichier libre supprimé avant l'arrivée d'un nouveau `packed-refs`) en même temps que l'index, HEAD ressemble à une nouvelle branche et le commit n'est pas bloqué : il démarre un nouvel historique ne contenant que les fichiers indexés. Rien n'est supprimé du disque, et l'ancien historique est toujours dans le reflog (`git reflog`).
- Avec un index fractionné, ou là où les objets ne doivent pas être lus (clone partiel avec un git antérieur à 2.45, alternatifs uniquement dans le cloud), l'index n'est pas comparé à HEAD fichier par fichier : seul un index manquant ou vide est détecté.
- Avec `core.splitIndex`, un index qui ne liste aucun fichier n'est pas reconnu (son en-tête ne compte que la partie fractionnée), donc un commit fait à partir de lui n'est pas bloqué.
- Hooks : seuls les hooks de commit sont retrouvés à partir du processus git qui les a lancés. Les autres hooks (`pre-push`, `pre-applypatch`) doivent lancer la vérification depuis l'arbre de travail, sinon ils sont refusés (sortie 2). Là où le processus git n'est pas visible (un client qui exécute lui-même les hooks, ou un listage des processus bloqué), la vérification se rabat sur le répertoire courant : alors, si le répertoire de git ne renvoie pas vers l'arbre de travail (`--separate-git-dir`, `.git.nosync`, `--work-tree`), un index sans fichiers bloque toujours le commit (utilisez `--no-verify` pour un effacement délibéré), un répertoire courant ne contenant aucun des fichiers de HEAD n'est pas vérifié comme arbre de travail (`WORKTREE_GUESSED`), et un hook qui s'est déplacé dans un autre dépôt ou dans le répertoire git est refusé (sortie 2). Les hooks de réception (`pre-receive`) ne sont pas un garde-fou pris en charge.
- `move-out` : un fichier `.git` n'est pas compris par tous les outils. Git et les outils qui lancent git fonctionnent ; isomorphic-git (Obsidian Git sur mobile) et les outils qui ouvrent `.git` comme un répertoire ne fonctionnent pas.
- `move-out` : renommer ou déplacer le dossier synchronisé casse le fichier `.git` relatif. `check` nomme alors le répertoire manquant ; remettez le dossier en place, ou faites pointer à la main le fichier `.git` vers le répertoire git.
- `move-out` exige que le dossier synchronisé soit au même endroit sous le home sur chaque appareil (iCloud Drive l'est toujours ; Dropbox peut être dans `~/Dropbox` sur un appareil et dans `~/Library/CloudStorage/Dropbox` sur un autre). Là où ce n'est pas le cas, le fichier `.git` pointe ailleurs et `--attach` refuse (`TARGET_UNEXPECTED`).
- Le répertoire git déplacé reste sur le système de fichiers du home ; aucune option ne permet de le placer ailleurs.
- `move-out --prepare` copie l'historique de l'appareil à cet instant : faites le déplacement peu après la préparation, sans nouveaux commits entre-temps.
- `move-out` et `fix` ont été essayés dans le dossier propre d'iCloud Drive, pas dans le dossier iCloud d'une app (le `iCloud~md~obsidian` d'Obsidian).

## Contribuer

Voir [CONTRIBUTING.md](../../CONTRIBUTING.md). Signalez les problèmes de sécurité comme décrit dans [SECURITY.md](../../SECURITY.md), et non dans une issue publique.

## Licence

MIT
