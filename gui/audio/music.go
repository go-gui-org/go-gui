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

// LoadMusicBytes loads a music track from in-memory bytes, for a track
// embedded in the binary.  The format is detected from the leading
// magic bytes; WAV, MP3, OGG and FLAC are supported.  The caller must
// not modify data after this call.
//
// Prefer this over [LoadSoundBytes] for anything long — a [Music]
// decodes as it plays, where a [Sound] holds the whole track decoded at
// the output rate.
func LoadMusicBytes(data []byte) (*Music, error) {
	return backend.LoadMusicBytes(data)
}

// Play starts music playback.  loops is the number of extra loops
// (0 = play once, -1 = loop forever).  Any currently playing music
// is halted first.
//
// Every music call below holds initMu, so it cannot run while Init or
// quit is building or tearing down the music state.  Before Init the
// controls do nothing and Play and FadeIn return an error.
func (m *Music) Play(loops int) error {
	initMu.Lock()
	defer initMu.Unlock()
	if !initialized {
		return errNotInitialized
	}
	return backend.MusicPlay(m, loops)
}

// FadeIn starts music with a fade-in over ms milliseconds.  Returns an
// error when audio is not [Init]ialized.
// exportaudit:keep — music API consumed by apps outside this repo
// (the showcase fades its loaded track via the global control).
func (m *Music) FadeIn(loops, ms int) error {
	initMu.Lock()
	defer initMu.Unlock()
	if !initialized {
		return errNotInitialized
	}
	return backend.MusicFadeIn(m, loops, ms)
}

// Free releases the underlying resources.  The Music must not be used
// after calling Free.  Safe to call on a nil Music, and before [Init].
func (m *Music) Free() {
	initMu.Lock()
	defer initMu.Unlock()
	backend.MusicFree(m)
}

// --- Global music controls (single music channel) ---

// HaltMusic stops the currently playing music immediately.
func HaltMusic() {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.HaltMusic()
	}
}

// FadeOutMusic fades out the current music over ms milliseconds,
// then halts it.
func FadeOutMusic(ms int) {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.FadeOutMusic(ms)
	}
}

// PauseMusic pauses music playback.
func pauseMusic() {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.PauseMusic()
	}
}

// ResumeMusic resumes paused music.
func resumeMusic() {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.ResumeMusic()
	}
}

// IsMusicPlaying reports whether music is currently playing.
func isMusicPlaying() bool {
	initMu.Lock()
	defer initMu.Unlock()
	return initialized && backend.IsMusicPlaying()
}

// IsMusicPaused reports whether music is currently paused.
func isMusicPaused() bool {
	initMu.Lock()
	defer initMu.Unlock()
	return initialized && backend.IsMusicPaused()
}

// RewindMusic rewinds to the beginning.
func rewindMusic() {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		backend.RewindMusic()
	}
}
