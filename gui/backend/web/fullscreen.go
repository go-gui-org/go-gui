//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// Fullscreen on the web (issue #1171) puts the whole page in
// fullscreen, document.documentElement, not the canvas: the hidden IME
// input and the accessibility tree sit beside the canvas and must stay
// on screen with it.
//
// A browser grants fullscreen only right after the user acts: a click,
// a key press, the end of a tap. The start of a tap does not count, yet
// gui fires an OnClick outside a scrollable at touch-down, so a
// SetFullscreen there is refused. Such a request is held and made at
// the touchend that follows, which does count (measured on Chrome for
// Android, iPhone Safari and iPad Safari). Only a touchstart holds a
// request: one held from a timer would fire at some later, unrelated
// tap.
//
// iPhone Safari has no element fullscreen. SetFullscreen does nothing
// there, and IsFullscreen stays false.

// pageFullscreen enters and leaves fullscreen for the page. JS runs on
// one thread, so no lock.
type pageFullscreen struct {
	doc js.Value
	nav js.Value // navigator, for userActivation
	// inTouchStart is true while a touchstart is dispatched.
	inTouchStart bool
	// held is a request made during a touchstart, waiting for the
	// touchend.
	held bool
	// ignore swallows the rejection of a refused request, which the
	// console would otherwise show as an uncaught error.
	ignore js.Func
}

// newPageFullscreen returns the page's fullscreen control. Its callback
// joins callbacks, which live as long as the page.
func newPageFullscreen(doc, nav js.Value, callbacks *[]js.Func) *pageFullscreen {
	f := &pageFullscreen{doc: doc, nav: nav}
	f.ignore = js.FuncOf(func(_ js.Value, _ []js.Value) any { return nil })
	*callbacks = append(*callbacks, f.ignore)
	return f
}

// set enters or leaves fullscreen. A new call replaces a held request.
func (f *pageFullscreen) set(on bool) {
	f.held = false
	if on == f.active() {
		return
	}
	if !on {
		ignoreRejection(f.doc.Call("exitFullscreen"), f.ignore)
		return
	}
	if !f.doc.Get("documentElement").Get("requestFullscreen").Truthy() {
		return // iPhone Safari
	}
	if f.inTouchStart && !f.userActive() {
		f.held = true
		return
	}
	f.request()
}

// active reports whether the page, or an element in it, is fullscreen.
func (f *pageFullscreen) active() bool {
	return f.doc.Get("fullscreenElement").Truthy()
}

// beforeTouch and afterTouch bracket the dispatch of one touch event:
// a request made during a touchstart waits, and the touchend after it
// makes the request. A cancelled touch drops it.
func (f *pageFullscreen) beforeTouch(typ gui.EventType) {
	f.inTouchStart = typ == gui.EventTouchesBegan
	if typ == gui.EventTouchesCancelled {
		f.held = false
	}
}

func (f *pageFullscreen) afterTouch(typ gui.EventType) {
	f.inTouchStart = false
	if typ == gui.EventTouchesEnded && f.held {
		f.held = false
		f.request()
	}
}

func (f *pageFullscreen) request() {
	ignoreRejection(
		f.doc.Get("documentElement").Call("requestFullscreen"), f.ignore)
}

// userActive reports whether the page may ask for fullscreen now. A
// browser without navigator.userActivation reads as no, so a request
// during a touchstart waits for the touchend there too.
func (f *pageFullscreen) userActive() bool {
	if f.nav.Type() != js.TypeObject {
		return false
	}
	ua := f.nav.Get("userActivation")
	return ua.Type() == js.TypeObject && jsBool(ua.Get("isActive"))
}

// ignoreRejection attaches ignore to p's rejection, when p is a
// promise. The browser refuses a request it sees no user action for;
// IsFullscreen staying false already says so.
func ignoreRejection(p js.Value, ignore js.Func) {
	if p.Type() == js.TypeObject && p.Get("catch").Type() == js.TypeFunction {
		p.Call("catch", ignore)
	}
}
