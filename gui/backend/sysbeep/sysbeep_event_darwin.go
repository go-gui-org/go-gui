//go:build darwin && !ios

package sysbeep

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>
#include "sysbeep_event_darwin.h"
*/
import "C"

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

// cNames holds one C string per event, allocated on first use, so a
// cue costs no allocation and no free. The table is fixed and small,
// and the strings live for the life of the process by design.
var cNames [eventCount]*C.char

func playEvent(e Event) {
	name := eventSounds[e]
	if name == "" {
		return
	}
	if cNames[e] == nil {
		cNames[e] = C.CString(name)
	}
	C.sysbeepPlayEvent(cNames[e])
}

func eventAvailable() bool { return C.sysbeepEventAvailable() != 0 }
