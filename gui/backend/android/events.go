//go:build android

package android

import "github.com/go-gui-org/go-gui/gui"

// imeComposition dispatches an IME preedit composition event.
func imeComposition(text string, cursor, selLen int32) {
	if androidWindow == nil {
		return
	}
	evt := gui.Event{
		Type:      gui.EventIMEComposition,
		IMEText:   text,
		IMEStart:  cursor,
		IMELength: selLen,
	}
	androidWindow.EventFn(&evt)
}

// imeCommit dispatches committed text as individual EventChar
// events, one per rune.
func imeCommit(text string) {
	if androidWindow == nil {
		return
	}
	for _, r := range text {
		evt := gui.Event{
			Type:     gui.EventChar,
			CharCode: uint32(r),
		}
		androidWindow.EventFn(&evt)
	}
}

// touchEvent dispatches a raw touch event to the window's event
// pipeline. The gesture recognizer in gui/gesture.go processes
// these into gesture events and synthesized mouse events.
func touchEvent(typ gui.EventType, id uint64, x, y float32) {
	if androidWindow == nil {
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
	androidWindow.EventFn(&evt)
}
