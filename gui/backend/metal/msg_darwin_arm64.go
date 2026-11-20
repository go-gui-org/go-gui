//go:build darwin && arm64 && !ios

package metal

// The sends of the event path: one poll, one NSEvent read, one wake. They
// run for each event and each animation frame, so they must not allocate.
// objc.Send allocates 4 to 9 times per call, and a mouse event reads about
// ten values. darwinffi.FCaller makes these sends with no allocation and
// reads float and NSPoint/NSRect results, which purego.SyscallN cannot.
//
// mfc is main-thread only, as the event loop is. A send through it can
// reach Go again (sendEvent: runs delegate methods that send through it):
// that is safe, because each Call reads its arguments into registers
// before the callee runs, and the caller reads the results at once.

import (
	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

var mfc = new(darwinffi.FCaller)

// msg sends sel to obj with up to six integer arguments and returns the
// integer or object result.
func msg(obj objc.ID, sel objc.SEL, args ...uintptr) uintptr {
	n := copy(mfc.X[2:], args)
	if n != len(args) {
		panic("metal: msg takes at most 6 arguments")
	}
	mfc.Send(uintptr(obj), uintptr(sel))
	return mfc.R0
}

// msgF64 sends sel to obj and returns a double result.
func msgF64(obj objc.ID, sel objc.SEL) float64 {
	mfc.Send(uintptr(obj), uintptr(sel))
	return mfc.RD[0]
}

// msgPoint sends sel to obj and returns an NSPoint result.
func msgPoint(obj objc.ID, sel objc.SEL) nsPoint {
	mfc.Send(uintptr(obj), uintptr(sel))
	return nsPoint{mfc.RD[0], mfc.RD[1]}
}

// msgRect sends sel to obj and returns an NSRect result.
func msgRect(obj objc.ID, sel objc.SEL) nsRect {
	mfc.Send(uintptr(obj), uintptr(sel))
	return nsRect{mfc.RD[0], mfc.RD[1], mfc.RD[2], mfc.RD[3]}
}

// msgDateSinceNow returns +[NSDate dateWithTimeIntervalSinceNow:sec],
// autoreleased.
func msgDateSinceNow(sec float64) objc.ID {
	mfc.D[0] = sec
	mfc.Send(uintptr(objs.nsDate), uintptr(sels.dateWithTimeIntervalSinceNow))
	return objc.ID(mfc.R0)
}

// msgWakeEvent builds the application-defined event that ends a wait in
// nextEventMatchingMask:, autoreleased. The location, timestamp and data
// are all zero. data2 is the ninth integer argument, so it goes on the
// stack.
func msgWakeEvent() objc.ID {
	mfc.X = [8]uintptr{
		uintptr(objs.nsEvent), uintptr(sels.otherEventWithType),
		nsEventTypeApplicationDefined, 0, 0, 0, 0, 0,
	}
	mfc.D[0], mfc.D[1], mfc.D[2] = 0, 0, 0 // location, timestamp
	mfc.S[0] = 0                           // data2
	mfc.Call(darwinffi.MsgSend())
	return objc.ID(mfc.R0)
}

// msgCall calls the C function fn with up to eight integer arguments.
func msgCall(fn uintptr, args ...uintptr) uintptr {
	n := copy(mfc.X[:], args)
	if n != len(args) {
		panic("metal: msgCall takes at most 8 arguments")
	}
	mfc.Call(fn)
	return mfc.R0
}
