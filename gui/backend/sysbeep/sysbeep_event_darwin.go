//go:build darwin && !ios

package sysbeep

import (
	"runtime"
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// eventSounds maps an Event onto a sound in /System/Library/Sounds —
// the same names the Sound preference pane lists, so every one of them
// is a sound the user already recognises.
//
// Tink is short and neutral, which is what a click wants; Pop and
// Bottle read as on and off; Ping marks a pick; Basso is the standard
// failure sound; Purr is soft enough for an unasked-for toast; Blow
// opens; Glass reads as completion. An empty entry is silent.
var eventSounds = [eventCount]string{
	EventClick:     "Tink",
	EventToggleOn:  "Pop",
	EventToggleOff: "Bottle",
	EventSelection: "Ping",
	EventError:     "Basso",
	EventNotify:    "Purr",
	EventOpen:      "Blow",
	EventSuccess:   "Glass",
}

// snd is the NSSound state. PlayEvent can run on any goroutine, so mu
// guards all of it, the argument array in call included.
//
// The NSSound objects are cached per event: soundNamed: reads the file
// the first time, and a UI cue must not touch the disk on every click.
// After the first play of an event, a cue makes no heap allocation:
// the sends go through call, not objc.ID.Send (#1307).
var snd struct {
	mu   sync.Mutex
	once sync.Once
	call darwinffi.Caller

	sounds [eventCount]objc.ID

	msgSend, poolPush, poolPop uintptr

	nsSound                  objc.ID
	selSoundNamed, selRetain objc.SEL
	selStop, selPlay         objc.SEL
}

// loadSnd binds the runtime entry points and selectors. Called with
// snd.mu held.
func loadSnd() {
	snd.once.Do(func() {
		darwinffi.Open(darwinffi.PathAppKit) // NSSound lives in AppKit
		snd.msgSend = darwinffi.MsgSend()
		snd.poolPush = darwinffi.PoolPushAddr()
		snd.poolPop = darwinffi.PoolPopAddr()
		snd.nsSound = darwinffi.Class("NSSound")
		snd.selSoundNamed = darwinffi.Sel("soundNamed:")
		snd.selRetain = darwinffi.Sel("retain")
		snd.selStop = darwinffi.Sel("stop")
		snd.selPlay = darwinffi.Sel("play")
	})
}

// namedSound returns the cached NSSound for e, or 0 if the system has
// no such sound: a user can remove one from /System/Library/Sounds, and
// a missing sound is silence, not an error. Called with snd.mu held and
// inside an autorelease pool.
func namedSound(e Event) objc.ID {
	if s := snd.sounds[e]; s != 0 {
		return s
	}
	// First use only: NSString allocates, which is fine once per event.
	name := darwinffi.NSString(eventSounds[e])
	s := objc.ID(snd.call.Call(snd.msgSend, uintptr(snd.nsSound), uintptr(snd.selSoundNamed), uintptr(name)))
	if s == 0 {
		return 0
	}
	// soundNamed: returns an autoreleased object. The cache keeps it
	// for the life of the process, so retain it once and never release.
	snd.call.Call(snd.msgSend, uintptr(s), uintptr(snd.selRetain))
	snd.sounds[e] = s
	return s
}

// poolPush opens an autorelease pool with no heap allocation. A pool
// belongs to its thread, so the goroutine stays locked to its OS thread
// until poolPop. Called with snd.mu held.
func poolPush() uintptr {
	runtime.LockOSThread()
	return snd.call.Call(snd.poolPush)
}

// poolPop drains the pool that poolPush opened and unlocks the thread.
func poolPop(pool uintptr) {
	snd.call.Call(snd.poolPop, pool)
	runtime.UnlockOSThread()
}

func playEvent(e Event) {
	if eventSounds[e] == "" {
		return
	}
	snd.mu.Lock()
	defer snd.mu.Unlock()
	loadSnd()
	pool := poolPush()
	if s := namedSound(e); s != 0 {
		// Stop before playing: a cached NSSound already playing ignores
		// a second play, which would swallow the second of two quick
		// cues. Restarting is the behaviour a UI sound wants.
		snd.call.Call(snd.msgSend, uintptr(s), uintptr(snd.selStop))
		snd.call.Call(snd.msgSend, uintptr(s), uintptr(snd.selPlay))
	}
	poolPop(pool)
}

func eventAvailable() bool {
	snd.mu.Lock()
	defer snd.mu.Unlock()
	loadSnd()
	pool := poolPush()
	// Tink is the click sound and has shipped in every macOS release;
	// if it cannot be loaded, neither can the rest.
	ok := namedSound(EventClick) != 0
	poolPop(pool)
	return ok
}
