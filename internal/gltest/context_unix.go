//go:build freebsd || linux || netbsd

package gltest

import (
	"errors"
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/gen2brain/gl/internal/call"
	"github.com/gen2brain/gl/internal/dl"
)

const (
	eglPlatformSurfacelessMESA = 0x31DD
	eglNone                    = 0x3038
	eglRenderableType          = 0x3040
	eglSurfaceType             = 0x3033
	eglOpenGLBit               = 0x0008
	eglOpenGLES3Bit            = 0x0040
	eglOpenGLAPI               = 0x30A2
	eglOpenGLESAPI             = 0x30A0
	eglContextMajorVersion     = 0x3098
	eglContextMinorVersion     = 0x30FB
	eglContextProfileMask      = 0x30FD
	eglContextOpenGLDebug      = 0x31B0
)

// Backends are the context APIs tested on this platform.
var Backends = []string{"egl", "glx"}

// NewContext makes a context current and returns its release function.
func NewContext(c Config) (func(), error) {
	if c.Backend == "glx" {
		return newGLX(c)
	}
	return newEGL(c)
}

func newEGL(c Config) (func(), error) {
	egl, err := dl.Open("libEGL.so.1", "libEGL.so")
	if err != nil {
		return nil, err
	}
	sym := func(name string) uintptr { return dl.Sym(egl, name) }
	cstr := func(s string) *byte { b := append([]byte(s), 0); return &b[0] }

	getPlatformDisplay := call.Call1(sym("eglGetProcAddress"), call.PTR, uintptr(unsafe.Pointer(cstr("eglGetPlatformDisplayEXT"))))
	if getPlatformDisplay == 0 {
		return nil, errors.New("gltest: no eglGetPlatformDisplayEXT")
	}
	dpy := call.Call3(getPlatformDisplay, 0x442, eglPlatformSurfacelessMESA, 0, 0)
	if dpy == 0 {
		return nil, errors.New("gltest: no surfaceless display")
	}
	if call.Call3(sym("eglInitialize"), 0x444, dpy, 0, 0) == 0 {
		return nil, errors.New("gltest: eglInitialize failed")
	}

	api, bit := uintptr(eglOpenGLAPI), int32(eglOpenGLBit)
	if c.ES {
		api, bit = eglOpenGLESAPI, eglOpenGLES3Bit
	}
	if call.Call1(sym("eglBindAPI"), call.I32, api) == 0 {
		return nil, errors.New("gltest: eglBindAPI failed")
	}

	cfgAttribs := []int32{eglRenderableType, bit, eglSurfaceType, 0, eglNone}
	var cfg uintptr
	var n int32
	if call.Call5(sym("eglChooseConfig"), 0x42444, dpy, uintptr(unsafe.Pointer(&cfgAttribs[0])), uintptr(unsafe.Pointer(&cfg)), 1, uintptr(unsafe.Pointer(&n))) == 0 || n == 0 {
		return nil, errors.New("gltest: no EGL config")
	}

	ctxAttribs := []int32{eglContextMajorVersion, int32(c.Major), eglContextMinorVersion, int32(c.Minor)}
	if !c.ES {
		profile := int32(1)
		if c.Compat {
			profile = 2
		}
		ctxAttribs = append(ctxAttribs, eglContextProfileMask, profile)
	}
	if c.Debug {
		ctxAttribs = append(ctxAttribs, eglContextOpenGLDebug, 1)
	}
	ctxAttribs = append(ctxAttribs, eglNone)
	ctx := call.Call4(sym("eglCreateContext"), 0x4444, dpy, cfg, 0, uintptr(unsafe.Pointer(&ctxAttribs[0])))
	if ctx == 0 {
		return nil, errors.New("gltest: eglCreateContext failed")
	}
	if call.Call4(sym("eglMakeCurrent"), 0x4444, dpy, 0, 0, ctx) == 0 {
		return nil, errors.New("gltest: eglMakeCurrent failed")
	}
	return func() {
		call.Call4(sym("eglMakeCurrent"), 0x4444, dpy, 0, 0, 0)
		call.Call2(sym("eglDestroyContext"), 0x44, dpy, ctx)
		call.Call1(sym("eglTerminate"), call.PTR, dpy)
	}, nil
}

const (
	glxDrawableType            = 0x8010
	glxRenderType              = 0x8011
	glxPbufferBit              = 0x4
	glxRGBABit                 = 0x1
	glxRedSize                 = 8
	glxGreenSize               = 9
	glxBlueSize                = 10
	glxAlphaSize               = 11
	glxPbufferWidth            = 0x8041
	glxPbufferHeight           = 0x8040
	glxContextMajorVersion     = 0x2091
	glxContextMinorVersion     = 0x2092
	glxContextFlags            = 0x2094
	glxContextProfileMask      = 0x9126
	glxContextES2ProfileBit    = 0x4
	glxContextCoreProfileBit   = 0x1
	glxContextCompatProfileBit = 0x2
)

var (
	xErrorOnce    sync.Once
	xErrorHandler uintptr
	xError        bool
)

func newGLX(c Config) (func(), error) {
	if os.Getenv("DISPLAY") == "" {
		return nil, errors.New("gltest: DISPLAY not set")
	}
	x11, err := dl.Open("libX11.so.6", "libX11.so")
	if err != nil {
		return nil, err
	}
	glx, err := dl.Open("libGLX.so.0", "libGL.so.1", "libGL.so")
	if err != nil {
		return nil, err
	}
	xs := func(name string) uintptr { return dl.Sym(x11, name) }
	gs := func(name string) uintptr { return dl.Sym(glx, name) }

	xErrorOnce.Do(func() {
		xErrorHandler = purego.NewCallback(func(dpy, ev uintptr) uintptr {
			xError = true
			return 0
		})
	})
	call.Call1(xs("XSetErrorHandler"), call.PTR, xErrorHandler)

	dpy := call.Call1(xs("XOpenDisplay"), call.PTR, 0)
	if dpy == 0 {
		return nil, errors.New("gltest: XOpenDisplay failed")
	}
	screen := call.Call1(xs("XDefaultScreen"), call.PTR, dpy)

	fbAttribs := []int32{glxDrawableType, glxPbufferBit, glxRenderType, glxRGBABit, glxRedSize, 8, glxGreenSize, 8,
		glxBlueSize, 8, glxAlphaSize, 8, 0}
	var n int32
	cfgs := call.Call4(gs("glXChooseFBConfig"), 0x4424, dpy, screen, uintptr(unsafe.Pointer(&fbAttribs[0])), uintptr(unsafe.Pointer(&n)))
	if cfgs == 0 || n == 0 {
		call.Call1(xs("XCloseDisplay"), call.PTR, dpy)
		return nil, errors.New("gltest: no GLX framebuffer config")
	}
	cfg := *(*uintptr)(*(*unsafe.Pointer)(unsafe.Pointer(&cfgs)))
	call.Call1(xs("XFree"), call.PTR, cfgs)

	name := []byte("glXCreateContextAttribsARB\x00")
	create := call.Call1(gs("glXGetProcAddressARB"), call.PTR, uintptr(unsafe.Pointer(&name[0])))
	profile := int32(glxContextCoreProfileBit)
	switch {
	case c.ES:
		profile = glxContextES2ProfileBit
	case c.Compat:
		profile = glxContextCompatProfileBit
	}
	flags := int32(0)
	if c.Debug {
		flags = 1
	}
	ctxAttribs := []int32{glxContextMajorVersion, int32(c.Major), glxContextMinorVersion, int32(c.Minor),
		glxContextProfileMask, profile, glxContextFlags, flags, 0}
	xError = false
	ctx := call.Call5(create, 0x42444, dpy, cfg, 0, 1, uintptr(unsafe.Pointer(&ctxAttribs[0])))
	call.Call2(xs("XSync"), 0x24, dpy, 0)
	if ctx == 0 || xError {
		call.Call1(xs("XCloseDisplay"), call.PTR, dpy)
		return nil, errors.New("gltest: glXCreateContextAttribsARB failed")
	}

	pbAttribs := []int32{glxPbufferWidth, 4, glxPbufferHeight, 4, 0}
	pbuf := call.Call3(gs("glXCreatePbuffer"), 0x444, dpy, cfg, uintptr(unsafe.Pointer(&pbAttribs[0])))
	if call.Call4(gs("glXMakeContextCurrent"), 0x4444, dpy, pbuf, pbuf, ctx) == 0 {
		return nil, errors.New("gltest: glXMakeContextCurrent failed")
	}
	return func() {
		call.Call4(gs("glXMakeContextCurrent"), 0x4444, dpy, 0, 0, 0)
		call.Call2(gs("glXDestroyPbuffer"), 0x44, dpy, pbuf)
		call.Call2(gs("glXDestroyContext"), 0x44, dpy, ctx)
		call.Call1(xs("XCloseDisplay"), call.PTR, dpy)
	}, nil
}
