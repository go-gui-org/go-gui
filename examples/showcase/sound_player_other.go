//go:build js || android || ios

package main

import "github.com/go-gui-org/go-gui/gui"

// gui/audio does not build for these targets, so widget sound degrades
// to silence rather than to a build failure. The gui/ side is
// unchanged: a nil SoundPlayer is the default everywhere.

type soundPlayerKind uint8

const (
	soundPlayerSynth soundPlayerKind = iota
	soundPlayerBeep
	soundPlayerSystem
)

func installWidgetSounds(_ *gui.Window, _ soundPlayerKind) {}

func removeWidgetSounds(_ *gui.Window) {}

var (
	_ = soundPlayerSynth
	_ = soundPlayerBeep
	_ = soundPlayerSystem
	_ = installWidgetSounds
	_ = removeWidgetSounds
)

var soundPlayerLabels = []string{
	"Synthesized (gui/audio)",
	"System event sounds",
	"System alert on errors only",
}

func soundPlayerValue(kind soundPlayerKind) string {
	switch kind {
	case soundPlayerBeep:
		return soundPlayerLabels[2]
	case soundPlayerSystem:
		return soundPlayerLabels[1]
	default:
		return soundPlayerLabels[0]
	}
}

func soundPlayerKindFor(label string) soundPlayerKind {
	switch label {
	case soundPlayerLabels[2]:
		return soundPlayerBeep
	case soundPlayerLabels[1]:
		return soundPlayerSystem
	default:
		return soundPlayerSynth
	}
}
