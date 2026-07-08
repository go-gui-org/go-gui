//go:build linux && !js && !android

package gl

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
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
		b.emit(gui.Event{
			Type:      gui.EventKeyDown,
			KeyCode:   x11key.MapKeySym(b.plat.keysym(e.Detail, 0)),
			Modifiers: x11key.MapModifiers(e.State),
		})
		col := 0
		if e.State&xproto.KeyButMaskShift != 0 {
			col = 1
		}
		if r := x11key.KeysymToRune(b.plat.keysym(e.Detail, col)); r >= 0x20 && r != 0x7f {
			b.emitChar(r, e.State)
		}

	case xproto.KeyReleaseEvent:
		b.emit(gui.Event{
			Type:      gui.EventKeyUp,
			KeyCode:   x11key.MapKeySym(b.plat.keysym(e.Detail, 0)),
			Modifiers: x11key.MapModifiers(e.State),
		})

	case xproto.ButtonPressEvent:
		switch e.Detail {
		case 4:
			b.emitScroll(0, 1, e.EventX, e.EventY, e.State)
		case 5:
			b.emitScroll(0, -1, e.EventX, e.EventY, e.State)
		case 6:
			b.emitScroll(1, 0, e.EventX, e.EventY, e.State)
		case 7:
			b.emitScroll(-1, 0, e.EventX, e.EventY, e.State)
		default:
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
		b.emit(gui.Event{
			Type:      gui.EventMouseMove,
			MouseX:    x,
			MouseY:    y,
			Modifiers: x11key.MapModifiers(e.State),
		})

	case xproto.ConfigureNotifyEvent:
		nw, nh := int32(e.Width), int32(e.Height)
		if nw == b.plat.physW && nh == b.plat.physH {
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
			b.emit(gui.Event{Type: gui.EventUnfocused})
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

// --- IME (stub; KeyPress still yields EventChar for printable keys) ---

func (n *nativePlatform) IMEStart()                   {}
func (n *nativePlatform) IMEStop()                    {}
func (n *nativePlatform) IMESetRect(_, _, _, _ int32) {}
