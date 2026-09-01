//go:build !js && !android && !ios

package audio

import "github.com/gopxl/beep/v2"

// resampleQuality is the beep interpolation window.  4 is beep's
// documented "good quality" point; 1 is audibly rough, and past ~8 the
// cost climbs with no gain on speech/ambience material.
const resampleQuality = 4

// resampleTo wraps s so it streams at the dst rate when it was decoded
// at src.  Returns s unchanged when the rates already match or either
// rate is unknown (0) — the identity case must not allocate, because
// every playback path calls through here.
//
// The output sink runs at one fixed rate (see [Init]), so a stream left
// at its own rate plays at dst/src speed: a 16 kHz file into a 44.1 kHz
// speaker runs 2.76x fast, about 1.5 octaves sharp.
func resampleTo(src, dst beep.SampleRate, s beep.Streamer) beep.Streamer {
	if src <= 0 || dst <= 0 || src == dst {
		return s
	}
	return beep.Resample(resampleQuality, src, dst, s)
}
