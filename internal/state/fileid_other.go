//go:build !unix

package state

// FileID is not available here: "" (changes are refused on such systems).
func FileID(p string) string { return "" }
