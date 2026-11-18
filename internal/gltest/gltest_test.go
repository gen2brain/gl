//go:build !js && !haiku

package gltest

import (
	"runtime"
	"strings"
	"testing"
	"unsafe"

	gl21 "github.com/gen2brain/gl/v2.1/gl"
	gles2 "github.com/gen2brain/gl/v3.0/gles2"
	gl "github.com/gen2brain/gl/v3.3-core/gl"
)

func withContext(t *testing.T, c Config, f func(t *testing.T)) {
	for _, b := range Backends {
		t.Run(b, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			c.Backend = b
			release, err := NewContext(c)
			if err != nil {
				t.Skip(err)
			}
			defer release()
			major, minor, err := Version(c)
			if err != nil {
				t.Skip(err)
			}
			if major*10+minor < c.Major*10+c.Minor {
				t.Skipf("context version %d.%d", major, minor)
			}
			f(t)
		})
	}
}

func readPixel(t *testing.T, readPixels func(x, y, w, h int32, format, xtype uint32, pixels unsafe.Pointer)) [4]uint8 {
	t.Helper()
	var px [4]uint8
	readPixels(1, 1, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&px[0]))
	return px
}

func near(a, b [4]uint8) bool {
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < -1 || d > 1 {
			return false
		}
	}
	return true
}

func TestCore33(t *testing.T) {
	withContext(t, Config{Major: 3, Minor: 3, Debug: true}, func(t *testing.T) {
		if err := gl.Init(); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s, %s", gl.GoStr(gl.GetString(gl.VERSION)), gl.GoStr(gl.GetString(gl.RENDERER)))

		var fbo, rb uint32
		gl.GenFramebuffers(1, &fbo)
		gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		gl.GenRenderbuffers(1, &rb)
		gl.BindRenderbuffer(gl.RENDERBUFFER, rb)
		gl.RenderbufferStorage(gl.RENDERBUFFER, gl.RGBA8, 4, 4)
		gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.RENDERBUFFER, rb)
		if s := gl.CheckFramebufferStatus(gl.FRAMEBUFFER); s != gl.FRAMEBUFFER_COMPLETE {
			t.Fatalf("framebuffer status %#x", s)
		}
		gl.Viewport(0, 0, 4, 4)

		gl.ClearColor(0.25, 0.5, 0.75, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		if px := readPixel(t, gl.ReadPixels); !near(px, [4]uint8{64, 128, 191, 255}) {
			t.Errorf("clear: got %v", px)
		}

		vs := compile(t, gl.VERTEX_SHADER, `#version 330 core
	layout(location = 0) in vec2 pos;
	layout(location = 1) in float scale;
	void main() { gl_Position = vec4(pos * scale, 0.0, 1.0); }
	`)
		fs := compile(t, gl.FRAGMENT_SHADER, `#version 330 core
	uniform vec4 color;
	uniform float k;
	uniform int n;
	out vec4 frag;
	void main() { frag = color * k + vec4(float(n) / 255.0, 0.0, 0.0, 0.0); }
	`)
		prog := gl.CreateProgram()
		gl.AttachShader(prog, vs)
		gl.AttachShader(prog, fs)
		gl.LinkProgram(prog)
		var status int32
		gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
		if status != gl.TRUE {
			t.Fatalf("link: %s", programLog(prog))
		}
		gl.UseProgram(prog)
		gl.Uniform4f(gl.GetUniformLocation(prog, gl.Str("color\x00")), 0.2, 0.4, 0.6, 1.0)
		gl.Uniform1f(gl.GetUniformLocation(prog, gl.Str("k\x00")), 0.5)
		gl.Uniform1i(gl.GetUniformLocation(prog, gl.Str("n\x00")), 10)

		verts := []float32{-1, -1, 3, -1, -1, 3}
		var vao, vbo uint32
		gl.GenVertexArrays(1, &vao)
		gl.BindVertexArray(vao)
		gl.GenBuffers(1, &vbo)
		gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
		gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.STATIC_DRAW)
		gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 8, 0)
		gl.EnableVertexAttribArray(0)
		gl.VertexAttrib1f(1, 1.0)
		gl.DrawArrays(gl.TRIANGLES, 0, 3)
		if px := readPixel(t, gl.ReadPixels); !near(px, [4]uint8{26 + 10, 51, 77, 128}) {
			t.Errorf("draw: got %v", px)
		}

		gl.VertexAttrib1f(1, 0.0)
		gl.ClearColor(0, 0, 0, 0)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		gl.DrawArrays(gl.TRIANGLES, 0, 3)
		if px := readPixel(t, gl.ReadPixels); px != [4]uint8{} {
			t.Errorf("degenerate draw: got %v", px)
		}

		if !gl.IsEnabled(gl.DITHER) || gl.IsEnabled(gl.BLEND) {
			t.Error("IsEnabled")
		}
		if !gl.IsProgram(prog) {
			t.Error("IsProgram")
		}

		var buf uint32
		gl.GenBuffers(1, &buf)
		gl.BindBuffer(gl.COPY_WRITE_BUFFER, buf)
		gl.BufferData(gl.COPY_WRITE_BUFFER, 16, nil, gl.DYNAMIC_DRAW)
		p := gl.MapBufferRange(gl.COPY_WRITE_BUFFER, 0, 16, gl.MAP_WRITE_BIT)
		if p == nil {
			t.Fatal("MapBufferRange returned nil")
		}
		copy(unsafe.Slice((*byte)(p), 16), "0123456789abcdef")
		if !gl.UnmapBuffer(gl.COPY_WRITE_BUFFER) {
			t.Error("UnmapBuffer")
		}
		var back [16]byte
		gl.GetBufferSubData(gl.COPY_WRITE_BUFFER, 4, 8, unsafe.Pointer(&back[0]))
		if string(back[:8]) != "456789ab" {
			t.Errorf("buffer: got %q", back[:8])
		}

		if gl.GetError() != gl.NO_ERROR {
			t.Error("unexpected GL error")
		}

		if hasExtension("GL_KHR_debug") {
			var got []string
			gl.Enable(gl.DEBUG_OUTPUT_SYNCHRONOUS)
			gl.DebugMessageCallback(func(source, gltype, id, severity uint32, length int32, message string, userParam unsafe.Pointer) {
				if gltype == gl.DEBUG_TYPE_ERROR {
					got = append(got, message)
				}
			}, nil)
			gl.Enable(0xFFFF)
			gl.DebugMessageCallback(nil, nil)
			if gl.GetError() != gl.INVALID_ENUM {
				t.Error("expected INVALID_ENUM")
			}
			if len(got) == 0 {
				t.Error("debug callback not called")
			} else {
				t.Logf("debug: %s", got[0])
			}
		}
	})
}

func hasExtension(name string) bool {
	var n int32
	gl.GetIntegerv(gl.NUM_EXTENSIONS, &n)
	for i := range uint32(n) {
		if gl.GoStr(gl.GetStringi(gl.EXTENSIONS, i)) == name {
			return true
		}
	}
	return false
}

func compile(t *testing.T, kind uint32, src string) uint32 {
	t.Helper()
	sh := gl.CreateShader(kind)
	csrc, free := gl.Strs(src)
	n := int32(len(src))
	gl.ShaderSource(sh, 1, csrc, &n)
	free()
	gl.CompileShader(sh)
	var status int32
	gl.GetShaderiv(sh, gl.COMPILE_STATUS, &status)
	if status != gl.TRUE {
		var l int32
		gl.GetShaderiv(sh, gl.INFO_LOG_LENGTH, &l)
		log := strings.Repeat("\x00", int(l+1))
		gl.GetShaderInfoLog(sh, l, nil, gl.Str(log))
		t.Fatalf("compile: %s", log)
	}
	return sh
}

func programLog(prog uint32) string {
	var l int32
	gl.GetProgramiv(prog, gl.INFO_LOG_LENGTH, &l)
	log := strings.Repeat("\x00", int(l+1))
	gl.GetProgramInfoLog(prog, l, nil, gl.Str(log))
	return log
}

func TestCompat21(t *testing.T) {
	withContext(t, Config{Major: 2, Minor: 1, Compat: true}, func(t *testing.T) {
		if err := gl21.Init(); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", gl21.GoStr(gl21.GetString(gl21.VERSION)))

		var fbo, rb uint32
		gl21.GenFramebuffersEXT(1, &fbo)
		gl21.BindFramebufferEXT(gl21.FRAMEBUFFER_EXT, fbo)
		gl21.GenRenderbuffersEXT(1, &rb)
		gl21.BindRenderbufferEXT(gl21.RENDERBUFFER_EXT, rb)
		gl21.RenderbufferStorageEXT(gl21.RENDERBUFFER_EXT, gl21.RGBA8, 4, 4)
		gl21.FramebufferRenderbufferEXT(gl21.FRAMEBUFFER_EXT, gl21.COLOR_ATTACHMENT0_EXT, gl21.RENDERBUFFER_EXT, rb)
		gl21.Viewport(0, 0, 4, 4)

		gl21.ClearColor(0, 0, 0, 1)
		gl21.Clear(gl21.COLOR_BUFFER_BIT)
		gl21.MatrixMode(gl21.PROJECTION)
		gl21.LoadIdentity()
		gl21.Ortho(-1, 1, -1, 1, -1, 1)
		gl21.Begin(gl21.TRIANGLES)
		gl21.Color4ub(200, 100, 50, 255)
		gl21.Vertex2f(-1, -1)
		gl21.Vertex2f(3, -1)
		gl21.Vertex2f(-1, 3)
		gl21.End()
		if px := readPixel(t, gl21.ReadPixels); !near(px, [4]uint8{200, 100, 50, 255}) {
			t.Errorf("immediate mode: got %v", px)
		}

		gl21.Clear(gl21.COLOR_BUFFER_BIT)
		gl21.Rotated(90, 0, 0, 1)
		gl21.Translated(0.5, 0, 0)
		gl21.Begin(gl21.QUADS)
		gl21.Color3f(0.2, 0.4, 0.6)
		gl21.Vertex3d(-0.3, -0.3, 0)
		gl21.Vertex3d(0.3, -0.3, 0)
		gl21.Vertex3d(0.3, 0.3, 0)
		gl21.Vertex3d(-0.3, 0.3, 0)
		gl21.End()
		var px [4]uint8
		gl21.ReadPixels(1, 3, 1, 1, gl21.RGBA, gl21.UNSIGNED_BYTE, unsafe.Pointer(&px[0]))
		if !near(px, [4]uint8{51, 102, 153, 255}) {
			t.Errorf("rotated quad: got %v", px)
		}
		if gl21.GetError() != gl21.NO_ERROR {
			t.Error("unexpected GL error")
		}
	})
}

func TestES30(t *testing.T) {
	withContext(t, Config{ES: true, Major: 3, Minor: 0}, func(t *testing.T) {
		if err := gles2.Init(); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", gles2.GoStr(gles2.GetString(gles2.VERSION)))

		var fbo, rb uint32
		gles2.GenFramebuffers(1, &fbo)
		gles2.BindFramebuffer(gles2.FRAMEBUFFER, fbo)
		gles2.GenRenderbuffers(1, &rb)
		gles2.BindRenderbuffer(gles2.RENDERBUFFER, rb)
		gles2.RenderbufferStorage(gles2.RENDERBUFFER, gles2.RGBA8, 4, 4)
		gles2.FramebufferRenderbuffer(gles2.FRAMEBUFFER, gles2.COLOR_ATTACHMENT0, gles2.RENDERBUFFER, rb)
		gles2.Viewport(0, 0, 4, 4)
		gles2.ClearColor(1, 0.5, 0, 1)
		gles2.Clear(gles2.COLOR_BUFFER_BIT)
		if px := readPixel(t, gles2.ReadPixels); !near(px, [4]uint8{255, 128, 0, 255}) {
			t.Errorf("clear: got %v", px)
		}
		gles2.Enable(gles2.SCISSOR_TEST)
		gles2.Scissor(0, 0, 2, 2)
		gles2.ClearBufferfv(gles2.COLOR, 0, &[]float32{0, 1, 0, 1}[0])
		gles2.Disable(gles2.SCISSOR_TEST)
		if px := readPixel(t, gles2.ReadPixels); px != [4]uint8{0, 255, 0, 255} {
			t.Errorf("scissored clear: got %v", px)
		}
		if gles2.GetError() != gles2.NO_ERROR {
			t.Error("unexpected GL error")
		}
	})
}
