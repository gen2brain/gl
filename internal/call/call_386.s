//go:build linux || windows

#include "textflag.h"
#include "go_asm.h"

GLOBL ·trampolineABI0(SB), NOPTR|RODATA, $4
DATA ·trampolineABI0(SB)/4, $trampoline(SB)

TEXT trampoline(SB), NOSPLIT|NOFRAME, $0
	PUSHL BP
	MOVL  SP, BP
	PUSHL BX
	PUSHL SI
	PUSHL DI
	MOVL  8(BP), BX
	SUBL  $128, SP
	ANDL  $~15, SP

	LEAL frame_stack(BX), SI
	MOVL SP, DI
	MOVL frame_nstack(BX), CX
	REP; MOVSL

	MOVL frame_fn(BX), AX
	CALL AX

	MOVL AX, frame_r1(BX)
	MOVL DX, frame_r2(BX)
	CMPL frame_fret(BX), $0
	JEQ  done
	FMOVDP F0, frame_f1(BX)

done:
	LEAL -12(BP), SP
	POPL DI
	POPL SI
	POPL BX
	POPL BP
	RET
