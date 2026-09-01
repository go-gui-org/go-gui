package sysbeep

// Event names a user-interface sound the platform already ships as a
// system event sound. It is not a sample and not a file path: each
// platform maps the event onto its own sound-naming scheme (NSSound
// names on macOS, registry aliases on Windows, freedesktop sound-naming
// ids on Linux), so an app gets a native-sounding cue with no assets.
//
// The list is open and append-only, mirroring gui.SoundCue. An event a
// platform has no sound for is silent, never an error (issue #469).
type Event uint8

// Event values.
const (
	// EventClick is a momentary activation — a button, a menu item.
	EventClick Event = iota
	// EventToggleOn is a state that went off -> on.
	EventToggleOn
	// EventToggleOff is a state that went on -> off.
	EventToggleOff
	// EventSelection is one option picked out of several.
	EventSelection
	// EventError is a rejection: invalid input, a refused commit.
	EventError
	// EventNotify is something that appeared unasked — a toast.
	EventNotify
	// EventOpen is a modal surface opening — a dialog.
	EventOpen
	// EventSuccess is a multi-field action being accepted.
	EventSuccess
	// eventCount bounds the per-platform mapping tables. Keep last.
	eventCount
)

// PlayEvent plays the system sound mapped to e, if this platform has
// one. Non-blocking, and silent — never a panic — for an event this
// platform does not map, for an out-of-range value, and on every target
// with no event sounds at all.
//
// Unlike Play, which is the out-of-band alert, these are the ordinary
// interface sounds: they follow the user's system event-sound settings
// and carry no app-level volume. See gui.NewSystemSoundPlayer.
func PlayEvent(e Event) {
	if e >= eventCount {
		return
	}
	playEvent(e)
}

// EventAvailable reports whether PlayEvent can produce sound on this
// platform. False on targets with no event sounds, and on Linux when
// canberra-gtk-play is not installed.
func EventAvailable() bool { return eventAvailable() }
