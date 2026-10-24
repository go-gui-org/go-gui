//go:build !js && !android && !ios

package main

import (
	_ "embed"
	"log"

	"github.com/go-gui-org/go-gui/gui/audio"
)

// The track is FLAC: lossless, so the loop point stays seamless. MP3's
// encoder padding would leave a gap at every loop.
//
// The source was an 8-bit, 16 kHz WAV. The music sits about 32 dB below
// full scale, so it used only about 5 of the 8 bits, and the quantization
// noise came through as an audible hiss. The track is low-passed at
// 3.5 kHz (two 2-pole passes) and stored as 16-bit: the ambient pad is
// almost all below that, so the cut takes the hiss down 13 dB without
// changing the music level. Filtered 16-bit data compresses less well
// than raw 8-bit, so the file is 3 MB, still under the 4 MB WAV. The
// command was:
//
//	ffmpeg -i in.wav -af "aformat=sample_fmts=fltp,lowpass=f=3500:poles=2,
//	  lowpass=f=3500:poles=2" -sample_fmt s16 -compression_level 12 out.flac
//
//go:embed spacey-ambience.flac
var ambienceFLAC []byte

// ambienceMusic keeps the decoder alive for the life of the program.
// The mixer holds the streamer chain, but this is the owning handle —
// it must not be Freed while the track loops.
var ambienceMusic *audio.Music //nolint:unused // retained for GC

// startAmbience inits the audio subsystem and loops the embedded
// spacey-ambience.flac forever as background. Failure is logged, not
// fatal — the orrery runs silently when no audio server is present
// (e.g. headless CI or missing PulseAudio).
//
// Music rather than Sound: a Music decodes as it plays, where a Sound
// would hold the whole two-minute track decoded and resampled to the
// output rate — about 22 MB resident for a clip that plays once and
// repeats.
func startAmbience() {
	ambienceStarts++
	if err := audio.Init(); err != nil {
		log.Printf("solar_system: audio init: %v", err)
		return
	}
	m, err := audio.LoadMusicBytes(ambienceFLAC)
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

// setAmbienceMuted silences or restores the track through the master
// volume, which scales music too. Muting by volume rather than by
// halting keeps the track's place. audio.Init resets the volume to 1,
// but main runs startAmbience before the event loop, so no toggle can
// come before it.
func setAmbienceMuted(muted bool) {
	v := 1.0
	if muted {
		v = 0
	}
	audio.SetMasterVolume(v)
}
