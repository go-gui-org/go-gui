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
	"github.com/veandco/go-sdl2/sdl"
)

// a11yActionCallback stores the Go callback invoked from ObjC
// when VoiceOver triggers an action.
var a11yActionCallback func(action, index int)

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

func a11yInitBridge(win *sdl.Window) {
	cWin := (*C.SDL_Window)(unsafe.Pointer(win))
	C.a11yInit(cWin)
}

func a11ySyncBridge(nodes []gui.A11yNode, count, focusedIdx int, windowH float32) {
	if count <= 0 {
		return
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

	// Free all C strings and nil out to avoid dangling pointers.
	for i, cs := range cStringBuf {
		C.free(unsafe.Pointer(cs))
		cStringBuf[i] = nil
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
