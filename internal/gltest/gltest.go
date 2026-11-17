//go:build !js

// Package gltest creates headless OpenGL contexts for tests.
package gltest

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/gen2brain/gl/internal/call"
	"github.com/gen2brain/gl/internal/procaddr"
)

// Config describes the requested context.
type Config struct {
	ES     bool
	Major  int
	Minor  int
	Compat bool
	Debug  bool

	Backend string
}

const glVersion = 0x1F02

// Version returns the version of the current context.
func Version(c Config) (major, minor int, err error) {
	api := procaddr.GL
	if c.ES {
		api = procaddr.GLES
	}
	get, err := procaddr.New(api)
	if err != nil {
		return 0, 0, err
	}
	p := get("glGetString")
	if p == nil {
		return 0, 0, fmt.Errorf("gltest: no glGetString")
	}
	r := call.Call1(uintptr(p), call.I32, glVersion)
	if r == 0 {
		return 0, 0, fmt.Errorf("gltest: no GL_VERSION")
	}
	b := *(**byte)(unsafe.Pointer(&r))
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(b), n)) != 0 {
		n++
	}
	s := strings.TrimPrefix(string(unsafe.Slice(b, n)), "OpenGL ES ")
	if _, err := fmt.Sscanf(s, "%d.%d", &major, &minor); err != nil {
		return 0, 0, fmt.Errorf("gltest: GL_VERSION %q", s)
	}
	return major, minor, nil
}
