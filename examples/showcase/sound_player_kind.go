package main

// soundPlayerKind picks which of the three players the showcase
// installs. All three render the same cues; they differ in what a cue
// sounds like and in what they cost to ship.
//
// This file carries no build tag on purpose: the kind, its labels and
// the two mappers are identical on every platform, while only the
// install path differs (gui/audio does not build for js, android or
// ios — see sound_player.go and sound_player_other.go). One copy means
// the mobile stub and the desktop demo cannot drift apart.
type soundPlayerKind uint8

const (
	// soundPlayerSynth is the synthesized player: every cue is a
	// blip, no assets, gain honoured.
	soundPlayerSynth soundPlayerKind = iota
	// soundPlayerBeep is the zero-dependency path from the guide: the
	// system alert on SoundError and silence otherwise, with no audio
	// library. What an app wants when it only needs to signal a
	// rejection.
	soundPlayerBeep
	// soundPlayerSystem is the platform's own event sounds: a cue for
	// every role, no assets, no audio library, gain ignored.
	soundPlayerSystem
)

// soundPlayerLabels are the Select's options, and soundPlayerValue /
// soundPlayerKindFor map them onto the player kind so the app state
// stays typed rather than holding a string.
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
