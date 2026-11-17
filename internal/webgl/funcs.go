//go:build js

package webgl

import (
	"strings"
	"syscall/js"
	"unsafe"
)

const (
	glColor                            = 0x1800
	glExtensions                       = 0x1F03
	glVersion                          = 0x1F02
	glShadingLanguageVersion           = 0x8B8C
	glNumExtensions                    = 0x821D
	glMajorVersion                     = 0x821B
	glMinorVersion                     = 0x821C
	glNumShaderBinaryFormats           = 0x8DF9
	glNumProgramBinaryFormats          = 0x87FE
	glInfoLogLength                    = 0x8B84
	glShaderSourceLength               = 0x8B88
	glActiveUniformMaxLength           = 0x8B87
	glActiveAttributeMaxLength         = 0x8B8A
	glTransformFeedbackVaryingMaxLen   = 0x8C76
	glActiveUniformBlockMaxNameLength  = 0x8A35
	glActiveUniforms                   = 0x8B86
	glActiveAttributes                 = 0x8B89
	glTransformFeedbackVaryings        = 0x8C83
	glActiveUniformBlocks              = 0x8A36
	glProgramBinaryLength              = 0x8741
	glUniformBlockNameLength           = 0x8A41
	glUniformBlockActiveUniformIndices = 0x8A43
	glUniformNameLength                = 0x8A39
)

// BindBuffer binds buffer to target.
func BindBuffer(target uint32, buffer uint32) {
	switch target {
	case glPixelPackBuffer:
		packBuffer = buffer
	case glPixelUnpackBuffer:
		unpackBuffer = buffer
	}
	call("bindBuffer", target, Buffers.Get(buffer))
}

// PixelStorei sets pixel storage modes.
func PixelStorei(pname uint32, param int32) {
	v := int(param)
	switch pname {
	case glPackAlignment:
		pack.alignment = v
	case glPackRowLength:
		pack.rowLength = v
	case glPackSkipRows:
		pack.skipRows = v
	case glPackSkipPixels:
		pack.skipPixels = v
	case glUnpackAlignment:
		unpack.alignment = v
	case glUnpackRowLength:
		unpack.rowLength = v
	case glUnpackImageHeight:
		unpack.imageHeight = v
	case glUnpackSkipRows:
		unpack.skipRows = v
	case glUnpackSkipPixels:
		unpack.skipPixels = v
	case glUnpackSkipImages:
		unpack.skipImages = v
	}
	call("pixelStorei", pname, param)
}

// DeleteProgram deletes program and its uniform locations.
func DeleteProgram(program uint32) {
	call("deleteProgram", Programs.Get(program))
	removeUniforms(program)
	delete(uniformCache, program)
	Programs.Remove(program)
}

// BufferData creates the data store of the bound buffer.
func BufferData(target uint32, size int, data unsafe.Pointer, usage uint32) {
	if data == nil {
		call("bufferData", target, size, usage)
		return
	}
	upload(data, size)
	call("bufferData", target, heapU8, usage, 0, size)
}

// BufferSubData updates a subset of the data store of the bound buffer.
func BufferSubData(target uint32, offset int, size int, data unsafe.Pointer) {
	upload(data, size)
	call("bufferSubData", target, offset, heapU8, 0, size)
}

func clearBuffer(name string, view string, buffer uint32, drawbuffer int32, value unsafe.Pointer) {
	n := 1
	if buffer == glColor {
		n = 4
	}
	call(name, buffer, drawbuffer, list(view, 4, value, n))
}

// ClearBufferfv clears a buffer of the draw framebuffer.
func ClearBufferfv(buffer uint32, drawbuffer int32, value *float32) {
	clearBuffer("clearBufferfv", "Float32Array", buffer, drawbuffer, unsafe.Pointer(value))
}

// ClearBufferiv clears a buffer of the draw framebuffer.
func ClearBufferiv(buffer uint32, drawbuffer int32, value *int32) {
	clearBuffer("clearBufferiv", "Int32Array", buffer, drawbuffer, unsafe.Pointer(value))
}

// ClearBufferuiv clears a buffer of the draw framebuffer.
func ClearBufferuiv(buffer uint32, drawbuffer int32, value *uint32) {
	clearBuffer("clearBufferuiv", "Uint32Array", buffer, drawbuffer, unsafe.Pointer(value))
}

func compressed(name string, args []any, imageSize int32, data unsafe.Pointer) {
	if unpackBuffer != 0 {
		call(name, append(args, imageSize, uintptr(data))...)
		return
	}
	upload(data, int(imageSize))
	call(name, append(args, heapU8, 0, imageSize)...)
}

// CompressedTexImage2D specifies a compressed two-dimensional texture image.
func CompressedTexImage2D(target uint32, level int32, internalformat uint32, width int32, height int32, border int32, imageSize int32, data unsafe.Pointer) {
	compressed("compressedTexImage2D", []any{target, level, internalformat, width, height, border}, imageSize, data)
}

// CompressedTexImage3D specifies a compressed three-dimensional texture image.
func CompressedTexImage3D(target uint32, level int32, internalformat uint32, width int32, height int32, depth int32, border int32, imageSize int32, data unsafe.Pointer) {
	compressed("compressedTexImage3D", []any{target, level, internalformat, width, height, depth, border}, imageSize, data)
}

// CompressedTexSubImage2D specifies a compressed two-dimensional texture subimage.
func CompressedTexSubImage2D(target uint32, level int32, xoffset int32, yoffset int32, width int32, height int32, format uint32, imageSize int32, data unsafe.Pointer) {
	compressed("compressedTexSubImage2D", []any{target, level, xoffset, yoffset, width, height, format}, imageSize, data)
}

// CompressedTexSubImage3D specifies a compressed three-dimensional texture subimage.
func CompressedTexSubImage3D(target uint32, level int32, xoffset int32, yoffset int32, zoffset int32, width int32, height int32, depth int32, format uint32, imageSize int32, data unsafe.Pointer) {
	compressed("compressedTexSubImage3D", []any{target, level, xoffset, yoffset, zoffset, width, height, depth, format}, imageSize, data)
}

func texImage(name string, args []any, w, h, d int32, format, xtype uint32, pixels unsafe.Pointer) {
	args = append(args, format, xtype)
	switch {
	case unpackBuffer != 0:
		call(name, append(args, uintptr(pixels))...)
	case pixels == nil:
		call(name, append(args, js.Null())...)
	default:
		upload(pixels, unpack.size(int(w), int(h), int(d), format, xtype))
		view, _ := typeView(xtype)
		call(name, append(args, views[view], 0)...)
	}
}

// TexImage2D specifies a two-dimensional texture image.
func TexImage2D(target uint32, level int32, internalformat int32, width int32, height int32, border int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	texImage("texImage2D", []any{target, level, internalformat, width, height, border}, width, height, 1, format, xtype, pixels)
}

// TexImage3D specifies a three-dimensional texture image.
func TexImage3D(target uint32, level int32, internalformat int32, width int32, height int32, depth int32, border int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	texImage("texImage3D", []any{target, level, internalformat, width, height, depth, border}, width, height, depth, format, xtype, pixels)
}

// TexSubImage2D specifies a two-dimensional texture subimage.
func TexSubImage2D(target uint32, level int32, xoffset int32, yoffset int32, width int32, height int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	texImage("texSubImage2D", []any{target, level, xoffset, yoffset, width, height}, width, height, 1, format, xtype, pixels)
}

// TexSubImage3D specifies a three-dimensional texture subimage.
func TexSubImage3D(target uint32, level int32, xoffset int32, yoffset int32, zoffset int32, width int32, height int32, depth int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	texImage("texSubImage3D", []any{target, level, xoffset, yoffset, zoffset, width, height, depth}, width, height, depth, format, xtype, pixels)
}

// ReadPixels reads a block of pixels from the read framebuffer.
func ReadPixels(x int32, y int32, width int32, height int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	if packBuffer != 0 {
		call("readPixels", x, y, width, height, format, xtype, uintptr(pixels))
		return
	}
	n := pack.size(int(width), int(height), 1, format, xtype)
	grow(n)
	view, _ := typeView(xtype)
	call("readPixels", x, y, width, height, format, xtype, views[view], 0)
	download(pixels, n)
}

// TexParameterfv sets a texture parameter.
func TexParameterfv(target uint32, pname uint32, params *float32) {
	call("texParameterf", target, pname, *params)
}

// TexParameteriv sets a texture parameter.
func TexParameteriv(target uint32, pname uint32, params *int32) {
	call("texParameteri", target, pname, *params)
}

// SamplerParameterfv sets a sampler parameter.
func SamplerParameterfv(sampler uint32, pname uint32, param *float32) {
	call("samplerParameterf", Samplers.Get(sampler), pname, *param)
}

// SamplerParameteriv sets a sampler parameter.
func SamplerParameteriv(sampler uint32, pname uint32, param *int32) {
	call("samplerParameteri", Samplers.Get(sampler), pname, *param)
}

// DrawBuffers specifies the color buffers to draw into.
func DrawBuffers(n int32, bufs *uint32) {
	call("drawBuffers", uint32s(unsafe.Slice(bufs, n)))
}

// InvalidateFramebuffer invalidates framebuffer attachments.
func InvalidateFramebuffer(target uint32, numAttachments int32, attachments *uint32) {
	call("invalidateFramebuffer", target, uint32s(unsafe.Slice(attachments, numAttachments)))
}

// InvalidateSubFramebuffer invalidates a region of framebuffer attachments.
func InvalidateSubFramebuffer(target uint32, numAttachments int32, attachments *uint32, x int32, y int32, width int32, height int32) {
	call("invalidateSubFramebuffer", target, uint32s(unsafe.Slice(attachments, numAttachments)), x, y, width, height)
}

func uint32s(v []uint32) []any {
	out := make([]any, len(v))
	for i, x := range v {
		out[i] = x
	}
	return out
}

func stringsAny(v []string) []any {
	out := make([]any, len(v))
	for i, x := range v {
		out[i] = x
	}
	return out
}

// DrawElements renders primitives from the bound element array buffer.
func DrawElements(mode uint32, count int32, xtype uint32, indices unsafe.Pointer) {
	call("drawElements", mode, count, xtype, uintptr(indices))
}

// DrawElementsWithOffset renders primitives from the bound element array buffer.
func DrawElementsWithOffset(mode uint32, count int32, xtype uint32, indices uintptr) {
	call("drawElements", mode, count, xtype, indices)
}

// DrawElementsInstanced renders instanced primitives from the bound element array buffer.
func DrawElementsInstanced(mode uint32, count int32, xtype uint32, indices unsafe.Pointer, instancecount int32) {
	call("drawElementsInstanced", mode, count, xtype, uintptr(indices), instancecount)
}

// DrawRangeElements renders primitives from the bound element array buffer.
func DrawRangeElements(mode uint32, start uint32, end uint32, count int32, xtype uint32, indices unsafe.Pointer) {
	call("drawRangeElements", mode, start, end, count, xtype, uintptr(indices))
}

// VertexAttribPointer defines an array of vertex attribute data in the bound array buffer.
func VertexAttribPointer(index uint32, size int32, xtype uint32, normalized bool, stride int32, pointer unsafe.Pointer) {
	call("vertexAttribPointer", index, size, xtype, normalized, stride, uintptr(pointer))
}

// VertexAttribPointerWithOffset defines an array of vertex attribute data in the bound array buffer.
func VertexAttribPointerWithOffset(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset uintptr) {
	call("vertexAttribPointer", index, size, xtype, normalized, stride, offset)
}

// VertexAttribIPointer defines an array of integer vertex attribute data in the bound array buffer.
func VertexAttribIPointer(index uint32, size int32, xtype uint32, stride int32, pointer unsafe.Pointer) {
	call("vertexAttribIPointer", index, size, xtype, stride, uintptr(pointer))
}

// VertexAttribIPointerWithOffset defines an array of integer vertex attribute data in the bound array buffer.
func VertexAttribIPointerWithOffset(index uint32, size int32, xtype uint32, stride int32, offset uintptr) {
	call("vertexAttribIPointer", index, size, xtype, stride, offset)
}

// ShaderSource replaces the source code of shader.
func ShaderSource(shader uint32, count int32, xstring **uint8, length *int32) {
	call("shaderSource", Shaders.Get(shader), strings.Join(cStrs(count, xstring, length), ""))
}

// TransformFeedbackVaryings specifies the varyings to record in transform feedback.
func TransformFeedbackVaryings(program uint32, count int32, varyings **uint8, bufferMode uint32) {
	call("transformFeedbackVaryings", Programs.Get(program), stringsAny(cStrs(count, varyings, nil)), bufferMode)
}

// BindAttribLocation binds a vertex attribute index to a named attribute.
func BindAttribLocation(program uint32, index uint32, name *uint8) {
	call("bindAttribLocation", Programs.Get(program), index, goStr(name))
}

// GetAttribLocation returns the location of an attribute.
func GetAttribLocation(program uint32, name *uint8) int32 {
	return int32(Number(call("getAttribLocation", Programs.Get(program), goStr(name))))
}

// GetFragDataLocation returns the location of a fragment shader output.
func GetFragDataLocation(program uint32, name *uint8) int32 {
	return int32(Number(call("getFragDataLocation", Programs.Get(program), goStr(name))))
}

// GetUniformBlockIndex returns the index of a uniform block.
func GetUniformBlockIndex(program uint32, uniformBlockName *uint8) uint32 {
	return uint32(int64(Number(call("getUniformBlockIndex", Programs.Get(program), goStr(uniformBlockName)))))
}

// GetUniformLocation returns the location of a uniform variable.
func GetUniformLocation(program uint32, name *uint8) int32 {
	n := goStr(name)
	if loc, ok := uniformCache[program][n]; ok {
		return loc
	}
	loc := addUniform(program, call("getUniformLocation", Programs.Get(program), n))
	if uniformCache[program] == nil {
		uniformCache[program] = map[string]int32{}
	}
	uniformCache[program][n] = loc
	return loc
}

// GetUniformIndices returns the indices of uniform variables.
func GetUniformIndices(program uint32, uniformCount int32, uniformNames **uint8, uniformIndices *uint32) {
	storeUint32(call("getUniformIndices", Programs.Get(program), stringsAny(cStrs(uniformCount, uniformNames, nil))), uniformIndices)
}

func activeInfo(info js.Value, bufSize int32, length *int32, size *int32, xtype *uint32, name *uint8) {
	if info.IsNull() || info.IsUndefined() {
		writeStr("", bufSize, length, name)
		return
	}
	writeStr(info.Get("name").String(), bufSize, length, name)
	if size != nil {
		*size = int32(info.Get("size").Int())
	}
	if xtype != nil {
		*xtype = uint32(info.Get("type").Int())
	}
}

// GetActiveAttrib returns information about an active attribute.
func GetActiveAttrib(program uint32, index uint32, bufSize int32, length *int32, size *int32, xtype *uint32, name *uint8) {
	activeInfo(call("getActiveAttrib", Programs.Get(program), index), bufSize, length, size, xtype, name)
}

// GetActiveUniform returns information about an active uniform.
func GetActiveUniform(program uint32, index uint32, bufSize int32, length *int32, size *int32, xtype *uint32, name *uint8) {
	activeInfo(call("getActiveUniform", Programs.Get(program), index), bufSize, length, size, xtype, name)
}

// GetTransformFeedbackVarying returns information about a transform feedback varying.
func GetTransformFeedbackVarying(program uint32, index uint32, bufSize int32, length *int32, size *int32, xtype *uint32, name *uint8) {
	activeInfo(call("getTransformFeedbackVarying", Programs.Get(program), index), bufSize, length, size, xtype, name)
}

// GetActiveUniformBlockName returns the name of a uniform block.
func GetActiveUniformBlockName(program uint32, uniformBlockIndex uint32, bufSize int32, length *int32, uniformBlockName *uint8) {
	writeStr(jsString(call("getActiveUniformBlockName", Programs.Get(program), uniformBlockIndex)), bufSize, length, uniformBlockName)
}

// GetActiveUniformBlockiv queries a uniform block parameter.
func GetActiveUniformBlockiv(program uint32, uniformBlockIndex uint32, pname uint32, params *int32) {
	if pname == glUniformBlockNameLength {
		*params = int32(len(jsString(call("getActiveUniformBlockName", Programs.Get(program), uniformBlockIndex))) + 1)
		return
	}
	storeInt32(call("getActiveUniformBlockParameter", Programs.Get(program), uniformBlockIndex, pname), params)
}

// GetActiveUniformsiv queries parameters of active uniforms.
func GetActiveUniformsiv(program uint32, uniformCount int32, uniformIndices *uint32, pname uint32, params *int32) {
	indices := unsafe.Slice(uniformIndices, uniformCount)
	if pname == glUniformNameLength {
		out := unsafe.Slice(params, uniformCount)
		for i, idx := range indices {
			out[i] = int32(len(activeName("getActiveUniform", program, idx))) + 1
		}
		return
	}
	storeInt32(call("getActiveUniforms", Programs.Get(program), uint32s(indices), pname), params)
}

// GetAttachedShaders returns the shaders attached to program.
func GetAttachedShaders(program uint32, maxCount int32, count *int32, shaders *uint32) {
	list := call("getAttachedShaders", Programs.Get(program))
	n := 0
	if list.Type() == js.TypeObject {
		n = min(list.Length(), int(maxCount))
		out := unsafe.Slice(shaders, n)
		for i := range out {
			out[i] = Shaders.ID(list.Index(i))
		}
	}
	if count != nil {
		*count = int32(n)
	}
}

func jsString(v js.Value) string {
	if v.Type() != js.TypeString {
		return ""
	}
	return v.String()
}

func activeName(method string, program uint32, index uint32) string {
	info := call(method, Programs.Get(program), index)
	if info.IsNull() || info.IsUndefined() {
		return ""
	}
	return info.Get("name").String()
}

// GetShaderInfoLog returns the information log of shader.
func GetShaderInfoLog(shader uint32, bufSize int32, length *int32, infoLog *uint8) {
	writeStr(jsString(call("getShaderInfoLog", Shaders.Get(shader))), bufSize, length, infoLog)
}

// GetShaderSource returns the source code of shader.
func GetShaderSource(shader uint32, bufSize int32, length *int32, source *uint8) {
	writeStr(jsString(call("getShaderSource", Shaders.Get(shader))), bufSize, length, source)
}

// GetProgramInfoLog returns the information log of program.
func GetProgramInfoLog(program uint32, bufSize int32, length *int32, infoLog *uint8) {
	writeStr(jsString(call("getProgramInfoLog", Programs.Get(program))), bufSize, length, infoLog)
}

func strLen(s string) int32 {
	if s == "" {
		return 0
	}
	return int32(len(s)) + 1
}

// GetShaderiv queries a shader parameter.
func GetShaderiv(shader uint32, pname uint32, params *int32) {
	switch pname {
	case glInfoLogLength:
		*params = strLen(jsString(call("getShaderInfoLog", Shaders.Get(shader))))
	case glShaderSourceLength:
		*params = strLen(jsString(call("getShaderSource", Shaders.Get(shader))))
	default:
		storeInt32(call("getShaderParameter", Shaders.Get(shader), pname), params)
	}
}

func maxNameLength(program uint32, countParam uint32, method string) int32 {
	n := int(Number(call("getProgramParameter", Programs.Get(program), countParam)))
	m := 0
	for i := range n {
		m = max(m, len(activeName(method, program, uint32(i)))+1)
	}
	return int32(m)
}

// GetProgramiv queries a program parameter.
func GetProgramiv(program uint32, pname uint32, params *int32) {
	switch pname {
	case glInfoLogLength:
		*params = strLen(jsString(call("getProgramInfoLog", Programs.Get(program))))
	case glActiveUniformMaxLength:
		*params = maxNameLength(program, glActiveUniforms, "getActiveUniform")
	case glActiveAttributeMaxLength:
		*params = maxNameLength(program, glActiveAttributes, "getActiveAttrib")
	case glTransformFeedbackVaryingMaxLen:
		*params = maxNameLength(program, glTransformFeedbackVaryings, "getTransformFeedbackVarying")
	case glActiveUniformBlockMaxNameLength:
		n := int(Number(call("getProgramParameter", Programs.Get(program), glActiveUniformBlocks)))
		m := 0
		for i := range n {
			m = max(m, len(jsString(call("getActiveUniformBlockName", Programs.Get(program), i)))+1)
		}
		*params = int32(m)
	case glProgramBinaryLength:
		*params = 0
	default:
		storeInt32(call("getProgramParameter", Programs.Get(program), pname), params)
	}
}

func parameter(pname uint32) js.Value {
	switch pname {
	case glNumExtensions:
		return js.ValueOf(len(extensionList()))
	case glMajorVersion:
		return js.ValueOf(3)
	case glMinorVersion, glNumShaderBinaryFormats, glNumProgramBinaryFormats:
		return js.ValueOf(0)
	}
	return call("getParameter", pname)
}

// GetIntegerv returns the value of a state variable.
func GetIntegerv(pname uint32, data *int32) { storeInt32(parameter(pname), data) }

// GetInteger64v returns the value of a state variable.
func GetInteger64v(pname uint32, data *int64) { storeInt64(parameter(pname), data) }

// GetFloatv returns the value of a state variable.
func GetFloatv(pname uint32, data *float32) { storeFloat32(parameter(pname), data) }

// GetBooleanv returns the value of a state variable.
func GetBooleanv(pname uint32, data *bool) { storeBool(parameter(pname), data) }

// GetIntegeri_v returns the value of an indexed state variable.
func GetIntegeri_v(target uint32, index uint32, data *int32) {
	storeInt32(call("getIndexedParameter", target, index), data)
}

// GetInteger64i_v returns the value of an indexed state variable.
func GetInteger64i_v(target uint32, index uint32, data *int64) {
	storeInt64(call("getIndexedParameter", target, index), data)
}

// GetBufferParameteriv queries a parameter of the bound buffer.
func GetBufferParameteriv(target uint32, pname uint32, params *int32) {
	if v, ok := mapParameter(target, pname); ok {
		*params = int32(v)
		return
	}
	storeInt32(call("getBufferParameter", target, pname), params)
}

// GetBufferParameteri64v queries a parameter of the bound buffer.
func GetBufferParameteri64v(target uint32, pname uint32, params *int64) {
	if v, ok := mapParameter(target, pname); ok {
		*params = v
		return
	}
	storeInt64(call("getBufferParameter", target, pname), params)
}

// GetFramebufferAttachmentParameteriv queries a framebuffer attachment parameter.
func GetFramebufferAttachmentParameteriv(target uint32, attachment uint32, pname uint32, params *int32) {
	storeInt32(call("getFramebufferAttachmentParameter", target, attachment, pname), params)
}

// GetRenderbufferParameteriv queries a parameter of the bound renderbuffer.
func GetRenderbufferParameteriv(target uint32, pname uint32, params *int32) {
	storeInt32(call("getRenderbufferParameter", target, pname), params)
}

// GetTexParameterfv queries a texture parameter.
func GetTexParameterfv(target uint32, pname uint32, params *float32) {
	storeFloat32(call("getTexParameter", target, pname), params)
}

// GetTexParameteriv queries a texture parameter.
func GetTexParameteriv(target uint32, pname uint32, params *int32) {
	storeInt32(call("getTexParameter", target, pname), params)
}

// GetSamplerParameterfv queries a sampler parameter.
func GetSamplerParameterfv(sampler uint32, pname uint32, params *float32) {
	storeFloat32(call("getSamplerParameter", Samplers.Get(sampler), pname), params)
}

// GetSamplerParameteriv queries a sampler parameter.
func GetSamplerParameteriv(sampler uint32, pname uint32, params *int32) {
	storeInt32(call("getSamplerParameter", Samplers.Get(sampler), pname), params)
}

// GetQueryiv queries the active query of target.
func GetQueryiv(target uint32, pname uint32, params *int32) {
	storeInt32(call("getQuery", target, pname), params)
}

// GetQueryObjectuiv queries a query object parameter.
func GetQueryObjectuiv(id uint32, pname uint32, params *uint32) {
	storeUint32(call("getQueryParameter", Queries.Get(id), pname), params)
}

// GetSynciv queries a sync object parameter.
func GetSynciv(sync uintptr, pname uint32, count int32, length *int32, values *int32) {
	n := 0
	if count > 0 {
		storeInt32(call("getSyncParameter", Syncs.Get(uint32(sync)), pname), values)
		n = 1
	}
	if length != nil {
		*length = int32(n)
	}
}

// GetInternalformativ queries an internal format parameter.
func GetInternalformativ(target uint32, internalformat uint32, pname uint32, count int32, params *int32) {
	vals := values(call("getInternalformatParameter", target, internalformat, pname))
	out := unsafe.Slice(params, min(len(vals), int(count)))
	for i := range out {
		out[i] = int32(vals[i])
	}
}

// GetShaderPrecisionFormat returns the range and precision of a shader numeric format.
func GetShaderPrecisionFormat(shadertype uint32, precisiontype uint32, xrange *int32, precision *int32) {
	f := call("getShaderPrecisionFormat", shadertype, precisiontype)
	if f.IsNull() || f.IsUndefined() {
		return
	}
	r := unsafe.Slice(xrange, 2)
	r[0] = int32(f.Get("rangeMin").Int())
	r[1] = int32(f.Get("rangeMax").Int())
	*precision = int32(f.Get("precision").Int())
}

// GetUniformfv returns the value of a uniform variable.
func GetUniformfv(program uint32, location int32, params *float32) {
	storeFloat32(call("getUniform", Programs.Get(program), Uniform(location)), params)
}

// GetUniformiv returns the value of a uniform variable.
func GetUniformiv(program uint32, location int32, params *int32) {
	storeInt32(call("getUniform", Programs.Get(program), Uniform(location)), params)
}

// GetUniformuiv returns the value of a uniform variable.
func GetUniformuiv(program uint32, location int32, params *uint32) {
	storeUint32(call("getUniform", Programs.Get(program), Uniform(location)), params)
}

// GetVertexAttribfv returns a vertex attribute parameter.
func GetVertexAttribfv(index uint32, pname uint32, params *float32) {
	storeFloat32(call("getVertexAttrib", index, pname), params)
}

// GetVertexAttribiv returns a vertex attribute parameter.
func GetVertexAttribiv(index uint32, pname uint32, params *int32) {
	storeInt32(call("getVertexAttrib", index, pname), params)
}

// GetVertexAttribIiv returns a vertex attribute parameter.
func GetVertexAttribIiv(index uint32, pname uint32, params *int32) {
	storeInt32(call("getVertexAttrib", index, pname), params)
}

// GetVertexAttribIuiv returns a vertex attribute parameter.
func GetVertexAttribIuiv(index uint32, pname uint32, params *uint32) {
	storeUint32(call("getVertexAttrib", index, pname), params)
}

// GetVertexAttribPointerv returns the offset of a vertex attribute array.
func GetVertexAttribPointerv(index uint32, pname uint32, pointer *unsafe.Pointer) {
	*(*uintptr)(unsafe.Pointer(pointer)) = uintptr(Number(call("getVertexAttribOffset", index, pname)))
}

// GetVertexAttribPointerWithOffsetv returns the offset of a vertex attribute array.
func GetVertexAttribPointerWithOffsetv(index uint32, pname uint32, offset **uintptr) {
	*(*uintptr)(unsafe.Pointer(offset)) = uintptr(Number(call("getVertexAttribOffset", index, pname)))
}

var extensionNames = map[string]string{
	"WEBGL_compressed_texture_s3tc":                  "GL_EXT_texture_compression_s3tc",
	"WEBGL_compressed_texture_s3tc_srgb":             "GL_EXT_texture_compression_s3tc_srgb",
	"WEBGL_compressed_texture_astc":                  "GL_KHR_texture_compression_astc_ldr",
	"WEBGL_compressed_texture_pvrtc":                 "GL_IMG_texture_compression_pvrtc",
	"WEBGL_compressed_texture_etc1":                  "GL_OES_compressed_ETC1_RGB8_texture",
	"WEBGL_compressed_texture_etc":                   "GL_OES_compressed_ETC2_RGB8_texture",
	"WEBGL_multi_draw":                               "GL_EXT_multi_draw_arrays",
	"WEBGL_draw_instanced_base_vertex_base_instance": "GL_EXT_base_instance",
	"EXT_disjoint_timer_query_webgl2":                "GL_EXT_disjoint_timer_query",
	"WEBGL_clip_cull_distance":                       "GL_EXT_clip_cull_distance",
	"WEBGL_blend_func_extended":                      "GL_EXT_blend_func_extended",
	"WEBGL_polygon_mode":                             "GL_NV_polygon_mode",
	"WEBGL_debug_renderer_info":                      "",
	"WEBGL_debug_shaders":                            "",
	"WEBGL_lose_context":                             "",
}

func extensionList() []string {
	if extensions == nil {
		extensions = []string{}
		list := call("getSupportedExtensions")
		if list.Type() == js.TypeObject {
			for i := range list.Length() {
				name := list.Index(i).String()
				gl, ok := extensionNames[name]
				if !ok {
					gl = "GL_" + name
				}
				if gl != "" {
					extensions = append(extensions, gl)
				}
			}
		}
	}
	return extensions
}

func cString(key uint32, s string) *uint8 {
	b, ok := cstrings[key]
	if !ok {
		b = append([]byte(s), 0)
		cstrings[key] = b
	}
	return &b[0]
}

// GetString returns a string describing the current connection.
func GetString(name uint32) *uint8 {
	switch name {
	case glExtensions:
		return cString(name, strings.Join(extensionList(), " "))
	case glVersion:
		return cString(name, "OpenGL ES 3.0 "+jsString(call("getParameter", name)))
	case glShadingLanguageVersion:
		return cString(name, "OpenGL ES GLSL ES 3.00 "+jsString(call("getParameter", name)))
	}
	v := call("getParameter", name)
	if v.Type() != js.TypeString {
		return nil
	}
	return cString(name, v.String())
}

// GetStringi returns an indexed string.
func GetStringi(name uint32, index uint32) *uint8 {
	list := extensionList()
	if name != glExtensions || int(index) >= len(list) {
		setError(0x0501)
		return nil
	}
	return cString(1<<31|index, list[index])
}
