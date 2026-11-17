//go:build !js

// Package procaddr resolves OpenGL function addresses of the current context.
package procaddr

import (
	"unsafe"

	"github.com/gen2brain/gl/internal/call"
)

// API is the client API whose functions are resolved.
type API int

const (
	GL API = iota
	GLES
)

type loader struct {
	lib     uintptr
	gpa     uintptr
	gpaLast bool
	sym     func(lib uintptr, name string) uintptr
}

func (l *loader) get(name string) unsafe.Pointer {
	var p uintptr
	if !l.gpaLast {
		p = l.proc(name)
	}
	if p == 0 && l.lib != 0 {
		p = l.sym(l.lib, name)
	}
	if p == 0 && l.gpaLast {
		p = l.proc(name)
	}
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

func (l *loader) proc(name string) uintptr {
	if l.gpa == 0 {
		return 0
	}
	b := make([]byte, len(name)+1)
	copy(b, name)
	p := call.Call1(l.gpa, call.PTR, uintptr(unsafe.Pointer(&b[0])))
	if !valid(p) {
		return 0
	}
	return p
}

func gostr(p uintptr) string {
	b := *(**byte)(unsafe.Pointer(&p))
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(b), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(b, n))
}
