//go:build ios

package ios

import "github.com/go-gui-org/go-gui/gui"

// touchEvent dispatches a raw touch event to the window's event
// pipeline. The gesture recognizer in gui/gesture.go processes
// these into gesture events and synthesized mouse events.
func touchEvent(typ gui.EventType, id uint64, x, y float32) {
	if iosWindow == nil {
		return
	}
	evt := gui.Event{
		Type:       typ,
		NumTouches: 1,
		Touches: [8]gui.TouchPoint{{
			Identifier: id,
			PosX:       x,
			PosY:       y,
			ToolType:   gui.TouchToolFinger,
			Changed:    true,
		}},
	}
	iosWindow.EventFn(&evt)
}
