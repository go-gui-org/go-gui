//go:build ios

package ios

// visibility_ios.go — the app going to the background on iOS (issue
// #1170).
//
// The controller (ios_app.m) reports the scene entering the background
// and coming back to the foreground. The same pair pauses and resumes
// the display link, so gui reports the window as shown exactly while
// frames are drawn. Not willDeactivate / didActivate: those also fire
// for Control Center, the notification pull-down and a system alert,
// while the app is still on screen.

/*
#include "ios_app.h"
*/
import "C"

import "github.com/go-gui-org/go-gui/gui"

// goIOSVisibility receives the scene's background state on the main
// thread. gui ignores a repeat, and a report before the window exists.
//
//export goIOSVisibility
func goIOSVisibility(visible C.int) {
	gui.DispatchWindowOccluded(iosWindow, visible == 0)
}
