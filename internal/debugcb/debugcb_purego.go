//go:build ((amd64 || arm64) && (darwin || freebsd || linux || netbsd || windows)) || (amd64 && haiku) || (386 && (linux || windows)) || (arm && linux)

package debugcb

import (
	"unsafe"

	"github.com/gen2brain/purego"
)

// New returns a GLDEBUGPROC that calls f.
func New(f Func) uintptr {
	return purego.NewCallback(func(source, gltype, id, severity uint32, length int32, message *uint8, userParam unsafe.Pointer) uintptr {
		f(source, gltype, id, severity, length, message, userParam)
		return 0
	})
}
