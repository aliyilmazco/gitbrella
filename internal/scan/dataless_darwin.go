//go:build darwin

package scan

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
)

// sfDataless is SF_DATALESS from <sys/stat.h>: the file's contents live only in the cloud.
const sfDataless = 0x40000000

type statFlagReader struct{}

func (statFlagReader) IsDataless(path string) (bool, error) {
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return false, err
	}
	return st.Flags&sfDataless != 0, nil
}

// DefaultFlagReader reads st_flags with lstat.
func DefaultFlagReader() FlagReader { return statFlagReader{} }

// Constants from <sys/resource.h>, documented in setiopolicy_np(3).
const (
	iopolTypeVFSMaterializeDatalessFiles = 3
	iopolScopeProcess                    = 0
	iopolScopeThread                     = 1
	iopolMaterializeDatalessFilesOff     = 1
	iopolMaterializeDatalessFilesOn      = 2
)

var (
	setiopolicyOnce sync.Once
	setiopolicyNP   func(iotype, scope, policy int32) int32
	setiopolicyErr  error
)

// setiopolicy calls libSystem's setiopolicy_np through purego, so the binary
// builds with CGO_ENABLED=0 (same approach as dundee/gdu).
func setiopolicy(scope, policy int32) error {
	setiopolicyOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			setiopolicyErr = fmt.Errorf("dlopen libSystem: %w", err)
			return
		}
		sym, err := purego.Dlsym(lib, "setiopolicy_np")
		if err != nil {
			setiopolicyErr = fmt.Errorf("dlsym setiopolicy_np: %w", err)
			return
		}
		purego.RegisterFunc(&setiopolicyNP, sym)
	})
	if setiopolicyErr != nil {
		return setiopolicyErr
	}
	if rc := setiopolicyNP(iopolTypeVFSMaterializeDatalessFiles, scope, policy); rc != 0 {
		return fmt.Errorf("setiopolicy_np returned %d", rc)
	}
	return nil
}

// PreventMaterialization stops the kernel from downloading dataless files
// for this process, so a stray read fails with EDEADLK instead of fetching
// data. Apple documents this in TN3150.
func PreventMaterialization() error {
	return setiopolicy(iopolScopeProcess, iopolMaterializeDatalessFilesOff)
}

// AllowMaterialization runs fn on a thread of its own whose policy lets the
// file provider act. Moving a directory out of (or into) an iCloud Drive
// folder is such an act: under the process policy it fails with EDEADLK.
// Call it only for files already checked to be local, then nothing is
// downloaded, or to download on purpose (fix's download step). The thread
// stays locked to its goroutine, so when that ends the runtime ends the
// thread, and the policy with it.
func AllowMaterialization(fn func() error) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // never unlocked, see above
		if err := setiopolicy(iopolScopeThread, iopolMaterializeDatalessFilesOn); err != nil {
			done <- err
			return
		}
		done <- fn()
	}()
	return <-done
}
