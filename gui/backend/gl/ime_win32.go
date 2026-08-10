//go:build windows && !js

package gl

import (
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

// --- IMM32 bindings ---
//
// Loaded lazily from System32 like the kernel32/user32 procs in
// platform_win32.go, so the IME path stays cgo-free and matches the rest
// of the backend.
var (
	imm32 = windows.NewLazySystemDLL("imm32.dll")

	pImmGetContext            = imm32.NewProc("ImmGetContext")
	pImmReleaseContext        = imm32.NewProc("ImmReleaseContext")
	pImmGetCompositionStringW = imm32.NewProc("ImmGetCompositionStringW")
	pImmSetCompositionWindow  = imm32.NewProc("ImmSetCompositionWindow")
	pImmSetCandidateWindow    = imm32.NewProc("ImmSetCandidateWindow")
	pImmAssociateContextEx    = imm32.NewProc("ImmAssociateContextEx")
)

// Win32 IME messages and IMM32 flags.
const (
	wmIMEStartComposition = 0x010D
	wmIMEEndComposition   = 0x010E
	wmIMEComposition      = 0x010F
	wmIMESetContext       = 0x0281
	wmIMEChar             = 0x0286

	// vkProcessKey is the virtual-key code WM_KEYDOWN carries once the
	// input method has claimed a keystroke.
	vkProcessKey = 0xE5

	gcsCompStr   = 0x0008
	gcsCompAttr  = 0x0010
	gcsCursorPos = 0x0080
	gcsResultStr = 0x0800

	// iscShowUICompositionWindow asks DefWindowProc to draw the IME's own
	// floating composition window. Cleared before forwarding, because the
	// preedit is drawn inline instead.
	iscShowUICompositionWindow = 0x80000000

	// attrTargetConverted marks the clause the IME has selected for
	// conversion. Unlike the IBus attribute conventions on X11 this is
	// specified, so no guessing at engine behaviour is needed.
	attrTargetConverted = 1

	iaceDefault = 0x0010

	cfsPoint        = 0x0002
	cfsCandidatePos = 0x0040
	cfsExclude      = 0x0080

	// maxIMECompChars bounds a composition-string read. The length comes
	// from the input method, which is out-of-process; cap the allocation
	// the way maxClipboardChars caps a clipboard read. A preedit is a
	// phrase, so 4096 UTF-16 units is far past any real composition.
	maxIMECompChars = 4096
)

// compositionForm mirrors COMPOSITIONFORM.
type compositionForm struct {
	dwStyle      uint32
	ptCurrentPos pointW
	rcArea       rectW
}

// candidateForm mirrors CANDIDATEFORM.
type candidateForm struct {
	dwIndex      uint32
	dwStyle      uint32
	ptCurrentPos pointW
	rcArea       rectW
}

// --- message handling ---

// imeMessage handles the WM_IME_* family. Composition messages are never
// forwarded to DefWindowProc: doing so both draws the IME's own floating
// composition window and synthesizes a WM_CHAR for the commit, which
// charInput would then insert on top of the commit emitted here.
//
// charInput is deliberately left unguarded. Owning every message that
// DefWindowProc would translate is what keeps WM_CHAR out of a
// composition; a "currently composing" flag on top of that would add a
// second way for ordinary typing to break — one stuck flag and every
// keystroke is swallowed — in exchange for nothing this path does not
// already cover.
func (b *Backend) imeMessage(msg, wparam, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmIMEStartComposition:
		// Handled without DefWindowProc purely to suppress the IME's own
		// floating composition window; the preedit is drawn inline.
		return 0, true

	case wmIMEComposition:
		b.imeComposition(lparam)
		return 0, true

	case wmIMEEndComposition:
		b.emit(gui.Event{Type: gui.EventIMEComposition})
		return 0, true

	case wmIMESetContext:
		// The preedit is rendered inline, so suppress the IME's UI before
		// letting DefWindowProc set up the context. handleMessage cannot
		// rewrite lParam on the way out, so the forward happens here.
		r, _, _ := pDefWindowProcW.Call(
			b.plat.hwnd, msg, wparam, lparam&^iscShowUICompositionWindow)
		return r, true

	case wmIMEChar:
		// Swallowed: the commit is emitted from GCS_RESULTSTR. Letting
		// DefWindowProc translate this into WM_CHAR is what inserts every
		// committed character twice.
		return 0, true
	}
	return 0, false
}

// imeComposition turns one WM_IME_COMPOSITION into gui events. The
// commit is dispatched before the preedit update so text lands in the
// order the user produced it.
func (b *Backend) imeComposition(lparam uintptr) {
	himc, _, _ := pImmGetContext.Call(b.plat.hwnd)
	if himc == 0 {
		return
	}
	defer pImmReleaseContext.Call(b.plat.hwnd, himc)

	committed := false
	if lparam&gcsResultStr != 0 {
		var ok bool
		b.plat.imeBufW, ok = imeReadString(
			himc, gcsResultStr, b.plat.imeBufW)
		if ok {
			b.emitCommit(b.plat.imeBufW)
			committed = true
		}
	}

	if lparam&gcsCompStr != 0 {
		var ok bool
		b.plat.imeBufW, ok = imeReadString(himc, gcsCompStr, b.plat.imeBufW)
		if ok {
			b.plat.imeAttr = imeReadAttr(himc, b.plat.imeAttr)
			b.emit(imeCompositionEvent(
				b.plat.imeBufW, b.plat.imeAttr, imeCursorPos(himc)))
			return
		}
		// The preedit went empty — backspacing off the last character
		// asks for a composition string of length zero. Fall through to
		// the clear below.
	}

	// Either a commit with no surviving preedit, or a preedit erased down
	// to nothing. Clear it explicitly rather than waiting for
	// WM_IME_ENDCOMPOSITION, which some input methods defer.
	if committed || lparam&gcsCompStr != 0 {
		b.emit(gui.Event{Type: gui.EventIMEComposition})
	}
}

// emitCommit dispatches committed text as a single EventChar. CharCode
// carries the first rune and IMEText the whole string, which is what
// Input inserts — a multi-rune CJK commit is one insert and one undo
// step. Committed text is plain input, so no modifiers apply.
func (b *Backend) emitCommit(u []uint16) {
	s := imeDecode(u)
	if s == "" {
		return
	}
	first, _ := utf8.DecodeRuneInString(s)
	b.emit(gui.Event{
		Type:     gui.EventChar,
		CharCode: uint32(first),
		IMEText:  s,
	})
}

// --- IMM32 reads ---

// imeReadString reads one composition-string component into dst,
// reusing its capacity so a keystroke costs no allocation. Reports
// false when the component is absent, empty, or unreadable.
func imeReadString(himc, index uintptr, dst []uint16) ([]uint16, bool) {
	n, ok := imeStringLen(himc, index)
	if !ok {
		return dst[:0], false
	}
	if cap(dst) < n {
		dst = make([]uint16, n)
	}
	dst = dst[:n]
	r, _, _ := pImmGetCompositionStringW.Call(
		himc, index, uintptr(unsafe.Pointer(&dst[0])), uintptr(n*2))
	if int32(r) <= 0 {
		return dst[:0], false
	}
	// Trust the second length over the first: the composition can change
	// between the two calls.
	if got := int(int32(r)) / 2; got < n {
		dst = dst[:got]
	}
	return dst, len(dst) > 0
}

// imeStringLen queries a component's length in UTF-16 units, clamped to
// maxIMECompChars.
func imeStringLen(himc, index uintptr) (int, bool) {
	r, _, _ := pImmGetCompositionStringW.Call(himc, index, 0, 0)
	// The result is a signed byte count; negative values are IMM error
	// codes (IMM_ERROR_NODATA, IMM_ERROR_GENERAL).
	nBytes := int32(r)
	if nBytes <= 0 {
		return 0, false
	}
	n := min(int(nBytes)/2, maxIMECompChars)
	return n, n > 0
}

// imeReadAttr reads GCS_COMPATTR, one byte per UTF-16 unit of the
// preedit. A missing or unreadable attribute list only costs the clause
// highlight, so failure yields an empty slice rather than dropping the
// composition.
func imeReadAttr(himc uintptr, dst []byte) []byte {
	r, _, _ := pImmGetCompositionStringW.Call(himc, gcsCompAttr, 0, 0)
	// One byte per UTF-16 unit, so the same cap applies as to the
	// composition string; negative values are IMM error codes.
	n := min(int(int32(r)), maxIMECompChars)
	if n <= 0 {
		return dst[:0]
	}
	if cap(dst) < n {
		dst = make([]byte, n)
	}
	dst = dst[:n]
	r, _, _ = pImmGetCompositionStringW.Call(
		himc, gcsCompAttr, uintptr(unsafe.Pointer(&dst[0])), uintptr(n))
	if got := int32(r); got <= 0 {
		return dst[:0]
	} else if int(got) < len(dst) {
		dst = dst[:got]
	}
	return dst
}

// imeCursorPos reads GCS_CURSORPOS, which IMM returns as the value
// rather than through the buffer. The offset is in UTF-16 units.
func imeCursorPos(himc uintptr) int {
	r, _, _ := pImmGetCompositionStringW.Call(himc, gcsCursorPos, 0, 0)
	if n := int32(r); n > 0 {
		return int(n)
	}
	return 0
}

// --- pure helpers ---

// imeDecode converts a UTF-16 composition buffer to a string, dropping
// U+FFFD so a decoding failure upstream never reaches a widget.
func imeDecode(u []uint16) string {
	if len(u) == 0 {
		return ""
	}
	runes := utf16.Decode(u)
	out := runes[:0]
	for _, r := range runes {
		if r != 0xFFFD {
			out = append(out, r)
		}
	}
	return string(out)
}

// imeCompositionEvent builds the EventIMEComposition for a preedit.
//
// IMM reports both the cursor and the clause bounds in UTF-16 units,
// while Event.IMEStart and Event.IMELength are in characters, so every
// offset is converted. Skipping that step is the bug PR #210 fixed on
// macOS.
func imeCompositionEvent(comp []uint16, attr []byte, cursor int) gui.Event {
	e := gui.Event{
		Type:    gui.EventIMEComposition,
		IMEText: imeDecode(comp),
	}
	if start, end, ok := imeClauseRange(attr); ok {
		rs := imeRuneIndex(comp, start)
		e.IMEStart = rs
		e.IMELength = imeRuneIndex(comp, end) - rs
		return e
	}
	e.IMEStart = imeRuneIndex(comp, cursor)
	return e
}

// imeClauseRange returns the half-open UTF-16 range of the clause the
// IME has selected for conversion: the first run of
// ATTR_TARGET_CONVERTED in a GCS_COMPATTR buffer. Reports false when no
// clause is marked, which is the state before the user presses convert.
func imeClauseRange(attr []byte) (start, end int, ok bool) {
	start = -1
	for i, a := range attr {
		switch {
		case a == attrTargetConverted && start < 0:
			start = i
		case a != attrTargetConverted && start >= 0:
			return start, i, true
		}
	}
	if start >= 0 {
		return start, len(attr), true
	}
	return 0, 0, false
}

// imeRuneIndex converts a UTF-16 code-unit offset into u to a rune
// offset, clamping to the buffer. Low surrogates do not advance the
// count, so a non-BMP character costs one rune rather than two.
func imeRuneIndex(u []uint16, n int) int32 {
	if n > len(u) {
		n = len(u)
	}
	count := int32(0)
	for i := 0; i < n; i++ {
		if u[i] >= 0xDC00 && u[i] <= 0xDFFF {
			continue // low surrogate: same rune as the high one
		}
		count++
	}
	return count
}

// imeCaretBound caps a caret coordinate. Win32 window coordinates
// historically live in int16 range, and no real caret sits outside it;
// the cap exists so a NaN or infinity that became a garbage int32 in
// Window.IMESetRect's float conversion cannot propagate into IMM.
const imeCaretBound = 32767

// imePixel converts a scaled logical coordinate to a bounded pixel.
// NaN fails both comparisons and lands on zero.
func imePixel(v float32) int32 {
	switch {
	case v > imeCaretBound:
		return imeCaretBound
	case v < -imeCaretBound:
		return -imeCaretBound
	case v >= -imeCaretBound && v <= imeCaretBound:
		return int32(v)
	default:
		return 0 // NaN
	}
}

// --- NativePlatform ---

// IMEStart re-associates the window's default IME context. The context
// is detached at window creation, so composition only becomes possible
// once a text widget takes focus.
func (n *nativePlatform) IMEStart() {
	imeAssociate(n.b.plat.hwnd, iaceDefault)
}

// IMEStop detaches the IME context. No composition event is emitted
// here: the gui layer already clears its own state on a focus change
// (Window.setFocusID), and re-entering EventFn from inside an event
// handler would clobber the event being dispatched.
func (n *nativePlatform) IMEStop() {
	imeAssociate(n.b.plat.hwnd, 0)
	n.b.plat.imeHaveRect = false
}

// imeAssociate attaches (iaceDefault) or detaches (0) the window's IME
// context.
func imeAssociate(hwnd, flags uintptr) {
	if hwnd == 0 {
		return
	}
	pImmAssociateContextEx.Call(hwnd, 0, flags)
}

// IMESetRect anchors the candidate window to the caret. The gui layer
// works in logical points relative to the client area; IMM wants
// physical client pixels.
//
// renderIMEPreeditUnderline calls this every frame while composing, so
// an unchanged rect skips the three syscalls.
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	b := n.b
	if b.plat.hwnd == 0 {
		return
	}
	s := b.dpiScale
	if s <= 0 {
		s = 1
	}
	// Scaled in float32 and clamped on the way back: x+w in int32 can
	// overflow, and the caret rect is derived from glyph layout, which a
	// degenerate style (zero font size, collapsed container) can drive to
	// a nonsensical magnitude. IMM must not be handed either.
	rc := rectW{
		left:   imePixel(float32(x) * s),
		top:    imePixel(float32(y) * s),
		right:  imePixel((float32(x) + float32(w)) * s),
		bottom: imePixel((float32(y) + float32(h)) * s),
	}
	// A negative width or height would invert the rect, and CFS_EXCLUDE
	// on an inverted rcArea has no defined meaning.
	if rc.right < rc.left {
		rc.right = rc.left
	}
	if rc.bottom < rc.top {
		rc.bottom = rc.top
	}
	if b.plat.imeHaveRect && b.plat.imeRect == rc {
		return
	}

	himc, _, _ := pImmGetContext.Call(b.plat.hwnd)
	if himc == 0 {
		return
	}
	defer pImmReleaseContext.Call(b.plat.hwnd, himc)

	// Cached only once a context exists: recording a rect that was never
	// applied would short-circuit the next call and leave the composition
	// window stranded until the caret moves.
	b.plat.imeRect, b.plat.imeHaveRect = rc, true

	cf := compositionForm{
		dwStyle:      cfsPoint,
		ptCurrentPos: pointW{x: rc.left, y: rc.top},
		rcArea:       rc,
	}
	pImmSetCompositionWindow.Call(himc, uintptr(unsafe.Pointer(&cf)))

	// CFS_EXCLUDE keeps the candidate list off the text being composed
	// instead of merely placing its corner at the caret.
	can := candidateForm{
		dwStyle:      cfsCandidatePos | cfsExclude,
		ptCurrentPos: pointW{x: rc.left, y: rc.bottom},
		rcArea:       rc,
	}
	pImmSetCandidateWindow.Call(himc, uintptr(unsafe.Pointer(&can)))
}
