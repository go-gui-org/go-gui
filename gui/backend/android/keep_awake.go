//go:build android

package android

// keep_awake.go — keeping the display on (issue #1172).
//
// The backend cannot call Java, so it keeps the state and the host's
// renderer polls PendingKeepAwake each frame, setting or clearing
// FLAG_KEEP_SCREEN_ON on the activity's window when the value changes.
// gui ends the request when the activity stops, where Android ignores
// the flag anyway.

import "sync/atomic"

// keepAwake is what gui last asked for.
var keepAwake atomic.Bool

// SetKeepAwake records the request for the host to apply.
func (n *nativePlatform) SetKeepAwake(on bool, _, _ string) error {
	keepAwake.Store(on)
	return nil
}

// PendingKeepAwake reports whether the app wants the screen kept on.
// It is the state, not an event: it is not cleared on read. The host
// polls it each frame and adds or clears
// WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON on the UI thread when
// it changes (see examples/android_demo).
func PendingKeepAwake() bool { return keepAwake.Load() }
