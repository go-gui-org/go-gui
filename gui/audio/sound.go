//go:build !js && !android && !ios

package audio

import "github.com/gopxl/beep/v2"

// Sound is a loaded sound effect.
//
// Sounds play on numbered mixing channels.  Pass channel -1 to
// auto-select the first free channel.
type Sound struct {
	buffer *beep.Buffer
	format beep.Format
	volume float64
}

// LoadSound loads a sound effect from a file path.
// Supports WAV, OGG, FLAC, MP3 and other formats depending on file
// extension.
func LoadSound(path string) (*Sound, error) {
	return backend.LoadSound(path)
}

// LoadSoundBytes loads a sound effect from in-memory bytes.
// The caller must not modify data after this call.
func LoadSoundBytes(data []byte) (*Sound, error) {
	return backend.LoadSoundBytes(data)
}

// Play plays the sound on the given channel (-1 = first free).
// loops is the number of extra loops (0 = play once,
// -1 = loop forever).  Returns the channel number used.
func (s *Sound) Play(channel, loops int) (int, error) {
	return backend.SoundPlay(s, channel, loops)
}

// PlayOnce plays the sound once on the first free channel.
func (s *Sound) PlayOnce() (int, error) {
	return backend.SoundPlay(s, -1, 0)
}

// FadeIn plays the sound with a fade-in over ms milliseconds.
func (s *Sound) FadeIn(channel, loops, ms int) (int, error) {
	return backend.SoundFadeIn(s, channel, loops, ms)
}

// SetVolume sets this sound's volume.  v is clamped to [0, 1].
func (s *Sound) SetVolume(v float64) {
	backend.SoundSetVolume(s, v)
}

// Volume returns the sound's current volume (0–1).
func (s *Sound) Volume() float64 {
	return backend.SoundVolume(s)
}

// Free releases the underlying resources.  The Sound must not be used
// after calling Free.  Do not Free a Sound that is still playing —
// halt the channel first.  Safe to call on a nil Sound.
func (s *Sound) Free() {
	backend.SoundFree(s)
}

// --- Channel-level helpers ---

// HaltChannel stops playback on the given channel (-1 = all).
func HaltChannel(channel int) { backend.HaltChannel(channel) }

// FadeOutChannel fades out the given channel over ms milliseconds,
// then halts it.
func FadeOutChannel(channel, ms int) { backend.FadeOutChannel(channel, ms) }

// PauseChannel pauses the given channel (-1 = all).
func PauseChannel(channel int) { backend.PauseChannel(channel) }

// ResumeChannel resumes the given channel (-1 = all).
func ResumeChannel(channel int) { backend.ResumeChannel(channel) }

// IsPlaying reports whether the given channel is currently playing.
func IsPlaying(channel int) bool { return backend.IsPlaying(channel) }
