//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// fakeFullscreenDoc is a document with the Fullscreen API: it counts
// the calls and never changes fullscreenElement itself. Without
// fullscreen it has no requestFullscreen, as in iPhone Safari.
type fakeFullscreenDoc struct {
	doc, nav        js.Value
	requests, exits int
	funcs           []js.Func
	activation      js.Value // navigator.userActivation
}

func newFakeFullscreenDoc(t *testing.T, withoutFullscreen bool) *fakeFullscreenDoc {
	t.Helper()
	obj := func() js.Value { return js.Global().Get("Object").New() }
	d := &fakeFullscreenDoc{doc: obj(), nav: obj(), activation: obj()}
	fn := func(count *int) js.Func {
		f := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			*count++
			return js.Global().Get("Promise").Call("resolve")
		})
		d.funcs = append(d.funcs, f)
		return f
	}
	el := obj()
	if !withoutFullscreen {
		el.Set("requestFullscreen", fn(&d.requests))
	}
	d.doc.Set("documentElement", el)
	d.doc.Set("fullscreenElement", js.Null())
	d.doc.Set("exitFullscreen", fn(&d.exits))
	d.activation.Set("isActive", false)
	d.nav.Set("userActivation", d.activation)
	t.Cleanup(func() {
		for _, f := range d.funcs {
			f.Release()
		}
	})
	return d
}

func (d *fakeFullscreenDoc) control(t *testing.T) *pageFullscreen {
	t.Helper()
	var callbacks []js.Func
	f := newPageFullscreen(d.doc, d.nav, &callbacks)
	t.Cleanup(func() {
		for _, c := range callbacks {
			c.Release()
		}
	})
	return f
}

// tap dispatches one touch event, running during between the brackets
// as gui's handlers would.
func tap(f *pageFullscreen, typ gui.EventType, during func()) {
	f.beforeTouch(typ)
	if during != nil {
		during()
	}
	f.afterTouch(typ)
}

func TestPageFullscreenRequestsNowOutsideTouch(t *testing.T) {
	d := newFakeFullscreenDoc(t, false)
	f := d.control(t)
	f.set(true)
	if d.requests != 1 {
		t.Fatalf("requests = %d, want 1", d.requests)
	}
}

func TestPageFullscreenHoldsTouchStartForTouchEnd(t *testing.T) {
	d := newFakeFullscreenDoc(t, false)
	f := d.control(t)
	tap(f, gui.EventTouchesBegan, func() { f.set(true) })
	if d.requests != 0 {
		t.Fatalf("requests after touchstart = %d, want 0", d.requests)
	}
	tap(f, gui.EventTouchesMoved, nil)
	if d.requests != 0 {
		t.Fatalf("requests after touchmove = %d, want 0", d.requests)
	}
	tap(f, gui.EventTouchesEnded, nil)
	if d.requests != 1 {
		t.Fatalf("requests after touchend = %d, want 1", d.requests)
	}
	tap(f, gui.EventTouchesBegan, nil)
	tap(f, gui.EventTouchesEnded, nil)
	if d.requests != 1 {
		t.Fatalf("a later tap requested again: %d", d.requests)
	}
}

func TestPageFullscreenTouchStartWithActivationRequestsNow(t *testing.T) {
	d := newFakeFullscreenDoc(t, false)
	d.activation.Set("isActive", true)
	f := d.control(t)
	tap(f, gui.EventTouchesBegan, func() { f.set(true) })
	if d.requests != 1 {
		t.Fatalf("requests = %d, want 1", d.requests)
	}
	tap(f, gui.EventTouchesEnded, nil)
	if d.requests != 1 {
		t.Fatalf("touchend requested again: %d", d.requests)
	}
}

func TestPageFullscreenWithoutUserActivationAPIHolds(t *testing.T) {
	d := newFakeFullscreenDoc(t, false)
	d.nav.Set("userActivation", js.Undefined())
	f := d.control(t)
	tap(f, gui.EventTouchesBegan, func() { f.set(true) })
	tap(f, gui.EventTouchesEnded, nil)
	if d.requests != 1 {
		t.Fatalf("requests = %d, want 1 at touchend", d.requests)
	}
}

func TestPageFullscreenCancelDropsHeld(t *testing.T) {
	d := newFakeFullscreenDoc(t, false)
	f := d.control(t)
	tap(f, gui.EventTouchesBegan, func() { f.set(true) })
	tap(f, gui.EventTouchesCancelled, nil)
	tap(f, gui.EventTouchesBegan, nil)
	tap(f, gui.EventTouchesEnded, nil)
	if d.requests != 0 {
		t.Fatalf("requests = %d, want 0 after a cancelled touch", d.requests)
	}
}

func TestPageFullscreenLeaveDropsHeld(t *testing.T) {
	d := newFakeFullscreenDoc(t, false)
	f := d.control(t)
	tap(f, gui.EventTouchesBegan, func() {
		f.set(true)
		f.set(false)
	})
	tap(f, gui.EventTouchesEnded, nil)
	if d.requests != 0 || d.exits != 0 {
		t.Fatalf("requests, exits = %d, %d, want 0, 0", d.requests, d.exits)
	}
}

func TestPageFullscreenTouchEndRequestsOnce(t *testing.T) {
	// Inside a scrollable gui fires OnClick at the touchend, where the
	// request goes out at once; a request held from the touchstart
	// must not go out a second time.
	d := newFakeFullscreenDoc(t, false)
	f := d.control(t)
	tap(f, gui.EventTouchesBegan, func() { f.set(true) })
	tap(f, gui.EventTouchesEnded, func() { f.set(true) })
	if d.requests != 1 {
		t.Fatalf("requests = %d, want 1", d.requests)
	}
}

func TestPageFullscreenLeave(t *testing.T) {
	d := newFakeFullscreenDoc(t, false)
	f := d.control(t)
	f.set(false)
	if d.exits != 0 {
		t.Fatalf("exits while not fullscreen = %d, want 0", d.exits)
	}
	d.doc.Set("fullscreenElement", d.doc.Get("documentElement"))
	if !f.active() {
		t.Fatal("active() = false with a fullscreen element")
	}
	f.set(true)
	if d.requests != 0 {
		t.Fatalf("requests while fullscreen = %d, want 0", d.requests)
	}
	f.set(false)
	if d.exits != 1 {
		t.Fatalf("exits = %d, want 1", d.exits)
	}
}

func TestPageFullscreenUnsupportedDoesNothing(t *testing.T) {
	d := newFakeFullscreenDoc(t, true)
	f := d.control(t)
	tap(f, gui.EventTouchesBegan, func() { f.set(true) })
	tap(f, gui.EventTouchesEnded, nil)
	f.set(true)
	if d.requests != 0 || f.held {
		t.Fatalf("requests = %d, held = %v, want 0, false",
			d.requests, f.held)
	}
}

func TestIgnoreRejectionSkipsNonPromise(t *testing.T) {
	ignore := js.FuncOf(func(_ js.Value, _ []js.Value) any { return nil })
	defer ignore.Release()
	// An old prefixed API returns undefined; Call("catch") on it
	// would panic.
	ignoreRejection(js.Undefined(), ignore)
	ignoreRejection(js.Global().Get("Object").New(), ignore)
}
