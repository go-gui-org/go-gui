//go:build (!darwin || ios || !cgo) && !windows && (!linux || android)

package sysbeep

// No system event sounds on mobile or wasm targets, for the same
// reason there is no alert sound: iOS and Android route feedback
// through their own frameworks, and the browser has no such concept.

// playEvent is a no-op on platforms without system event sounds.
func playEvent(Event) {}

// eventAvailable reports that this platform has no event sounds.
func eventAvailable() bool { return false }
