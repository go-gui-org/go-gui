//go:build js && wasm

package web

import (
	"github.com/go-gui-org/go-gui/gui"
)

// touchEventFromLists builds the framework touch event from one DOM
// touch event's two touch lists: all holds e.touches (the fingers
// still down) and changed holds e.changedTouches (the fingers this
// event reports).
//
// Began and Moved carry the fingers still down, with Changed marked
// on the ones this event moved. Ended and Cancelled carry the lifted
// fingers instead: e.touches no longer holds them, so building from
// it sends an empty event that removes nothing and leaves the
// recognizer wedged with the finger still tracked. Every gesture
// after that misfires: the second tap of a double-tap arrives as a
// second finger of a phantom multi-touch, and pinch and rotate never
// get their Ended.
//
// The inputs are raw positions with Changed unset; the helper marks
// it by matching changed. Output never holds more than the fixed
// gui.Event.Touches array fits.
func touchEventFromLists(
	typ gui.EventType,
	all, changed []gui.TouchPoint,
) gui.Event {
	src := all
	if typ == gui.EventTouchesEnded ||
		typ == gui.EventTouchesCancelled {
		src = changed
	}
	var evt gui.Event
	evt.Type = typ
	evt.NumTouches = min(len(src), len(evt.Touches))
	for i := range evt.NumTouches {
		evt.Touches[i] = src[i]
	}
	for _, c := range changed {
		for j := range evt.NumTouches {
			if evt.Touches[j].Identifier == c.Identifier {
				evt.Touches[j].Changed = true
				break
			}
		}
	}
	return evt
}
