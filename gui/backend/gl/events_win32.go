//go:build windows && !js

package gl

import (
	"unicode/utf16"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/winkey"
)

// Win32 window messages (subset).
const (
	wmSize        = 0x0005
	wmSetFocus    = 0x0007
	wmKillFocus   = 0x0008
	wmPaint       = 0x000F
	wmClose       = 0x0010
	wmEraseBkgnd  = 0x0014
	wmSetCursor   = 0x0020
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmChar        = 0x0102
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmMButtonDown = 0x0207
	wmMButtonUp   = 0x0208
	wmMouseWheel  = 0x020A
	wmMouseHWheel = 0x020E
	wmApp         = 0x8000

	htClient      = 1
	sizeMinimized = 1
	wheelDelta    = 120
	keyRepeatBit  = 0x40000000 // lParam bit 30: previous key state
)

func loWordS(v uintptr) int32 { return int32(int16(v & 0xFFFF)) }
func hiWordS(v uintptr) int32 { return int32(int16((v >> 16) & 0xFFFF)) }

// emit dispatches an event, reusing a single gui.Event stored on the
// platform state to avoid a heap allocation per message. EventFn does
// not retain the pointer beyond the call (same contract the SDL2 path
// relies on).
func (b *Backend) emit(e gui.Event) {
	b.plat.evt = e
	b.plat.w.EventFn(&b.plat.evt)
}

// logicalXY converts physical client-pixel coordinates to logical
// points using the current DPI scale.
func (b *Backend) logicalXY(px, py int32) (float32, float32) {
	s := b.dpiScale
	if s <= 0 {
		s = 1
	}
	return float32(px) / s, float32(py) / s
}

// handleMessage translates a window message to a gui.Event and
// dispatches it. Returns (result, true) when the message is fully
// handled, or (0, false) to defer to DefWindowProc.
//
//nolint:gocyclo // message dispatch switch
func (b *Backend) handleMessage(msg, wparam, lparam uintptr) (uintptr, bool) {
	w := b.plat.w
	switch msg {
	case wmMouseMove:
		x, y := b.logicalXY(loWordS(lparam), hiWordS(lparam))
		b.emit(gui.Event{
			Type:      gui.EventMouseMove,
			MouseX:    x,
			MouseY:    y,
			Modifiers: winkey.ModState() | winkey.MouseButtons(wparam),
		})
		return 0, true

	case wmLButtonDown:
		return b.mouseButton(gui.EventMouseDown, gui.MouseLeft, lparam, true)
	case wmLButtonUp:
		return b.mouseButton(gui.EventMouseUp, gui.MouseLeft, lparam, false)
	case wmRButtonDown:
		return b.mouseButton(gui.EventMouseDown, gui.MouseRight, lparam, true)
	case wmRButtonUp:
		return b.mouseButton(gui.EventMouseUp, gui.MouseRight, lparam, false)
	case wmMButtonDown:
		return b.mouseButton(gui.EventMouseDown, gui.MouseMiddle, lparam, true)
	case wmMButtonUp:
		return b.mouseButton(gui.EventMouseUp, gui.MouseMiddle, lparam, false)

	case wmMouseWheel:
		return b.mouseWheel(0, float32(hiWordS(wparam))/wheelDelta, lparam)
	case wmMouseHWheel:
		return b.mouseWheel(float32(hiWordS(wparam))/wheelDelta, 0, lparam)

	case wmKeyDown, wmSysKeyDown:
		b.emit(gui.Event{
			Type:      gui.EventKeyDown,
			KeyCode:   winkey.MapVKey(wparam),
			Modifiers: winkey.ModState(),
			KeyRepeat: lparam&keyRepeatBit != 0,
		})
		return 0, true
	case wmKeyUp, wmSysKeyUp:
		b.emit(gui.Event{
			Type:      gui.EventKeyUp,
			KeyCode:   winkey.MapVKey(wparam),
			Modifiers: winkey.ModState(),
		})
		return 0, true

	case wmChar:
		return b.charInput(wparam)

	case wmSize:
		if wparam == sizeMinimized {
			return 0, true
		}
		// Refresh physical size + DPI, then derive the logical size
		// from the authoritative client rect rather than lParam's
		// 16-bit words (which sign-extend past 32767).
		b.handleResize()
		lw, lh := b.logicalXY(b.physW, b.physH)
		b.emit(gui.Event{
			Type:         gui.EventResized,
			WindowWidth:  int(lw),
			WindowHeight: int(lh),
		})
		// Repaint live during modal drag-resize.
		w.FrameFn()
		b.renderFrame(w)
		return 0, true

	case wmPaint:
		pValidateRect.Call(b.plat.hwnd, 0)
		return 0, true
	case wmEraseBkgnd:
		return 1, true // avoid background flicker; GL clears each frame

	case wmSetFocus:
		b.emit(gui.Event{Type: gui.EventFocused})
		return 0, true
	case wmKillFocus:
		b.emit(gui.Event{Type: gui.EventUnfocused})
		return 0, true

	case wmSetCursor:
		if loWordS(lparam) == htClient {
			if b.plat.curCursor != 0 {
				pSetCursor.Call(b.plat.curCursor)
			}
			return 1, true
		}
		return 0, false

	case wmClose:
		gui.DispatchCloseRequest(w)
		return 0, true
	}
	return 0, false
}

func (b *Backend) mouseButton(t gui.EventType, btn gui.MouseButton,
	lparam uintptr, down bool) (uintptr, bool) {

	if down {
		pSetCapture.Call(b.plat.hwnd)
	} else {
		pReleaseCapture.Call()
	}
	x, y := b.logicalXY(loWordS(lparam), hiWordS(lparam))
	b.emit(gui.Event{
		Type:        t,
		MouseX:      x,
		MouseY:      y,
		MouseButton: btn,
		Modifiers:   winkey.ModState(),
	})
	return 0, true
}

func (b *Backend) mouseWheel(sx, sy float32, lparam uintptr) (uintptr, bool) {
	// Wheel coordinates are screen-relative; convert to client.
	pt := pointW{x: loWordS(lparam), y: hiWordS(lparam)}
	pScreenToClient.Call(b.plat.hwnd, uintptr(unsafe.Pointer(&pt)))
	x, y := b.logicalXY(pt.x, pt.y)
	b.emit(gui.Event{
		Type:      gui.EventMouseScroll,
		ScrollX:   sx,
		ScrollY:   sy,
		MouseX:    x,
		MouseY:    y,
		Modifiers: winkey.ModState(),
	})
	return 0, true
}

// charInput handles WM_CHAR, reassembling UTF-16 surrogate pairs and
// filtering control characters (which arrive as key events instead).
func (b *Backend) charInput(wparam uintptr) (uintptr, bool) {
	c := uint16(wparam)
	if utf16.IsSurrogate(rune(c)) {
		if c >= 0xD800 && c <= 0xDBFF {
			b.plat.highSurr = c
			return 0, true
		}
		// Low surrogate: combine with the pending high surrogate.
		if b.plat.highSurr != 0 {
			r := utf16.DecodeRune(rune(b.plat.highSurr), rune(c))
			b.plat.highSurr = 0
			b.emitChar(r)
		}
		return 0, true
	}
	b.plat.highSurr = 0
	r := rune(c)
	if r < 0x20 || r == 0x7F {
		return 0, true // control chars come through as key events
	}
	b.emitChar(r)
	return 0, true
}

func (b *Backend) emitChar(r rune) {
	if r == 0xFFFD {
		return
	}
	b.emit(gui.Event{
		Type:      gui.EventChar,
		CharCode:  uint32(r),
		IMEText:   string(r),
		Modifiers: winkey.ModState(),
	})
}
