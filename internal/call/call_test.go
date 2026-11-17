//go:build ((amd64 || arm64) && (darwin || freebsd || linux || netbsd || windows)) || (386 && (linux || windows)) || (arm && linux)

package call

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/gen2brain/gl/internal/dl"
)

type arg struct {
	k uint64
	v uint64
}

func i(k uint64, v int64) arg  { return arg{k, uint64(v)} }
func f32(v float32) arg        { return arg{F32, uint64(math.Float32bits(v))} }
func f64(v float64) arg        { return arg{F64, math.Float64bits(v)} }
func ptr(p unsafe.Pointer) arg { return arg{PTR, uint64(uintptr(p))} }

func invoke(fn uintptr, fret bool, args ...arg) (uintptr, uint64, uint64) {
	var d [2]uint64
	var w []uintptr
	for n, a := range args {
		d[n/16] |= a.k << (4 * (n % 16))
		if split && (a.k == I64 || a.k == F64) {
			w = append(w, uintptr(a.v), uintptr(a.v>>32))
		} else {
			w = append(w, uintptr(a.v))
		}
	}
	return ret(do(fn, d, w, fret))
}

func loadLib(t testing.TB) uintptr {
	lib := os.Getenv("CALL_TESTLIB")
	if lib == "" {
		cc := os.Getenv("CC")
		if cc == "" {
			cc = "cc"
		}
		if _, err := exec.LookPath(cc); err != nil {
			t.Skip("no C compiler and CALL_TESTLIB not set")
		}
		lib = filepath.Join(t.TempDir(), "libabi.so")
		out, err := exec.Command(cc, "-shared", "-fPIC", "-O2", "-o", lib, filepath.Join("testdata", "abi.c")).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	h, err := dl.Open(lib)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dl.Close(h) })
	return h
}

func sym(t testing.TB, h uintptr, name string) uintptr {
	p := dl.Sym(h, name)
	if p == 0 {
		t.Fatalf("missing %s", name)
	}
	return p
}

func TestCall(t *testing.T) {
	h := loadLib(t)

	t.Run("ints15", func(t *testing.T) {
		var buf [4]byte
		p := unsafe.Pointer(&buf[0])
		args := []arg{i(I8, 200), i(I16, -3000), i(I32, -70000), i(I64, -5000000000), i(I32, 4000000000), i(I8, 7),
			i(I16, 65000), i(I32, -9), i(I8, 255), i(I32, 123456), i(I8, 1), i(I16, -32000), i(I64, 9000000000),
			i(I32, 77), ptr(p)}
		want := int64(0)
		for n, a := range args[:14] {
			v := int64(a.v)
			if a.k == I32 && n != 4 {
				v = int64(int32(a.v))
			}
			want += int64(n+1) * v
		}
		want += 15 * int64(uintptr(p))
		r, _, w := invoke(sym(t, h, "ints15"), false, args...)
		runtime.KeepAlive(&buf)
		if got := int64(w); got != want {
			t.Errorf("got %d (r1 %#x), want %d", got, r, want)
		}
	})

	t.Run("texsubimage3d", func(t *testing.T) {
		vals := []int64{0x806F, 2, 3, -4, 5, 600, 700, 8, 0x1908, 0x1401}
		want := int64(0)
		var args []arg
		for n, v := range vals {
			want += int64(n+1) * v
			args = append(args, i(I32, v))
		}
		args = append(args, i(PTR, 0x1234568))
		want += 11 * 0x1234568
		_, _, w := invoke(sym(t, h, "texsubimage3d"), false, args...)
		if got := int64(w); got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		_, x, _ := invoke(sym(t, h, "mixed"), true,
			i(I32, -1), f32(2.5), i(I32, 3), f64(4.25), i(I8, 5), f32(6.5), i(I64, -7), f32(8), f32(9), f32(10),
			f32(11), f32(12), f32(13), i(I32, 14), f32(15.5))
		want := 1*-1.0 + 2*2.5 + 3*3 + 4*4.25 + 5*5 + 6*6.5 + 7*-7.0 + 8*8 + 9*9 + 10*10 + 11*11 + 12*12 + 13*13 + 14*14 + 15*15.5
		if got := math.Float64frombits(x); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("many23", func(t *testing.T) {
		_, x, _ := invoke(sym(t, h, "many23"), true,
			i(I32, -1), f32(2.5), i(I8, 3), f64(4.25), i(I16, -5), f32(6.5), i(I64, -7), f32(8), i(I32, 9), f32(10),
			i(I32, -11), f32(12), i(I8, 13), f32(14), i(I32, 15), f32(16), i(I16, -17), f32(18), i(I8, 19), f64(20.5),
			i(I32, -21), f32(22), i(I8, 23))
		want := 1*-1.0 + 2*2.5 + 3*3 + 4*4.25 + 5*-5.0 + 6*6.5 + 7*-7.0 + 8*8 + 9*9 + 10*10 + 11*-11.0 + 12*12 +
			13*13 + 14*14 + 15*15 + 16*16 + 17*-17.0 + 18*18 + 19*19 + 20*20.5 + 21*-21.0 + 22*22 + 23*23
		if got := math.Float64frombits(x); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("uniform1f", func(t *testing.T) {
		_, x, _ := invoke(sym(t, h, "uniform1f"), true, i(I32, 3), f32(0.5))
		if got := math.Float32frombits(F32Bits(x)); got != 3000.5 {
			t.Errorf("got %v", got)
		}
	})

	t.Run("ret64", func(t *testing.T) {
		_, _, w := invoke(sym(t, h, "ret64"), false, i(I32, 5), i(I64, 0x123456789), i(I32, -1))
		if w != 0x123456789*2+4 {
			t.Errorf("got %#x", w)
		}
	})

	t.Run("isenabled", func(t *testing.T) {
		fn := sym(t, h, "isenabled")
		if got := uint8(Call1(fn, I32, 0x0B71)); got != 1 {
			t.Errorf("got %d", got)
		}
		if got := uint8(Call1(fn, I32, 0x0B72)); got != 0 {
			t.Errorf("got %d", got)
		}
	})

	t.Run("ptrret", func(t *testing.T) {
		x := new(int64)
		if got := Call1(sym(t, h, "ptrret"), PTR, uintptr(unsafe.Pointer(x))); got != uintptr(unsafe.Pointer(x)) {
			t.Errorf("got %x", got)
		}
	})
}

func BenchmarkCall(b *testing.B) {
	if os.Getenv("CALL_TESTLIB") == "" {
		b.Skip("CALL_TESTLIB not set")
	}
	fn := sym(b, loadLib(b), "uniform1f")
	b.ReportAllocs()
	for b.Loop() {
		FCall2(fn, I32|F32<<4, 3, uintptr(math.Float32bits(0.5)))
	}
}
