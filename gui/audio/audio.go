//go:build !js && !android && !ios

package audio

import (
	"cmp"
	"fmt"
	"math"
)

// Cfg configures the audio subsystem.  Zero value selects sensible
// defaults.
// exportaudit:keep — reachable from an exported signature
type Cfg struct {
	// Frequency is the output sample rate in Hz.  Default: 44100.
	frequency int
	// OutputChannels is the number of output channels
	// (1 = mono, 2 = stereo).  beep is stereo-only; this field is
	// accepted but ignored.  Default: 2.
	outputChannels int
	// ChunkSize is the speaker buffer size in samples.  Smaller
	// values reduce latency but increase CPU.  Default: 2048.
	chunkSize int
	// MixChannels is the number of mixing channels for sound
	// effects.  Default: 16.
	mixChannels int
}

var (
	backend     Backend = &beepBackend{}
	initialized bool
)

// Init initializes the audio subsystem.  It is opt-in and
// independent of the GUI backend.
//
// Pass zero or one [Cfg]; additional values are ignored.
// Call from any goroutine.  Idempotent — repeated calls return nil.
func Init(opts ...Cfg) error {
	if initialized {
		return nil
	}
	var c Cfg
	if len(opts) > 0 {
		c = opts[0]
	}
	c.frequency = cmp.Or(c.frequency, 44100)
	c.outputChannels = cmp.Or(c.outputChannels, 2)
	c.chunkSize = cmp.Or(c.chunkSize, 2048)
	c.mixChannels = cmp.Or(c.mixChannels, 16)

	if c.frequency < 8000 || c.frequency > 192000 {
		return fmt.Errorf("audio: frequency %d out of range [8000, 192000]", c.frequency)
	}
	if c.chunkSize < 64 || c.chunkSize > 16384 {
		return fmt.Errorf("audio: chunk size %d out of range [64, 16384]", c.chunkSize)
	}
	if c.mixChannels < 1 || c.mixChannels > 256 {
		return fmt.Errorf("audio: mix channels %d out of range [1, 256]", c.mixChannels)
	}

	if err := backend.Init(c); err != nil {
		return err
	}
	initialized = true
	return nil
}

// Quit shuts down the audio subsystem.  All playing sounds and music
// are halted.  Safe to call even if [Init] was never called.
func quit() {
	if !initialized {
		return
	}
	backend.Quit()
	initialized = false
}

// SetMasterVolume sets the volume for all sound channels.
// v is clamped to [0, 1].
func SetMasterVolume(v float64) {
	backend.SetMasterVolume(v)
}

// MasterVolume returns the current master sound volume (0–1).
// exportaudit:keep — collides with the backend's masterVolume field
func MasterVolume() float64 {
	return backend.MasterVolume()
}

// SetMusicVolume sets the global music volume.
// v is clamped to [0, 1].
func setMusicVolume(v float64) {
	backend.SetMusicVolume(v)
}

// MusicVolume returns the current music volume (0–1).
func musicVolume() float64 {
	return backend.MusicVolume()
}

// --- internal helpers ---

func clamp01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return max(0, min(1, v))
}
