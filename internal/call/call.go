//go:build ((amd64 || arm64) && (darwin || freebsd || linux || netbsd || windows)) || (amd64 && haiku) || (386 && (linux || windows)) || (arm && linux)

// Package call calls C functions with integer and floating-point arguments.
//
//go:generate go run gen.go
package call

import (
	"math"
	"runtime"
	"sync"
	"unsafe"
)

// Argument kinds, four bits per argument in a descriptor.
const (
	I8  = 0
	I16 = 1
	I32 = 2
	I64 = 3
	PTR = 4
	F32 = 6
	F64 = 7
)

const (
	maxArgs  = 32
	maxStack = 32
	split    = unsafe.Sizeof(uintptr(0)) == 4
)

type frame struct {
	fn     uintptr
	ints   [8]uintptr
	floats [8]uint64
	stack  [maxStack]uintptr
	nstack uintptr
	fret   uintptr
	r1     uintptr
	r2     uintptr
	f1     uint64
}

var pool = sync.Pool{New: func() any { return new(frame) }}

var trampolineABI0 uintptr

//go:linkname runtime_cgocall runtime.cgocall
func runtime_cgocall(fn uintptr, arg unsafe.Pointer) int32

func kindOf(desc [2]uint64, i int) uint64 {
	return desc[i/16] >> (4 * (i % 16)) & 0xf
}

func isFloat(k uint64) bool { return k == F32 || k == F64 }

func sizeOf(k uint64) uintptr {
	switch k {
	case I8:
		return 1
	case I16:
		return 2
	case I32, F32:
		return 4
	case PTR:
		return unsafe.Sizeof(uintptr(0))
	}
	return 8
}

func (f *frame) push(a uintptr, ns *int) {
	if *ns == maxStack {
		panic("call: too many stack arguments")
	}
	f.stack[*ns] = a
	*ns++
}

func (f *frame) arrange(desc [2]uint64, args []uintptr) {
	if len(args) > maxArgs {
		panic("call: too many arguments")
	}
	switch {
	case runtime.GOARCH == "386":
		f.nstack = uintptr(copy(f.stack[:], args))
	case runtime.GOARCH == "arm":
		f.arrangeARM(desc, args)
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		for i, a := range args {
			if i < 4 {
				f.ints[i] = a
				f.floats[i] = uint64(a)
			} else {
				f.stack[i-4] = a
			}
		}
		f.nstack = uintptr(max(len(args)-4, 0))
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		ni, nf, off := 0, 0, uintptr(0)
		for i, a := range args {
			k := kindOf(desc, i)
			if isFloat(k) {
				if nf < 8 {
					f.floats[nf] = uint64(a)
					nf++
					continue
				}
			} else if ni < 8 {
				f.ints[ni] = a
				ni++
				continue
			}
			size := sizeOf(k)
			off = (off + size - 1) &^ (size - 1)
			if off+size > maxStack*8 {
				panic("call: too many stack arguments")
			}
			p := unsafe.Add(unsafe.Pointer(&f.stack), off)
			switch size {
			case 1:
				*(*uint8)(p) = uint8(a)
			case 2:
				*(*uint16)(p) = uint16(a)
			case 4:
				*(*uint32)(p) = uint32(a)
			default:
				*(*uint64)(p) = uint64(a)
			}
			off += size
		}
		f.nstack = (off + 7) / 8
	default:
		nint := 8
		if runtime.GOARCH == "amd64" {
			nint = 6
		}
		ni, nf, ns := 0, 0, 0
		for i, a := range args {
			if isFloat(kindOf(desc, i)) {
				if nf < 8 {
					f.floats[nf] = uint64(a)
					nf++
					continue
				}
			} else if ni < nint {
				f.ints[ni] = a
				ni++
				continue
			}
			f.push(a, &ns)
		}
		f.nstack = uintptr(ns)
	}
}

func (f *frame) arrangeARM(desc [2]uint64, args []uintptr) {
	ncrn, ns := 0, 0
	sfree := uint32(0xffff)
	vfpStack := false
	for p, w := 0, 0; w < len(args); p++ {
		switch k := kindOf(desc, p); k {
		case F32:
			if !vfpStack && sfree != 0 {
				i := 0
				for sfree&(1<<i) == 0 {
					i++
				}
				sfree &^= 1 << i
				shift := 32 * (i % 2)
				f.floats[i/2] = f.floats[i/2]&^(0xffffffff<<shift) | uint64(uint32(args[w]))<<shift
				w++
				continue
			}
			vfpStack = true
			f.push(args[w], &ns)
			w++
		case F64:
			if !vfpStack {
				j := 0
				for j < 8 && sfree&(3<<(2*j)) != 3<<(2*j) {
					j++
				}
				if j < 8 {
					sfree &^= 3 << (2 * j)
					f.floats[j] = uint64(uint32(args[w])) | uint64(uint32(args[w+1]))<<32
					w += 2
					continue
				}
			}
			vfpStack = true
			ns = (ns + 1) &^ 1
			f.push(args[w], &ns)
			f.push(args[w+1], &ns)
			w += 2
		case I64:
			ncrn = (ncrn + 1) &^ 1
			if ncrn <= 2 {
				f.ints[ncrn], f.ints[ncrn+1] = args[w], args[w+1]
				ncrn += 2
			} else {
				ncrn = 4
				ns = (ns + 1) &^ 1
				f.push(args[w], &ns)
				f.push(args[w+1], &ns)
			}
			w += 2
		default:
			if ncrn < 4 {
				f.ints[ncrn] = args[w]
				ncrn++
			} else {
				f.push(args[w], &ns)
			}
			w++
		}
	}
	f.nstack = uintptr(ns)
}

func do(fn uintptr, desc [2]uint64, args []uintptr, fret bool) *frame {
	if fn == 0 {
		panic("call: function not loaded")
	}
	f := pool.Get().(*frame)
	*f = frame{fn: fn}
	if fret {
		f.fret = 1
	}
	f.arrange(desc, args)
	runtime_cgocall(trampolineABI0, unsafe.Pointer(f))
	return f
}

func ret(f *frame) (uintptr, uint64, uint64) {
	r, x := f.r1, f.f1
	w := uint64(r)
	if split {
		w |= uint64(f.r2) << 32
	}
	pool.Put(f)
	return r, x, w
}

// F32Bits returns the float32 bits of a floating-point result returned by FCall.
func F32Bits(bits uint64) uint32 {
	if runtime.GOARCH == "386" {
		return math.Float32bits(float32(math.Float64frombits(bits)))
	}
	return uint32(bits)
}
