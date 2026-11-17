//go:build ios

package ios

// keep_awake_ios.go — keeping the screen on (issue #1172), through
// UIApplication.idleTimerDisabled. gui ends the request when the app
// goes to the background and asks again when it comes back.

/*
#include "ios_app.h"
*/
import "C"

// SetKeepAwake turns the idle timer off while on is true.
func (n *nativePlatform) SetKeepAwake(on bool, _, _ string) error {
	C.iosSetKeepAwake(cBool(on))
	return nil
}
