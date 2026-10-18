<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="logo de gitbrella: un paraguas cuyo mango es una rama de git">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Encuentra el daño que la sincronización en la nube causa a los repositorios git,<br>
  sin cambiar nada y sin descargar nada.
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
  <b>Español</b> ·
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
  <a href="#usage">Uso</a> ·
  <a href="#repairing">Reparación</a> ·
  <a href="#moving-git-out-of-the-synced-folder">Sacar .git</a> ·
  <a href="#what-it-reports">Qué informa</a> ·
  <a href="#install">Instalación</a> ·
  <a href="#limits">Límites</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive y Syncthing sincronizan los archivos uno a uno. Git espera que su directorio `.git` cambie como un todo, así que los repositorios en carpetas sincronizadas acaban con copias de conflicto como `.git/index 2` o `refs/heads/main 2`, índices desaparecidos junto a archivos `index.lock` abandonados y objetos que solo existen en la nube. Las propias FAQ de Git advierten contra guardar un repositorio en una carpeta sincronizada, pero los valores predeterminados siguen poniéndolos ahí: Escritorio y Documentos de macOS en iCloud, la copia de seguridad de carpetas de Windows en OneDrive, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Los clientes de sincronización copian .git entre dispositivos archivo por archivo y dejan copias de conflicto como index 2 y refs/heads/main 2, un index.lock junto a un índice perdido y objetos que solo existen en la nube">
</p>

`gitbrella check` te dice, normalmente en menos de un segundo, si un repositorio tiene ese daño. Está pensado para ejecutarse antes de que un agente de IA o un hook de git toque el repositorio. `gitbrella fix` repara lo que tiene una única reparación correcta, y `gitbrella move-out` acaba con el daño para siempre: mueve `.git` fuera de la carpeta sincronizada y deja un archivo `.git` que apunta a él, de modo que el cliente de sincronización transporta tus archivos y git se guarda su historial para sí.

| Comando | Qué hace |
| --- | --- |
| `gitbrella check` | informa del daño; solo lectura |
| `gitbrella fix` | repara lo que tiene una única respuesta correcta |
| `gitbrella move-out` | saca `.git` de la carpeta sincronizada para siempre |
| `gitbrella undo` | revierte la reparación o el traslado más reciente |
| `gitbrella history` | lista los cambios hechos en este dispositivo |
| `gitbrella prune-backups` | borra copias de seguridad antiguas de cambios que ya no se pueden deshacer |
| `gitbrella hook --for claude\|codex` | protege las llamadas a herramientas de los agentes de IA |

## Inicio rápido

**1. Mira.** Solo lectura, sin descargas, sin red:

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # salida 0: sano, 1: daño, 2: no pudo ejecutarse
```

**2. Repara** lo que tiene una única respuesta correcta. Sin `--apply` solo muestra el plan:

```sh
gitbrella fix                 # qué repararía y qué te deja a ti
gitbrella fix --apply         # reparar; `gitbrella undo --apply` lo devuelve todo
```

**3. Acaba con ello para siempre.** Saca `.git` de la carpeta sincronizada:

```sh
gitbrella move-out            # qué pasaría y qué lo impide
gitbrella move-out --apply
```

**Mantenlo protegido.** Bloquea los commits en un repositorio dañado:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Para agentes de IA, consulta [Para agentes](#for-agents).

## Lo que promete

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` es de solo lectura.** Nunca escribe en el repositorio. Git se ejecuta con `GIT_OPTIONAL_LOCKS=0`, con gc automático y mantenimiento desactivados, y solo con comandos que leen. Las pruebas de extremo a extremo para los casos de riesgo (copias del índice, un índice ausente, clones parciales, índices divididos, directorios git movidos) registran el tamaño, el mtime y el inode de cada archivo antes y después de la comprobación y exigen que coincidan.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Sin descargas de la nube.** En macOS desactiva la descarga bajo demanda para su propio proceso (y el git que lanza) antes de tocar el disco; si macOS se niega, la comprobación no se ejecuta (salida 2). Los archivos que solo existen en iCloud se informan, no se descargan.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Sin red.** Los clones parciales nunca descargan objetos ausentes (`GIT_NO_LAZY_FETCH`). Con git anterior a 2.45 se omiten las comprobaciones de objetos.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Silencioso en repositorios sanos.** Un bloqueo solo es daño cuando es antiguo y ningún git que trabaje en el repositorio (en cualquiera de sus worktrees) pudo haberlo escrito: git crea un bloqueo cuando lo toma, así que un git que empezó después no puede tenerlo. Los archivos temporales de git y los archivos de otras herramientas en `.git` son información. Ejecutado sobre 89 repositorios sanos y en uso, no informó de ningún daño.

<a name="usage"></a>

## Uso

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Estado de salida: **0** sin daño, **1** daño encontrado, **2** la comprobación no pudo ejecutarse. Los comandos que cambian algo (`move-out`, `fix`, `undo`) salen con **0** planificado, hecho o nada que hacer (`already`); **1** rechazado (con los motivos), hecho con una advertencia, o con daño que queda para ti (la lista MANUAL de `fix`, que `check` seguiría informando); **2** fallido (revertido o detenido; el JSON conserva lo recuperado y la entrada del diario) o no pudo ejecutarse.

`--deep` también ejecuta un `git fsck` completo (lee todos los objetos, por lo que se omite cuando hay objetos que solo existen en la nube). `--provider` fuerza el cliente de sincronización en lugar de detectarlo por la ruta: `auto` (predeterminado), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage` o `none`. Las carpetas de Syncthing y Nextcloud (u ownCloud) se reconocen por el marcador que su cliente guarda en ellas (`.stfolder`, `.sync_*.db`), estén donde estén. `fix` y `move-out` aceptan el mismo `--provider`: dales el mismo que diste a `check`, ya que algunos nombres de copia solo cuentan para un cliente concreto. En ese caso el JSON indica `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Salida de gitbrella check: DAMAGE INDEX_COPY index 2, reparable con gitbrella fix, e INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>La misma salida como texto</summary>

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

### Como hook de git

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit ejecuta el hook pre-commit, que ejecuta gitbrella check: la salida 0 deja pasar el commit, 1 o 2 lo bloquea">
</p>

Un repositorio dañado bloquea el commit; uno sano hace el commit como siempre. La herramienta sabe que git está ejecutando el hook y no confunde el bloqueo del propio commit con uno abandonado.

**Qué repositorio se comprueba.** En los hooks de commit (git les pasa `GIT_INDEX_FILE` o un `GIT_DIR` absoluto) encuentra el repositorio a partir del proceso git que ejecutó el hook, así que el hook puede cambiar antes de directorio: al directorio git, a su directorio de hooks o a un directorio fuera de cualquier repositorio. Cuando git nombra el repositorio con un `GIT_DIR` absoluto (worktrees enlazados, submódulos, directorios git separados), cada comando git del hook actúa sobre él vaya donde vaya el hook, y la comprobación también. En otro caso mira adónde llegaría el propio git desde el directorio actual (el directorio git que nombra allí un `GIT_DIR` relativo, o el repositorio encontrado desde allí): si es otro repositorio u otro worktree del mismo (un hook que entra en él, o el editor del mensaje de commit), se niega (salida 2) en lugar de responder por el equivocado.

Ejecutado sin ruta, comprueba el repositorio que git exportó al hook (`GIT_DIR`), así que también funcionan las configuraciones de dotfiles con `--git-dir` y los directorios git dentro de su propio worktree. También respeta un `GIT_DIR` definido en tu shell cuando no se da ruta; a diferencia de git, ignora un `GIT_DIR` que no existe y rechaza un worktree enlazado cuyo origen no puede rastrear, en lugar de tomar el directorio actual como árbol de trabajo.

**El índice que registra el commit.** Lee `GIT_INDEX_FILE` cuando nombra un archivo dentro del directorio git, como lo define git para los hooks de commit: si el índice que registra el commit falta o está vacío, o ya no lista un archivo de HEAD que sigue en el árbol de trabajo, el commit se bloquea antes de que pueda registrar esos archivos como borrados. Un archivo de índice al que apuntes tú mismo `GIT_INDEX_FILE` en otro lugar no se lee: en un hook se comprueba en su lugar el índice del propio repositorio, y una comprobación ejecutada a mano con esa variable definida se niega (salida 2).

**Eliminaciones deliberadas.** Para dejar de seguir un archivo que conservas en disco (`git rm --cached`, dejar de seguir un directorio que ahora ignoras), o para hacer un commit deliberado de "borrarlo todo" mientras tus archivos siguen en disco (`git rm -r --cached .`), sáltate el hook con `git commit --no-verify`.

### Con un gestor de hooks

- **pre-commit** (pre-commit.com): añade el repositorio con el id de hook `gitbrella` (pre-commit lo compila con Go) o `gitbrella-system` (el binario en PATH); consulta `.pre-commit-hooks.yaml`.
- **lefthook**: incorpora `hooks/lefthook.yml` con `remotes`.
- **husky**: `.husky/pre-commit` con `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }` y después `overcommit --sign`.

Cualquier salida distinta de cero detiene el commit en todos ellos, así que una comprobación que no puede ejecutarse (salida 2) también lo detiene. Aider hace commit con `--no-verify` salvo que se defina `git-commit-verify: true`, y entonces no se ejecuta ningún hook de commit. Obsidian Git (escritorio) hace commit con el git de tu sistema y ejecuta el hook; pon la carpeta que contiene `gitbrella` en su ajuste "Additional PATH", porque las apps abiertas desde el Dock no reciben el PATH de la shell. Obsidian Git en móvil (isomorphic-git) no ejecuta hooks.

<a name="for-agents"></a>

### Para agentes

`--json` imprime el mismo informe como un documento estable y versionado (`schemaVersion: 1`, `findings` es siempre una lista). Decide según el estado de salida y muestra los hallazgos al usuario.

Los hooks propios de los agentes solo detienen una llamada a herramienta con la salida 2: una salida 1 ("daño encontrado") deja seguir la llamada. `gitbrella hook --for claude|codex` habla el idioma de sus hooks. Lee el evento que el agente le pasa por stdin:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="La llamada a herramienta de un agente pasa por gitbrella hook: leer y editar continúan; en un repositorio dañado, las escrituras de git y las escrituras en .git se detienen con salida 2">
</p>

- **Antes de una llamada a herramienta** (`PreToolUse`): en un repositorio dañado, un comando git que escribe (commit, add, reset, rebase, push y cualquier cosa que no se sepa que solo lee) y una escritura dentro de `.git` detienen la llamada (salida 2, con el motivo en stderr para el agente). Leer, editar tus archivos, `git restore` y el propio `gitbrella` continúan, para que el daño pueda examinarse y repararse.
  - *Repositorios inutilizables.* Donde los archivos de git solo existen en la nube, o git no puede usar el repositorio en absoluto (un `HEAD` renombrado, un archivo `.git` que no apunta a ningún sitio), se detiene cualquier comando git: se colgaría o actuaría sobre un repositorio contenedor. Allí las escrituras dentro de `.git` continúan: esa reparación se hace a mano dentro de `.git`, como indica el motivo (`mv '.git/HEAD 2' .git/HEAD`). El remedio que nombra la pista de `core.worktree` (`git config --file … --unset core.worktree`) también continúa.
  - *Qué repositorio.* El que contiene los archivos sobre los que trabaja la llamada (un parche de Codex nombra los suyos; una ruta dentro de `.git` cuenta para el árbol de trabajo de encima). Para un comando de shell, es la carpeta de la sesión y cada carpeta en la que se ejecutan sus comandos git (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), primero las que escriben, como máximo cinco; cuando una que escribe no cabe, la llamada pasa con un aviso. Cada carpeta se juzga por lo que la llamada hace allí: desde una carpeta de sesión dañada, `git -C ../sound commit` continúa. Una carpeta que no es repositorio, y uno bare, pasan sin decir nada.
  - *Cuando la comprobación no puede ejecutarse.* Cualquier cosa que impida ejecutar la comprobación (incluido el `--timeout` de 20 segundos, un único plazo para toda la llamada) deja pasar la llamada con un aviso; `--fail-open` hace lo mismo también donde git no puede usar el repositorio.
  - *Cómo se lee el comando.* Se lee, nunca se ejecuta: comillas, comentarios, subshells, sustituciones (también en un heredoc sin comillas), heredocs, variables que define, `&&`, `||`, `if`, `case` y bucles (después de `cd a || cd b`, o de un `cd` a una carpeta que no existe, cuenta cada carpeta en la que puede estar la shell; un bucle `for` se lee una vez por valor), envoltorios (`timeout`, `sudo`, `nice`, `xargs`, `env`) y los scripts que se dan a una shell (`-c`, `eval`, `watch`, `source`, un archivo, un heredoc, `bash < f`, una tubería hacia `sh`). Las funciones y los alias que define el comando se leen donde se llaman. Un script de shell ejecutado por su ruta o dado a una shell (`./deploy.sh`, `bash deploy.sh`) se lee cuando es un archivo pequeño en esta máquina (también a través de un enlace simbólico, y una sola vez aunque se ejecute a sí mismo). `make`, `npm run` y similares no se leen.
  - *Qué cuenta como escritura.* Otro programa escribiendo dentro de `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` sobre `.git/config`, `rm -rf` o `find -delete` sobre una carpeta que contiene `.git`, un parche de Codex a través de la shell) cuenta como escritura dentro de `.git`; copiar `.git` a otro sitio, empaquetarlo con `tar` o leerlo no. `gh pr checkout` y `git-lfs` cuentan como comandos git que escriben.
  - *En caso de duda.* Cada uno de estos cuenta como un comando git que escribe: una lectura que tarda más de 5000 pasos o la mitad del `--timeout` (en una carpeta que no puede determinar); la palabra `git` en un lugar que la lectura no sigue; un script enviado por tubería a una shell desde un programa que la lectura no sigue (`curl … | sh`), donde se ejecuta; un script dado a una shell que existe pero no puede leerse (demasiado grande, solo en la nube), donde se ejecuta. Cuando no puede determinar la carpeta en la que escribe un comando git (`cd "$(…)"`, `for d in */`), el comando cuenta en todas las carpetas comprobadas y la llamada recibe un aviso.
- **Al iniciar una sesión** (`SessionStart`): una línea en stdout, que el agente lee como contexto, dice que el repositorio está en una carpeta sincronizada (salvo que su directorio git ya esté fuera) y qué está dañado. Nunca bloquea.

El hook no cambia nada. Claude Code, en `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex, en `.codex/hooks.json` (Codex ejecuta un hook solo después de que confíes en él en `/hooks`):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

La comprobación se ejecuta una vez por cada una de esas carpetas, normalmente de 0,1 a 0,4 segundos cada una, así que un parche repartido por varias carpetas de un repositorio lo comprueba varias veces; `--timeout` acota toda la llamada.

<a name="repairing"></a>

## Reparación

```sh
gitbrella fix                 # qué repararía y qué te deja a ti
gitbrella fix --apply         # reparar
gitbrella undo --apply        # devolverlo todo
gitbrella history             # los cambios hechos aquí, sus copias de seguridad y tamaños
gitbrella prune-backups       # qué copias de seguridad ya no puede necesitar ningún undo
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix muestra el plan; fix --apply registra cada paso y luego la comprobación se repite: hecho, o revertido; undo --apply revierte el cambio más reciente">
</p>

`check` indica bajo cada daño si `fix` lo repara (`repair:`, y `"repair": "fix"` o `"manual"` en el JSON), nombrando `fix --provider NAME` cuando a la comprobación se le dio uno; dentro de los hooks de commit no lo pregunta.

`fix` solo repara donde hay una única respuesta correcta y no se sobrescribe nada que pudieras haber querido:

| Daño | Reparación |
| --- | --- |
| copias de conflicto en `.git` junto a su original (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, copias de Dropbox y Syncthing, y una copia del bloqueo de una ref como `refs/heads/main 2.lock`); una copia del índice solo cuando es idéntica al índice o un índice válido igual a HEAD | se mueven a la copia de seguridad; un objeto al que solo apuntaba la copia (un commit, un tag, un árbol o un archivo) se conserva primero como `refs/gcd-rescue/<name>` |
| un directorio de rebase abandonado, `desktop.ini`, `Icon` y archivos parecidos que git no creó (salvo que git lea uno como rama, o que su nombre sea el de un objeto), una copia de `.git` en el árbol de trabajo cuyos commits tiene el repositorio | se mueven a la copia de seguridad |
| índice ausente, sin copia ni bloqueo suyo en ningún sitio | se crea desde HEAD (`git read-tree HEAD`: los archivos no se tocan); solo con un commit desde el que construirlo (si también falta el commit de HEAD, primero se descarga, como se explica abajo) y fuera de un sparse checkout |
| un índice que no lista ningún archivo (un índice válido, no dividido, fuera de un sparse checkout) | se mueve a la copia de seguridad y se crea desde HEAD: no había nada preparado en él |
| un `index.lock` huérfano que es un índice válido igual a HEAD | pasa a ser el índice |
| un archivo de rama vacío | el commit que su reflog registró por última vez |
| `.git.nosync` que git añadiría a un commit | la línea `/.git.nosync/` en `.git/info/exclude` |
| falta el commit de HEAD, o un archivo de objeto suelto vacío (sin copia al lado) | el archivo vacío se mueve a la copia de seguridad y los objetos se descargan de nuevo (`git fetch --refetch`, git 2.36 o posterior) desde `--remote`, si no desde el remoto de la rama, si no desde `origin`, si no desde el único remoto; las ramas y tags del remoto se descargan en un espacio de nombres temporal `refs/gcd-fetch/` que se borra después, así que ninguna ref cambia. Si el remoto no devuelve el objeto, la reparación se revierte |
| con `fix --deep`: objetos que `git fsck` informa como dañados | cada objeto de las ramas y tags del remoto se descarga de nuevo de la misma forma; después `git fsck` se ejecuta otra vez y, si sigue informando errores (un objeto que ningún remoto tiene, un archivo corrupto que permanece), la reparación se revierte |
| copias de conflicto de tus propios archivos (`notes 2.md`) que git no sigue y que contienen los mismos bytes que el archivo que copian | se mueven a la copia de seguridad (`git add -A` las incluiría en un commit); una copia que difiere, está seguida, es un enlace, solo está en la nube o está en un submódulo, se queda |
| el `gc.pid` de otra máquina con más de una hora; un `gc.log` | se mueven a la copia de seguridad, para que el gc automático de git vuelva a ejecutarse aquí |
| archivos de `.git`, o de un almacén de objetos alternativo, que solo existen en la nube | se descargan, y nada más: hasta entonces la comprobación no puede leer los objetos, así que el resto del daño se lista como MANUAL para la siguiente ejecución de `fix` |

Todo lo demás se lista como MANUAL con lo que hay que ejecutar. Eso incluye cualquier cosa que pueda contener trabajo que no esté en ningún otro sitio:
- un índice dañado (git aún puede leerlo, con el trabajo preparado incluido);
- una copia del índice o un `index.lock` que difiere de HEAD, y una copia de `index.lock`;
- una copia cuyo original falta (puede ser la única: devuélvela a su sitio, salvo que sea una copia de un bloqueo, que se rescata y se aparta);
- un bloqueo que ningún git de esta máquina tiene (puede tenerlo git en otro dispositivo o en un contenedor; su consejo nombra el commit que contiene);
- una copia que nombra más de 100 objetos (un reflog largo);
- una copia de la rama que nombra algún HEAD, de su reflog o de un directorio que contiene esa rama (git la lee, se parezca a lo que se parezca su nombre).

También incluye cualquier cosa en la que fix tendría que elegir entre dos versiones, o no podría mantener vivos los commits de una copia:
- cualquier cosa del almacén de objetos: copias, archivos con el nombre de un objeto (comprueba cuál coincide con su hash), archivos de objeto vacíos sin remoto del que descargar;
- cualquier cosa del directorio git de un submódulo (ejecuta `fix` en el submódulo);
- una copia de rama que nombra un commit que aún no está aquí (puede que todavía llegue);
- una copia de `.git` en el árbol de trabajo con commits que le faltan al repositorio (cuál de los dos es el actual lo decides tú);
- un archivo vacío junto a una copia suya o de su directorio;
- un directorio de administración de worktree enlazado roto (`git worktree repair`);
- un índice que perdió archivos que siguen en disco (dejar de seguirlos a propósito se ve igual; cuando no hay nada más preparado, el consejo dice que `git read-tree HEAD` los devuelve sin perder nada);
- objetos que `git fsck` informa como dañados, sin `--deep` o sin remoto.

`fix` no toca en absoluto los repositorios reftable; allí el `REFTABLE_TABLE_UNLISTED` de `check` explica cómo conservar las refs de otro dispositivo. La copia de seguridad debe estar en el disco del repositorio, para que lo que fix aparta se mueva en un solo paso; si no, `fix` se niega (una reparación que solo descarga no mueve nada y no la necesita).

Cada paso se registra antes de actuar y al final la comprobación se repite: si algo que reparó sigue informándose, o apareció daño nuevo, los pasos se revierten. Una reparación interrumpida la revierte el siguiente comando de cambio ejecutado con `--apply` en ese repositorio, que después se detiene; ejecútalo otra vez. Pasados 30 segundos (`--watch`), `fix` comprueba una vez más: el daño que volvió lo trajo de vuelta el cliente de sincronización, y la cura duradera es `move-out`.

`undo` revierte primero el cambio más reciente, una reparación o un traslado. Un paso cuyo resultado cambió desde la reparación se deja como está y se lista, nunca se sobrescribe: git reescribió el índice reconstruido, el cliente de sincronización trajo de vuelta una copia, una ref de rescate se movió, se borró la copia de seguridad o se movió `.git`. Una ref de rescate permanece mientras la copia que mantiene viva siga en la copia de seguridad. Los archivos descargados se quedan en el disco y los objetos descargados de nuevo se quedan en el almacén de objetos: ninguno sustituye nada. El resto se revierte y el undo termina, para que el cambio anterior pueda deshacerse a continuación.

`history` lista los cambios que este dispositivo hizo en el repositorio (reparaciones, traslados, deshaceres), la copia de seguridad que conserva cada uno con su tamaño y cuál revertirá `undo` a continuación. `prune-backups` borra, solo con `--apply`, las copias de seguridad de cambios deshechos, revertidos o que nunca se pudieron deshacer (un undo, un prepare, un attach) y cuyo último paso es más antiguo que `--older-than` (por defecto `720h`). Una reparación o un traslado que aún se puede deshacer conserva su copia de seguridad, aunque haya cambios más recientes encima.

<a name="moving-git-out-of-the-synced-folder"></a>

## Sacar .git de la carpeta sincronizada

```sh
gitbrella move-out            # qué pasaría y qué lo impide
gitbrella move-out --apply    # hacerlo
gitbrella undo --apply        # devolverlo a su sitio
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Antes: .git dentro de la carpeta sincronizada. Después de move-out: un archivo .git en la carpeta sincronizada apunta a ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d, fuera de ella">
</p>

`move-out` copia `.git` a `~/.local/share/gitbrella/gitdirs/<name>` (un clon APFS en macOS: no ocupa espacio), demuestra archivo por archivo que la copia es idéntica, mueve el original a una copia de seguridad en `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`) y escribe un archivo `.git` como `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. La ruta es relativa, así que cada dispositivo que tenga la carpeta en el mismo lugar bajo su home escribe el mismo archivo `.git`. Después comprueba que git encuentra el mismo HEAD, estado, configuración y directorio de hooks, y durante 30 segundos (`--watch`) que el cliente de sincronización deja `.git` en paz y, con iCloud, sube el archivo nuevo. El original se aparta antes de la última comparación, así que una escritura que se coló durante la copia se detecta y el traslado se deshace. Con iCloud, `move-out` se niega mientras iCloud siga subiendo el repositorio; `--wait-upload 10m` espera hasta ese tiempo en su lugar.

Cada cambio se registra en un diario antes de producirse, con el dispositivo y el inode de todo lo que crea (y los bytes del archivo `.git`); una reversión no toca nada más. Si el proceso muere a mitad, el siguiente `move-out --apply` (o `undo --apply`) lo termina o lo revierte y se detiene ahí (ejecuta el comando de nuevo para seguir); lo encuentra por el inode de la carpeta incluso con otra grafía, tras un renombrado o dentro de otro repositorio, y mientras tanto `check` informa del cambio inacabado como daño. Una vez que git ha usado el directorio movido, nada se revierte sobre él: si contiene algo que le falta al original, el comando se detiene y nombra ambos. No se borra nada: el original permanece en la copia de seguridad hasta que lo elimines. `undo` devuelve el directorio git que está en uso ahora, con los commits hechos desde el traslado; la copia de seguridad de antes del traslado se deja en paz. Si el cliente de sincronización ha desalojado el archivo `.git` (en este Mac solo está su nombre), `undo` y la recuperación te piden que primero lo descargues: leerlo lo descargaría, y la herramienta nunca hace eso.

Si una reversión encuentra tanto el original (en la copia de seguridad) como un `.git` nuevo (creado después de que el traslado se detuviera, por ti o por el cliente de sincronización), no sobrescribe ninguno y lo dice: compáralos, saca de la carpeta el `.git` que no quieras y ejecuta el comando de nuevo.

Se niega (salida 1, con los motivos) mientras git o algo que tenga un bloqueo de git trabaja en el repositorio, durante un merge o rebase, mientras haya archivos de `.git` que solo existen en la nube, mientras iCloud siga subiendo la carpeta, mientras haya un socket (un `git fsmonitor--daemon` en ejecución) en `.git`, cuando existen dos directorios git tras un traslado, y con disposiciones cuyos enlaces se romperían: worktrees enlazados, submódulos, objetos prestados (`alternates`), un `.git` que es un enlace simbólico o que no es el propio directorio git. El resto del daño que encuentra `check` se mueve tal cual y se lista como trasladado: ejecuta `check` otra vez después.

### Otros dispositivos

El cliente de sincronización borra `.git` en todos los demás dispositivos que sincronizan la carpeta y lleva en su lugar el archivo `.git`. Para conservar su historial:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: en cada uno de los otros dispositivos, move-out --prepare --apply. 2: en un dispositivo, move-out --apply. 3: en cada uno de los otros dispositivos, move-out --attach --apply">
</p>

1. En cada uno de los otros dispositivos, antes del traslado: `gitbrella move-out --prepare --apply` copia su `.git` al lugar al que apuntará el archivo `.git`.
2. En un dispositivo: `gitbrella move-out --apply`.
3. En cada uno de los otros dispositivos, una vez que haya llegado el archivo `.git`: `gitbrella move-out --attach --apply`. Un dispositivo que no se preparó clona el historial en su lugar: `--attach --from <remote URL> --apply`. En ambos casos el árbol de trabajo no se toca.

El archivo `.git` llega a través del cliente de sincronización, así que attach confía en él solo hasta donde llegan los registros de este dispositivo: debe apuntar dentro del `~/.local/share/gitbrella/gitdirs` de este dispositivo, y un directorio que ya esté allí solo se usa si `--prepare` lo creó para este repositorio.

A partir de entonces el cliente de sincronización transporta los archivos y no el historial. Comparte los commits a través de un remoto. En un dispositivo cuyos archivos ya actualizó el cliente de sincronización, `git pull` se niega ("would be overwritten"): usa `git fetch` y `git reset --mixed @{u}`, que mueve la rama y el índice y deja los archivos sincronizados como están.

<a name="what-it-reports"></a>

## Qué informa

Daño (salida 1):

| Código | Significado |
| --- | --- |
| `INDEX_COPY` | copia de conflicto del índice (`index 2`; nombres de Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive y Seafile; `index (1)` de Google Drive, `index-COMPUTER` de OneDrive en una carpeta de OneDrive, y los nombres del Nextcloud antiguo, Synology Drive, pCloud y Resilio, que solo se buscan para los nombres de archivo propios de git); indica si es un índice válido y si coincide con HEAD |
| `REF_COPY` | copia de conflicto de una ref de rama o tag; indica si la copia es más antigua, más nueva, divergente o nombra un commit ausente |
| `COPY_IN_GITDIR` | cualquier otra copia de conflicto dentro de `.git` |
| `INDEX_MISSING` | el índice ha desaparecido; git mostraría todos los archivos como borrados (dentro de un hook de commit: el commit borraría todos los archivos) |
| `INDEX_EMPTY` | el índice no lista ningún archivo mientras los archivos de HEAD siguen en el árbol de trabajo, como cuando git lo reconstruyó después de que el cliente de sincronización lo borrara; un commit borraría todos los archivos |
| `INDEX_SHRUNK` | el índice ya no lista archivos de HEAD que siguen en el árbol de trabajo (en un sparse checkout: cualquier archivo de HEAD), como cuando git lo reconstruyó a partir de los archivos preparados después de que el cliente de sincronización lo borrara; un commit los borraría. No cuentan los renombrados, un directorio convertido en enlace simbólico o en un submódulo que declara `.gitmodules`, ni los archivos borrados también del disco; cuando no se pueden listar los archivos de HEAD se informa como no verificado |
| `HEAD_COMMIT_MISSING` | HEAD nombra un commit que no está en el repositorio (la ref llegó antes que sus objetos) |
| `ORPHAN_INDEX_LOCK` | el índice ha desaparecido y quedó un `index.lock` antiguo (a menudo un índice completo) |
| `INDEX_INVALID` | el índice no supera su propia suma de comprobación |
| `STALE_LOCK` | un bloqueo de git que ningún git en ejecución puede tener: ninguno trabaja en el repositorio, o todos empezaron después de que se escribiera el bloqueo. Solo se ven los procesos git de esta máquina: antes de quitar el bloqueo, asegúrate de que ningún git use el repositorio en un contenedor, en otra máquina que comparta la carpeta o dentro de una app que incluya git |
| `DATALESS_IN_GITDIR` | hay archivos de `.git` que solo existen en la nube; se omitieron las comprobaciones de objetos |
| `FOREIGN_FILE_IN_GITDIR` | un archivo que git no creó en `refs/` u `objects/` (`desktop.ini`, archivos `.tmp` en `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | un objeto o ref de cero bytes |
| `GITDIR_COPY` | una copia del directorio git en el árbol de trabajo (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | el directorio git está dentro del árbol de trabajo con otro nombre (`.git.nosync`) y `git add -A` lo incluiría en un commit |
| `WORKTREE_ADMIN_BROKEN` | un worktree enlazado perdió sus archivos de administración |
| `STALE_OPERATION_STATE` | un directorio de rebase abandonado hace creer a git que hay un rebase en curso |
| `FSCK_ERRORS` | solo con `--deep`: `git fsck` informó errores (cuando no termina, el mismo código es información) |
| `REFTABLE_TABLE_UNLISTED` | un repositorio reftable contiene tablas de refs que su lista no nombra: las de otro dispositivo, traídas por el cliente de sincronización mientras su lista se convertía en una copia que git nunca lee. El siguiente `git pack-refs` o mantenimiento las borra, y con ellas las ramas de ese dispositivo; copia antes el directorio git a un lugar seguro |
| `CHANGE_UNFINISHED` | un `fix`, `undo` o `move-out` de gitbrella no terminó (se interrumpió); ejecuta con `--apply` el comando que indica para terminarlo o revertirlo antes de usar git allí |
| `GIT_DIR_RETURNED` | después de `move-out`, `.git` vuelve a ser un directorio (el cliente de sincronización trajo de vuelta el antiguo), así que existen dos directorios git; compáralos y quédate con uno antes de hacer commit |
| `STAGED_CONFLICT_COPY` | el índice añade una copia de conflicto de sincronización de un archivo que también lista (`notes 2.md` junto a `notes.md`, un directorio copiado `src 2/`, nombres de Syncthing); un commit metería la copia en el historial. Solo deciden los nombres; no cuentan un renombrado a uno de esos nombres ni un archivo con nombre de copia sin su original; no se comprueba con un índice dividido |

Información (el estado de salida no cambia): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (el directorio git se sacó de la carpeta sincronizada: la disposición segura), `WORKTREE_COPIES` (copias de conflicto de tus propios archivos), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (un almacén de objetos alternativo contiene archivos solo en la nube, así que se omitieron las comprobaciones de objetos), `GC_PID_FOREIGN` (el `gc.pid` de otra máquina: git omite su gc automático durante 12 horas), `GC_LOG_FRESH` (un `gc.log` de menos de un día detiene el gc automático), `HOOKS_IN_SYNCED_FOLDER` (`core.hooksPath` o un comando `core.fsmonitor` está en una carpeta sincronizada fuera del directorio git, así que un cambio en otro dispositivo se ejecuta aquí), `WORKTREE_GUESSED` (`GIT_DIR` sin árbol de trabajo, y ninguno de los archivos de HEAD está en el directorio actual: el árbol de trabajo no se comprobó).

Cuando la ruta no es en absoluto un repositorio, dice por qué si la causa está en el disco: un `.git 2` renombrado, archivos propios de git que solo existen en la nube, un directorio git sin HEAD, un archivo `.git` que apunta a un directorio git que ya no existe, o un `core.worktree` que apunta a una carpeta que esta máquina no tiene (un ajuste de otra máquina que trajo el cliente de sincronización). Si la ruta tiene un `.git` que git no puede usar, se detiene ahí (salida 2) en lugar de responder por un repositorio contenedor, que es lo que haría el propio git.

<a name="install"></a>

## Instalación

Requiere git 2.31 o posterior. macOS y Linux, amd64 y arm64, sin cgo.

```
brew install aliyilmazco/tap/gitbrella
```

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

o descarga un archivo desde la página de releases.

### Verificar una versión

Cada versión incluye `checksums.txt` (el SHA-256 de cada archivo y SBOM), su firma cosign sin claves `checksums.txt.sigstore.json` hecha por el flujo de trabajo de release, un SBOM SPDX por cada archivo y una atestación de procedencia de compilación de GitHub. Comprueba primero la firma y después el archivo contra la lista firmada (el ejemplo es v1.2.3 para Apple silicon). Usa cosign v3.1.3 o posterior (v2.6.5 en la línea v2): las versiones anteriores pueden ser engañadas con un paquete falsificado (GHSA-fx35-mq7g-6g98).

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

Una versión la compila `.github/workflows/release.yml` a partir de un tag enviado, con `.goreleaser.yaml`, y queda como borrador hasta que una persona la publica. Los binarios se compilan con `-trimpath` y la hora del commit, así que el mismo tag y la misma versión de Go (el flujo toma el parche 1.26 más reciente) compilan los mismos binarios; los archivos y SBOM que los rodean llevan su hora de compilación.

<a name="limits"></a>

## Límites

- `fix` solo repara donde hay una única respuesta correcta y te deja el resto con el comando que hay que ejecutar; la solución duradera es `move-out`, que mantiene `.git` fuera de la carpeta sincronizada.
- Las copias de conflicto creadas por el propio iCloud entre dos dispositivos no se reprodujeron en las pruebas; la detección se prueba con copias que siguen el mismo patrón de nombres.
- Codex no pasa a los hooks el `workdir` de su herramienta de shell: un comando que Codex ejecuta en otra carpeta se comprueba en la carpeta de la sesión.
- Windows y OneDrive en Windows no están probados. Los repositorios reftable reciben las comprobaciones a nivel de archivo y la de tablas no listadas; allí las copias de refs no se comparan con sus ramas.
- Los nombres de copia cuya marca podría llevar un nombre a propósito solo se buscan donde el propio git crea el nombre, nunca en el árbol de trabajo: el `index (1)` de Google Drive para los archivos propios de git y en `refs/`; el `index-COMPUTER` de OneDrive para los archivos propios de git en la raíz de una carpeta de OneDrive, con una mayúscula en el nombre del dispositivo y sin una palabra con la que la gente termina el nombre de una copia de seguridad (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` son tuyos), de 15 caracteres como máximo (un nombre de Mac más largo se pasa por alto); el `_conflict-YYYYMMDD-HHMMSS` del Nextcloud antiguo, el de Synology Drive, el `.conflicted` de pCloud y el `.Conflict` de Resilio para los archivos propios de git en la raíz (el del Nextcloud antiguo también en `refs/` y `logs/` de una carpeta de Nextcloud, junto a su original). Una rama que nombra algún HEAD (como `ref:` con cualquier espaciado, o como enlace simbólico), su reflog y un directorio que la contiene nunca se apartan, tengan el nombre que tengan. Así que `report (1).pdf`, una rama `fix.Conflict`, un tag `v1.0.conflicted` o `config-backup` son nombres corrientes.
- Los submódulos se comprueban desde dentro del submódulo. Desde el superproyecto, una copia de conflicto dentro de `.git/modules/<name>/` solo se informa cuando su original está al lado, y un submódulo con nombre de copia (`lib 2`) se informa como copia.
- El índice ausente de un worktree enlazado se informa cuando se comprueba ese worktree, no cuando se comprueba el principal.
- Con `.git` enlazado simbólicamente a un almacén fuera de la carpeta sincronizada, ejecuta git y la comprobación desde el árbol de trabajo enlazado. Desde el directorio del almacén, que no contiene ninguno de los archivos, un índice vaciado parece un "borrarlo todo" deliberado y no bloquea el commit.
- Algunos commits deliberados necesitan `--no-verify`: dejar de seguir archivos que permanecen en disco, borrar todos los archivos donde no deben leerse objetos, un renombrado que solo cambia mayúsculas o la forma Unicode de un archivo con otro enlace duro, y en un sparse checkout (en modo cone o no) cualquier commit que elimine o renombre un archivo seguido o lo mueva con un cambio de tipo (un archivo que pasa a ser directorio, un directorio que pasa a ser enlace simbólico o submódulo). Allí los patrones sparse de git deciden qué archivos están en disco, así que una eliminación deliberada no se distingue de un índice perdido; los patrones no se leen.
- Si el cliente de sincronización pierde la propia ref de la rama (su archivo suelto borrado antes de que llegue un `packed-refs` nuevo) junto con el índice, HEAD parece una rama nueva y el commit no se bloquea: inicia un historial nuevo que solo contiene los archivos preparados. No se borra nada del disco, y el historial antiguo sigue en el reflog (`git reflog`).
- Con un índice dividido, o donde no deben leerse objetos (clon parcial con git anterior a 2.45, alternativos solo en la nube), el índice no se compara con HEAD archivo por archivo: solo se detecta un índice ausente o vacío.
- Con `core.splitIndex`, un índice que no lista ningún archivo no se reconoce (su cabecera solo cuenta la parte dividida), así que un commit desde él no se bloquea.
- Hooks: solo los hooks de commit se encuentran a partir del proceso git que los ejecutó. Otros hooks (`pre-push`, `pre-applypatch`) deben ejecutar la comprobación desde el árbol de trabajo, o se rechazan (salida 2). Donde no se ve el proceso git (un cliente que ejecuta los hooks por sí mismo, o el listado de procesos bloqueado), la comprobación recurre al directorio actual: entonces, si el directorio de git no apunta de vuelta al árbol de trabajo (`--separate-git-dir`, `.git.nosync`, `--work-tree`), un índice sin archivos siempre bloquea el commit (usa `--no-verify` para un borrado deliberado), un directorio actual sin ninguno de los archivos de HEAD no se comprueba como árbol de trabajo (`WORKTREE_GUESSED`), y un hook que entró en otro repositorio o en el directorio git se rechaza (salida 2). Los hooks de recepción (`pre-receive`) no son una barrera admitida.
- `move-out`: no todas las herramientas entienden un archivo `.git`. Git y las herramientas que ejecutan git funcionan; isomorphic-git (Obsidian Git en móvil) y las herramientas que abren `.git` como directorio no.
- `move-out`: renombrar o mover la carpeta sincronizada rompe el archivo `.git` relativo. `check` nombra entonces el directorio ausente; devuelve la carpeta a su sitio o apunta a mano el archivo `.git` al directorio git.
- `move-out` necesita la carpeta sincronizada en el mismo lugar bajo el home en cada dispositivo (iCloud Drive siempre lo está; Dropbox puede estar en `~/Dropbox` en un dispositivo y en `~/Library/CloudStorage/Dropbox` en otro). Donde no lo está, allí el archivo `.git` apunta a otro sitio y `--attach` se niega (`TARGET_UNEXPECTED`).
- El directorio git movido se queda en el sistema de archivos del home; no hay opción para ponerlo en otro sitio.
- `move-out --prepare` copia el historial del dispositivo en ese momento: haz el traslado poco después de preparar, sin commits nuevos entre medias.
- `move-out` y `fix` se probaron en la propia carpeta de iCloud Drive, no dentro de la carpeta de iCloud de una app (`iCloud~md~obsidian` de Obsidian).

## Contribuir

Consulta [CONTRIBUTING.md](../../CONTRIBUTING.md). Informa de los problemas de seguridad como se describe en [SECURITY.md](../../SECURITY.md), no en un issue público.

## Licencia

MIT
