//go:build darwin && cgo && !ios

package metal

// The file has no cgo of its own since #1307: the sends go through
// purego. It keeps the cgo build term because nativePlatform and the
// rest of this package still need cgo, until the last port step.

import (
	"errors"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// nsActivityIdleDisplaySleepDisabled is NSActivityIdleDisplaySleepDisabled
// (NSProcessInfo.h): 1 << 40. It keeps the display on, and with it the
// Mac out of idle sleep.
const nsActivityIdleDisplaySleepDisabled = 1 << 40

// keepAwakeActivity is the held activity token, 0 while none is held.
// The token is the request: ending it, or the process exiting, lets
// the display sleep again. Called only from the UI thread, so no lock.
var keepAwakeActivity objc.ID

// SetKeepAwake begins or ends the process's activity that keeps the
// display on (issue #1172). The activity belongs to the process, so any
// window's platform may end it. app is not passed on: macOS names the
// process itself. pmset -g assertions lists the reason.
//
// The sends go through purego (#1307). A keep-awake change is rare, so
// objc.ID.Send, which allocates per call, is fine here.
func (n *nativePlatform) SetKeepAwake(on bool, _, reason string) error {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	info := darwinffi.Class("NSProcessInfo").Send(darwinffi.Sel("processInfo"))
	if !on {
		if keepAwakeActivity == 0 {
			return nil
		}
		info.Send(darwinffi.Sel("endActivity:"), keepAwakeActivity)
		keepAwakeActivity.Send(darwinffi.Sel("release"))
		keepAwakeActivity = 0
		return nil
	}
	if keepAwakeActivity != 0 {
		return nil
	}
	if reason == "" {
		reason = "Keeping the display awake"
	}
	text := darwinffi.NSString(reason)
	if text == 0 {
		text = darwinffi.NSString("Keeping the display awake")
	}
	act := info.Send(darwinffi.Sel("beginActivityWithOptions:reason:"),
		uint64(nsActivityIdleDisplaySleepDisabled), text)
	if act == 0 {
		return errors.New("NSProcessInfo gave no activity")
	}
	// The token is autoreleased. The cgo form held it in an ARC strong
	// static; here an explicit retain holds it past the pool, and the
	// end path releases it.
	keepAwakeActivity = act.Send(darwinffi.Sel("retain"))
	return nil
}
