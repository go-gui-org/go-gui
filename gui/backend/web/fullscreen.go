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
// The browser enters and leaves asynchronously, and fullscreenElement
// changes only when it is done. A call that arrives meanwhile records
// what the app wants, and the transition, once settled, goes on toward
// it: a leave asked for while the entry is pending exits right after.
// An entry asked for while an exit is pending is requested then, which
// the browser may refuse for lack of a recent user action.
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
	// want is the app's last request.
	want bool
	// entering and exiting are true while the browser's promise for a
	// request or an exit is pending.
	entering, exiting bool
	// The promise handlers. A refused request lands in enterFailed
	// instead of showing in the console as an uncaught error.
	entered, enterFailed, exitSettled js.Func
}

// newPageFullscreen returns the page's fullscreen control. Its callbacks
// join callbacks, which live as long as the page.
func newPageFullscreen(doc, nav js.Value, callbacks *[]js.Func) *pageFullscreen {
	f := &pageFullscreen{doc: doc, nav: nav}
	f.entered = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		f.entering = false
		if !f.want && f.active() {
			f.exit()
		}
		return nil
	})
	f.enterFailed = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		f.entering = false
		return nil
	})
	f.exitSettled = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		f.exiting = false
		if f.want && !f.active() {
			f.request()
		}
		return nil
	})
	*callbacks = append(*callbacks, f.entered, f.enterFailed, f.exitSettled)
	return f
}

// set enters or leaves fullscreen. A new call replaces a held request,
// and one made while the browser is still entering or leaving waits
// for that to settle.
func (f *pageFullscreen) set(on bool) {
	f.want = on
	f.held = false
	if f.entering || f.exiting || on == f.active() {
		return
	}
	if !on {
		f.exit()
		return
	}
	if !f.supported() {
		return // iPhone Safari
	}
	if f.inTouchStart && !f.userActive() {
		f.held = true
		return
	}
	f.request()
}

func (f *pageFullscreen) supported() bool {
	return f.doc.Get("documentElement").Get("requestFullscreen").Truthy()
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
	if !f.supported() {
		return
	}
	p := f.doc.Get("documentElement").Call("requestFullscreen")
	if isPromise(p) {
		f.entering = true
		p.Call("then", f.entered, f.enterFailed)
	}
}

func (f *pageFullscreen) exit() {
	p := f.doc.Call("exitFullscreen")
	if isPromise(p) {
		f.exiting = true
		p.Call("then", f.exitSettled, f.exitSettled)
	}
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

// isPromise reports whether p can be waited on. An old prefixed API
// returns undefined.
func isPromise(p js.Value) bool {
	return p.Type() == js.TypeObject && p.Get("then").Type() == js.TypeFunction
}
