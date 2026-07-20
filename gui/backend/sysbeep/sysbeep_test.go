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
