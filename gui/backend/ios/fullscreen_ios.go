//go:build ios

package ios

// fullscreen_ios.go — fullscreen on iOS (issue #1171).
//
// iOS has no fullscreen window to ask for: an app already fills its
// window. Fullscreen hides the status bar and lets the home indicator
// fade out. Neither resizes the view, and a notched iPhone announces
// neither, so the controller (ios_app.m) works out IsFullscreen
// itself: asked for, and the window fills the screen. A windowed iPad
// does not, so IsFullscreen stays false there until the window does.
//
// The home indicator comes back on a touch and fades again; iOS allows
// no more. A Swift host (Pattern B) owns its view controller, so
// SetFullscreen does nothing there.

/*
#include "ios_app.h"
*/
import "C"

import "github.com/go-gui-org/go-gui/gui"

// SetWindowFullscreen hands the request to the main queue, as the
// keyboard requests do: an app calls it from inside an event handler,
// and the controller's report comes back into gui.
func (n *nativePlatform) SetWindowFullscreen(on bool) {
	C.iosSetFullscreen(cBool(on))
}

// goIOSFullscreen receives what the controller worked out: fullscreen
// asked for, and the window fills the screen. gui ignores a repeat.
//
//export goIOSFullscreen
func goIOSFullscreen(on C.int) {
	gui.DispatchWindowState(iosWindow, false, on != 0)
}
