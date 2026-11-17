package procaddr

import (
	"runtime"
	"unsafe"

	"github.com/gen2brain/gl/internal/dl"
)

func valid(p uintptr) bool { return p != 0 }

// New returns the function loader of the OpenGL or OpenGL ES framework.
func New(api API) (func(name string) unsafe.Pointer, error) {
	var names []string
	switch {
	case api == GLES && runtime.GOOS == "ios":
		names = []string{"/System/Library/Frameworks/OpenGLES.framework/OpenGLES"}
	case api == GLES:
		names = []string{"libGLESv2.dylib"}
	default:
		names = []string{"/System/Library/Frameworks/OpenGL.framework/OpenGL"}
	}
	lib, err := dl.Open(names...)
	if err != nil {
		return nil, err
	}
	l := &loader{lib: lib, sym: dl.Sym}
	return l.get, nil
}
