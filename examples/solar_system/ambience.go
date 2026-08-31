//go:build !js && !android && !ios

package main

import (
	_ "embed"
	"log"

	"github.com/go-gui-org/go-gui/gui/audio"
)

//go:embed spacey-ambience.wav
var ambienceWAV []byte

// ambienceSound keeps the decoded buffer alive for the life of the
// program. The mixer retains its own streamer, but the Sound's buffer
// is the owning copy, so it must not be GC'd or Freed while looping.
var ambienceSound *audio.Sound //nolint:unused // retained for GC

// startAmbience inits the audio subsystem and loops the embedded
// spacey-ambience.wav forever as background. Failure is logged, not
// fatal — the orrery runs silently when no audio server is present
// (e.g. headless CI or missing PulseAudio).
func startAmbience() {
	if err := audio.Init(); err != nil {
		log.Printf("solar_system: audio init: %v", err)
		return
	}
	snd, err := audio.LoadSoundBytes(ambienceWAV)
	if err != nil {
		log.Printf("solar_system: load ambience: %v", err)
		return
	}
	ambienceSound = snd
	// channel -1 = first free, loops -1 = forever.
	if _, err := snd.Play(-1, -1); err != nil {
		log.Printf("solar_system: play ambience: %v", err)
		return
	}
}
