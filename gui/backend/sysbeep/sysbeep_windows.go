//go:build windows

package sysbeep

import "golang.org/x/sys/windows"

var (
	user32       = windows.NewLazySystemDLL("user32.dll")
	pMessageBeep = user32.NewProc("MessageBeep")
)

// mbDefault is MB_OK — the sound mapped to the "Default Beep" system
// event. Passing 0xFFFFFFFF instead would fall back to the PC speaker
// when no sound is configured, which is jarring on modern machines.
const mbDefault = 0x00000000

// Play plays the Windows default alert sound via MessageBeep. The call
// is asynchronous: it returns as soon as the sound is queued.
func Play() { _, _, _ = pMessageBeep.Call(uintptr(mbDefault)) }

// Available reports that this platform has a system alert sound.
func Available() bool { return true }
