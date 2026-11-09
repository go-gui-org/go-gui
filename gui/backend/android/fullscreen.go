//go:build android

package android

// fullscreen.go — fullscreen on Android (issue #1171).
//
// The backend cannot call Java, so the host does the work, as it does
// for URIs: SetWindowFullscreen leaves a request, the host polls it
// each frame with PendingFullscreen, applies it on the UI thread and
// answers through FullscreenChanged. The view does not resize on
// Android 15 and later, so the state has to come back from the host;
// EventResized cannot carry it. examples/android_demo has the host side
// (GoGuiFullscreen.kt) for every Android version from 7 (API 24).
//
// A new activity starts out of fullscreen and says so through
// FullscreenChanged, so IsFullscreen turns false there; an app that
// still wants fullscreen asks again.

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// The values PendingFullscreen returns.
const (
	fullscreenNone  int32 = 0
	fullscreenEnter int32 = 1
	fullscreenLeave int32 = 2
)

var (
	pendingFullscreenMu sync.Mutex
	pendingFullscreen   int32
)

// SetWindowFullscreen leaves the request for the host. The last call
// before the next poll wins.
func (n *nativePlatform) SetWindowFullscreen(on bool) {
	req := fullscreenLeave
	if on {
		req = fullscreenEnter
	}
	pendingFullscreenMu.Lock()
	pendingFullscreen = req
	pendingFullscreenMu.Unlock()
}

// PendingFullscreen returns and clears the last fullscreen request: 1
// to enter, 2 to leave, 0 when there is none. The host polls it each
// frame, like PendingURI, and applies the request on the UI thread. A
// host that never polls it leaves SetFullscreen without effect.
func PendingFullscreen() int32 {
	pendingFullscreenMu.Lock()
	req := pendingFullscreen
	pendingFullscreen = fullscreenNone
	pendingFullscreenMu.Unlock()
	return req
}

// FullscreenChanged is called from Kotlin with whether the activity is
// now fullscreen: asked for, and not sharing the screen in multi-window
// mode. It feeds Window.IsFullscreen. Call it on the GL thread
// (queueEvent), like SafeAreaInsets. gui ignores a repeat.
func FullscreenChanged(on bool) {
	gui.DispatchWindowState(androidWindow, false, on)
}
