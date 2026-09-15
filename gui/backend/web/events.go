//go:build js && wasm

package web

import (
	"strings"
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
			data := imeSanitizeText(jsString(e.Get("data")))
			start, length := imeClauseFromTarget(
				e.Get("target"), data)
			*evt = gui.Event{
				Type:      gui.EventIMEComposition,
				IMEText:   data,
				IMEStart:  start,
				IMELength: length,
			}
			w.EventFn(evt)
			return nil
		})

	reg(js.Global(), "compositionend",
		func(_ js.Value, args []js.Value) any {
			e := args[0]
			text := imeSanitizeText(jsString(e.Get("data")))
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

// jsString reads a JS string property without coercion: a missing
// value (null or undefined) yields "" rather than the literals
// "null" or "undefined" that Value.String would produce, which would
// otherwise arrive as a bogus one-word composition.
func jsString(v js.Value) string {
	if v.Type() != js.TypeString {
		return ""
	}
	return v.String()
}

// maxIMETextRunes bounds a composition or commit string from the
// browser, which arrives with no length promised. Mirrors
// maxIMEPreeditRunes in gui/ime.go; a phrase is far past any real
// input.
const maxIMETextRunes = 4096

// imeSanitizeText strips decoding failures and caps the length of a
// composition or commit string, returning empty when nothing usable
// remains. An empty composition still emits (it ends the preedit);
// an empty commit emits nothing.
func imeSanitizeText(s string) string {
	if strings.ContainsRune(s, 0xFFFD) {
		s = strings.Map(func(r rune) rune {
			if r == 0xFFFD {
				return -1
			}
			return r
		}, s)
	}
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > maxIMETextRunes {
		i := 0
		for range maxIMETextRunes {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
		}
		s = s[:i]
	}
	return s
}

// imeClauseFromTarget reads the selected clause of a live composition
// from the hidden input's selection. During compositionupdate the
// input holds the preedit and its selection marks the clause the IME
// selected for conversion; a collapsed selection is the cursor.
// Anything unexpected — no target, no numeric selection — yields the
// zero clause rather than a guess.
//
// Selection offsets arrive in UTF-16 units and leave in characters:
// imeUpdate clamps, but a surrogate pair before the clause would
// still shift the range by one, the bug PR #210 fixed on macOS.
func imeClauseFromTarget(target js.Value, data string) (int32, int32) {
	if !target.Truthy() {
		return 0, 0
	}
	ss := target.Get("selectionStart")
	se := target.Get("selectionEnd")
	if ss.Type() != js.TypeNumber || se.Type() != js.TypeNumber {
		return 0, 0
	}
	start := utf16RuneIndex(data, ss.Int())
	end := max(utf16RuneIndex(data, se.Int()), start)
	return start, end - start
}

// utf16RuneIndex converts a UTF-16 code-unit offset into s to a rune
// offset, clamping to the string. A rune counts once its first unit
// is reached, so an offset inside a surrogate pair lands after the
// astral character, matching the Win32 imeRuneIndex rule.
func utf16RuneIndex(s string, n int) int32 {
	var count, units int32
	for _, r := range s {
		if units >= int32(n) {
			break
		}
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
		count++
	}
	return count
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
	// Extracted into fixed arrays, not slices: this runs per touch
	// event, so it must not allocate on the heap. The arrays live
	// on the stack and touchEventFromLists copies them out.
	var allPts, changedPts [8]gui.TouchPoint
	all := e.Get("touches")
	changed := e.Get("changedTouches")
	an := min(all.Length(), len(allPts))
	for i := range an {
		t := all.Index(i)
		allPts[i] = gui.TouchPoint{
			Identifier: uint64(t.Get("identifier").Int()),
			PosX:       float32(t.Get("clientX").Float() - left),
			PosY:       float32(t.Get("clientY").Float() - top),
			ToolType:   gui.TouchToolFinger,
		}
	}
	cn := min(changed.Length(), len(changedPts))
	for i := range cn {
		t := changed.Index(i)
		changedPts[i] = gui.TouchPoint{
			Identifier: uint64(t.Get("identifier").Int()),
			PosX:       float32(t.Get("clientX").Float() - left),
			PosY:       float32(t.Get("clientY").Float() - top),
			ToolType:   gui.TouchToolFinger,
		}
	}
	// Which list the event carries is decided in one place,
	// touchEventFromLists: Ended and Cancelled carry the lifted
	// fingers (changed), the rest carry the fingers still down.
	*evt = touchEventFromLists(typ, allPts[:an], changedPts[:cn])
}
