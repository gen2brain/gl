//go:build freebsd || linux || netbsd

package procaddr

import (
	"errors"
	"slices"
	"strings"
	"unsafe"

	"github.com/gen2brain/gl/internal/call"
	"github.com/gen2brain/gl/internal/dl"
)

func valid(p uintptr) bool { return p != 0 }

const eglExtensions = 0x3055

func allProcAddresses(egl uintptr) bool {
	query := dl.Sym(egl, "eglQueryString")
	currentDisplay := dl.Sym(egl, "eglGetCurrentDisplay")
	if query == 0 || currentDisplay == 0 {
		return false
	}
	has := func(dpy uintptr, ext string) bool {
		p := call.Call2(query, 0x24, dpy, eglExtensions)
		if p == 0 {
			return false
		}
		return slices.Contains(strings.Fields(gostr(p)), ext)
	}
	return has(0, "EGL_KHR_client_get_all_proc_addresses") ||
		has(call.Call0(currentDisplay, 0), "EGL_KHR_get_all_proc_addresses")
}

// New returns the function loader of the current EGL or GLX context.
func New(api API) (func(name string) unsafe.Pointer, error) {
	if egl, err := dl.Open("libEGL.so.1", "libEGL.so"); err == nil {
		current := dl.Sym(egl, "eglGetCurrentContext")
		if current != 0 && call.Call0(current, 0) != 0 {
			l := &loader{gpa: dl.Sym(egl, "eglGetProcAddress"), gpaLast: !allProcAddresses(egl), sym: dl.Sym}
			if api == GLES {
				l.lib, _ = dl.Open("libGLESv2.so.2", "libGLESv2.so")
			} else {
				l.lib, _ = dl.Open("libOpenGL.so.0", "libGL.so.1", "libGL.so")
			}
			return l.get, nil
		}
	}

	glx, err := dl.Open("libGLX.so.0", "libGL.so.1", "libGL.so")
	if err != nil {
		return nil, errors.New("procaddr: no current EGL context and no GLX library")
	}
	gpa := dl.Sym(glx, "glXGetProcAddressARB")
	if gpa == 0 {
		gpa = dl.Sym(glx, "glXGetProcAddress")
	}
	l := &loader{gpa: gpa, sym: dl.Sym}
	l.lib, _ = dl.Open("libGL.so.1", "libOpenGL.so.0", "libGL.so")
	return l.get, nil
}
