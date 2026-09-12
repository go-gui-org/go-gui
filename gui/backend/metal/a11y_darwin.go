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

// a11yCallbacks routes VoiceOver actions to the owning window: one
// live tree per window, so one callback per window. Guarded by a11yMu
// alongside the marshaling buffers below.
var a11yCallbacks = make(map[uintptr]func(action, index int))

func a11yToken(w C.GoGuiNSWindow) uintptr {
	return uintptr(unsafe.Pointer(w))
}

func setA11yCallback(token uintptr, cb func(action, index int)) {
	a11yMu.Lock()
	defer a11yMu.Unlock()
	if cb == nil {
		delete(a11yCallbacks, token)
		return
	}
	a11yCallbacks[token] = cb
}

func clearA11yCallback(token uintptr) {
	a11yMu.Lock()
	defer a11yMu.Unlock()
	delete(a11yCallbacks, token)
}

//export goA11yAction
func goA11yAction(action, index C.int, token C.uintptr_t) {
	a11yMu.Lock()
	cb := a11yCallbacks[uintptr(token)]
	a11yMu.Unlock()
	if cb != nil {
		cb(int(action), int(index))
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

func a11ySyncBridge(w C.GoGuiNSWindow, nodes []gui.A11yNode, count, focusedIdx int, windowH float32) {
	// Clamp first: a count past the slice end reads out of bounds,
	// and a clamped-to-zero count must take the clearing path
	// below rather than indexing an empty buffer.
	if count > len(nodes) {
		count = len(nodes)
	}
	if count <= 0 {
		// Push the emptied tree so the previous content clears on
		// the ObjC side rather than lingering. Nil nodes are safe:
		// the callee clears without dereferencing.
		a11yMu.Lock()
		defer a11yMu.Unlock()
		C.a11ySync(w, nil, C.int(0), C.int(focusedIdx), C.float(windowH))
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
		w,
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
