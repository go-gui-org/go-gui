//go:build windows && cgo

package gui

/*
#include <stdarg.h>

// __ms_vsscanf was dropped from MinGW-w64 runtime (GCC ≥13).
// go-sdl2 bundled static libs compiled against older MinGW still
// reference it. Provide a weak definition: on new toolchains it
// fills the gap; on old toolchains (where libmingwex already
// provides it) the weak symbol is silently ignored.
//
// Don't include <stdio.h> — vsscanf is a macro there and would
// expand to the very symbol we're defining.

__attribute__((weak)) int
__ms_vsscanf(const char *str, const char *format, va_list ap) {
	// __mingw_vsscanf is the current MinGW internal name.
	extern int __mingw_vsscanf(const char *, const char *, va_list);
	return __mingw_vsscanf(str, format, ap);
}
*/
import "C"
