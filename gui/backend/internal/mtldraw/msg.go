//go:build darwin && arm64

package mtldraw

import (
	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// mfc is the renderer's one FCaller. Every send from this package goes
// through it, with no heap allocation (see darwinffi.FCaller). It is
// global because an FCaller must not live on the stack. Only the
// backend's main thread draws, so one is enough.
//
// The backend packages keep their own FCaller. Two callers on one
// thread never overlap: a send returns before the next one starts.
var mfc = new(darwinffi.FCaller)

// sels holds the NSObject selectors the renderer sends. loadRenderer
// fills it, along with rsel.
var sels struct {
	alloc, init, retain, release objc.SEL
}

// msg sends sel to obj with up to six integer arguments and returns X0.
// Float arguments go in mfc.D before the call (see msgClearColor).
func msg(obj objc.ID, sel objc.SEL, args ...uintptr) uintptr {
	n := copy(mfc.X[2:], args)
	if n != len(args) {
		panic("mtldraw: msg takes at most 6 arguments")
	}
	mfc.Send(uintptr(obj), uintptr(sel))
	return mfc.R0
}

// msgCall calls the C function fn with up to eight integer arguments.
func msgCall(fn uintptr, args ...uintptr) uintptr {
	n := copy(mfc.X[:], args)
	if n != len(args) {
		panic("mtldraw: msgCall takes at most 8 arguments")
	}
	mfc.Call(fn)
	return mfc.R0
}
