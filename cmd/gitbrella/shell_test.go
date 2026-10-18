package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// How the hook reads a shell command: each git command it finds, the folder
// it runs in, and whether the reading followed it. Quotes, escapes, subshells,
// heredocs, redirections, cd and pushd, env and git's own options are
// followed; git anywhere else is "unknown" (a write, for the hook).
func TestReadShell(t *testing.T) {
	// every folder exists but those named "missing": a cd elsewhere succeeds
	defer func(f func(string) bool) { isDir = f }(isDir)
	isDir = func(p string) bool { return !strings.Contains(p, "missing") }
	for cmd, want := range map[string]string{ // runs as "dir:subcommand", unknown as "dir:?"
		"git commit -m x":                                                    "/w:commit",
		"git -C inner commit -m x":                                           "/w/inner:commit",
		"git -C a -C b status":                                               "/w/a/b:status",
		"git -C /abs/repo log":                                               "/abs/repo:log",
		"cd sub && git commit":                                               "/w/sub:commit",
		"cd sub; git -C x add . && git log":                                  "/w/sub/x:add /w/sub:log",
		"cd $HOME && git commit":                                             ":commit",
		"(cd sound && git pull) && (cd dmg && git pull)":                     "/w/sound:pull /w/dmg:pull",
		"pushd ../dmg && git commit":                                         "/dmg:commit",
		"pushd a && pushd b && popd && git status":                           "/w/a:status",
		"cd dmg >/dev/null && git commit":                                    "/w/dmg:commit",
		"cd dmg 2>/dev/null || exit 1; git commit":                           "/w/dmg:commit",
		"cd -P dmg && git commit":                                            "/w/dmg:commit",
		"X=$(cd ../dmg && pwd); git commit":                                  "/w:commit",
		"(cd ../dmg && ls); git commit":                                      "/w:commit",
		`cd "Projects (old)/dmg3" && git commit`:                             "/w/Projects (old)/dmg3:commit",
		"cd 'a&b/dmg4' && git commit":                                        "/w/a&b/dmg4:commit",
		`cd with\ space/dmg2 && git commit`:                                  "/w/with space/dmg2:commit",
		"git commit -m 'wip; cd ../dmg && git push'":                         "/w:commit",
		"git commit -F - <<EOF\nmsg\ncd ../dmg && git push\nEOF\ngit status": "/w:commit /w:status",
		"git commit -F - <<-'END'\n\tcd x && git push\n\tEND\n":              "/w:commit",
		"GIT_DIR=../dmg/.git GIT_WORK_TREE=../dmg git commit":                "/dmg:commit",
		"git --git-dir=../dmg/.git commit":                                   "/dmg:commit",
		"git --git-dir ../dmg/.git --work-tree=../dmg commit":                "/dmg:commit",
		"env -C ../dmg git commit":                                           "/dmg:commit",
		"env -i GIT_DIR=/r/.git git log":                                     "/r:log",
		"bash -lc 'cd x && git commit'":                                      "/w/x:commit",
		"if ! git diff --quiet; then git commit -am x; fi":                   "/w:diff /w:commit",
		`for f in a.txt; do git add "$f"; done`:                              "/w:add",
		"! git commit -m x":                                                  "/w:commit",
		"time git push":                                                      "/w:push",
		"timeout 30 git commit":                                              "/w:commit",
		`\git commit`:                                                        "/w:commit",
		"sudo -u bob git status":                                             "/w:status",
		"ls | xargs git add":                                                 "/w:add",
		`echo "(cd ../dmg && git commit)" > notes.txt`:                       "",
		`echo "$(git rev-parse HEAD)"`:                                       "/w:rev-parse",
		"echo $(git rev-parse HEAD)":                                         "/w:rev-parse",
		"echo `git rev-parse HEAD`":                                          "/w:rev-parse",
		"ls && npm test":                                                     "",
		"git":                                                                "/w:--version",
		"git commit -m 'oops":                                                ":?",
		"cd .git/hooks && git status":                                        "/w/.git/hooks:status",
		"gitbrella fix --apply":                                              "",
		"/usr/bin/git -c core.pager=cat push":                                "/w:push",
		"echo git is great":                                                  "",
		"printf '%s\\n' git; which git; command -v git":                      "/w:--version",
		"git -C $(pwd)/dmg commit":                                           "/w/dmg:commit",
		"cd -P lnk/.. && git commit":                                         "/w:commit",
		"export GIT_DIR=../dmg/.git; git commit":                             "/dmg:commit",
		"GIT_DIR=/x/.git; git commit":                                        "/w:commit",
		"(export GIT_DIR=/x/.git); git commit":                               "/w:commit",
		"export GIT_DIR=/x/.git; unset GIT_DIR; git commit":                  "/w:commit",
		"sudo -u bob git -C dmg commit":                                      "/w/dmg:commit",
		"nice git -C dmg commit":                                             "/w/dmg:commit",
		// the last review (K1–K12)
		"cd $(git rev-parse --show-toplevel) && git commit -am x": "/w:rev-parse :commit",
		"git -C $(pwd) commit":                                                       "/w:commit",
		"cd dmg && git commit -am x # do not push yet":                               "/w/dmg:commit",
		"git log -5 # what's new":                                                    "/w:log",
		"# merge a << b\ncd dmg\ngit commit -am x":                                   "/w/dmg:commit",
		"git -C dmg commit -m \"$(cat <<'EOF'\nFix the user's bug (really\nEOF\n)\"": "/w/dmg:commit",
		"rm -f .git/index.lock":                                                      "/w:>git",
		"mv '.git/index 2' .git/index":                                               "/w:>git",
		"echo x > dmg/.git/HEAD":                                                     "/w/dmg:>git",
		"cat .git/HEAD":                                                              "",
		"ls -la .git && grep x .git/config":                                          "",
		"timeout 60 bash -c 'git commit -am x'":                                      "/w:commit",
		"sudo sh -c 'cd dmg && git commit'":                                          "/w/dmg:commit",
		"bash -o pipefail -c 'git push'":                                             "/w:push",
		`eval "git commit -am x"`:                                                    "/w:commit",
		"watch -n1 'git status'":                                                     "/w:status",
		"G=git; $G commit":                                                           "/w:commit",
		"for d in dmg ok; do git -C \"$d\" commit -am x; done":                       "/w/dmg:commit /w/ok:commit",
		"R=/abs/dmg; cd \"$R\" && git commit":                                        "/abs/dmg:commit",
		"ls | xargs -I{} git -C {} commit":                                           ":commit",
		"bash <<'EOF'\ngit commit -am x\nEOF":                                        "/w:commit",
		"bash <<< 'git push'":                                                        "/w:push",
		"cat <<EOF | sh\ngit commit -am x\nEOF":                                      "/w:commit",
		"python3 - <<EOF\nimport os; os.system('git commit')\nEOF":                   "/w:?",
		"cat <(git commit -am x)":                                                    "/w:commit",
		"git --git-dir=../dmg/.git --work-tree=. commit":                             "/dmg:commit /w:commit",
		"git --work-tree=../ok commit":                                               "/w:commit /ok:commit",
		"(cd dmg && case x in x) git commit;; esac)":                                 "/w/dmg:commit",
		"echo $((1<<2)); git commit":                                                 "/w:commit",
		"f(){ git commit; }; cd dmg && f":                                            "/w/dmg:commit",
		"cd ../ok\r\ngit commit":                                                     ":commit /w:commit",
		"builtin cd dmg && git commit":                                               "/w/dmg:commit",
		"command cd dmg && git commit":                                               "/w/dmg:commit",
		// the review of the reader (P1–P12)
		"for d in ok dmg; do (cd \"$d\" && git add -A && git commit -m x); done": "/w/ok:add /w/ok:commit /w/dmg:add /w/dmg:commit",
		"for d in /abs/dmg /abs/ok; do cd $d && git commit -am x; done":          "/abs/dmg:commit /abs/ok:commit",
		"git -C ../ok commit -m \"Fix parse() bug\"":                             "/ok:commit",
		"cd ../ok && git commit -m \"Refactor function names\"":                  "/ok:commit",
		"git -C dmg status && git -C ok commit -m \"fix init()\"":                "/w/dmg:status /w/ok:commit",
		"git commit -F - <<'EOF'\nadd function x() and alias y=z\nEOF":           "/w:commit",
		"cd .git && rm -f index.lock":                                            "/w:>git",
		"cd .git; echo 'ref: refs/heads/main' > HEAD":                            "/w:>git",
		"rm -f \"$(git rev-parse --git-dir)/index.lock\"":                        "/w:rev-parse /w:>git",
		"rm -f \"$(git rev-parse --git-path index.lock)\"":                       "/w:rev-parse /w:>git",
		"G=.git; echo x > $G/HEAD":                                               "/w:>git",
		"find .git -name '*.lock' | xargs rm -f":                                 "/w:>git",
		"perl -pi -e 's/a/b/' .git/config":                                       "/w:>git",
		"python3 -c \"open('.git/HEAD','w').write('x')\"":                        "/w:>git",
		"cat > /tmp/gcd-f.sh <<'X'\ngit commit -am x\nX\nbash /tmp/gcd-f.sh":     "/w:commit",
		"GIT=$(which git); $GIT commit -am x":                                    "/w:commit",
		"G=\"$(command -v git)\"; \"$G\" commit":                                 "/w:--version /w:commit",
		"X=$(foo git); $X commit":                                                "/w:? /w:?",
		"cd /abs/dmg 2>/dev/null || cd /abs/ok; git commit -am x":                "/abs/dmg:commit",
		"cd /abs/missing 2>/dev/null || cd /abs/ok; git commit -am x":            "/abs/missing:commit /abs/ok:commit",
		"test -d x && cd x; git commit":                                          "/w/x:commit /w:commit",
		"if [ -d dmg ]; then cd dmg; else cd ok; fi; git commit":                 "/w/dmg:commit /w/ok:commit",
		"case $1 in a) cd dmg;; b) cd ok;; esac; git commit":                     "/w/dmg:commit /w/ok:commit /w:commit",
		"case $1 in a) cd dmg;; *) cd ok;; esac; git commit":                     "/w/dmg:commit /w/ok:commit",
		"cd missing; git commit":                                                 "/w/missing:commit /w:commit",
		"mkdir new && cd new && git init":                                        "/w/new:init",
		"cd dmg & git commit":                                                    "/w:commit",
		"while read d; do git -C \"$d\" pull; done < list.txt":                   ":pull",
		"cp -R .git /tmp/b":                                                      "",
		"cp backup .git/index":                                                   "/w:>git",
		"tar czf b.tgz .git":                                                     "",
		"tar xzf b.tgz -C .git":                                                  "/w:>git",
		"rsync -a --exclude .git ./ /x/":                                         "",
		"fd --exclude .git":                                                      "",
		"xattr -l .git/index":                                                    "",
		"xattr -d com.apple.x .git/index":                                        "/w:>git",
		"sed -n 1,5p .git/config":                                                "",
		"sed -i '' s/a/b/ .git/config":                                           "/w:>git",
		"awk '{print}' .git/packed-refs":                                         "",
		"sort .git/packed-refs":                                                  "",
		"gh pr checkout 12":                                                      "/w:gh",
		"gh pr view 12":                                                          "",
		"git-lfs install":                                                        "/w:lfs",
		"echo x >& .git/HEAD":                                                    "/w:>git",
		"dd if=/dev/zero of=.git/index count=0":                                  "/w:>git",
		"env -S 'git commit'":                                                    "/w:commit",
		"sudo -D dmg git commit":                                                 "/w/dmg:commit",
		"cat <<EOF\n$(git commit -am x)\nEOF":                                    "/w:commit",
		"cat <<'EOF'\n$(git commit -am x)\nEOF":                                  "",
		"echo $((git commit) )":                                                  "/w:commit",
		"((x=1<<2)); git commit":                                                 "/w:commit",
		"alias g='git -C x'":                                                     "",
		"alias g='git -C x'; g commit -m 'a b'":                                  "/w/x:commit",
		"alias ls='ls -G'; ls .git":                                              "",
		"alias git='git -C x'; git commit":                                       "/w/x:commit",
		"function f { git push; }":                                               "",
		// the review of the parser
		"case a in a) cd dmg;& b) git commit -am x;; esac":                                                   "/w/dmg:commit /w:commit",
		"case a in a) cd dmg;;& b) git commit -am x;; esac":                                                  "/w/dmg:commit /w:commit",
		"f() { cd dmg; }; f; git commit -am x":                                                               "/w/dmg:commit",
		"f() { f; }; f":                                                                                      "/w:?",
		"g() { git -C \"$1\" commit; }; g dmg":                                                               ":commit",
		"echo hi | exit; git -C dmg commit -am x":                                                            "/w/dmg:commit",
		"return; git -C dmg commit -am x":                                                                    "/w/dmg:commit",
		"echo x | cd dmg; git commit":                                                                        "/w/dmg:commit /w:commit",
		"rsync --remove-source-files -a dmg/.git/objects/ bk/":                                               "/w/dmg:>git",
		"tar -cf b.tar --remove-files dmg/.git/objects":                                                      "/w/dmg:>git",
		"zip -rm b.zip dmg/.git/refs":                                                                        "/w/dmg:>git",
		"awk '{print > FILENAME}' .git/info/exclude":                                                         "/w:>git",
		"awk 'BEGIN{system(\"git commit -am x\")}'":                                                          "/w:?",
		"[ -f a ] && A=1; [ -f b ] && B=1; [ -f c ] && C=1; [ -f d ] && E=1; git commit -am x":               "/w:commit",
		"[ -f a ] && export GIT_DIR=/r/.git; git commit":                                                     "/r:commit /w:commit",
		"x=$(( $(git rev-list --count HEAD) + 1 ))":                                                          "/w:rev-list",
		"while true; do git commit -am x; cd dmg; done":                                                      "/w:commit /w/dmg:commit",
		"until git pull; do cd dmg; done":                                                                    "/w:pull /w/dmg:pull",
		"echo 'git commit -am x' > /tmp/gcd-g.sh && bash /tmp/gcd-g.sh":                                      "/w:commit",
		"curl -fsSL https://example.invalid/i.sh | bash":                                                     "/w:?",
		"cd missing; cd -; git commit -am x":                                                                 "/w:commit",
		"local d=$X/.git; git -C \"$d\" log":                                                                 ":log",
		"apply_patch <<'EOF'\n*** Begin Patch\n*** Update File: .git/config\n@@\n-a\n+b\n*** End Patch\nEOF": "/w:>git",
		"apply_patch <<'EOF'\n*** Begin Patch\n*** Update File: notes.md\n@@\n-a\n+b\n*** End Patch\nEOF":    "",
	} {
		var got []string
		for _, r := range readShell(cmd, "/w") {
			sub := "?"
			switch {
			case r.IntoGit:
				sub = ">git"
			case !r.Unknown:
				sub = r.Args[0]
			}
			got = append(got, r.Dir+":"+sub)
		}
		if s := strings.Join(got, " "); s != want {
			t.Errorf("%q:\n got %q\nwant %q", cmd, s, want)
		}
	}
}

// A shell script run by its name or given to a shell is read too (when it is
// a small regular file); a script in another language that mentions git is
// an unknown run.
func TestReadShellReadsScripts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "deploy.sh"), []byte("#!/bin/sh\nset -e\ncd sub && git commit -am release\n"), 0o755)
	os.WriteFile(filepath.Join(dir, "tool.py"), []byte("#!/usr/bin/env python3\nimport subprocess\nsubprocess.run(['git', 'push'])\n"), 0o755)
	for cmd, want := range map[string]string{
		"bash deploy.sh": filepath.Join(dir, "sub") + ":commit",
		"./deploy.sh":    filepath.Join(dir, "sub") + ":commit",
		"sh ./deploy.sh": filepath.Join(dir, "sub") + ":commit",
		"./tool.py":      dir + ":?",
		"./missing.sh":   "",
	} {
		var got []string
		for _, r := range readShell(cmd, dir) {
			sub := "?"
			if !r.Unknown {
				sub = r.Args[0]
			}
			got = append(got, r.Dir+":"+sub)
		}
		if s := strings.Join(got, " "); s != want {
			t.Errorf("%q: %q, want %q", cmd, s, want)
		}
	}
}

// A script read from a pipe, from stdin, by source or through a symlink is
// read too; one a shell is given but the reading cannot read is an unknown
// run where it runs. A script that runs itself is read once.
func TestReadShellFollowsScriptsEverywhere(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fix.sh"), []byte("cd sub && git commit -am x\n"), 0o644)
	os.Symlink(filepath.Join(dir, "fix.sh"), filepath.Join(dir, "link.sh"))
	os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\ncase \"$1\" in\nall) ./run.sh build && ./run.sh lint ;;\nbuild) git commit -am x ;;\nesac\n"), 0o755)
	os.WriteFile(filepath.Join(dir, "big.sh"), []byte(strings.Repeat("# filler\n", 8<<10)+"git commit -am x\n"), 0o644)
	sub := filepath.Join(dir, "sub") + ":commit"
	for cmd, want := range map[string]string{
		"cat fix.sh | bash":   sub,
		"bash < fix.sh":       sub,
		"source fix.sh":       sub,
		". ./fix.sh":          sub,
		"bash link.sh":        sub,
		"bash missing.sh":     "",
		"source missing.sh":   "",
		"cat missing.sh | sh": "",
		"bash big.sh":         dir + ":?",
		"cat big.sh | sh":     dir + ":?",
		"./run.sh all":        dir + ":? " + dir + ":commit",
	} {
		start := time.Now()
		var got []string
		for _, r := range readShell(cmd, dir) {
			s := "?"
			if !r.Unknown {
				s = r.Args[0]
			}
			got = append(got, r.Dir+":"+s)
		}
		if s := strings.Join(got, " "); s != want {
			t.Errorf("%q: %q, want %q", cmd, s, want)
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%q took %s", cmd, d)
		}
	}
}

// rm -r and find -delete on a folder holding a .git write inside it.
func TestReadShellFindsWritesUnderAGitDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "repo", ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, "plain"), 0o755)
	repo := filepath.Join(dir, "repo")
	for cmd, want := range map[string]string{
		"cd repo && find . -name '* 2' -delete":      repo + ":>git",
		"find repo -name '* 2' -exec rm {} \\;":      repo + ":>git",
		"find repo -name '* 2' -exec grep -l x {} +": "",
		"rm -rf repo":       repo + ":>git",
		"rm -rf plain":      "",
		"rm repo/notes.txt": "",
		"cd repo && find . -name node_modules -type d | xargs rm -rf": "",
		"cd repo && find .git -name '*.lock' | xargs rm -f":           repo + ":>git",
	} {
		var got []string
		for _, r := range readShell(cmd, dir) {
			s := "?"
			switch {
			case r.IntoGit:
				s = ">git"
			case !r.Unknown:
				s = r.Args[0]
			}
			got = append(got, r.Dir+":"+s)
		}
		if s := strings.Join(got, " "); s != want {
			t.Errorf("%q: %q, want %q", cmd, s, want)
		}
	}
}

// A reading that would go on too long stops, quickly, with an unknown run.
func TestReadShellStopsInTime(t *testing.T) {
	var vals []string
	for i := 0; i < 60; i++ {
		vals = append(vals, strconv.Itoa(i))
	}
	v := strings.Join(vals, " ")
	cmd := "for a in " + v + "; do for b in " + v + "; do for c in " + v + "; do git -C $a/$b/$c status; done; done; done"
	start := time.Now()
	runs := readShell(cmd, "/w")
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %s", d)
	}
	// loops around nothing but compound commands count too
	empty := "for a in " + v + "; do for b in " + v + "; do for c in " + v + "; do for d in " + v + "; do ( : ); done; done; done; done"
	start = time.Now()
	readShell(empty, "/w")
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("loops around ( : ) took %s", d)
	}
	if len(runs) == 0 || !runs[len(runs)-1].Unknown || runs[len(runs)-1].Dir != "" {
		t.Fatalf("no unknown run at the end: %d runs", len(runs))
	}
}

// git -C takes ~ as the home folder, then follows .. from there.
func TestPhysicalHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	real, _ := filepath.EvalSymlinks(home)
	if got, want := physical("/elsewhere", "~/../x"), filepath.Join(filepath.Dir(real), "x"); got != want {
		t.Fatalf("%q, want %q", got, want)
	}
}

// The reading never panics and never takes long, whatever the command.
func FuzzReadShell(f *testing.F) {
	for _, s := range []string{"git commit -m x", "cd a && git -C b commit", "git commit -m \"$(cat <<'EOF'\nx\nEOF\n)\"",
		"for d in a b; do (cd $d && git push); done", "case x in a) git x;; esac", "echo `a \\`b\\``", "bash -c 'eval \"git x\"'",
		"x=$(( 1 << 2 )); cat <<-E\n\tE\n", "echo $(echo $(echo $(git x)))", "a <(b) >(c) 2>&1 &>> .git/x",
		"if a; then cd b; elif c; then cd d; else cd e; fi; git x", "f(){ git x; }; while f; do cd ..; done", "((x=1<<2)) || cd a"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		start := time.Now()
		readShell(s, "/w")
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("slow: %v for %d bytes", d, len(s))
		}
	})
}
