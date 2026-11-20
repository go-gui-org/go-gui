//go:build darwin && !ios

package darwinffi

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CFStringEncodingUTF8 is kCFStringEncodingUTF8.
const CFStringEncodingUTF8 = 0x08000100

// cf holds the CoreFoundation functions that more than one service
// needs. They are bound on first use. These calls are not on a
// per-frame path, so the typed RegisterFunc form, which allocates on
// each call, is acceptable here.
var cf struct {
	once sync.Once

	release           func(ref uintptr)
	stringCreate      func(alloc uintptr, s string, enc uint32) uintptr
	stringGetLength   func(s uintptr) int
	stringMaxSize     func(length int, enc uint32) int
	stringGetCString  func(s uintptr, buf *byte, size int, enc uint32) bool
	getTypeID         func(ref uintptr) uintptr
	dataGetTypeID     func() uintptr
	dataCreate        func(alloc uintptr, bytes *byte, n int) uintptr
	dataGetLength     func(d uintptr) int
	dataGetBytePtr    func(d uintptr) uintptr
	dictCreateMutable func(alloc uintptr, capacity int, keyCB, valueCB uintptr) uintptr
	dictSetValue      func(d, key, value uintptr)

	// Addresses of the callback structs, not their contents:
	// CFDictionaryCreateMutable takes a pointer to each.
	dictKeyCallBacks, dictValueCallBacks uintptr
	booleanTrue                          uintptr
}

func loadCF() {
	cf.once.Do(func() {
		lib := Open(PathCoreFoundation)
		purego.RegisterLibFunc(&cf.release, lib, "CFRelease")
		purego.RegisterLibFunc(&cf.stringCreate, lib, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&cf.stringGetLength, lib, "CFStringGetLength")
		purego.RegisterLibFunc(&cf.stringMaxSize, lib, "CFStringGetMaximumSizeForEncoding")
		purego.RegisterLibFunc(&cf.stringGetCString, lib, "CFStringGetCString")
		purego.RegisterLibFunc(&cf.getTypeID, lib, "CFGetTypeID")
		purego.RegisterLibFunc(&cf.dataGetTypeID, lib, "CFDataGetTypeID")
		purego.RegisterLibFunc(&cf.dataCreate, lib, "CFDataCreate")
		purego.RegisterLibFunc(&cf.dataGetLength, lib, "CFDataGetLength")
		purego.RegisterLibFunc(&cf.dataGetBytePtr, lib, "CFDataGetBytePtr")
		purego.RegisterLibFunc(&cf.dictCreateMutable, lib, "CFDictionaryCreateMutable")
		purego.RegisterLibFunc(&cf.dictSetValue, lib, "CFDictionarySetValue")
		cf.dictKeyCallBacks = Func(lib, "kCFTypeDictionaryKeyCallBacks")
		cf.dictValueCallBacks = Func(lib, "kCFTypeDictionaryValueCallBacks")
		cf.booleanTrue = Data(lib, "kCFBooleanTrue")
	})
}

// CFRelease releases ref. A zero ref is a no-op: CFRelease(NULL)
// crashes, so callers need not check first.
func CFRelease(ref uintptr) {
	if ref == 0 {
		return
	}
	loadCF()
	cf.release(ref)
}

// CFString returns a new CFString that holds s, or 0 when s is not
// valid UTF-8. The caller releases it. As with C.CString, s ends at
// its first NUL byte.
func CFString(s string) uintptr {
	loadCF()
	return cf.stringCreate(0, s, CFStringEncodingUTF8)
}

// GoString copies the CFString s into a Go string. A zero s gives "".
func GoString(s uintptr) string {
	if s == 0 {
		return ""
	}
	loadCF()
	// Room for the longest UTF-8 form plus the NUL. The same sizing as
	// the cgo code this replaces (locale_detect_darwin.go before #1307).
	size := cf.stringMaxSize(cf.stringGetLength(s), CFStringEncodingUTF8) + 1
	if size <= 1 {
		return ""
	}
	buf := make([]byte, size)
	if !cf.stringGetCString(s, &buf[0], size, CFStringEncodingUTF8) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// CFTypeID returns CFGetTypeID(ref).
func CFTypeID(ref uintptr) uintptr {
	loadCF()
	return cf.getTypeID(ref)
}

// CFDataTypeID returns CFDataGetTypeID().
func CFDataTypeID() uintptr {
	loadCF()
	return cf.dataGetTypeID()
}

// CFData returns a new CFData that holds a copy of b. The caller
// releases it. CFDataCreate copies the bytes, so b is free to change
// after the call.
func CFData(b []byte) uintptr {
	loadCF()
	var p *byte
	if len(b) > 0 {
		p = &b[0]
	}
	return cf.dataCreate(0, p, len(b))
}

// CFDataLen returns CFDataGetLength(d).
func CFDataLen(d uintptr) int {
	loadCF()
	return cf.dataGetLength(d)
}

// CFDataPtr returns CFDataGetBytePtr(d). The memory belongs to d and
// is valid until d is released.
func CFDataPtr(d uintptr) uintptr {
	loadCF()
	return cf.dataGetBytePtr(d)
}

// CFDataBytes returns a Go copy of the bytes of the CFData d. The
// result is never nil: an empty d gives an empty slice. NSData is
// toll-free bridged to CFData, so d can be an NSData.
func CFDataBytes(d uintptr) []byte {
	n := CFDataLen(d)
	out := make([]byte, n)
	if n > 0 {
		// The bytes belong to d, outside the Go heap, and stay valid
		// until d is released. The bits go through a pointer variable
		// because go vet flags every direct uintptr-to-pointer cast.
		var p *byte
		*(*uintptr)(unsafe.Pointer(&p)) = CFDataPtr(d)
		copy(out, unsafe.Slice(p, n))
	}
	return out
}

// CFMutableDict returns a new, empty CFMutableDictionary that retains
// its keys and values (the kCFType callbacks). The caller releases it.
func CFMutableDict() uintptr {
	loadCF()
	return cf.dictCreateMutable(0, 0, cf.dictKeyCallBacks, cf.dictValueCallBacks)
}

// CFDictSet sets d[key] = value. The dictionary retains both.
func CFDictSet(d, key, value uintptr) {
	loadCF()
	cf.dictSetValue(d, key, value)
}

// CFTrue returns kCFBooleanTrue.
func CFTrue() uintptr {
	loadCF()
	return cf.booleanTrue
}
