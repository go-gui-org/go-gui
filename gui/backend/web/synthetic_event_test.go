//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// bareEvent is what new Event("keydown") gives a listener: a target and
// preventDefault, and none of the KeyboardEvent, MouseEvent or
// TouchEvent fields. Chrome's password autofill sends such keydown and
// keyup events to the hidden input.
func bareEvent() js.Value {
	obj := js.Global().Get("Object")
	target := obj.New()
	target.Set("hasAttribute", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return args[0].String() == imeInputAttr
	}))
	e := obj.New()
	e.Set("target", target)
	e.Set("preventDefault", js.FuncOf(func(_ js.Value, _ []js.Value) any { return nil }))
	return e
}

// A listener reads a missing field as its zero value. A panic there
// would stop the whole program and leave the page dead to input.
func TestListenersSurviveBareEvent(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(js.Value) []gui.Event
	}{
		{"keydown", func(e js.Value) []gui.Event {
			emit, got := recordEvents()
			handleKeyDown(emit, new(gui.Event), e)
			return *got
		}},
		{"keyup", func(e js.Value) []gui.Event {
			emit, got := recordEvents()
			handleKeyUp(emit, new(gui.Event), e)
			return *got
		}},
		{"touchstart", func(e js.Value) []gui.Event {
			var evt gui.Event
			mapTouchEvent(0, 0, e, gui.EventTouchesBegan, &evt)
			return []gui.Event{evt}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panics on a bare Event: %v", r)
				}
			}()
			for _, ev := range tc.run(bareEvent()) {
				if ev.Type == gui.EventChar {
					t.Fatalf("typed %q from a bare Event", ev.IMEText)
				}
				if ev.KeyCode != gui.KeyInvalid || ev.Modifiers != 0 || ev.NumTouches != 0 {
					t.Fatalf("bare Event mapped to %+v, want zero key, modifiers and touches", ev)
				}
			}
		})
	}
	if m := mapModifiers(bareEvent()); m != 0 {
		t.Fatalf("mapModifiers(bare Event) = %v, want 0", m)
	}
}
