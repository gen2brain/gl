//go:build haiku

package procaddr

import (
	"unsafe"

	"github.com/gen2brain/gl/internal/dl"
)

func valid(p uintptr) bool { return p != 0 }

// New returns the function loader of the Haiku OpenGL library.
func New(api API) (func(name string) unsafe.Pointer, error) {
	lib, err := dl.Open("libGL.so", "libGL.so.1")
	if err != nil {
		return nil, err
	}
	l := &loader{lib: lib, sym: dl.Sym}
	return l.get, nil
}
