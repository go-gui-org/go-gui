//go:build js && wasm

package web

import (
	"errors"
	"syscall/js"
	"testing"
	"time"
)

// fakeWakeLock is a navigator with the Screen Wake Lock API. Each
// request returns a promise that settles only when the test calls grant
// or refuse.
type fakeWakeLock struct {
	nav, doc js.Value
	requests []string // the type of each request
	// then holds the handlers of the pending promise.
	then []js.Value
	// sentinels are the locks granted so far, with their release counts.
	sentinels []js.Value
	releases  []int
	// listener is the "release" listener of the last sentinel.
	listener js.Value
	funcs    []js.Func
	l        *pageWakeLock
}

func newFakeWakeLock(t *testing.T, withAPI bool) *fakeWakeLock {
	t.Helper()
	obj := func() js.Value { return js.Global().Get("Object").New() }
	f := &fakeWakeLock{nav: obj(), doc: obj()}
	f.doc.Set("hidden", false)
	fn := func(g func(_ js.Value, args []js.Value) any) js.Func {
		jf := js.FuncOf(g)
		f.funcs = append(f.funcs, jf)
		return jf
	}
	if withAPI {
		wl := obj()
		wl.Set("request", fn(func(_ js.Value, args []js.Value) any {
			f.requests = append(f.requests, args[0].String())
			p := obj()
			p.Set("then", fn(func(_ js.Value, h []js.Value) any {
				f.then = h
				return nil
			}))
			return p
		}))
		f.nav.Set("wakeLock", wl)
	}
	var callbacks []js.Func
	f.l = newPageWakeLock(f.nav, f.doc, &callbacks)
	t.Cleanup(func() {
		for _, c := range append(callbacks, f.funcs...) {
			c.Release()
		}
	})
	return f
}

// grant settles the pending request with a new sentinel.
func (f *fakeWakeLock) grant() {
	i := len(f.sentinels)
	s := js.Global().Get("Object").New()
	s.Set("released", false)
	f.releases = append(f.releases, 0)
	jf := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		f.releases[i]++
		s.Set("released", true)
		return nil
	})
	f.funcs = append(f.funcs, jf)
	s.Set("release", jf)
	add := js.FuncOf(func(_ js.Value, args []js.Value) any {
		f.listener = args[1]
		return nil
	})
	f.funcs = append(f.funcs, add)
	s.Set("addEventListener", add)
	f.sentinels = append(f.sentinels, s)
	f.then[0].Invoke(s)
}

// refuse settles the pending request with an error.
func (f *fakeWakeLock) refuse() {
	f.then[1].Invoke(js.Global().Get("Error").New("NotAllowedError"))
}

// On asks for a screen lock, and off releases the one granted.
func TestWakeLockRequestAndRelease(t *testing.T) {
	f := newFakeWakeLock(t, true)
	if err := f.l.set(true); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 || f.requests[0] != "screen" {
		t.Fatalf("requests = %v, want [screen]", f.requests)
	}
	f.grant()
	if err := f.l.set(false); err != nil {
		t.Fatal(err)
	}
	if f.releases[0] != 1 {
		t.Errorf("lock released %d times, want 1", f.releases[0])
	}
}

// An on while the browser has not answered makes no second request.
func TestWakeLockOnWhilePending(t *testing.T) {
	f := newFakeWakeLock(t, true)
	_ = f.l.set(true)
	_ = f.l.set(true)
	if len(f.requests) != 1 {
		t.Errorf("%d requests, want 1", len(f.requests))
	}
}

// An off before the browser answers releases the lock it then grants.
func TestWakeLockOffBeforeGrant(t *testing.T) {
	f := newFakeWakeLock(t, true)
	_ = f.l.set(true)
	_ = f.l.set(false)
	f.grant()
	if f.releases[0] != 1 {
		t.Errorf("a lock granted after off was released %d times, want 1",
			f.releases[0])
	}
}

// A refusal holds nothing, and the next on asks again.
func TestWakeLockRefused(t *testing.T) {
	f := newFakeWakeLock(t, true)
	_ = f.l.set(true)
	f.refuse()
	_ = f.l.set(false)
	_ = f.l.set(true)
	if len(f.requests) != 2 {
		t.Errorf("%d requests after a refusal and a new on, want 2",
			len(f.requests))
	}
}

// drop makes the browser let go of the last lock, as it does for a
// hidden page or a power saver.
func (f *fakeWakeLock) drop() {
	f.sentinels[len(f.sentinels)-1].Set("released", true)
	f.listener.Invoke()
}

// setMinHold sets wakeLockMinHold for one test.
func setMinHold(t *testing.T, d time.Duration) {
	prev := wakeLockMinHold
	wakeLockMinHold = d
	t.Cleanup(func() { wakeLockMinHold = prev })
}

// A lock dropped while the page is hidden is not asked for again here:
// gui ends the request and asks again when the page is shown.
func TestWakeLockDroppedWhileHidden(t *testing.T) {
	setMinHold(t, 0)
	f := newFakeWakeLock(t, true)
	_ = f.l.set(true)
	f.grant()
	f.doc.Set("hidden", true)
	f.drop()
	if len(f.requests) != 1 {
		t.Fatalf("%d requests after a drop while hidden, want 1",
			len(f.requests))
	}
	f.doc.Set("hidden", false)
	_ = f.l.set(false)
	_ = f.l.set(true)
	if len(f.requests) != 2 {
		t.Errorf("%d requests after off and on, want 2", len(f.requests))
	}
	if f.releases[0] != 0 {
		t.Errorf("a dropped lock was released again %d times", f.releases[0])
	}
}

// A lock dropped while the page is still visible is asked for again at
// once: gui still takes the request as held and will not ask.
func TestWakeLockDroppedWhileVisible(t *testing.T) {
	setMinHold(t, 0)
	f := newFakeWakeLock(t, true)
	_ = f.l.set(true)
	f.grant()
	f.drop()
	if len(f.requests) != 2 {
		t.Fatalf("%d requests after a drop while visible, want 2",
			len(f.requests))
	}
	f.grant()
	_ = f.l.set(false)
	if f.releases[1] != 1 {
		t.Errorf("the new lock was released %d times on off, want 1",
			f.releases[1])
	}
}

// A browser that drops each lock right after granting it is not asked
// again in a loop.
func TestWakeLockDroppedRightAfterGrant(t *testing.T) {
	setMinHold(t, time.Hour)
	f := newFakeWakeLock(t, true)
	_ = f.l.set(true)
	f.grant()
	f.drop()
	if len(f.requests) != 1 {
		t.Errorf("%d requests after a drop right after the grant, want 1",
			len(f.requests))
	}
}

// Without the API (an http page, an old browser), on is refused with
// ErrUnsupported, which gui reports.
func TestWakeLockWithoutAPI(t *testing.T) {
	f := newFakeWakeLock(t, false)
	if err := f.l.set(true); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("set(true) = %v, want ErrUnsupported", err)
	}
	if err := f.l.set(false); err != nil {
		t.Errorf("set(false) = %v, want nil", err)
	}
}
