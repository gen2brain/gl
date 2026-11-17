// Package dl loads shared libraries and resolves symbols.
package dl

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Open loads the first library in names that can be loaded.
func Open(names ...string) (uintptr, error) {
	var err error
	for _, name := range names {
		var h syscall.Handle
		h, err = syscall.LoadLibrary(name)
		if err == nil {
			return uintptr(h), nil
		}
	}
	return 0, fmt.Errorf("dl: cannot load %v: %w", names, err)
}

// Sym returns the address of name in the library h, or 0.
func Sym(h uintptr, name string) uintptr {
	p, err := syscall.GetProcAddress(syscall.Handle(h), name)
	if err != nil {
		return 0
	}
	return p
}

var loadLibraryExW = syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")

// OpenSystem loads name from the system directory.
func OpenSystem(name string) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	h, _, e := loadLibraryExW.Call(uintptr(unsafe.Pointer(p)), 0, 0x800)
	if h == 0 {
		return 0, fmt.Errorf("dl: cannot load %s: %w", name, e)
	}
	return h, nil
}
