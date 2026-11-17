package gltest

import (
	"errors"
	"syscall"
	"unsafe"

	"github.com/gen2brain/gl/internal/call"
	"github.com/gen2brain/gl/internal/dl"
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type pixelFormatDescriptor struct {
	size                                           uint16
	version                                        uint16
	flags                                          uint32
	pixelType, colorBits                           uint8
	redBits, redShift, greenBits, greenShift       uint8
	blueBits, blueShift, alphaBits, alphaShift     uint8
	accumBits, accumRed, accumGreen, accumBlue     uint8
	accumAlpha, depthBits, stencilBits, auxBuffers uint8
	layerType, reserved                            uint8
	layerMask, visibleMask, damageMask             uint32
}

const (
	wglContextMajorVersion = 0x2091
	wglContextMinorVersion = 0x2092
	wglContextFlags        = 0x2094
	wglContextProfileMask  = 0x9126
)

// Backends are the context APIs tested on this platform.
var Backends = []string{"wgl"}

// NewContext makes a WGL context of a hidden window current and returns its release function.
func NewContext(c Config) (func(), error) {
	if c.ES {
		return nil, errors.New("gltest: no OpenGL ES on Windows")
	}
	user32, _ := dl.OpenSystem("user32.dll")
	gdi32, _ := dl.OpenSystem("gdi32.dll")
	opengl32, err := dl.OpenSystem("opengl32.dll")
	if err != nil {
		return nil, err
	}
	kernel32, _ := dl.OpenSystem("kernel32.dll")
	u := func(name string) uintptr { return dl.Sym(user32, name) }
	g := func(name string) uintptr { return dl.Sym(gdi32, name) }
	w := func(name string) uintptr { return dl.Sym(opengl32, name) }

	instance := call.Call1(dl.Sym(kernel32, "GetModuleHandleW"), call.PTR, 0)
	name, _ := syscall.UTF16PtrFromString("gltest")
	wc := wndClassEx{wndProc: u("DefWindowProcW"), instance: instance, className: name}
	wc.size = uint32(unsafe.Sizeof(wc))
	call.Call1(u("RegisterClassExW"), call.PTR, uintptr(unsafe.Pointer(&wc)))
	hwnd := call.Call12(u("CreateWindowExW"), 0x444422222442, 0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)),
		0x06000000, 0, 0, 16, 16, 0, 0, instance, 0)
	if hwnd == 0 {
		return nil, errors.New("gltest: CreateWindowExW failed")
	}
	dc := call.Call1(u("GetDC"), call.PTR, hwnd)

	pfd := pixelFormatDescriptor{version: 1, flags: 0x4 | 0x20 | 0x1, colorBits: 32, depthBits: 24}
	pfd.size = uint16(unsafe.Sizeof(pfd))
	format := call.Call2(g("ChoosePixelFormat"), 0x44, dc, uintptr(unsafe.Pointer(&pfd)))
	if format == 0 || call.Call3(g("SetPixelFormat"), 0x424, dc, format, uintptr(unsafe.Pointer(&pfd))) == 0 {
		return nil, errors.New("gltest: no pixel format")
	}

	legacy := call.Call1(w("wglCreateContext"), call.PTR, dc)
	if legacy == 0 || call.Call2(w("wglMakeCurrent"), 0x44, dc, legacy) == 0 {
		return nil, errors.New("gltest: wglCreateContext failed")
	}
	ctx := legacy
	if c.Major > 2 {
		cname := []byte("wglCreateContextAttribsARB\x00")
		create := call.Call1(w("wglGetProcAddress"), call.PTR, uintptr(unsafe.Pointer(&cname[0])))
		if create == 0 {
			return nil, errors.New("gltest: no wglCreateContextAttribsARB")
		}
		profile, flags := int32(1), int32(0)
		if c.Compat {
			profile = 2
		}
		if c.Debug {
			flags = 1
		}
		attribs := []int32{wglContextMajorVersion, int32(c.Major), wglContextMinorVersion, int32(c.Minor),
			wglContextProfileMask, profile, wglContextFlags, flags, 0}
		ctx = call.Call3(create, 0x444, dc, 0, uintptr(unsafe.Pointer(&attribs[0])))
		call.Call2(w("wglMakeCurrent"), 0x44, 0, 0)
		call.Call1(w("wglDeleteContext"), call.PTR, legacy)
		if ctx == 0 || call.Call2(w("wglMakeCurrent"), 0x44, dc, ctx) == 0 {
			return nil, errors.New("gltest: wglCreateContextAttribsARB failed")
		}
	}
	return func() {
		call.Call2(w("wglMakeCurrent"), 0x44, 0, 0)
		call.Call1(w("wglDeleteContext"), call.PTR, ctx)
		call.Call2(u("ReleaseDC"), 0x44, hwnd, dc)
		call.Call1(u("DestroyWindow"), call.PTR, hwnd)
	}, nil
}
