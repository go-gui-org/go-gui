//go:build linux && !android

package sysbeep

import (
	"os/exec"
	"sync"
)

// Linux has no in-process system-alert API equivalent to NSBeep or
// MessageBeep, so the freedesktop sound theme is played through
// canberra-gtk-play — the same helper GTK apps use, which honors the
// user's chosen sound theme and their desktop's event-sound setting.
// Absent that binary there is no sensible fallback (the kernel console
// beep is not reachable from a Wayland/X client), so Play is a no-op.
var lookupOnce = sync.OnceValue(func() string {
	path, err := exec.LookPath("canberra-gtk-play")
	if err != nil {
		return ""
	}
	return path
})

// Play plays the freedesktop "bell" event sound, if canberra-gtk-play
// is installed. Non-blocking: the helper is spawned and reaped in the
// background so a bell never stalls the caller.
func Play() {
	path := lookupOnce()
	if path == "" {
		return
	}
	// #nosec G204 — path from exec.LookPath, args are fixed
	cmd := exec.Command(path, "-i", "bell")
	if err := cmd.Start(); err != nil {
		return
	}
	// Reap asynchronously; leaving this out would accumulate zombies
	// for the life of the process.
	go func() { _ = cmd.Wait() }()
}

// Available reports whether a system alert sound can be played, i.e.
// whether canberra-gtk-play was found on PATH.
func Available() bool { return lookupOnce() != "" }
