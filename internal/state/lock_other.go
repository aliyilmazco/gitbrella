//go:build !unix

package state

import "errors"

// ErrBusy: another gitbrella change holds the repository's lock.
var ErrBusy = errors.New("another gitbrella change is running on this repository")

// Lock is not available here: changes are refused.
func Lock(dir, id string) (release func(), err error) {
	return nil, errors.New("changes are supported on macOS and Linux only")
}
