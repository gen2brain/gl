//go:build haiku

package dl

/*
#include <dlfcn.h>
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// Open loads the first library in names that can be loaded.
func Open(names ...string) (uintptr, error) {
	var err error
	for _, name := range names {
		cs := C.CString(name)
		h := C.dlopen(cs, C.RTLD_NOW|C.RTLD_GLOBAL)
		C.free(unsafe.Pointer(cs))
		if h != nil {
			return uintptr(h), nil
		}
		err = errors.New(C.GoString(C.dlerror()))
	}
	return 0, fmt.Errorf("dl: cannot load %v: %w", names, err)
}

// Sym returns the address of name in the library h, or 0.
func Sym(h uintptr, name string) uintptr {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	return uintptr(C.dlsym(*(*unsafe.Pointer)(unsafe.Pointer(&h)), cs))
}

// Close unloads the library h.
func Close(h uintptr) error {
	if C.dlclose(*(*unsafe.Pointer)(unsafe.Pointer(&h))) != 0 {
		return errors.New(C.GoString(C.dlerror()))
	}
	return nil
}
