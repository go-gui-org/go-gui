//go:build js && wasm

package web

import (
	"syscall/js"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

// Wheel delta normalization constants. Converts browser delta
// values to approximate scroll "notches":
//   - DOM_DELTA_PIXEL: ~53px per trackpad notch
//   - DOM_DELTA_LINE: ~3 lines per discrete wheel notch
//   - DOM_DELTA_PAGE: each page maps to ~10 notches
const (
	wheelPixelDivisor   = 53
	wheelLineDivisor    = 3
	wheelPageMultiplier = 10
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
	buttons := e.Get("buttons").Int()
	if buttons&1 != 0 {
		m |= gui.ModLMB
	}
	if buttons&2 != 0 {
		m |= gui.ModRMB
	}
	if buttons&4 != 0 {
		m |= gui.ModMMB
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

//nolint:gocyclo // key-mapping switch
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

	switch code {
	case "Space":
		return gui.KeySpace
	case "Enter", "NumpadEnter":
		return gui.KeyEnter
	case "Escape":
		return gui.KeyEscape
	case "Tab":
		return gui.KeyTab
	case "Backspace":
		return gui.KeyBackspace
	case "Delete":
		return gui.KeyDelete
	case "Insert":
		return gui.KeyInsert
	case "ArrowRight":
		return gui.KeyRight
	case "ArrowLeft":
		return gui.KeyLeft
	case "ArrowDown":
		return gui.KeyDown
	case "ArrowUp":
		return gui.KeyUp
	case "PageUp":
		return gui.KeyPageUp
	case "PageDown":
		return gui.KeyPageDown
	case "Home":
		return gui.KeyHome
	case "End":
		return gui.KeyEnd
	case "ShiftLeft":
		return gui.KeyLeftShift
	case "ShiftRight":
		return gui.KeyRightShift
	case "ControlLeft":
		return gui.KeyLeftControl
	case "ControlRight":
		return gui.KeyRightControl
	case "AltLeft":
		return gui.KeyLeftAlt
	case "AltRight":
		return gui.KeyRightAlt
	case "MetaLeft":
		return gui.KeyLeftSuper
	case "MetaRight":
		return gui.KeyRightSuper
	case "Comma":
		return gui.KeyComma
	case "Minus":
		return gui.KeyMinus
	case "Period":
		return gui.KeyPeriod
	case "Slash":
		return gui.KeySlash
	case "Semicolon":
		return gui.KeySemicolon
	case "Equal":
		return gui.KeyEqual
	case "BracketLeft":
		return gui.KeyLeftBracket
	case "Backslash":
		return gui.KeyBackslash
	case "BracketRight":
		return gui.KeyRightBracket
	case "Backquote":
		return gui.KeyGraveAccent
	case "CapsLock":
		return gui.KeyCapsLock
	case "F1":
		return gui.KeyF1
	case "F2":
		return gui.KeyF2
	case "F3":
		return gui.KeyF3
	case "F4":
		return gui.KeyF4
	case "F5":
		return gui.KeyF5
	case "F6":
		return gui.KeyF6
	case "F7":
		return gui.KeyF7
	case "F8":
		return gui.KeyF8
	case "F9":
		return gui.KeyF9
	case "F10":
		return gui.KeyF10
	case "F11":
		return gui.KeyF11
	case "F12":
		return gui.KeyF12
	default:
		return gui.KeyInvalid
	}
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
