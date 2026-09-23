//go:build !js && !android && !ios

package audio

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"sync"
)

// Cfg configures the audio subsystem.  Zero value selects sensible
// defaults.
// exportaudit:keep — reachable from an exported signature
type Cfg struct {
	// Frequency is the output sample rate in Hz.  Default: 44100.
	// exportaudit:keep — caller-facing config, documented in widget_audio.md
	Frequency int
	// OutputChannels is the number of output channels
	// (1 = mono, 2 = stereo).  beep is stereo-only; this field is
	// accepted but ignored.  Default: 2.
	// exportaudit:keep — caller-facing config, documented in widget_audio.md
	OutputChannels int
	// ChunkSize is the speaker buffer size in samples.  Smaller
	// values reduce latency but increase CPU.  Default: 2048.
	// exportaudit:keep — caller-facing config, documented in widget_audio.md
	ChunkSize int
	// MixChannels is the number of mixing channels for sound
	// effects.  Default: 16.
	// exportaudit:keep — caller-facing config, documented in widget_audio.md
	MixChannels int
}

var (
	backend     Backend = &beepBackend{}
	initialized bool
	initMu      sync.Mutex
)

// errNotInitialized is returned by calls that need [Init] to have run.
var errNotInitialized = errors.New("audio: not initialized")

// Init initializes the audio subsystem.  It is opt-in and
// independent of the GUI backend.
//
// Pass zero or one [Cfg]; additional values are ignored.
// Call from any goroutine.  Idempotent — repeated calls return nil.
func Init(opts ...Cfg) error {
	// initMu makes the check, backend.Init and the flag write one step.
	// Without it two first calls both see false and open the sink twice.
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		return nil
	}
	var c Cfg
	if len(opts) > 0 {
		c = opts[0]
	}
	c.Frequency = cmp.Or(c.Frequency, 44100)
	c.OutputChannels = cmp.Or(c.OutputChannels, 2)
	c.ChunkSize = cmp.Or(c.ChunkSize, 2048)
	c.MixChannels = cmp.Or(c.MixChannels, 16)

	if c.Frequency < 8000 || c.Frequency > 192000 {
		return fmt.Errorf("audio: frequency %d out of range [8000, 192000]", c.Frequency)
	}
	if c.OutputChannels != 1 && c.OutputChannels != 2 {
		return fmt.Errorf("audio: output channels %d out of range [1, 2]", c.OutputChannels)
	}
	if c.ChunkSize < 64 || c.ChunkSize > 16384 {
		return fmt.Errorf("audio: chunk size %d out of range [64, 16384]", c.ChunkSize)
	}
	if c.MixChannels < 1 || c.MixChannels > 256 {
		return fmt.Errorf("audio: mix channels %d out of range [1, 256]", c.MixChannels)
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
	initMu.Lock()
	defer initMu.Unlock()
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
