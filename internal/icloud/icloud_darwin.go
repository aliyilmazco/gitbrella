//go:build darwin

package icloud

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	setupOnce sync.Once
	setupErr  error
	keyUbiq   objc.ID // NSURLIsUbiquitousItemKey
	keyUp     objc.ID // NSURLUbiquitousItemIsUploadedKey
	keyUping  objc.ID // NSURLUbiquitousItemIsUploadingKey
)

func setup() {
	fw, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		setupErr = err
		return
	}
	for name, dst := range map[string]*objc.ID{
		"NSURLIsUbiquitousItemKey":          &keyUbiq,
		"NSURLUbiquitousItemIsUploadedKey":  &keyUp,
		"NSURLUbiquitousItemIsUploadingKey": &keyUping,
	} {
		sym, err := purego.Dlsym(fw, name)
		if err != nil {
			setupErr = err
			return
		}
		// sym is the address of the NSString constant; read the pointer
		// through a pointer to the variable (vet rejects uintptr->Pointer).
		addr := *(*unsafe.Pointer)(unsafe.Pointer(&sym))
		*dst = *(*objc.ID)(addr)
	}
}

// Probe reads NSURL resource values for path through the Objective-C runtime
// (purego, no cgo).
func Probe(path string) (Status, error) {
	if _, err := os.Lstat(path); err != nil {
		return Status{}, err
	}
	setupOnce.Do(setup)
	if setupErr != nil {
		return Status{}, fmt.Errorf("iCloud status unavailable: %w", setupErr)
	}
	// An autorelease pool belongs to the thread that made it: draining it on
	// another one crashes. Keep the goroutine on one thread throughout.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	sel := objc.RegisterName
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	b := append([]byte(path), 0)
	str := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), unsafe.Pointer(&b[0]))
	runtime.KeepAlive(b)
	url := objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), str)
	get := func(key objc.ID) (value, present bool, err error) {
		var val, nserr objc.ID
		if !objc.Send[bool](url, sel("getResourceValue:forKey:error:"), unsafe.Pointer(&val), key, unsafe.Pointer(&nserr)) {
			return false, false, fmt.Errorf("iCloud status of %s could not be read", path)
		}
		if val == 0 {
			return false, false, nil
		}
		return objc.Send[bool](val, sel("boolValue")), true, nil
	}
	var s Status
	var err error
	if s.Ubiquitous, _, err = get(keyUbiq); err != nil {
		return Status{}, err
	}
	s.Known = true
	if !s.Ubiquitous {
		return s, nil
	}
	if s.Uploaded, _, err = get(keyUp); err != nil {
		return Status{}, err
	}
	if s.Uploading, _, err = get(keyUping); err != nil {
		return Status{}, err
	}
	return s, nil
}
