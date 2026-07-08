//go:build !js && !android && !ios

// Package audio provides opt-in audio playback for sound effects and
// music.  Call [Init] before loading or playing audio.  Call [Quit]
// when done.
//
// Sound effects play on numbered mixing channels (default 16) and
// support overlapping playback.  Music is a single track — starting
// a new track halts the previous one.
package audio

// Backend is the audio subsystem implementation.
type Backend interface {
	Init(cfg Cfg) error
	Quit()
	SetMasterVolume(v float64)
	MasterVolume() float64
	SetMusicVolume(v float64)
	MusicVolume() float64
	LoadMusic(path string) (*Music, error)
	LoadSound(path string) (*Sound, error)
	LoadSoundBytes(data []byte) (*Sound, error)
	MusicFree(m *Music)
	MusicPlay(m *Music, loops int) error
	MusicFadeIn(m *Music, loops, ms int) error
	HaltMusic()
	FadeOutMusic(ms int)
	PauseMusic()
	ResumeMusic()
	IsMusicPlaying() bool
	IsMusicPaused() bool
	RewindMusic()
	SoundFree(s *Sound)
	SoundPlay(s *Sound, channel, loops int) (int, error)
	SoundFadeIn(s *Sound, channel, loops, ms int) (int, error)
	SoundSetVolume(s *Sound, v float64)
	SoundVolume(s *Sound) float64
	HaltChannel(channel int)
	FadeOutChannel(channel, ms int)
	PauseChannel(channel int)
	ResumeChannel(channel int)
	IsPlaying(channel int) bool
}
