//go:build linux && !android

package sysbeep

import "os/exec"

// eventNames maps an Event onto a freedesktop sound-naming id — the
// same vocabulary GTK apps use, so the sound the user hears is the one
// their chosen sound theme defines. Indexed by Event; an empty entry is
// silent.
//
// The ids come from the freedesktop sound-naming spec: button-pressed
// and button-released are the two halves of a press, so a toggle uses
// them for its two directions; "complete" marks a finished action, and
// "message" is the notification sound a toast wants.
var eventNames = [eventCount]string{
	EventClick:     "button-pressed",
	EventToggleOn:  "button-pressed",
	EventToggleOff: "button-released",
	EventSelection: "button-pressed",
	EventError:     "dialog-error",
	EventNotify:    "message",
	EventOpen:      "dialog-information",
	EventSuccess:   "complete",
}

// playEvent shells out to canberra-gtk-play, exactly as Play does for
// the bell. One process per cue: fine for the occasional dialog or
// toast, wrong for a click-heavy UI — such an app wants a sampled
// player (see gui/audio) rather than system event sounds.
func playEvent(e Event) {
	path := lookupOnce()
	if path == "" {
		return
	}
	name := eventNames[e]
	if name == "" {
		return
	}
	// #nosec G204 — path from exec.LookPath, name from the fixed table
	cmd := exec.Command(path, "-i", name)
	if err := cmd.Start(); err != nil {
		return
	}
	// Reap asynchronously; without this every cue leaves a zombie for
	// the life of the process.
	go func() { _ = cmd.Wait() }()
}

// eventAvailable reports whether canberra-gtk-play was found on PATH.
func eventAvailable() bool { return lookupOnce() != "" }
