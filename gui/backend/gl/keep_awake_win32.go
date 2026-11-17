//go:build windows && !js

package gl

// keep_awake_win32.go — keeping the display on (issue #1172).
//
// One power request for the whole process, set for the display and for
// the system: Microsoft's PowerSetRequest documentation says a display
// request alone does not keep the system from sleeping. The request is
// a handle, so any thread may begin or end it, and it ends with the
// process. Its reason shows in `powercfg /requests`.
//
// Limit: on a Modern Standby PC running on battery, Windows ends a
// system request five minutes after the sleep timeout.

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	pPowerCreateRequest = kernel32.NewProc("PowerCreateRequest")
	pPowerSetRequest    = kernel32.NewProc("PowerSetRequest")
	pPowerClearRequest  = kernel32.NewProc("PowerClearRequest")
)

// Power request constants (minwinbase.h, winnt.h).
const (
	powerRequestContextVersion      = 0   // POWER_REQUEST_CONTEXT_VERSION
	powerRequestContextSimpleString = 0x1 // POWER_REQUEST_CONTEXT_SIMPLE_STRING
	powerRequestDisplayRequired     = 0   // PowerRequestDisplayRequired
	powerRequestSystemRequired      = 1   // PowerRequestSystemRequired
)

// reasonContext mirrors REASON_CONTEXT with the SimpleReasonString arm
// of its union. The padding fills the union out to its other arm
// (HMODULE, two ULONGs, LPWSTR*), so the struct has the C size.
type reasonContext struct {
	version uint32
	flags   uint32
	reason  *uint16
	_       [2]uint32
	_       uintptr
}

// powerRequest is the process's request. It is kept at package level
// because gui may end it through another window's platform than the one
// that began it.
var powerRequest struct {
	mu sync.Mutex
	h  windows.Handle // 0 while none is held
}

// SetKeepAwake begins or ends the process's power request. app is not
// passed on: Windows names the process itself.
func (n *nativePlatform) SetKeepAwake(on bool, _, reason string) error {
	powerRequest.mu.Lock()
	defer powerRequest.mu.Unlock()
	if !on {
		if powerRequest.h == 0 {
			return nil
		}
		h := powerRequest.h
		powerRequest.h = 0
		return endPowerRequest(h)
	}
	if powerRequest.h != 0 {
		return nil
	}
	h, err := beginPowerRequest(reason)
	if err != nil {
		return err
	}
	powerRequest.h = h
	return nil
}

// beginPowerRequest makes a power request with reason and sets it for
// the display and the system. Nothing is held when it fails.
func beginPowerRequest(reason string) (windows.Handle, error) {
	for _, p := range [...]*windows.LazyProc{
		pPowerCreateRequest, pPowerSetRequest, pPowerClearRequest,
	} {
		if err := p.Find(); err != nil {
			return 0, fmt.Errorf("%s is unavailable: %w", p.Name, err)
		}
	}
	text, err := windows.UTF16PtrFromString(reason)
	if err != nil {
		return 0, fmt.Errorf("reason: %w", err)
	}
	ctx := reasonContext{
		version: powerRequestContextVersion,
		flags:   powerRequestContextSimpleString,
		reason:  text,
	}
	r, _, createErr := pPowerCreateRequest.Call(uintptr(unsafe.Pointer(&ctx)))
	h := windows.Handle(r)
	if h == windows.InvalidHandle || h == 0 {
		return 0, fmt.Errorf("PowerCreateRequest: %w", createErr)
	}
	ok, _, displayErr := pPowerSetRequest.Call(r, powerRequestDisplayRequired)
	if ok == 0 {
		_ = windows.CloseHandle(h)
		return 0, fmt.Errorf("PowerSetRequest(display): %w", displayErr)
	}
	ok, _, systemErr := pPowerSetRequest.Call(r, powerRequestSystemRequired)
	if ok == 0 {
		pPowerClearRequest.Call(r, powerRequestDisplayRequired)
		_ = windows.CloseHandle(h)
		return 0, fmt.Errorf("PowerSetRequest(system): %w", systemErr)
	}
	return h, nil
}

// endPowerRequest clears both settings and closes the request.
func endPowerRequest(h windows.Handle) error {
	pPowerClearRequest.Call(uintptr(h), powerRequestSystemRequired)
	pPowerClearRequest.Call(uintptr(h), powerRequestDisplayRequired)
	if err := windows.CloseHandle(h); err != nil {
		return fmt.Errorf("CloseHandle: %w", err)
	}
	return nil
}
