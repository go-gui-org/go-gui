//go:build darwin

package darwinffi

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Framework paths. Every macOS release has them: they are in the dyld
// shared cache, so Dlopen finds them without a file on disk.
const (
	PathObjC           = "/usr/lib/libobjc.A.dylib"
	PathCoreFoundation = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	PathFoundation     = "/System/Library/Frameworks/Foundation.framework/Foundation"
	PathAppKit         = "/System/Library/Frameworks/AppKit.framework/AppKit"
	PathMetal          = "/System/Library/Frameworks/Metal.framework/Metal"
	PathQuartzCore     = "/System/Library/Frameworks/QuartzCore.framework/QuartzCore"
	PathSecurity       = "/System/Library/Frameworks/Security.framework/Security"
	PathPDFKit         = "/System/Library/Frameworks/PDFKit.framework/PDFKit"
	PathUTI            = "/System/Library/Frameworks/UniformTypeIdentifiers.framework/UniformTypeIdentifiers"
)

// libs caches one handle per path. A framework loads on first use, not
// at init, so a program that only detects its locale never loads AppKit.
var libs sync.Map // path -> uintptr

// Open returns the handle for the framework or library at path. It
// panics when the path does not load: these are system frameworks, and
// a Mac without one cannot run the app anyway.
func Open(path string) uintptr {
	if h, ok := libs.Load(path); ok {
		return h.(uintptr)
	}
	h, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		panic(fmt.Sprintf("darwinffi: load %s: %v", path, err))
	}
	// Two first callers can both Dlopen. dyld counts references, so the
	// second handle is the same library; LoadOrStore keeps one.
	got, _ := libs.LoadOrStore(path, h)
	return got.(uintptr)
}

// Func returns the address of the C function name in lib. It panics
// when the symbol is missing, for the same reason as Open.
func Func(lib uintptr, name string) uintptr {
	p, err := purego.Dlsym(lib, name)
	if err != nil {
		panic(fmt.Sprintf("darwinffi: %v", err))
	}
	return p
}

// Data returns the value of the pointer-size global variable name in
// lib. Framework constants such as kSecClass or NSAppearanceNameAqua
// are globals that hold a pointer to the object, not functions: Dlsym
// gives the address of the global, and the object is what it holds.
func Data(lib uintptr, name string) uintptr {
	p := Func(lib, name)
	// p is the address of a C global, outside the Go heap, so the
	// garbage collector never moves or frees it. Store it into a
	// pointer variable by its bits: a direct unsafe.Pointer(p) does
	// the same thing, but go vet flags every uintptr-to-pointer cast.
	var global *uintptr
	*(*uintptr)(unsafe.Pointer(&global)) = p
	return *global
}
