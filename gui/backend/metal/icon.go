//go:build darwin && !ios

package metal

/*
#cgo LDFLAGS: -framework Cocoa

#include "metal_window.h"
*/
import "C"
import (
	"unsafe"
)

// activateApp sets the NSApplication activation policy to Regular
// and brings the app to the foreground. Must be called before
// creating any windows for CLI-launched (non-bundled) binaries.
func activateApp() {
	C.metalAppInit()
}

// setAppIcon sets the macOS Dock icon from PNG data.
func setAppIcon(png []byte) {
	if len(png) == 0 {
		return
	}
	C.metalSetDockIcon(unsafe.Pointer(&png[0]), C.int(len(png)))
}
