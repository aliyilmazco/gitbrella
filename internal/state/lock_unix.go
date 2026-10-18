//go:build unix

package state

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// ErrBusy: another gitbrella change holds the repository's lock.
var ErrBusy = errors.New("another gitbrella change is running on this repository")

// Lock takes the per-repository lock for a change; release gives it back.
// The kernel drops the lock when the process ends, so a crash leaves no lock
// to clean up.
func Lock(dir, id string) (release func(), err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, id+".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
