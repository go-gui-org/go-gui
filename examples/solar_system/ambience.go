//go:build !js && !android && !ios

package main

import (
	_ "embed"
	"log"

	"github.com/go-gui-org/go-gui/gui/audio"
)

//go:embed spacey-ambience.wav
var ambienceWAV []byte

// ambienceMusic keeps the decoder alive for the life of the program.
// The mixer holds the streamer chain, but this is the owning handle —
// it must not be Freed while the track loops.
var ambienceMusic *audio.Music //nolint:unused // retained for GC

// startAmbience inits the audio subsystem and loops the embedded
// spacey-ambience.wav forever as background. Failure is logged, not
// fatal — the orrery runs silently when no audio server is present
// (e.g. headless CI or missing PulseAudio).
//
// Music rather than Sound: a Music decodes as it plays, where a Sound
// would hold the whole 4 MB track decoded and resampled to the output
// rate — about 22 MB resident for a clip that plays once and repeats.
func startAmbience() {
	if err := audio.Init(); err != nil {
		log.Printf("solar_system: audio init: %v", err)
		return
	}
	m, err := audio.LoadMusicBytes(ambienceWAV)
	if err != nil {
		log.Printf("solar_system: load ambience: %v", err)
		return
	}
	ambienceMusic = m
	// loops -1 = forever.
	if err := m.Play(-1); err != nil {
		log.Printf("solar_system: play ambience: %v", err)
		return
	}
}
