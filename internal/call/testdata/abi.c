#include <stdint.h>

#ifdef _WIN32
#define EXPORT __declspec(dllexport)
#else
#define EXPORT
#endif

EXPORT int64_t ints15(uint8_t a1, int16_t a2, int32_t a3, int64_t a4, uint32_t a5, uint8_t a6, uint16_t a7, int32_t a8,
                      uint8_t a9, uint32_t a10, uint8_t a11, int16_t a12, int64_t a13, uint32_t a14, void *a15) {
	return 1 * (int64_t)a1 + 2 * (int64_t)a2 + 3 * (int64_t)a3 + 4 * a4 + 5 * (int64_t)a5 + 6 * (int64_t)a6 +
	       7 * (int64_t)a7 + 8 * (int64_t)a8 + 9 * (int64_t)a9 + 10 * (int64_t)a10 + 11 * (int64_t)a11 +
	       12 * (int64_t)a12 + 13 * a13 + 14 * (int64_t)a14 + 15 * (int64_t)(intptr_t)a15;
}

EXPORT int64_t texsubimage3d(uint32_t target, int32_t level, int32_t x, int32_t y, int32_t z, int32_t w, int32_t h,
                             int32_t d, uint32_t format, uint32_t type, const void *pixels) {
	return 1 * (int64_t)target + 2 * (int64_t)level + 3 * (int64_t)x + 4 * (int64_t)y + 5 * (int64_t)z +
	       6 * (int64_t)w + 7 * (int64_t)h + 8 * (int64_t)d + 9 * (int64_t)format + 10 * (int64_t)type +
	       11 * (int64_t)(intptr_t)pixels;
}

EXPORT double mixed(int32_t a1, float a2, int32_t a3, double a4, uint8_t a5, float a6, int64_t a7, float a8,
                    float a9, float a10, float a11, float a12, float a13, int32_t a14, float a15) {
	return 1 * (double)a1 + 2 * (double)a2 + 3 * (double)a3 + 4 * a4 + 5 * (double)a5 + 6 * (double)a6 +
	       7 * (double)a7 + 8 * (double)a8 + 9 * (double)a9 + 10 * (double)a10 + 11 * (double)a11 +
	       12 * (double)a12 + 13 * (double)a13 + 14 * (double)a14 + 15 * (double)a15;
}

EXPORT float uniform1f(int32_t location, float v0) {
	return (float)location * 1000.0f + v0;
}

EXPORT uint8_t isenabled(uint32_t cap) {
	return cap == 0x0B71;
}

EXPORT void *ptrret(void *p) {
	return p;
}

EXPORT double many23(int32_t a1, float a2, uint8_t a3, double a4, int16_t a5, float a6, int64_t a7, float a8,
                     uint32_t a9, float a10, int32_t a11, float a12, uint8_t a13, float a14, int32_t a15, float a16,
                     int16_t a17, float a18, uint8_t a19, double a20, int32_t a21, float a22, uint8_t a23) {
	return 1 * (double)a1 + 2 * (double)a2 + 3 * (double)a3 + 4 * a4 + 5 * (double)a5 + 6 * (double)a6 +
	       7 * (double)a7 + 8 * (double)a8 + 9 * (double)a9 + 10 * (double)a10 + 11 * (double)a11 +
	       12 * (double)a12 + 13 * (double)a13 + 14 * (double)a14 + 15 * (double)a15 + 16 * (double)a16 +
	       17 * (double)a17 + 18 * (double)a18 + 19 * (double)a19 + 20 * a20 + 21 * (double)a21 +
	       22 * (double)a22 + 23 * (double)a23;
}

EXPORT uint64_t ret64(uint32_t a, uint64_t b, int32_t c) {
	return b * 2 + a + (uint64_t)(int64_t)c;
}
