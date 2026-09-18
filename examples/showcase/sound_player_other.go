//go:build js || android || ios

package main

import "github.com/go-gui-org/go-gui/gui"

// gui/audio does not build for these targets, so widget sound degrades
// to silence rather than to a build failure. The gui/ side is
// unchanged: a nil SoundPlayer is the default everywhere. The player
// kind and its labels live in sound_player_kind.go, which builds on
// every platform, so only the install path is stubbed here.

func installWidgetSounds(_ *gui.Window, _ soundPlayerKind) {}

func removeWidgetSounds(_ *gui.Window) {}

// The only caller, demo_audio.go, is desktop-only; without this the
// unused linter fails lint-js on the stub.
var _ = removeWidgetSounds
