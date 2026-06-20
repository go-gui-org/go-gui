//go:build darwin && !ios

package metal

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include "a11y_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// a11yActionCallback stores the Go callback invoked from ObjC
// when VoiceOver triggers an action.
var a11yActionCallback func(action, index int)

func setA11yCallback(cb func(action, index int)) {
	a11yActionCallback = cb
}

//export goA11yAction
func goA11yAction(action, index C.int) {
	if a11yActionCallback != nil {
		a11yActionCallback(int(action), int(index))
	}
}

// Reusable C buffers — grow only, never shrink.
var (
	a11yMu     sync.Mutex
	cNodeBuf   []C.A11yCNode
	cStringBuf []*C.char
)

// maxA11yNodes caps the accessibility node count to prevent
// unbounded C allocations from buggy or malicious callers.
const maxA11yNodes = 50000

func a11ySyncBridge(nodes []gui.A11yNode, count, focusedIdx int, windowH float32) {
	if count <= 0 {
		return
	}
	if count > maxA11yNodes {
		count = maxA11yNodes
	}
	a11yMu.Lock()
	defer a11yMu.Unlock()
	// Grow buffer if needed.
	if cap(cNodeBuf) < count {
		cNodeBuf = make([]C.A11yCNode, count)
	}
	cNodeBuf = cNodeBuf[:count]

	// Reslice reusable C string buffer.
	cStringBuf = cStringBuf[:0]

	for i := range count {
		n := &nodes[i]
		cn := &cNodeBuf[i]

		cn.role = C.int(n.Role)
		cn.state = C.int(n.State)
		cn.x = C.float(n.X)
		cn.y = C.float(n.Y)
		cn.w = C.float(n.W)
		cn.h = C.float(n.H)
		cn.parentIdx = C.int(n.ParentIdx)
		cn.childrenStart = C.int(n.ChildrenStart)
		cn.childrenCount = C.int(n.ChildrenCount)

		cn.label = cStringOrNil(n.Label, &cStringBuf)
		cn.value = cStringOrNil(n.Value, &cStringBuf)
		cn.description = cStringOrNil(n.Description, &cStringBuf)
	}

	C.a11ySync(
		&cNodeBuf[0],
		C.int(count),
		C.int(focusedIdx),
		C.float(windowH),
	)

	// Free all C strings. cStringBuf is re-sliced to zero on
	// the next call, so explicit nil-assignment is unnecessary.
	for _, cs := range cStringBuf {
		C.free(unsafe.Pointer(cs))
	}
}

// cStringOrNil converts a Go string to a C string, appending it
// to the collector for later freeing. Returns nil for empty
// strings.
func cStringOrNil(s string, collector *[]*C.char) *C.char {
	if s == "" {
		return nil
	}
	cs := C.CString(s)
	*collector = append(*collector, cs)
	return cs
}

func a11yDestroyBridge() {
	C.a11yDestroy()
}

func a11yAnnounceBridge(text string) {
	cs := C.CString(text)
	defer C.free(unsafe.Pointer(cs))
	C.a11yAnnounce(cs)
}

// Test helpers — wrap C types and calls so _test.go files
// don't need their own import "C" block (not supported by the
// go toolchain for in-package cgo tests).

type cchar = *C.char

func cFree(p cchar)            { C.free(unsafe.Pointer(p)) }
func cGoString(p cchar) string { return C.GoString(p) }
