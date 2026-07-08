//go:build !js && !android && !ios

package audio

import "github.com/gopxl/beep/v2"

// Music is a loaded music track.
//
// Only one music track can play at a time.  Starting a new track
// halts the previous one.  For layered or simultaneous audio, use
// [Sound] on separate channels.
type Music struct {
	beepStream beep.StreamSeekCloser
	format     beep.Format
}

// LoadMusic loads a music track from a file path.
// Supports WAV, MP3, OGG, FLAC depending on file extension.
func LoadMusic(path string) (*Music, error) {
	return backend.LoadMusic(path)
}

// Play starts music playback.  loops is the number of extra loops
// (0 = play once, -1 = loop forever).  Any currently playing music
// is halted first.
func (m *Music) Play(loops int) error {
	return backend.MusicPlay(m, loops)
}

// FadeIn starts music with a fade-in over ms milliseconds.
func (m *Music) FadeIn(loops, ms int) error {
	return backend.MusicFadeIn(m, loops, ms)
}

// Free releases the underlying resources.  The Music must not be used
// after calling Free.  Safe to call on a nil Music.
func (m *Music) Free() {
	backend.MusicFree(m)
}

// --- Global music controls (single music channel) ---

// HaltMusic stops the currently playing music immediately.
func HaltMusic() { backend.HaltMusic() }

// FadeOutMusic fades out the current music over ms milliseconds,
// then halts it.
func FadeOutMusic(ms int) { backend.FadeOutMusic(ms) }

// PauseMusic pauses music playback.
func PauseMusic() { backend.PauseMusic() }

// ResumeMusic resumes paused music.
func ResumeMusic() { backend.ResumeMusic() }

// IsMusicPlaying reports whether music is currently playing.
func IsMusicPlaying() bool { return backend.IsMusicPlaying() }

// IsMusicPaused reports whether music is currently paused.
func IsMusicPaused() bool { return backend.IsMusicPaused() }

// RewindMusic rewinds to the beginning.
func RewindMusic() { backend.RewindMusic() }
