//go:build !js && !android && !ios

package audio

import "github.com/gopxl/beep/v2"

// Sound is a loaded sound effect.
//
// Sounds play on numbered mixing channels.  Pass channel -1 to
// auto-select the first free channel.
// exportaudit:keep — sample-playback API consumed by apps outside this
// repo; the showcase exercises live sources instead (issue #331).
type Sound struct {
	buffer *beep.Buffer
	format beep.Format
	// volume is set by the app and read by the audio thread every buffer.
	volume atomicFloat64
}

// LoadSound loads a sound effect from a file path.
// Supports WAV, OGG, FLAC, MP3 and other formats depending on file
// extension.
func loadSound(path string) (*Sound, error) {
	return backend.LoadSound(path)
}

// LoadSoundBytes loads a sound effect from in-memory bytes.
// The caller must not modify data after this call.
// exportaudit:keep — sample-playback API consumed by apps outside this
// repo; the showcase exercises live sources instead (issue #331).
func LoadSoundBytes(data []byte) (*Sound, error) {
	return backend.LoadSoundBytes(data)
}

// Play plays the sound on the given channel (-1 = first free).
// loops is the number of extra loops (0 = play once,
// -1 = loop forever).  Returns the channel number used.
//
// Play, PlayOnce, FadeIn, Free and the channel helpers hold initMu, so
// they cannot run while Init or quit is building or tearing down the
// mixer.  Before Init the channel helpers do nothing and the play calls
// return an error.  Loading and per-sound volume need no lock: they
// touch no mixer state, and what they share with the audio thread is
// atomic.
func (s *Sound) Play(channel, loops int) (int, error) {
	initMu.Lock()
	defer initMu.Unlock()
	if !initialized {
		return -1, errNotInitialized
	}
	return backend.SoundPlay(s, channel, loops)
}

// PlayOnce plays the sound once on the first free channel.  Returns an
// error when audio is not [Init]ialized.
// exportaudit:keep — sample-playback API consumed by apps outside this
// repo; the showcase exercises live sources instead (issue #331).
func (s *Sound) PlayOnce() (int, error) {
	return s.Play(-1, 0)
}

// FadeIn plays the sound with a fade-in over ms milliseconds.  Returns
// an error when audio is not [Init]ialized.
// exportaudit:keep — sample-playback API consumed by apps outside this
// repo; the showcase exercises live sources instead (issue #331).
func (s *Sound) FadeIn(channel, loops, ms int) (int, error) {
	initMu.Lock()
	defer initMu.Unlock()
	if !initialized {
		return -1, errNotInitialized
	}
	return backend.SoundFadeIn(s, channel, loops, ms)
}

// SetVolume sets this sound's volume.  v is clamped to [0, 1].
func (s *Sound) setVolume(v float64) {
	backend.SoundSetVolume(s, v)
}

// Volume returns the sound's current volume (0–1).
func (s *Sound) Volume() float64 {
	return backend.SoundVolume(s)
}

// Free releases the underlying resources.  The Sound must not be used
// after calling Free.  Do not Free a Sound that is still playing —
// halt the channel first.  Safe to call on a nil Sound, and before
// [Init].
func (s *Sound) Free() {
	// The lock orders Free against a Play reading s.buffer on another
	// goroutine.
	initMu.Lock()
	defer initMu.Unlock()
	backend.SoundFree(s)
}

// --- Channel-level helpers ---

// HaltChannel stops playback on the given channel (-1 = all).
// exportaudit:keep — channel API consumed by apps outside this repo;
// the showcase exercises live sources instead (issue #331).
func HaltChannel(channel int) {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.HaltChannel(channel)
	}
}

// FadeOutChannel fades out the given channel over ms milliseconds,
// then halts it.
func fadeOutChannel(channel, ms int) {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.FadeOutChannel(channel, ms)
	}
}

// PauseChannel pauses the given channel (-1 = all).
func pauseChannel(channel int) {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.PauseChannel(channel)
	}
}

// ResumeChannel resumes the given channel (-1 = all).
func resumeChannel(channel int) {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.ResumeChannel(channel)
	}
}

// IsPlaying reports whether the given channel is currently playing.
// exportaudit:keep — collides with channelMixer.isPlaying
func IsPlaying(channel int) bool {
	initMu.Lock()
	defer initMu.Unlock()
	return initialized && backend.IsPlaying(channel)
}
