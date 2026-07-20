// Package sysbeep plays the OS alert sound — the same sound the
// platform uses for an out-of-band "something needs your attention"
// event, honoring whatever the user has configured system-wide
// (alert sound choice, alert volume, and any "mute UI sounds"
// accessibility setting).
//
// Deliberately not a general audio API: there is no asset to load, no
// output device held open, and no volume control. Consumers that want
// to play their own sounds should use the gui/audio package instead.
package sysbeep
