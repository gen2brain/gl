//go:build haiku

#include "_cgo_export.h"

#define TRAMPOLINE(n) \
	static void trampoline##n(unsigned int s, unsigned int t, unsigned int i, unsigned int v, int l, const char *m, const void *u) { \
		goglDebug(n, s, t, i, v, l, (char *)m, (void *)u); \
	}

TRAMPOLINE(0)
TRAMPOLINE(1)
TRAMPOLINE(2)
TRAMPOLINE(3)
TRAMPOLINE(4)
TRAMPOLINE(5)
TRAMPOLINE(6)
TRAMPOLINE(7)

static void *trampolines[] = {
	trampoline0, trampoline1, trampoline2, trampoline3,
	trampoline4, trampoline5, trampoline6, trampoline7,
};

void *goglDebugTrampoline(int n) {
	return trampolines[n];
}
