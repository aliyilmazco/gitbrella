<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="gitbrella logosu: sapı bir git dalı olan şemsiye">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Bulut senkronizasyonunun git depolarına verdiği hasarı bulur;<br>
  hiçbir şeyi değiştirmeden ve hiçbir şey indirmeden.
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
  <b>Türkçe</b> ·
  <a href="README.ar.md">العربية</a> ·
  <a href="README.hi.md">हिन्दी</a> ·
  <a href="README.id.md">Bahasa Indonesia</a> ·
  <a href="README.vi.md">Tiếng Việt</a>
</p>

<p align="center">
  <a href="#usage">Kullanım</a> ·
  <a href="#repairing">Onarım</a> ·
  <a href="#moving-git-out-of-the-synced-folder">.git'i dışarı taşıma</a> ·
  <a href="#what-it-reports">Raporladıkları</a> ·
  <a href="#install">Kurulum</a> ·
  <a href="#limits">Sınırlar</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive ve Syncthing dosyaları tek tek senkronize eder. Git ise `.git` dizininin bir bütün olarak değişmesini bekler; bu yüzden senkronize klasörlerdeki depolarda `.git/index 2` veya `refs/heads/main 2` gibi çakışma kopyaları, geride kalmış `index.lock` dosyalarının yanında kaybolmuş index'ler ve yalnızca bulutta bulunan nesneler oluşur. Git'in kendi SSS'si depoyu senkronize bir klasörde tutmamayı öğütler, ama varsayılan ayarlar depoları oraya koymaya devam eder: macOS'ta iCloud'daki Masaüstü ve Belgeler, Windows'ta OneDrive'a klasör yedekleme, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Senkronizasyon istemcileri .git'i cihazlar arasında dosya dosya kopyalar ve index 2, refs/heads/main 2 gibi çakışma kopyaları, kaybolmuş bir index'in yanında index.lock ve yalnızca bulutta duran nesneler bırakır">
</p>

`gitbrella check`, genellikle bir saniyeden kısa sürede, bir depoda bu hasarın olup olmadığını söyler. Bir yapay zekâ ajanı veya git hook'u depoya dokunmadan önce çalışması için tasarlanmıştır. `gitbrella fix` tek bir doğru onarımı olan hasarları onarır, `gitbrella move-out` ise hasarı kalıcı olarak bitirir: `.git`'i senkronize klasörün dışına taşır ve yerine ona işaret eden bir `.git` dosyası bırakır; böylece senkronizasyon istemcisi dosyalarınızı taşır, git de geçmişini kendine saklar.

| Komut | Ne yapar |
| --- | --- |
| `gitbrella check` | hasarı raporlar; salt okunur |
| `gitbrella fix` | tek bir doğru cevabı olanı onarır |
| `gitbrella move-out` | `.git`'i senkronize klasörün dışına kalıcı olarak taşır |
| `gitbrella undo` | en son onarımı veya taşımayı geri alır |
| `gitbrella history` | bu cihazda yapılan değişiklikleri listeler |
| `gitbrella prune-backups` | artık geri alınamayacak değişikliklerin eski yedeklerini siler |
| `gitbrella hook --for claude\|codex` | yapay zekâ ajanlarının araç çağrılarını korur |

## Hızlı başlangıç

**1. Bakın.** Salt okunur; indirme yok, ağ yok:

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # çıkış 0: sağlam, 1: hasar, 2: çalışamadı
```

**2. Onarın:** tek bir doğru cevabı olanı. `--apply` olmadan yalnızca planı gösterir:

```sh
gitbrella fix                 # neyi onaracağı ve neyi size bırakacağı
gitbrella fix --apply         # onar; `gitbrella undo --apply` geri alır
```

**3. Kalıcı olarak bitirin.** `.git`'i senkronize klasörün dışına taşıyın:

```sh
gitbrella move-out            # ne olacağı ve onu neyin durduracağı
gitbrella move-out --apply
```

**Koruma altında tutun.** Hasarlı bir depoda commit'leri engelleyin:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Yapay zekâ ajanları için [Ajanlar için](#for-agents) bölümüne bakın.

## Verdiği sözler

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` salt okunurdur.** Depoya asla yazmaz. Git; `GIT_OPTIONAL_LOCKS=0` ile, otomatik gc ve bakım kapalı olarak ve yalnızca okuyan komutlarla çalışır. Riskli durumlar (index kopyaları, eksik index, kısmi klonlar, bölünmüş index'ler, taşınmış git dizinleri) için uçtan uca testler, kontrolden önce ve sonra her dosyanın boyutunu, mtime'ını ve inode'unu kaydeder ve bunların aynı olmasını şart koşar.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Buluttan indirme yok.** macOS'ta diske dokunmadan önce kendi süreci (ve başlattığı git) için isteğe bağlı indirmeyi kapatır; macOS reddederse kontrol çalışmaz (çıkış 2). Yalnızca iCloud'da bulunan dosyalar raporlanır, indirilmez.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Ağ yok.** Kısmi klonlar eksik nesneleri asla getirmez (`GIT_NO_LAZY_FETCH`). 2.45'ten eski git'te bunun yerine nesne kontrolleri atlanır.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Sağlam depolarda sessiz.** Bir kilit ancak eskiyse ve depo üzerinde çalışan (herhangi bir worktree'sinde) hiçbir git onu yazmış olamıyorsa hasar sayılır: git kilidi alırken oluşturur, dolayısıyla sonradan başlamış bir git onu tutuyor olamaz. Git'in geçici dosyaları ve `.git` içindeki diğer araçların dosyaları bilgi olarak gösterilir. Çalışan 89 sağlam depoda denendiğinde hiç hasar raporlamadı.

<a name="usage"></a>

## Kullanım

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Çıkış durumu: **0** hasar yok, **1** hasar bulundu, **2** kontrol çalışamadı. Bir şeyi değiştiren komutlar (`move-out`, `fix`, `undo`) şu kodlarla çıkar: **0** planlandı, yapıldı veya yapılacak bir şey yok (`already`); **1** reddedildi (gerekçeleriyle), bir uyarıyla yapıldı ya da size bırakılan hasarla yapıldı (`fix`'in MANUAL listesi; `check` bunları hâlâ raporlar); **2** başarısız oldu (geri alındı veya durdu; JSON kurtarılanları ve günlük kaydını tutar) ya da çalışamadı.

`--deep` ayrıca tam bir `git fsck` çalıştırır (her nesneyi okur, bu yüzden nesneler yalnızca buluttaysa atlanır). `--provider`, senkronizasyon istemcisini yoldan algılamak yerine zorlar: `auto` (varsayılan), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage` veya `none`. Syncthing ve Nextcloud (veya ownCloud) klasörleri, nerede olurlarsa olsunlar, istemcilerinin içlerinde tuttuğu işaret dosyasıyla (`.stfolder`, `.sync_*.db`) tanınır. `fix` ve `move-out` da aynı `--provider`'ı alır: `check`'e verdiğinizi onlara da verin, çünkü bazı kopya adları yalnızca belirli bir istemci için sayılır. JSON bu durumda `"providerForced": true` der.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="gitbrella check çıktısı: gitbrella fix ile onarılabilen DAMAGE INDEX_COPY index 2 ve INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>Aynı çıktı metin olarak</summary>

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

### Git hook'u olarak

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit pre-commit hook'unu, o da gitbrella check'i çalıştırır: çıkış 0 commit'e izin verir, 1 veya 2 engeller">
</p>

Hasarlı bir depo commit'i engeller; sağlam bir depo her zamanki gibi commit eder. Araç, hook'u git'in çalıştırdığını bilir ve commit'in kendi kilidini eskimiş bir kilitle karıştırmaz.

**Hangi depo kontrol edilir.** Commit hook'larında (git onlara `GIT_INDEX_FILE` veya mutlak bir `GIT_DIR` geçirir) depoyu hook'u çalıştıran git sürecinden bulur; bu yüzden hook önce dizin değiştirebilir: git dizinine, onun hooks dizinine veya hiçbir deponun içinde olmayan bir dizine. Git depoyu mutlak bir `GIT_DIR` ile adlandırdığında (bağlı worktree'ler, alt modüller, ayrı git dizinleri), hook'taki her git komutu, hook nereye giderse gitsin o depoya etki eder; kontrol de öyle. Aksi halde git'in kendisinin geçerli dizinden nereye varacağına bakar (orada göreli bir `GIT_DIR`'in adlandırdığı git dizini ya da oradan bulunan depo): bu başka bir depo veya aynı deponun başka bir worktree'siyse (oraya geçen bir hook ya da commit mesajı düzenleyicisi), yanlış depo için cevap vermek yerine reddeder (çıkış 2).

Yol verilmeden çalıştırıldığında git'in hook'a aktardığı depoyu (`GIT_DIR`) kontrol eder; böylece `--git-dir` ile dotfiles kurulumları ve kendi worktree'si içindeki git dizinleri de çalışır. Yol verilmediğinde kabuğunuzda ayarlanmış bir `GIT_DIR`'e de uyar; git'ten farklı olarak, var olmayan bir `GIT_DIR`'i yok sayar ve geriye izleyemediği bağlı bir worktree'yi, geçerli dizini çalışma ağacı kabul etmek yerine reddeder.

**Commit'in kaydettiği index.** `GIT_INDEX_FILE` git dizini içindeki bir dosyayı gösterdiğinde (git bunu commit hook'ları için böyle ayarlar) onu okur: commit'in kaydettiği index eksik veya boşsa ya da hâlâ çalışma ağacında olan bir HEAD dosyasını artık listelemiyorsa, commit bu dosyaları silinmiş olarak kaydedemeden engellenir. `GIT_INDEX_FILE`'ı kendiniz başka bir yerdeki index dosyasına yönlendirirseniz o okunmaz: hook içinde bunun yerine deponun kendi index'i kontrol edilir; bu değişken ayarlıyken elle çalıştırılan bir kontrol ise reddeder (çıkış 2).

**Bilinçli kaldırmalar.** Diskte tuttuğunuz bir dosyanın takibini bırakmak (`git rm --cached`, artık yok saydığınız bir dizinin takibini bırakmak) veya dosyalarınız hâlâ diskteyken bilinçli bir "her şeyi sil" commit'i yapmak (`git rm -r --cached .`) için hook'u `git commit --no-verify` ile atlayın.

### Bir hook yöneticisiyle

- **pre-commit** (pre-commit.com): depoyu `gitbrella` (pre-commit onu Go ile derler) veya `gitbrella-system` (PATH'teki ikili) hook kimliğiyle ekleyin; bkz. `.pre-commit-hooks.yaml`.
- **lefthook**: `hooks/lefthook.yml`'yi `remotes` ile çekin.
- **husky**: `gitbrella check` içeren bir `.husky/pre-commit`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, ardından `overcommit --sign`.

Sıfırdan farklı her çıkış hepsinde commit'i durdurur; dolayısıyla çalışamayan bir kontrol (çıkış 2) da durdurur. Aider, `git-commit-verify: true` ayarlanmadıkça `--no-verify` ile commit eder; bu durumda hiçbir commit hook'u çalışmaz. Obsidian Git (masaüstü) sisteminizdeki git ile commit eder ve hook'u çalıştırır; `gitbrella`'nın bulunduğu klasörü onun "Additional PATH" ayarına ekleyin, çünkü Dock'tan başlatılan uygulamalar kabuğun PATH'ini almaz. Mobildeki Obsidian Git (isomorphic-git) hiçbir hook çalıştırmaz.

<a name="for-agents"></a>

### Ajanlar için

`--json`, aynı raporu kararlı ve sürümlü bir belge olarak yazdırır (`schemaVersion: 1`, `findings` her zaman bir listedir). Kararı çıkış durumuna göre verin, bulguları kullanıcıya gösterin.

Ajanların kendi hook'ları bir araç çağrısını yalnızca çıkış 2'de durdurur: çıkış 1 ("hasar bulundu") çağrının devam etmesine izin verir. `gitbrella hook --for claude|codex` onların hook dilini konuşur. Ajanın stdin üzerinden geçirdiği olayı okur:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="Bir ajanın araç çağrısı gitbrella hook'tan geçer: okuma ve düzenleme devam eder; hasarlı bir depoda git yazmaları ve .git içine yazma çıkış 2 ile durur">
</p>

- **Bir araç çağrısından önce** (`PreToolUse`): hasarlı bir depoda yazan bir git komutu (commit, add, reset, rebase, push ve yalnızca okuduğu bilinmeyen her şey) ve `.git` içine yazma çağrıyı durdurur (çıkış 2, gerekçe ajan için stderr'de). Okuma, dosyalarınızı düzenleme, `git restore` ve `gitbrella`'nın kendisi devam eder; böylece hasara bakılabilir ve onarılabilir.
  - *Kullanılamayan depolar.* Git dosyalarının yalnızca bulutta olduğu ya da git'in depoyu hiç kullanamadığı yerlerde (yeniden adlandırılmış bir `HEAD`, hiçbir yeri göstermeyen bir `.git` dosyası) her git komutu durur: ya takılır ya da dıştaki bir depoya etki eder. Orada `.git` içine yazmalar devam eder: o onarım, gerekçede söylendiği gibi `.git` içinde elle yapılır (`mv '.git/HEAD 2' .git/HEAD`). `core.worktree` ipucunun adlandırdığı çare (`git config --file … --unset core.worktree`) de devam eder.
  - *Hangi depo.* Çağrının üzerinde çalıştığı dosyaları tutan depo (bir Codex yaması kendi dosyalarını adlandırır; `.git` içindeki bir yol, üstündeki çalışma ağacı için sayılır). Bir kabuk komutu için oturumun klasörü ve git komutlarının çalıştığı her klasördür (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`); önce yazanlar, en fazla beş tane. Yazan biri sığmadığında çağrı bir uyarıyla geçer. Her klasör, çağrının orada ne yaptığına göre değerlendirilir: hasarlı bir oturum klasöründen `git -C ../sound commit` devam eder. Depo olmayan bir klasör ve bare bir depo sessizce geçer.
  - *Kontrol çalışamadığında.* Kontrolün çalışmasını engelleyen her şey (20 saniyelik `--timeout` dahil; tüm çağrı için tek bir süre sınırı) çağrıyı bir uyarıyla geçirir; `--fail-open`, git'in depoyu kullanamadığı yerlerde de bunu yapar.
  - *Komut nasıl okunur.* Okunur, asla çalıştırılmaz: tırnaklar, yorumlar, alt kabuklar, ikameler (tırnaksız bir heredoc içinde de), heredoc'lar, ayarladığı değişkenler, `&&`, `||`, `if`, `case` ve döngüler (`cd a || cd b`'den sonra veya var olmayan bir klasöre `cd`'den sonra kabuğun bulunabileceği her klasör sayılır; bir `for` döngüsü her değer için bir kez okunur), sarmalayıcılar (`timeout`, `sudo`, `nice`, `xargs`, `env`) ve bir kabuğa verilen betikler (`-c`, `eval`, `watch`, `source`, bir dosya, bir heredoc, `bash < f`, `sh`'ye bir pipe). Komutun tanımladığı fonksiyonlar ve alias'lar çağrıldıkları yerde okunur. Yoluyla çalıştırılan veya bir kabuğa verilen bir kabuk betiği (`./deploy.sh`, `bash deploy.sh`), bu makinedeki küçük bir dosyaysa okunur (bir symlink üzerinden de; kendini çalıştırsa bile bir kez). `make`, `npm run` ve benzerleri okunmaz.
  - *Yazma sayılanlar.* Başka bir programın `.git` içine yazması (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `.git/config` üzerinde `sed -i`, `.git` içeren bir klasörde `rm -rf` veya `find -delete`, kabuk üzerinden bir Codex yaması) `.git` içine yazma sayılır; `.git`'i başka yere kopyalamak, `tar`lamak veya okumak sayılmaz. `gh pr checkout` ve `git-lfs`, yazan git komutları sayılır.
  - *Şüphe durumunda.* Bunların her biri yazan bir git komutu sayılır: 5000 adımdan veya `--timeout`'un yarısından uzun süren bir okuma (hangi klasör olduğunu ayırt edemediği yerde); okumanın takip etmediği bir yerdeki `git` kelimesi; okumanın takip etmediği bir programdan bir kabuğa pipe edilen betik (`curl … | sh`), çalıştığı yerde; bir kabuğa verilen, mevcut ama okunamayan betik (çok büyük, yalnızca bulutta), çalıştığı yerde. Bir git komutunun hangi klasöre yazdığını ayırt edemediğinde (`cd "$(…)"`, `for d in */`), komut kontrol edilen her klasörde sayılır ve çağrı bir uyarı alır.
- **Oturum başında** (`SessionStart`): stdout'a bir satır yazar; ajan bunu bağlam olarak okur. Satır, deponun senkronize bir klasörde olduğunu (git dizini zaten dışarıda değilse) ve neyin hasarlı olduğunu söyler. Asla engellemez.

Hook hiçbir şeyi değiştirmez. Claude Code için `.claude/settings.json` içinde:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Codex için `.codex/hooks.json` içinde (Codex bir hook'u ancak siz `/hooks` içinde ona güvendikten sonra çalıştırır):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

Kontrol bu klasörlerin her biri için bir kez çalışır, genellikle her biri 0,1 ile 0,4 saniye sürer; bu yüzden bir deponun birkaç klasörüne yayılan bir yama depoyu birkaç kez kontrol eder. `--timeout` tüm çağrıyı sınırlar.

<a name="repairing"></a>

## Onarım

```sh
gitbrella fix                 # neyi onaracağı ve neyi size bırakacağı
gitbrella fix --apply         # onar
gitbrella undo --apply        # her şeyi geri koy
gitbrella history             # burada yapılan değişiklikler, yedekleri ve boyutları
gitbrella prune-backups       # artık hiçbir undo'nun ihtiyaç duymayacağı yedekler
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix planı gösterir; fix --apply her adımı kaydeder, sonra kontrol yeniden çalışır: tamamlandı ya da geri alındı; undo --apply en son değişikliği geri alır">
</p>

`check`, her hasarın altında `fix`'in onu onarıp onarmadığını söyler (`repair:` ve JSON'da `"repair": "fix"` veya `"manual"`); kontrole bir `--provider` verildiyse `fix --provider NAME`'i adlandırır. Commit hook'larının içinde sormaz.

`fix` yalnızca tek bir doğru cevabın olduğu ve kastetmiş olabileceğiniz hiçbir şeyin üzerine yazılmadığı yerlerde onarır:

| Hasar | Onarım |
| --- | --- |
| `.git` içinde, orijinalinin yanında duran çakışma kopyaları (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, Dropbox ve Syncthing kopyaları ve `refs/heads/main 2.lock` gibi bir ref kilidinin kopyası); bir index kopyası yalnızca index ile birebir aynıysa veya HEAD'e eşit geçerli bir index ise | yedeğe taşınır; yalnızca kopyanın gösterdiği bir nesne (commit, tag, tree veya dosya) önce `refs/gcd-rescue/<name>` olarak korunur |
| geride kalmış bir rebase dizini, `desktop.ini`, `Icon` ve git'in oluşturmadığı benzer dosyalar (git birini dal olarak okumuyorsa veya adı bir nesnenin adı değilse), çalışma ağacında commit'leri depoda bulunan bir `.git` kopyası | yedeğe taşınır |
| hiçbir yerde kopyası veya kilidi olmayan eksik index | HEAD'den oluşturulur (`git read-tree HEAD`: dosyalara dokunulmaz); yalnızca üzerine kurulacak bir commit varsa (HEAD'in commit'i de eksikse önce aşağıdaki gibi getirilir) ve sparse checkout dışında |
| hiç dosya listelemeyen bir index (sparse checkout dışında geçerli, bölünmemiş bir index) | yedeğe taşınır ve HEAD'den oluşturulur: içinde stage edilmiş bir şey yoktu |
| HEAD'e eşit geçerli bir index olan sahipsiz bir `index.lock` | index olur |
| boş bir dal dosyası | reflog'unun en son kaydettiği commit |
| git'in bir commit'e ekleyeceği `.git.nosync` | `.git/info/exclude` içinde `/.git.nosync/` satırı |
| HEAD'in commit'i eksik veya boş bir loose nesne dosyası (yanında kopyası olmadan) | boş dosya yedeğe taşınır ve nesneler yeniden getirilir (`git fetch --refetch`, git 2.36 veya daha yeni): `--remote`'tan, yoksa dalın remote'undan, yoksa `origin`'den, yoksa tek remote'tan. Remote'un dalları ve tag'leri sonradan silinen geçici bir `refs/gcd-fetch/` ad alanına getirilir, böylece hiçbir ref değişmez. Remote nesneyi geri getirmezse onarım geri alınır |
| `fix --deep` ile: `git fsck`'in hasarlı bildirdiği nesneler | remote'un dallarının ve tag'lerinin her nesnesi aynı şekilde yeniden getirilir; ardından `git fsck` yeniden çalışır ve hâlâ hata bildirirse (hiçbir remote'ta olmayan bir nesne, kalan bozuk bir dosya) onarım geri alınır |
| kendi dosyalarınızın, git'in takip etmediği ve kopyaladığı dosyayla aynı baytları taşıyan çakışma kopyaları (`notes 2.md`) | yedeğe taşınır (`git add -A` onları commit ederdi); farklı olan, takip edilen, link olan, yalnızca bulutta duran veya bir alt modülde olan kopya kalır |
| başka bir makinenin bir saatten eski `gc.pid`'i; bir `gc.log` | yedeğe taşınır, böylece git'in otomatik gc'si burada yeniden çalışır |
| `.git`'in veya alternatif bir nesne deposunun yalnızca bulutta bulunan dosyaları | indirilir, başka bir şey yapılmaz: o zamana kadar kontrol nesneleri okuyamaz, bu yüzden kalan hasar bir sonraki `fix` çalıştırması için MANUAL olarak listelenir |

Geri kalan her şey, ne çalıştırılacağıyla birlikte MANUAL olarak listelenir. Buna başka hiçbir yerde bulunmayan işi tutabilecek her şey dahildir:
- hasarlı bir index (git onu stage edilmiş işle birlikte hâlâ okuyabilir);
- HEAD'den farklı bir index kopyası veya `index.lock` ve bir `index.lock` kopyası;
- orijinali eksik olan bir kopya (tek kopya o olabilir: geri koyun; bir kilit kopyasıysa kurtarılıp kenara taşınır);
- bu makinedeki hiçbir git'in tutmadığı bir kilit (başka bir cihazdaki veya bir container'daki git tutuyor olabilir; önerisi içindeki commit'i adlandırır);
- 100'den fazla nesne adlandıran bir kopya (uzun bir reflog);
- bir HEAD'in adlandırdığı dalın, reflog'unun veya o dalı içeren bir dizinin kopyası (git, adı neye benzerse benzesin onu okur).

Ayrıca `fix`'in iki sürüm arasında seçim yapmak zorunda kalacağı ya da bir kopyanın commit'lerini canlı tutamayacağı her şey de dahildir:
- nesne deposundaki her şey: kopyalar, bir nesnenin adını taşıyan dosyalar (hangisinin hash'iyle eşleştiğini kontrol edin), getirilecek remote'u olmayan boş nesne dosyaları;
- bir alt modülün git dizinindeki her şey (`fix`'i alt modülde çalıştırın);
- henüz burada olmayan bir commit'i adlandıran bir dal kopyası (hâlâ gelebilir);
- çalışma ağacında, deponun sahip olmadığı commit'leri içeren bir `.git` kopyası (hangisinin güncel olduğuna siz karar verirsiniz);
- yanında kendisinin veya dizininin kopyası duran boş bir dosya;
- bozuk bir bağlı worktree yönetim dizini (`git worktree repair`);
- hâlâ diskte olan dosyaları kaybetmiş bir index (bilerek takipten çıkarmak da aynı görünür; stage edilmiş başka bir şey yoksa öneri, `git read-tree HEAD`'in onları hiçbir şey kaybetmeden geri getirdiğini söyler);
- `--deep` olmadan veya remote olmadan, `git fsck`'in hasarlı bildirdiği nesneler.

`fix` reftable depolarına hiç dokunmaz; orada `check`'in `REFTABLE_TABLE_UNLISTED` bulgusu başka bir cihazın ref'lerinin nasıl korunacağını söyler. Yedek, deponun diskinde olmalıdır ki `fix`'in kenara taşıdıkları tek adımda taşınsın; değilse `fix` reddeder (yalnızca indirme yapan bir onarım hiçbir şey taşımaz ve buna ihtiyaç duymaz).

Her adım uygulanmadan önce kaydedilir ve sonunda kontrol yeniden çalışır: onardığı bir şey hâlâ raporlanıyorsa veya yeni bir hasar ortaya çıktıysa adımlar geri alınır. Öldürülmüş bir onarım, o depoda `--apply` ile çalıştırılan bir sonraki değişiklik komutu tarafından geri alınır; komut sonra durur, onu yeniden çalıştırın. 30 saniye sonra (`--watch`) `fix` bir kez daha kontrol eder: geri gelen hasarı senkronizasyon istemcisi geri getirmiştir ve kalıcı çare `move-out`'tur.

`undo` önce en yeni değişikliği, bir onarımı veya bir taşımayı geri alır. Sonucu onarımdan bu yana değişmiş bir adım olduğu gibi bırakılır ve listelenir, asla üzerine yazılmaz: git yeniden oluşturulan index'i yeniden yazdıysa, senkronizasyon istemcisi bir kopyayı geri getirdiyse, bir kurtarma ref'i yer değiştirdiyse, yedek silindiyse veya `.git` taşındıysa. Bir kurtarma ref'i, canlı tuttuğu kopya yedekte durdukça kalır. İndirilen dosyalar diskte kalır ve yeniden getirilen nesneler nesne deposunda kalır: hiçbiri bir şeyin yerine geçmez. Geri kalanı geri alınır ve undo tamamlanır; böylece ondan önceki değişiklik sonra geri alınabilir.

`history`, bu cihazın depoda yaptığı değişiklikleri (onarımlar, taşımalar, geri almalar), her birinin tuttuğu yedeği boyutuyla ve `undo`'nun sırada hangisini geri alacağını listeler. `prune-backups` yalnızca `--apply` ile; geri alınmış, geri döndürülmüş veya hiç geri alınamayan (bir undo, bir prepare, bir attach) ve son adımı `--older-than`'dan (varsayılan `720h`) eski olan değişikliklerin yedeklerini siler. Hâlâ geri alınabilen bir onarım veya taşıma, üzerinde daha yeni değişiklikler olsa bile yedeğini korur.

<a name="moving-git-out-of-the-synced-folder"></a>

## .git'i senkronize klasörün dışına taşıma

```sh
gitbrella move-out            # ne olacağı ve onu neyin durduracağı
gitbrella move-out --apply    # yap
gitbrella undo --apply        # geri taşı
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Önce: .git senkronize klasörün içinde. move-out'tan sonra: senkronize klasördeki bir .git dosyası dışarıdaki ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d'yi gösterir">
</p>

`move-out`, `.git`'i `~/.local/share/gitbrella/gitdirs/<name>` konumuna kopyalar (macOS'ta bir APFS klonu: yer kaplamaz), kopyanın dosya dosya aynı olduğunu kanıtlar, orijinali `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`) altındaki bir yedeğe taşır ve `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d` gibi bir `.git` dosyası yazar. Yol görelidir; bu yüzden klasörü ev dizini altında aynı yerde tutan her cihaz aynı `.git` dosyasını yazar. Ardından git'in aynı HEAD'i, durumu, yapılandırmayı ve hooks dizinini bulduğunu kontrol eder ve 30 saniye boyunca (`--watch`) senkronizasyon istemcisinin `.git`'e dokunmadığını ve iCloud altında yeni dosyayı yüklediğini izler. Orijinal son karşılaştırmadan önce kenara taşınır; böylece kopyalama sırasında araya giren bir yazma yakalanır ve taşıma geri alınır. iCloud altında `move-out`, iCloud depoyu hâlâ yüklerken reddeder; `--wait-upload 10m` bunun yerine o kadar süre bekler.

Her değişiklik, gerçekleşmeden önce bir günlüğe kaydedilir; oluşturduğu her şeyin cihazı ve inode'u (ve `.git` dosyasının baytları) ile birlikte. Bir geri alma başka hiçbir şeye dokunmaz. Süreç yarıda ölürse, bir sonraki `move-out --apply` (veya `undo --apply`) işi tamamlar ya da geri alır ve orada durur (devam etmek için komutu yeniden çalıştırın); yarım kalan iş, başka bir yazımla, bir yeniden adlandırmadan sonra veya başka bir deponun içinde bile klasörün inode'u ile bulunur. Bu arada `check` yarım kalan değişikliği hasar olarak raporlar. Git taşınan dizini kullanmaya başladıktan sonra hiçbir şey onun üzerine geri alınmaz: orijinalde olmayan bir şey içeriyorsa komut durur ve ikisini de adlandırır. Hiçbir şey silinmez: orijinal, siz kaldırana kadar yedekte kalır. `undo`, şu an kullanılan git dizinini taşımadan bu yana yapılan commit'lerle birlikte geri getirir; taşımadan önceki yedeğe dokunulmaz. Senkronizasyon istemcisi `.git` dosyasını boşalttıysa (bu Mac'te yalnızca adı duruyorsa), `undo` ve kurtarma sizden önce onu indirmenizi ister: onu okumak indirmek demektir ve araç bunu asla yapmaz.

Bir geri alma hem orijinali (yedekte) hem de yeni bir `.git`'i (taşıma durduktan sonra sizin veya senkronizasyon istemcisinin oluşturduğu) bulursa, ikisinin de üzerine yazmaz ve bunu söyler: ikisini karşılaştırın, istemediğiniz `.git`'i klasörün dışına taşıyın ve komutu yeniden çalıştırın.

Şu durumlarda reddeder (çıkış 1, gerekçeleriyle): git veya bir git kilidi tutan bir şey depo üzerinde çalışırken, bir merge veya rebase sırasında, `.git` dosyaları yalnızca buluttayken, iCloud klasörü hâlâ yüklerken, `.git` içinde bir soket (çalışan bir `git fsmonitor--daemon`) varken, bir taşımadan sonra iki git dizini varken ve linkleri bozulacak yapılarda: bağlı worktree'ler, alt modüller, ödünç alınmış nesneler (`alternates`), symlink olan veya git dizininin kendisi olmayan bir `.git`. `check`'in bulduğu diğer hasarlar olduğu gibi taşınır ve taşınanlar olarak listelenir: sonrasında `check`'i yeniden çalıştırın.

### Diğer cihazlar

Senkronizasyon istemcisi, klasörü senkronize eden diğer her cihazda `.git`'i siler ve yerine `.git` dosyasını getirir. Onların geçmişini korumak için:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: diğer her cihazda move-out --prepare --apply. 2: tek bir cihazda move-out --apply. 3: diğer her cihazda move-out --attach --apply">
</p>

1. Diğer her cihazda, taşımadan önce: `gitbrella move-out --prepare --apply`, o cihazın `.git`'ini `.git` dosyasının göstereceği yere kopyalar.
2. Tek bir cihazda: `gitbrella move-out --apply`.
3. Diğer her cihazda, `.git` dosyası geldikten sonra: `gitbrella move-out --attach --apply`. Hazırlanmamış bir cihaz bunun yerine geçmişi klonlar: `--attach --from <remote URL> --apply`. Her iki durumda da çalışma ağacına dokunulmaz.

`.git` dosyası senkronizasyon istemcisi üzerinden gelir; bu yüzden attach ona yalnızca bu cihazın kayıtlarının izin verdiği ölçüde güvenir: dosya bu cihazın `~/.local/share/gitbrella/gitdirs` dizinini göstermelidir ve orada zaten bulunan bir dizin yalnızca `--prepare` onu bu depo için oluşturduysa kullanılır.

Bundan sonra senkronizasyon istemcisi geçmişi değil dosyaları taşır. Commit'leri bir remote üzerinden paylaşın. Dosyaları senkronizasyon istemcisi tarafından zaten güncellenmiş bir cihazda `git pull` reddeder ("would be overwritten"): `git fetch` ve ardından `git reset --mixed @{u}` kullanın; bu, dalı ve index'i taşır ve senkronize dosyaları olduğu gibi bırakır.

<a name="what-it-reports"></a>

## Raporladıkları

Hasar (çıkış 1):

| Kod | Anlamı |
| --- | --- |
| `INDEX_COPY` | index'in çakışma kopyası (`index 2`; Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive ve Seafile adları; Google Drive'ın `index (1)`'i, bir OneDrive klasöründe OneDrive'ın `index-COMPUTER`'ı ve eski Nextcloud, Synology Drive, pCloud ve Resilio adları; bunlar yalnızca git'in kendi dosya adları için eşleştirilir); geçerli bir index olup olmadığını ve HEAD ile eşleşip eşleşmediğini söyler |
| `REF_COPY` | bir dal veya tag ref'inin çakışma kopyası; kopyanın daha eski, daha yeni, ayrışmış olduğunu ya da eksik bir commit'i adlandırdığını söyler |
| `COPY_IN_GITDIR` | `.git` içindeki diğer her çakışma kopyası |
| `INDEX_MISSING` | index yok; git her dosyayı silinmiş gösterir (bir commit hook'u içinde: commit her dosyayı silerdi) |
| `INDEX_EMPTY` | HEAD'in dosyaları hâlâ çalışma ağacındayken index hiç dosya listelemiyor; senkronizasyon istemcisi index'i sildikten sonra git onu yeniden oluşturduğunda olduğu gibi. Bir commit her dosyayı silerdi |
| `INDEX_SHRUNK` | index, hâlâ çalışma ağacında olan HEAD dosyalarını artık listelemiyor (sparse checkout'ta: herhangi bir HEAD dosyası); senkronizasyon istemcisi index'i sildikten sonra git onu stage edilmiş dosyalardan yeniden oluşturduğunda olduğu gibi. Bir commit onları silerdi. Yeniden adlandırmalar, symlink'e veya `.gitmodules`'ın tanımladığı bir alt modüle dönüşen bir dizin ve diskten de silinen dosyalar sayılmaz; HEAD'in dosyaları listelenemediğinde doğrulanmamış olarak raporlanır |
| `HEAD_COMMIT_MISSING` | HEAD depoda olmayan bir commit'i adlandırıyor (ref, nesnelerinden önce geldi) |
| `ORPHAN_INDEX_LOCK` | index yok ve geride eski bir `index.lock` kalmış (çoğu zaman tam bir index) |
| `INDEX_INVALID` | index kendi sağlama toplamından geçemiyor |
| `STALE_LOCK` | çalışan hiçbir git'in tutamayacağı bir git kilidi: depo üzerinde hiçbiri çalışmıyor ya da hepsi kilit yazıldıktan sonra başlamış. Yalnızca bu makinedeki git süreçleri görülür: kilidi kaldırmadan önce depoyu bir container'da, klasörü paylaşan başka bir makinede veya git'i gömülü kullanan bir uygulamada hiçbir git'in kullanmadığından emin olun |
| `DATALESS_IN_GITDIR` | `.git` içindeki dosyalar yalnızca bulutta; nesne kontrolleri atlandı |
| `FOREIGN_FILE_IN_GITDIR` | `refs/` veya `objects/` içinde git'in oluşturmadığı bir dosya (`desktop.ini`, `objects/` içinde `.tmp` dosyaları) |
| `EMPTY_FILE_IN_GITDIR` | sıfır baytlık bir nesne veya ref |
| `GITDIR_COPY` | çalışma ağacında git dizininin bir kopyası (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | git dizini çalışma ağacının içinde başka bir adla duruyor (`.git.nosync`) ve `git add -A` onu commit ederdi |
| `WORKTREE_ADMIN_BROKEN` | bağlı bir worktree yönetim dosyalarını kaybetmiş |
| `STALE_OPERATION_STATE` | geride kalmış bir rebase dizini git'in bir rebase'in sürdüğünü düşünmesine yol açıyor |
| `FSCK_ERRORS` | yalnızca `--deep`: `git fsck` hata bildirdi (tamamlanmadığında aynı kod bilgi olur) |
| `REFTABLE_TABLE_UNLISTED` | bir reftable deposu, listesinin adlandırmadığı ref tabloları tutuyor: senkronizasyon istemcisinin getirdiği başka bir cihazın tabloları; o cihazın listesi git'in hiç okumadığı bir kopyaya dönüşmüş. Sonraki `git pack-refs` veya bakım onları ve onlarla birlikte o cihazın dallarını siler; önce git dizinini güvenli bir yere kopyalayın |
| `CHANGE_UNFINISHED` | gitbrella'nın bir `fix`, `undo` veya `move-out` işi tamamlanmadı (öldürüldü); orada git'i kullanmadan önce, tamamlamak veya geri almak için adlandırdığı komutu `--apply` ile çalıştırın |
| `GIT_DIR_RETURNED` | `move-out`'tan sonra `.git` yeniden bir dizin (senkronizasyon istemcisi eskisini geri getirdi), yani iki git dizini var; commit etmeden önce karşılaştırıp birini tutun |
| `STAGED_CONFLICT_COPY` | index, aynı zamanda listelediği bir dosyanın senkronizasyon çakışma kopyasını ekliyor (`notes.md`'nin yanında `notes 2.md`, kopyalanmış bir `src 2/` dizini, Syncthing adları); bir commit kopyayı geçmişe koyardı. Yalnızca adlar karar verir; böyle bir ada yeniden adlandırma ve orijinali olmadan kopya gibi adlandırılmış bir dosya sayılmaz; bölünmüş index altında kontrol edilmez |

Bilgi (çıkış durumu değişmez): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (git dizini senkronize klasörün dışına taşınmış: güvenli düzen), `WORKTREE_COPIES` (kendi dosyalarınızın çakışma kopyaları), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (alternatif bir nesne deposu yalnızca bulutta duran dosyalar tutuyor, bu yüzden nesne kontrolleri atlandı), `GC_PID_FOREIGN` (başka bir makinenin `gc.pid`'i: git otomatik gc'sini 12 saat atlar), `GC_LOG_FRESH` (bir günden yeni bir `gc.log` otomatik gc'yi durdurur), `HOOKS_IN_SYNCED_FOLDER` (`core.hooksPath` veya bir `core.fsmonitor` komutu git dizini dışındaki senkronize bir klasörde duruyor; böylece başka bir cihazdaki bir değişiklik burada çalışır), `WORKTREE_GUESSED` (çalışma ağacı olmadan `GIT_DIR` ve HEAD'in dosyalarından hiçbiri geçerli dizinde yok: çalışma ağacı kontrol edilmedi).

Yol hiç depo değilse, neden diskteyse nedenini söyler: yeniden adlandırılmış bir `.git 2`, git'in kendi dosyalarının yalnızca bulutta olması, HEAD'i olmayan bir git dizini, artık var olmayan bir git dizinini gösteren bir `.git` dosyası ya da bu makinede olmayan bir klasörü gösteren bir `core.worktree` (senkronizasyon istemcisinin getirdiği başka bir makinenin ayarı). Yolun git'in kullanamadığı bir `.git`'i varsa, git'in kendisinin yapacağı gibi dıştaki bir depo adına cevap vermek yerine orada durur (çıkış 2).

<a name="install"></a>

## Kurulum

git 2.31 veya daha yenisini gerektirir. macOS ve Linux, amd64 ve arm64, cgo yok.

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

ya da releases sayfasından bir arşiv indirin.

### Bir sürümü doğrulama

Her sürüm `checksums.txt`'yi (her arşivin ve SBOM'un SHA-256'sı), release iş akışının oluşturduğu anahtarsız cosign imzası `checksums.txt.sigstore.json`'u, her arşiv için bir SPDX SBOM'u ve bir GitHub build-provenance attestation'ını taşır. Önce imzayı, sonra arşivi imzalı listeye göre kontrol edin (örnek, Apple silicon için v1.2.3). cosign v3.1.3 veya daha yenisini (v2 hattında v2.6.5) kullanın: eski sürümler sahte bir paketle kandırılabilir (GHSA-fx35-mq7g-6g98).

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

Bir sürüm, gönderilen bir tag'den `.github/workflows/release.yml` tarafından `.goreleaser.yaml` ile derlenir ve bir kişi yayınlayana kadar taslak olarak kalır. İkililer `-trimpath` ve commit'in zamanıyla derlenir; böylece aynı tag ve aynı Go sürümü (iş akışı en yeni 1.26 yamasını alır) aynı ikilileri üretir; onları saran arşivler ve SBOM'lar ise derleme zamanlarını taşır.

<a name="limits"></a>

## Sınırlar

- `fix` yalnızca tek bir doğru cevabın olduğu yerde onarır ve gerisini çalıştırılacak komutla birlikte size bırakır; kalıcı çözüm, `.git`'i senkronize klasörün dışında tutan `move-out`'tur.
- iCloud'un kendisinin iki cihaz arasında oluşturduğu çakışma kopyaları testlerde yeniden üretilmedi; algılama, aynı adlandırmayı izleyen kopyalarla test edilmiştir.
- Codex, kabuk aracının `workdir`'ini hook'lara geçirmez: Codex'in başka bir klasörde çalıştırdığı bir komut oturumun klasöründe kontrol edilir.
- Windows ve Windows'taki OneDrive test edilmedi. reftable depoları dosya düzeyindeki kontrolleri ve listelenmemiş tablo kontrolünü alır; orada ref kopyaları dallarıyla karşılaştırılmaz.
- Bir adın bilerek taşıyabileceği işaretlere sahip kopya adları yalnızca git'in kendisinin adı oluşturduğu yerlerde eşleştirilir, asla çalışma ağacında değil: Google Drive'ın `index (1)`'i git'in kendi dosyaları için ve `refs/` içinde; OneDrive'ın `index-COMPUTER`'ı bir OneDrive klasörünün en üstündeki git'in kendi dosyaları için, cihaz adında büyük harf olan ve insanların bir yedeğin adını bitirdiği bir kelime olmayan (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` sizindir), en fazla 15 karakterli adlarla (daha uzun bir Mac adı kaçırılır); eski Nextcloud'un `_conflict-YYYYMMDD-HHMMSS`'i, Synology Drive'ınki, pCloud'un `.conflicted`'ı ve Resilio'nun `.Conflict`'i en üstteki git'in kendi dosyaları için (eski Nextcloud'unki ayrıca bir Nextcloud klasörünün `refs/` ve `logs/` dizinlerinde, orijinalinin yanında). Bir HEAD'in adlandırdığı dal (herhangi bir boşlukla `ref:` olarak veya symlink olarak), onun reflog'u ve onu içeren bir dizin, adları ne olursa olsun asla kenara alınmaz. Yani `report (1).pdf`, `fix.Conflict` adlı bir dal, `v1.0.conflicted` adlı bir tag veya `config-backup` sıradan adlardır.
- Alt modüller alt modülün içinden kontrol edilir. Üst projeden, `.git/modules/<name>/` içindeki bir çakışma kopyası yalnızca orijinali yanındaysa raporlanır ve kopya gibi adlandırılmış bir alt modül (`lib 2`) kopya olarak raporlanır.
- Bağlı bir worktree'nin eksik index'i, ana worktree kontrol edildiğinde değil, o worktree kontrol edildiğinde raporlanır.
- `.git` senkronize klasörün dışındaki bir depoya symlink'lenmişse, git'i ve kontrolü bağlı çalışma ağacından çalıştırın. Hiçbir dosyayı tutmayan depo dizininden, boşaltılmış bir index bilinçli bir "her şeyi sil" gibi görünür ve commit'i engellemez.
- Bazı bilinçli commit'ler `--no-verify` gerektirir: diskte kalan dosyaların takibini bırakmak, nesnelerin okunmaması gereken yerde her dosyayı silmek, başka bir hard link'i olan bir dosyanın yalnızca büyük/küçük harf veya Unicode farkıyla yeniden adlandırılması ve bir sparse checkout'ta (cone modunda olsun olmasın), takip edilen bir dosyayı kaldıran veya yeniden adlandıran ya da onu bir tür değişikliğiyle taşıyan (dizine dönüşen bir dosya, symlink'e veya alt modüle dönüşen bir dizin) her commit. Orada diskte hangi dosyaların olduğuna git'in sparse kalıpları karar verir; bu yüzden bilinçli bir kaldırma kaybolmuş bir index'ten ayırt edilemez; kalıplar okunmaz.
- Senkronizasyon istemcisi index ile birlikte dal ref'inin kendisini de kaybederse (loose dosyası yeni bir `packed-refs` gelmeden önce silinirse), HEAD yeni bir dal gibi görünür ve commit engellenmez: yalnızca stage edilmiş dosyaları tutan yeni bir geçmiş başlatır. Diskteki hiçbir şey silinmez ve eski geçmiş hâlâ reflog'dadır (`git reflog`).
- Bölünmüş bir index altında veya nesnelerin okunmaması gereken yerlerde (2.45'ten eski git'te kısmi klon, yalnızca bulutta duran alternatifler), index HEAD ile dosya dosya karşılaştırılmaz: yalnızca eksik veya boş bir index yakalanır.
- `core.splitIndex` ile, hiç dosya listelemeyen bir index tanınmaz (başlığı yalnızca bölünmüş kısmı sayar); bu yüzden ondan yapılan bir commit engellenmez.
- Hook'lar: yalnızca commit hook'ları onları çalıştıran git sürecinden bulunur. Diğer hook'lar (`pre-push`, `pre-applypatch`) kontrolü çalışma ağacından çalıştırmalıdır, yoksa reddedilirler (çıkış 2). Git sürecinin görülemediği yerlerde (hook'ları kendisi çalıştıran bir istemci veya engellenmiş süreç listeleme), kontrol geçerli dizine geri döner: o zaman git'in dizini çalışma ağacını geri göstermiyorsa (`--separate-git-dir`, `.git.nosync`, `--work-tree`), hiç dosyası olmayan bir index commit'i her zaman engeller (bilinçli bir silme için `--no-verify` kullanın), HEAD'in dosyalarından hiçbirini içermeyen bir geçerli dizin çalışma ağacı olarak kontrol edilmez (`WORKTREE_GUESSED`) ve başka bir depoya veya git dizinine geçen bir hook reddedilir (çıkış 2). Receive hook'ları (`pre-receive`) desteklenen bir kapı değildir.
- `move-out`: bir `.git` dosyası her araç tarafından anlaşılmaz. Git ve git'i çalıştıran araçlar çalışır; isomorphic-git (mobilde Obsidian Git) ve `.git`'i dizin olarak açan araçlar çalışmaz.
- `move-out`: senkronize klasörü yeniden adlandırmak veya taşımak göreli `.git` dosyasını bozar. `check` o zaman eksik dizini adlandırır; klasörü geri koyun veya `.git` dosyasını elle git dizinine yönlendirin.
- `move-out`, senkronize klasörün her cihazda ev dizini altında aynı yerde olmasını gerektirir (iCloud Drive her zaman öyledir; Dropbox bir cihazda `~/Dropbox`, başka bir cihazda `~/Library/CloudStorage/Dropbox` olabilir). Öyle olmadığında `.git` dosyası orada başka yeri gösterir ve `--attach` reddeder (`TARGET_UNEXPECTED`).
- Taşınan git dizini ev dizininin dosya sisteminde kalır; onu başka bir yere koymak için bir seçenek yoktur.
- `move-out --prepare`, cihazın geçmişini o anki haliyle kopyalar: hazırladıktan kısa süre sonra, arada yeni commit yapmadan taşıyın.
- `move-out` ve `fix`, bir uygulamanın iCloud klasöründe (Obsidian'ın `iCloud~md~obsidian`'ı) değil, iCloud Drive'ın kendi klasöründe denendi.

## Katkıda bulunma

Bkz. [CONTRIBUTING.md](../../CONTRIBUTING.md). Güvenlik sorunlarını herkese açık bir issue'da değil, [SECURITY.md](../../SECURITY.md)'de anlatıldığı şekilde bildirin.

## Lisans

MIT
