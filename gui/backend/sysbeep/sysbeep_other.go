//go:build (!darwin || ios) && !windows && (!linux || android)

package sysbeep

// No system alert sound on mobile or wasm targets: iOS and Android
// route alerts through their own notification frameworks rather than
// an app-triggered beep, and the browser has no such concept.

// Play is a no-op on platforms without a system alert sound.
func Play() {}

// Available reports that this platform has no system alert sound.
func Available() bool { return false }
