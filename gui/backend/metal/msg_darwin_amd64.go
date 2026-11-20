//go:build darwin && cgo && !ios

package metal

// The event-path sends of msg_darwin_arm64.go, for darwin/amd64. They go
// through objc.Send and purego.SyscallN, which allocate on each call.
// darwinffi.FCaller is arm64 only, and darwin/amd64 loses the backend in
// the last port step (docs/specs/cgo-free-backend-feasibility.md § Port
// order), so this file only keeps the amd64 build working until then.

import (
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

func msg(obj objc.ID, sel objc.SEL, args ...uintptr) uintptr {
	all := append([]uintptr{uintptr(obj), uintptr(sel)}, args...)
	r, _, _ := purego.SyscallN(darwinffi.MsgSend(), all...)
	return r
}

func msgF64(obj objc.ID, sel objc.SEL) float64 {
	return objc.Send[float64](obj, sel)
}

func msgPoint(obj objc.ID, sel objc.SEL) nsPoint {
	return objc.Send[nsPoint](obj, sel)
}

func msgRect(obj objc.ID, sel objc.SEL) nsRect {
	return objc.Send[nsRect](obj, sel)
}

func msgDateSinceNow(sec float64) objc.ID {
	return objs.nsDate.Send(sels.dateWithTimeIntervalSinceNow, sec)
}

func msgWakeEvent() objc.ID {
	return objs.nsEvent.Send(sels.otherEventWithType,
		uint(nsEventTypeApplicationDefined), nsPoint{}, uint(0), float64(0),
		0, objc.ID(0), int16(0), 0, 0)
}

func msgCall(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}
