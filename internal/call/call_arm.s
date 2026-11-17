//go:build linux

#include "textflag.h"
#include "go_asm.h"

GLOBL ·trampolineABI0(SB), NOPTR|RODATA, $4
DATA ·trampolineABI0(SB)/4, $trampoline(SB)

TEXT trampoline(SB), NOSPLIT|NOFRAME, $0
	MOVM.DB.W [R4-R9, R14], (R13)
	MOVW R0, R4
	MOVW R13, R5
	SUB  $128, R13
	BIC  $7, R13

	ADD  $frame_stack, R4, R6
	MOVW R13, R7
	MOVW frame_nstack(R4), R8

copy:
	CMP     $0, R8
	BEQ     copied
	MOVW.P  4(R6), R9
	MOVW.P  R9, 4(R7)
	SUB     $1, R8
	B       copy

copied:
	MOVD (frame_floats+0)(R4), F0
	MOVD (frame_floats+8)(R4), F1
	MOVD (frame_floats+16)(R4), F2
	MOVD (frame_floats+24)(R4), F3
	MOVD (frame_floats+32)(R4), F4
	MOVD (frame_floats+40)(R4), F5
	MOVD (frame_floats+48)(R4), F6
	MOVD (frame_floats+56)(R4), F7

	MOVW (frame_ints+0)(R4), R0
	MOVW (frame_ints+4)(R4), R1
	MOVW (frame_ints+8)(R4), R2
	MOVW (frame_ints+12)(R4), R3

	MOVW frame_fn(R4), R12
	BL   (R12)

	MOVW R0, frame_r1(R4)
	MOVW R1, frame_r2(R4)
	MOVD F0, frame_f1(R4)

	MOVW R5, R13
	MOVM.IA.W (R13), [R4-R9, R15]
