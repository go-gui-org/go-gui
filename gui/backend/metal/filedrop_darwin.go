//go:build darwin && !ios

package metal

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include "filedrop_darwin.h"
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/veandco/go-sdl2/sdl"
)

// fileDropCallbacks maps SDL_Window pointer → drop handler.
// All access is main-thread only (SDL event loop + ObjC drag).
var fileDropCallbacks = map[uintptr]func(path string){}

var disableSDLDrop = sync.OnceFunc(func() {
	sdl.EventState(sdl.DROPFILE, sdl.DISABLE)
	sdl.EventState(sdl.DROPBEGIN, sdl.DISABLE)
	sdl.EventState(sdl.DROPCOMPLETE, sdl.DISABLE)
	sdl.EventState(sdl.DROPTEXT, sdl.DISABLE)
})

//export goFileDrop
func goFileDrop(cwin *C.SDL_Window, cpath *C.char) {
	if cpath == nil {
		return
	}
	key := uintptr(unsafe.Pointer(cwin))
	if cb := fileDropCallbacks[key]; cb != nil {
		cb(C.GoString(cpath))
	}
}

// fileDropInitBridge registers the SDL window's NSView as a
// Cocoa drag-and-drop target, bypassing SDL2's broken DropEvent
// handling (go-sdl2 calls SDL_free on a non-SDL-allocated
// pointer).
func fileDropInitBridge(win *sdl.Window, w *gui.Window) {
	cWin := (*C.SDL_Window)(unsafe.Pointer(win))
	key := uintptr(unsafe.Pointer(cWin))

	fileDropCallbacks[key] = func(path string) {
		mx, my, _ := sdl.GetMouseState()
		evt := &gui.Event{
			Type:     gui.EventFileDropped,
			FilePath: path,
			MouseX:   float32(mx),
			MouseY:   float32(my),
		}
		w.EventFn(evt)
		w.UpdateWindow()
	}

	// Disable SDL2 drop events to prevent go-sdl2's buggy
	// SDL_free on macOS Cocoa file paths.
	disableSDLDrop()

	C.fileDropInit(cWin)
}

func fileDropDestroyBridge(win *sdl.Window) {
	cWin := (*C.SDL_Window)(unsafe.Pointer(win))
	key := uintptr(unsafe.Pointer(cWin))
	delete(fileDropCallbacks, key)
	C.fileDropDestroy(cWin)
}
