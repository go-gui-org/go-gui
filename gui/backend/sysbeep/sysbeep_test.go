//go:build (linux || windows || (darwin && cgo)) && !ios

package sysbeep

import "testing"

// TestPlayDoesNotPanic exercises the platform Play path. It cannot
// assert audio came out, but it does catch a missing symbol, a bad cgo
// signature, or a nil-deref in the lookup path — and on CI it also
// proves Play stays non-blocking.
func TestPlayDoesNotPanic(t *testing.T) {
	Play()
}

// TestAvailableMatchesPlatform pins the contract that Available is
// answerable without side effects and is stable across calls, so
// callers can use it to pick a visual fallback once at startup.
func TestAvailableMatchesPlatform(t *testing.T) {
	first := Available()
	if second := Available(); first != second {
		t.Errorf("Available not stable: %v then %v", first, second)
	}
}

// PlayEvent must be silent, never a panic, for every event — including
// on targets with no event sounds at all — and for a value outside the
// enum, which is what an older backend paired with a newer gui would
// pass (issue #469).
func TestPlayEventDoesNotPanic(t *testing.T) {
	for e := range int(eventCount) {
		PlayEvent(Event(e))
	}
	PlayEvent(eventCount)
	PlayEvent(Event(200))
}

// EventAvailable is a property of the platform, so it must not change
// between calls; a caller uses it to decide on a visual fallback once.
func TestEventAvailableStable(t *testing.T) {
	first := EventAvailable()
	for range 3 {
		if EventAvailable() != first {
			t.Fatal("EventAvailable changed between calls")
		}
	}
}
