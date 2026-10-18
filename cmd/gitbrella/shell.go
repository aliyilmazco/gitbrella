package main

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/aliyilmazco/gitbrella/internal/shq"
)

// The hook's reading of a shell command. It is rough, not a shell, and it
// leans one way: reading precisely may only let a call through, never let a
// git write slip by. Where the word git appears in a place the reading does
// not follow, the call counts as running a git command that writes; where the
// reading cannot tell the folder a git command runs in, the command counts in
// every folder checked and the call gets a notice.
//
// The reading follows the ways a command may go: after "cd x || cd y", an if,
// a case or a cd that may fail, each folder the shell may be in is read on
// (up to maxStates), and a for loop is read once per value.

// gitRun is one git command found in a shell command, or a write into a git
// directory by another program.
type gitRun struct {
	Dir     string   // the folder git works on; "" when the reading cannot tell
	Args    []string // the subcommand and its arguments; nil for an unknown run
	Unknown bool     // git appears where the reading does not follow it
	IntoGit bool     // another program writes inside Dir's git directory
}

const (
	tWord  = iota
	tSep   // ; ;; && || & newline, in text
	tPipe  // |
	tOpen  // ( : a subshell
	tClose // )
)

type token struct {
	kind   int
	text   string   // a word, quotes and escapes removed; a separator as written
	subst  []string // command and process substitutions in the word, as written
	redir  string   // for a word that is a redirection's target: its operator
	stdin  []string // heredoc bodies and here-strings a command reads (on its first word)
	quoted bool     // the word had quotes
}

// gitWordRe finds the word git in a text, not gitbrella or digital.
var gitWordRe = regexp.MustCompile(`(^|[^A-Za-z0-9_.-])git($|[^A-Za-z0-9_-])`)

// gitPathRe finds a path with a .git component.
var gitPathRe = regexp.MustCompile(`(?i)(^|/)\.git(/|$)`)

// gitLooseRe finds .git in a program's text: open('.git/HEAD').
var gitLooseRe = regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_.-])\.git($|[^A-Za-z0-9_-])`)

// shebangShellRe finds a shell in a script's #! line.
var shebangShellRe = regexp.MustCompile(`\b(ba|z|k|da)?sh\b`)

// maxShellLen is the longest command read; a longer one is one unknown run.
const maxShellLen = 64 << 10

const (
	maxSteps  = 5000 // commands one reading reads, scripts and loops included
	maxStates = 8    // ways the shell may be at one command
	maxValues = 64   // values of a for loop read one by one
)

// unreadable stands, in what a command writes to a pipe, for a file the
// reading could not read.
const unreadable = "\x00unreadable"

// shellKeywords start a command without being one.
var shellKeywords = []string{"if", "then", "else", "elif", "fi", "do", "while", "until", "!", "{", "}"}

// readers are programs that only read the files they are given: a .git path
// among their arguments writes nothing there.
var readers = []string{"cat", "less", "more", "head", "tail", "ls", "stat", "file", "wc", "du", "grep", "egrep", "fgrep",
	"rg", "ag", "diff", "cmp", "readlink", "realpath", "basename", "dirname", "echo", "printf", "test", "[", "od", "xxd",
	"hexdump", "strings", "md5", "md5sum", "shasum", "sha1sum", "sha256sum", "open", "tree", "jq", "gitbrella", "true", ":",
	"cut", "tr", "column", "nl", "paste", "comm", "fmt", "fold", "rev", "tac", "bat", "batcat", "[["}

// dataPrograms take git as data, not as a program to run: printf git, which git.
var dataPrograms = []string{"which", "type", "whereis", "man", "help", "apropos", "info", "tldr", "hash"}

var shells = []string{"bash", "sh", "zsh", "dash", "ksh"}

var interpreters = []string{"python", "python3", "perl", "ruby", "node", "osascript", "php", "lua", "deno"}

// reader is one reading of a shell command and the scripts it runs.
type reader struct {
	steps    int                // commands read
	over     bool               // past maxSteps: the rest is not read
	mentions bool               // a text read mentions git or .git
	scripts  []string           // the script files being read
	written  map[string]string  // files the command writes (a heredoc, echo), by path
	pipeNext bool               // the command being read writes to a pipe or a file
	funcs    map[string]funcDef // functions defined, read where they are called
	aliases  map[string]string  // aliases defined, read where they are used
	calls    []string           // the functions and aliases being read
}

// funcDef is a function's body, as tokens.
type funcDef struct {
	toks     []token
	mentions bool
}

// readShell finds the git commands of a shell command started in cwd, in
// order: the folder each runs in follows cd, pushd and popd (not out of a
// subshell), git's -C (composed), --git-dir and --work-tree, GIT_DIR and
// GIT_WORK_TREE, env -C, variables and for lists the command sets, and the
// scripts a shell is given (-c, eval, source, a file, a heredoc, a pipe). A
// quote left open makes the whole command one unknown run if it mentions git.
func readShell(command, cwd string) []gitRun {
	rd := &reader{written: map[string]string{}, funcs: map[string]funcDef{}, aliases: map[string]string{}}
	st := &shellState{cur: cwd, vars: map[string]string{}, exported: map[string]string{}, rd: rd}
	runs, _ := rd.read(command, []*shellState{st}, 0)
	if rd.over && rd.mentions {
		runs = append(runs, gitRun{Unknown: true})
	}
	return runs
}

// read reads a shell text from the states in: the git runs, and the states
// it leaves.
func (rd *reader) read(text string, in []*shellState, depth int) ([]gitRun, []*shellState) {
	mentions := gitWordRe.MatchString(text) || gitPathRe.MatchString(text)
	rd.mentions = rd.mentions || mentions
	rd.steps += 1 + len(text)/4096
	if rd.steps > maxSteps {
		rd.over = true
	}
	if len(text) > maxShellLen || depth > 16 || rd.over {
		if mentions {
			return []gitRun{{Unknown: true}}, in
		}
		return nil, in
	}
	toks, _, ok := tokenize(text, 0, false)
	if !ok {
		if mentions {
			return []gitRun{{Unknown: true}}, in
		}
		return nil, in
	}
	p := &parser{rd: rd, toks: toks, depth: depth, mentions: mentions}
	states := in
	for {
		_, _, states = p.list(states, nil)
		if p.i >= len(p.toks) || rd.over {
			break
		}
		p.i++ // a token no list takes (a stray ")" or "fi"): go on after it
	}
	return p.runs, states
}

// parser reads the tokens of one text.
type parser struct {
	rd       *reader
	toks     []token
	i        int
	depth    int
	mentions bool // the text mentions git
	runs     []gitRun
}

func (p *parser) next() (token, bool) {
	if p.i < len(p.toks) {
		return p.toks[p.i], true
	}
	return token{}, false
}

// at reports whether the next token is an unquoted word among words.
func (p *parser) at(words ...string) bool {
	t, ok := p.next()
	return ok && t.kind == tWord && t.redir == "" && !t.quoted && slices.Contains(words, t.text)
}

// atSep reports whether the next token is a separator among texts.
func (p *parser) atSep(texts ...string) bool {
	t, ok := p.next()
	return ok && t.kind == tSep && slices.Contains(texts, t.text)
}

func (p *parser) skip(texts ...string) {
	for p.atSep(texts...) {
		p.i++
	}
}

// list reads and-or lists up to a word among stops, a ")", ";;" or the end:
// the states the last one succeeds and fails in, and the states after them.
func (p *parser) list(in []*shellState, stops []string) (ok, fail, after []*shellState) {
	ok, after = in, in
	for {
		p.skip(";", "\n", "&")
		t, more := p.next()
		if !more || p.rd.over || t.kind == tClose || p.atSep(";;", ";&", ";;&") || p.at(stops...) {
			return ok, fail, after
		}
		start := p.i
		s, f := p.andor(copies(after))
		if p.atSep("&") { // in the background: the shell goes on where it was
			ok, fail = copies(after), nil
		} else {
			ok, fail = s, f
			after = merge(s, f)
		}
		if p.i == start {
			p.i++ // never stall
		}
	}
}

// andor reads pipelines joined by && and ||: a pipeline after && runs where
// the one before succeeded, after || where it failed.
func (p *parser) andor(in []*shellState) (ok, fail []*shellState) {
	ok, fail = p.pipeline(in)
	for p.atSep("&&", "||") {
		op := p.toks[p.i].text
		p.i++
		p.skip("\n")
		if op == "&&" {
			s, f := p.pipeline(ok)
			ok, fail = s, merge(fail, f)
		} else {
			s, f := p.pipeline(fail)
			ok, fail = merge(ok, s), f
		}
	}
	return ok, fail
}

// pipeline reads commands joined by |. Each but the last runs apart from the
// shell; what one writes is what the next reads.
func (p *parser) pipeline(in []*shellState) (ok, fail []*shellState) {
	neg := false
	for p.at("!") {
		neg = !neg
		p.i++
	}
	var piped []string
	piping := false
	for {
		s, f, writes := p.command(copies(in), piped)
		if t, more := p.next(); more && t.kind == tPipe {
			p.i++
			p.skip("\n")
			piped = writes
			if piped == nil { // what the command writes the reading does not know
				piped = []string{unreadable}
			}
			piping = true
			continue
		}
		if piping { // bash runs the last command apart from the shell too, zsh in it
			s, f = merge(s, copies(in)), merge(f, copies(in))
		}
		if neg {
			return f, s
		}
		return s, f
	}
}

// command reads one command, simple or compound.
func (p *parser) command(in []*shellState, piped []string) (ok, fail []*shellState, writes []string) {
	if p.rd.steps += max(1, len(in)); p.rd.steps > maxSteps {
		p.rd.over = true
	}
	t, more := p.next()
	switch {
	case !more:
		return in, nil, nil
	case t.kind == tOpen: // a subshell: what it does to the shell stays in it
		p.i++
		p.list(copies(in), nil)
		if t, more := p.next(); more && t.kind == tClose {
			p.i++
		}
		p.redirs(in)
		return in, copies(in), nil
	case p.at("{"):
		p.i++
		s, f, after := p.list(in, []string{"}"})
		if p.at("}") {
			p.i++
		}
		p.redirs(after)
		return s, f, nil
	case p.at("if"):
		return p.ifClause(in)
	case p.at("while", "until"):
		return p.whileLoop(in)
	case p.at("for", "select"):
		return p.forLoop(in)
	case p.at("case"):
		return p.caseClause(in)
	case p.at("function"):
		p.i++
		name := ""
		if t, more := p.next(); more && t.kind == tWord {
			name = t.text
			p.i++
		}
		if p.i+1 < len(p.toks) && p.toks[p.i].kind == tOpen && p.toks[p.i+1].kind == tClose {
			p.i += 2
		}
		p.define(name)
		return in, nil, nil
	case p.at("then", "else", "elif", "fi", "do", "done", "esac", "}"):
		p.i++ // a keyword out of place: read on
		return in, nil, nil
	}
	return p.simple(in, piped)
}

// define keeps a function's body: nothing runs where it is defined; it is
// read where it is called, in the shell that calls it.
func (p *parser) define(name string) {
	p.skip("\n")
	from := p.i
	p.command(nil, nil) // to the end of the body
	if name != "" {
		p.rd.funcs[name] = funcDef{slices.Clone(p.toks[from:p.i]), p.mentions}
	}
}

// call reads a function or an alias where it is used, in the shell st. One
// already being read (it calls itself) is an unknown run here.
func (p *parser) call(key string, st *shellState, read func() ([]gitRun, []*shellState, []*shellState)) (ok, fail []*shellState) {
	if slices.Contains(p.rd.calls, key) || p.depth > 16 {
		p.runs = append(p.runs, gitRun{Dir: st.cur, Unknown: true})
		return []*shellState{st}, []*shellState{st.copyPtr()}
	}
	p.rd.calls = append(p.rd.calls, key)
	defer func() { p.rd.calls = p.rd.calls[:len(p.rd.calls)-1] }()
	runs, s, f := read()
	p.runs = append(p.runs, runs...)
	return s, f
}

// simple reads a simple command in each state.
func (p *parser) simple(in []*shellState, piped []string) (ok, fail []*shellState, writes []string) {
	var toks []token
	for t, more := p.next(); more && t.kind == tWord; t, more = p.next() {
		toks = append(toks, t)
		p.i++
	}
	if len(toks) == 1 && !toks[0].quoted && toks[0].redir == "" && p.i+1 < len(p.toks) && p.toks[p.i].kind == tOpen && p.toks[p.i+1].kind == tClose {
		p.i += 2 // name(): a function
		p.define(toks[0].text)
		return in, nil, nil
	}
	if len(toks) == 0 {
		return in, nil, nil
	}
	nt, more := p.next()
	pipeNext := more && nt.kind == tPipe
	start := len(p.runs)
	defer func() { p.runs = append(p.runs[:start], once(p.runs[start:])...) }() // states that agree: one run
	for _, st := range in {
		if p.rd.steps++; p.rd.steps > maxSteps {
			p.rd.over = true
			return nil, nil, nil
		}
		for _, t := range toks { // substitutions run where the command runs
			for _, sub := range t.subst {
				r, _ := p.rd.read(sub, []*shellState{st.copyPtr()}, p.depth+1)
				p.runs = append(p.runs, r...)
			}
		}
		if p.mentions && st.unresolved(toks) {
			// the program is a word the reading cannot expand ($G, $(...))
			p.runs = append(p.runs, gitRun{Dir: st.cur, Unknown: true})
		}
		if name, rest := st.program(toks); name != "" {
			if def, found := p.rd.funcs[name]; found {
				s, f := p.call("f:"+name, st, func() ([]gitRun, []*shellState, []*shellState) {
					sub := &parser{rd: p.rd, toks: def.toks, depth: p.depth + 1, mentions: def.mentions}
					s, f, _ := sub.command([]*shellState{st}, piped)
					return sub.runs, s, f
				})
				ok, fail = append(ok, s...), append(fail, f...)
				continue
			}
			// an alias is not expanded again inside its own expansion
			if text, isAlias := p.rd.aliases[name]; isAlias && !slices.Contains(p.rd.calls, "a:"+name) {
				s, f := p.call("a:"+name, st, func() ([]gitRun, []*shellState, []*shellState) {
					for _, a := range rest {
						text += " " + shq.Quote(a)
					}
					runs, outs := p.rd.read(text, []*shellState{st}, p.depth+1)
					st.adopt(outs)
					return runs, []*shellState{st}, []*shellState{st.copyPtr()}
				})
				ok, fail = append(ok, s...), append(fail, f...)
				continue
			}
		}
		p.rd.pipeNext = pipeNext
		r, w, oc := st.command(toks, piped, p.depth)
		p.runs = append(p.runs, r...)
		if w != nil && writes == nil { // known to write nothing, not unknown
			writes = []string{}
		}
		writes = append(writes, w...)
		switch {
		case oc.ok && oc.fail && oc.failSt != nil:
			ok, fail = append(ok, st), append(fail, oc.failSt)
		case oc.ok && oc.fail:
			ok, fail = append(ok, st), append(fail, st.copyPtr())
		case oc.ok:
			ok = append(ok, st)
		case oc.fail:
			fail = append(fail, st)
		}
	}
	return ok, fail, writes
}

// once drops runs the same as an earlier one.
func once(runs []gitRun) []gitRun {
	var out []gitRun
	seen := map[string]bool{}
	for _, r := range runs {
		k := fmt.Sprint(r.Dir, "\x00", r.Args, r.Unknown, r.IntoGit)
		if !seen[k] {
			seen[k] = true
			out = append(out, r)
		}
	}
	return out
}

// ifClause reads if ... then ... elif ... else ... fi: a branch runs where
// its condition succeeded, the next where it failed.
func (p *parser) ifClause(in []*shellState) (ok, fail []*shellState, writes []string) {
	p.i++
	var ends []*shellState
	rest := in
	for {
		s, f, _ := p.list(rest, []string{"then"})
		if p.at("then") {
			p.i++
		}
		_, _, after := p.list(s, []string{"elif", "else", "fi"})
		ends, rest = append(ends, after...), f
		if !p.at("elif") {
			break
		}
		p.i++
	}
	if p.at("else") {
		p.i++
		_, _, after := p.list(rest, []string{"fi"})
		ends = append(ends, after...)
	} else {
		ends = append(ends, rest...)
	}
	if p.at("fi") {
		p.i++
	}
	ends = merge(ends)
	p.redirs(ends)
	return ends, copies(ends), nil
}

// whileLoop reads while/until ... do ... done: the body may run or not; one
// that moves the shell moves it again on the next round, to a folder the
// reading cannot tell.
func (p *parser) whileLoop(in []*shellState) (ok, fail []*shellState, writes []string) {
	p.i++
	condAt := p.i
	round := func(from []*shellState) (exits, after []*shellState) {
		p.i = condAt
		s, f, _ := p.list(from, []string{"do"})
		if p.at("do") {
			p.i++
		}
		cond := merge(s, f)
		exits = copies(cond)
		_, _, after = p.list(cond, []string{"done"})
		return exits, after
	}
	exits, after := round(in)
	ends := merge(exits, after)
	if len(after) > 0 && moved(exits, after) {
		// the next round starts where this one left the shell
		exits2, after2 := round(copies(after))
		ends = merge(ends, exits2, after2)
		if moved(merge(copies(exits), copies(exits2)), after2) {
			u := ends[0].copyPtr()
			u.cur = ""
			ends = merge(ends, []*shellState{u})
		}
	}
	if p.at("done") {
		p.i++
	}
	p.redirs(ends)
	return ends, copies(ends), nil
}

// forLoop reads for x in ...; do ...; done once per value (a value the
// reading cannot tell stands for every folder), one after the other as the
// shell runs them.
func (p *parser) forLoop(in []*shellState) (ok, fail []*shellState, writes []string) {
	p.i++
	name := ""
	if t, more := p.next(); more && t.kind == tWord {
		name = t.text
		p.i++
	}
	if strings.HasPrefix(name, "((") { // for ((...)): no variable the reading follows
		name = ""
	}
	var vals []token
	listed := false
	if p.at("in") {
		p.i++
		listed = true
		for t, more := p.next(); more && t.kind == tWord && t.redir == ""; t, more = p.next() {
			vals = append(vals, t)
			p.i++
		}
	}
	p.skip(";", "\n")
	if p.at("do") {
		p.i++
	}
	body := p.i
	var ends []*shellState
	for _, st := range in {
		for _, t := range vals {
			for _, sub := range t.subst {
				r, _ := p.rd.read(sub, []*shellState{st.copyPtr()}, p.depth+1)
				p.runs = append(p.runs, r...)
			}
		}
		values := st.values(vals, listed)
		if len(values) > maxValues {
			values = []string{"$?"}
		}
		states := []*shellState{st}
		for _, v := range values {
			if p.rd.steps++; p.rd.steps > maxSteps {
				p.rd.over = true
			}
			if p.rd.over {
				break
			}
			for _, s := range states {
				if name != "" {
					s.vars[name] = v
				}
			}
			p.i = body
			_, _, states = p.list(states, []string{"done"})
		}
		ends = append(ends, states...)
	}
	p.i = body
	p.list(nil, []string{"done"}) // to the end of the body
	if p.at("done") {
		p.i++
	}
	ends = merge(ends)
	p.redirs(ends)
	return ends, copies(ends), nil
}

// caseClause reads case ... in pattern) ...;; esac: each arm may run; with
// no * arm, none may.
func (p *parser) caseClause(in []*shellState) (ok, fail []*shellState, writes []string) {
	p.i++
	if t, more := p.next(); more && t.kind == tWord {
		for _, st := range in {
			for _, sub := range t.subst {
				r, _ := p.rd.read(sub, []*shellState{st.copyPtr()}, p.depth+1)
				p.runs = append(p.runs, r...)
			}
		}
		p.i++
	}
	if p.at("in") {
		p.i++
	}
	var ends, carry []*shellState
	star := false
	for {
		p.skip("\n", ";")
		if _, more := p.next(); !more || p.at("esac") {
			break
		}
		if t, _ := p.next(); t.kind == tOpen {
			p.i++
		}
		for t, more := p.next(); more && (t.kind == tWord && t.redir == "" || t.kind == tPipe); t, more = p.next() {
			star = star || t.kind == tWord && t.text == "*" && !t.quoted
			p.i++
		}
		if t, more := p.next(); !more || t.kind != tClose {
			break // no arm the reading can tell
		}
		p.i++
		_, _, after := p.list(merge(carry, copies(in)), []string{"esac"})
		ends, carry = append(ends, after...), nil
		switch {
		case p.atSep(";&", ";;&"): // the next arm runs on from here
			carry = copies(after)
			p.i++
		case p.atSep(";;"):
			p.i++
		}
	}
	if p.at("esac") {
		p.i++
	}
	if !star {
		ends = append(ends, in...)
	}
	ends = merge(ends)
	p.redirs(ends)
	return ends, copies(ends), nil
}

// redirs reads the redirections after a compound command.
func (p *parser) redirs(states []*shellState) {
	for t, more := p.next(); more && t.kind == tWord && t.redir != ""; t, more = p.next() {
		p.i++
		if !writesFile(t) {
			continue
		}
		for _, st := range states {
			if r, ok := st.inGitPath(st.substitute(t)); ok {
				p.runs = append(p.runs, r)
			}
		}
	}
}

// moved reports whether a state in after is in a folder no state in before is.
func moved(before, after []*shellState) bool {
	for _, a := range after {
		if !slices.ContainsFunc(before, func(b *shellState) bool { return b.cur == a.cur }) {
			return true
		}
	}
	return false
}

func copies(states []*shellState) []*shellState {
	out := make([]*shellState, len(states))
	for i, st := range states {
		out[i] = st.copyPtr()
	}
	return out
}

// merge joins sets of states. States in the same place (folder, and the
// GIT_DIR and GIT_WORK_TREE git would see) become one, keeping the variables
// they agree on; past maxStates places the rest become one in a folder the
// reading cannot tell.
func merge(sets ...[]*shellState) []*shellState {
	var out []*shellState
	at := map[string]int{}
	joined := map[int]bool{}
	for _, set := range sets {
		for _, st := range set {
			k := st.place()
			i, ok := at[k]
			if !ok {
				at[k] = len(out)
				out = append(out, st)
				continue
			}
			if !joined[i] { // a copy: the state may be another list's too
				out[i], joined[i] = out[i].copyPtr(), true
			}
			out[i].keepCommon(st)
		}
	}
	if len(out) > maxStates {
		u := out[0].copyPtr()
		u.cur, u.prev, u.pushed = "", "", nil
		out = append(out[:maxStates-1], u)
	}
	return out
}

// shellState is where the commands read so far left the shell.
type shellState struct {
	cur, prev string            // the folder now and before the last cd ("" unknown)
	pushed    []string          // pushd's stack
	vars      map[string]string // shell variables set
	exported  map[string]string // and those exported, which git sees
	rd        *reader
}

func (sh *shellState) copy() shellState {
	c := *sh
	c.pushed = slices.Clone(sh.pushed)
	c.vars, c.exported = maps.Clone(sh.vars), maps.Clone(sh.exported)
	if c.vars == nil {
		c.vars = map[string]string{}
	}
	if c.exported == nil {
		c.exported = map[string]string{}
	}
	return c
}

func (sh *shellState) copyPtr() *shellState {
	c := sh.copy()
	return &c
}

// place is what tells two states apart for git: the folder, cd's and
// pushd's memory, and the GIT_DIR and GIT_WORK_TREE set.
func (sh *shellState) place() string {
	return strings.Join([]string{sh.cur, sh.prev, strings.Join(sh.pushed, "\x01"),
		sh.exported["GIT_DIR"], sh.exported["GIT_WORK_TREE"], sh.vars["GIT_DIR"], sh.vars["GIT_WORK_TREE"]}, "\x00")
}

// keepCommon drops the variables o does not share.
func (sh *shellState) keepCommon(o *shellState) {
	for _, m := range [][2]map[string]string{{sh.vars, o.vars}, {sh.exported, o.exported}} {
		for k, v := range m[0] {
			if w, ok := m[1][k]; !ok || w != v {
				delete(m[0], k)
			}
		}
	}
}

// program is the name of the program a command runs, and its words after it.
func (sh *shellState) program(toks []token) (string, []string) {
	var words []string
	for _, t := range toks {
		if t.redir == "" {
			words = append(words, sh.substitute(t))
		}
	}
	for i, w := range words {
		if isAssignment(w) || slices.Contains(shellKeywords, w) {
			continue
		}
		if x, ok := sh.expand(w); ok {
			rest := words[i+1:]
			for j, a := range rest {
				if y, ok := sh.expand(a); ok {
					rest[j] = y
				}
			}
			return x, rest
		}
		return "", nil
	}
	return "", nil
}

// adopt takes the states a script run in this shell left (eval, source): its
// folder if they agree, else one the reading cannot tell.
func (sh *shellState) adopt(outs []*shellState) {
	if len(outs) == 0 {
		return
	}
	*sh = outs[0].copy()
	for _, o := range outs[1:] {
		if o.cur != sh.cur {
			sh.cur = ""
		}
	}
}

// outcome is how a command may end: it may succeed, it may fail, and a
// failure may leave the shell elsewhere than a success (a cd that may fail).
type outcome struct {
	ok, fail bool
	failSt   *shellState
}

var either = outcome{ok: true, fail: true}

var varRe = regexp.MustCompile(`\$(\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*)`)

// expand replaces the variables of w the reading knows; ok is false when a $
// is left (an unknown variable, a substitution).
func (sh *shellState) expand(w string) (string, bool) {
	if !strings.Contains(w, "$") {
		return w, true
	}
	known := true
	out := varRe.ReplaceAllStringFunc(w, func(m string) string {
		name := strings.Trim(m[1:], "{}")
		if v, ok := sh.vars[name]; ok {
			return v
		}
		if v, ok := sh.exported[name]; ok {
			return v
		}
		known = false
		return m
	})
	if !known || strings.Contains(out, "$") {
		return w, false
	}
	return out, true
}

// values are the words of a for list, "$?" for one the reading cannot tell.
func (sh *shellState) values(toks []token, listed bool) []string {
	if !listed {
		return []string{"$?"} // "$@": the arguments
	}
	var out []string
	for _, t := range toks {
		x, ok := sh.expand(sh.substitute(t))
		switch {
		case !ok || !t.quoted && strings.ContainsAny(x, "*?["):
			out = append(out, "$?")
		case t.quoted:
			out = append(out, x)
		default:
			out = append(out, strings.Fields(x)...)
		}
	}
	return out
}

// unresolved reports whether the program of a command is a word the reading
// cannot expand.
func (sh *shellState) unresolved(toks []token) bool {
	for _, t := range toks {
		if t.redir != "" || isAssignment(t.text) || !t.quoted && (slices.Contains(shellKeywords, t.text) || t.text == "time") {
			continue
		}
		x, ok := sh.expand(sh.substitute(t))
		return !ok || strings.Contains(x, "$")
	}
	return false
}

// resolve is d taken from base, or "" when the shell would have to expand it.
func resolve(base, d string) string {
	for i := 0; i < len(d); i++ {
		if d[i] < 0x20 { // a control character (a \r): no folder git would find
			return ""
		}
	}
	if strings.HasPrefix(d, "~") {
		home, err := os.UserHomeDir()
		if err != nil || (d != "~" && !strings.HasPrefix(d, "~/")) {
			return ""
		}
		d = filepath.Join(home, strings.TrimPrefix(d, "~"))
	}
	if d == "" || strings.ContainsAny(d, "$`*?[{") {
		return ""
	}
	if filepath.IsAbs(d) {
		return filepath.Clean(d)
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, d)
}

func isAssignment(w string) bool {
	k, _, ok := strings.Cut(w, "=")
	if !ok || k == "" {
		return false
	}
	for i, c := range k {
		if !(c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// writesFile reports whether a redirection writes the file it names: >, >>,
// &>, and >& with a file rather than a descriptor.
func writesFile(t token) bool {
	switch t.redir {
	case ">", "&>":
		return true
	case ">&":
		return t.text != "-" && strings.Trim(t.text, "0123456789") != ""
	}
	return false
}

// inGitPath is a write into a git directory when the path w, taken from the
// folder now, is inside one.
func (sh *shellState) inGitPath(w string) (gitRun, bool) {
	x, ok := sh.expand(w)
	if ok && x != "" {
		p := ""
		switch {
		case strings.HasPrefix(x, "~"):
			p = resolve(sh.cur, x)
		case filepath.IsAbs(x):
			p = filepath.Clean(x)
		case sh.cur != "":
			p = filepath.Join(sh.cur, x)
		}
		if p != "" {
			if gitPathRe.MatchString(p) {
				return intoGit(p), true
			}
			return gitRun{}, false
		}
	}
	if gitPathRe.MatchString(x) {
		return gitRun{IntoGit: true}, true // a folder the reading cannot tell
	}
	return gitRun{}, false
}

func intoGit(p string) gitRun {
	if top, ok := aboveGitDir(p); ok {
		return gitRun{Dir: top, IntoGit: true}
	}
	return gitRun{Dir: p, IntoGit: true}
}

// holdsGit reports whether the folder p holds a .git, which a recursive
// write in p goes through.
func holdsGit(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Lstat(filepath.Join(p, ".git"))
	return err == nil
}

// command reads one simple command in this state: its words (redirection
// targets marked) and the text piped into it. It returns the git commands it
// runs, what it writes to a following pipe, and how it may end.
func (sh *shellState) command(toks []token, piped []string, depth int) (runs []gitRun, writes []string, oc outcome) {
	var stdin, w, from, to []string
	for _, t := range toks {
		stdin = append(stdin, t.stdin...)
		switch {
		case t.redir == "":
			w = append(w, sh.substitute(t))
		case t.redir == "<":
			if t.text != "" {
				from = append(from, sh.substitute(t))
			}
		case writesFile(t):
			target := sh.substitute(t)
			if r, ok := sh.inGitPath(target); ok {
				runs = append(runs, r)
			}
			to = append(to, target)
		}
	}
	stdin = append(stdin, piped...)
	if len(w) == 0 {
		return runs, nil, outcome{ok: true}
	}
	if len(from) > 0 && slices.ContainsFunc(w, func(a string) bool {
		n := filepath.Base(a)
		return slices.Contains(shells, n) || slices.Contains(interpreters, n)
	}) {
		for _, f := range from { // a script read from a file: bash < fix.sh
			switch body, _, status := sh.scriptText(f); status {
			case scriptRead:
				stdin = append(stdin, body)
			case scriptUnreadable:
				stdin = append(stdin, unreadable)
			}
		}
	}
	// a heredoc written to a file is a script a later command may run
	if n := filepath.Base(w[0]); len(stdin) > 0 && (n == "cat" && len(w) == 1 || n == "tee") {
		files := to
		if n == "tee" {
			files = append(files, w[1:]...)
		}
		for _, f := range files {
			if x, ok := sh.expand(f); ok && !strings.HasPrefix(x, "-") {
				if p := resolve(sh.cur, x); p != "" {
					sh.rd.written[p] = strings.Join(stdin, "")
				}
			}
		}
	}
	sh.rd.pipeNext = sh.rd.pipeNext || len(to) > 0
	r, wr, oc := sh.simple(w, stdin, depth)
	if len(to) > 0 && len(wr) > 0 && !slices.Contains(wr, unreadable) { // echo '…' > run.sh
		for _, f := range to {
			if x, ok := sh.expand(f); ok {
				if p := resolve(sh.cur, x); p != "" {
					sh.rd.written[p] = strings.Join(wr, "\n")
				}
			}
		}
	}
	return append(runs, r...), wr, oc
}

// What became of reading a script file.
const (
	scriptRead       = iota
	scriptMissing    // no such file: a shell given it runs nothing
	scriptUnreadable // too big, not a regular file, only in the cloud, or a path the reading cannot tell
)

// scriptText is the script at p, from the folder now: what this command
// wrote there, or a small regular file (one only in the cloud is not read:
// reading would fetch it), and its path.
func (sh *shellState) scriptText(p string) (body, path string, status int) {
	x, ok := sh.expand(p)
	if !ok {
		return "", "", scriptUnreadable
	}
	f := resolve(sh.cur, x)
	if f == "" {
		return "", "", scriptUnreadable
	}
	if body, ok := sh.rd.written[f]; ok {
		return body, f, scriptRead
	}
	info, err := os.Stat(f)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", f, scriptMissing
	case err != nil || !info.Mode().IsRegular() || info.Size() > maxShellLen:
		return "", f, scriptUnreadable
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return "", f, scriptUnreadable
	}
	return string(b), f, scriptRead
}

// runScript reads the script file p, in this shell (source) or in another.
// One the reading cannot read, or one already being read, is an unknown run
// here.
func (sh *shellState) runScript(p string, depth int, here bool) []gitRun {
	body, path, status := sh.scriptText(p)
	switch {
	case status == scriptMissing:
		return nil
	case status == scriptUnreadable || slices.Contains(sh.rd.scripts, path):
		return []gitRun{{Dir: sh.cur, Unknown: true}}
	}
	sh.rd.scripts = append(sh.rd.scripts, path)
	defer func() { sh.rd.scripts = sh.rd.scripts[:len(sh.rd.scripts)-1] }()
	if here {
		runs, outs := sh.rd.read(body, []*shellState{sh}, depth+1)
		sh.adopt(outs)
		return runs
	}
	runs, _ := sh.rd.read(body, []*shellState{sh.copyPtr()}, depth+1)
	return runs
}

// wrapperOpts are the options that take a value, for the programs that run
// the command after their options.
var wrapperOpts = map[string][]string{
	"sudo":       {"-u", "-g", "-h", "-p", "-C", "-T", "-r", "-t", "-U", "-D", "--user", "--group", "--host", "--prompt", "--chdir"},
	"doas":       {"-u", "-C"},
	"timeout":    {"-s", "-k", "--signal", "--kill-after"},
	"nice":       {"-n", "--adjustment"},
	"ionice":     {"-c", "-n", "-p", "--class", "--classdata"},
	"stdbuf":     {"-i", "-o", "-e"},
	"xargs":      {"-I", "-n", "-P", "-L", "-d", "-E", "-s", "-a", "--replace", "--max-args", "--max-procs", "--delimiter", "--arg-file"},
	"caffeinate": {"-t", "-w"},
	"nohup":      {},
	"command":    {},
	"builtin":    {},
	"exec":       {"-a"},
}

// simple reads a command with its variables expanded where known.
func (sh *shellState) simple(w []string, stdin []string, depth int) (runs []gitRun, writes []string, oc outcome) {
	env := map[string]string{}
	assigned := map[string]string{}
	for len(w) > 0 {
		switch {
		case isAssignment(w[0]):
			k, v, _ := strings.Cut(w[0], "=")
			if x, ok := sh.expand(v); ok {
				v = x
			}
			env[k], assigned[k] = v, v
		case w[0] == "time":
			if len(w) > 1 && w[1] == "-p" {
				w = w[1:]
			}
		case slices.Contains(shellKeywords, w[0]):
		default:
			goto read
		}
		w = w[1:]
	}
	for k, v := range assigned { // assignments alone set shell variables
		sh.vars[k] = v
		if _, ok := sh.exported[k]; ok {
			sh.exported[k] = v
		}
	}
	return nil, nil, outcome{ok: true}
read:
	for k, v := range sh.exported { // what git sees: the exported variables, then this command's
		if _, ok := env[k]; !ok {
			env[k] = v
		}
	}
	words := make([]string, len(w))
	for i, a := range w { // expanded where the reading knows the variables
		if x, ok := sh.expand(a); ok {
			words[i] = x
		} else {
			words[i] = a
		}
	}
	w = words
	name := filepath.Base(w[0])
	switch name {
	case "true", ":":
		return nil, nil, outcome{ok: true}
	case "false":
		return nil, nil, outcome{fail: true}
	case "exit":
		return nil, nil, outcome{}
	case "return": // outside a function bash goes on; inside, what follows is read too
		return nil, nil, either
	case "export":
		for _, a := range w[1:] {
			if k, v, ok := strings.Cut(a, "="); ok && isAssignment(a) {
				sh.vars[k], sh.exported[k] = v, v
			} else if v, ok := sh.vars[a]; ok {
				sh.exported[a] = v
			}
		}
		return nil, nil, outcome{ok: true}
	case "local", "declare", "typeset", "readonly", "integer", "float": // assignments, no program
		for _, a := range w[1:] {
			if k, v, ok := strings.Cut(a, "="); ok && isAssignment(a) {
				sh.vars[k] = v
			}
		}
		return nil, nil, outcome{ok: true}
	case "unset":
		for _, a := range w[1:] {
			delete(sh.vars, a)
			delete(sh.exported, a)
		}
		return nil, nil, outcome{ok: true}
	case "cd", "pushd":
		var args []string
		for i, a := range w[1:] {
			if a == "--" {
				args = append(args, w[i+2:]...)
				break
			}
			if a == "-" || !strings.HasPrefix(a, "-") {
				args = append(args, a)
			}
		}
		target := "~"
		if len(args) > 0 {
			target = args[0]
		}
		if target == "-" && name == "cd" {
			if sh.prev == "" { // OLDPWD not set: cd - fails, the shell stays
				return nil, nil, outcome{fail: true}
			}
			sh.cur, sh.prev = sh.prev, sh.cur
			return nil, nil, outcome{ok: true}
		}
		pre := sh.copyPtr()
		if name == "pushd" {
			sh.pushed = append(sh.pushed, sh.cur)
		}
		next := resolve(sh.cur, target)
		if slices.Contains(w[1:], "-P") {
			next = physical(sh.cur, target) // -P: symlinks followed before ..
		}
		sh.prev, sh.cur = sh.cur, next
		// a cd to a folder that is not there (or not known) may fail, and
		// the shell stays
		oc = outcome{ok: true}
		if next == "" || !isDir(next) {
			oc.fail, oc.failSt = true, pre
		}
		return nil, nil, oc
	case "popd":
		pre := sh.copyPtr()
		if n := len(sh.pushed); n > 0 {
			sh.cur, sh.pushed = sh.pushed[n-1], sh.pushed[:n-1]
			return nil, nil, outcome{ok: true}
		}
		sh.cur = ""
		return nil, nil, outcome{ok: true, fail: true, failSt: pre}
	case "eval":
		runs, outs := sh.rd.read(strings.Join(w[1:], " "), []*shellState{sh}, depth+1)
		sh.adopt(outs)
		return runs, nil, either
	case "source", ".":
		if len(w) < 2 {
			return nil, nil, either
		}
		return sh.runScript(w[1], depth, true), nil, either
	case "alias": // read where it is used
		for _, a := range w[1:] {
			if k, v, ok := strings.Cut(a, "="); ok && k != "" {
				sh.rd.aliases[k] = v
			}
		}
		return nil, nil, outcome{ok: true}
	case "watch":
		i := 1
		for i < len(w) && strings.HasPrefix(w[i], "-") {
			if slices.Contains([]string{"-n", "--interval", "-q", "--equexit"}, w[i]) {
				i++
			}
			i++
		}
		if i < len(w) {
			runs, _ := sh.rd.read(strings.Join(w[i:], " "), []*shellState{sh.copyPtr()}, depth+1)
			return runs, nil, either
		}
		return nil, nil, either
	case "env":
		dir := sh.cur
		sub := func() *shellState {
			s := sh.copyPtr()
			s.cur = dir
			for k, v := range env {
				s.exported[k] = v
			}
			return s
		}
		for i := 1; i < len(w); i++ {
			a := w[i]
			switch {
			case a == "-i" || a == "-0" || a == "-" || a == "--ignore-environment" || a == "--null":
			case a == "-u" || a == "--unset":
				i++
			case strings.HasPrefix(a, "--unset="):
			case a == "-C" || a == "--chdir":
				if i+1 < len(w) {
					dir = resolve(dir, w[i+1])
				}
				i++
			case strings.HasPrefix(a, "--chdir="):
				dir = resolve(dir, strings.TrimPrefix(a, "--chdir="))
			case a == "-S" || a == "--split-string" || strings.HasPrefix(a, "--split-string=") || strings.HasPrefix(a, "-S"):
				// -S 'git commit': the string is split into the command
				v, rest := "", w[i+1:]
				switch {
				case a == "-S" || a == "--split-string":
					if len(rest) > 0 {
						v, rest = rest[0], rest[1:]
					}
				case strings.HasPrefix(a, "--split-string="):
					v = strings.TrimPrefix(a, "--split-string=")
				default:
					v = strings.TrimPrefix(a, "-S")
				}
				runs, _ := sh.rd.read(strings.Join(append([]string{v}, rest...), " "), []*shellState{sub()}, depth+1)
				return runs, nil, either
			case isAssignment(a):
				k, v, _ := strings.Cut(a, "=")
				env[k] = v
			case strings.HasPrefix(a, "-"):
				return unknownGit(w, sh.cur, env), nil, either
			default:
				runs, _, _ := sub().simple(w[i:], stdin, depth)
				return runs, nil, either
			}
		}
		return nil, nil, either
	case "bash", "sh", "zsh", "dash", "ksh":
		c := false
		for i := 1; i < len(w); i++ {
			a := w[i]
			switch {
			case slices.Contains([]string{"-o", "-O", "+o", "+O"}, a):
				i++
			case strings.HasPrefix(a, "--"):
			case strings.HasPrefix(a, "-") || strings.HasPrefix(a, "+"):
				c = c || strings.Contains(a, "c")
			case c: // the script -c runs
				runs, _ := sh.rd.read(a, []*shellState{sh.copyPtr()}, depth+1)
				return runs, nil, either
			default: // a script file
				return sh.runScript(a, depth, false), nil, either
			}
		}
		for _, in := range stdin { // a shell reading its script from stdin
			if in == unreadable {
				runs = append(runs, gitRun{Dir: sh.cur, Unknown: true})
				continue
			}
			r, _ := sh.rd.read(in, []*shellState{sh.copyPtr()}, depth+1)
			runs = append(runs, r...)
		}
		return runs, nil, either
	case "python", "python3", "perl", "ruby", "node", "osascript", "php", "lua", "deno":
		return sh.foreign(w, stdin), nil, either
	case "awk", "gawk", "mawk", "nawk":
		if !onlyReads(name, w[1:]) { // its program writes files or runs commands
			return sh.foreign(w, stdin), nil, either
		}
	case "apply_patch", "applypatch": // Codex's patch, run through its shell tool
		for _, text := range append(slices.Clone(w[1:]), stdin...) {
			for _, line := range strings.Split(text, "\n") {
				for _, mark := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
					if f, ok := strings.CutPrefix(strings.TrimSpace(line), mark); ok {
						if r, ok := sh.inGitPath(strings.TrimSpace(f)); ok {
							runs = append(runs, r)
						}
					}
				}
			}
		}
		return runs, nil, either
	case "echo", "printf":
		writes = append(append(writes, strings.Join(w[1:], " ")), stdin...)
	case "cat":
		writes = []string{}
		files := false
		for _, a := range w[1:] {
			if strings.HasPrefix(a, "-") && a != "-" {
				continue
			}
			files = true
			switch {
			case !sh.rd.pipeNext:
			case a == "-":
				writes = append(writes, stdin...)
			default:
				switch body, _, status := sh.scriptText(a); status {
				case scriptRead:
					writes = append(writes, body)
				case scriptUnreadable:
					writes = append(writes, unreadable)
				}
			}
		}
		if !files {
			writes = append(writes, stdin...)
		}
	case "find", "ls":
		for _, p := range startPaths(w[1:]) { // the paths it prints are under these
			writes = append(writes, strings.TrimSuffix(p, "/")+"/…")
		}
	case "gh":
		// gh commands that check out or rewrite the local repository
		if len(w) >= 3 && (w[1] == "pr" && slices.Contains([]string{"checkout", "co", "merge"}, w[2]) ||
			w[1] == "repo" && slices.Contains([]string{"sync", "set-default"}, w[2])) {
			return []gitRun{{Dir: sh.cur, Args: append([]string{"gh"}, w[1:]...)}}, nil, either
		}
	}
	if strings.HasPrefix(name, "git-") && name != "gitbrella" { // git-lfs install: git lfs install
		return parseGit(append([]string{strings.TrimPrefix(name, "git-")}, w[1:]...), sh.cur, env), writes, either
	}
	if opts, ok := wrapperOpts[name]; ok {
		i, placeholder, dir := 1, "", sh.cur
		for i < len(w) && strings.HasPrefix(w[i], "-") && w[i] != "-" {
			a := w[i]
			if a == "--" {
				i++
				break
			}
			if name == "sudo" && (a == "-D" || a == "--chdir") && i+1 < len(w) { // sudo's own cd
				dir = resolve(sh.cur, w[i+1])
				i += 2
				continue
			}
			if k, v, ok := strings.Cut(a, "="); ok && slices.Contains(opts, k) {
				if k == "--replace" {
					placeholder = v
				}
				if k == "--chdir" && name == "sudo" {
					dir = resolve(sh.cur, v)
				}
				i++
				continue
			}
			if slices.Contains(opts, a) {
				if a == "-I" && name == "xargs" && i+1 < len(w) {
					placeholder = w[i+1]
				}
				i += 2
				continue
			}
			switch {
			case name != "xargs":
			case strings.HasPrefix(a, "-I") && len(a) > 2: // -I{}
				placeholder = a[2:]
			case a == "-i": // GNU: -i alone means -I {}
				placeholder = "{}"
			case strings.HasPrefix(a, "-i"): // -iSTR: the string attached
				placeholder = a[2:]
			}
			i++
		}
		if name == "timeout" && i < len(w) {
			i++ // the duration
		}
		if i >= len(w) {
			return nil, writes, either
		}
		rest := slices.Clone(w[i:])
		if placeholder != "" {
			for j := range rest {
				rest[j] = strings.ReplaceAll(rest[j], placeholder, "$?")
			}
		} else if name == "xargs" { // the words piped in are its command's last arguments
			for _, in := range stdin {
				if in != unreadable {
					rest = append(rest, strings.Fields(in)...)
				}
			}
		}
		if slices.Contains([]string{"command", "builtin", "exec"}, name) {
			return sh.simple(rest, stdin, depth) // the shell's own: a cd behind them moves it
		}
		sub := sh.copyPtr() // another process
		sub.cur = dir
		r, _, _ := sub.simple(rest, stdin, depth)
		return r, writes, either
	}
	if name == "git" {
		return parseGit(w[1:], sh.cur, env), writes, either
	}
	if strings.Contains(w[0], "/") { // a program run by its path: a script is read
		if body, _, status := sh.scriptText(w[0]); status == scriptRead {
			first, _, _ := strings.Cut(body, "\n")
			switch {
			case !strings.HasPrefix(first, "#!") || shebangShellRe.MatchString(first):
				return sh.runScript(w[0], depth, false), writes, either
			case gitWordRe.MatchString(body):
				return []gitRun{{Dir: sh.cur, Unknown: true}}, writes, either
			}
		}
	}
	if onlyReads(name, w[1:]) {
		return nil, writes, either // git among their words is data
	}
	if r := unknownGit(w, sh.cur, env); r != nil {
		return r, writes, either
	}
	// a program given paths inside a git directory writes there, unless it
	// only reads; rm -r and find -delete go through a .git in the folder
	seen := map[string]bool{}
	add := func(r gitRun) {
		if !seen[r.Dir] {
			seen[r.Dir] = true
			runs = append(runs, r)
		}
	}
	for _, a := range pathArgs(name, w[1:]) {
		if r, ok := sh.inGitPath(a); ok {
			add(r)
		}
	}
	if recursive(name, w[1:]) {
		for _, a := range startPaths(w[1:]) {
			if x, ok := sh.expand(a); ok {
				if p := resolve(sh.cur, x); holdsGit(p) {
					add(intoGit(p))
				}
			}
		}
	}
	return runs, writes, either
}

// foreign reads a program in another language: git in its words is an
// unknown run, a .git path in them a write into that git directory.
func (sh *shellState) foreign(w []string, stdin []string) []gitRun {
	all := append(slices.Clone(w[1:]), stdin...)
	for _, a := range all {
		if gitWordRe.MatchString(a) {
			return []gitRun{{Dir: sh.cur, Unknown: true}}
		}
	}
	for _, a := range w[1:] {
		if r, ok := sh.inGitPath(a); ok {
			return []gitRun{r}
		}
	}
	for _, a := range all { // a .git path in the program's own text
		if gitLooseRe.MatchString(a) {
			return []gitRun{{Dir: sh.cur, IntoGit: true}}
		}
	}
	return nil
}

// onlyReads reports whether a program with these arguments only reads the
// files it is given.
func onlyReads(name string, args []string) bool {
	has := func(f func(string) bool) bool { return slices.ContainsFunc(args, f) }
	switch name {
	case "sed", "gsed": // not -i, -Ei, --in-place
		return !has(func(a string) bool {
			return strings.HasPrefix(a, "--in-place") || strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "i")
		})
	case "awk", "gawk", "mawk", "nawk": // not gawk -i inplace, a program file, print >, | or system()
		return !has(func(a string) bool {
			return strings.HasPrefix(a, "-i") || strings.HasPrefix(a, "--include") || a == "-f" || strings.ContainsAny(a, ">|") || strings.Contains(a, "system")
		})
	case "sort":
		return !has(func(a string) bool { return strings.HasPrefix(a, "-o") || strings.HasPrefix(a, "--output") })
	case "xattr": // -l and -p read; -w, -d and -c write
		return !has(func(a string) bool { return strings.HasPrefix(a, "-") && strings.ContainsAny(a, "wdc") })
	case "fd", "fdfind":
		return !has(func(a string) bool { return a == "-x" || a == "-X" || strings.HasPrefix(a, "--exec") })
	case "find":
		return execsOnlyRead(args)
	case "tar": // listing an archive
		return tarMode(args) == 't'
	}
	return slices.Contains(readers, name) || slices.Contains(dataPrograms, name)
}

// execsOnlyRead reports whether find deletes nothing and every command it
// runs (-exec, -ok) only reads.
func execsOnlyRead(args []string) bool {
	if slices.Contains(args, "-delete") {
		return false
	}
	for i, a := range args {
		if (strings.HasPrefix(a, "-exec") || strings.HasPrefix(a, "-ok")) && i+1 < len(args) {
			if n := filepath.Base(args[i+1]); n == "git" || !onlyReads(n, nil) {
				return false
			}
		}
	}
	return true
}

// recursive reports whether a program writes in its paths and all below
// them: rm -r, find -delete or -exec.
func recursive(name string, args []string) bool {
	switch name {
	case "rm":
		return slices.ContainsFunc(args, func(a string) bool {
			return a == "--recursive" || strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR")
		})
	case "find":
		return !execsOnlyRead(args)
	}
	return false
}

// startPaths are the paths a command is given: the words before find's
// expression, or ls's and rm's words that are no options.
func startPaths(args []string) []string {
	var out []string
	for _, a := range args {
		if a == "(" || a == "!" || strings.HasPrefix(a, "-") && a != "-" {
			if len(out) > 0 {
				break // find's expression has begun
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

// tarMode is tar's operation: 'c' writing an archive, 'x' extracting, 't'
// listing, 0 unknown.
func tarMode(args []string) byte {
	for i, a := range args {
		switch {
		case a == "--create" || a == "--append" || a == "--update":
			return 'c'
		case a == "--extract" || a == "--get":
			return 'x'
		case a == "--list":
			return 't'
		case strings.HasPrefix(a, "--"):
		case strings.HasPrefix(a, "-") || i == 0:
			for _, m := range []byte{'c', 'r', 'u', 'x', 't'} {
				if strings.IndexByte(a, m) >= 0 {
					if m == 'r' || m == 'u' {
						return 'c' // appending writes the archive too
					}
					return m
				}
			}
		}
	}
	return 0
}

// pathArgs are the arguments of a program that may name files it writes:
// for cp and the like only the destination, for tar and zip the archive (or,
// extracting, the folder), else every argument and the value of a key=value
// (dd of=).
func pathArgs(name string, args []string) []string {
	var plain []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			plain = append(plain, a)
		}
	}
	valueOf := func(opts ...string) string {
		for i, a := range args {
			for _, o := range opts {
				if a == o && i+1 < len(args) {
					return args[i+1]
				}
				if v, ok := strings.CutPrefix(a, o+"="); ok && strings.HasPrefix(o, "--") {
					return v
				}
			}
		}
		return ""
	}
	removes := slices.ContainsFunc(args, func(a string) bool { // the sources go too
		return a == "--remove-source-files" || a == "--remove-files" || a == "--move" ||
			name == "zip" && strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "m")
	})
	if removes && slices.Contains([]string{"rsync", "tar", "zip"}, name) {
		return plain
	}
	switch name {
	case "cp", "rsync", "ditto", "scp":
		if t := valueOf("-t", "--target-directory"); t != "" {
			return []string{t}
		}
		if len(plain) > 0 {
			return plain[len(plain)-1:]
		}
		return nil
	case "tar":
		switch tarMode(args) {
		case 'x':
			if d := valueOf("-C", "--directory"); d != "" {
				return []string{d}
			}
			return nil
		case 'c':
			if f := valueOf("-f", "--file"); f != "" {
				return []string{f}
			}
			// old style: tar czf archive ...
			if len(args) > 1 && !strings.HasPrefix(args[0], "-") && strings.Contains(args[0], "f") {
				return []string{args[1]}
			}
			return nil
		}
	case "zip":
		if len(plain) > 0 {
			return plain[:1]
		}
		return nil
	case "unzip":
		if d := valueOf("-d"); d != "" {
			return []string{d}
		}
		return nil
	}
	var out []string
	for _, a := range args {
		k, v, kv := strings.Cut(a, "=")
		switch {
		case strings.HasPrefix(a, "-"):
			if kv {
				out = append(out, v)
			}
		case kv && k != "":
			out = append(out, a, v)
		default:
			out = append(out, a)
		}
	}
	return out
}

// unknownGit is an unknown run if any word is git (as a program name): the
// folder is read from the words after it, as git would (sudo -u u git -C d).
func unknownGit(w []string, dir string, env map[string]string) []gitRun {
	for i, a := range w {
		if filepath.Base(a) == "git" {
			rs := parseGit(w[i+1:], dir, env)
			for j := range rs {
				rs[j].Args, rs[j].Unknown = nil, true
			}
			return rs
		}
	}
	return nil
}

// parseGit reads git's own options before the subcommand. The repository is
// the one the git directory names, else the one found from the folder; a
// work tree elsewhere is a second folder the command works on.
func parseGit(args []string, dir string, env map[string]string) []gitRun {
	gitDir, workTree := env["GIT_DIR"], env["GIT_WORK_TREE"]
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		a := args[0]
		switch {
		case a == "-C" && len(args) > 1:
			dir = physical(dir, args[1])
			args = args[2:]
		case a == "--git-dir" && len(args) > 1:
			gitDir, args = args[1], args[2:]
		case strings.HasPrefix(a, "--git-dir="):
			gitDir, args = strings.TrimPrefix(a, "--git-dir="), args[1:]
		case a == "--work-tree" && len(args) > 1:
			workTree, args = args[1], args[2:]
		case strings.HasPrefix(a, "--work-tree="):
			workTree, args = strings.TrimPrefix(a, "--work-tree="), args[1:]
		case slices.Contains(gitValueOptions, a) && len(args) > 1:
			args = args[2:]
		default:
			args = args[1:]
		}
	}
	if len(args) == 0 {
		args = []string{"--version"}
	}
	repo := dir
	if gitDir != "" {
		if g := resolve(dir, gitDir); isGitDirName(filepath.Base(g)) {
			repo = filepath.Dir(g)
		} else {
			repo = g
		}
	}
	runs := []gitRun{{Dir: repo, Args: args}}
	if workTree != "" {
		if wt := resolve(dir, workTree); wt != repo {
			runs = append(runs, gitRun{Dir: wt, Args: args})
		}
	}
	return runs
}

// physical is d taken from base the way chdir takes it: a symlink before ".."
// is followed first.
func physical(base, d string) string {
	r := resolve(base, d)
	if r == "" || !strings.Contains(d, "..") {
		return r
	}
	if strings.HasPrefix(d, "~") { // the home folder, then .. from there
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		d = home + strings.TrimPrefix(d, "~")
	}
	cur := base
	if filepath.IsAbs(d) {
		cur = "/"
	}
	for _, part := range strings.Split(d, "/") {
		switch part {
		case "", ".":
		case "..":
			if real, err := filepath.EvalSymlinks(cur); err == nil {
				cur = real
			}
			cur = filepath.Dir(cur)
		default:
			cur = filepath.Join(cur, part)
		}
	}
	return cur
}

// gitValueOptions are git's own options that take the next word as a value.
var gitValueOptions = []string{"-c", "--namespace", "--exec-path", "--config-env", "--super-prefix", "--attr-source", "--list-cmds"}

// tokenize reads s from i: words, separators and subshell marks, honouring
// quotes, backslashes and comments, marking redirection targets, giving
// heredoc bodies and here-strings to the command that reads them and keeping
// command and process substitutions (read the same way) inside their word.
// With nested it stops at the ")" closing a substitution and returns the
// index after it. ok is false for a quote, substitution or subshell left
// open.
func tokenize(s string, i int, nested bool) (toks []token, end int, ok bool) {
	var b strings.Builder
	inWord, quoted, escaped := false, false, false
	var substs []string
	redir := ""           // the operator whose target the next word is
	cmdStart := 0         // index in toks of the current command's first word
	var cmdStdin []string // here-strings of the current command
	type heredoc struct {
		delim  string
		strip  bool
		cmd    int  // the command's first token
		quoted bool // a quoted delimiter: the body is not expanded
	}
	var pending []heredoc
	delimNext, stripNext := false, false
	depth := 0 // open ( subshells
	flush := func() {
		if inWord {
			text := b.String()
			switch {
			case delimNext:
				pending = append(pending, heredoc{text, stripNext, cmdStart, quoted || escaped})
				delimNext = false
			case redir == "<<<":
				cmdStdin = append(cmdStdin, text)
				substsHere := substs
				if len(substsHere) > 0 { // a substitution in a here-string runs
					toks = append(toks, token{kind: tWord, text: "", subst: substsHere, redir: "<"})
				}
				redir = ""
			case redir != "":
				toks = append(toks, token{kind: tWord, text: text, subst: substs, redir: redir, quoted: quoted})
				redir = ""
			default:
				toks = append(toks, token{kind: tWord, text: text, subst: substs, quoted: quoted})
			}
		}
		b.Reset()
		inWord, quoted, escaped, substs = false, false, false, nil
	}
	giveStdin := func(from int, text string) {
		for j := from; j < len(toks); j++ {
			if toks[j].kind == tWord && toks[j].redir == "" {
				toks[j].stdin = append(toks[j].stdin, text)
				return
			}
		}
	}
	// giveSubst gives a command the substitutions of its unquoted heredoc,
	// which the shell runs
	giveSubst := func(from int, subs []string) {
		for j := from; j < len(toks); j++ {
			if toks[j].kind == tWord {
				toks[j].subst = append(toks[j].subst, subs...)
				return
			}
		}
	}
	endCommand := func() {
		for _, h := range cmdStdin {
			giveStdin(cmdStart, h)
		}
		cmdStdin = nil
		cmdStart = len(toks)
	}
	sep := func(kind int, text string) {
		flush()
		endCommand()
		toks = append(toks, token{kind: kind, text: text})
		cmdStart = len(toks)
	}
	// subst reads a substitution opened just before s[j] ("$(", "<(", ">(")
	subst := func(j int) (int, bool) {
		_, e, ok := tokenize(s, j, true)
		if !ok {
			return 0, false
		}
		substs = append(substs, s[j:e-1])
		return e, true
	}
	n := len(s)
	for i < n {
		c := s[i]
		switch {
		case c == '\\':
			if i+1 < n && s[i+1] != '\n' {
				b.WriteByte(s[i+1])
				inWord, escaped = true, true
			}
			i += 2
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, 0, false
			}
			b.WriteString(s[i+1 : i+1+j])
			inWord, quoted = true, true
			i += j + 2
		case c == '"':
			j := i + 1
			for ; j < n && s[j] != '"'; j++ {
				switch {
				case s[j] == '\\' && j+1 < n && strings.IndexByte("$`\"\\\n", s[j+1]) >= 0:
					j++
					b.WriteByte(s[j])
				case s[j] == '$' && j+1 < n && s[j+1] == '(' && !strings.HasPrefix(s[j:], "$(("):
					e, ok := subst(j + 2)
					if !ok {
						return nil, 0, false
					}
					b.WriteString(substMark(len(substs) - 1))
					j = e - 1
				case s[j] == '`':
					k := strings.IndexByte(s[j+1:], '`')
					if k < 0 {
						return nil, 0, false
					}
					substs = append(substs, s[j+1:j+1+k])
					b.WriteString(substMark(len(substs) - 1))
					j += k + 1
				default:
					b.WriteByte(s[j])
				}
			}
			if j >= n {
				return nil, 0, false
			}
			inWord, quoted = true, true
			i = j + 1
		case c == '#' && !inWord:
			for i < n && s[i] != '\n' { // a comment, to the end of the line
				i++
			}
		case c == ' ' || c == '\t':
			flush()
			i++
		case c == '\n':
			sep(tSep, "\n")
			i++
			for _, h := range pending { // the heredoc bodies start on the next line
				start, body := i, ""
				for i < n {
					e := strings.IndexByte(s[i:], '\n')
					line, next := s[i:], n
					if e >= 0 {
						line, next = s[i:i+e], i+e+1
					}
					check := line
					if h.strip {
						check = strings.TrimLeft(line, "\t")
					}
					if check == h.delim {
						body = s[start:i]
						i = next
						break
					}
					i = next
					body = s[start:i]
				}
				giveStdin(h.cmd, body)
				if !h.quoted {
					giveSubst(h.cmd, bodySubsts(body))
				}
			}
			pending = nil
		case c == ';':
			switch { // ;; ends a case arm, ;& and ;;& run on into the next
			case strings.HasPrefix(s[i:], ";;&"):
				sep(tSep, ";;&")
				i += 3
			case strings.HasPrefix(s[i:], ";;"), strings.HasPrefix(s[i:], ";&"):
				sep(tSep, s[i:i+2])
				i += 2
			default:
				sep(tSep, ";")
				i++
			}
		case c == '&':
			switch {
			case strings.HasPrefix(s[i:], "&&"):
				sep(tSep, "&&")
				i += 2
			case strings.HasPrefix(s[i:], "&>"):
				flush()
				i += 2
				redir = "&>"
				if i < n && s[i] == '>' {
					i++
				}
			default:
				sep(tSep, "&")
				i++
			}
		case c == '|':
			if strings.HasPrefix(s[i:], "||") {
				sep(tSep, "||")
				i += 2
			} else {
				sep(tPipe, "|")
				i++
				if i < n && s[i] == '&' {
					i++
				}
			}
		case c == '(' && !inWord && strings.HasPrefix(s[i:], "(("):
			// an arithmetic command, ((x=1<<2)): a word; with git in it, bash
			// may read it as subshells
			k, d := i+2, 2
			for ; k < n && d > 0; k++ {
				switch s[k] {
				case '(':
					d++
				case ')':
					d--
				}
			}
			if d > 0 {
				return nil, 0, false
			}
			substs = append(substs, arithSubsts(s[i+1:k-1])...)
			b.WriteString("((…))")
			inWord = true
			i = k
		case c == '(':
			sep(tOpen, "(")
			depth++
			i++
		case c == ')':
			if nested && depth == 0 {
				flush()
				endCommand()
				return toks, i + 1, true
			}
			sep(tClose, ")")
			if depth > 0 {
				depth--
			}
			i++
		case c == '$' && strings.HasPrefix(s[i:], "$(("):
			// arithmetic: a word, no command in it
			k, d := i+3, 2
			for ; k < n && d > 0; k++ {
				switch s[k] {
				case '(':
					d++
				case ')':
					d--
				}
			}
			if d > 0 {
				return nil, 0, false
			}
			substs = append(substs, arithSubsts(s[i+2:k-1])...)
			b.WriteString("$((…))")
			inWord = true
			i = k
		case c == '$' && i+1 < n && s[i+1] == '(':
			e, ok := subst(i + 2)
			if !ok {
				return nil, 0, false
			}
			b.WriteString(substMark(len(substs) - 1))
			inWord = true
			i = e
		case c == '`':
			k := strings.IndexByte(s[i+1:], '`')
			if k < 0 {
				return nil, 0, false
			}
			substs = append(substs, s[i+1:i+1+k])
			b.WriteString(substMark(len(substs) - 1))
			inWord = true
			i += k + 2
		case (c == '<' || c == '>') && i+1 < n && s[i+1] == '(':
			flush()
			e, ok := subst(i + 2)
			if !ok {
				return nil, 0, false
			}
			b.WriteString("$(…)")
			inWord = true
			i = e
		case c == '<' || c == '>':
			if inWord && strings.Trim(b.String(), "0123456789") == "" && !quoted {
				b.Reset() // a file descriptor number: 2>
				inWord = false
			} else {
				flush()
			}
			switch {
			case strings.HasPrefix(s[i:], "<<<"):
				i += 3
				redir = "<<<"
			case strings.HasPrefix(s[i:], "<<"):
				i += 2
				stripNext = i < n && s[i] == '-'
				if stripNext {
					i++
				}
				delimNext = true
			case c == '<':
				i++
				if i < n && s[i] == '&' {
					i++
				}
				redir = "<"
			default:
				i++
				redir = ">"
				if i < n && strings.IndexByte(">|", s[i]) >= 0 {
					i++
				}
				if i < n && s[i] == '&' { // >&2: a descriptor, no file
					i++
					redir = ">&"
				}
			}
		default:
			b.WriteByte(c)
			inWord = true
			i++
		}
	}
	flush()
	endCommand()
	if nested || depth != 0 {
		return nil, 0, false
	}
	return toks, n, true
}

// arithSubsts are what runs in an arithmetic text (inner, its parentheses
// included): its substitutions, $(( $(git x) + 1 )); or, with none and git in
// it, the text itself, which bash reads as subshells: $((git x) ).
func arithSubsts(inner string) []string {
	if subs := bodySubsts(inner); len(subs) > 0 {
		return subs
	}
	if gitWordRe.MatchString(inner) {
		return []string{inner}
	}
	return nil
}

// bodySubsts are the command substitutions in an unquoted heredoc's body,
// which the shell runs.
func bodySubsts(body string) []string {
	var out []string
	for j := 0; j < len(body); j++ {
		switch {
		case body[j] == '\\':
			j++
		case strings.HasPrefix(body[j:], "$(("):
			j += 2 // arithmetic
		case strings.HasPrefix(body[j:], "$("):
			_, e, ok := tokenize(body, j+2, true)
			if !ok {
				return append(out, body[j+2:])
			}
			out = append(out, body[j+2:e-1])
			j = e - 1
		case body[j] == '`':
			k := strings.IndexByte(body[j+1:], '`')
			if k < 0 {
				return append(out, body[j+1:])
			}
			out = append(out, body[j+1:j+1+k])
			j += k + 1
		}
	}
	return out
}

// substMark stands for the k-th substitution of a word: it holds a $, so a
// folder with it cannot be resolved unless the substitution is one the
// reading can answer (substitute).
func substMark(k int) string { return "$(" + strconv.Itoa(k) + ")" }

// substitute answers the substitutions of a word the reading can: pwd, git
// rev-parse --show-toplevel (the working tree around the folder), --git-dir
// and --git-path (its git directory), and which git.
func (sh *shellState) substitute(t token) string {
	text := t.text
	for k, sub := range t.subst {
		var v string
		switch s := strings.Join(strings.Fields(sub), " "); s {
		case "pwd", "pwd -P", "pwd -L":
			v = sh.cur
		case "git rev-parse --show-toplevel":
			v = topOf(sh.cur)
		case "git rev-parse --git-dir", "git rev-parse --absolute-git-dir", "git rev-parse --git-common-dir":
			v = gitDirOf(sh.cur)
		case "which git", "command -v git", "type -p git", "type -P git":
			v = "git"
		default:
			if rest, ok := strings.CutPrefix(s, "git rev-parse --git-path "); ok && !strings.ContainsAny(rest, " $") {
				if g := gitDirOf(sh.cur); g != "" {
					v = filepath.Join(g, rest)
				}
			}
		}
		if v != "" {
			text = strings.ReplaceAll(text, substMark(k), v)
		}
	}
	return text
}

// topOf is the folder holding the .git that dir is in, or "".
func topOf(dir string) string {
	if dir == "" {
		return ""
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// gitDirOf is the git directory git finds from dir: the .git of the working
// tree around it, or dir's own when none is found yet.
func gitDirOf(dir string) string {
	if dir == "" {
		return ""
	}
	if top := topOf(dir); top != "" {
		return filepath.Join(top, ".git")
	}
	return filepath.Join(dir, ".git")
}

// isGitDirName reports whether a path component names a git directory: on
// macOS, whose file systems ignore case, .GIT is the same folder.
func isGitDirName(p string) bool {
	if runtime.GOOS == "darwin" {
		return strings.EqualFold(p, ".git")
	}
	return p == ".git"
}

// isDir reports whether p is a folder now; a test replaces it.
var isDir = func(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
