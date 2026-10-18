//go:build !darwin

package icloud

import "os"

// Probe has nothing to ask outside macOS: the status is unknown.
func Probe(path string) (Status, error) {
	_, err := os.Lstat(path)
	return Status{}, err
}
