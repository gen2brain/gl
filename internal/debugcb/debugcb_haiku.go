//go:build haiku

package debugcb

/*
extern void *goglDebugTrampoline(int n);
*/
import "C"

import (
	"sync"
	"unsafe"
)

const maxCallbacks = 8

var (
	mu    sync.Mutex
	funcs []Func
)

//export goglDebug
func goglDebug(n C.int, source, gltype, id, severity C.uint, length C.int, message *C.char, userParam unsafe.Pointer) {
	mu.Lock()
	f := funcs[n]
	mu.Unlock()
	f(uint32(source), uint32(gltype), uint32(id), uint32(severity), int32(length), (*uint8)(unsafe.Pointer(message)), userParam)
}

// New returns a GLDEBUGPROC that calls f.
func New(f Func) uintptr {
	mu.Lock()
	defer mu.Unlock()
	if len(funcs) == maxCallbacks {
		panic("debugcb: too many callbacks")
	}
	funcs = append(funcs, f)
	return uintptr(C.goglDebugTrampoline(C.int(len(funcs) - 1)))
}
