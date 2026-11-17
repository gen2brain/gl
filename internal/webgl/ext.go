//go:build js

package webgl

import (
	"syscall/js"
	"unsafe"
)

const (
	glContextLost       = 0x0507
	glContextLostWebGL  = 0x9242
	glUnknownReset      = 0x8255
	glMapReadBit        = 0x0001
	glMapWriteBit       = 0x0002
	glMapFlushExplicit  = 0x0010
	glBufferMapped      = 0x88BC
	glBufferAccessFlags = 0x911F
	glBufferMapLength   = 0x9120
	glBufferMapOffset   = 0x9121
	glBufferMapPointer  = 0x88BD

	extDrawBase  = "WEBGL_draw_instanced_base_vertex_base_instance"
	extMultiDraw = "WEBGL_multi_draw"
)

var (
	canvas       js.Value
	listeners    []js.Func
	rebind       func()
	lost         bool
	resetPending bool
)

func listen() {
	c := ctx.Get("canvas")
	if c.Equal(canvas) {
		return
	}
	if canvas.Truthy() {
		canvas.Call("removeEventListener", "webglcontextlost", listeners[0])
		canvas.Call("removeEventListener", "webglcontextrestored", listeners[1])
		for _, l := range listeners {
			l.Release()
		}
	}
	canvas = c
	lost, resetPending = false, false
	listeners = []js.Func{
		js.FuncOf(func(_ js.Value, args []js.Value) any {
			args[0].Call("preventDefault")
			lost, resetPending = true, true
			return nil
		}),
		js.FuncOf(func(js.Value, []js.Value) any {
			lost = false
			reset()
			if rebind != nil {
				rebind()
			}
			return nil
		}),
	}
	canvas.Call("addEventListener", "webglcontextlost", listeners[0])
	canvas.Call("addEventListener", "webglcontextrestored", listeners[1])
}

// OnRestore sets the function that binds the context methods again after a context restore.
func OnRestore(f func()) {
	rebind = f
}

// GetGraphicsResetStatusKHR reports a lost or restored context.
func GetGraphicsResetStatusKHR() uint32 {
	if lost || Bool(call("isContextLost")) {
		return glUnknownReset
	}
	if resetPending {
		resetPending = false
		return glUnknownReset
	}
	return noError
}

type mapping struct {
	buf    []byte
	offset int
	access uint32
}

var (
	mappings      map[uint32]*mapping
	bindingPnames = map[uint32]uint32{
		0x8892: 0x8894,
		0x8893: 0x8895,
		0x8F36: 0x8F36,
		0x8F37: 0x8F37,
		0x88EB: 0x88ED,
		0x88EC: 0x88EF,
		0x8C8E: 0x8C8F,
		0x8A11: 0x8A28,
	}
)

func boundBuffer(target uint32) uint32 {
	pname, ok := bindingPnames[target]
	if !ok {
		return 0
	}
	return Buffers.ID(call("getParameter", pname))
}

func mapped(target uint32) *mapping {
	return mappings[boundBuffer(target)]
}

func (m *mapping) write(target uint32, offset, length int) {
	if length <= 0 {
		return
	}
	upload(unsafe.Pointer(&m.buf[offset]), length)
	call("bufferSubData", target, m.offset+offset, heapU8, 0, length)
}

// MapBufferRange maps a range of the bound buffer into Go memory.
func MapBufferRange(target uint32, offset int, length int, access uint32) unsafe.Pointer {
	id := boundBuffer(target)
	if id == 0 || mappings[id] != nil || length <= 0 || offset < 0 || access&(glMapReadBit|glMapWriteBit) == 0 {
		setError(invalidOperation)
		return nil
	}
	m := &mapping{buf: make([]byte, length), offset: offset, access: access}
	if access&glMapReadBit != 0 {
		grow(length)
		call("getBufferSubData", target, offset, heapU8, 0, length)
		download(unsafe.Pointer(&m.buf[0]), length)
	}
	mappings[id] = m
	return unsafe.Pointer(&m.buf[0])
}

// FlushMappedBufferRange writes a range of an explicitly flushed mapping to the buffer.
func FlushMappedBufferRange(target uint32, offset int, length int) {
	m := mapped(target)
	if m == nil || m.access&glMapFlushExplicit == 0 || offset < 0 || offset+length > len(m.buf) {
		setError(invalidOperation)
		return
	}
	m.write(target, offset, length)
}

// UnmapBuffer writes a written mapping to the bound buffer and releases it.
func UnmapBuffer(target uint32) bool {
	id := boundBuffer(target)
	m := mappings[id]
	if m == nil {
		setError(invalidOperation)
		return false
	}
	if m.access&glMapWriteBit != 0 && m.access&glMapFlushExplicit == 0 {
		m.write(target, 0, len(m.buf))
	}
	delete(mappings, id)
	return true
}

// GetBufferPointerv returns the mapped pointer of the bound buffer.
func GetBufferPointerv(target uint32, pname uint32, params *unsafe.Pointer) {
	if pname != glBufferMapPointer {
		setError(0x0500)
		return
	}
	*params = nil
	if m := mapped(target); m != nil {
		*params = unsafe.Pointer(&m.buf[0])
	}
}

func mapParameter(target, pname uint32) (int64, bool) {
	m := mapped(target)
	switch pname {
	case glBufferMapped:
		if m != nil {
			return 1, true
		}
		return 0, true
	case glBufferAccessFlags:
		if m != nil {
			return int64(m.access), true
		}
		return 0, true
	case glBufferMapLength:
		if m != nil {
			return int64(len(m.buf)), true
		}
		return 0, true
	case glBufferMapOffset:
		if m != nil {
			return int64(m.offset), true
		}
		return 0, true
	}
	return 0, false
}

// IsEnablediOES reports whether an indexed capability is enabled.
func IsEnablediOES(target uint32, index uint32) bool {
	return Bool(call("getIndexedParameter", target, index))
}

// GetQueryObjectivEXT queries a query object parameter.
func GetQueryObjectivEXT(id uint32, pname uint32, params *int32) {
	storeInt32(call("getQueryParameter", Queries.Get(id), pname), params)
}

// GetQueryObjecti64vEXT queries a query object parameter.
func GetQueryObjecti64vEXT(id uint32, pname uint32, params *int64) {
	storeInt64(call("getQueryParameter", Queries.Get(id), pname), params)
}

// GetQueryObjectui64vEXT queries a query object parameter.
func GetQueryObjectui64vEXT(id uint32, pname uint32, params *uint64) {
	*params = uint64(Number(call("getQueryParameter", Queries.Get(id), pname)))
}

// DrawElementsInstancedBaseVertexBaseInstanceEXT renders instanced primitives with a base vertex and instance.
func DrawElementsInstancedBaseVertexBaseInstanceEXT(mode uint32, count int32, xtype uint32, indices unsafe.Pointer, instancecount int32, basevertex int32, baseinstance uint32) {
	extCall(extDrawBase, "drawElementsInstancedBaseVertexBaseInstanceWEBGL", mode, count, xtype, uintptr(indices), instancecount, basevertex, baseinstance)
}

// DrawElementsInstancedBaseVertexEXT renders instanced primitives with a base vertex.
func DrawElementsInstancedBaseVertexEXT(mode uint32, count int32, xtype uint32, indices unsafe.Pointer, instancecount int32, basevertex int32) {
	extCall(extDrawBase, "drawElementsInstancedBaseVertexBaseInstanceWEBGL", mode, count, xtype, uintptr(indices), instancecount, basevertex, 0)
}

// DrawElementsInstancedBaseInstanceEXT renders instanced primitives with a base instance.
func DrawElementsInstancedBaseInstanceEXT(mode uint32, count int32, xtype uint32, indices unsafe.Pointer, instancecount int32, baseinstance uint32) {
	extCall(extDrawBase, "drawElementsInstancedBaseVertexBaseInstanceWEBGL", mode, count, xtype, uintptr(indices), instancecount, 0, baseinstance)
}

func int32sAny(p *int32, n int32) []any {
	out := make([]any, n)
	for i, v := range unsafe.Slice(p, n) {
		out[i] = v
	}
	return out
}

// MultiDrawArraysEXT renders several ranges of array data.
func MultiDrawArraysEXT(mode uint32, first *int32, count *int32, primcount int32) {
	extCall(extMultiDraw, "multiDrawArraysWEBGL", mode, int32sAny(first, primcount), 0, int32sAny(count, primcount), 0, primcount)
}

// MultiDrawElementsEXT renders several ranges of the bound element array buffer.
func MultiDrawElementsEXT(mode uint32, count *int32, xtype uint32, indices *unsafe.Pointer, primcount int32) {
	offsets := make([]any, primcount)
	for i, p := range unsafe.Slice(indices, primcount) {
		offsets[i] = uintptr(p)
	}
	extCall(extMultiDraw, "multiDrawElementsWEBGL", mode, int32sAny(count, primcount), 0, xtype, offsets, 0, primcount)
}
