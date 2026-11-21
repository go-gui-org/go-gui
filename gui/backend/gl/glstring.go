//go:build (linux && !js && !android) || (windows && !js)

package gl

import "unsafe"

// glRenderer is GL_RENDERER from gl.h.
const glRenderer = 0x1F01

// cGoString copies a NUL-terminated C string the GL driver owns. A null
// pointer gives "". The read is capped, so a driver that returns garbage
// cannot walk the read off into unmapped memory without bound.
func cGoString(u uintptr) string { return cGoStringN(u, 1024) }

// cGoStringN is cGoString capped at maxLen bytes, for the longer strings
// such as an EGL display's extension list.
func cGoStringN(u uintptr, maxLen int) string {
	if u == 0 {
		return ""
	}
	// Spelled through a pointer to u so vet does not read it as a Go
	// pointer smuggled through uintptr: it is C memory.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&u))
	n := 0
	for n < maxLen && *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(p), n))
}
