//go:build (!linux || otoaudio) && !js && !android && !ios

package audio

import (
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

// oto output sink (ALSA/CoreAudio/WASAPI) via beep's speaker package.
// See the package doc in backend.go for how the sinks are selected.

func outputInit(sr beep.SampleRate, bufferSize int) error {
	return speaker.Init(sr, bufferSize)
}

func outputPlay(s beep.Streamer) { speaker.Play(s) }

func outputClose() { speaker.Close() }
