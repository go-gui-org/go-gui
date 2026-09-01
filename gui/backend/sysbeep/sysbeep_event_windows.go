//go:build windows

package sysbeep

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmm      = windows.NewLazySystemDLL("winmm.dll")
	pPlaySound = winmm.NewProc("PlaySoundW")
)

const (
	// sndAsync returns as soon as the sound is queued.
	sndAsync = 0x00000001
	// sndNoDefault is load-bearing: an alias the user's sound scheme
	// leaves unset plays nothing, instead of falling back to the
	// default beep, which would make an ordinary click sound like an
	// alert.
	sndNoDefault = 0x00000002
	// sndAlias reads the name as a system-event alias from the
	// registry (HKCU\AppEvents\Schemes\Apps\.Default).
	sndAlias = 0x00010000
)

// eventAliases maps an Event onto a Windows system-event alias.
// MenuCommand is the sound Windows itself uses for an interface
// command, so it covers click, toggle and selection; the three
// System* aliases are the documented dialog sounds. An empty entry is
// silent.
var eventAliases = [eventCount]string{
	EventClick:     "MenuCommand",
	EventToggleOn:  "MenuCommand",
	EventToggleOff: "MenuCommand",
	EventSelection: "MenuPopup",
	EventError:     "SystemHand",
	EventNotify:    "Notification.Default",
	EventOpen:      "SystemAsterisk",
	EventSuccess:   ".Default",
}

// playEvent calls PlaySoundW with the alias flags. A failed call is
// silent: a cue must never surface an error to the user.
func playEvent(e Event) {
	alias := eventAliases[e]
	if alias == "" {
		return
	}
	name, err := windows.UTF16PtrFromString(alias)
	if err != nil {
		return
	}
	_, _, _ = pPlaySound.Call(
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(sndAlias|sndAsync|sndNoDefault),
	)
}

// eventAvailable reports that Windows has system event sounds. Whether
// any given alias is bound to a file is the user's scheme choice, and
// sndNoDefault makes an unbound one silent rather than a beep.
func eventAvailable() bool { return true }
