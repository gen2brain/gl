//go:build darwin || freebsd || haiku || linux || netbsd

// Package dl loads shared libraries and resolves symbols.
package dl

import (
	"fmt"

	"github.com/gen2brain/purego"
)

// Open loads the first library in names that can be loaded.
func Open(names ...string) (uintptr, error) {
	var err error
	for _, name := range names {
		var h uintptr
		h, err = purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return h, nil
		}
	}
	return 0, fmt.Errorf("dl: cannot load %v: %w", names, err)
}

// Sym returns the address of name in the library h, or 0.
func Sym(h uintptr, name string) uintptr {
	p, err := purego.Dlsym(h, name)
	if err != nil {
		return 0
	}
	return p
}

// Close unloads the library h.
func Close(h uintptr) error {
	return purego.Dlclose(h)
}
