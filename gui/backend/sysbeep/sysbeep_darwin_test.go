//go:build darwin && !ios

package sysbeep

import "testing"

// Issue #1307: a cue goes through purego. After the first play of an
// event loads and caches its NSSound, a cue makes no heap allocation,
// the same as the cgo form. objc.ID.Send would allocate on every send,
// so the sends use darwinffi.Caller.
func TestPlayEventDoesNotAllocate(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts do not hold under -race")
	}
	if !EventAvailable() {
		t.Skip("no system sounds on this Mac")
	}
	PlayEvent(EventClick) // load and cache the sound
	if n := testing.AllocsPerRun(20, func() { PlayEvent(EventClick) }); n != 0 {
		t.Errorf("PlayEvent allocates %.0f times per cue, want 0", n)
	}
}

// namedSound returns one retained NSSound per event, and the same one
// on the next call.
func TestNamedSoundCached(t *testing.T) {
	snd.mu.Lock()
	defer snd.mu.Unlock()
	loadSnd()
	pool := poolPush()
	defer poolPop(pool)
	first := namedSound(EventClick)
	if first == 0 {
		t.Skip("Tink is missing from /System/Library/Sounds")
	}
	if again := namedSound(EventClick); again != first {
		t.Errorf("second lookup = %#x, want the cached %#x", again, first)
	}
}
