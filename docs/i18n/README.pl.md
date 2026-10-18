<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="Logo gitbrella: parasol, którego rączką jest gałąź git">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Wykrywa szkody, jakie synchronizacja w chmurze wyrządziła repozytorium git.<br>
  Niczego nie zmienia. Niczego nie pobiera.
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
  <b>Polski</b> ·
  <a href="README.tr.md">Türkçe</a> ·
  <a href="README.ar.md">العربية</a> ·
  <a href="README.hi.md">हिन्दी</a> ·
  <a href="README.id.md">Bahasa Indonesia</a> ·
  <a href="README.vi.md">Tiếng Việt</a>
</p>

<p align="center">
  <a href="#usage">Użycie</a> ·
  <a href="#repairing">Naprawa</a> ·
  <a href="#moving-git-out-of-the-synced-folder">Przenoszenie .git</a> ·
  <a href="#what-it-reports">Co zgłasza</a> ·
  <a href="#install">Instalacja</a> ·
  <a href="#limits">Ograniczenia</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive i Syncthing synchronizują pliki pojedynczo. Git natomiast zakłada, że katalog `.git` zmienia się jako całość, więc w repozytorium w synchronizowanym folderze gromadzą się kopie konfliktowe, takie jak `.git/index 2` i `refs/heads/main 2`, zaginiony index obok pozostawionego `index.lock` oraz obiekty istniejące tylko w chmurze. FAQ samego gita odradza trzymanie repozytoriów w synchronizowanych folderach, ale ustawienia domyślne wciąż je tam umieszczają: Biurko i Dokumenty w iCloud na macOS, kopia zapasowa folderów w OneDrive na Windows, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Klient synchronizacji kopiuje .git między urządzeniami plik po pliku i zostawia kopie konfliktowe, takie jak index 2 i refs/heads/main 2, index.lock obok brakującego indexu oraz obiekty istniejące tylko w chmurze">
</p>

`gitbrella check` mówi, zwykle w mniej niż sekundę, czy repozytorium ma takie uszkodzenia. Zaprojektowano go do uruchamiania, zanim agent AI lub hook gita dotknie repozytorium. `gitbrella fix` naprawia to, co ma tylko jeden poprawny sposób naprawy, a `gitbrella move-out` kończy problem u źródła: przenosi `.git` poza synchronizowany folder i zostawia plik `.git`, który tam wskazuje, dzięki czemu klient synchronizacji przenosi twoje pliki, a git trzyma historię u siebie.

| Polecenie | Co robi |
| --- | --- |
| `gitbrella check` | zgłasza uszkodzenia (tylko odczyt) |
| `gitbrella fix` | naprawia to, co ma jedną poprawną odpowiedź |
| `gitbrella move-out` | przenosi `.git` poza synchronizowany folder na stałe |
| `gitbrella undo` | cofa ostatnią naprawę lub przeniesienie |
| `gitbrella history` | wyświetla zmiany wprowadzone na tym urządzeniu |
| `gitbrella prune-backups` | usuwa stare kopie zapasowe zmian, których nie można już cofnąć |
| `gitbrella hook --for claude\|codex` | chroni wywołania narzędzi agenta AI |

## Szybki start

**1. Sprawdź.** Tylko odczyt, bez pobierania, bez sieci.

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # wyjście 0: zdrowe, 1: uszkodzenia, 2: nie dało się uruchomić
```

**2. Napraw.** Tylko to, co ma jedną poprawną odpowiedź. Bez `--apply` pokazuje jedynie plan.

```sh
gitbrella fix                 # co naprawi, a co zostawi tobie
gitbrella fix --apply         # napraw; `gitbrella undo --apply` przywraca stan sprzed
```

**3. Rozwiąż u źródła.** Przenieś `.git` poza synchronizowany folder.

```sh
gitbrella move-out            # co się stanie i co by to zatrzymało
gitbrella move-out --apply
```

**Pozostań chroniony.** Blokuj commity w uszkodzonym repozytorium:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Agenci AI: zobacz [Dla agentów](#for-agents).

## Co obiecuje

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` działa tylko w trybie odczytu.** Nigdy nie zapisuje w repozytorium. Git uruchamia wyłącznie polecenia odczytu, z `GIT_OPTIONAL_LOCKS=0` oraz wyłączonym automatycznym gc i konserwacją. Testy end-to-end dla ryzykownych przypadków (kopia indexu, brakujący index, częściowy klon, dzielony index, przeniesiony katalog git) zapisują rozmiar, mtime i inode każdego pliku przed sprawdzeniem i po nim, a następnie je porównują.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Niczego nie pobiera z chmury.** Na macOS wyłącza pobieranie na żądanie dla własnego procesu (i uruchamianego gita), zanim dotknie dysku; jeśli macOS odmówi, sprawdzenie się nie uruchamia (wyjście 2). Plik dostępny tylko w iCloud jest zgłaszany, a nie pobierany.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Nie korzysta z sieci.** Częściowy klon nigdy nie dociąga brakujących obiektów (`GIT_NO_LAZY_FETCH`); z gitem starszym niż 2.45 sprawdzanie obiektów jest zamiast tego pomijane.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Milczy przy zdrowych repozytoriach.** Blokada liczy się jako uszkodzenie tylko wtedy, gdy jest stara i żaden git działający na tym repozytorium (na którymkolwiek z jego drzew roboczych) nie mógł jej zapisać: git tworzy blokadę w chwili jej przejęcia, więc git uruchomiony później nie może jej trzymać. Pliki tymczasowe gita i pliki innych narzędzi w `.git` mają charakter informacyjny. Uruchomienie na 89 zdrowych, używanych repozytoriach nie zgłosiło żadnych uszkodzeń.

<a name="usage"></a>

## Użycie

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Kody wyjścia: **0** brak uszkodzeń, **1** znaleziono uszkodzenia, **2** nie udało się wykonać sprawdzenia. Dla poleceń, które coś zmieniają (`move-out`, `fix`, `undo`): **0** zaplanowano, wykonano albo nie ma nic do zrobienia (`already`), **1** odmowa (z powodami), wykonano z ostrzeżeniami lub zostały uszkodzenia pozostawione tobie (lista MANUAL w `fix`; `check` nadal je zgłasza), **2** niepowodzenie (wycofano lub zatrzymano; JSON zachowuje to, co odzyskano, oraz wpisy dziennika) albo nie dało się uruchomić.

`--deep` uruchamia też pełny `git fsck` (czyta każdy obiekt, więc jest pomijany, jeśli obiekty są tylko w chmurze). `--provider` wymusza klienta synchronizacji zamiast wykrywać go na podstawie ścieżki: `auto` (domyślnie), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage`, `none`. Foldery Syncthing i Nextcloud (lub ownCloud) są rozpoznawane, gdziekolwiek się znajdują, po znaczniku, który klient w nich umieszcza (`.stfolder`, `.sync_*.db`). `fix` i `move-out` przyjmują ten sam `--provider`; podaj tę samą wartość co dla `check`, bo niektóre nazwy kopii liczą się tylko dla konkretnego klienta. JSON pokazuje wtedy `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Wynik gitbrella check: DAMAGE INDEX_COPY index 2, który naprawia gitbrella fix, oraz INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>Ten sam wynik jako tekst</summary>

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

### Jako hook gita

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit uruchamia hook pre-commit, który uruchamia gitbrella check: przy wyjściu 0 commit przechodzi, przy 1 lub 2 jest blokowany">
</p>

Commit w uszkodzonym repozytorium jest blokowany; w zdrowym przechodzi normalnie. Narzędzie wie, że hook uruchamia git, więc nie myli własnej blokady commita ze starą.

**Które repozytorium jest sprawdzane.** W hookach commita (git przekazuje im `GIT_INDEX_FILE` lub bezwzględny `GIT_DIR`) repozytorium jest znajdowane na podstawie procesu gita, który uruchomił hook, więc hook może najpierw zmienić katalog: na katalog git, jego katalog hooks albo katalog poza jakimkolwiek repozytorium. Gdy git wskazuje repozytorium bezwzględnym `GIT_DIR` (powiązane drzewo robocze, submoduł, oddzielony katalog git), każde polecenie gita w hooku działa na nim, dokądkolwiek hook przejdzie, i tak samo sprawdzenie. W przeciwnym razie sprawdza, dokąd bieżący katalog prowadzi dla samego gita (katalog git, na który wskazuje tam względny `GIT_DIR`, lub repozytorium znalezione stamtąd); jeśli to inne repozytorium lub inne drzewo robocze tego samego repozytorium (hook, który tam przechodzi, albo edytor komunikatu commita), odmawia (wyjście 2) zamiast odpowiadać za niewłaściwe repozytorium.

Bez ścieżki sprawdza repozytorium, które git wyeksportował do hooka (`GIT_DIR`), więc działają konfiguracje dotfiles z `--git-dir` i katalog git wewnątrz własnego drzewa roboczego. Bez ścieżki respektuje też `GIT_DIR` ustawiony w powłoce; w odróżnieniu od gita ignoruje nieistniejący `GIT_DIR` i odrzuca powiązane drzewo robocze, za którym nie da się podążyć, zamiast traktować bieżący katalog jako drzewo robocze.

**Index, który zapisuje commit.** Gdy `GIT_INDEX_FILE` wskazuje plik wewnątrz katalogu git (tak git ustawia go dla hooków commita), czytany jest ten plik. Jeśli index, który zapisze commit, nie istnieje lub jest pusty albo nie wymienia już plików z HEAD, które wciąż są w drzewie roboczym, commit jest blokowany, zanim zapisze te pliki jako usunięte. Plik indexu w innym miejscu, wskazany przez ciebie za pomocą `GIT_INDEX_FILE`, nie jest czytany: w hooku sprawdzany jest zamiast niego własny index repozytorium, a ręczne sprawdzenie z ustawioną tą zmienną jest odrzucane (wyjście 2).

**Celowe usunięcia.** Aby przestać śledzić pliki, które zostaną na dysku (`git rm --cached`, przestanie śledzenia katalogu, który jest teraz ignorowany), albo celowo zacommitować „usuń wszystko”, gdy pliki wciąż są na dysku (`git rm -r --cached .`), pomiń hook za pomocą `git commit --no-verify`.

### Z menedżerem hooków

- **pre-commit** (pre-commit.com): dodaj to repozytorium z identyfikatorem hooka `gitbrella` (pre-commit zbuduje go w Go) lub `gitbrella-system` (plik binarny z PATH); zobacz `.pre-commit-hooks.yaml`.
- **lefthook**: zaimportuj `hooks/lefthook.yml` przez `remotes`.
- **husky**: plik `.husky/pre-commit` zawierający `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, a następnie `overcommit --sign`.

We wszystkich każdy niezerowy kod wyjścia zatrzymuje commit, więc sprawdzenie, którego nie da się uruchomić (wyjście 2), również go zatrzymuje. Aider commituje z `--no-verify`, chyba że ustawiono `git-commit-verify: true`, a wtedy nie uruchamia się żaden hook commita. Obsidian Git (desktop) commituje systemowym gitem i uruchamia hook; dodaj folder z `gitbrella` do jego ustawienia „Additional PATH”, ponieważ aplikacje uruchamiane z Docka nie dostają PATH powłoki. Obsidian Git na urządzeniach mobilnych (isomorphic-git) nie uruchamia hooków.

<a name="for-agents"></a>

### Dla agentów

`--json` wypisuje ten sam raport jako stabilny, wersjonowany dokument (`schemaVersion: 1`, `findings` zawsze jest listą). Decyzję podejmuj na podstawie kodu wyjścia, a wyniki pokaż użytkownikowi.

Własne hooki agentów zatrzymują wywołanie narzędzia tylko przy wyjściu 2: wyjście 1 („znaleziono uszkodzenia”) przepuszcza wywołanie. `gitbrella hook --for claude|codex` mówi językiem ich hooków. Czyta zdarzenie, które agent przekazuje na stdin:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="Wywołanie narzędzia przez agenta przechodzi przez gitbrella hook: odczyty i edycje przechodzą; w uszkodzonym repozytorium zapisy gita i zapisy w .git są zatrzymywane wyjściem 2">
</p>

- **Przed wywołaniem narzędzia** (`PreToolUse`): w uszkodzonym repozytorium zapisujące polecenie gita (commit, add, reset, rebase, push i wszystko, o czym nie wiadomo, że tylko czyta) oraz zapis w `.git` zatrzymują wywołanie (wyjście 2, powód na stderr dla agenta). Odczyty, edycja twoich plików, `git restore` i sam `gitbrella` przechodzą, aby uszkodzenie można było obejrzeć i naprawić.
  - *Nieużywalne repozytoria.* Tam, gdzie pliki gita są tylko w chmurze lub git w ogóle nie może użyć repozytorium (zmieniona nazwa `HEAD`, plik `.git` wskazujący donikąd), zatrzymywane jest każde polecenie gita: zawiesiłoby się albo zadziałało na zewnętrznym repozytorium. Zapisy w `.git` przechodzą tam: taką naprawę wykonuje się ręcznie w `.git`, jak mówi powód (`mv '.git/HEAD 2' .git/HEAD`). Przechodzi też środek wskazany w podpowiedzi `core.worktree` (`git config --file … --unset core.worktree`).
  - *Które repozytorium.* To, które zawiera pliki, na których działa wywołanie (łatka Codex wymienia swoje pliki; ścieżka w `.git` liczy się dla drzewa roboczego nad nią). Dla polecenia powłoki: folder sesji i każdy folder, w którym działają jego polecenia gita (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), najpierw zapisujące, najwyżej pięć; gdy zapisujący się nie mieści, wywołanie przechodzi z ostrzeżeniem. Każdy folder jest oceniany po tym, co wywołanie w nim robi: `git -C ../sound commit` z uszkodzonego folderu sesji przechodzi. Folder, który nie jest repozytorium, i repozytorium bare przechodzą po cichu.
  - *Gdy sprawdzenia nie da się uruchomić.* Wszystko, co uniemożliwia sprawdzenie (w tym 20-sekundowy `--timeout`, jeden termin na całe wywołanie), przepuszcza wywołanie z ostrzeżeniem; `--fail-open` robi to samo tam, gdzie git nie może użyć repozytorium.
  - *Jak czytane jest polecenie.* Jest czytane, nigdy wykonywane: cudzysłowy, komentarze, podpowłoki, podstawienia (także wewnątrz heredoc bez cudzysłowów), heredoc, ustawiane zmienne, `&&`, `||`, `if`, `case` i pętle (po `cd a || cd b` albo `cd` do nieistniejącego folderu liczy się każdy folder, w którym może być powłoka; pętla `for` jest czytana raz dla każdej wartości), wrappery (`timeout`, `sudo`, `nice`, `xargs`, `env`) oraz skrypty przekazane powłoce (`-c`, `eval`, `watch`, `source`, plik, heredoc, `bash < f`, potok do `sh`). Funkcje i aliasy zdefiniowane przez polecenie są czytane tam, gdzie są wywoływane. Skrypt powłoki uruchamiany po ścieżce lub przekazany powłoce (`./deploy.sh`, `bash deploy.sh`) jest czytany, jeśli to mały plik na tej maszynie (także przez dowiązanie symboliczne; raz, nawet jeśli uruchamia sam siebie); `make`, `npm run` i podobne nie są czytane.
  - *Co liczy się jako zapis.* Zapis innego programu w `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` na `.git/config`, `rm -rf` lub `find -delete` na folderze zawierającym `.git`, łatka Codex przez powłokę) liczy się jako zapis w `.git`; skopiowanie `.git` gdzie indziej, spakowanie `tar` lub odczyt — nie. `gh pr checkout` i `git-lfs` liczą się jako zapisujące polecenia gita.
  - *W razie wątpliwości.* Każde z poniższych liczy się jako zapisujące polecenie gita: odczyt dłuższy niż 5000 kroków lub połowa `--timeout` (w folderze, którego nie udało się ustalić); słowo `git` w miejscu, za którym odczyt nie podąża; skrypt przekazany potokiem do powłoki z programu, za którym odczyt nie podąża (`curl … | sh`), tam, gdzie działa; istniejący, ale nieczytelny skrypt przekazany powłoce (za duży, tylko w chmurze), tam, gdzie działa. Gdy nie da się ustalić, w którym folderze zapisuje polecenie gita (`cd "$(…)"`, `for d in */`), polecenie liczy się w każdym sprawdzanym folderze, a wywołanie dostaje ostrzeżenie.
- **Na początku sesji** (`SessionStart`): jedna linia na stdout, którą agent czyta jako kontekst, mówiąca, że repozytorium jest w synchronizowanym folderze (chyba że jego katalog git jest już na zewnątrz) i co jest uszkodzone. Nigdy nie blokuje.

Hook niczego nie zmienia. Claude Code, w `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex, w `.codex/hooks.json` (Codex uruchamia hook dopiero po tym, jak mu zaufasz w `/hooks`):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

Sprawdzenie uruchamia się raz dla każdego z tych folderów, zwykle po 0,1–0,4 sekundy, więc łatka obejmująca kilka folderów jednego repozytorium sprawdza to repozytorium kilka razy; `--timeout` ogranicza całe wywołanie.

<a name="repairing"></a>

## Naprawa

```sh
gitbrella fix                 # co naprawi, a co zostawi tobie
gitbrella fix --apply         # napraw
gitbrella undo --apply        # przywróć wszystko
gitbrella history             # zmiany wprowadzone tutaj, ich kopie zapasowe i rozmiary
gitbrella prune-backups       # kopie zapasowe, których żadne undo już nie potrzebuje
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix pokazuje plan; fix --apply zapisuje każdy krok, potem sprawdzenie uruchamia się ponownie: gotowe lub wycofane; undo --apply cofa ostatnią zmianę">
</p>

`check` podaje pod każdym uszkodzeniem, czy `fix` je naprawia (`repair:`, a w JSON `"repair": "fix"` lub `"manual"`), i wymienia `fix --provider NAME`, gdy sprawdzeniu podano dostawcę. W hookach commita o nic nie pyta.

`fix` naprawia tylko tam, gdzie jest jedna poprawna odpowiedź i nic, co mogłeś mieć na myśli, nie zostanie nadpisane:

| Uszkodzenie | Naprawa |
| --- | --- |
| kopie konfliktowe w `.git` obok oryginału (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, kopie Dropbox i Syncthing oraz kopia blokady refa, np. `refs/heads/main 2.lock`); kopia indexu tylko wtedy, gdy jest identyczna z indexem lub jest poprawnym indexem równym HEAD | przenoszone do kopii zapasowej; obiekt (commit, tag, drzewo lub plik), na który wskazuje tylko kopia, jest najpierw zachowywany jako `refs/gcd-rescue/<name>` |
| pozostawiony katalog rebase, `desktop.ini`, `Icon` i podobne pliki niestworzone przez gita (chyba że git czyta któryś jako gałąź lub jego nazwa jest nazwą obiektu), kopia `.git` w drzewie roboczym, której commity repozytorium już ma | przenoszone do kopii zapasowej |
| brakujący index bez kopii ani blokady gdziekolwiek | tworzony z HEAD (`git read-tree HEAD`: pliki nie są ruszane), tylko jeśli istnieje commit, z którego można go zbudować (jeśli brakuje też commita HEAD, najpierw jest pobierany, jak opisano niżej), i poza sparse checkout |
| index, który nie wymienia żadnego pliku (poprawny, niedzielony index poza sparse checkout) | przenoszony do kopii zapasowej i tworzony z HEAD: nie było w nim nic przygotowanego do commita |
| osierocony `index.lock`, który jest poprawnym indexem równym HEAD | staje się indexem |
| pusty plik gałęzi | commit, który ostatnio zapisał jej reflog |
| `.git.nosync`, który git dodałby do commita | linia `/.git.nosync/` w `.git/info/exclude` |
| brakujący commit HEAD lub pusty plik obiektu loose (bez kopii obok) | pusty plik trafia do kopii zapasowej, a obiekty są pobierane ponownie (`git fetch --refetch`, git 2.36 lub nowszy) z `--remote`, w przeciwnym razie z remote gałęzi, w przeciwnym razie z `origin`, w przeciwnym razie z jedynego remote; gałęzie i tagi remote są pobierane do tymczasowej przestrzeni nazw `refs/gcd-fetch/`, która jest potem usuwana, więc żaden ref się nie zmienia. Jeśli remote nie odda obiektu, naprawa jest wycofywana |
| z `fix --deep`: obiekty, które `git fsck` zgłosił jako uszkodzone | każdy obiekt gałęzi i tagów remote jest pobierany ponownie w ten sam sposób; potem `git fsck` uruchamia się ponownie i jeśli nadal zgłasza błędy (obiekt, którego nie ma żaden remote, pozostały uszkodzony plik), naprawa jest wycofywana |
| kopie konfliktowe twoich własnych plików (`notes 2.md`), których git nie śledzi i które mają te same bajty co plik, z którego je skopiowano | przenoszone do kopii zapasowej (`git add -A` by je zacommitował); kopia, która się różni, jest śledzona, jest dowiązaniem, jest tylko w chmurze lub w submodule, zostaje |
| `gc.pid` innej maszyny starszy niż godzina; `gc.log` | przenoszone do kopii zapasowej, aby automatyczne gc gita znów tu działało |
| pliki `.git` lub alternatywnego magazynu obiektów dostępne tylko w chmurze | pobierane, i nic więcej: do tego czasu sprawdzenie nie może czytać obiektów, więc pozostałe uszkodzenia są wymieniane jako MANUAL do następnego uruchomienia `fix` |

Wszystko inne jest wymieniane jako MANUAL, z poleceniem do uruchomienia. Obejmuje to wszystko, co może zawierać pracę, której nie ma nigdzie indziej:
- uszkodzony index (git może nadal potrafić go odczytać razem z przygotowaną pracą);
- kopię indexu lub `index.lock` różną od HEAD oraz kopię `index.lock`;
- kopię, której oryginału brakuje (może być jedyna: odłóż ją na miejsce; jeśli to kopia blokady, jest zachowywana i odkładana na bok);
- blokadę, której nie trzyma żaden git na tej maszynie (może ją trzymać git na innym urządzeniu lub w kontenerze; porada wymienia commit w jej wnętrzu);
- kopię wskazującą ponad 100 obiektów (długi reflog);
- kopię gałęzi wskazanej przez dowolny HEAD, jej reflogu lub katalogu, który ją zawiera (git ją czyta, jakkolwiek wygląda nazwa).

Oraz wszystko, gdzie fix musiałby wybierać między dwiema wersjami albo nie może utrzymać przy życiu commitów kopii:
- wszystko w magazynie obiektów: kopie, pliki o nazwie obiektu (sprawdź, który pasuje do swojego hasha), puste pliki obiektów bez remote, z którego można je pobrać;
- wszystko w katalogu git submodułu (uruchom `fix` w submodule);
- kopię gałęzi wskazującą commit, który jeszcze tu nie dotarł (może jeszcze dotrzeć);
- kopię `.git` w drzewie roboczym z commitami, których brakuje repozytorium (ty decydujesz, która jest aktualna);
- pusty plik, obok którego leży kopia jego samego lub jego katalogu;
- uszkodzony katalog administracyjny powiązanego drzewa roboczego (`git worktree repair`);
- index, który stracił pliki wciąż obecne na dysku (celowe przestanie śledzenia wygląda tak samo; gdy nic innego nie jest przygotowane, porada mówi, że `git read-tree HEAD` przywraca je bez żadnych strat);
- bez `--deep` lub bez remote: obiekty, które `git fsck` zgłosił jako uszkodzone.

`fix` w ogóle nie dotyka repozytoriów reftable; tam wynik `REFTABLE_TABLE_UNLISTED` z `check` mówi, jak zachować refy innego urządzenia. Kopia zapasowa musi być na dysku repozytorium, aby to, co fix odkłada na bok, przenosiło się w jednym kroku; w przeciwnym razie `fix` odmawia (naprawa polegająca tylko na pobraniu niczego nie przenosi i tego nie potrzebuje).

Każdy krok jest zapisywany przed wykonaniem, a na końcu sprawdzenie uruchamia się ponownie: jeśli coś, co naprawił, jest nadal zgłaszane lub pojawia się nowe uszkodzenie, kroki są cofane. Przerwany fix jest wycofywany przez następne polecenie zmieniające uruchomione z `--apply` w tym repozytorium, które następnie się zatrzymuje; uruchom je ponownie. Po 30 sekundach (`--watch`) `fix` sprawdza jeszcze raz: uszkodzenie, które wraca, przyniósł klient synchronizacji, a trwałym rozwiązaniem jest `move-out`.

`undo` cofa najpierw najnowszą zmianę, naprawę lub przeniesienie. Krok, którego wynik zmienił się od naprawy, zostaje bez zmian i jest wymieniony, nigdy nadpisany: git przepisał odtworzony index, klient synchronizacji przywrócił kopię, ref ratunkowy się przesunął, kopię zapasową usunięto albo przeniesiono `.git`. Ref ratunkowy pozostaje, dopóki kopia, którą utrzymuje przy życiu, jest w kopii zapasowej. Pobrane pliki zostają na dysku, a ponownie pobrane obiekty w magazynie obiektów: żadne z nich niczego nie zastępuje. Reszta jest cofana i undo się kończy, więc zmianę sprzed niej można cofnąć w następnej kolejności.

`history` wymienia zmiany, które to urządzenie wprowadziło w repozytorium (naprawy, przeniesienia, cofnięcia), kopię zapasową każdej z rozmiarem oraz to, którą `undo` cofnie jako następną. `prune-backups`, tylko z `--apply`, usuwa kopie zapasowe zmian cofniętych, wycofanych lub nigdy niecofalnych (undo, prepare, attach), których ostatni krok jest starszy niż `--older-than` (domyślnie `720h`). Naprawa lub przeniesienie, które wciąż można cofnąć, zachowuje swoją kopię zapasową, nawet jeśli nad nią są nowsze zmiany.

<a name="moving-git-out-of-the-synced-folder"></a>

## Przenoszenie .git poza synchronizowany folder

```sh
gitbrella move-out            # co się stanie i co by to zatrzymało
gitbrella move-out --apply    # wykonaj
gitbrella undo --apply        # przenieś z powrotem
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Przed: .git w synchronizowanym folderze. Po move-out: plik .git w synchronizowanym folderze wskazuje na ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d na zewnątrz">
</p>

`move-out` kopiuje `.git` do `~/.local/share/gitbrella/gitdirs/<name>` (na macOS klon APFS: nie zajmuje miejsca), dowodzi plik po pliku, że kopia jest identyczna, przenosi oryginał do kopii zapasowej w `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`) i zapisuje plik `.git` w rodzaju `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. Ścieżka jest względna, więc każde urządzenie, które trzyma folder w tym samym miejscu katalogu domowego, zapisuje ten sam plik `.git`. Następnie sprawdza, czy git znajduje ten sam HEAD, status, konfigurację i katalog hooks, a przez 30 sekund (`--watch`), czy klient synchronizacji nie rusza `.git` i — w iCloud — wysyła nowy plik. Oryginał jest odkładany na bok przed końcowym porównaniem, więc zapis, który wślizgnie się podczas kopiowania, zostanie wykryty, a przeniesienie cofnięte. W iCloud `move-out` odmawia, dopóki iCloud nadal wysyła repozytorium; `--wait-upload 10m` zamiast tego czeka maksymalnie tyle.

Każda zmiana jest zapisywana w dzienniku, zanim nastąpi, wraz z urządzeniem i inode wszystkiego, co tworzy (oraz bajtami pliku `.git`); wycofanie nie dotyka niczego innego. Jeśli proces zginie w połowie, następne `move-out --apply` (lub `undo --apply`) kończy go lub wycofuje i tam się zatrzymuje (uruchom polecenie ponownie, aby kontynuować); niedokończona zmiana jest znajdowana po inode folderu, nawet gdy ścieżka jest zapisana inaczej, po zmianie nazwy lub wewnątrz innego repozytorium. W międzyczasie `check` zgłasza niedokończoną zmianę jako uszkodzenie. Nic nie jest wycofywane na przeniesiony katalog, gdy git zaczął go używać: jeśli zawiera coś, czego nie ma oryginał, polecenie zatrzymuje się i wymienia oba. Nic nie jest usuwane: oryginał pozostaje w kopii zapasowej, dopóki go nie usuniesz. `undo` przywraca używany obecnie katalog git z commitami zrobionymi po przeniesieniu; kopia zapasowa sprzed przeniesienia pozostaje nietknięta. Jeśli klient synchronizacji usunął lokalną zawartość pliku `.git` (na tym Macu została tylko jego nazwa), `undo` i odzyskiwanie poproszą cię najpierw o jego pobranie: odczytanie spowodowałoby pobranie, a to narzędzie nigdy tego nie robi.

Jeśli wycofanie znajdzie zarówno oryginał (w kopii zapasowej), jak i nowy `.git` (utworzony przez ciebie lub klienta synchronizacji po zatrzymaniu przeniesienia), nie nadpisuje żadnego z nich i o tym informuje: porównaj je, przenieś niechciany `.git` poza folder i uruchom polecenie ponownie.

Odmawia (wyjście 1, z powodami), gdy git lub coś trzymającego blokadę gita działa na repozytorium, podczas merge lub rebase, gdy pliki `.git` są tylko w chmurze, gdy iCloud nadal wysyła folder, gdy w `.git` jest gniazdo (działający `git fsmonitor--daemon`), gdy po przeniesieniu istnieją dwa katalogi git, oraz dla układów, w których zerwałyby się powiązania: powiązane drzewa robocze, submoduły, pożyczone obiekty (`alternates`), `.git` będący dowiązaniem symbolicznym lub niebędący samym katalogiem git. Inne uszkodzenia znalezione przez `check` są przenoszone bez zmian i wymieniane jako przeniesione: uruchom potem `check` ponownie.

### Inne urządzenia

Klient synchronizacji usuwa `.git` na każdym innym urządzeniu, które synchronizuje folder, i dostarcza tam plik `.git`. Aby zachować ich historię:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: move-out --prepare --apply na każdym innym urządzeniu. 2: move-out --apply na jednym urządzeniu. 3: move-out --attach --apply na każdym innym urządzeniu">
</p>

1. Na każdym innym urządzeniu, przed przeniesieniem: `gitbrella move-out --prepare --apply` kopiuje `.git` tego urządzenia tam, gdzie będzie wskazywał plik `.git`.
2. Na jednym urządzeniu: `gitbrella move-out --apply`.
3. Na każdym innym urządzeniu, po dotarciu pliku `.git`: `gitbrella move-out --attach --apply`. Nieprzygotowane urządzenie zamiast tego klonuje historię: `--attach --from <remote URL> --apply`. W obu przypadkach drzewo robocze nie jest ruszane.

Plik `.git` dociera przez klienta synchronizacji, więc attach ufa mu tylko w takim stopniu, na jaki pozwalają zapisy tego urządzenia: musi wskazywać wnętrze `~/.local/share/gitbrella/gitdirs` tego urządzenia, a katalog, który już tam istnieje, jest używany tylko wtedy, gdy utworzył go `--prepare` dla tego repozytorium.

Od tej chwili klient synchronizacji przenosi pliki, a nie historię. Udostępniaj commity przez remote. Na urządzeniu, którego pliki klient synchronizacji już zaktualizował, `git pull` odmawia („would be overwritten”): użyj `git fetch` i `git reset --mixed @{u}`, co przesuwa gałąź i index, a synchronizowane pliki zostawia bez zmian.

<a name="what-it-reports"></a>

## Co zgłasza

Uszkodzenia (wyjście 1):

| Kod | Znaczenie |
| --- | --- |
| `INDEX_COPY` | kopia konfliktowa indexu (`index 2`; nazwy Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive i Seafile; `index (1)` z Google Drive, `index-COMPUTER` z OneDrive w folderze OneDrive oraz nazwy starego Nextcloud, Synology Drive, pCloud i Resilio są dopasowywane tylko do nazw plików samego gita); mówi, czy to poprawny index i czy zgadza się z HEAD |
| `REF_COPY` | kopia konfliktowa refa gałęzi lub tagu; mówi, czy kopia jest starsza, nowsza, rozbieżna, czy wskazuje brakujący commit |
| `COPY_IN_GITDIR` | każda inna kopia konfliktowa w `.git` |
| `INDEX_MISSING` | brak indexu; git pokazuje każdy plik jako usunięty (w hooku commita: commit usunąłby każdy plik) |
| `INDEX_EMPTY` | index nie wymienia żadnego pliku, choć pliki HEAD są nadal w drzewie roboczym — tak jak wtedy, gdy git odtworzył go po usunięciu indexu przez klienta synchronizacji; commit usunąłby każdy plik |
| `INDEX_SHRUNK` | index nie wymienia już plików HEAD, które wciąż są w drzewie roboczym (w sparse checkout: dowolnego pliku HEAD) — tak jak wtedy, gdy git odtworzył go z przygotowanych plików po usunięciu indexu przez klienta synchronizacji; commit by je usunął. Zmiany nazw, katalog, który stał się dowiązaniem symbolicznym lub submodułem zadeklarowanym w `.gitmodules`, oraz pliki usunięte także z dysku się nie liczą; gdy plików HEAD nie da się wymienić, zgłaszane jako niezweryfikowane |
| `HEAD_COMMIT_MISSING` | HEAD wskazuje commit, którego nie ma w repozytorium (ref dotarł przed swoimi obiektami) |
| `ORPHAN_INDEX_LOCK` | brak indexu, a stary `index.lock` pozostał (często pełny index) |
| `INDEX_INVALID` | index nie przechodzi własnej sumy kontrolnej |
| `STALE_LOCK` | blokada gita, której nie może trzymać żaden działający git: żaden nie działa na repozytorium albo wszystkie uruchomiono po zapisaniu blokady. Widoczne są tylko procesy gita na tej maszynie: przed usunięciem blokady upewnij się, że żaden git nie używa repozytorium w kontenerze, na innej maszynie współdzielącej folder ani w aplikacji z wbudowanym gitem |
| `DATALESS_IN_GITDIR` | pliki w `.git` są tylko w chmurze; sprawdzanie obiektów pominięto |
| `FOREIGN_FILE_IN_GITDIR` | plik w `refs/` lub `objects/` niestworzony przez gita (`desktop.ini`, pliki `.tmp` w `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | obiekt lub ref o zerowym rozmiarze |
| `GITDIR_COPY` | kopia katalogu git w drzewie roboczym (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | katalog git znajduje się w drzewie roboczym pod inną nazwą (`.git.nosync`) i `git add -A` by go zacommitował |
| `WORKTREE_ADMIN_BROKEN` | powiązane drzewo robocze straciło swoje pliki administracyjne |
| `STALE_OPERATION_STATE` | pozostawiony katalog rebase sprawia, że git sądzi, iż trwa rebase |
| `FSCK_ERRORS` | tylko `--deep`: `git fsck` zgłosił błędy (gdy się nie kończy, ten sam kod jest informacyjny) |
| `REFTABLE_TABLE_UNLISTED` | repozytorium reftable zawiera tabele refów, których jego lista nie wymienia: tabele innego urządzenia przyniesione przez klienta synchronizacji, którego lista stała się kopią nigdy nieczytaną przez gita. Następne `git pack-refs` lub konserwacja usuną je, a wraz z nimi gałęzie tamtego urządzenia; najpierw skopiuj katalog git w bezpieczne miejsce |
| `CHANGE_UNFINISHED` | `fix`, `undo` lub `move-out` z gitbrella się nie zakończył (został przerwany); zanim użyjesz tam gita, uruchom wskazane polecenie z `--apply`, aby go dokończyć lub wycofać |
| `GIT_DIR_RETURNED` | po `move-out` `.git` znów jest katalogiem (klient synchronizacji przywrócił stary), więc istnieją dwa katalogi git; porównaj i zostaw jeden przed commitem |
| `STAGED_CONFLICT_COPY` | index dodaje synchronizacyjną kopię konfliktową pliku, który także wymienia (`notes 2.md` obok `notes.md`, skopiowany katalog `src 2/`, nazwy Syncthing); commit wprowadziłby kopię do historii. Decydują tylko nazwy; zmiana nazwy na taką nazwę i plik o nazwie kopii bez oryginału się nie liczą; nie sprawdzane przy dzielonym indexie |

Informacje (kod wyjścia się nie zmienia): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (katalog git przeniesiono poza synchronizowany folder: bezpieczny układ), `WORKTREE_COPIES` (kopie konfliktowe twoich własnych plików), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (alternatywny magazyn obiektów zawiera pliki tylko w chmurze, więc sprawdzanie obiektów pominięto), `GC_PID_FOREIGN` (`gc.pid` innej maszyny: git pomija automatyczne gc przez 12 godzin), `GC_LOG_FRESH` (`gc.log` młodszy niż dzień zatrzymuje automatyczne gc), `HOOKS_IN_SYNCED_FOLDER` (polecenie `core.hooksPath` lub `core.fsmonitor` leży w synchronizowanym folderze poza katalogiem git, więc zmiana wprowadzona na innym urządzeniu jest wykonywana tutaj), `WORKTREE_GUESSED` (`GIT_DIR` bez drzewa roboczego, a w bieżącym katalogu nie ma żadnego pliku HEAD: drzewo robocze nie zostało sprawdzone).

Gdy ścieżka w ogóle nie jest repozytorium, mówi dlaczego, jeśli przyczyna jest na dysku: `.git 2` ze zmienioną nazwą, pliki samego gita tylko w chmurze, katalog git bez HEAD, plik `.git` wskazujący katalog git, który już nie istnieje, lub `core.worktree` wskazujący folder, którego nie ma na tej maszynie (ustawienie innej maszyny przyniesione przez klienta synchronizacji). Jeśli na ścieżce jest `.git`, którego git nie może użyć, zatrzymuje się tam (wyjście 2) zamiast odpowiadać za zewnętrzne repozytorium, jak zrobiłby sam git.

<a name="install"></a>

## Instalacja

Wymaga gita 2.31 lub nowszego. macOS i Linux, amd64 i arm64, bez cgo.

```
brew install aliyilmazco/tap/gitbrella
```

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

albo pobierz archiwum ze strony wydań (releases).

### Weryfikacja wydania

Każde wydanie zawiera `checksums.txt` (SHA-256 każdego archiwum i SBOM), bezkluczowy podpis cosign `checksums.txt.sigstore.json` utworzony przez workflow wydania, SBOM SPDX dla każdego archiwum oraz atestację pochodzenia kompilacji GitHub (build-provenance attestation). Najpierw sprawdź podpis, potem archiwum względem podpisanej listy (przykład to v1.2.3 dla Apple silicon). Używaj cosign v3.1.3 lub nowszego (v2.6.5 w linii v2): starsze wersje można oszukać podrobionym pakietem (GHSA-fx35-mq7g-6g98).

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

Wydanie jest budowane z wypchniętego tagu przez `.github/workflows/release.yml` przy użyciu `.goreleaser.yaml` i pozostaje szkicem, dopóki ktoś go nie opublikuje. Pliki binarne są budowane z `-trimpath` i czasem commita, więc ten sam tag i to samo wydanie Go (workflow bierze najnowszą poprawkę 1.26) dają te same pliki binarne; archiwa i SBOM wokół nich zawierają czas swojej kompilacji.

<a name="limits"></a>

## Ograniczenia

- `fix` naprawia tylko tam, gdzie jest jedna poprawna odpowiedź, a resztę zostawia tobie z poleceniem do uruchomienia; trwałym rozwiązaniem jest `move-out`, który trzyma `.git` poza synchronizowanym folderem.
- Kopii konfliktowych, które sam iCloud tworzy między dwoma urządzeniami, nie odtworzono w testach; wykrywanie przetestowano na kopiach stosujących to samo nazewnictwo.
- Codex nie przekazuje hookom `workdir` swojego narzędzia powłoki: polecenie, które Codex uruchamia w innym folderze, jest sprawdzane w folderze sesji.
- Windows i OneDrive na Windows nie są przetestowane. Repozytoria reftable dostają sprawdzenia na poziomie plików i sprawdzenie niewymienionych tabel; kopie refów nie są tam porównywane z ich gałęziami.
- Nazwy kopii ze znacznikami, które nazwa może zawierać celowo, są dopasowywane tylko tam, gdzie nazwę tworzy sam git, nigdy w drzewie roboczym: `index (1)` z Google Drive dla plików samego gita i w `refs/`; `index-COMPUTER` z OneDrive dla plików samego gita na najwyższym poziomie folderu OneDrive, z nazwą urządzenia zawierającą wielką literę, niebędącą słowem, którym ludzie kończą nazwę kopii zapasowej (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` są twoje), i mającą najwyżej 15 znaków (dłuższa nazwa Maca zostanie pominięta); `_conflict-YYYYMMDD-HHMMSS` starego Nextcloud, nazwy Synology Drive, `.conflicted` z pCloud i `.Conflict` z Resilio dla plików samego gita na najwyższym poziomie (te ze starego Nextcloud także w `refs/` i `logs/` obok oryginału w folderze Nextcloud). Gałąź wskazana przez dowolny HEAD (jako `ref:` z dowolnymi odstępami lub jako dowiązanie symboliczne), jej reflog i katalogi, które ją zawierają, nigdy nie są odkładane na bok, niezależnie od nazwy. Dlatego `report (1).pdf`, gałąź o nazwie `fix.Conflict`, tag o nazwie `v1.0.conflicted` czy `config-backup` to zwykłe nazwy.
- Submoduły są sprawdzane z wnętrza submodułu. Z projektu nadrzędnego kopia konfliktowa w `.git/modules/<name>/` jest zgłaszana tylko wtedy, gdy obok jest jej oryginał, a submoduł o nazwie kopii (`lib 2`) jest zgłaszany jako kopia.
- Brakujący index powiązanego drzewa roboczego jest zgłaszany przy sprawdzaniu tego drzewa roboczego, a nie głównego.
- Gdy `.git` jest dowiązaniem symbolicznym do magazynu poza synchronizowanym folderem, uruchamiaj gita i sprawdzenie z powiązanego drzewa roboczego. Z katalogu magazynu, który nie zawiera żadnych plików, opróżniony index wygląda jak celowe „usuń wszystko” i nie blokuje commita.
- Niektóre celowe commity wymagają `--no-verify`: przestanie śledzenia plików pozostających na dysku, usunięcie każdego pliku tam, gdzie nie wolno czytać obiektów, zmiana nazwy zmieniająca tylko wielkość liter lub postać Unicode pliku z innym dowiązaniem twardym oraz każdy commit w sparse checkout (w trybie cone lub nie), który usuwa lub zmienia nazwę śledzonego pliku albo przenosi go przez zmianę typu (plik, który stał się katalogiem, katalog, który stał się dowiązaniem symbolicznym lub submodułem). Tam wzorce sparse gita decydują, które pliki są na dysku, więc celowego usunięcia nie da się odróżnić od utraconego indexu; wzorce nie są czytane.
- Jeśli klient synchronizacji wraz z indexem stracił też sam ref gałęzi (jego plik loose usunięto, zanim dotarł nowy `packed-refs`), HEAD wygląda jak nowa gałąź i commit nie jest blokowany: rozpoczyna nową historię zawierającą tylko przygotowane pliki. Nic na dysku nie jest usuwane, a stara historia pozostaje w reflogu (`git reflog`).
- Przy dzielonym indexie lub tam, gdzie nie wolno czytać obiektów (częściowy klon z gitem starszym niż 2.45, alternatywy tylko w chmurze), index nie jest porównywany z HEAD plik po pliku: wykrywany jest tylko brakujący lub pusty index.
- Z `core.splitIndex` index niewymieniający żadnego pliku nie jest rozpoznawany (jego nagłówek liczy tylko część dzieloną), więc commit z niego nie jest blokowany.
- Hooki: tylko hooki commita są znajdowane na podstawie procesu gita, który je uruchamia. Inne hooki (`pre-push`, `pre-applypatch`) muszą uruchamiać sprawdzenie z drzewa roboczego, inaczej zostają odrzucone (wyjście 2). Tam, gdzie proces gita nie jest widoczny (klient, który sam uruchamia hooki, lub zablokowane listowanie procesów), sprawdzenie wraca do bieżącego katalogu: wtedy, jeśli katalog gita nie wskazuje z powrotem na drzewo robocze (`--separate-git-dir`, `.git.nosync`, `--work-tree`), index bez plików zawsze blokuje commit (dla celowego opróżnienia użyj `--no-verify`), bieżący katalog niezawierający żadnego pliku HEAD nie jest sprawdzany jako drzewo robocze (`WORKTREE_GUESSED`), a hook przechodzący do innego repozytorium lub katalogu git zostaje odrzucony (wyjście 2). Hooki odbioru (`pre-receive`) nie są wspieraną bramką.
- `move-out`: nie każde narzędzie rozumie plik `.git`. Git i narzędzia uruchamiające gita działają; isomorphic-git (Obsidian Git na urządzeniach mobilnych) i narzędzia otwierające `.git` jako katalog — nie.
- `move-out`: zmiana nazwy lub przeniesienie synchronizowanego folderu psuje względny plik `.git`. `check` wymieni wtedy brakujący katalog; przywróć folder na miejsce lub ręcznie skieruj plik `.git` na katalog git.
- `move-out` wymaga, aby synchronizowany folder był w tym samym miejscu katalogu domowego na każdym urządzeniu (iCloud Drive zawsze jest; Dropbox może być `~/Dropbox` na jednym urządzeniu i `~/Library/CloudStorage/Dropbox` na innym). Gdy tak nie jest, plik `.git` wskazuje tam gdzie indziej, a `--attach` odmawia (`TARGET_UNEXPECTED`).
- Przeniesiony katalog git znajduje się w systemie plików katalogu domowego; nie ma opcji, by umieścić go gdzie indziej.
- `move-out --prepare` kopiuje historię urządzenia w danym momencie: przenieś wkrótce po przygotowaniu, bez nowych commitów w międzyczasie.
- `move-out` i `fix` wypróbowano w folderze samego iCloud Drive, a nie w folderze iCloud aplikacji (`iCloud~md~obsidian` w Obsidianie).

## Współtworzenie

Zobacz [CONTRIBUTING.md](../../CONTRIBUTING.md). Problemy z bezpieczeństwem zgłaszaj zgodnie z opisem w [SECURITY.md](../../SECURITY.md), a nie w publicznym issue.

## Licencja

MIT
