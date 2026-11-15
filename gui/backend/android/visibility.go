//go:build android

package android

// visibility.go — the app going to the background on Android (issue
// #1170).
//
// The backend cannot see the activity's lifecycle, so the host reports
// it: VisibilityChanged(false) from onStop, VisibilityChanged(true) from
// onStart. Not onPause / onResume: in multi-window mode a paused
// activity is still on screen. examples/android_demo has the host side.

import "github.com/go-gui-org/go-gui/gui"

// VisibilityChanged is called from Kotlin when the activity starts
// (true) or stops (false). gui then reports the window through
// Window.IsOccluded and sends EventVisibilityChanged, and parks the
// animation loop while the app is hidden.
//
// Call it on the GL thread (queueEvent), like TouchInput: the app's
// event handler runs inside the call. GLSurfaceView runs a queued event
// while it is paused, so the report from onStop arrives at once, not
// when the app comes back. A host that never calls it leaves
// IsOccluded false and sends no event.
//
// Do not report a stop for a configuration change
// (Activity.isChangingConfigurations): Android replaces the activity
// at once, on a new GL thread, and the app never leaves the screen.
func VisibilityChanged(visible bool) {
	gui.DispatchWindowOccluded(androidWindow, !visible)
}
