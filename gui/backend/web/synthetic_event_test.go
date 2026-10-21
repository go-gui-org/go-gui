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

// A bare mousedown or mouseup has no button field. Read as 0 it would
// be a left press at (0,0) and click whatever widget sits there; as
// MouseInvalid the window drops it (#1098).
func TestMouseButtonEventBareIsInvalid(t *testing.T) {
	for _, typ := range []gui.EventType{gui.EventMouseDown, gui.EventMouseUp} {
		if got := mouseButtonEvent(typ, bareEvent()).MouseButton; got != gui.MouseInvalid {
			t.Fatalf("%v from a bare Event: button %v, want MouseInvalid", typ, got)
		}
	}
	e := bareEvent()
	e.Set("button", 2)
	if got := mouseButtonEvent(gui.EventMouseDown, e).MouseButton; got != gui.MouseRight {
		t.Fatalf("button 2 mapped to %v, want MouseRight", got)
	}
}

// A change event built with new Event("change") has no matches field.
// Value.Bool on it panics and stops the program.
func TestAppearanceFromBareEvent(t *testing.T) {
	if got := appearanceFromEvent([]js.Value{bareEvent()}); got != gui.AppearanceLight {
		t.Fatalf("bare Event: %v, want AppearanceLight", got)
	}
	e := bareEvent()
	e.Set("matches", true)
	if got := appearanceFromEvent([]js.Value{e}); got != gui.AppearanceDark {
		t.Fatalf("matches=true: %v, want AppearanceDark", got)
	}
}

// A script can give a fake touch event a touch list with a hole or a
// null in it. Value.Get on that entry panics; the entry is skipped.
func TestTouchListSkipsHoles(t *testing.T) {
	obj := js.Global().Get("Object")
	pt := obj.New()
	pt.Set("identifier", 7)
	pt.Set("clientX", 3)
	pt.Set("clientY", 4)
	all := js.Global().Get("Array").New()
	all.Call("push", js.Null(), pt)
	hole := obj.New()
	hole.Set("length", 2)
	e := bareEvent()
	e.Set("touches", all)
	e.Set("changedTouches", hole)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panics on a touch list with a hole: %v", r)
		}
	}()
	var evt gui.Event
	mapTouchEvent(0, 0, e, gui.EventTouchesBegan, &evt)
	if evt.NumTouches != 1 || evt.Touches[0].Identifier != 7 ||
		evt.Touches[0].PosX != 3 || evt.Touches[0].PosY != 4 {
		t.Fatalf("got %d touches %+v, want one touch 7 at (3,4)",
			evt.NumTouches, evt.Touches[0])
	}
}
