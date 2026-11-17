//go:build darwin || freebsd || linux || netbsd || windows

#include "textflag.h"
#include "go_asm.h"

GLOBL ·trampolineABI0(SB), NOPTR|RODATA, $8
DATA ·trampolineABI0(SB)/8, $trampoline(SB)

TEXT trampoline(SB), NOSPLIT|NOFRAME, $0
	STP.W (R29, R30), -16(RSP)
	MOVD  RSP, R29
	STP.W (R19, R20), -16(RSP)
	MOVD  R0, R19
	SUB   $256, RSP

	ADD  $frame_stack, R19, R9
	MOVD RSP, R10
	MOVD frame_nstack(R19), R11
	ADD  $1, R11
	LSR  $1, R11
	CBZ  R11, copied

copy:
	LDP.P 16(R9), (R12, R13)
	STP.P (R12, R13), 16(R10)
	SUBS  $1, R11
	BNE   copy

copied:
	FMOVD (frame_floats+0)(R19), F0
	FMOVD (frame_floats+8)(R19), F1
	FMOVD (frame_floats+16)(R19), F2
	FMOVD (frame_floats+24)(R19), F3
	FMOVD (frame_floats+32)(R19), F4
	FMOVD (frame_floats+40)(R19), F5
	FMOVD (frame_floats+48)(R19), F6
	FMOVD (frame_floats+56)(R19), F7

	MOVD (frame_ints+0)(R19), R0
	MOVD (frame_ints+8)(R19), R1
	MOVD (frame_ints+16)(R19), R2
	MOVD (frame_ints+24)(R19), R3
	MOVD (frame_ints+32)(R19), R4
	MOVD (frame_ints+40)(R19), R5
	MOVD (frame_ints+48)(R19), R6
	MOVD (frame_ints+56)(R19), R7

	MOVD frame_fn(R19), R16
	BL   (R16)

	MOVD  R0, frame_r1(R19)
	FMOVD F0, frame_f1(R19)

	ADD   $256, RSP
	LDP.P 16(RSP), (R19, R20)
	LDP.P 16(RSP), (R29, R30)
	RET
