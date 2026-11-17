package gltest

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/gen2brain/gl/internal/call"
	"github.com/gen2brain/gl/internal/dl"
)

const (
	kCGLPFAAccelerated       = 73
	kCGLPFAColorSize         = 8
	kCGLPFAOpenGLProfile     = 99
	kCGLOGLPVersionLegacy    = 0x1000
	kCGLOGLPVersion3_2_Core  = 0x3200
	kCGLOGLPVersionGL4_Core  = 0x4100
	cglChoosePixelFormatDesc = 0x444
)

// Backends are the context APIs tested on this platform.
var Backends = []string{"cgl"}

// NewContext makes a CGL context current and returns its release function.
func NewContext(c Config) (func(), error) {
	if c.ES {
		return nil, errors.New("gltest: no OpenGL ES on macOS")
	}
	lib, err := dl.Open("/System/Library/Frameworks/OpenGL.framework/OpenGL")
	if err != nil {
		return nil, err
	}
	sym := func(name string) uintptr { return dl.Sym(lib, name) }

	profile := int32(kCGLOGLPVersionLegacy)
	switch {
	case c.Compat && c.Major > 2:
		return nil, errors.New("gltest: no compatibility profile above 2.1 on macOS")
	case c.Major == 4:
		profile = kCGLOGLPVersionGL4_Core
	case c.Major == 3:
		profile = kCGLOGLPVersion3_2_Core
	}
	attribs := []int32{kCGLPFAOpenGLProfile, profile, kCGLPFAAccelerated, kCGLPFAColorSize, 24, 0}
	var pix uintptr
	var n int32
	if e := call.Call3(sym("CGLChoosePixelFormat"), cglChoosePixelFormatDesc,
		uintptr(unsafe.Pointer(&attribs[0])), uintptr(unsafe.Pointer(&pix)), uintptr(unsafe.Pointer(&n))); e != 0 || pix == 0 {
		return nil, fmt.Errorf("gltest: CGLChoosePixelFormat error %d", e)
	}
	defer call.Call1(sym("CGLDestroyPixelFormat"), call.PTR, pix)

	var ctx uintptr
	if e := call.Call3(sym("CGLCreateContext"), 0x444, pix, 0, uintptr(unsafe.Pointer(&ctx))); e != 0 {
		return nil, fmt.Errorf("gltest: CGLCreateContext error %d", e)
	}
	if e := call.Call1(sym("CGLSetCurrentContext"), call.PTR, ctx); e != 0 {
		return nil, fmt.Errorf("gltest: CGLSetCurrentContext error %d", e)
	}
	return func() {
		call.Call1(sym("CGLSetCurrentContext"), call.PTR, 0)
		call.Call1(sym("CGLDestroyContext"), call.PTR, ctx)
	}, nil
}
