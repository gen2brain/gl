// Package debugcb creates GLDEBUGPROC callbacks.
package debugcb

import "unsafe"

// Func receives a debug message.
type Func func(source, gltype, id, severity uint32, length int32, message *uint8, userParam unsafe.Pointer)
