## gl
[![Go Reference](https://pkg.go.dev/badge/github.com/gen2brain/gl.svg)](https://pkg.go.dev/github.com/gen2brain/gl)

Go bindings for OpenGL and OpenGL ES without cgo, generated from the Khronos registry.

The API is the same as [go-gl/gl](https://github.com/go-gl/gl): same packages, functions and constants. The `v3.0/gles2` package also works in the browser, on top of WebGL 2.

### Packages

| Package                                             | API                                                      |
|-----------------------------------------------------|----------------------------------------------------------|
| `v2.1/gl`                                           | OpenGL 2.1                                               |
| `v3.2-core/gl` ... `v4.6-core/gl`                   | OpenGL 3.2 to 4.6, core profile                          |
| `v3.2-compatibility/gl` ... `v4.6-compatibility/gl` | OpenGL 3.2 to 4.6, compatibility profile                 |
| `all-core/gl`                                       | All core functions, missing functions do not fail `Init` |
| `v3.0/gles2`, `v3.1/gles2`, `v3.2/gles2`            | OpenGL ES 3.0 to 3.2                                     |

Each package includes all compatible extensions.

### Usage

```go
import "github.com/gen2brain/gl/v3.3-core/gl"

// With an OpenGL context current on this thread.
if err := gl.Init(); err != nil {
	log.Fatal(err)
}
```

`Init` loads the functions of the current context: EGL or GLX on Linux and BSD, WGL on Windows, CGL on macOS. `InitWithProcAddrFunc` takes the loader of a windowing library instead, e.g. `glfw.GetProcAddress`.

To switch from go-gl, replace `github.com/go-gl/gl` with `github.com/gen2brain/gl` in the imports.

### Web

With `GOOS=js GOARCH=wasm`, `v3.0/gles2` calls a WebGL 2 context:

```go
canvas := js.Global().Get("document").Call("getElementById", "canvas")
if err := gl.InitWithContext(canvas.Call("getContext", "webgl2")); err != nil {
	log.Fatal(err)
}
```

`Init` uses the context in `globalThis.goGL`.

* Shaders must use `#version 300 es`.
* Client-side vertex and index arrays are not supported, pointer arguments of `VertexAttribPointer` and `DrawElements` are offsets into the bound buffer.
* `MapBufferRange` is emulated with a copy.
* Supported WebGL extensions are enabled and listed by `GetStringi(EXTENSIONS, i)`.
* Functions without a WebGL 2 equivalent (`ProgramBinary`, `ShaderBinary`, most extensions) set `INVALID_OPERATION`.
* A lost context is reported by `GetGraphicsResetStatusKHR`; after the restore all objects have to be created again.

### Platforms

* Linux, FreeBSD, NetBSD, Windows, macOS on amd64 and arm64
* Linux and Windows on 386, Linux on arm (hard float)
* Haiku on amd64
* js/wasm (`v3.0/gles2`)

Binaries still load the system libraries at run time. On FreeBSD, `CGO_ENABLED=0` builds need `-gcflags=github.com/gen2brain/purego/internal/fakecgo=-std`.

### Generating

```
go run ./internal/glgen
```

Inputs are in `internal/glgen/xml`: `gl.xml` from the [OpenGL Registry](https://github.com/KhronosGroup/OpenGL-Registry), the WebGL IDL, and the overloads from [glow](https://github.com/go-gl/glow).
