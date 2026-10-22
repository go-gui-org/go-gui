//go:build js && wasm

package web

import (
	"math"
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
	target.Set("hasAttribute", bareHasAttribute)
	e := obj.New()
	e.Set("target", target)
	e.Set("preventDefault", barePreventDefault)
	return e
}

// The methods every bareEvent shares. Built once for the test binary
// rather than once per call: a js.FuncOf holds a slot in the wasm
// callback table until it is released, and these are never released.
var (
	bareHasAttribute = js.FuncOf(func(_ js.Value, args []js.Value) any {
		return args[0].String() == imeInputAttr
	})
	barePreventDefault = js.FuncOf(func(_ js.Value, _ []js.Value) any { return nil })
)

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

// A script can dispatch a mouse event whose offsetX is NaN or ±Inf
// (Object.defineProperty). Such a value would reach hit tests and
// scroll offsets; jsFloat reads it as 0 (#1131).
func TestJSFloatNonFiniteIsZero(t *testing.T) {
	inf := math.Inf(1)
	// 1e300 is finite in float64 but becomes +Inf in the float32 cast
	// to MouseX.
	for _, v := range []float64{math.NaN(), inf, -inf, 1e300, -1e300} {
		e := bareEvent()
		e.Set("offsetX", v)
		e.Set("button", 0)
		if got := mouseButtonEvent(gui.EventMouseDown, e).MouseX; got != 0 {
			t.Fatalf("offsetX %v mapped to MouseX %v, want 0", v, got)
		}
	}
	if got := jsFloat(js.ValueOf(2.5)); got != 2.5 {
		t.Fatalf("jsFloat(2.5) = %v, want 2.5", got)
	}
}

// Value.Int is int(v.Float()): a NaN, ±Inf or huge keyCode, deltaMode
// or identifier would arrive as a saturated int, not 0 (#1131).
func TestJSIntNonFiniteIsZero(t *testing.T) {
	inf := math.Inf(1)
	for _, v := range []float64{math.NaN(), inf, -inf, 1e300} {
		if got := jsInt(js.ValueOf(v)); got != 0 {
			t.Fatalf("jsInt(%v) = %v, want 0", v, got)
		}
	}
	if got := jsInt(js.ValueOf(13)); got != 13 {
		t.Fatalf("jsInt(13) = %v, want 13", got)
	}
}

// A script can give a fake touch list a huge length and no entries.
// readTouchList skips holes without counting them, so it would scan
// the whole length and hang the page (#1131). Wasm has one thread, so
// a hang here shows as the go test timeout, not a t.Fatal.
func TestTouchListHugeLengthReturns(t *testing.T) {
	for _, n := range []float64{math.Inf(1), 1e9} {
		list := js.Global().Get("Object").New()
		list.Set("length", n)
		var pts [8]gui.TouchPoint
		if got := readTouchList(list, 0, 0, &pts); got != 0 {
			t.Fatalf("length %v: read %d touches, want 0", n, got)
		}
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
