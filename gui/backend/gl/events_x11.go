//go:build linux && !js && !android

package gl

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ibus"
	"github.com/go-gui-org/go-gui/gui/backend/internal/x11key"
)

// emit dispatches an event, reusing a single gui.Event stored on the
// platform state to avoid a heap allocation per event. EventFn does not
// retain the pointer beyond the call.
func (b *Backend) emit(e gui.Event) {
	b.plat.evt = e
	b.plat.w.EventFn(&b.plat.evt)
}

// logicalXY converts physical pixel coordinates to logical points using
// the current DPI scale.
func (b *Backend) logicalXY(px, py int32) (float32, float32) {
	s := b.dpiScale
	if s <= 0 {
		s = 1
	}
	return float32(px) / s, float32(py) / s
}

// keysym looks up the keysym for a keycode at the given shift column.
func (p *platformState) keysym(code xproto.Keycode, col int) uint32 {
	if p.keymap == nil {
		return 0
	}
	per := int(p.keymap.KeysymsPerKeycode)
	if per == 0 {
		return 0
	}
	idx := (int(code)-int(p.minKeycode))*per + col
	if idx < 0 || idx >= len(p.keymap.Keysyms) {
		return 0
	}
	return uint32(p.keymap.Keysyms[idx])
}

// handleXEvent translates an X event to a gui.Event and dispatches it.
//
//nolint:gocyclo // event dispatch switch
func (b *Backend) handleXEvent(ev xgb.Event) {
	w := b.plat.w
	switch e := ev.(type) {
	case xproto.KeyPressEvent:
		col := keysymColumn(e.State)
		// A key the input method claims belongs entirely to it: while a
		// composition is live the engine owns the arrows, Return and
		// Backspace too, so neither the key event nor the character may
		// also reach the widget.
		if b.imeProcessKey(b.plat.keysym(e.Detail, col), e.Detail, e.State) {
			return
		}
		b.emit(gui.Event{
			Type:      gui.EventKeyDown,
			KeyCode:   x11key.MapKeySym(b.plat.keysym(e.Detail, 0)),
			Modifiers: x11key.MapModifiers(e.State),
		})
		// Dead keys and Multi_key compose here; the machine returns
		// the rune to emit (or 0 when the key must produce nothing).
		if r := b.plat.compose.feed(b.plat.keysym(e.Detail, col)); r != 0 {
			b.emitChar(r, e.State)
		}

	case xproto.KeyReleaseEvent:
		// Releases are forwarded but never suppress the key-up: engines
		// use them for modifier-only toggles (Shift to switch kana, say)
		// and nothing else depends on the result. The keysym uses the
		// same shift column as the press so the engine sees matching
		// press/release pairs.
		if b.plat.ime != nil {
			b.plat.ime.ProcessKeyRelease(
				b.plat.keysym(e.Detail, keysymColumn(e.State)),
				uint32(e.Detail)-x11KeycodeOffset,
				uint32(e.State),
			)
		}
		b.emit(gui.Event{
			Type:      gui.EventKeyUp,
			KeyCode:   x11key.MapKeySym(b.plat.keysym(e.Detail, 0)),
			Modifiers: x11key.MapModifiers(e.State),
		})

	case xproto.ButtonPressEvent:
		switch e.Detail {
		case 4:
			b.emitScroll(0, x11ScrollLines, e.EventX, e.EventY, e.State)
		case 5:
			b.emitScroll(0, -x11ScrollLines, e.EventX, e.EventY, e.State)
		case 6:
			b.emitScroll(x11ScrollLines, 0, e.EventX, e.EventY, e.State)
		case 7:
			b.emitScroll(-x11ScrollLines, 0, e.EventX, e.EventY, e.State)
		default:
			// Remember the press for a later StartWindowDrag /
			// StartWindowResize, which reports it to the WM.
			b.plat.pressRootX = e.RootX
			b.plat.pressRootY = e.RootY
			b.plat.pressButton = byte(e.Detail)
			b.plat.havePress = true
			x, y := b.logicalXY(int32(e.EventX), int32(e.EventY))
			b.emit(gui.Event{
				Type:        gui.EventMouseDown,
				MouseX:      x,
				MouseY:      y,
				MouseButton: x11key.MapButton(byte(e.Detail)),
				Modifiers:   x11key.MapModifiers(e.State),
			})
		}

	case xproto.ButtonReleaseEvent:
		if e.Detail >= 1 && e.Detail <= 3 {
			x, y := b.logicalXY(int32(e.EventX), int32(e.EventY))
			b.emit(gui.Event{
				Type:        gui.EventMouseUp,
				MouseX:      x,
				MouseY:      y,
				MouseButton: x11key.MapButton(byte(e.Detail)),
				Modifiers:   x11key.MapModifiers(e.State),
			})
		}

	case xproto.MotionNotifyEvent:
		x, y := b.logicalXY(int32(e.EventX), int32(e.EventY))
		dx, dy := b.mouseDelta(x, y)
		b.emit(gui.Event{
			Type:      gui.EventMouseMove,
			MouseX:    x,
			MouseY:    y,
			MouseDX:   dx,
			MouseDY:   dy,
			Modifiers: x11key.MapModifiers(e.State),
		})

	case xproto.ConfigureNotifyEvent:
		scaleChanged := b.maybeRescaleDPI()
		if !b.plat.haveRandr {
			// maybeRescaleDPI refreshes the cached root position only
			// when RandR is present. Without it, invalidate here so the
			// IME caret rect is recomputed against the new position.
			b.plat.haveLastPos = false
		}
		nw, nh := int32(e.Width), int32(e.Height)
		if !scaleChanged && nw == b.plat.physW && nh == b.plat.physH {
			return
		}
		b.plat.physW = nw
		b.plat.physH = nh
		b.handleResize()
		lw, lh := b.logicalXY(b.physW, b.physH)
		b.emit(gui.Event{
			Type:         gui.EventResized,
			WindowWidth:  int(lw),
			WindowHeight: int(lh),
		})
		w.FrameFn()
		b.renderFrame(w)

	case xproto.ClientMessageEvent:
		if e.Format == 32 && len(e.Data.Data32) > 0 &&
			xproto.Atom(e.Data.Data32[0]) == b.plat.wmDelete {
			gui.DispatchCloseRequest(w)
		}

	case xproto.FocusInEvent:
		if focusRealChange(e.Mode, e.Detail) {
			b.emit(gui.Event{Type: gui.EventFocused})
		}

	case xproto.FocusOutEvent:
		if focusRealChange(e.Mode, e.Detail) {
			// An open dead-key / Multi_key sequence must not survive a
			// focus change; X11 cancels in-flight composition there.
			b.plat.compose.reset()
			b.emit(gui.Event{Type: gui.EventUnfocused})
		}

	case xproto.SelectionRequestEvent:
		b.plat.serveSelectionRequest(e)

	case xproto.SelectionClearEvent:
		// Only the selection actually taken from us is released. Clearing
		// both would make a getClipboard after losing PRIMARY go out to the
		// server for text we still own.
		if _, owns, ok := b.plat.selectionState(e.Selection); ok {
			*owns = false
		}

	case xproto.ExposeEvent:
		// Damage is repainted by the next frame; nothing to do.
	}
}

// focusRealChange reports whether an X focus event reflects a genuine
// keyboard-focus change rather than one synthesized by a pointer grab
// (e.g. an interactive WM move/resize) or by pointer motion. Grab-induced
// focus churn must be ignored, otherwise focus-gated events such as
// EventResized are dropped while the window is being resized, leaving the
// layout stale until the next event.
func focusRealChange(mode, detail byte) bool {
	if mode == xproto.NotifyModeGrab || mode == xproto.NotifyModeUngrab {
		return false
	}
	switch detail {
	case xproto.NotifyDetailPointer,
		xproto.NotifyDetailPointerRoot,
		xproto.NotifyDetailNone:
		return false
	}
	return true
}

// x11ScrollLines is the lines-per-notch reported for core X11 wheel
// buttons 4–7. Unlike Windows (SPI_GETWHEELSCROLLLINES) and macOS
// (AppKit acceleration), core X11 button events carry no magnitude and
// the toolkit is expected to supply one — three lines is both the
// Windows default and what GTK and Qt use, so it is what gui.Event's
// line unit means here.
const x11ScrollLines float32 = 3

func (b *Backend) emitScroll(sx, sy float32, px, py int16, state uint16) {
	x, y := b.logicalXY(int32(px), int32(py))
	b.emit(gui.Event{
		Type:      gui.EventMouseScroll,
		ScrollX:   sx,
		ScrollY:   sy,
		MouseX:    x,
		MouseY:    y,
		Modifiers: x11key.MapModifiers(state),
	})
}

func (b *Backend) emitChar(r rune, state uint16) {
	if r == 0xFFFD {
		return
	}
	b.emit(gui.Event{
		Type:      gui.EventChar,
		CharCode:  uint32(r),
		IMEText:   string(r),
		Modifiers: x11key.MapModifiers(state),
	})
}

// --- IME ---
//
// Input methods are reached over D-Bus (see gui/backend/ibus) rather
// than through XIM. When no input method is running plat.ime is nil and
// every call below is a no-op, leaving the keysym path above untouched.

// x11KeycodeOffset converts an X11 keycode to the evdev code IBus wants.
const x11KeycodeOffset = 8

// keysymColumn returns the shift column for the given modifier state:
// column 0 is the unshifted keysym, column 1 the shifted one.
func keysymColumn(state uint16) int {
	if state&xproto.KeyButMaskShift != 0 {
		return 1
	}
	return 0
}

// imeProcessKey offers a key press to the input method and reports
// whether it was consumed.
//
// The queue is drained on both sides of the call so composition results
// interleave with key events in the order the user produced them: a
// commit queued by the previous keystroke must not land after this one.
func (b *Backend) imeProcessKey(keyval uint32, code xproto.Keycode, state uint16) bool {
	if b.plat.ime == nil {
		return false
	}
	b.drainIME()
	consumed := b.plat.ime.ProcessKey(
		keyval, uint32(code)-x11KeycodeOffset, uint32(state),
	)
	b.drainIME()
	return consumed
}

// drainIME turns queued input-method results into gui events. The queue
// is filled from a D-Bus goroutine; this must only run on the main
// thread, where EventFn is safe to call.
func (b *Backend) drainIME() {
	if b.plat.ime == nil {
		return
	}
	b.plat.imeBuf = b.plat.ime.Drain(b.plat.imeBuf[:0])
	b.plat.imeEvts = imeEvents(b.plat.imeBuf, b.plat.imeEvts[:0])
	for i := range b.plat.imeEvts {
		b.emit(b.plat.imeEvts[i])
	}
}

// imeEvents converts input-method results to gui events, appending to
// dst. Kept separate from dispatch so the mapping is testable without
// an X connection or a running input method.
func imeEvents(in []ibus.Event, dst []gui.Event) []gui.Event {
	for i := range in {
		ev := &in[i]
		switch ev.Kind {
		case ibus.KindPreedit:
			dst = append(dst, gui.Event{
				Type:      gui.EventIMEComposition,
				IMEText:   ev.Text,
				IMEStart:  ev.Cursor,
				IMELength: ev.SelLen,
			})

		case ibus.KindCommit:
			// Committed text is plain input: no modifiers apply, and a
			// decoding failure upstream must not reach the widget.
			for _, r := range ev.Text {
				if r == 0xFFFD {
					continue
				}
				dst = append(dst, gui.Event{
					Type:     gui.EventChar,
					CharCode: uint32(r),
					IMEText:  string(r),
				})
			}

		case ibus.KindForwardKey:
			state := uint16(ev.State & 0xffff)
			dst = append(dst, gui.Event{
				Type:      gui.EventKeyDown,
				KeyCode:   x11key.MapKeySym(ev.Keyval),
				Modifiers: x11key.MapModifiers(state),
			})
			if r := x11key.KeysymToRune(ev.Keyval); r >= 0x20 && r != 0x7f {
				dst = append(dst, gui.Event{
					Type:      gui.EventChar,
					CharCode:  uint32(r),
					IMEText:   string(r),
					Modifiers: x11key.MapModifiers(state),
				})
			}
		}
	}
	return dst
}

// rootOrigin returns the window's origin in root coordinates, the frame
// the caret rect must be reported in. maybeRescaleDPI keeps the cached
// value fresh on every move, so this normally costs nothing.
func (b *Backend) rootOrigin() (int16, int16) {
	if !b.plat.haveLastPos {
		t, err := xproto.TranslateCoordinates(
			b.plat.conn, b.plat.window, b.plat.root, 0, 0,
		).Reply()
		if err == nil && t != nil {
			b.plat.lastRootX, b.plat.lastRootY = t.DstX, t.DstY
			b.plat.haveLastPos = true
		}
	}
	return b.plat.lastRootX, b.plat.lastRootY
}

func (n *nativePlatform) IMEStart() { n.b.plat.ime.FocusIn() }

// IMEStop drops the input method's focus. No composition event is
// emitted here: the gui layer already clears its own state on a focus
// change (Window.setFocusID), and re-entering EventFn from inside an
// event handler would clobber the event being dispatched.
func (n *nativePlatform) IMEStop() { n.b.plat.ime.FocusOut() }

// IMESetRect reports the caret rect so the candidate window can anchor
// to it. The gui layer works in logical points relative to the window;
// IBus wants physical pixels relative to the root.
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	b := n.b
	if b.plat.ime == nil {
		return
	}
	s := b.dpiScale
	if s <= 0 {
		s = 1
	}
	rx, ry := b.rootOrigin()
	b.plat.ime.SetCursorLocation(
		int32(rx)+int32(float32(x)*s),
		int32(ry)+int32(float32(y)*s),
		int32(float32(w)*s),
		int32(float32(h)*s),
	)
}
