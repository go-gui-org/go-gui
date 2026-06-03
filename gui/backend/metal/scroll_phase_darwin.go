//go:build darwin && !ios

package metal

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include "scroll_phase_darwin.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/veandco/go-sdl2/sdl"
)

// scrollBeganCallbacks maps SDL_Window pointer → callback.
// All access is main-thread only (SDL event loop + ObjC monitor).
var scrollBeganCallbacks = map[uintptr]func(){}

//export goScrollBegan
func goScrollBegan(cwin *C.SDL_Window) {
	key := uintptr(unsafe.Pointer(cwin))
	if cb := scrollBeganCallbacks[key]; cb != nil {
		cb()
	}
}

func scrollPhaseInitBridge(win *sdl.Window, w *gui.Window) {
	cWin := (*C.SDL_Window)(unsafe.Pointer(win))
	key := uintptr(unsafe.Pointer(cWin))
	scrollBeganCallbacks[key] = func() {
		evt := &gui.Event{Type: gui.EventScrollBegan}
		w.EventFn(evt)
		w.UpdateWindow()
	}
	C.scrollPhaseInit(cWin)
}

func scrollPhaseDestroyBridge(win *sdl.Window) {
	cWin := (*C.SDL_Window)(unsafe.Pointer(win))
	key := uintptr(unsafe.Pointer(cWin))
	delete(scrollBeganCallbacks, key)
	C.scrollPhaseDestroy(cWin)
}
