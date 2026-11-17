//go:build darwin || freebsd || linux || netbsd || windows

#include "textflag.h"
#include "go_asm.h"

#ifdef GOOS_windows
#define SHADOW 32
#else
#define SHADOW 0
#endif

#define STACK_SIZE (SHADOW + 256)

GLOBL ·trampolineABI0(SB), NOPTR|RODATA, $8
DATA ·trampolineABI0(SB)/8, $trampoline(SB)

TEXT trampoline(SB), NOSPLIT|NOFRAME, $0
	PUSHQ BP
	MOVQ  SP, BP
	PUSHQ BX
#ifdef GOOS_windows
	MOVQ CX, BX
#else
	MOVQ DI, BX
#endif
	SUBQ $STACK_SIZE, SP
	ANDQ $~15, SP

	LEAQ frame_stack(BX), SI
	LEAQ SHADOW(SP), DI
	MOVQ frame_nstack(BX), CX
	REP; MOVSQ

	MOVQ (frame_floats+0)(BX), X0
	MOVQ (frame_floats+8)(BX), X1
	MOVQ (frame_floats+16)(BX), X2
	MOVQ (frame_floats+24)(BX), X3
	MOVQ (frame_floats+32)(BX), X4
	MOVQ (frame_floats+40)(BX), X5
	MOVQ (frame_floats+48)(BX), X6
	MOVQ (frame_floats+56)(BX), X7

#ifdef GOOS_windows
	MOVQ (frame_ints+0)(BX), CX
	MOVQ (frame_ints+8)(BX), DX
	MOVQ (frame_ints+16)(BX), R8
	MOVQ (frame_ints+24)(BX), R9
#else
	MOVQ (frame_ints+0)(BX), DI
	MOVQ (frame_ints+8)(BX), SI
	MOVQ (frame_ints+16)(BX), DX
	MOVQ (frame_ints+24)(BX), CX
	MOVQ (frame_ints+32)(BX), R8
	MOVQ (frame_ints+40)(BX), R9
	MOVL $8, AX
#endif

	MOVQ frame_fn(BX), R10
	CALL R10

	MOVQ AX, frame_r1(BX)
	MOVQ X0, frame_f1(BX)

	LEAQ -8(BP), SP
	POPQ BX
	POPQ BP
	RET
