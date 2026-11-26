//go:build darwin && arm64

#include "textflag.h"

// fcallX calls a C function for FCaller.Call. runtime.cgocall runs it on
// the g0 stack with R0 set to the *FCaller. The offsets are the FCaller
// layout in fcall_arm64.go; TestFCallerLayout keeps the two the same.
//
//	fn    0
//	X     8   (8 words)
//	D     72  (8 doubles)
//	R0    136
//	RD    144 (4 doubles)
//	S     176 (2 words)
//
// AAPCS64 puts integer and pointer arguments in X0-X7 and floating-point
// arguments in D0-D7, each class counted on its own. A float or double
// result is in D0. A struct of up to four floats or doubles (an HFA, for
// example NSPoint, NSSize and NSRect) comes back in D0-D3. purego's own
// trampoline saves D0-D3 too, but SyscallN returns only X0 and X1.
//
// An integer argument after the eighth goes on the stack. S holds two of
// them, at SP+0 and SP+8 when the callee starts. Apple's arm64 ABI packs
// stack arguments by their natural size, so each S slot is one argument
// only when the arguments there are 8 bytes wide.
GLOBL ·fcallABI0(SB), NOPTR|RODATA, $8
DATA ·fcallABI0(SB)/8, $fcallX(SB)

// The assembler saves LR for a function that calls, as purego's
// syscallX also needs. SP stays 16-byte aligned for the C call.
TEXT fcallX(SB), NOSPLIT, $0
	SUB  $32, RSP
	MOVD R0, 16(RSP) // keep the *FCaller across the call
	MOVD R0, R9

	MOVD 176(R9), R10
	MOVD R10, 0(RSP) // stack argument 1
	MOVD 184(R9), R10
	MOVD R10, 8(RSP) // stack argument 2

	FMOVD 72(R9), F0
	FMOVD 80(R9), F1
	FMOVD 88(R9), F2
	FMOVD 96(R9), F3
	FMOVD 104(R9), F4
	FMOVD 112(R9), F5
	FMOVD 120(R9), F6
	FMOVD 128(R9), F7

	MOVD 8(R9), R0
	MOVD 16(R9), R1
	MOVD 24(R9), R2
	MOVD 32(R9), R3
	MOVD 40(R9), R4
	MOVD 48(R9), R5
	MOVD 56(R9), R6
	MOVD 64(R9), R7

	MOVD 0(R9), R10
	BL   (R10)

	MOVD 16(RSP), R2
	ADD  $32, RSP

	MOVD  R0, 136(R2)
	FMOVD F0, 144(R2)
	FMOVD F1, 152(R2)
	FMOVD F2, 160(R2)
	FMOVD F3, 168(R2)
	RET
