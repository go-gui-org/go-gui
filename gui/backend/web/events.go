//go:build js && wasm

package web

import (
	"syscall/js"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

// Wheel delta normalization constants. Converts browser delta values to
// gui.Event's scroll unit, lines of text (see Event.ScrollY):
//   - DOM_DELTA_PIXEL: ~17.7px per line, so a 53px trackpad notch stays
//     the three lines every other backend reports
//   - DOM_DELTA_LINE: already lines; passed through
//   - DOM_DELTA_PAGE: a page is ~30 lines
//
// The ratios are unchanged from when this produced "notches" — only the
// unit is scaled, so web scrolling feels exactly as it did.
const (
	wheelPixelDivisor   = 17.7
	wheelLineDivisor    = 1
	wheelPageMultiplier = 30
)

// registerEvents attaches DOM event listeners to the canvas and
// window. Registered callbacks are appended to b.callbacks to
// prevent garbage collection.
func (b *Backend) registerEvents(w *gui.Window) {
	doc := js.Global().Get("document")
	canvas := b.canvas

	reg := func(target js.Value, name string,
		fn func(js.Value, []js.Value) any) {
		f := js.FuncOf(fn)
		b.callbacks = append(b.callbacks, f)
		target.Call("addEventListener", name, f)
	}

	// Single shared Event — safe in WASM's single-threaded JS
	// runtime. Must not be read from goroutines.
	evt := new(gui.Event)

	reg(canvas, "mousedown", func(_ js.Value, args []js.Value) any {
		e := args[0]
		*evt = gui.Event{
			Type:        gui.EventMouseDown,
			MouseX:      float32(e.Get("offsetX").Float()),
			MouseY:      float32(e.Get("offsetY").Float()),
			MouseButton: mapMouseButton(e.Get("button").Int()),
			Modifiers:   mapModifiers(e),
		}
		w.EventFn(evt)
		return nil
	})

	reg(canvas, "mouseup", func(_ js.Value, args []js.Value) any {
		e := args[0]
		*evt = gui.Event{
			Type:        gui.EventMouseUp,
			MouseX:      float32(e.Get("offsetX").Float()),
			MouseY:      float32(e.Get("offsetY").Float()),
			MouseButton: mapMouseButton(e.Get("button").Int()),
			Modifiers:   mapModifiers(e),
		}
		w.EventFn(evt)
		return nil
	})

	reg(canvas, "mousemove", func(_ js.Value, args []js.Value) any {
		e := args[0]
		*evt = gui.Event{
			Type:      gui.EventMouseMove,
			MouseX:    float32(e.Get("offsetX").Float()),
			MouseY:    float32(e.Get("offsetY").Float()),
			MouseDX:   float32(e.Get("movementX").Float()),
			MouseDY:   float32(e.Get("movementY").Float()),
			Modifiers: mapModifiers(e),
		}
		w.EventFn(evt)
		return nil
	})

	reg(canvas, "wheel", func(_ js.Value, args []js.Value) any {
		e := args[0]
		e.Call("preventDefault")
		dx := e.Get("deltaX").Float()
		dy := e.Get("deltaY").Float()
		switch e.Get("deltaMode").Int() {
		case 0: // DOM_DELTA_PIXEL
			dx /= wheelPixelDivisor
			dy /= wheelPixelDivisor
		case 1: // DOM_DELTA_LINE
			dx /= wheelLineDivisor
			dy /= wheelLineDivisor
		case 2: // DOM_DELTA_PAGE
			dx *= wheelPageMultiplier
			dy *= wheelPageMultiplier
		}
		*evt = gui.Event{
			Type:      gui.EventMouseScroll,
			ScrollX:   -float32(dx),
			ScrollY:   -float32(dy),
			MouseX:    float32(e.Get("offsetX").Float()),
			MouseY:    float32(e.Get("offsetY").Float()),
			Modifiers: mapModifiers(e),
		}
		w.EventFn(evt)
		return nil
	})

	reg(canvas, "mouseenter", func(_ js.Value, _ []js.Value) any {
		*evt = gui.Event{Type: gui.EventMouseEnter}
		w.EventFn(evt)
		return nil
	})

	reg(canvas, "mouseleave", func(_ js.Value, _ []js.Value) any {
		*evt = gui.Event{Type: gui.EventMouseLeave}
		w.EventFn(evt)
		return nil
	})

	reg(doc, "keydown", func(_ js.Value, args []js.Value) any {
		e := args[0]

		// While a composition is live the input method owns the
		// keyboard — arrows move between clauses, Enter commits,
		// Escape reverts — and the browser still fires keydown for
		// those keys (key == "Process", isComposing == true). Let
		// them through and the field's own caret moves out from
		// under the preedit, or Enter submits mid-composition.
		// Truthy, not Bool: Bool panics on a browser that predates
		// the property and leaves it undefined.
		if e.Get("isComposing").Truthy() {
			return nil
		}

		code := e.Get("code").String()
		key := e.Get("key").String()
		mods := mapModifiers(e)

		// Prevent browser defaults for navigation keys.
		if shouldPreventDefault(code) {
			e.Call("preventDefault")
		}

		// Key event.
		kc := mapKeyCode(code)
		*evt = gui.Event{
			Type:      gui.EventKeyDown,
			KeyCode:   kc,
			Modifiers: mods,
			KeyRepeat: e.Get("repeat").Bool(),
		}
		w.EventFn(evt)

		// Generate char event for printable single-rune keys.
		// Multi-byte single-rune input (e.g. emoji via keyboard
		// shortcut) is excluded here; IME-based emoji is handled
		// by the compositionend listener.
		if len(key) > 0 && !e.Get("ctrlKey").Bool() &&
			!e.Get("metaKey").Bool() {
			r, sz := utf8.DecodeRuneInString(key)
			if r != utf8.RuneError && sz == len(key) &&
				r >= 32 && r != 127 {
				*evt = gui.Event{
					Type:      gui.EventChar,
					CharCode:  uint32(r),
					IMEText:   key,
					Modifiers: mods,
				}
				w.EventFn(evt)
			}
		}
		return nil
	})

	reg(doc, "keyup", func(_ js.Value, args []js.Value) any {
		e := args[0]
		*evt = gui.Event{
			Type:      gui.EventKeyUp,
			KeyCode:   mapKeyCode(e.Get("code").String()),
			Modifiers: mapModifiers(e),
		}
		w.EventFn(evt)
		return nil
	})

	reg(js.Global(), "compositionupdate",
		func(_ js.Value, args []js.Value) any {
			e := args[0]
			*evt = gui.Event{
				Type:    gui.EventIMEComposition,
				IMEText: e.Get("data").String(),
			}
			w.EventFn(evt)
			return nil
		})

	reg(js.Global(), "compositionend",
		func(_ js.Value, args []js.Value) any {
			e := args[0]
			text := e.Get("data").String()
			if len(text) == 0 {
				// Cancelled composition (Escape). Report the end
				// so the preedit clears; without it the overlay
				// can outlive the composition.
				*evt = gui.Event{Type: gui.EventIMEComposition}
				w.EventFn(evt)
				return nil
			}
			// CharCode carries only the first rune; the full
			// committed string is in IMEText for multi-char
			// input (e.g. Chinese phrases).
			r, _ := utf8.DecodeRuneInString(text)
			*evt = gui.Event{
				Type:     gui.EventChar,
				CharCode: uint32(r),
				IMEText:  text,
			}
			w.EventFn(evt)
			return nil
		})

	reg(js.Global(), "resize", func(_ js.Value, _ []js.Value) any {
		ww := js.Global().Get("innerWidth").Int()
		wh := js.Global().Get("innerHeight").Int()
		b.resizeCanvas(ww, wh)
		*evt = gui.Event{
			Type:         gui.EventResized,
			WindowWidth:  ww,
			WindowHeight: wh,
		}
		w.EventFn(evt)
		return nil
	})

	reg(js.Global(), "focus", func(_ js.Value, _ []js.Value) any {
		*evt = gui.Event{Type: gui.EventFocused}
		w.EventFn(evt)
		return nil
	})

	reg(js.Global(), "blur", func(_ js.Value, _ []js.Value) any {
		*evt = gui.Event{Type: gui.EventUnfocused}
		w.EventFn(evt)
		return nil
	})

	reg(doc, "paste", func(_ js.Value, args []js.Value) any {
		e := args[0]
		cd := e.Get("clipboardData")
		if !cd.IsNull() && !cd.IsUndefined() {
			b.lastPasteText = cd.Call("getData", "text/plain").String()
		}
		*evt = gui.Event{Type: gui.EventClipboardPasted}
		w.EventFn(evt)
		return nil
	})

	// Prevent default context menu on canvas.
	reg(canvas, "contextmenu", func(_ js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		return nil
	})

	// Touch events — map to framework touch event types.
	touchHandler := func(typ gui.EventType) func(js.Value, []js.Value) any {
		return func(_ js.Value, args []js.Value) any {
			e := args[0]
			e.Call("preventDefault")
			mapTouchEvent(b.canvasLeft, b.canvasTop, e, typ, evt)
			w.EventFn(evt)
			return nil
		}
	}
	reg(canvas, "touchstart", touchHandler(gui.EventTouchesBegan))
	reg(canvas, "touchmove", touchHandler(gui.EventTouchesMoved))
	reg(canvas, "touchend", touchHandler(gui.EventTouchesEnded))
	reg(canvas, "touchcancel", touchHandler(gui.EventTouchesCancelled))
}

func mapMouseButton(b int) gui.MouseButton {
	switch b {
	case 0:
		return gui.MouseLeft
	case 1:
		return gui.MouseMiddle
	case 2:
		return gui.MouseRight
	default:
		return gui.MouseInvalid
	}
}

func mapModifiers(e js.Value) gui.Modifier {
	var m gui.Modifier
	if e.Get("shiftKey").Bool() {
		m |= gui.ModShift
	}
	if e.Get("ctrlKey").Bool() {
		m |= gui.ModCtrl
	}
	if e.Get("altKey").Bool() {
		m |= gui.ModAlt
	}
	if e.Get("metaKey").Bool() {
		m |= gui.ModSuper
	}
	// JS MouseEvent.buttons bitmask: 1=LMB, 2=RMB, 4=MMB.
	// Guard against KeyboardEvent which lacks .buttons.
	if b := e.Get("buttons"); b.Type() == js.TypeNumber {
		buttons := b.Int()
		if buttons&1 != 0 {
			m |= gui.ModLMB
		}
		if buttons&2 != 0 {
			m |= gui.ModRMB
		}
		if buttons&4 != 0 {
			m |= gui.ModMMB
		}
	}
	return m
}

func shouldPreventDefault(code string) bool {
	switch code {
	case "Tab", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight",
		"Backspace", "Space":
		return true
	}
	return false
}

// keyCodes maps DOM KeyboardEvent.code values to gui.KeyCode. The zero
// value of gui.KeyCode is gui.KeyInvalid, which the map lookup returns
// naturally for any unmapped code, so no explicit default is needed.
var keyCodes = map[string]gui.KeyCode{
	"Space":        gui.KeySpace,
	"Enter":        gui.KeyEnter,
	"NumpadEnter":  gui.KeyEnter,
	"Escape":       gui.KeyEscape,
	"Tab":          gui.KeyTab,
	"Backspace":    gui.KeyBackspace,
	"Delete":       gui.KeyDelete,
	"Insert":       gui.KeyInsert,
	"ArrowRight":   gui.KeyRight,
	"ArrowLeft":    gui.KeyLeft,
	"ArrowDown":    gui.KeyDown,
	"ArrowUp":      gui.KeyUp,
	"PageUp":       gui.KeyPageUp,
	"PageDown":     gui.KeyPageDown,
	"Home":         gui.KeyHome,
	"End":          gui.KeyEnd,
	"ShiftLeft":    gui.KeyLeftShift,
	"ShiftRight":   gui.KeyRightShift,
	"ControlLeft":  gui.KeyLeftControl,
	"ControlRight": gui.KeyRightControl,
	"AltLeft":      gui.KeyLeftAlt,
	"AltRight":     gui.KeyRightAlt,
	"MetaLeft":     gui.KeyLeftSuper,
	"MetaRight":    gui.KeyRightSuper,
	"Comma":        gui.KeyComma,
	"Minus":        gui.KeyMinus,
	"Period":       gui.KeyPeriod,
	"Slash":        gui.KeySlash,
	"Semicolon":    gui.KeySemicolon,
	"Equal":        gui.KeyEqual,
	"BracketLeft":  gui.KeyLeftBracket,
	"Backslash":    gui.KeyBackslash,
	"BracketRight": gui.KeyRightBracket,
	"Backquote":    gui.KeyGraveAccent,
	"CapsLock":     gui.KeyCapsLock,
	"F1":           gui.KeyF1,
	"F2":           gui.KeyF2,
	"F3":           gui.KeyF3,
	"F4":           gui.KeyF4,
	"F5":           gui.KeyF5,
	"F6":           gui.KeyF6,
	"F7":           gui.KeyF7,
	"F8":           gui.KeyF8,
	"F9":           gui.KeyF9,
	"F10":          gui.KeyF10,
	"F11":          gui.KeyF11,
	"F12":          gui.KeyF12,
}

func mapKeyCode(code string) gui.KeyCode {
	// Single-letter keys: KeyA..KeyZ.
	if len(code) == 4 && code[:3] == "Key" {
		ch := code[3]
		if ch >= 'A' && ch <= 'Z' {
			return gui.KeyCode(ch)
		}
	}
	// Digit keys: Digit0..Digit9.
	if len(code) == 6 && code[:5] == "Digit" {
		ch := code[5]
		if ch >= '0' && ch <= '9' {
			return gui.KeyCode(ch)
		}
	}
	return keyCodes[code]
}

// cursorCSS maps gui.MouseCursor to CSS cursor values.
var cursorCSS = map[gui.MouseCursor]string{
	gui.CursorDefault:      "default",
	gui.CursorArrow:        "default",
	gui.CursorIBeam:        "text",
	gui.CursorCrosshair:    "crosshair",
	gui.CursorPointingHand: "pointer",
	gui.CursorResizeEW:     "ew-resize",
	gui.CursorResizeNS:     "ns-resize",
	gui.CursorResizeNWSE:   "nwse-resize",
	gui.CursorResizeNESW:   "nesw-resize",
	gui.CursorResizeAll:    "move",
	gui.CursorNotAllowed:   "not-allowed",
}

func mapTouchEvent(
	left, top float64,
	e js.Value,
	typ gui.EventType,
	evt *gui.Event,
) {
	all := e.Get("touches")
	changed := e.Get("changedTouches")
	// Cap at fixed-size Touches array (8 simultaneous touches).
	n := min(all.Length(), len(evt.Touches))

	*evt = gui.Event{Type: typ, NumTouches: n}
	for i := range n {
		t := all.Index(i)
		evt.Touches[i] = gui.TouchPoint{
			Identifier: uint64(t.Get("identifier").Int()),
			PosX:       float32(t.Get("clientX").Float() - left),
			PosY:       float32(t.Get("clientY").Float() - top),
			ToolType:   gui.TouchToolFinger,
		}
	}
	for i := range changed.Length() {
		cid := uint64(changed.Index(i).Get("identifier").Int())
		for j := range n {
			if evt.Touches[j].Identifier == cid {
				evt.Touches[j].Changed = true
				break
			}
		}
	}
}
