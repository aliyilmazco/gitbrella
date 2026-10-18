// Package shq quotes words for a POSIX shell, for the commands the tool
// suggests: a path is pasted as one word, whatever it holds.
package shq

import "strings"

// Quote returns s as one single-quoted shell word.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
