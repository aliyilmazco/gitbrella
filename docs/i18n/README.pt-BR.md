<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="logo do gitbrella: um guarda-chuva cujo cabo é um branch do git">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Encontra o dano que a sincronização em nuvem causa a repositórios git,<br>
  sem alterar nada e sem baixar nada.
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
  <b>Português</b> ·
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
  <a href="#repairing">Reparo</a> ·
  <a href="#moving-git-out-of-the-synced-folder">Tirar o .git</a> ·
  <a href="#what-it-reports">O que relata</a> ·
  <a href="#install">Instalação</a> ·
  <a href="#limits">Limites</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive e Syncthing sincronizam os arquivos um por um. O Git espera que seu diretório `.git` mude como um todo, então repositórios em pastas sincronizadas acabam com cópias de conflito como `.git/index 2` ou `refs/heads/main 2`, índices desaparecidos ao lado de arquivos `index.lock` esquecidos e objetos que existem só na nuvem. O próprio FAQ do Git desaconselha manter um repositório numa pasta sincronizada, mas as configurações padrão continuam colocando-os lá: Mesa e Documentos do macOS no iCloud, o backup de pastas do Windows no OneDrive, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Clientes de sincronização copiam o .git entre dispositivos arquivo por arquivo e deixam cópias de conflito como index 2 e refs/heads/main 2, um index.lock ao lado de um índice perdido e objetos que existem só na nuvem">
</p>

`gitbrella check` diz, normalmente em menos de um segundo, se um repositório tem esse dano. Ele foi feito para rodar antes que um agente de IA ou um hook do git toque no repositório. `gitbrella fix` repara o que tem um único reparo correto, e `gitbrella move-out` acaba com o dano de vez: move o `.git` para fora da pasta sincronizada e deixa um arquivo `.git` apontando para ele, de modo que o cliente de sincronização leva seus arquivos e o git guarda o histórico para si.

| Comando | O que faz |
| --- | --- |
| `gitbrella check` | relata o dano; somente leitura |
| `gitbrella fix` | repara o que tem uma única resposta correta |
| `gitbrella move-out` | tira o `.git` da pasta sincronizada de vez |
| `gitbrella undo` | desfaz o reparo ou a mudança mais recente |
| `gitbrella history` | lista as alterações feitas neste dispositivo |
| `gitbrella prune-backups` | apaga backups antigos de alterações que não podem mais ser desfeitas |
| `gitbrella hook --for claude\|codex` | protege as chamadas de ferramentas de agentes de IA |

## Início rápido

**1. Olhe.** Somente leitura, sem downloads, sem rede:

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # saída 0: saudável, 1: dano, 2: não pôde rodar
```

**2. Repare** o que tem uma única resposta correta. Sem `--apply` ele só mostra o plano:

```sh
gitbrella fix                 # o que ele repararia e o que deixa para você
gitbrella fix --apply         # reparar; `gitbrella undo --apply` desfaz tudo
```

**3. Resolva de vez.** Tire o `.git` da pasta sincronizada:

```sh
gitbrella move-out            # o que aconteceria e o que impede
gitbrella move-out --apply
```

**Mantenha protegido.** Bloqueie commits em um repositório danificado:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Para agentes de IA, veja [Para agentes](#for-agents).

## O que ele promete

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` é somente leitura.** Nunca escreve no repositório. O git roda com `GIT_OPTIONAL_LOCKS=0`, gc automático e manutenção desligados, e apenas comandos que leem. Testes de ponta a ponta para os casos de risco (cópias do índice, índice ausente, clones parciais, índices divididos, diretórios git movidos) registram o tamanho, o mtime e o inode de cada arquivo antes e depois da verificação e exigem que sejam iguais.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Sem downloads da nuvem.** No macOS ele desliga o download sob demanda para o próprio processo (e para o git que ele inicia) antes de tocar no disco; se o macOS recusar, a verificação não roda (saída 2). Arquivos que existem só no iCloud são relatados, não baixados.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Sem rede.** Clones parciais nunca buscam objetos ausentes (`GIT_NO_LAZY_FETCH`). Em git anterior ao 2.45, as verificações de objetos são puladas.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Silencioso em repositórios saudáveis.** Um lock só é dano quando é antigo e nenhum git trabalhando no repositório (em qualquer de suas worktrees) poderia tê-lo escrito: o git cria um lock quando o adquire, então um git que começou depois não pode estar com ele. Os arquivos temporários do git e os arquivos de outras ferramentas em `.git` são informação. Rodando em 89 repositórios saudáveis em uso, ele não relatou nenhum dano.

<a name="usage"></a>

## Uso

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Status de saída: **0** nenhum dano, **1** dano encontrado, **2** a verificação não pôde rodar. Os comandos que alteram algo (`move-out`, `fix`, `undo`) saem com **0** planejado, feito ou nada a fazer (`already`); **1** recusado (com os motivos), feito com um aviso, ou com dano deixado para você (a lista MANUAL do `fix`, que o `check` continuaria relatando); **2** falhou (revertido ou interrompido; o JSON guarda o que foi recuperado e a entrada do diário) ou não pôde rodar.

`--deep` também roda um `git fsck` completo (ele lê todos os objetos, então é pulado quando há objetos só na nuvem). `--provider` força o cliente de sincronização em vez de detectá-lo pelo caminho: `auto` (padrão), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage` ou `none`. Pastas do Syncthing e do Nextcloud (ou ownCloud) são reconhecidas pelo marcador que o cliente mantém nelas (`.stfolder`, `.sync_*.db`), onde quer que estejam. `fix` e `move-out` aceitam o mesmo `--provider`: passe o mesmo que você passou ao `check`, pois alguns nomes de cópia só contam para um cliente específico. Nesse caso o JSON diz `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Saída do gitbrella check: DAMAGE INDEX_COPY index 2, reparável com gitbrella fix, e INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>A mesma saída em texto</summary>

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

### Como hook do git

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit roda o hook pre-commit, que roda gitbrella check: a saída 0 deixa o commit seguir, 1 ou 2 o bloqueia">
</p>

Um repositório danificado bloqueia o commit; um saudável faz o commit normalmente. A ferramenta sabe que é o git que está rodando o hook e não confunde o lock do próprio commit com um lock esquecido.

**Qual repositório é verificado.** Em hooks de commit (o git passa a eles `GIT_INDEX_FILE` ou um `GIT_DIR` absoluto), ele encontra o repositório a partir do processo git que rodou o hook, então o hook pode mudar de diretório antes: para o diretório git, para o diretório de hooks dele ou para um diretório fora de qualquer repositório. Quando o git nomeia o repositório com um `GIT_DIR` absoluto (worktrees vinculadas, submódulos, diretórios git separados), todo comando git no hook age sobre ele aonde quer que o hook vá, e a verificação também. Caso contrário, ele olha aonde o próprio git chegaria a partir do diretório atual (o diretório git que um `GIT_DIR` relativo nomeia ali, ou o repositório encontrado a partir dali): se for outro repositório ou outra worktree do mesmo (um hook que entra nele, ou o editor da mensagem de commit), ele recusa (saída 2) em vez de responder pelo repositório errado.

Rodado sem caminho, ele verifica o repositório que o git exportou para o hook (`GIT_DIR`), então configurações de dotfiles com `--git-dir` e diretórios git dentro da própria worktree também funcionam. Ele também respeita um `GIT_DIR` definido no seu shell quando nenhum caminho é dado; diferente do git, ignora um `GIT_DIR` que não existe e recusa uma worktree vinculada cuja origem não consegue rastrear, em vez de tomar o diretório atual como árvore de trabalho.

**O índice que o commit registra.** Ele lê `GIT_INDEX_FILE` quando este nomeia um arquivo dentro do diretório git, como o git define para hooks de commit: se o índice que o commit registra estava ausente ou vazio, ou não lista mais um arquivo do HEAD que ainda está na árvore de trabalho, o commit é bloqueado antes de registrar esses arquivos como apagados. Um arquivo de índice em outro lugar para o qual você mesmo aponte `GIT_INDEX_FILE` não é lido: num hook, o índice do próprio repositório é verificado no lugar, e uma verificação rodada à mão com essa variável definida recusa (saída 2).

**Remoções intencionais.** Para deixar de rastrear um arquivo que você mantém no disco (`git rm --cached`, deixar de rastrear um diretório que agora você ignora), ou para fazer um commit intencional de "apagar tudo" enquanto seus arquivos ainda estão no disco (`git rm -r --cached .`), pule o hook com `git commit --no-verify`.

### Com um gerenciador de hooks

- **pre-commit** (pre-commit.com): adicione o repositório com o id de hook `gitbrella` (o pre-commit o compila com Go) ou `gitbrella-system` (o binário no PATH); veja `.pre-commit-hooks.yaml`.
- **lefthook**: traga `hooks/lefthook.yml` com `remotes`.
- **husky**: `.husky/pre-commit` contendo `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }` e depois `overcommit --sign`.

Qualquer saída diferente de zero interrompe o commit em todos eles, então uma verificação que não pôde rodar (saída 2) também o interrompe. O Aider faz commit com `--no-verify`, a menos que `git-commit-verify: true` esteja definido, e aí nenhum hook de commit roda. O Obsidian Git (desktop) faz commit com o git do seu sistema e roda o hook; coloque a pasta que contém o `gitbrella` na configuração "Additional PATH" dele, pois apps abertos pelo Dock não recebem o PATH do shell. O Obsidian Git no celular (isomorphic-git) não roda hooks.

<a name="for-agents"></a>

### Para agentes

`--json` imprime o mesmo relatório como um documento estável e versionado (`schemaVersion: 1`, `findings` é sempre uma lista). Decida pelo status de saída e mostre os achados ao usuário.

Os próprios hooks dos agentes só interrompem uma chamada de ferramenta na saída 2: uma saída 1 ("dano encontrado") deixa a chamada seguir. `gitbrella hook --for claude|codex` fala a língua dos hooks deles. Ele lê o evento que o agente passa pelo stdin:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="A chamada de ferramenta de um agente passa pelo gitbrella hook: leitura e edição seguem; num repositório danificado, escritas do git e escritas em .git param com saída 2">
</p>

- **Antes de uma chamada de ferramenta** (`PreToolUse`): num repositório danificado, um comando git que escreve (commit, add, reset, rebase, push, e qualquer coisa que não se saiba que só lê) e uma escrita dentro de `.git` interrompem a chamada (saída 2, com o motivo no stderr para o agente). Ler, editar seus arquivos, `git restore` e o próprio `gitbrella` seguem, para que o dano possa ser examinado e reparado.
  - *Repositórios inutilizáveis.* Onde os arquivos do git existem só na nuvem, ou o git não consegue usar o repositório de jeito nenhum (um `HEAD` renomeado, um arquivo `.git` apontando para lugar nenhum), todo comando git é interrompido: ele travaria ou agiria sobre um repositório externo. Escritas dentro de `.git` seguem nesse caso: esse reparo é feito à mão dentro de `.git`, como diz o motivo (`mv '.git/HEAD 2' .git/HEAD`). A solução que a dica de `core.worktree` aponta (`git config --file … --unset core.worktree`) também segue.
  - *Qual repositório.* O que contém os arquivos com que a chamada trabalha (um patch do Codex nomeia os seus; um caminho dentro de `.git` conta para a árvore de trabalho acima dele). Para um comando de shell, é a pasta da sessão e cada pasta onde seus comandos git rodam (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), primeiro as que escrevem, no máximo cinco; quando uma que escreve não cabe, a chamada passa com um aviso. Cada pasta é julgada pelo que a chamada faz ali: a partir de uma pasta de sessão danificada, `git -C ../sound commit` segue. Uma pasta que não é repositório, e um repositório bare, passam em silêncio.
  - *Quando a verificação não pode rodar.* Qualquer coisa que impeça a verificação de rodar (incluindo o `--timeout` de 20 segundos, um único prazo para a chamada inteira) deixa a chamada passar com um aviso; `--fail-open` faz isso também onde o git não consegue usar o repositório.
  - *Como o comando é lido.* Ele é lido, nunca executado: aspas, comentários, subshells, substituições (também num heredoc sem aspas), heredocs, variáveis que ele define, `&&`, `||`, `if`, `case` e laços (depois de `cd a || cd b`, ou de um `cd` para uma pasta que não existe, conta cada pasta em que o shell pode estar; um laço `for` é lido uma vez por valor), wrappers (`timeout`, `sudo`, `nice`, `xargs`, `env`) e os scripts entregues a um shell (`-c`, `eval`, `watch`, `source`, um arquivo, um heredoc, `bash < f`, um pipe para `sh`). Funções e aliases que o comando define são lidos onde são chamados. Um script de shell rodado pelo caminho ou entregue a um shell (`./deploy.sh`, `bash deploy.sh`) é lido quando é um arquivo pequeno nesta máquina (também por um link simbólico, e uma só vez mesmo quando roda a si mesmo). `make`, `npm run` e afins não são lidos.
  - *O que conta como escrita.* Outro programa escrevendo dentro de `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` em `.git/config`, `rm -rf` ou `find -delete` numa pasta que contém `.git`, um patch do Codex pelo shell) conta como escrita dentro de `.git`; copiar o `.git` para outro lugar, empacotá-lo com `tar` ou lê-lo não conta. `gh pr checkout` e `git-lfs` contam como comandos git que escrevem.
  - *Na dúvida.* Cada um destes conta como um comando git que escreve: uma leitura que leva mais de 5000 passos ou metade do `--timeout` (numa pasta que ele não consegue determinar); a palavra `git` num lugar que a leitura não acompanha; um script enviado por pipe a um shell a partir de um programa que a leitura não acompanha (`curl … | sh`), onde ele roda; um script entregue a um shell que existe mas não pode ser lido (grande demais, só na nuvem), onde ele roda. Quando não consegue determinar a pasta em que um comando git escreve (`cd "$(…)"`, `for d in */`), o comando conta em todas as pastas verificadas e a chamada recebe um aviso.
- **No início de uma sessão** (`SessionStart`): uma linha no stdout, que o agente lê como contexto, diz que o repositório está numa pasta sincronizada (a menos que seu diretório git já esteja fora dela) e o que está danificado. Nunca bloqueia.

O hook não altera nada. Claude Code, em `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex, em `.codex/hooks.json` (o Codex só roda um hook depois que você confia nele em `/hooks`):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

A verificação roda uma vez para cada uma dessas pastas, normalmente de 0,1 a 0,4 segundo cada, então um patch espalhado por várias pastas de um repositório o verifica várias vezes; `--timeout` limita a chamada inteira.

<a name="repairing"></a>

## Reparo

```sh
gitbrella fix                 # o que ele repararia e o que deixa para você
gitbrella fix --apply         # reparar
gitbrella undo --apply        # devolver tudo como estava
gitbrella history             # as alterações feitas aqui, seus backups e tamanhos
gitbrella prune-backups       # quais backups nenhum undo pode mais precisar
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix mostra o plano; fix --apply registra cada passo e depois a verificação roda de novo: concluído, ou revertido; undo --apply desfaz a alteração mais recente">
</p>

O `check` diz, sob cada dano, se o `fix` o repara (`repair:`, e `"repair": "fix"` ou `"manual"` no JSON), citando `fix --provider NAME` quando a verificação recebeu um; dentro de hooks de commit ele não pergunta.

O `fix` só repara onde há uma única resposta correta e nada que você possa ter querido é sobrescrito:

| Dano | Reparo |
| --- | --- |
| cópias de conflito em `.git` ao lado do original (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, cópias do Dropbox e do Syncthing, e uma cópia do lock de uma ref como `refs/heads/main 2.lock`); uma cópia do índice só quando é idêntica ao índice ou um índice válido igual ao HEAD | movidas para o backup; um objeto para o qual só a cópia apontava (um commit, uma tag, uma tree ou um arquivo) é antes guardado como `refs/gcd-rescue/<name>` |
| um diretório de rebase esquecido, `desktop.ini`, `Icon` e arquivos parecidos que o git não criou (a menos que o git leia um deles como branch, ou que seu nome seja o de um objeto), uma cópia do `.git` na árvore de trabalho cujos commits o repositório tem | movidos para o backup |
| índice ausente, sem cópia nem lock dele em lugar nenhum | criado a partir do HEAD (`git read-tree HEAD`: os arquivos não são tocados); só com um commit a partir do qual construir (quando o commit do HEAD também está ausente, ele é buscado antes, como abaixo) e fora de um sparse checkout |
| um índice que não lista nenhum arquivo (um índice válido, não dividido, fora de um sparse checkout) | movido para o backup e criado a partir do HEAD: nada estava preparado nele |
| um `index.lock` órfão que é um índice válido igual ao HEAD | ele se torna o índice |
| um arquivo de branch vazio | o commit que seu reflog registrou por último |
| `.git.nosync` que o git adicionaria a um commit | a linha `/.git.nosync/` em `.git/info/exclude` |
| commit do HEAD ausente, ou um arquivo de objeto solto vazio (sem cópia ao lado) | o arquivo vazio é movido para o backup e os objetos são buscados de novo (`git fetch --refetch`, git 2.36 ou mais novo) de `--remote`, senão do remoto do branch, senão de `origin`, senão do único remoto; os branches e tags do remoto são buscados num namespace temporário `refs/gcd-fetch/` que é apagado depois, então nenhuma ref muda. Se o remoto não trouxer o objeto de volta, o reparo é revertido |
| com `fix --deep`: objetos que o `git fsck` relata como danificados | todo objeto dos branches e tags do remoto é buscado de novo, do mesmo jeito; o `git fsck` roda de novo depois e, se ainda relatar erros (um objeto que nenhum remoto tem, um arquivo corrompido que permanece), o reparo é revertido |
| cópias de conflito dos seus próprios arquivos (`notes 2.md`) que o git não rastreia e que têm os mesmos bytes do arquivo que copiam | movidas para o backup (`git add -A` as incluiria num commit); uma cópia que difere, é rastreada, é um link, está só na nuvem ou num submódulo permanece |
| o `gc.pid` de outra máquina com mais de uma hora; um `gc.log` | movidos para o backup, para que o gc automático do git volte a rodar aqui |
| arquivos do `.git`, ou de um armazenamento alternativo de objetos, que existem só na nuvem | baixados, e nada mais: até lá a verificação não consegue ler os objetos, então o restante do dano é listado como MANUAL para a próxima execução do `fix` |

Todo o resto é listado como MANUAL, com o que rodar. Isso inclui qualquer coisa que possa conter trabalho que não existe em nenhum outro lugar:
- um índice danificado (o git ainda pode lê-lo, com o trabalho preparado e tudo);
- uma cópia do índice ou um `index.lock` que difere do HEAD, e uma cópia de `index.lock`;
- uma cópia cujo original está ausente (ela pode ser a única: coloque-a de volta, a menos que seja uma cópia de um lock, que é resgatada e posta de lado);
- um lock que nenhum git desta máquina detém (um git em outro dispositivo ou num contêiner pode deter; o conselho dele cita o commit que contém);
- uma cópia que nomeia mais de 100 objetos (um reflog longo);
- uma cópia do branch que algum HEAD nomeia, do reflog dele, ou de um diretório que contém esse branch (o git a lê, seja qual for a aparência do nome).

Inclui também qualquer coisa em que o fix teria de escolher entre duas versões, ou não conseguiria manter vivos os commits de uma cópia:
- qualquer coisa no armazenamento de objetos: cópias, arquivos com o nome de um objeto (verifique qual corresponde ao hash), arquivos de objeto vazios sem remoto do qual buscar;
- qualquer coisa no diretório git de um submódulo (rode o `fix` no submódulo);
- uma cópia de branch que nomeia um commit que ainda não chegou aqui (ele ainda pode chegar);
- uma cópia do `.git` na árvore de trabalho com commits que faltam ao repositório (qual dos dois é o atual, você decide);
- um arquivo vazio ao lado de uma cópia dele ou do seu diretório;
- um diretório de administração de worktree vinculada quebrado (`git worktree repair`);
- um índice que perdeu arquivos que ainda estão no disco (deixar de rastreá-los de propósito parece igual; quando nada mais está preparado, o conselho diz que `git read-tree HEAD` os traz de volta sem perder nada);
- objetos que o `git fsck` relata como danificados, sem `--deep` ou sem um remoto.

O `fix` não toca em repositórios reftable; neles, o `REFTABLE_TABLE_UNLISTED` do `check` diz como preservar as refs de outro dispositivo. O backup precisa estar no disco do repositório, para que o que o fix põe de lado seja movido num único passo; caso contrário o `fix` recusa (um reparo que só baixa não move nada e não precisa disso).

Cada passo é registrado antes de agir, e a verificação roda de novo no fim: se algo que ele reparou ainda for relatado, ou se surgir dano novo, os passos são revertidos. Um fix interrompido é revertido pelo próximo comando de alteração rodado com `--apply` naquele repositório, que então para; rode-o de novo. Após 30 segundos (`--watch`) o `fix` verifica mais uma vez: dano que voltou foi trazido de volta pelo cliente de sincronização, e a cura duradoura é o `move-out`.

O `undo` desfaz primeiro a alteração mais recente, um reparo ou uma mudança. Um passo cujo resultado mudou desde o reparo é deixado como está e listado, nunca sobrescrito: o git reescreveu o índice reconstruído, o cliente de sincronização trouxe uma cópia de volta, uma ref de resgate se moveu, o backup foi apagado ou o `.git` foi movido. Uma ref de resgate permanece enquanto a cópia que ela mantém viva estiver no backup. Arquivos baixados ficam no disco e objetos buscados de novo ficam no armazenamento de objetos: nenhum deles substitui nada. O resto é desfeito e o undo termina, para que a alteração anterior possa ser desfeita em seguida.

O `history` lista as alterações que este dispositivo fez no repositório (reparos, mudanças, undos), o backup que cada uma mantém com seu tamanho, e qual delas o `undo` desfaz a seguir. O `prune-backups` apaga, só com `--apply`, os backups de alterações desfeitas, revertidas ou que nunca puderam ser desfeitas (um undo, um prepare, um attach) e cujo último passo é mais antigo que `--older-than` (padrão `720h`). Um reparo ou uma mudança que ainda pode ser desfeita mantém seu backup, mesmo com alterações mais novas por cima.

<a name="moving-git-out-of-the-synced-folder"></a>

## Tirar o .git da pasta sincronizada

```sh
gitbrella move-out            # o que aconteceria e o que impede
gitbrella move-out --apply    # fazer
gitbrella undo --apply        # trazer de volta
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Antes: .git dentro da pasta sincronizada. Depois do move-out: um arquivo .git na pasta sincronizada aponta para ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d, fora dela">
</p>

O `move-out` copia o `.git` para `~/.local/share/gitbrella/gitdirs/<name>` (um clone APFS no macOS: não ocupa espaço), prova arquivo por arquivo que a cópia é idêntica, move o original para um backup em `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`) e escreve um arquivo `.git` como `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. O caminho é relativo, então todo dispositivo que mantém a pasta no mesmo lugar dentro da home escreve o mesmo arquivo `.git`. Depois ele verifica se o git encontra o mesmo HEAD, status, configuração e diretório de hooks, e por 30 segundos (`--watch`) se o cliente de sincronização deixa o `.git` em paz e, no iCloud, envia o novo arquivo. O original é posto de lado antes da última comparação, então uma escrita que escapou durante a cópia é detectada e a mudança é desfeita. No iCloud, o `move-out` recusa enquanto o iCloud ainda estiver enviando o repositório; `--wait-upload 10m` espera até esse tempo em vez disso.

Toda alteração é registrada num diário antes de acontecer, com o dispositivo e o inode de tudo o que ela cria (e os bytes do arquivo `.git`); uma reversão não toca em mais nada. Se o processo morrer no meio, o próximo `move-out --apply` (ou `undo --apply`) termina ou reverte e para ali (rode o comando de novo para continuar); ele encontra a alteração pelo inode da pasta mesmo com outra grafia, depois de uma renomeação ou dentro de outro repositório, e o `check` relata a alteração inacabada como dano nesse meio-tempo. Depois que o git usou o diretório movido, nada é revertido por cima dele: se ele contém algo que falta ao original, o comando para e nomeia os dois. Nada é apagado: o original fica no backup até você removê-lo. O `undo` traz de volta o diretório git em uso agora, com os commits feitos desde a mudança; o backup de antes da mudança fica intocado. Se o cliente de sincronização tiver despejado o arquivo `.git` (só o nome dele está neste Mac), o `undo` e a recuperação pedem que você o baixe primeiro: lê-lo faria o download, coisa que a ferramenta nunca faz.

Se uma reversão encontrar tanto o original (no backup) quanto um novo `.git` (criado depois que a mudança parou, por você ou pelo cliente de sincronização), ela não sobrescreve nenhum dos dois e avisa: compare-os, tire da pasta o `.git` que você não quer e rode o comando de novo.

Ele recusa (saída 1, com os motivos) enquanto o git ou algo que detém um lock do git trabalha no repositório, durante um merge ou rebase, enquanto arquivos do `.git` existem só na nuvem, enquanto o iCloud ainda está enviando a pasta, enquanto há um socket (um `git fsmonitor--daemon` rodando) em `.git`, quando existem dois diretórios git depois de uma mudança, e em layouts cujos links quebrariam: worktrees vinculadas, submódulos, objetos emprestados (`alternates`), um `.git` que é um link simbólico ou que não é o próprio diretório git. Outros danos que o `check` encontra são movidos como estão e listados como levados junto: rode o `check` de novo depois.

### Outros dispositivos

O cliente de sincronização apaga o `.git` em todos os outros dispositivos que sincronizam a pasta e leva o arquivo `.git` no lugar. Para manter o histórico deles:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: em cada um dos outros dispositivos, move-out --prepare --apply. 2: em um dispositivo, move-out --apply. 3: em cada um dos outros dispositivos, move-out --attach --apply">
</p>

1. Em cada um dos outros dispositivos, antes da mudança: `gitbrella move-out --prepare --apply` copia o `.git` dele para onde o arquivo `.git` vai apontar.
2. Em um dispositivo: `gitbrella move-out --apply`.
3. Em cada um dos outros dispositivos, depois que o arquivo `.git` chegar: `gitbrella move-out --attach --apply`. Um dispositivo que não foi preparado clona o histórico em vez disso: `--attach --from <remote URL> --apply`. De qualquer forma, a árvore de trabalho não é tocada.

O arquivo `.git` chega pelo cliente de sincronização, então o attach só confia nele até onde vão os registros deste dispositivo: ele precisa apontar para dentro do `~/.local/share/gitbrella/gitdirs` deste dispositivo, e um diretório que já esteja ali só é usado se o `--prepare` o criou para este repositório.

Daí em diante, o cliente de sincronização leva os arquivos e não o histórico. Compartilhe commits por um remoto. Num dispositivo cujos arquivos o cliente de sincronização já atualizou, `git pull` recusa ("would be overwritten"): use `git fetch` e `git reset --mixed @{u}`, que move o branch e o índice e deixa os arquivos sincronizados como estão.

<a name="what-it-reports"></a>

## O que ele relata

Dano (saída 1):

| Código | Significado |
| --- | --- |
| `INDEX_COPY` | cópia de conflito do índice (`index 2`; nomes do Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive e Seafile; o `index (1)` do Google Drive, o `index-COMPUTER` do OneDrive numa pasta do OneDrive, e os nomes do Nextcloud antigo, Synology Drive, pCloud e Resilio, que só são reconhecidos para os nomes de arquivo do próprio git); diz se é um índice válido e se corresponde ao HEAD |
| `REF_COPY` | cópia de conflito de uma ref de branch ou tag; diz se a cópia é mais antiga, mais nova, divergente ou nomeia um commit ausente |
| `COPY_IN_GITDIR` | qualquer outra cópia de conflito dentro de `.git` |
| `INDEX_MISSING` | o índice sumiu; o git mostraria todos os arquivos como apagados (dentro de um hook de commit: o commit apagaria todos os arquivos) |
| `INDEX_EMPTY` | o índice não lista nenhum arquivo enquanto os arquivos do HEAD ainda estão na árvore de trabalho, como quando o git o reconstruiu depois que o cliente de sincronização o apagou; um commit apagaria todos os arquivos |
| `INDEX_SHRUNK` | o índice não lista mais arquivos do HEAD que ainda estão na árvore de trabalho (num sparse checkout: qualquer arquivo do HEAD), como quando o git o reconstruiu a partir dos arquivos preparados depois que o cliente de sincronização o apagou; um commit os apagaria. Renomeações, um diretório que virou link simbólico ou um submódulo declarado em `.gitmodules`, e arquivos apagados também do disco não contam; quando os arquivos do HEAD não podem ser listados, é relatado como não verificado |
| `HEAD_COMMIT_MISSING` | o HEAD nomeia um commit que não está no repositório (a ref chegou antes dos objetos) |
| `ORPHAN_INDEX_LOCK` | o índice sumiu e ficou para trás um `index.lock` antigo (muitas vezes um índice completo) |
| `INDEX_INVALID` | o índice não passa na própria soma de verificação |
| `STALE_LOCK` | um lock do git que nenhum git em execução pode deter: nenhum trabalha no repositório, ou todos começaram depois que o lock foi escrito. Só os processos git desta máquina são vistos: antes de remover o lock, certifique-se de que nenhum git usa o repositório num contêiner, em outra máquina que compartilha a pasta ou dentro de um app que embute o git |
| `DATALESS_IN_GITDIR` | arquivos em `.git` existem só na nuvem; as verificações de objetos foram puladas |
| `FOREIGN_FILE_IN_GITDIR` | um arquivo que o git não criou em `refs/` ou `objects/` (`desktop.ini`, arquivos `.tmp` em `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | um objeto ou ref de zero bytes |
| `GITDIR_COPY` | uma cópia do diretório git na árvore de trabalho (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | o diretório git está dentro da árvore de trabalho com outro nome (`.git.nosync`) e `git add -A` o incluiria num commit |
| `WORKTREE_ADMIN_BROKEN` | uma worktree vinculada perdeu seus arquivos de administração |
| `STALE_OPERATION_STATE` | um diretório de rebase esquecido faz o git achar que há um rebase em andamento |
| `FSCK_ERRORS` | só com `--deep`: o `git fsck` relatou erros (quando ele não termina, o mesmo código é informação) |
| `REFTABLE_TABLE_UNLISTED` | um repositório reftable contém tabelas de refs que sua lista não nomeia: as de outro dispositivo, trazidas pelo cliente de sincronização enquanto a lista dele virava uma cópia que o git nunca lê. O próximo `git pack-refs` ou manutenção as apaga, e com elas os branches daquele dispositivo; copie antes o diretório git para um lugar seguro |
| `CHANGE_UNFINISHED` | um `fix`, `undo` ou `move-out` do gitbrella não terminou (foi interrompido); rode com `--apply` o comando que ele indica para terminar ou reverter antes de usar o git ali |
| `GIT_DIR_RETURNED` | depois do `move-out`, o `.git` voltou a ser um diretório (o cliente de sincronização trouxe o antigo de volta), então existem dois diretórios git; compare-os e fique com um antes de fazer commit |
| `STAGED_CONFLICT_COPY` | o índice adiciona uma cópia de conflito de sincronização de um arquivo que ele também lista (`notes 2.md` ao lado de `notes.md`, um diretório copiado `src 2/`, nomes do Syncthing); um commit poria a cópia no histórico. Só os nomes decidem; uma renomeação para um desses nomes e um arquivo com nome de cópia sem o original não contam; não é verificado com índice dividido |

Informação (o status de saída não muda): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (o diretório git foi tirado da pasta sincronizada: o layout seguro), `WORKTREE_COPIES` (cópias de conflito dos seus próprios arquivos), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (um armazenamento alternativo de objetos contém arquivos só na nuvem, então as verificações de objetos foram puladas), `GC_PID_FOREIGN` (o `gc.pid` de outra máquina: o git pula o gc automático por 12 horas), `GC_LOG_FRESH` (um `gc.log` com menos de um dia impede o gc automático), `HOOKS_IN_SYNCED_FOLDER` (`core.hooksPath` ou um comando `core.fsmonitor` fica numa pasta sincronizada fora do diretório git, então uma alteração em outro dispositivo roda aqui), `WORKTREE_GUESSED` (`GIT_DIR` sem árvore de trabalho, e nenhum dos arquivos do HEAD está no diretório atual: a árvore de trabalho não foi verificada).

Quando o caminho não é um repositório, ele diz por quê, se a causa estiver no disco: um `.git 2` renomeado, arquivos do próprio git existindo só na nuvem, um diretório git sem HEAD, um arquivo `.git` apontando para um diretório git que não existe mais, ou um `core.worktree` que aponta para uma pasta que esta máquina não tem (uma configuração de outra máquina trazida pelo cliente de sincronização). Se o caminho tem um `.git` que o git não consegue usar, ele para ali (saída 2) em vez de responder por um repositório externo, que é o que o próprio git faria.

<a name="install"></a>

## Instalação

Requer git 2.31 ou mais novo. macOS e Linux, amd64 e arm64, sem cgo.

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

ou baixe um arquivo da página de releases.

### Verificando uma versão

Cada versão traz `checksums.txt` (o SHA-256 de cada arquivo e SBOM), sua assinatura cosign sem chave `checksums.txt.sigstore.json` feita pelo workflow de release, um SBOM SPDX para cada arquivo e uma atestação de proveniência de build do GitHub. Verifique primeiro a assinatura, depois o arquivo contra a lista assinada (o exemplo é a v1.2.3 para Apple silicon). Use cosign v3.1.3 ou mais novo (v2.6.5 na linha v2): versões mais antigas podem ser enganadas por um pacote forjado (GHSA-fx35-mq7g-6g98).

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

Uma versão é compilada pelo `.github/workflows/release.yml` a partir de uma tag enviada, com `.goreleaser.yaml`, e fica como rascunho até que uma pessoa a publique. Os binários são compilados com `-trimpath` e o horário do commit, então a mesma tag e a mesma versão do Go (o workflow usa o patch 1.26 mais recente) produzem os mesmos binários; os arquivos e SBOMs em volta deles levam o horário da compilação.

<a name="limits"></a>

## Limites

- O `fix` só repara onde há uma única resposta correta e deixa o resto para você, com o comando a rodar; a solução duradoura é o `move-out`, que mantém o `.git` fora da pasta sincronizada.
- Cópias de conflito criadas pelo próprio iCloud entre dois dispositivos não foram reproduzidas nos testes; a detecção é testada com cópias que seguem o mesmo padrão de nomes.
- O Codex não passa aos hooks o `workdir` da sua ferramenta de shell: um comando que o Codex roda em outra pasta é verificado na pasta da sessão.
- Windows e OneDrive no Windows não foram testados. Repositórios reftable recebem as verificações em nível de arquivo e a de tabelas não listadas; neles, cópias de refs não são comparadas com seus branches.
- Nomes de cópia cuja marca um nome poderia ter de propósito só são reconhecidos onde o próprio git cria o nome, nunca na árvore de trabalho: o `index (1)` do Google Drive para os arquivos do próprio git e em `refs/`; o `index-COMPUTER` do OneDrive para os arquivos do próprio git na raiz de uma pasta do OneDrive, com uma maiúscula no nome do dispositivo e sem uma palavra com que as pessoas terminam o nome de um backup (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` são seus), com no máximo 15 caracteres (um nome de Mac mais longo escapa); o `_conflict-YYYYMMDD-HHMMSS` do Nextcloud antigo, o do Synology Drive, o `.conflicted` do pCloud e o `.Conflict` do Resilio para os arquivos do próprio git na raiz (o do Nextcloud antigo também em `refs/` e `logs/` de uma pasta do Nextcloud, ao lado do original). Um branch que algum HEAD nomeia (como `ref:` com qualquer espaçamento, ou como link simbólico), seu reflog e um diretório que o contém nunca são postos de lado, sejam quais forem seus nomes. Assim, `report (1).pdf`, um branch `fix.Conflict`, uma tag `v1.0.conflicted` ou `config-backup` são nomes comuns.
- Submódulos são verificados de dentro do submódulo. A partir do superprojeto, uma cópia de conflito dentro de `.git/modules/<name>/` só é relatada quando o original está ao lado, e um submódulo com nome de cópia (`lib 2`) é relatado como cópia.
- O índice ausente de uma worktree vinculada é relatado quando essa worktree é verificada, não quando a principal é.
- Com o `.git` ligado por link simbólico a um armazenamento fora da pasta sincronizada, rode o git e a verificação a partir da árvore de trabalho vinculada. A partir do diretório do armazenamento, que não contém nenhum dos arquivos, um índice esvaziado parece um "apagar tudo" intencional e não bloqueia o commit.
- Alguns commits intencionais precisam de `--no-verify`: deixar de rastrear arquivos que continuam no disco, apagar todos os arquivos onde objetos não devem ser lidos, uma renomeação só de maiúsculas/minúsculas ou só de forma Unicode de um arquivo que tem outro hard link, e, num sparse checkout (em modo cone ou não), qualquer commit que remova ou renomeie um arquivo rastreado ou o mova por uma mudança de tipo (um arquivo que vira diretório, um diretório que vira link simbólico ou submódulo). Ali os padrões sparse do git decidem quais arquivos estão no disco, então uma remoção intencional não pode ser distinguida de um índice perdido; os padrões não são lidos.
- Se o cliente de sincronização perder a própria ref do branch (seu arquivo solto apagado antes de chegar um `packed-refs` novo) junto com o índice, o HEAD parece um branch novo e o commit não é bloqueado: ele inicia um histórico novo contendo só os arquivos preparados. Nada no disco é apagado, e o histórico antigo continua no reflog (`git reflog`).
- Com um índice dividido, ou onde objetos não devem ser lidos (clone parcial em git anterior ao 2.45, alternativos só na nuvem), o índice não é comparado com o HEAD arquivo por arquivo: só um índice ausente ou vazio é detectado.
- Com `core.splitIndex`, um índice que não lista nenhum arquivo não é reconhecido (seu cabeçalho conta só a parte dividida), então um commit a partir dele não é bloqueado.
- Hooks: só os hooks de commit são encontrados a partir do processo git que os rodou. Outros hooks (`pre-push`, `pre-applypatch`) precisam rodar a verificação a partir da árvore de trabalho, ou são recusados (saída 2). Onde o processo git não pode ser visto (um cliente que roda os hooks por conta própria, ou listagem de processos bloqueada), a verificação recorre ao diretório atual: então, se o diretório do git não aponta de volta para a árvore de trabalho (`--separate-git-dir`, `.git.nosync`, `--work-tree`), um índice sem arquivos sempre bloqueia o commit (use `--no-verify` para um apagamento intencional), um diretório atual sem nenhum dos arquivos do HEAD não é verificado como árvore de trabalho (`WORKTREE_GUESSED`), e um hook que entrou em outro repositório ou no diretório git é recusado (saída 2). Hooks de recebimento (`pre-receive`) não são uma barreira suportada.
- `move-out`: nem toda ferramenta entende um arquivo `.git`. O git e as ferramentas que rodam o git funcionam; o isomorphic-git (Obsidian Git no celular) e ferramentas que abrem o `.git` como diretório não.
- `move-out`: renomear ou mover a pasta sincronizada quebra o arquivo `.git` relativo. O `check` então nomeia o diretório ausente; coloque a pasta de volta ou aponte o arquivo `.git` para o diretório git à mão.
- O `move-out` precisa da pasta sincronizada no mesmo lugar dentro da home em todos os dispositivos (o iCloud Drive sempre está; o Dropbox pode estar em `~/Dropbox` num dispositivo e em `~/Library/CloudStorage/Dropbox` em outro). Onde não está, o arquivo `.git` aponta para outro lugar ali e o `--attach` recusa (`TARGET_UNEXPECTED`).
- O diretório git movido fica no sistema de arquivos da home; não há opção para colocá-lo em outro lugar.
- `move-out --prepare` copia o histórico do dispositivo naquele momento: faça a mudança logo depois de preparar, sem commits novos no meio.
- `move-out` e `fix` foram testados na própria pasta do iCloud Drive, não dentro da pasta de iCloud de um app (`iCloud~md~obsidian` do Obsidian).

## Contribuindo

Veja [CONTRIBUTING.md](../../CONTRIBUTING.md). Relate problemas de segurança como descrito em [SECURITY.md](../../SECURITY.md), não numa issue pública.

## Licença

MIT
