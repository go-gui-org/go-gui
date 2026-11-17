//go:build darwin && !ios

package metal

/*
#include <stdlib.h>
#include "keep_awake_darwin.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// SetKeepAwake begins or ends the process's activity that keeps the
// display on (issue #1172). The activity belongs to the process, so any
// window's platform may end it. app is not passed on: macOS names the
// process itself. pmset -g assertions lists the reason.
func (n *nativePlatform) SetKeepAwake(on bool, _, reason string) error {
	if !on {
		C.metalKeepAwakeEnd()
		return nil
	}
	cs := C.CString(reason)
	defer C.free(unsafe.Pointer(cs))
	if C.metalKeepAwakeBegin(cs) == 0 {
		return errors.New("NSProcessInfo gave no activity")
	}
	return nil
}
