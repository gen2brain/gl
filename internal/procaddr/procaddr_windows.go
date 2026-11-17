package procaddr

import (
	"unsafe"

	"github.com/gen2brain/gl/internal/call"
	"github.com/gen2brain/gl/internal/dl"
)

func valid(p uintptr) bool {
	return p >= 4 && p != ^uintptr(0)
}

// New returns the function loader of the current WGL or EGL (ANGLE) context.
func New(api API) (func(name string) unsafe.Pointer, error) {
	if api == GLES {
		if egl, err := dl.Open("libEGL.dll"); err == nil {
			current := dl.Sym(egl, "eglGetCurrentContext")
			if current != 0 && call.Call0(current, 0) != 0 {
				l := &loader{gpa: dl.Sym(egl, "eglGetProcAddress"), gpaLast: true, sym: dl.Sym}
				l.lib, _ = dl.Open("libGLESv2.dll")
				return l.get, nil
			}
		}
	}
	gl, err := dl.OpenSystem("opengl32.dll")
	if err != nil {
		return nil, err
	}
	l := &loader{lib: gl, gpa: dl.Sym(gl, "wglGetProcAddress"), sym: dl.Sym}
	return l.get, nil
}
