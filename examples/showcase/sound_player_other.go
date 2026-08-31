//go:build js || android || ios

package main

import "github.com/go-gui-org/go-gui/gui"

// gui/audio does not build for these targets, so widget sound degrades
// to silence rather than to a build failure. The gui/ side is
// unchanged: a nil SoundPlayer is the default everywhere.

func installWidgetSounds(_ *gui.Window, _ bool) {}

func removeWidgetSounds(_ *gui.Window) {}

var (
	_ = installWidgetSounds
	_ = removeWidgetSounds
)
