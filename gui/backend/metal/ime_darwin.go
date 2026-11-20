//go:build darwin && arm64 && !ios

package metal

// MetalContentView: the CAMetalLayer-backed content view of a window, and
// its NSTextInputClient methods (IME). Main-thread only. See
// window_darwin.go.

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// newContentView makes the content view of w, +1, and its CAMetalLayer.
// Call it inside an autorelease pool.
func newContentView(w *macWindow, frame nsRect, device objc.ID) objc.ID {
	// The CAMetalLayer is made here and set as the backing layer. A
	// layerClass override is not reliable: AppKit can make a plain
	// CALayer first if wantsLayer is read in a particular order.
	layer := darwinffi.Class("CAMetalLayer").Send(darwinffi.Sel("layer"))
	layer.Send(darwinffi.Sel("setDevice:"), device)
	layer.Send(darwinffi.Sel("setPixelFormat:"), uint(mtlPixelFormatBGRA8Unorm))
	// No drawable readback: filters render into offscreen targets, never
	// from the drawable.
	layer.Send(darwinffi.Sel("setFramebufferOnly:"), true)
	// Off outside live resize. Transaction mode costs a main-thread block
	// per frame, which delays every queued UI update. It is on only while
	// a resize drag runs, where it stops the content shifting inside the
	// window: see windowWillStartLiveResize: and the rationale in
	// metalCtxCreate.
	layer.Send(darwinffi.Sel("setPresentsWithTransaction:"), false)

	view := objc.ID(contentViewClass).Send(sels.alloc).Send(darwinffi.Sel("initWithFrame:"), frame)
	view.Send(darwinffi.Sel("setWantsLayer:"), true)
	view.Send(darwinffi.Sel("setLayer:"), layer)
	w.layer = layer

	// AppKit posts no exit event without a tracking area. InVisibleRect
	// keeps the area the size of the view through resizes, so no
	// updateTrackingAreas override is needed. ActiveAlways reports the
	// exit in a background window too, which the gui layer accepts.
	area := darwinffi.Class("NSTrackingArea").Send(sels.alloc).Send(
		darwinffi.Sel("initWithRect:options:owner:userInfo:"),
		nsRect{}, uint(nsTrackingMouseEnteredAndExited|nsTrackingActiveAlways|nsTrackingInVisibleRect),
		view, objc.ID(0))
	view.Send(darwinffi.Sel("addTrackingArea:"), area)
	area.Send(sels.release)
	return view
}

func contentViewMethods() []objc.MethodDef {
	noop := func(objc.ID, objc.SEL, objc.ID) {}
	return []objc.MethodDef{
		// First responder, so the view receives key events.
		{Cmd: darwinffi.Sel("acceptsFirstResponder"), Fn: yes},
		// Take part in AppKit's cursor rects, so the window server can
		// manage the cursors of the window frame (resize edges, title-bar
		// buttons). With no cursor rect, cursor tracking does not work,
		// not even for the frame. Go sets the content cursor through
		// metalWindowSetCursor when a widget asks for another shape.
		{Cmd: darwinffi.Sel("resetCursorRects"), Fn: func(self objc.ID, _ objc.SEL) {
			arrow := darwinffi.Class("NSCursor").Send(darwinffi.Sel("arrowCursor"))
			self.Send(darwinffi.Sel("addCursorRect:cursor:"), msgRect(self, sels.bounds), arrow)
		}},
		// AppKit can skip resetCursorRects in setContentView:, which
		// would leave cursor tracking off. Invalidate them here.
		{Cmd: darwinffi.Sel("viewDidMoveToWindow"), Fn: func(self objc.ID, cmd objc.SEL) {
			self.SendSuper(cmd)
			self.Send(sels.window).Send(darwinffi.Sel("invalidateCursorRectsForView:"), self)
		}},
		// All key events go through the input method system, so
		// insertText: fires for printable characters. Other keys
		// (arrows and similar) go to Go as EventKeyDown.
		{Cmd: darwinffi.Sel("keyDown:"), Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
			w := windowOf(self)
			if w == nil {
				return
			}
			wasComposing := w.markedText != 0
			pollState.imeDeclinedKey = false
			events := darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObject:"), event)
			self.Send(darwinffi.Sel("interpretKeyEvents:"), events)
			w.imeSettle(wasComposing)
		}},
		// Key-up and modifier changes go to Go through metalPollEvent.
		{Cmd: darwinffi.Sel("keyUp:"), Fn: noop},
		{Cmd: darwinffi.Sel("flagsChanged:"), Fn: noop},
		// The input source calls this for a key it declined as text
		// input. Recording it is what lets keyDown: tell "the IME used
		// this" from "the IME gave it back": see imeSettle. Nothing else
		// happens: Go handles every key command, and super would beep on
		// the ones it does not know.
		{Cmd: darwinffi.Sel("doCommandBySelector:"), Fn: func(objc.ID, objc.SEL, objc.SEL) {
			pollState.imeDeclinedKey = true
		}},

		// NSTextInputClient.
		{Cmd: darwinffi.Sel("insertText:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, str objc.ID, _ nsRange) {
			if w := windowOf(self); w != nil {
				w.insertText(str)
			}
		}},
		{Cmd: darwinffi.Sel("setMarkedText:selectedRange:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, str objc.ID, sel, _ nsRange) {
			if w := windowOf(self); w != nil {
				w.setMarkedText(str, sel)
			}
		}},
		{Cmd: darwinffi.Sel("unmarkText"), Fn: func(self objc.ID, _ objc.SEL) {
			if w := windowOf(self); w != nil {
				w.unmark()
			}
		}},
		{Cmd: darwinffi.Sel("hasMarkedText"), Fn: func(self objc.ID, _ objc.SEL) bool {
			w := windowOf(self)
			return w != nil && w.markedText != 0
		}},
		{Cmd: darwinffi.Sel("markedRange"), Fn: func(self objc.ID, _ objc.SEL) nsRange {
			if w := windowOf(self); w != nil {
				return w.markedRange
			}
			return nsRange{nsNotFound, 0}
		}},
		// The input method does range arithmetic against this value and
		// drops a commit when it is NSNotFound: a converted commit has to
		// replace text at the insertion point. go-gui keeps the document
		// text on the Go side, so the range is against the composition
		// alone: marked text always starts at 0, and selRange is the
		// caret the IME last reported inside it.
		{Cmd: darwinffi.Sel("selectedRange"), Fn: func(self objc.ID, _ objc.SEL) nsRange {
			if w := windowOf(self); w != nil {
				return w.selRange
			}
			return nsRange{}
		}},
		{Cmd: darwinffi.Sel("attributedSubstringForProposedRange:actualRange:"), Fn: func(self objc.ID, _ objc.SEL, r nsRange, actual *nsRange) objc.ID {
			w := windowOf(self)
			if w == nil {
				return 0
			}
			return w.substring(r, actual)
		}},
		{Cmd: darwinffi.Sel("firstRectForCharacterRange:actualRange:"), Fn: func(self objc.ID, _ objc.SEL, _ nsRange, _ unsafe.Pointer) nsRect {
			w := windowOf(self)
			if w == nil {
				return nsRect{}
			}
			return w.imeScreenRect()
		}},
		{Cmd: darwinffi.Sel("characterIndexForPoint:"), Fn: func(objc.ID, objc.SEL, nsPoint) uint {
			return 0
		}},
		// The attributes the client takes for marked text. An empty list
		// tells the input method the client cannot draw clause segments,
		// which makes conversion worse.
		{Cmd: darwinffi.Sel("validAttributesForMarkedText"), Fn: func(objc.ID, objc.SEL) objc.ID {
			appkit := darwinffi.Open(darwinffi.PathAppKit)
			arr := darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObject:"),
				objc.ID(darwinffi.Data(appkit, "NSUnderlineStyleAttributeName")))
			arr = arr.Send(darwinffi.Sel("arrayByAddingObject:"),
				objc.ID(darwinffi.Data(appkit, "NSUnderlineColorAttributeName")))
			return arr.Send(darwinffi.Sel("arrayByAddingObject:"),
				objc.ID(darwinffi.Data(appkit, "NSMarkedClauseSegmentAttributeName")))
		}},
	}
}

// imeSettle decides who owns the key that interpretKeyEvents: just
// handled, and drops a preedit nobody asked for. It is separate from
// keyDown: so a test can drive it: a synthetic NSEvent cannot make the
// real input source produce a dead key, so the test stages the preedit
// with setMarkedText and calls this.
func (w *macWindow) imeSettle(wasComposing bool) {
	// Nothing editable has focus, so this preedit is not wanted. On the
	// US layout Option+I is a dead circumflex: it took the app's Option
	// shortcut, then composed itself into the next keystroke (issue
	// #393). Drop it and let the key-down through.
	if w.markedText != 0 && !w.imeActive {
		w.unmark()
		// setMarkedText already queued the preedit, and unmark is silent
		// on purpose (see insertText), so clear the Go composition here.
		pushTextEvent(metalEventIMEComp, "", w.id, 0, 0)
		// This drops the dead key inside the input source. Without it the
		// next keystroke still composes against the cancelled accent.
		w.view.Send(darwinffi.Sel("inputContext")).Send(darwinffi.Sel("discardMarkedText"))
		return
	}
	// The input source gave the key back, so it belongs to the widget,
	// even during a composition. Tab on a pending dead key comes back this
	// way: the accent commits through insertText: and insertTab: comes
	// back here. Taking it left focus stuck in the field.
	if pollState.imeDeclinedKey {
		return
	}
	// What the input source used belongs to it: a composition in flight
	// owns arrows and Return (the engine walks its own clauses with
	// them), and so does a key that starts a preedit over an editable
	// widget: a CJK first letter, or the first half of Option+e for é.
	if wasComposing || w.markedText != 0 {
		pollState.imeConsumedKey = true
	}
}

// nsStringOf returns the NSString of str, an NSString or an
// NSAttributedString, or 0 for any other object.
func nsStringOf(str objc.ID) objc.ID {
	switch {
	case str == 0:
		return 0
	case objc.Send[bool](str, darwinffi.Sel("isKindOfClass:"), darwinffi.Class("NSAttributedString")):
		return str.Send(darwinffi.Sel("string"))
	case objc.Send[bool](str, darwinffi.Sel("isKindOfClass:"), objs.nsString):
		return str
	}
	return 0
}

func (w *macWindow) insertText(str objc.ID) {
	// Queued, not delivered at once: a CJK commit is often followed by
	// setMarkedText in the same sendEvent:.
	pushTextEvent(metalEventChar, goStr(nsStringOf(str)), w.id, 0, 0)
	w.unmark()
}

func (w *macWindow) setMarkedText(str objc.ID, sel nsRange) {
	switch {
	case str == 0:
	case objc.Send[bool](str, darwinffi.Sel("isKindOfClass:"), darwinffi.Class("NSAttributedString")):
		w.setMarked(str.Send(darwinffi.Sel("copy")))
	case objc.Send[bool](str, darwinffi.Sel("isKindOfClass:"), objs.nsString):
		w.setMarked(darwinffi.Class("NSAttributedString").Send(sels.alloc).
			Send(darwinffi.Sel("initWithString:"), str))
	}
	// Any other object leaves the marked text as it was, as in the cgo
	// code.

	n := uint(0)
	if w.markedText != 0 {
		n = objc.Send[uint](w.markedText, darwinffi.Sel("length"))
	}
	if n == 0 {
		w.unmark()
		// Empty marked text means the IME ended the composition (cancel,
		// or a commit that insertText: handles). It goes to Go as an
		// empty composition so Go clears the preedit. unmark stays
		// silent, so a commit does not send a second one.
		pushTextEvent(metalEventIMEComp, "", w.id, 0, 0)
		return
	}
	w.markedRange = nsRange{0, n}
	// The selection inside the composition, read back through
	// selectedRange during conversion. Kept as it is (length too:
	// conversion selects a whole clause), with a guard only for a
	// location the composition cannot hold.
	w.selRange = sel
	if w.selRange.Loc == nsNotFound || w.selRange.Loc > n {
		w.selRange = nsRange{n, 0}
	}
	// A subtraction, not a sum: the range members are unsigned and wrap.
	if w.selRange.Len > n-w.selRange.Loc {
		w.selRange.Len = n - w.selRange.Loc
	}
	// The event carries the clamped range, not the raw one: NSNotFound
	// would reach the renderer as a negative caret offset.
	//
	// selRange stays in UTF-16 units (selectedRange gives it back to the
	// IME, which does its own arithmetic in that space). The event is in
	// code points. Without the change, a surrogate pair before the
	// clause moves the preedit underline one character right (#169).
	text := goStr(w.markedText.Send(darwinffi.Sel("string")))
	loc := utf16UnitsToCodePoints(text, w.selRange.Loc)
	end := utf16UnitsToCodePoints(text, w.selRange.Loc+w.selRange.Len)
	pushTextEvent(metalEventIMEComp, text, w.id, int(loc), int(end-loc))
}

// setMarked makes m, +1, the marked text, and releases the old one.
func (w *macWindow) setMarked(m objc.ID) {
	if w.markedText != 0 {
		w.markedText.Send(sels.release)
	}
	w.markedText = m
}

// unmark is unmarkText: it ends the composition and sends nothing.
func (w *macWindow) unmark() {
	w.setMarked(0)
	w.markedRange = nsRange{nsNotFound, 0}
	w.selRange = nsRange{}
}

// substring answers attributedSubstringForProposedRange:actualRange:.
// nil here makes the input method drop a converted commit: it asks for
// the text it is about to replace. go-gui keeps the document on the Go
// side, so the composition stands in for the document: marked text
// always starts at 0, the space markedRange and selectedRange report in.
func (w *macWindow) substring(r nsRange, actual *nsRange) objc.ID {
	doc := objs.nsString.Send(darwinffi.Sel("string"))
	if w.markedText != 0 {
		doc = w.markedText.Send(darwinffi.Sel("string"))
	}
	n := objc.Send[uint](doc, darwinffi.Sel("length"))
	if r.Loc == nsNotFound || r.Loc > n {
		if actual != nil {
			*actual = nsRange{nsNotFound, 0}
		}
		return 0
	}
	// Subtract rather than compare Loc+Len: the members are unsigned,
	// and IMK sends lengths near NSUIntegerMax, which would wrap the sum
	// past a compare and crash substringWithRange:.
	if r.Len > n-r.Loc {
		r.Len = n - r.Loc
	}
	if actual != nil {
		*actual = r
	}
	sub := doc.Send(darwinffi.Sel("substringWithRange:"), r)
	return darwinffi.Class("NSAttributedString").Send(sels.alloc).
		Send(darwinffi.Sel("initWithString:"), sub).Send(sels.autorelease)
}

// imeScreenRect is the IME caret rect in screen coordinates, for the
// candidate window.
func (w *macWindow) imeScreenRect() nsRect {
	win := w.view.Send(sels.window)
	if win == 0 {
		return nsRect{}
	}
	// imeCursorRect has the gui top-left origin. This view is not
	// flipped, so AppKit reads y from the bottom. Without the flip the
	// candidate window is mirrored top to bottom: a caret on the first
	// line puts it at the bottom of the window. The flip is done here, at
	// query time, so a resize after IMESetRect still uses the current
	// height.
	r := w.imeCursorRect
	viewH := msgRect(w.view, sels.bounds).H
	r.Y = float64(imeRectViewY(float32(r.Y), float32(r.H), float32(viewH)))
	r = objc.Send[nsRect](w.view, darwinffi.Sel("convertRect:toView:"), r, objc.ID(0))
	return objc.Send[nsRect](win, darwinffi.Sel("convertRectToScreen:"), r)
}

// imeRectViewY flips the y of the IME rect from the gui top-left origin
// to the bottom-left origin of the view.
func imeRectViewY(y, h, viewH float32) float32 {
	return viewH - y - h
}

// utf16UnitsToCodePoints converts a UTF-16 offset (NSRange space) in s
// into a count of code points: the "characters" that Event.IMEStart and
// IMELength promise, and that the Go renderer uses. The two are the same
// for BMP text. A surrogate pair (emoji, CJK Ext-B) counts once. A
// character counts once its start is before unit, so a boundary between
// the halves of a pair (only a confused IME reports one) rounds to the
// end of the pair, never splits it.
func utf16UnitsToCodePoints(s string, unit uint) uint {
	var units, cps uint
	for _, r := range s {
		if units >= unit {
			break
		}
		units++
		if r >= 0x10000 {
			units++
		}
		cps++
	}
	return cps
}

// metalWindowIMESetCursorRect sets the IME caret rect, in the gui
// top-left space. imeScreenRect flips it when AppKit asks.
func metalWindowIMESetCursorRect(w *macWindow, x, y, width, height float32) {
	if w == nil {
		return
	}
	w.imeCursorRect = nsRect{float64(x), float64(y), float64(width), float64(height)}
}

// metalWindowIMESetActive says whether an editable text widget has
// focus (see gui/backend/CLAUDE.md, IME activation contract).
func metalWindowIMESetActive(w *macWindow, active bool) {
	if w == nil || w.ns == 0 {
		return
	}
	w.imeActive = active
	if active {
		w.ns.Send(darwinffi.Sel("makeFirstResponder:"), w.view)
	}
}
