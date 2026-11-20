//go:build darwin && !ios

package sysbeep

import (
	"sync"

	"github.com/ebitengine/purego"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// nsBeep is the address of AppKit's NSBeep, bound on first use. It is
// called through purego, so Play works with cgo off (#1307).
var nsBeep = sync.OnceValue(func() uintptr {
	return darwinffi.Func(darwinffi.Open(darwinffi.PathAppKit), "NSBeep")
})

// Play plays the macOS system alert sound via NSBeep.
//
// NSBeep is the AppKit alert sound. It respects the user's selected
// alert sound, the alert volume slider, and the "Play user interface
// sound effects" setting, so a muted Mac stays silent. It is also
// self-coalescing: overlapping calls do not stack into a chorus.
func Play() {
	fn := nsBeep()
	pool := darwinffi.PoolPush()
	purego.SyscallN(fn)
	darwinffi.PoolPop(pool)
}

// Available reports that this platform has a system alert sound.
func Available() bool { return true }
