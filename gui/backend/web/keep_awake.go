//go:build js && wasm

package web

// keep_awake.go — keeping the display on with the Screen Wake Lock API
// (issue #1172).
//
// navigator.wakeLock exists only on a secure page (HTTPS or localhost)
// in a browser that has the API. request("screen") returns a promise:
// the browser may refuse (a power saver, a low battery, an iframe
// without the screen-wake-lock permission). A lock it granted it may
// drop later. When the page is hidden it always does; gui then ends the
// request and asks again when the page is shown, so that case needs
// nothing here. When the page is still visible (a power saver turned
// on, for example), this file asks again by itself: a dropped lock
// stays dropped, and gui still takes the request as held.

import (
	"errors"
	"fmt"
	"syscall/js"
	"time"
)

// wakeLockMinHold is how long a lock must have been held for a drop to
// be answered with a new request. A browser that drops each lock right
// after granting it would otherwise be asked again in a loop. A
// variable so a test can change it.
var wakeLockMinHold = time.Second

// pageWakeLock holds the page's wake lock. JS runs on one thread, so no
// lock.
type pageWakeLock struct {
	nav js.Value // navigator
	doc js.Value // document, for hidden
	// want is the last thing gui asked for.
	want bool
	// pending is true while the browser has not answered a request.
	pending bool
	// sentinel is the held WakeLockSentinel, undefined while none.
	sentinel js.Value
	// grantedAt is when sentinel was granted.
	grantedAt time.Time
	// The promise handlers and the sentinel's release listener.
	granted, refused, dropped js.Func
}

// newPageWakeLock returns the page's wake lock control. Its callbacks
// join callbacks, which live as long as the page.
func newPageWakeLock(nav, doc js.Value, callbacks *[]js.Func) *pageWakeLock {
	l := &pageWakeLock{nav: nav, doc: doc, sentinel: js.Undefined()}
	l.granted = js.FuncOf(func(_ js.Value, args []js.Value) any {
		l.pending = false
		if len(args) == 0 || !args[0].Truthy() {
			return nil
		}
		s := args[0]
		if !l.want {
			// Ended before the browser answered.
			s.Call("release")
			return nil
		}
		l.sentinel = s
		l.grantedAt = time.Now()
		s.Call("addEventListener", "release", l.dropped)
		return nil
	})
	l.refused = js.FuncOf(func(_ js.Value, args []js.Value) any {
		l.pending = false
		if len(args) > 0 {
			warn("the browser refused the wake lock:", args[0])
		} else {
			warn("the browser refused the wake lock")
		}
		return nil
	})
	l.dropped = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		// Our own release in set has already cleared sentinel.
		if !l.sentinel.Truthy() || !l.sentinel.Get("released").Truthy() {
			return nil
		}
		held := time.Since(l.grantedAt)
		l.sentinel = js.Undefined()
		if !l.want || l.doc.Get("hidden").Truthy() {
			return nil // gui asks again when the page is shown
		}
		if held < wakeLockMinHold {
			warn("the browser dropped the wake lock right after granting " +
				"it; not asking again until the page is hidden and shown")
			return nil
		}
		l.request()
		return nil
	})
	*callbacks = append(*callbacks, l.granted, l.refused, l.dropped)
	return l
}

// warn writes to the console, where a web developer looks. Not
// log.Printf: Go's write to stderr waits on the JS event loop, which a
// nested callback holds.
func warn(args ...any) {
	js.Global().Get("console").Call("warn",
		append([]any{"go-gui: keep awake:"}, args...)...)
}

// set asks for the lock or lets it go. A refusal comes later, from the
// promise, and goes to the browser console: set cannot return it.
func (l *pageWakeLock) set(on bool) error {
	l.want = on
	if !on {
		if l.sentinel.Truthy() {
			s := l.sentinel
			l.sentinel = js.Undefined()
			s.Call("release")
		}
		return nil
	}
	if l.pending || l.sentinel.Truthy() {
		return nil
	}
	if !l.available() {
		return fmt.Errorf("this page has no navigator.wakeLock: it needs "+
			"HTTPS or localhost, and a browser with the Screen Wake Lock "+
			"API: %w", errors.ErrUnsupported)
	}
	l.request()
	return nil
}

// available reports whether the page has the Screen Wake Lock API.
func (l *pageWakeLock) available() bool {
	wl := l.nav.Get("wakeLock")
	return wl.Truthy() && wl.Get("request").Truthy()
}

// request asks the browser for a screen lock.
func (l *pageWakeLock) request() {
	l.pending = true
	l.nav.Get("wakeLock").Call("request", "screen").
		Call("then", l.granted, l.refused)
}

// SetKeepAwake asks the browser to keep the screen on, or lets it go.
func (n *nativePlatform) SetKeepAwake(on bool, _, _ string) error {
	if n.wakeLock == nil {
		return fmt.Errorf("no wake lock control: %w", errors.ErrUnsupported)
	}
	return n.wakeLock.set(on)
}
