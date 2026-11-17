//go:build js

// Package webgl implements OpenGL ES 3.0 on top of a WebGL2 context.
package webgl

import (
	"errors"
	"syscall/js"
)

const (
	invalidOperation = 0x0502
	noError          = 0
)

var (
	ctx         js.Value
	methods     = map[string]js.Value{}
	exts        = map[string]js.Value{}
	glError     uint32
	unsupported js.Func
)

// Init sets the WebGL2 rendering context.
func Init(c js.Value) error {
	if !c.Truthy() || c.Get("texStorage2D").Type() != js.TypeFunction {
		return errors.New("webgl: not a WebGL2 rendering context")
	}
	ctx = c
	reset()
	listen()
	return nil
}

func reset() {
	methods = map[string]js.Value{}
	exts = map[string]js.Value{}
	for _, t := range tables {
		t.reset()
	}
	uniforms.reset()
	resetState()
	glError = noError
	if list := ctx.Call("getSupportedExtensions"); list.Type() == js.TypeObject {
		for i := range list.Length() {
			Extension(list.Index(i).String())
		}
	}
}

// Extension enables and returns the WebGL extension name, null when unsupported.
func Extension(name string) js.Value {
	if e, ok := exts[name]; ok {
		return e
	}
	e := ctx.Call("getExtension", name)
	exts[name] = e
	return e
}

// Context returns the WebGL2 rendering context.
func Context() js.Value {
	return ctx
}

// Method returns the bound method name of the context, or of the extension ext when not empty.
// A missing method records GL_INVALID_OPERATION when called.
func Method(ext, name string) js.Value {
	key := ext + "." + name
	if m, ok := methods[key]; ok {
		return m
	}
	obj := ctx
	if ext != "" {
		obj = Extension(ext)
	}
	m := js.Undefined()
	if obj.Type() == js.TypeObject {
		m = obj.Get(name)
	}
	if m.Type() == js.TypeFunction {
		m = m.Call("bind", obj)
	} else {
		if unsupported.IsUndefined() {
			unsupported = js.FuncOf(func(js.Value, []js.Value) any {
				Unsupported()
				return nil
			})
		}
		m = unsupported.Value
	}
	methods[key] = m
	return m
}

func call(name string, args ...any) js.Value {
	return Method("", name).Invoke(args...)
}

func extCall(ext, name string, args ...any) js.Value {
	return Method(ext, name).Invoke(args...)
}

// Unsupported records GL_INVALID_OPERATION for a command without a WebGL2 equivalent.
func Unsupported() {
	setError(invalidOperation)
}

func setError(e uint32) {
	if glError == noError {
		glError = e
	}
}

// GetError returns the first recorded error, then the WebGL error.
func GetError() uint32 {
	if e := glError; e != noError {
		glError = noError
		return e
	}
	e := uint32(call("getError").Int())
	if e == glContextLostWebGL {
		return glContextLost
	}
	return e
}

// Bool converts a WebGL boolean result.
func Bool(v js.Value) bool {
	return v.Type() == js.TypeBoolean && v.Bool()
}

// Number converts a WebGL numeric result, null is 0.
func Number(v js.Value) float64 {
	switch v.Type() {
	case js.TypeNumber:
		return v.Float()
	case js.TypeBoolean:
		if v.Bool() {
			return 1
		}
	}
	return 0
}

const idKey = "__goglid"

// Table maps GL object names to WebGL objects.
type Table struct {
	objs map[uint32]js.Value
	next uint32
}

var (
	Buffers            = newTable()
	Textures           = newTable()
	Framebuffers       = newTable()
	Renderbuffers      = newTable()
	Programs           = newTable()
	Shaders            = newTable()
	VertexArrays       = newTable()
	Queries            = newTable()
	Samplers           = newTable()
	TransformFeedbacks = newTable()
	Syncs              = newTable()

	tables = []*Table{Buffers, Textures, Framebuffers, Renderbuffers, Programs, Shaders, VertexArrays, Queries,
		Samplers, TransformFeedbacks, Syncs}
)

func newTable() *Table {
	t := &Table{}
	t.reset()
	return t
}

func (t *Table) reset() {
	t.objs = map[uint32]js.Value{}
	t.next = 0
}

// Add stores o and returns its name, 0 for null.
func (t *Table) Add(o js.Value) uint32 {
	if o.IsNull() || o.IsUndefined() {
		return 0
	}
	t.next++
	t.objs[t.next] = o
	o.Set(idKey, t.next)
	return t.next
}

// Get returns the object of name, null for 0 or unknown names.
func (t *Table) Get(name uint32) js.Value {
	if o, ok := t.objs[name]; ok {
		return o
	}
	return js.Null()
}

// Remove forgets name.
func (t *Table) Remove(name uint32) {
	delete(t.objs, name)
}

// ID returns the name of the object o, 0 for null or foreign objects.
func (t *Table) ID(o js.Value) uint32 {
	if o.IsNull() || o.IsUndefined() {
		return 0
	}
	id := o.Get(idKey)
	if id.Type() != js.TypeNumber {
		return 0
	}
	return uint32(id.Int())
}

type uniformTable struct {
	locs    map[int32]js.Value
	program map[uint32][]int32
	next    int32
}

var uniforms = &uniformTable{}

func (u *uniformTable) reset() {
	u.locs = map[int32]js.Value{}
	u.program = map[uint32][]int32{}
	u.next = 0
}

// Uniform returns the WebGLUniformLocation of location, null for -1 or unknown locations.
func Uniform(location int32) js.Value {
	if l, ok := uniforms.locs[location]; ok {
		return l
	}
	return js.Null()
}

func addUniform(program uint32, l js.Value) int32 {
	if l.IsNull() || l.IsUndefined() {
		return -1
	}
	uniforms.next++
	uniforms.locs[uniforms.next] = l
	uniforms.program[program] = append(uniforms.program[program], uniforms.next)
	return uniforms.next
}

func removeUniforms(program uint32) {
	for _, id := range uniforms.program[program] {
		delete(uniforms.locs, id)
	}
	delete(uniforms.program, program)
}
