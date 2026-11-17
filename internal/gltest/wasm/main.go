//go:build js

package main

import (
	"fmt"
	"strings"
	"syscall/js"
	"time"
	"unsafe"

	gl "github.com/gen2brain/gl/v3.0/gles2"
)

var (
	failed []string
	info   string
)

func check(ok bool, format string, args ...any) {
	if !ok {
		failed = append(failed, fmt.Sprintf(format, args...))
	}
}

func near(a, b [4]uint8) bool {
	for i := range a {
		if d := int(a[i]) - int(b[i]); d < -1 || d > 1 {
			return false
		}
	}
	return true
}

func pixel(x, y int32) [4]uint8 {
	var px [4]uint8
	gl.ReadPixels(x, y, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&px[0]))
	return px
}

func compile(kind uint32, src string) uint32 {
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
		check(false, "compile: %s", log)
	}
	return sh
}

func run() {
	doc := js.Global().Get("document")
	canvas := doc.Call("createElement", "canvas")
	canvas.Set("width", 4)
	canvas.Set("height", 4)
	ctx := canvas.Call("getContext", "webgl2", map[string]any{"preserveDrawingBuffer": true, "antialias": false})
	if err := gl.InitWithContext(ctx); err != nil {
		check(false, "init: %v", err)
		return
	}
	version := gl.GoStr(gl.GetString(gl.VERSION))
	check(strings.HasPrefix(version, "OpenGL ES 3.0 "), "version %q", version)
	info = version + " | " + gl.GoStr(gl.GetString(gl.RENDERER))

	var fbo, tex uint32
	gl.GenFramebuffers(1, &fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, 4, 4, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
	check(gl.CheckFramebufferStatus(gl.FRAMEBUFFER) == gl.FRAMEBUFFER_COMPLETE, "framebuffer incomplete")
	var binding int32
	gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &binding)
	check(uint32(binding) == fbo, "FRAMEBUFFER_BINDING %d, want %d", binding, fbo)
	gl.Viewport(0, 0, 4, 4)

	gl.ClearColor(0.25, 0.5, 0.75, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	check(near(pixel(1, 1), [4]uint8{64, 128, 191, 255}), "clear: %v", pixel(1, 1))

	gl.ClearBufferfv(gl.COLOR, 0, &[]float32{0, 1, 0, 1}[0])
	check(pixel(2, 2) == [4]uint8{0, 255, 0, 255}, "ClearBufferfv: %v", pixel(2, 2))

	rgb := []uint8{
		10, 20, 30, 40, 50, 60, 70, 80, 90, 0, 0, 0,
		11, 21, 31, 41, 51, 61, 71, 81, 91, 0, 0, 0,
	}
	var tex2 uint32
	gl.GenTextures(1, &tex2)
	gl.BindTexture(gl.TEXTURE_2D, tex2)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB8, 3, 2, 0, gl.RGB, gl.UNSIGNED_BYTE, gl.Ptr(rgb))
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex2, 0)
	check(pixel(2, 1) == [4]uint8{71, 81, 91, 255}, "RGB upload with alignment: %v", pixel(2, 1))
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)

	vs := compile(gl.VERTEX_SHADER, `#version 300 es
layout(location = 0) in vec2 pos;
void main() { gl_Position = vec4(pos, 0.0, 1.0); }
`)
	fs := compile(gl.FRAGMENT_SHADER, `#version 300 es
precision mediump float;
uniform vec4 color;
uniform float k;
uniform mat2 m;
out vec4 frag;
void main() { frag = color * k + vec4(m[1][0], 0.0, 0.0, 0.0); }
`)
	prog := gl.CreateProgram()
	gl.AttachShader(prog, vs)
	gl.AttachShader(prog, fs)
	gl.LinkProgram(prog)
	var status, uniforms, maxLen int32
	gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
	check(status == gl.TRUE, "link failed")
	gl.GetProgramiv(prog, gl.ACTIVE_UNIFORMS, &uniforms)
	gl.GetProgramiv(prog, gl.ACTIVE_UNIFORM_MAX_LENGTH, &maxLen)
	check(uniforms == 3 && maxLen == 6, "uniforms %d max length %d", uniforms, maxLen)
	var shaders [4]uint32
	var nshaders int32
	gl.GetAttachedShaders(prog, 4, &nshaders, &shaders[0])
	check(nshaders == 2 && shaders[0]+shaders[1] == vs+fs, "attached %v", shaders[:nshaders])

	gl.UseProgram(prog)
	loc := gl.GetUniformLocation(prog, gl.Str("color\x00"))
	check(loc == gl.GetUniformLocation(prog, gl.Str("color\x00")), "uniform location not cached")
	check(gl.GetUniformLocation(prog, gl.Str("missing\x00")) == -1, "missing uniform")
	gl.Uniform4f(loc, 0.2, 0.4, 0.6, 1.0)
	gl.Uniform1f(gl.GetUniformLocation(prog, gl.Str("k\x00")), 0.5)
	gl.UniformMatrix2fv(gl.GetUniformLocation(prog, gl.Str("m\x00")), 1, false, &[]float32{0, 0, 10.0 / 255, 0}[0])
	var got [4]float32
	gl.GetUniformfv(prog, loc, &got[0])
	check(got == [4]float32{0.2, 0.4, 0.6, 1.0}, "GetUniformfv %v", got)

	var vao, vbo, ibo uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	verts := []float32{-1, -1, 1, -1, 1, 1, -1, 1}
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, nil, gl.STATIC_DRAW)
	gl.BufferSubData(gl.ARRAY_BUFFER, 0, len(verts)*4, gl.Ptr(verts))
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 8, 0)
	gl.EnableVertexAttribArray(0)
	gl.GenBuffers(1, &ibo)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)
	idx := []uint16{9, 9, 0, 1, 2, 0, 2, 3}
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(idx)*2, gl.Ptr(idx), gl.STATIC_DRAW)
	check(gl.IsBuffer(ibo) && !gl.IsBuffer(999), "IsBuffer")
	var size int32
	gl.GetBufferParameteriv(gl.ELEMENT_ARRAY_BUFFER, gl.BUFFER_SIZE, &size)
	check(size == 16, "BUFFER_SIZE %d", size)
	gl.DrawElementsWithOffset(gl.TRIANGLES, 6, gl.UNSIGNED_SHORT, 4)
	check(near(pixel(1, 2), [4]uint8{26 + 10, 51, 77, 128}), "draw: %v", pixel(1, 2))

	gl.ClearColor(0, 0, 0, 0)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.BindVertexArray(vao)
	gl.DrawArraysInstancedEXT(gl.TRIANGLE_FAN, 0, 4, 1)
	check(near(pixel(1, 2), [4]uint8{26 + 10, 51, 77, 128}), "DrawArraysInstancedEXT: %v", pixel(1, 2))

	gl.DeleteBuffers(1, &vbo)
	check(!gl.IsBuffer(vbo), "deleted buffer still a buffer")
	check(gl.GetError() == gl.NO_ERROR, "unexpected error")
	gl.ProgramBinary(prog, 0, nil, 0)
	check(gl.GetError() == gl.INVALID_OPERATION, "ProgramBinary must record INVALID_OPERATION")
	check(gl.GetError() == gl.NO_ERROR, "error not cleared")

	var mb uint32
	gl.GenBuffers(1, &mb)
	gl.BindBuffer(gl.COPY_WRITE_BUFFER, mb)
	gl.BufferData(gl.COPY_WRITE_BUFFER, 16, gl.Ptr([]byte("0123456789abcdef")), gl.DYNAMIC_DRAW)
	p := gl.MapBufferRange(gl.COPY_WRITE_BUFFER, 4, 8, gl.MAP_READ_BIT|gl.MAP_WRITE_BIT)
	check(p != nil && string(unsafe.Slice((*byte)(p), 8)) == "456789ab", "mapped read")
	var mapped int32
	gl.GetBufferParameteriv(gl.COPY_WRITE_BUFFER, gl.BUFFER_MAPPED, &mapped)
	check(mapped == 1, "BUFFER_MAPPED %d", mapped)
	if p != nil {
		copy(unsafe.Slice((*byte)(p), 8), "WXYZwxyz")
	}
	check(gl.UnmapBuffer(gl.COPY_WRITE_BUFFER), "UnmapBuffer")
	p = gl.MapBufferRange(gl.COPY_WRITE_BUFFER, 0, 16, gl.MAP_WRITE_BIT|gl.MAP_FLUSH_EXPLICIT_BIT)
	if p != nil {
		copy(unsafe.Slice((*byte)(p), 16), "................")
	}
	gl.FlushMappedBufferRange(gl.COPY_WRITE_BUFFER, 14, 2)
	gl.UnmapBuffer(gl.COPY_WRITE_BUFFER)
	p = gl.MapBufferRange(gl.COPY_WRITE_BUFFER, 0, 16, gl.MAP_READ_BIT)
	check(p != nil && string(unsafe.Slice((*byte)(p), 16)) == "0123WXYZwxyzcd..", "mapped write and flush: %q", unsafe.Slice((*byte)(p), 16))
	gl.UnmapBuffer(gl.COPY_WRITE_BUFFER)
	check(!gl.UnmapBuffer(gl.COPY_WRITE_BUFFER) && gl.GetError() == gl.INVALID_OPERATION, "unmap of unmapped buffer")

	var n int32
	gl.GetIntegerv(gl.NUM_EXTENSIONS, &n)
	info += fmt.Sprintf(" | %d extensions", n)
	if n > 0 {
		check(strings.HasPrefix(gl.GoStr(gl.GetStringi(gl.EXTENSIONS, 0)), "GL_"), "GetStringi")
	}
	var exts []string
	for i := range uint32(n) {
		exts = append(exts, gl.GoStr(gl.GetStringi(gl.EXTENSIONS, i)))
	}
	all := strings.Join(exts, " ")
	check(!strings.Contains(all, "WEBGL_lose_context") && !strings.Contains(all, "debug"), "hidden extensions listed")
	check(strings.Contains(gl.GoStr(gl.GetString(gl.EXTENSIONS)), exts[0]), "GetString(EXTENSIONS)")
	if strings.Contains(all, "GL_EXT_clip_control") {
		gl.ClipControlEXT(gl.UPPER_LEFT_EXT, gl.ZERO_TO_ONE_EXT)
		var origin int32
		gl.GetIntegerv(gl.CLIP_ORIGIN_EXT, &origin)
		check(origin == gl.UPPER_LEFT_EXT, "CLIP_ORIGIN %#x", origin)
		gl.ClipControlEXT(gl.LOWER_LEFT_EXT, gl.NEGATIVE_ONE_TO_ONE_EXT)
		info += " | clip_control"
	}
	check(gl.GetError() == gl.NO_ERROR, "unexpected error after extensions")

	var maxTex int32
	gl.GetIntegerv(gl.MAX_TEXTURE_SIZE, &maxTex)
	var viewport [4]int32
	gl.GetIntegerv(gl.VIEWPORT, &viewport[0])
	check(maxTex >= 2048 && viewport == [4]int32{0, 0, 4, 4}, "MAX_TEXTURE_SIZE %d VIEWPORT %v", maxTex, viewport)
	var blend bool
	gl.GetBooleanv(gl.BLEND, &blend)
	check(!blend && !gl.IsEnabled(gl.BLEND), "BLEND")

	contextLoss(ctx)
}

func wait(cond func() bool) bool {
	for range 200 {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func contextLoss(ctx js.Value) {
	ext := ctx.Call("getExtension", "WEBGL_lose_context")
	if !ext.Truthy() {
		return
	}
	check(gl.GetGraphicsResetStatusKHR() == gl.NO_ERROR, "reset status before loss")
	events := []string{}
	for _, name := range []string{"webglcontextlost", "webglcontextrestored"} {
		ctx.Get("canvas").Call("addEventListener", name, js.FuncOf(func(js.Value, []js.Value) any {
			events = append(events, name)
			return nil
		}))
	}
	ext.Call("loseContext")
	check(wait(func() bool { return gl.GetGraphicsResetStatusKHR() != gl.NO_ERROR }), "context loss not reported")
	check(gl.GetError() == gl.CONTEXT_LOST_KHR, "GetError after loss")
	check(wait(func() bool { return len(events) > 0 }), "no context lost event")
	ext.Call("restoreContext")
	check(wait(func() bool { return !ctx.Call("isContextLost").Bool() }), "context not restored")
	time.Sleep(50 * time.Millisecond)
	gl.GetGraphicsResetStatusKHR()
	check(gl.GetGraphicsResetStatusKHR() == gl.NO_ERROR, "reset status after restore")

	var fbo, rb uint32
	gl.GenFramebuffers(1, &fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	gl.GenRenderbuffers(1, &rb)
	gl.BindRenderbuffer(gl.RENDERBUFFER, rb)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.RGBA8, 4, 4)
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.RENDERBUFFER, rb)
	check(fbo == 1 && rb == 1, "names after restore %d %d", fbo, rb)
	gl.Viewport(0, 0, 4, 4)
	gl.ClearColor(1, 0, 1, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	check(pixel(0, 0) == [4]uint8{255, 0, 255, 255}, "clear after restore: %v", pixel(0, 0))
	check(gl.GetError() == gl.NO_ERROR, "error after restore")
	info += " | context restored"
}

func main() {
	run()
	result := "PASS " + info
	if len(failed) > 0 {
		result = "FAIL: " + strings.Join(failed, "; ") + " [" + info + "]"
	}
	fmt.Println(result)
	pre := js.Global().Get("document").Call("createElement", "pre")
	pre.Set("id", "result")
	pre.Set("textContent", result)
	js.Global().Get("document").Get("body").Call("appendChild", pre)
}
