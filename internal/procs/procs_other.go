//go:build !darwin && !linux

package procs

import (
	"context"
	"errors"
)

// GitProcesses is not implemented on this system.
func GitProcesses(context.Context) ([]Proc, error) {
	return nil, errors.New("process listing is not supported on this system")
}

// HookGit is not implemented on this system.
func HookGit(context.Context) (Proc, bool) { return Proc{}, false }

// Alive cannot tell on this system, so every process counts as running.
func Alive(int) bool { return true }
