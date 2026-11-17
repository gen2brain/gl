//go:build js

package webgl

import (
	"syscall/js"
	"unsafe"
)

var (
	heapSize int
	heapU8   js.Value
	views    map[string]js.Value
)

func grow(n int) {
	if n <= heapSize {
		return
	}
	size := max(n, 2*heapSize, 1<<16)
	size = (size + 7) &^ 7
	buf := js.Global().Get("ArrayBuffer").New(size)
	views = map[string]js.Value{}
	for _, t := range []string{"Uint8Array", "Int8Array", "Uint16Array", "Int16Array", "Uint32Array", "Int32Array", "Float32Array"} {
		views[t] = js.Global().Get(t).New(buf)
	}
	heapU8 = views["Uint8Array"]
	heapSize = size
}

func upload(p unsafe.Pointer, n int) {
	grow(n)
	if n > 0 {
		js.CopyBytesToJS(heapU8, unsafe.Slice((*byte)(p), n))
	}
}

func download(p unsafe.Pointer, n int) {
	if n > 0 {
		js.CopyBytesToGo(unsafe.Slice((*byte)(p), n), heapU8)
	}
}

func list(view string, size int, p unsafe.Pointer, n int) js.Value {
	upload(p, n*size)
	return views[view].Call("subarray", 0, n)
}

// Float32List copies n float32 values at p to a Float32Array.
func Float32List(p unsafe.Pointer, n int) js.Value { return list("Float32Array", 4, p, n) }

// Int32List copies n int32 values at p to an Int32Array.
func Int32List(p unsafe.Pointer, n int) js.Value { return list("Int32Array", 4, p, n) }

// Uint32List copies n uint32 values at p to a Uint32Array.
func Uint32List(p unsafe.Pointer, n int) js.Value { return list("Uint32Array", 4, p, n) }

const (
	glByte                     = 0x1400
	glUnsignedByte             = 0x1401
	glShort                    = 0x1402
	glUnsignedShort            = 0x1403
	glInt                      = 0x1404
	glUnsignedInt              = 0x1405
	glFloat                    = 0x1406
	glHalfFloat                = 0x140B
	glUnsignedShort4444        = 0x8033
	glUnsignedShort5551        = 0x8034
	glUnsignedShort565         = 0x8363
	glUnsignedInt2101010Rev    = 0x8368
	glUnsignedInt10F11F11FRev  = 0x8C3B
	glUnsignedInt5999Rev       = 0x8C3E
	glUnsignedInt248           = 0x84FA
	glFloat32UnsignedInt248Rev = 0x8DAD

	glRed            = 0x1903
	glRedInteger     = 0x8D94
	glAlpha          = 0x1906
	glLuminance      = 0x1909
	glDepthComponent = 0x1902
	glRG             = 0x8227
	glRGInteger      = 0x8228
	glLuminanceAlpha = 0x190A
	glDepthStencil   = 0x84F9
	glRGB            = 0x1907
	glRGBInteger     = 0x8D98

	glPackRowLength     = 0x0D02
	glPackSkipRows      = 0x0D03
	glPackSkipPixels    = 0x0D04
	glPackAlignment     = 0x0D05
	glUnpackRowLength   = 0x0CF2
	glUnpackSkipRows    = 0x0CF3
	glUnpackSkipPixels  = 0x0CF4
	glUnpackAlignment   = 0x0CF5
	glUnpackImageHeight = 0x806E
	glUnpackSkipImages  = 0x806D

	glPixelPackBuffer   = 0x88EB
	glPixelUnpackBuffer = 0x88EC
)

func typeView(xtype uint32) (string, int) {
	switch xtype {
	case glByte:
		return "Int8Array", 1
	case glShort:
		return "Int16Array", 2
	case glUnsignedShort, glHalfFloat, glUnsignedShort4444, glUnsignedShort5551, glUnsignedShort565:
		return "Uint16Array", 2
	case glInt:
		return "Int32Array", 4
	case glUnsignedInt, glUnsignedInt2101010Rev, glUnsignedInt10F11F11FRev, glUnsignedInt5999Rev, glUnsignedInt248,
		glFloat32UnsignedInt248Rev:
		return "Uint32Array", 4
	case glFloat:
		return "Float32Array", 4
	}
	return "Uint8Array", 1
}

func pixelSize(format, xtype uint32) int {
	switch xtype {
	case glUnsignedShort4444, glUnsignedShort5551, glUnsignedShort565:
		return 2
	case glUnsignedInt2101010Rev, glUnsignedInt10F11F11FRev, glUnsignedInt5999Rev, glUnsignedInt248:
		return 4
	case glFloat32UnsignedInt248Rev:
		return 8
	}
	n := 4
	switch format {
	case glRed, glRedInteger, glAlpha, glLuminance, glDepthComponent:
		n = 1
	case glRG, glRGInteger, glLuminanceAlpha, glDepthStencil:
		n = 2
	case glRGB, glRGBInteger:
		n = 3
	}
	_, size := typeView(xtype)
	return n * size
}

type pixelStore struct {
	alignment, rowLength, imageHeight, skipRows, skipPixels, skipImages int
}

var (
	pack, unpack             pixelStore
	packBuffer, unpackBuffer uint32
	cstrings                 map[uint32][]byte
	extensions               []string
	uniformCache             map[uint32]map[string]int32
)

func resetState() {
	pack = pixelStore{alignment: 4}
	unpack = pixelStore{alignment: 4}
	packBuffer, unpackBuffer = 0, 0
	cstrings = map[uint32][]byte{}
	extensions = nil
	uniformCache = map[uint32]map[string]int32{}
	mappings = map[uint32]*mapping{}
	heapSize = 0
}

func (s *pixelStore) size(w, h, d int, format, xtype uint32) int {
	if w <= 0 || h <= 0 || d <= 0 {
		return 0
	}
	bpp := pixelSize(format, xtype)
	rowLen := w
	if s.rowLength > 0 {
		rowLen = s.rowLength
	}
	a := max(s.alignment, 1)
	rowBytes := (rowLen*bpp + a - 1) / a * a
	imgHeight := h
	if s.imageHeight > 0 {
		imgHeight = s.imageHeight
	}
	imageBytes := rowBytes * imgHeight
	return s.skipImages*imageBytes + s.skipRows*rowBytes + s.skipPixels*bpp +
		(d-1)*imageBytes + (h-1)*rowBytes + w*bpp
}

func goStr(p *uint8) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*uint8)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

func goStrN(p *uint8, n int) string {
	if p == nil || n <= 0 {
		return ""
	}
	return string(unsafe.Slice(p, n))
}

func cStrs(count int32, strs **uint8, lengths *int32) []string {
	ptrs := unsafe.Slice(strs, count)
	var lens []int32
	if lengths != nil {
		lens = unsafe.Slice(lengths, count)
	}
	out := make([]string, count)
	for i, p := range ptrs {
		if lens != nil && lens[i] >= 0 {
			out[i] = goStrN(p, int(lens[i]))
		} else {
			out[i] = goStr(p)
		}
	}
	return out
}

func writeStr(s string, bufSize int32, length *int32, dst *uint8) {
	n := 0
	if bufSize > 0 && dst != nil {
		n = min(len(s), int(bufSize)-1)
		out := unsafe.Slice(dst, n+1)
		copy(out, s[:n])
		out[n] = 0
	}
	if length != nil {
		*length = int32(n)
	}
}

func values(v js.Value) []float64 {
	switch v.Type() {
	case js.TypeNumber:
		return []float64{v.Float()}
	case js.TypeBoolean:
		if v.Bool() {
			return []float64{1}
		}
		return []float64{0}
	case js.TypeObject:
		if id := v.Get(idKey); id.Type() == js.TypeNumber {
			return []float64{id.Float()}
		}
		if l := v.Get("length"); l.Type() == js.TypeNumber {
			out := make([]float64, l.Int())
			for i := range out {
				out[i] = Number(v.Index(i))
			}
			return out
		}
	}
	return []float64{0}
}

func storeInt32(v js.Value, p *int32) {
	vals := values(v)
	out := unsafe.Slice(p, len(vals))
	for i, f := range vals {
		out[i] = int32(int64(f))
	}
}

func storeUint32(v js.Value, p *uint32) {
	vals := values(v)
	out := unsafe.Slice(p, len(vals))
	for i, f := range vals {
		out[i] = uint32(int64(f))
	}
}

func storeInt64(v js.Value, p *int64) {
	vals := values(v)
	out := unsafe.Slice(p, len(vals))
	for i, f := range vals {
		out[i] = int64(f)
	}
}

func storeFloat32(v js.Value, p *float32) {
	vals := values(v)
	out := unsafe.Slice(p, len(vals))
	for i, f := range vals {
		out[i] = float32(f)
	}
}

func storeBool(v js.Value, p *bool) {
	vals := values(v)
	out := unsafe.Slice(p, len(vals))
	for i, f := range vals {
		out[i] = f != 0
	}
}
