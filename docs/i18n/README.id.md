<p align="center">
  <img src="../../assets/logo.svg" width="132" alt="Logo gitbrella: payung dengan gagang berupa cabang git">
</p>

<h1 align="center">gitbrella</h1>

<p align="center">
  Menemukan kerusakan yang dibuat sinkronisasi cloud pada repositori git.<br>
  Tidak mengubah apa pun. Tidak mengunduh apa pun.
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
  <b>Bahasa Indonesia</b> ·
  <a href="README.vi.md">Tiếng Việt</a>
</p>

<p align="center">
  <a href="#usage">Penggunaan</a> ·
  <a href="#repairing">Perbaikan</a> ·
  <a href="#moving-git-out-of-the-synced-folder">Memindahkan .git</a> ·
  <a href="#what-it-reports">Yang dilaporkan</a> ·
  <a href="#install">Instalasi</a> ·
  <a href="#limits">Batasan</a>
</p>

iCloud Drive, Dropbox, OneDrive, Google Drive, dan Syncthing menyinkronkan file satu per satu. Git mengharapkan direktori `.git` berubah sebagai satu kesatuan, sehingga repositori di dalam folder yang disinkronkan mendapat salinan konflik seperti `.git/index 2` dan `refs/heads/main 2`, index yang hilang di samping `index.lock` yang tertinggal, serta objek yang hanya ada di cloud. FAQ git sendiri melarang menyimpan repositori di folder yang disinkronkan, tetapi pengaturan bawaan tetap menaruhnya di sana: Desktop dan Dokumen di iCloud pada macOS, pencadangan folder OneDrive di Windows, `Documents/GitHub`.

<p align="center">
  <img src="../../assets/sync-damage.svg" width="820" alt="Klien sinkronisasi menyalin .git antarperangkat file demi file dan meninggalkan salinan konflik seperti index 2 dan refs/heads/main 2, index.lock di samping index yang hilang, serta objek yang hanya ada di cloud">
</p>

`gitbrella check` memberi tahu apakah sebuah repositori memiliki kerusakan ini, biasanya dalam waktu kurang dari satu detik. Alat ini dibuat untuk dijalankan sebelum agen AI atau git hook menyentuh repositori. `gitbrella fix` memperbaiki apa yang memiliki tepat satu perbaikan yang benar, dan `gitbrella move-out` menghilangkan kerusakan dari akarnya: memindahkan `.git` keluar dari folder yang disinkronkan dan meninggalkan file `.git` yang menunjuk ke sana, sehingga klien sinkronisasi membawa file Anda dan git menyimpan riwayatnya sendiri.

| Perintah | Fungsi |
| --- | --- |
| `gitbrella check` | melaporkan kerusakan (hanya baca) |
| `gitbrella fix` | memperbaiki apa yang memiliki satu jawaban benar |
| `gitbrella move-out` | memindahkan `.git` keluar dari folder yang disinkronkan secara permanen |
| `gitbrella undo` | membatalkan perbaikan atau pemindahan terakhir |
| `gitbrella history` | daftar perubahan yang dibuat di perangkat ini |
| `gitbrella prune-backups` | menghapus cadangan lama dari perubahan yang tidak bisa dibatalkan lagi |
| `gitbrella hook --for claude\|codex` | menjaga pemanggilan alat oleh agen AI |

## Mulai cepat

**1. Periksa.** Hanya baca, tanpa unduhan, tanpa jaringan.

```sh
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/notes
gitbrella check               # kode 0: sehat, 1: rusak, 2: tidak bisa berjalan
```

**2. Perbaiki.** Hanya yang memiliki satu jawaban benar. Tanpa `--apply`, hanya menampilkan rencana.

```sh
gitbrella fix                 # apa yang akan diperbaiki dan apa yang diserahkan kepada Anda
gitbrella fix --apply         # memperbaiki; `gitbrella undo --apply` mengembalikannya
```

**3. Selesaikan secara permanen.** Pindahkan `.git` keluar dari folder yang disinkronkan.

```sh
gitbrella move-out            # apa yang akan terjadi dan apa yang bisa menghalanginya
gitbrella move-out --apply
```

**Tetap terlindungi.** Blokir commit di repositori yang rusak:

```sh
printf '#!/bin/sh\nexec gitbrella check\n' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Untuk agen AI, lihat [Untuk agen](#for-agents).

## Yang dijanjikan

- <img src="../../assets/icons/read-only.svg" width="18" alt=""> **`check` hanya membaca.** Tidak pernah menulis ke repositori. Git hanya menjalankan perintah baca, dengan `GIT_OPTIONAL_LOCKS=0` dan gc otomatis serta pemeliharaan dimatikan. Pengujian end-to-end untuk kasus berisiko (salinan index, index hilang, partial clone, split index, direktori git yang dipindahkan) mencatat ukuran, mtime, dan inode setiap file sebelum dan sesudah pemeriksaan lalu membandingkannya.
- <img src="../../assets/icons/no-download.svg" width="18" alt=""> **Tidak mengunduh apa pun dari cloud.** Di macOS, alat ini mematikan unduhan sesuai permintaan untuk prosesnya sendiri (dan git yang dijalankannya) sebelum menyentuh disk; jika macOS menolak, pemeriksaan tidak dijalankan (kode 2). File yang hanya ada di iCloud dilaporkan, bukan diunduh.
- <img src="../../assets/icons/no-network.svg" width="18" alt=""> **Tidak mengakses jaringan.** Partial clone tidak pernah mengambil objek yang hilang (`GIT_NO_LAZY_FETCH`); dengan git yang lebih lama dari 2.45, pemeriksaan objek dilewati.
- <img src="../../assets/icons/quiet.svg" width="18" alt=""> **Diam pada repositori yang sehat.** Lock dianggap kerusakan hanya jika sudah lama dan tidak ada git yang sedang bekerja pada repositori itu (di worktree mana pun) yang mungkin menulisnya: git membuat lock saat mengambilnya, jadi git yang dimulai belakangan tidak mungkin memegangnya. File sementara git dan file milik alat lain di dalam `.git` hanya bersifat informasi. Menjalankannya pada 89 repositori kerja yang sehat tidak menemukan kerusakan apa pun.

<a name="usage"></a>

## Penggunaan

```
gitbrella check [--json] [--deep] [--provider NAME] [path]
```

Kode keluar: **0** tidak ada kerusakan, **1** ditemukan kerusakan, **2** pemeriksaan tidak dapat dijalankan. Untuk perintah yang mengubah sesuatu (`move-out`, `fix`, `undo`): **0** direncanakan, selesai, atau tidak ada yang perlu dilakukan (`already`), **1** ditolak (dengan alasan), selesai dengan peringatan, atau ada kerusakan tersisa yang diserahkan kepada Anda (daftar MANUAL dari `fix`; `check` tetap melaporkannya), **2** gagal (dikembalikan atau dihentikan; JSON menyimpan apa yang berhasil dipulihkan dan entri jurnal) atau tidak bisa berjalan.

`--deep` juga menjalankan `git fsck` penuh (yang membaca semua objek, sehingga dilewati bila ada objek yang hanya di cloud). `--provider` menetapkan klien sinkronisasi alih-alih mendeteksinya dari path: `auto` (bawaan), `icloud`, `icloud-desktop-documents`, `dropbox`, `onedrive`, `google-drive`, `syncthing`, `nextcloud`, `cloud-storage`, `none`. Folder Syncthing dan Nextcloud (atau ownCloud) dikenali di mana saja dari penanda yang diletakkan kliennya di dalamnya (`.stfolder`, `.sync_*.db`). `fix` dan `move-out` menerima `--provider` yang sama; berikan nilai yang sama seperti untuk `check`, karena beberapa nama salinan hanya dihitung untuk klien tertentu. JSON kemudian berisi `"providerForced": true`.

<p align="center">
  <img src="../../assets/check-output.svg" width="820" alt="Keluaran gitbrella check: DAMAGE INDEX_COPY index 2, yang diperbaiki oleh gitbrella fix, dan INFO IN_CLOUD_FOLDER">
</p>

<details>
<summary>Keluaran yang sama dalam bentuk teks</summary>

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

### Sebagai git hook

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec gitbrella check
```

<p align="center">
  <img src="../../assets/hook-flow.svg" width="820" alt="git commit menjalankan hook pre-commit, yang menjalankan gitbrella check: dengan kode 0 commit berlanjut, dengan 1 atau 2 commit diblokir">
</p>

Di repositori yang rusak, commit diblokir; di repositori yang sehat, commit berjalan seperti biasa. Alat ini tahu bahwa hook dijalankan oleh git, sehingga tidak mengira lock milik commit itu sendiri sebagai lock basi.

**Repositori mana yang diperiksa.** Di dalam hook commit (git memberi mereka `GIT_INDEX_FILE` atau `GIT_DIR` absolut), repositori ditemukan dari proses git yang menjalankan hook, sehingga hook boleh berpindah direktori lebih dulu: ke direktori git, direktori hooks-nya, atau direktori di luar repositori mana pun. Ketika git menyebut repositori melalui `GIT_DIR` absolut (linked worktree, submodule, direktori git terpisah), setiap perintah git di dalam hook bekerja pada repositori itu ke mana pun hook berpindah, begitu juga pemeriksaan. Selain itu, yang diperiksa adalah tujuan direktori saat ini menurut git sendiri (direktori git yang ditunjuk `GIT_DIR` relatif di sana, atau repositori yang ditemukan dari sana); jika itu repositori lain, atau worktree lain dari repositori yang sama (hook yang berpindah ke sana, atau editor pesan commit), pemeriksaan menolak (kode 2) alih-alih menjawab untuk repositori lain.

Tanpa path, yang diperiksa adalah repositori yang diekspor git ke hook (`GIT_DIR`), sehingga konfigurasi dotfiles `--git-dir` dan direktori git di dalam worktree-nya sendiri juga berfungsi. Tanpa path, `GIT_DIR` yang diatur di shell juga dipakai; berbeda dengan git, `GIT_DIR` yang tidak ada diabaikan, dan linked worktree yang tidak bisa diikuti ditolak alih-alih menganggap direktori saat ini sebagai worktree-nya.

**Index yang dicatat oleh commit.** Jika `GIT_INDEX_FILE` menunjuk ke file di dalam direktori git (seperti yang dilakukan git untuk hook commit), file itulah yang dibaca. Jika index yang akan dicatat commit hilang atau kosong, atau tidak lagi mencantumkan file HEAD yang masih ada di worktree, commit diblokir sebelum mencatat penghapusannya. File index di tempat lain yang Anda tunjuk dengan `GIT_INDEX_FILE` tidak dibaca: di dalam hook, index milik repositori sendiri yang diperiksa, dan pemeriksaan manual dengan variabel itu ditolak (kode 2).

**Penghapusan yang disengaja.** Untuk berhenti melacak file yang tetap ada di disk (`git rm --cached`, berhenti melacak direktori yang kini diabaikan), atau untuk sengaja meng-commit "hapus semuanya" selagi file masih di disk (`git rm -r --cached .`), lewati hook dengan `git commit --no-verify`.

### Dengan pengelola hook

- **pre-commit** (pre-commit.com): tambahkan repositori ini dengan id hook `gitbrella` (pre-commit membangunnya dengan Go) atau `gitbrella-system` (binary di PATH); lihat `.pre-commit-hooks.yaml`.
- **lefthook**: sertakan `hooks/lefthook.yml` melalui `remotes`.
- **husky**: `.husky/pre-commit` yang berisi `gitbrella check`.
- **Overcommit**: `PreCommit: { Gitbrella: { enabled: true, required_executable: gitbrella, command: [gitbrella, check] } }`, lalu `overcommit --sign`.

Pada semuanya, kode bukan nol apa pun menghentikan commit, jadi pemeriksaan yang tidak dapat berjalan (kode 2) juga menghentikannya. Aider melakukan commit dengan `--no-verify` kecuali `git-commit-verify: true` diatur, dan dalam keadaan itu tidak ada hook commit yang berjalan sama sekali. Obsidian Git (desktop) melakukan commit dengan git sistem dan menjalankan hook; tambahkan folder yang berisi `gitbrella` ke pengaturan "Additional PATH"-nya, karena aplikasi yang dibuka dari Dock tidak mendapat PATH shell Anda. Obsidian Git di ponsel (isomorphic-git) tidak menjalankan hook.

<a name="for-agents"></a>

### Untuk agen

`--json` mencetak laporan yang sama sebagai dokumen stabil berversi (`schemaVersion: 1`, `findings` selalu berupa daftar). Putuskan berdasarkan kode keluar dan tampilkan temuan kepada pengguna.

Hook bawaan agen hanya menghentikan pemanggilan alat pada kode 2: kode 1 ("ditemukan kerusakan") membiarkan pemanggilan berjalan. `gitbrella hook --for claude|codex` berbicara dengan bahasa hook mereka. Perintah ini membaca event yang dikirim agen melalui stdin:

<p align="center">
  <img src="../../assets/agent-hook.svg" width="820" alt="Pemanggilan alat oleh agen melewati gitbrella hook: membaca dan mengedit berlanjut; di repositori yang rusak, penulisan git dan penulisan ke .git dihentikan dengan kode 2">
</p>

- **Sebelum pemanggilan alat** (`PreToolUse`): di repositori yang rusak, perintah git yang menulis (commit, add, reset, rebase, push, dan apa pun yang tidak diketahui hanya membaca) serta penulisan ke `.git` menghentikan pemanggilan (kode 2, dengan alasan untuk agen di stderr). Membaca, mengedit file Anda, `git restore`, dan `gitbrella` sendiri tetap berjalan, sehingga kerusakan dapat diperiksa dan diperbaiki.
  - *Repositori yang tidak dapat dipakai.* Jika file git hanya ada di cloud, atau git sama sekali tidak dapat menggunakan repositori (`HEAD` yang berganti nama, file `.git` yang menunjuk ke tempat yang tidak ada), setiap perintah git dihentikan: perintah itu akan macet atau bekerja pada repositori luar. Penulisan ke `.git` tetap berjalan di sana: perbaikannya dilakukan secara manual di dalam `.git` seperti yang disebutkan alasannya (`mv '.git/HEAD 2' .git/HEAD`). Perbaikan `core.worktree` yang disebut dalam petunjuk (`git config --file … --unset core.worktree`) juga berjalan.
  - *Repositori mana.* Repositori yang berisi file yang dikerjakan oleh pemanggilan (patch Codex menyebut file-filenya; path di dalam `.git` termasuk worktree di atasnya). Untuk perintah shell, folder sesi dan setiap folder tempat perintah git-nya berjalan (`cd DIR`, `pushd`, `git -C DIR`, `--git-dir`, `--work-tree`, `GIT_DIR`, `env -C`, `bash -c '…'`), yang menulis lebih dulu, paling banyak lima; jika folder yang menulis tidak muat, pemanggilan diteruskan dengan peringatan. Setiap folder dinilai berdasarkan apa yang dilakukan pemanggilan di sana: `git -C ../sound commit` dari folder sesi yang rusak tetap berjalan. Folder yang bukan repositori dan repositori bare lolos tanpa pesan.
  - *Saat pemeriksaan tidak dapat berjalan.* Apa pun yang mencegah pemeriksaan berjalan (termasuk `--timeout` 20 detik, satu batas waktu untuk seluruh pemanggilan) membiarkan pemanggilan lewat dengan peringatan; `--fail-open` melakukan hal yang sama di tempat git tidak dapat menggunakan repositori.
  - *Bagaimana perintah dibaca.* Perintah dibaca, tidak pernah dijalankan: tanda kutip, komentar, subshell, substitusi (termasuk di dalam heredoc tanpa kutip), heredoc, variabel yang diaturnya, `&&`, `||`, `if`, `case`, dan perulangan (setelah `cd a || cd b`, atau `cd` ke folder yang tidak ada, setiap folder tempat shell mungkin berada dihitung; perulangan `for` dibaca sekali untuk setiap nilai), pembungkus (`timeout`, `sudo`, `nice`, `xargs`, `env`), dan skrip yang diberikan ke shell (`-c`, `eval`, `watch`, `source`, file, heredoc, `bash < f`, pipa ke `sh`). Fungsi dan alias yang didefinisikan perintah dibaca di tempat dipanggil. Skrip shell yang dijalankan lewat path atau diberikan ke shell (`./deploy.sh`, `bash deploy.sh`) dibaca jika berupa file kecil di mesin ini (termasuk melalui symlink; sekali saja, bahkan jika menjalankan dirinya sendiri); `make`, `npm run`, dan sejenisnya tidak dibaca.
  - *Apa yang dihitung sebagai penulisan.* Penulisan program lain ke `.git` (`rm .git/index.lock`, `> .git/HEAD`, `cd .git && rm index.lock`, `sed -i` pada `.git/config`, `rm -rf` atau `find -delete` pada folder yang berisi `.git`, patch Codex melalui shell) dihitung sebagai penulisan ke `.git`; menyalin `.git` ke tempat lain, mengemasnya dengan `tar`, atau membacanya tidak. `gh pr checkout` dan `git-lfs` dihitung sebagai perintah git yang menulis.
  - *Jika ragu.* Masing-masing berikut dihitung sebagai perintah git yang menulis: pembacaan yang melebihi 5000 langkah atau separuh `--timeout` (di folder yang tidak dapat ditentukan); kata `git` di tempat yang tidak diikuti pembacaan; skrip yang dialirkan ke shell dari program yang tidak diikuti pembacaan (`curl … | sh`), di tempat skrip itu berjalan; skrip yang ada tetapi tidak dapat dibaca yang diberikan ke shell (terlalu besar, hanya di cloud), di tempat skrip itu berjalan. Jika tidak dapat diketahui folder mana yang ditulis perintah git (`cd "$(…)"`, `for d in */`), perintah itu dihitung di setiap folder yang diperiksa, dan pemanggilan mendapat peringatan.
- **Saat sesi dimulai** (`SessionStart`): satu baris ke stdout yang dibaca agen sebagai konteks: bahwa repositori berada di folder yang disinkronkan (jika direktori git-nya belum dipindahkan keluar) dan apa yang rusak. Tidak pernah memblokir.

Hook tidak mengubah apa pun. Untuk Claude Code, di `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for claude"}]}]
  }
}
```

Untuk Codex, di `.codex/hooks.json` (Codex hanya menjalankan hook setelah Anda mempercayainya di `/hooks`):

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}],
    "PreToolUse": [{"matcher": "Bash|apply_patch",
                    "hooks": [{"type": "command", "command": "gitbrella hook --for codex"}]}]
  }
}
```

Pemeriksaan dijalankan sekali untuk setiap folder tersebut, biasanya 0,1–0,4 detik masing-masing, sehingga patch yang menyentuh beberapa folder dari satu repositori memeriksa repositori itu beberapa kali; `--timeout` membatasi seluruh pemanggilan.

<a name="repairing"></a>

## Perbaikan

```sh
gitbrella fix                 # apa yang akan diperbaiki dan apa yang diserahkan kepada Anda
gitbrella fix --apply         # memperbaiki
gitbrella undo --apply        # mengembalikan semuanya seperti semula
gitbrella history             # perubahan yang dibuat di sini, cadangannya, dan ukurannya
gitbrella prune-backups       # cadangan yang tidak lagi dibutuhkan undo mana pun
```

<p align="center">
  <img src="../../assets/fix-flow.svg" width="820" alt="fix menampilkan rencana; fix --apply mencatat setiap langkah, lalu pemeriksaan dijalankan ulang: selesai atau dikembalikan; undo --apply membatalkan perubahan terakhir">
</p>

`check` mencantumkan di bawah setiap kerusakan apakah `fix` memperbaikinya (`repair:`, dan `"repair": "fix"` atau `"manual"` di JSON), serta menyebut `fix --provider NAME` jika pemeriksaan diberi provider. Di dalam hook commit, alat ini tidak menanyakan apa pun.

`fix` hanya memperbaiki jika ada satu jawaban benar dan tidak ada yang mungkin Anda maksudkan yang tertimpa:

| Kerusakan | Perbaikan |
| --- | --- |
| salinan konflik di `.git` di samping aslinya (`HEAD 2`, `refs/heads/main 2`, `refs 2`, `packed-refs 2`, salinan Dropbox dan Syncthing, serta salinan lock ref seperti `refs/heads/main 2.lock`); salinan index hanya jika identik dengan index atau merupakan index valid yang sama dengan HEAD | dipindahkan ke cadangan; objek (commit, tag, tree, atau file) yang hanya ditunjuk oleh salinan disimpan lebih dulu sebagai `refs/gcd-rescue/<name>` |
| direktori rebase yang tertinggal, `desktop.ini`, `Icon`, dan file serupa yang tidak dibuat git (kecuali git membacanya sebagai cabang atau namanya adalah nama objek), salinan `.git` di worktree yang commit-nya sudah ada di repositori | dipindahkan ke cadangan |
| index hilang tanpa salinan dan tanpa lock di mana pun | dibangun ulang dari HEAD (`git read-tree HEAD`: file tidak disentuh), hanya jika ada commit untuk membangunnya (jika commit HEAD juga hilang, commit itu diambil ulang lebih dulu seperti di bawah) dan di luar sparse checkout |
| index yang tidak mencantumkan file apa pun (index valid, tidak terpisah, di luar sparse checkout) | dipindahkan ke cadangan dan dibangun ulang dari HEAD: tidak ada yang di-stage di dalamnya |
| `index.lock` yatim yang merupakan index valid sama dengan HEAD | menjadi index |
| file cabang kosong | commit yang terakhir dicatat oleh reflog-nya |
| `.git.nosync` yang akan di-commit oleh git | baris `/.git.nosync/` di `.git/info/exclude` |
| commit HEAD yang hilang atau file loose object kosong (tanpa salinan di sampingnya) | file kosong dipindahkan ke cadangan dan objek diambil ulang (`git fetch --refetch`, git 2.36 atau lebih baru) dari `--remote`, jika tidak dari remote cabang, jika tidak dari `origin`, jika tidak dari satu-satunya remote; cabang dan tag remote diambil ke namespace sementara `refs/gcd-fetch/` yang kemudian dihapus, sehingga tidak ada ref yang berubah. Jika remote tidak mengirim objek itu, perbaikan dikembalikan |
| dengan `fix --deep`: objek yang dilaporkan rusak oleh `git fsck` | setiap objek cabang dan tag remote diambil ulang dengan cara yang sama; lalu `git fsck` dijalankan lagi, dan jika masih melaporkan kesalahan (objek yang tidak ada di remote mana pun, file rusak yang tertinggal), perbaikan dikembalikan |
| salinan konflik dari file Anda sendiri (`notes 2.md`) yang tidak dilacak git dan sama byte demi byte dengan file yang disalin | dipindahkan ke cadangan (`git add -A` akan meng-commit-nya); salinan yang berbeda, dilacak, berupa link, hanya di cloud, atau berada di dalam submodule tetap dibiarkan |
| `gc.pid` milik mesin lain yang lebih lama dari satu jam; `gc.log` | dipindahkan ke cadangan agar gc otomatis git berjalan lagi di sini |
| file `.git` atau penyimpanan objek alternate yang hanya ada di cloud | diunduh, dan tidak lebih: sebelum itu pemeriksaan tidak dapat membaca objek, jadi kerusakan lain dicantumkan sebagai MANUAL hingga `fix` dijalankan berikutnya |

Selebihnya dicantumkan sebagai MANUAL bersama perintah yang harus dijalankan. Termasuk apa pun yang mungkin berisi pekerjaan yang tidak ada di tempat lain:
- index yang rusak (git mungkin masih bisa membacanya beserta pekerjaan yang sudah di-stage);
- salinan index atau `index.lock` yang berbeda dari HEAD, serta salinan `index.lock`;
- salinan yang tidak memiliki asli (mungkin itu satu-satunya: kembalikan ke tempatnya; jika berupa salinan lock, disimpan lalu disisihkan);
- lock yang tidak dipegang git mana pun di mesin ini (mungkin dipegang git di perangkat lain atau di container; saran menyebut commit di dalamnya);
- salinan yang menunjuk lebih dari 100 objek (reflog yang panjang);
- salinan dari cabang yang ditunjuk HEAD mana pun, reflog-nya, atau direktori yang memuatnya (git membacanya, bagaimanapun namanya terlihat).

Dan apa pun yang mengharuskan fix memilih di antara dua versi atau tidak dapat menyimpan commit salinan:
- apa pun di penyimpanan objek: salinan, file dengan nama objek (periksa mana yang cocok dengan hash-nya), file objek kosong tanpa remote untuk diambil ulang;
- apa pun di direktori git submodule (jalankan `fix` di submodule);
- salinan cabang yang menunjuk commit yang belum ada di sini (mungkin masih akan datang);
- salinan `.git` di worktree yang commit-nya tidak ada di repositori (Anda yang memutuskan mana yang terkini);
- file kosong yang di sampingnya ada salinan dirinya atau direktorinya;
- direktori administrasi linked worktree yang rusak (`git worktree repair`);
- index yang kehilangan file yang masih ada di disk (berhenti melacak dengan sengaja terlihat sama; jika tidak ada lagi yang di-stage, saran mengatakan bahwa `git read-tree HEAD` mengembalikannya tanpa kehilangan apa pun);
- tanpa `--deep` atau tanpa remote, objek yang dilaporkan rusak oleh `git fsck`.

`fix` sama sekali tidak menyentuh repositori reftable; di sana temuan `REFTABLE_TABLE_UNLISTED` dari `check` menjelaskan cara menyimpan ref perangkat lain. Cadangan harus berada di disk yang sama dengan repositori agar apa yang disisihkan fix berpindah dalam satu langkah; jika tidak, `fix` menolak (perbaikan yang hanya berupa unduhan tidak memindahkan apa pun dan tidak memerlukannya).

Setiap langkah dicatat sebelum dijalankan, dan di akhir pemeriksaan dijalankan ulang: jika apa yang diperbaiki masih terdeteksi atau muncul kerusakan baru, langkah-langkahnya dibatalkan. Fix yang terputus dikembalikan oleh perintah pengubah berikutnya yang dijalankan dengan `--apply` di repositori itu; setelah itu perintah tersebut berhenti — jalankan lagi. Setelah 30 detik (`--watch`), `fix` memeriksa sekali lagi: kerusakan yang kembali dibawa oleh klien sinkronisasi, dan solusi jangka panjangnya adalah `move-out`.

`undo` membatalkan perubahan terbaru lebih dulu, baik perbaikan maupun pemindahan. Langkah yang hasilnya berubah sejak perbaikan dibiarkan apa adanya dan dicantumkan, tetapi tidak pernah ditimpa: git menulis ulang index yang dibangun ulang, klien sinkronisasi mengembalikan salinan, rescue ref bergeser, cadangan dihapus, atau `.git` dipindahkan. Rescue ref tetap ada selama salinan yang disimpannya masih di cadangan. File yang diunduh tetap di disk, dan objek yang diambil ulang tetap di penyimpanan objek: keduanya tidak menggantikan apa pun. Sisanya dibatalkan dan undo selesai, sehingga perubahan sebelumnya dapat dibatalkan berikutnya.

`history` mencantumkan perubahan yang dibuat perangkat ini pada repositori (perbaikan, pemindahan, pembatalan), cadangan masing-masing beserta ukurannya, dan perubahan mana yang akan dibatalkan `undo` berikutnya. `prune-backups`, hanya dengan `--apply`, menghapus cadangan dari perubahan yang telah dibatalkan, dikembalikan, atau memang tidak pernah bisa dibatalkan (undo, prepare, attach), dan yang langkah terakhirnya lebih lama dari `--older-than` (bawaan `720h`). Perbaikan atau pemindahan yang masih bisa dibatalkan mempertahankan cadangannya, meskipun ada perubahan yang lebih baru di atasnya.

<a name="moving-git-out-of-the-synced-folder"></a>

## Memindahkan .git keluar dari folder yang disinkronkan

```sh
gitbrella move-out            # apa yang akan terjadi dan apa yang bisa menghalanginya
gitbrella move-out --apply    # menjalankannya
gitbrella undo --apply        # mengembalikannya
```

<p align="center">
  <img src="../../assets/move-out.svg" width="860" alt="Sebelum: .git di dalam folder yang disinkronkan. Setelah move-out: file .git di folder yang disinkronkan menunjuk ke ~/.local/share/gitbrella/gitdirs/notes-1a2b3c4d di luar">
</p>

`move-out` menyalin `.git` ke `~/.local/share/gitbrella/gitdirs/<name>` (di macOS berupa clone APFS yang tidak memakan ruang), membuktikan file demi file bahwa salinannya identik, memindahkan aslinya ke cadangan di `~/Library/Application Support/gitbrella` (Linux: `$XDG_STATE_HOME/gitbrella`), dan menulis file `.git` seperti `gitdir: ../../../../.local/share/gitbrella/gitdirs/notes-1a2b3c4d`. Path-nya relatif, sehingga setiap perangkat yang folder-nya berada di tempat yang sama di dalam direktori home menulis file `.git` yang sama. Kemudian dipastikan bahwa git menemukan HEAD, status, konfigurasi, dan direktori hooks yang sama, dan selama 30 detik (`--watch`) bahwa klien sinkronisasi tidak menyentuh `.git` dan, di iCloud, mengunggah file baru. Aslinya disisihkan sebelum perbandingan akhir, sehingga penulisan yang menyelinap selama penyalinan akan terdeteksi dan pemindahan dibatalkan. Di iCloud, `move-out` menolak selama iCloud masih mengunggah repositori; `--wait-upload 10m` menunggu hingga waktu tersebut.

Setiap perubahan dicatat ke jurnal sebelum dilakukan, bersama perangkat dan inode dari semua yang dibuatnya (dan byte file `.git`); pengembalian tidak menyentuh apa pun selain itu. Jika proses terputus di tengah jalan, `move-out --apply` berikutnya (atau `undo --apply`) menyelesaikan atau mengembalikannya lalu berhenti (jalankan lagi untuk melanjutkan); perubahan yang belum selesai ditemukan dari inode folder, bahkan jika path-nya ditulis berbeda, setelah folder diganti nama, atau di dalam repositori lain. Sementara itu, `check` melaporkan perubahan yang belum selesai sebagai kerusakan. Tidak ada yang dikembalikan di atas direktori yang dipindahkan setelah git mulai menggunakannya: jika direktori itu berisi sesuatu yang tidak ada di aslinya, perintah berhenti dan menyebut keduanya. Tidak ada yang dihapus: aslinya tetap di cadangan sampai Anda menghapusnya. `undo` mengembalikan direktori git yang sedang dipakai, termasuk commit yang dibuat sejak pemindahan; cadangan dari sebelum pemindahan tidak disentuh. Jika klien sinkronisasi telah melepas file `.git` (hanya namanya yang tersisa di Mac ini), `undo` dan pemulihan meminta Anda mengunduhnya lebih dulu: membacanya akan memicu unduhan, dan alat ini tidak pernah melakukannya.

Jika pengembalian menemukan aslinya (di cadangan) sekaligus `.git` baru (dibuat oleh Anda atau klien sinkronisasi setelah pemindahan berhenti), keduanya tidak ditimpa dan hal itu dilaporkan: bandingkan keduanya, pindahkan `.git` yang tidak Anda inginkan keluar dari folder, lalu jalankan perintah lagi.

Menolak (kode 1, dengan alasan) selama git atau sesuatu yang memegang lock git sedang bekerja pada repositori, selama merge atau rebase, selama ada file `.git` yang hanya di cloud, selama iCloud masih mengunggah folder, selama `.git` berisi socket (`git fsmonitor--daemon` sedang berjalan), ketika setelah pemindahan ada dua direktori git, serta untuk konfigurasi yang tautannya akan rusak: linked worktree, submodule, objek pinjaman (`alternates`), `.git` yang berupa symlink atau bukan direktori git itu sendiri. Kerusakan lain yang ditemukan `check` ikut dipindahkan apa adanya dan dicantumkan sebagai ikut terbawa: jalankan `check` lagi sesudahnya.

### Perangkat lain

Klien sinkronisasi menghapus `.git` di setiap perangkat lain yang menyinkronkan folder itu dan mengirimkan file `.git` ke sana. Untuk menyimpan riwayat mereka:

<p align="center">
  <img src="../../assets/other-devices.svg" width="820" alt="1: move-out --prepare --apply di setiap perangkat lain. 2: move-out --apply di satu perangkat. 3: move-out --attach --apply di setiap perangkat lain">
</p>

1. Di setiap perangkat lain, sebelum pemindahan: `gitbrella move-out --prepare --apply` menyalin `.git` perangkat itu ke tempat yang akan ditunjuk file `.git`.
2. Di satu perangkat: `gitbrella move-out --apply`.
3. Di setiap perangkat lain, setelah file `.git` tiba: `gitbrella move-out --attach --apply`. Perangkat yang tidak disiapkan meng-clone riwayatnya: `--attach --from <remote URL> --apply`. Keduanya tidak menyentuh worktree.

File `.git` tiba melalui klien sinkronisasi, sehingga attach hanya memercayainya sejauh catatan perangkat ini mengizinkan: file itu harus menunjuk ke dalam `~/.local/share/gitbrella/gitdirs` milik perangkat ini, dan direktori yang sudah ada di sana hanya digunakan jika dibuat oleh `--prepare` untuk repositori ini.

Mulai saat itu, klien sinkronisasi membawa file, bukan riwayat. Bagikan commit melalui remote. Di perangkat tempat klien sinkronisasi sudah memperbarui file, `git pull` menolak ("would be overwritten"): gunakan `git fetch` lalu `git reset --mixed @{u}`, yang memindahkan cabang dan index serta membiarkan file yang disinkronkan apa adanya.

<a name="what-it-reports"></a>

## Yang dilaporkan

Kerusakan (kode 1):

| Kode | Arti |
| --- | --- |
| `INDEX_COPY` | salinan konflik index (`index 2`; nama Dropbox, Syncthing, Nextcloud/ownCloud, Proton Drive, dan Seafile; `index (1)` dari Google Drive, `index-COMPUTER` dari OneDrive di folder OneDrive, serta nama Nextcloud lama, Synology Drive, pCloud, dan Resilio hanya dicocokkan dengan nama file milik git sendiri); menyatakan apakah itu index valid dan apakah sama dengan HEAD |
| `REF_COPY` | salinan konflik ref cabang atau tag; menyatakan apakah salinan lebih lama, lebih baru, menyimpang, atau menunjuk commit yang hilang |
| `COPY_IN_GITDIR` | salinan konflik lainnya di `.git` |
| `INDEX_MISSING` | index hilang; git menunjukkan setiap file sebagai terhapus (di hook commit: commit akan menghapus semua file) |
| `INDEX_EMPTY` | index tidak mencantumkan file apa pun padahal file HEAD masih ada di worktree, seperti yang terjadi ketika git membangun ulang index setelah klien sinkronisasi menghapusnya; commit akan menghapus semua file |
| `INDEX_SHRUNK` | index tidak lagi mencantumkan file HEAD yang masih ada di worktree (di sparse checkout: file HEAD mana pun), seperti yang terjadi ketika git membangunnya ulang dari file yang di-stage setelah klien sinkronisasi menghapus index; commit akan menghapusnya. Penggantian nama, direktori yang menjadi symlink atau submodule yang dideklarasikan di `.gitmodules`, dan file yang juga terhapus dari disk tidak dihitung; jika file HEAD tidak dapat dicantumkan, dilaporkan sebagai tidak terverifikasi |
| `HEAD_COMMIT_MISSING` | HEAD menunjuk commit yang tidak ada di repositori (ref tiba sebelum objeknya) |
| `ORPHAN_INDEX_LOCK` | index hilang dan `index.lock` lama tertinggal (sering kali berupa index lengkap) |
| `INDEX_INVALID` | index gagal pada checksum-nya sendiri |
| `STALE_LOCK` | lock git yang tidak mungkin dipegang git yang sedang berjalan: tidak ada git yang bekerja pada repositori, atau semuanya dimulai setelah lock ditulis. Hanya proses git di mesin ini yang terlihat: sebelum menghapus lock, pastikan tidak ada git yang menggunakan repositori di container, di mesin lain yang berbagi folder, atau di aplikasi dengan git bawaan |
| `DATALESS_IN_GITDIR` | file di `.git` hanya ada di cloud; pemeriksaan objek dilewati |
| `FOREIGN_FILE_IN_GITDIR` | file di `refs/` atau `objects/` yang tidak dibuat git (`desktop.ini`, file `.tmp` di `objects/`) |
| `EMPTY_FILE_IN_GITDIR` | objek atau ref berukuran nol byte |
| `GITDIR_COPY` | salinan direktori git di worktree (`.git 2`) |
| `GIT_DIR_NOT_IGNORED` | direktori git berada di worktree dengan nama lain (`.git.nosync`) dan `git add -A` akan meng-commit-nya |
| `WORKTREE_ADMIN_BROKEN` | linked worktree kehilangan file administrasinya |
| `STALE_OPERATION_STATE` | direktori rebase yang tertinggal membuat git mengira rebase sedang berlangsung |
| `FSCK_ERRORS` | hanya `--deep`: `git fsck` melaporkan kesalahan (jika tidak selesai, kode yang sama menjadi informasi) |
| `REFTABLE_TABLE_UNLISTED` | repositori reftable memiliki tabel ref yang tidak disebut dalam daftarnya: tabel perangkat lain yang dibawa klien sinkronisasi, yang daftarnya menjadi salinan yang tidak pernah dibaca git. `git pack-refs` atau pemeliharaan berikutnya akan menghapusnya beserta cabang perangkat itu; salin direktori git ke tempat aman lebih dulu |
| `CHANGE_UNFINISHED` | `fix`, `undo`, atau `move-out` gitbrella tidak selesai (terputus); sebelum menggunakan git di sana, jalankan perintah yang disebut dengan `--apply` untuk menyelesaikan atau mengembalikannya |
| `GIT_DIR_RETURNED` | setelah `move-out`, `.git` kembali menjadi direktori (klien sinkronisasi mengembalikan yang lama), sehingga ada dua direktori git; bandingkan dan simpan satu sebelum melakukan commit |
| `STAGED_CONFLICT_COPY` | index menambahkan salinan konflik sinkronisasi dari file yang juga dicantumkannya (`notes 2.md` di samping `notes.md`, direktori tersalin `src 2/`, nama Syncthing); commit akan memasukkan salinan itu ke riwayat. Hanya nama yang menentukan; penggantian nama ke nama seperti itu dan file bernama salinan tanpa asli tidak dihitung; tidak diperiksa saat split index |

Informasi (kode keluar tidak berubah): `IN_CLOUD_FOLDER`, `GIT_DIR_NOT_SYNCED` (direktori git sudah dipindahkan keluar dari folder yang disinkronkan: konfigurasi aman), `WORKTREE_COPIES` (salinan konflik dari file Anda sendiri), `GIT_RUNNING`, `LOCK_IN_USE`, `LOCK_RECENT`, `IN_PROGRESS_OPERATION`, `UNKNOWN_IN_GITDIR`, `NO_CHECKOUT`, `PARTIAL_CLONE_UNVERIFIED`, `WORKTREE_SCAN_TRUNCATED`, `DEEP_CHECK_SKIPPED`, `REFTABLE_UNSUPPORTED`, `UNREADABLE_IN_GITDIR`, `PROCESS_CHECK_FAILED`, `GIT_DIR_IGNORE_UNCHECKED`, `GIT_TEMP_FILE`, `DATALESS_IN_ALTERNATE` (penyimpanan objek alternate memiliki file yang hanya di cloud, sehingga pemeriksaan objek dilewati), `GC_PID_FOREIGN` (`gc.pid` milik mesin lain: git melewati gc otomatis selama 12 jam), `GC_LOG_FRESH` (`gc.log` yang berumur kurang dari sehari menghentikan gc otomatis), `HOOKS_IN_SYNCED_FOLDER` (perintah `core.hooksPath` atau `core.fsmonitor` berada di folder yang disinkronkan di luar direktori git, sehingga perubahan di perangkat lain dijalankan di sini), `WORKTREE_GUESSED` (`GIT_DIR` tanpa worktree dan direktori saat ini tidak berisi file HEAD apa pun: worktree tidak diperiksa).

Jika path sama sekali bukan repositori, alasannya dilaporkan bila penyebabnya ada di disk: `.git 2` yang berganti nama, file git sendiri yang hanya di cloud, direktori git tanpa HEAD, file `.git` yang menunjuk ke direktori git yang sudah tidak ada, atau `core.worktree` yang menunjuk ke folder yang tidak ada di mesin ini (pengaturan mesin lain yang dibawa klien sinkronisasi). Jika ada `.git` di jalur itu yang tidak dapat digunakan git, pemeriksaan berhenti di sana (kode 2) alih-alih menjawab untuk repositori luar seperti yang akan dilakukan git sendiri.

<a name="install"></a>

## Instalasi

Membutuhkan git 2.31 atau lebih baru. macOS dan Linux, amd64 dan arm64, tanpa cgo.

```
go install github.com/aliyilmazco/gitbrella/cmd/gitbrella@latest
```

atau unduh arsip dari halaman releases.

### Memverifikasi rilis

Setiap rilis menyertakan `checksums.txt` (SHA-256 dari setiap arsip dan SBOM), tanda tangan cosign tanpa kunci `checksums.txt.sigstore.json` yang dibuat oleh workflow rilis, SBOM SPDX untuk setiap arsip, dan attestation asal-usul build (build provenance) GitHub. Verifikasi tanda tangan lebih dulu, lalu arsip terhadap daftar yang ditandatangani (contohnya v1.2.3 untuk Apple silicon). Gunakan cosign v3.1.3 atau lebih baru (v2.6.5 di jalur v2): versi lama dapat diakali dengan bundle palsu (GHSA-fx35-mq7g-6g98).

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

Rilis dibangun dari tag yang di-push oleh workflow `.github/workflows/release.yml` menggunakan `.goreleaser.yaml`, dan tetap berupa draf hingga seseorang memublikasikannya. Binary dibangun dengan `-trimpath` dan waktu commit, sehingga tag yang sama dan rilis Go yang sama (workflow memakai patch 1.26 terbaru) menghasilkan binary yang identik; arsip dan SBOM di sekitarnya memuat waktu build.

<a name="limits"></a>

## Batasan

- `fix` hanya memperbaiki jika ada satu jawaban benar dan menyerahkan sisanya kepada Anda beserta perintah untuk dijalankan; solusi jangka panjangnya adalah `move-out`, yang menjaga `.git` tetap di luar folder yang disinkronkan.
- Salinan konflik yang dibuat iCloud sendiri antara dua perangkat belum direproduksi dalam pengujian; deteksi diuji pada salinan dengan penamaan yang sama.
- Codex tidak meneruskan `workdir` alat shell-nya ke hook: perintah yang dijalankan Codex di folder lain diperiksa di folder sesi.
- Windows dan OneDrive di Windows belum diuji. Repositori reftable mendapat pemeriksaan tingkat file dan pemeriksaan tabel yang tidak tercantum; salinan ref di sana tidak dibandingkan dengan cabangnya.
- Nama salinan dengan tanda yang mungkin memang disengaja dalam sebuah nama hanya dicocokkan di tempat git sendiri yang membuat nama itu, dan tidak pernah di worktree: `index (1)` dari Google Drive pada file milik git sendiri dan di `refs/`; `index-COMPUTER` dari OneDrive pada file milik git sendiri di tingkat teratas folder OneDrive, dengan nama perangkat yang memiliki huruf besar, bukan kata yang biasa mengakhiri nama cadangan (`index-old`, `config-OLD`, `config-Backup`, `index-BAK` adalah milik Anda), dan paling banyak 15 karakter (nama Mac yang lebih panjang akan terlewat); `_conflict-YYYYMMDD-HHMMSS` dari Nextcloud lama, nama Synology Drive, `.conflicted` dari pCloud, dan `.Conflict` dari Resilio pada file milik git sendiri di tingkat teratas (nama Nextcloud lama juga di `refs/` dan `logs/` di samping aslinya di folder Nextcloud). Cabang yang ditunjuk HEAD mana pun (sebagai `ref:` dengan spasi apa pun, atau sebagai symlink), reflog-nya, dan direktori yang memuatnya tidak pernah disisihkan, apa pun namanya. Jadi `report (1).pdf`, cabang `fix.Conflict`, tag `v1.0.conflicted`, atau `config-backup` adalah nama biasa.
- Submodule diperiksa dari dalam submodule. Dari proyek induk, salinan konflik di `.git/modules/<name>/` terdeteksi hanya jika aslinya ada di sampingnya, dan submodule dengan nama salinan (`lib 2`) dilaporkan sebagai salinan.
- Index yang hilang pada linked worktree terdeteksi saat worktree itu diperiksa, bukan worktree utama.
- Jika `.git` adalah symlink ke penyimpanan di luar folder yang disinkronkan, jalankan git dan pemeriksaan dari linked worktree. Dari direktori penyimpanan, yang tidak berisi file apa pun, index yang kosong terlihat seperti "hapus semuanya" yang disengaja dan tidak memblokir commit.
- Beberapa commit yang disengaja memerlukan `--no-verify`: berhenti melacak file yang tetap ada di disk, menghapus semua file di tempat objek tidak dapat dibaca, penggantian nama yang hanya mengubah huruf besar-kecil atau bentuk Unicode dari file yang memiliki hard link lain, serta commit apa pun di sparse checkout (mode cone atau tidak) yang menghapus atau mengganti nama file terlacak, atau memindahkannya melalui perubahan tipe (file yang menjadi direktori, direktori yang menjadi symlink atau submodule). Di sana, pola sparse git menentukan file mana yang ada di disk, sehingga penghapusan yang disengaja tidak bisa dibedakan dari index yang hilang; pola tersebut tidak dibaca.
- Jika klien sinkronisasi juga kehilangan ref cabang itu sendiri bersama index (file loose-nya terhapus sebelum `packed-refs` baru tiba), HEAD terlihat seperti cabang baru dan commit tidak diblokir: commit itu memulai riwayat baru hanya dengan file yang di-stage. Tidak ada yang dihapus dari disk, dan riwayat lama tetap ada di reflog (`git reflog`).
- Dengan split index, atau di tempat objek tidak dapat dibaca (partial clone dengan git lebih lama dari 2.45, penyimpanan alternate yang hanya di cloud), index tidak dibandingkan dengan HEAD file demi file: hanya index yang hilang atau kosong yang terdeteksi.
- Dengan `core.splitIndex`, index yang tidak mencantumkan file apa pun tidak dikenali (header-nya hanya menghitung bagian split), sehingga commit darinya tidak diblokir.
- Hook: hanya hook commit yang ditemukan dari proses git yang menjalankannya. Hook lain (`pre-push`, `pre-applypatch`) harus menjalankan pemeriksaan dari worktree, atau ditolak (kode 2). Di tempat proses git tidak terlihat (klien yang menjalankan hook sendiri, atau penelusuran proses yang diblokir), pemeriksaan menggunakan direktori saat ini: maka, jika direktori git tidak menunjuk kembali ke worktree (`--separate-git-dir`, `.git.nosync`, `--work-tree`), index tanpa file selalu memblokir commit (gunakan `--no-verify` untuk pengosongan yang disengaja), direktori saat ini yang tidak berisi file HEAD tidak diperiksa sebagai worktree (`WORKTREE_GUESSED`), dan hook yang berpindah ke repositori atau direktori git lain ditolak (kode 2). Hook penerimaan (`pre-receive`) bukan batas pertahanan yang didukung.
- `move-out`: tidak semua alat memahami file `.git`. Git dan alat yang menjalankan git berfungsi; isomorphic-git (Obsidian Git di ponsel) dan alat yang membuka `.git` sebagai direktori tidak.
- `move-out`: mengganti nama atau memindahkan folder yang disinkronkan merusak file `.git` relatif. `check` kemudian menyebut direktori yang hilang; kembalikan folder ke tempatnya, atau arahkan file `.git` ke direktori git secara manual.
- `move-out` mengharuskan folder yang disinkronkan berada di tempat yang sama di dalam direktori home di setiap perangkat (iCloud Drive selalu begitu; Dropbox bisa `~/Dropbox` di satu perangkat dan `~/Library/CloudStorage/Dropbox` di perangkat lain). Jika tidak, file `.git` di sana menunjuk ke tempat lain dan `--attach` menolak (`TARGET_UNEXPECTED`).
- Direktori git yang dipindahkan berada di sistem file direktori home; tidak ada opsi untuk menaruhnya di tempat lain.
- `move-out --prepare` menyalin riwayat perangkat pada saat itu: lakukan pemindahan segera setelah persiapan, tanpa commit baru di antaranya.
- `move-out` dan `fix` telah dicoba di folder iCloud Drive sendiri, bukan di folder iCloud milik aplikasi (`iCloud~md~obsidian` milik Obsidian).

## Berkontribusi

Lihat [CONTRIBUTING.md](../../CONTRIBUTING.md). Laporkan masalah keamanan seperti dijelaskan di [SECURITY.md](../../SECURITY.md), bukan di issue publik.

## Lisensi

MIT
