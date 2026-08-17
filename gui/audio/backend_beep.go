//go:build !js && !android && !ios

package audio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gopxl/beep/v2"
)

var _ Backend = (*beepBackend)(nil)

// ---------------------------------------------------------------------------
// musicState
// ---------------------------------------------------------------------------

// musicState manages the single music track.
type musicState struct {
	ctrl   *beep.Ctrl
	volume float64
}

// ---------------------------------------------------------------------------
// beepBackend
// ---------------------------------------------------------------------------

type beepBackend struct {
	sampleRate   beep.SampleRate
	bufferSize   int
	channels     *channelMixer
	masterVolume float64
	music        musicState
	initialized  bool
}

func (b *beepBackend) Init(opts Cfg) error {
	if b.initialized {
		return nil
	}
	// Init (audio.go) already applied the defaults and range-validated
	// every field, so no re-defaulting here.
	sr := beep.SampleRate(opts.frequency)
	bufSize := opts.chunkSize
	nch := opts.mixChannels
	if err := outputInit(sr, bufSize); err != nil {
		return fmt.Errorf("audio: init output: %w", err)
	}
	b.sampleRate = sr
	b.bufferSize = bufSize
	b.masterVolume = 1
	b.music.volume = 1
	b.music.ctrl = &beep.Ctrl{}
	b.channels = newChannelMixer(nch, &b.masterVolume)

	outputPlay(b.channels)
	outputPlay(&neverDrain{streamer: b.music.ctrl})

	b.initialized = true
	return nil
}

func (b *beepBackend) Quit() {
	if !b.initialized {
		return
	}
	// Stop the output's playback goroutine before mutating the streamers
	// it reads; otherwise halt/Streamer writes race the mixer callback.
	outputClose()
	b.channels.halt(-1)
	b.music.ctrl.Streamer = nil
	b.initialized = false
}

// --- master volume ---

func (b *beepBackend) SetMasterVolume(v float64) {
	b.masterVolume = clamp01(v)
}

func (b *beepBackend) MasterVolume() float64 {
	return b.masterVolume
}

// --- music volume ---

func (b *beepBackend) SetMusicVolume(v float64) {
	b.music.volume = clamp01(v)
}

func (b *beepBackend) MusicVolume() float64 {
	return b.music.volume
}

// --- load / decode ---

func (b *beepBackend) LoadMusic(path string) (*Music, error) {
	ext := filepath.Ext(path)
	// #nosec G304 — path is a public-API argument; loading a caller-named
	// audio file by arbitrary path is the intended behavior.
	rc, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("audio: open music %q: %w", path, err)
	}
	stream, format, err := decodeReader(ext, rc)
	if err != nil {
		_ = rc.Close()
		return nil, err
	}
	return &Music{beepStream: stream, format: format}, nil
}

func (b *beepBackend) LoadSound(path string) (*Sound, error) {
	// #nosec G304 — path is a public-API argument; loading a caller-named
	// audio file by arbitrary path is the intended behavior.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("audio: read sound %q: %w", path, err)
	}
	return b.LoadSoundBytes(data)
}

func (b *beepBackend) LoadSoundBytes(data []byte) (*Sound, error) {
	const maxBytes = 50 << 20 // 50 MB
	if len(data) > maxBytes {
		return nil, fmt.Errorf(
			"audio: sound data too large (%d bytes, max %d)",
			len(data), maxBytes)
	}
	stream, format, err := decodeBytes(data)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()
	buf := beep.NewBuffer(format)
	buf.Append(stream)
	return &Sound{buffer: buf, format: format, volume: 1}, nil
}

// --- music playback ---

func (b *beepBackend) MusicFree(m *Music) {
	if m == nil || m.beepStream == nil {
		return
	}
	b.music.ctrl.Streamer = nil
	_ = m.beepStream.Close()
	m.beepStream = nil
}

func (b *beepBackend) MusicPlay(m *Music, loops int) error {
	if m == nil || m.beepStream == nil {
		return errors.New("audio: music not loaded")
	}
	if err := m.beepStream.Seek(0); err != nil {
		return fmt.Errorf("audio: seek music: %w", err)
	}
	b.music.ctrl.Streamer = b.musicChain(m, loops)
	b.music.ctrl.Paused = false
	return nil
}

func (b *beepBackend) MusicFadeIn(m *Music, loops, ms int) error {
	if m == nil || m.beepStream == nil {
		return errors.New("audio: music not loaded")
	}
	if err := m.beepStream.Seek(0); err != nil {
		return fmt.Errorf("audio: seek music: %w", err)
	}
	b.music.ctrl.Streamer = b.musicChainFade(m, loops, ms)
	b.music.ctrl.Paused = false
	return nil
}

func (b *beepBackend) musicChain(m *Music, loops int) beep.Streamer {
	var s beep.Streamer
	switch {
	case loops == 0:
		s = m.beepStream
	case loops < 0:
		ls, err := beep.Loop2(m.beepStream)
		if err != nil {
			s = m.beepStream
		} else {
			s = ls
		}
	default:
		ls, err := beep.Loop2(m.beepStream, beep.LoopTimes(loops))
		if err != nil {
			s = m.beepStream
		} else {
			s = ls
		}
	}
	return &volumeStreamer{
		streamer: s,
		getVolume: func() float64 {
			return b.music.volume * b.masterVolume
		},
	}
}

func (b *beepBackend) musicChainFade(m *Music, loops, ms int) beep.Streamer {
	inner := b.musicChain(m, loops)
	return &fadeStreamer{
		streamer:   inner,
		sampleRate: b.sampleRate,
		startVol:   0,
		targetVol:  1,
		endSamples: b.sampleRate.N(time.Duration(ms) * time.Millisecond),
	}
}

func (b *beepBackend) HaltMusic() {
	b.music.ctrl.Streamer = nil
}

func (b *beepBackend) FadeOutMusic(ms int) {
	if b.music.ctrl.Streamer == nil {
		return
	}
	inner := b.music.ctrl.Streamer
	b.music.ctrl.Streamer = &fadeStreamer{
		streamer:   inner,
		sampleRate: b.sampleRate,
		startVol:   1,
		targetVol:  0,
		endSamples: b.sampleRate.N(time.Duration(ms) * time.Millisecond),
		onComplete: func() { b.music.ctrl.Streamer = nil },
	}
}

func (b *beepBackend) PauseMusic() {
	b.music.ctrl.Paused = true
}

func (b *beepBackend) ResumeMusic() {
	b.music.ctrl.Paused = false
}

func (b *beepBackend) IsMusicPlaying() bool {
	return b.music.ctrl.Streamer != nil && !b.music.ctrl.Paused
}

func (b *beepBackend) IsMusicPaused() bool {
	return b.music.ctrl.Streamer != nil && b.music.ctrl.Paused
}

func (b *beepBackend) RewindMusic() {
	if ssc, ok := b.music.ctrl.Streamer.(beep.StreamSeeker); ok {
		_ = ssc.Seek(0)
	}
}

// --- sound playback ---

func (b *beepBackend) SoundFree(s *Sound) {
	if s == nil || s.buffer == nil {
		return
	}
	s.buffer = nil
}

func (b *beepBackend) SoundPlay(s *Sound, channel, loops int) (int, error) {
	if s == nil || s.buffer == nil {
		return -1, errors.New("audio: sound not loaded")
	}
	if channel < 0 {
		channel = b.channels.firstFree()
	}
	if channel < 0 || channel >= b.channels.numChannels() {
		return -1, errors.New("audio: no free channel for sound")
	}
	var streamer beep.Streamer
	switch {
	case loops == 0:
		streamer = s.buffer.Streamer(0, s.buffer.Len())
	case loops < 0:
		ls, err := beep.Loop2(s.buffer.Streamer(0, s.buffer.Len()))
		if err != nil {
			streamer = s.buffer.Streamer(0, s.buffer.Len())
		} else {
			streamer = ls
		}
	default:
		ls, err := beep.Loop2(s.buffer.Streamer(0, s.buffer.Len()),
			beep.LoopTimes(loops))
		if err != nil {
			streamer = s.buffer.Streamer(0, s.buffer.Len())
		} else {
			streamer = ls
		}
	}
	b.channels.set(channel, &volumeStreamer{
		streamer:  streamer,
		getVolume: func() float64 { return s.volume },
	})
	return channel, nil
}

func (b *beepBackend) SoundFadeIn(s *Sound, channel, loops, ms int) (int, error) {
	if s == nil || s.buffer == nil {
		return -1, errors.New("audio: sound not loaded")
	}
	if channel < 0 {
		channel = b.channels.firstFree()
	}
	if channel < 0 || channel >= b.channels.numChannels() {
		return -1, errors.New("audio: no free channel for sound")
	}
	var streamer beep.Streamer
	switch {
	case loops == 0:
		streamer = s.buffer.Streamer(0, s.buffer.Len())
	case loops < 0:
		ls, err := beep.Loop2(s.buffer.Streamer(0, s.buffer.Len()))
		if err != nil {
			streamer = s.buffer.Streamer(0, s.buffer.Len())
		} else {
			streamer = ls
		}
	default:
		ls, err := beep.Loop2(s.buffer.Streamer(0, s.buffer.Len()),
			beep.LoopTimes(loops))
		if err != nil {
			streamer = s.buffer.Streamer(0, s.buffer.Len())
		} else {
			streamer = ls
		}
	}
	volStreamer := &volumeStreamer{
		streamer:  streamer,
		getVolume: func() float64 { return s.volume },
	}
	fade := &fadeStreamer{
		streamer:   volStreamer,
		sampleRate: b.sampleRate,
		startVol:   0,
		targetVol:  1,
		endSamples: b.sampleRate.N(time.Duration(ms) * time.Millisecond),
	}
	b.channels.set(channel, fade)
	return channel, nil
}

func (b *beepBackend) SoundSetVolume(s *Sound, v float64) {
	if s == nil {
		return
	}
	s.volume = clamp01(v)
}

func (b *beepBackend) SoundVolume(s *Sound) float64 {
	if s == nil {
		return 0
	}
	return s.volume
}

// --- channel controls ---

func (b *beepBackend) HaltChannel(channel int) {
	b.channels.halt(channel)
}

func (b *beepBackend) FadeOutChannel(channel, ms int) {
	if channel < 0 || channel >= b.channels.numChannels() {
		return
	}
	ch := channel // capture for closure
	b.channels.mu.Lock()
	inner := b.channels.chans[ch].Streamer
	if inner == nil {
		b.channels.mu.Unlock()
		return
	}
	b.channels.chans[ch].Streamer = &fadeStreamer{
		streamer:   inner,
		sampleRate: b.sampleRate,
		startVol:   1,
		targetVol:  0,
		endSamples: b.sampleRate.N(time.Duration(ms) * time.Millisecond),
	}
	b.channels.mu.Unlock()
}

func (b *beepBackend) PauseChannel(channel int) {
	b.channels.pause(channel)
}

func (b *beepBackend) ResumeChannel(channel int) {
	b.channels.resume(channel)
}

func (b *beepBackend) IsPlaying(channel int) bool {
	return b.channels.isPlaying(channel)
}

// --- live sources ---

// PlaySource starts a live [Source] on the given channel.  The adapter
// is a plain struct: Fill writes into the caller-owned buffer, so the
// path allocates nothing per callback.
func (b *beepBackend) PlaySource(channel int, s Source) error {
	if channel < 0 {
		channel = b.channels.firstFree()
	}
	if channel < 0 {
		return errors.New("audio: no free channel for source")
	}
	if channel >= b.channels.numChannels() {
		return fmt.Errorf("audio: channel %d out of range [0, %d)",
			channel, b.channels.numChannels()-1)
	}
	b.channels.set(channel, &sourceStreamer{src: s})
	return nil
}

// SampleRate returns the configured output sample rate in Hz.
func (b *beepBackend) SampleRate() int {
	return int(b.sampleRate)
}
