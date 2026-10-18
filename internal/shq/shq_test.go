package shq

import (
	"os/exec"
	"testing"
)

// A quoted word reaches a POSIX shell as the very same string, whatever it
// holds: spaces, quotes, $(...), backticks, tabs, newlines.
func TestQuote(t *testing.T) {
	for _, s := range []string{"plain", "a b", "it's", `cs $(echo Z) x`, "back`tick`", "tab\there", "new\nline", `"dq"`, `$HOME`, "", "ğüş"} {
		out, err := exec.Command("sh", "-c", "printf %s "+Quote(s)).Output()
		if err != nil || string(out) != s {
			t.Errorf("%q: the shell got %q (%v)", s, out, err)
		}
	}
}
