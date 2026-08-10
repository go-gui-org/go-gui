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

	wmCaptureChanged = 0x0215

	wmApp = 0x8000

	htClient      = 1
	sizeMinimized = 1
	wheelDelta    = 120
	keyRepeatBit  = 0x40000000 // lParam bit 30: previous key state

	// Wheel-speed settings. Windows exposes lines-per-notch (vertical) and
	// chars-per-notch (horizontal) as user preferences; the defaults match
	// a fresh Windows install.
	spiGetWheelScrollLines = 0x0068
	spiGetWheelScrollChars = 0x006C
	defaultScrollLines     = 3
	defaultScrollChars     = 3

	// wheelPageScroll is the SPI_GETWHEELSCROLLLINES sentinel for "scroll
	// one screen per notch" (WHEEL_PAGESCROLL == UINT_MAX).
	wheelPageScroll = 0xFFFFFFFF
	linesPerPage    = 25
)

func loWordS(v uintptr) int32 { return int32(int16(v & 0xFFFF)) }
func hiWordS(v uintptr) int32 { return int32(int16((v >> 16) & 0xFFFF)) }

// emit dispatches an event, reusing a single gui.Event stored on the
// platform state to avoid a heap allocation per message. EventFn does
// not retain the pointer beyond the call.
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
		dx, dy := b.mouseDelta(x, y)
		b.emit(gui.Event{
			Type:      gui.EventMouseMove,
			MouseX:    x,
			MouseY:    y,
			MouseDX:   dx,
			MouseDY:   dy,
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

	case wmCaptureChanged:
		// Win32 can revoke mouse capture without ever sending the
		// matching button-up: another process calls SetCapture, a
		// system modal or UAC prompt appears, Ctrl+Alt+Del or Win+L
		// fires, a debugger breaks in. The drag is over, but nothing
		// tells the gui layer, so a MouseLock would stay locked and
		// every later move would keep driving it with no button held.
		//
		// capturing is cleared *before* our own ReleaseCapture, whose
		// WM_CAPTURECHANGED reenters this wndproc synchronously — so
		// reaching here with it still set means the loss was involuntary.
		if b.plat.capturing {
			b.plat.capturing = false
			w.MouseCancel()
		}
		return 0, true

	case wmMouseWheel:
		return b.mouseWheel(0, notchesToLines(hiWordS(wparam)), lparam)
	case wmMouseHWheel:
		return b.mouseWheel(notchesToChars(hiWordS(wparam)), 0, lparam)

	case wmKeyDown, wmSysKeyDown:
		// VK_PROCESSKEY means the input method consumed the keystroke:
		// TranslateMessage has already handed it to IMM, and the WM_IME_*
		// messages that follow carry its effect. Emitting it as well would
		// let a widget act on an Enter or an arrow the IME owns, which is
		// the suppression the X11 path gets from imeProcessKey.
		if wparam == vkProcessKey {
			return 0, true
		}
		b.emit(gui.Event{
			Type:      gui.EventKeyDown,
			KeyCode:   winkey.MapVKey(wparam),
			Modifiers: winkey.ModState(),
			KeyRepeat: lparam&keyRepeatBit != 0,
		})
		return 0, true
	case wmKeyUp, wmSysKeyUp:
		if wparam == vkProcessKey {
			return 0, true
		}
		b.emit(gui.Event{
			Type:      gui.EventKeyUp,
			KeyCode:   winkey.MapVKey(wparam),
			Modifiers: winkey.ModState(),
		})
		return 0, true

	case wmChar:
		return b.charInput(wparam)

	case wmIMEStartComposition, wmIMEComposition, wmIMEEndComposition,
		wmIMESetContext, wmIMEChar:
		return b.imeMessage(msg, wparam, lparam)

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
		b.plat.capturing = true
		pSetCapture.Call(b.plat.hwnd)
	} else if b.plat.capturing {
		// Clear first: ReleaseCapture sends WM_CAPTURECHANGED back
		// through this wndproc before it returns, and that handler
		// must read a deliberate release, not a revoked one.
		b.plat.capturing = false
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

// notchesToLines converts a WM_MOUSEWHEEL delta into gui.Event's scroll
// unit: lines of text. Windows reports wheel travel in 1/120ths of a
// notch and leaves the lines-per-notch multiplier to the app, which must
// read it from SPI_GETWHEELSCROLLLINES (Control Panel → Mouse → Wheel;
// three by default). Skipping that step is what made one notch scroll a
// single line-equivalent instead of the three the user asked for.
func notchesToLines(delta int32) float32 {
	lines := wheelScrollLines()
	if lines == wheelPageScroll {
		// The user selected "one screen at a time". No viewport height is
		// available here, so approximate a page — consumers clamp to their
		// own bounds anyway.
		lines = linesPerPage
	}
	return float32(delta) / wheelDelta * float32(lines)
}

// notchesToChars is the horizontal counterpart, using the separate
// SPI_GETWHEELSCROLLCHARS setting. It has no page-scroll sentinel.
func notchesToChars(delta int32) float32 {
	return float32(delta) / wheelDelta * float32(wheelScrollChars())
}

// wheelScrollLines reads SPI_GETWHEELSCROLLLINES, falling back to the
// Windows default when the call fails.
func wheelScrollLines() uint32 { return sysParamUint(spiGetWheelScrollLines, defaultScrollLines) }

// wheelScrollChars reads SPI_GETWHEELSCROLLCHARS, falling back to the
// Windows default when the call fails.
func wheelScrollChars() uint32 { return sysParamUint(spiGetWheelScrollChars, defaultScrollChars) }

// sysParamUint fetches a uint-valued SystemParametersInfo action. The
// settings are re-read per event rather than cached: they change while
// the app runs (Control Panel, or a WM_SETTINGCHANGE broadcast), and a
// wheel message is far too rare for the syscall to matter.
func sysParamUint(action uintptr, fallback uint32) uint32 {
	var v uint32
	r, _, _ := pSysParamsInfoW.Call(
		action, 0, uintptr(unsafe.Pointer(&v)), 0)
	if r == 0 || v == 0 {
		return fallback
	}
	return v
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
