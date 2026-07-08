//go:build !js && !android && !ios

package audio

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

var _ Backend = (*beepBackend)(nil)

// ---------------------------------------------------------------------------
// volumeStreamer
// ---------------------------------------------------------------------------

// volumeStreamer wraps a [beep.Streamer] and multiplies each sample by
// a volume factor returned by getVolume, clamped to [0, 1].
type volumeStreamer struct {
	streamer  beep.Streamer
	getVolume func() float64
}

func (v *volumeStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = v.streamer.Stream(samples)
	vol := clamp01(v.getVolume())
	if vol < 1 {
		for i := range n {
			samples[i][0] *= vol
			samples[i][1] *= vol
		}
	}
	return
}

func (v *volumeStreamer) Err() error { return v.streamer.Err() }

// ---------------------------------------------------------------------------
// fadeStreamer
// ---------------------------------------------------------------------------

// fadeStreamer wraps a [beep.Streamer] and ramps the volume from
// startVol to targetVol over a duration.  After the ramp completes:
//   - for fade-in (targetVol > 0), the streamer becomes transparent.
//   - for fade-out (targetVol == 0), the streamer drains and calls
//     onComplete (which must not acquire speaker or channel locks).
type fadeStreamer struct {
	streamer   beep.Streamer
	sampleRate beep.SampleRate
	startVol   float64
	targetVol  float64
	endSamples int
	elapsed    int
	done       bool
	onComplete func()
}

func (f *fadeStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	if f.done || f.endSamples <= 0 {
		if f.targetVol == 0 {
			if f.onComplete != nil {
				f.onComplete()
			}
			return 0, false
		}
		return f.streamer.Stream(samples)
	}
	n, ok = f.streamer.Stream(samples)
	if n == 0 && !ok {
		return 0, false
	}
	for i := range n {
		t := min(float64(f.elapsed)/float64(f.endSamples), 1)
		vol := f.startVol + (f.targetVol-f.startVol)*t
		samples[i][0] *= vol
		samples[i][1] *= vol
		f.elapsed++
	}
	if f.elapsed >= f.endSamples {
		f.done = true
		if f.targetVol == 0 {
			if f.onComplete != nil {
				f.onComplete()
			}
			return n, false
		}
	}
	return n, ok
}

func (f *fadeStreamer) Err() error { return f.streamer.Err() }

// ---------------------------------------------------------------------------
// channelMixer
// ---------------------------------------------------------------------------

// channelMixer provides N numbered sound-effect channels backed by a
// single custom [beep.Streamer].  Each channel wraps a [beep.Ctrl] so
// it can be paused/resumed/replaced without allocations.
type channelMixer struct {
	chans        []*beep.Ctrl
	playing      []bool
	masterVolume *float64
	mu           sync.Mutex
}

func newChannelMixer(n int, master *float64) *channelMixer {
	chans := make([]*beep.Ctrl, n)
	for i := range chans {
		chans[i] = &beep.Ctrl{}
	}
	return &channelMixer{
		chans:        chans,
		playing:      make([]bool, n),
		masterVolume: master,
	}
}

func (cm *channelMixer) Stream(samples [][2]float64) (n int, ok bool) {
	var tmp [512][2]float64
	for len(samples) > 0 {
		toStream := min(len(tmp), len(samples))
		clear(samples[:toStream])
		active := false
		cm.mu.Lock()
		for i := range cm.chans {
			if cm.playing[i] {
				sn, sok := cm.chans[i].Stream(tmp[:toStream])
				if sn == 0 && !sok {
					cm.chans[i].Streamer = nil
					cm.playing[i] = false
				} else {
					for j := range sn {
						samples[j][0] += tmp[j][0]
						samples[j][1] += tmp[j][1]
					}
					active = true
				}
			}
		}
		cm.mu.Unlock()
		if !active {
			for i := range toStream {
				samples[i][0] = 0
				samples[i][1] = 0
			}
		} else if *cm.masterVolume < 1 {
			for i := range toStream {
				samples[i][0] *= *cm.masterVolume
				samples[i][1] *= *cm.masterVolume
			}
		}
		samples = samples[toStream:]
		n += toStream
	}
	return n, true
}

func (cm *channelMixer) Err() error { return nil }

func (cm *channelMixer) firstFree() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for i := range cm.chans {
		if !cm.playing[i] {
			return i
		}
	}
	return -1
}

func (cm *channelMixer) set(ch int, s beep.Streamer) {
	cm.mu.Lock()
	cm.chans[ch].Streamer = s
	cm.chans[ch].Paused = false
	cm.playing[ch] = true
	cm.mu.Unlock()
}

func (cm *channelMixer) halt(channel int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 {
		for i := range cm.chans {
			cm.chans[i].Streamer = nil
			cm.playing[i] = false
		}
		return
	}
	if channel < len(cm.chans) {
		cm.chans[channel].Streamer = nil
		cm.playing[channel] = false
	}
}

func (cm *channelMixer) pause(channel int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 {
		for i := range cm.chans {
			cm.chans[i].Paused = true
		}
		return
	}
	if channel < len(cm.chans) {
		cm.chans[channel].Paused = true
	}
}

func (cm *channelMixer) resume(channel int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 {
		for i := range cm.chans {
			cm.chans[i].Paused = false
		}
		return
	}
	if channel < len(cm.chans) {
		cm.chans[channel].Paused = false
	}
}

func (cm *channelMixer) isPlaying(channel int) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if channel < 0 || channel >= len(cm.chans) {
		return false
	}
	return cm.playing[channel]
}

func (cm *channelMixer) numChannels() int { return len(cm.chans) }

// ---------------------------------------------------------------------------
// neverDrain
// ---------------------------------------------------------------------------

// neverDrain wraps a [beep.Streamer] so it never appears drained.
// When the inner streamer returns 0,false, the buffer is filled with
// silence and ok=true is returned.  This prevents the speaker mixer
// from auto-removing the streamer.
type neverDrain struct {
	streamer beep.Streamer
}

func (n *neverDrain) Stream(samples [][2]float64) (int, bool) {
	if n.streamer == nil {
		clear(samples)
		return len(samples), true
	}
	nn, ok := n.streamer.Stream(samples)
	if !ok {
		clear(samples[nn:])
		return len(samples), true
	}
	return nn, ok
}

func (n *neverDrain) Err() error {
	if n.streamer == nil {
		return nil
	}
	return n.streamer.Err()
}

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

func (b *beepBackend) Init(cfg Cfg) error {
	if b.initialized {
		return nil
	}
	sr := beep.SampleRate(cfg.Frequency)
	if sr <= 0 {
		sr = 44100
	}
	bufSize := cfg.ChunkSize
	if bufSize <= 0 {
		bufSize = 2048
	}
	nch := cfg.MixChannels
	if nch <= 0 {
		nch = 16
	}
	if err := speaker.Init(sr, bufSize); err != nil {
		return fmt.Errorf("audio: init speaker: %w", err)
	}
	b.sampleRate = sr
	b.bufferSize = bufSize
	b.masterVolume = 1
	b.music.volume = 1
	b.music.ctrl = &beep.Ctrl{}
	b.channels = newChannelMixer(nch, &b.masterVolume)

	speaker.Play(b.channels)
	speaker.Play(&neverDrain{streamer: b.music.ctrl})

	b.initialized = true
	return nil
}

func (b *beepBackend) Quit() {
	if !b.initialized {
		return
	}
	// Stop the speaker's playback goroutine before mutating the streamers
	// it reads; otherwise halt/Streamer writes race the mixer callback.
	speaker.Close()
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

// ---------------------------------------------------------------------------
// decode helpers
// ---------------------------------------------------------------------------

// decodeReader detects the audio format from file extension and decodes.
func decodeReader(ext string, rc interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
	Close() error
}) (beep.StreamSeekCloser, beep.Format, error) {
	switch ext {
	case ".wav":
		stream, format, err := wav.Decode(rc)
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: wav: %w", err)
		}
		return stream, format, nil
	case ".mp3":
		stream, format, err := mp3.Decode(rc.(io.ReadCloser))
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: mp3: %w", err)
		}
		return stream, format, nil
	case ".ogg":
		stream, format, err := vorbis.Decode(rc.(io.ReadCloser))
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: ogg: %w", err)
		}
		return stream, format, nil
	case ".flac":
		stream, format, err := flac.Decode(rc.(io.ReadCloser))
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: flac: %w", err)
		}
		return stream, format, nil
	default:
		return nil, beep.Format{}, fmt.Errorf("audio: unsupported audio format %q", ext)
	}
}

// decodeBytes decodes in-memory audio data.  Detects format by magic
// bytes and delegates to the appropriate decoder.
func decodeBytes(data []byte) (beep.StreamSeekCloser, beep.Format, error) {
	if len(data) < 4 {
		return nil, beep.Format{}, fmt.Errorf(
			"audio: data too short (%d bytes)", len(data))
	}

	switch {
	case string(data[:4]) == "RIFF":
		return wav.Decode(newReadSeekCloser(data))
	case data[0] == 0xFF && len(data) > 1 && data[1]&0xE0 == 0xE0:
		return mp3.Decode(newReadCloser(data))
	case string(data[:4]) == "OggS":
		return vorbis.Decode(newReadCloser(data))
	case string(data[:4]) == "fLaC":
		return flac.Decode(newReadCloser(data))
	default:
		return nil, beep.Format{}, fmt.Errorf(
			"audio: unrecognized audio format (magic: % x)", data[:min(4, len(data))])
	}
}

// readSeekCloser wraps a [bytes.Reader] into [io.ReadSeekCloser].
type readSeekCloser struct {
	r      *bytes.Reader
	closed bool
}

func newReadSeekCloser(data []byte) *readSeekCloser {
	return &readSeekCloser{r: bytes.NewReader(data)}
}

// readCloser wraps a [bytes.Reader] into [io.ReadCloser].
type readCloser struct {
	r      *bytes.Reader
	closed bool
}

func newReadCloser(data []byte) *readCloser {
	return &readCloser{r: bytes.NewReader(data)}
}

func (r *readCloser) Read(p []byte) (int, error) {
	if r.closed {
		return 0, errors.New("audio: reader closed")
	}
	return r.r.Read(p)
}

func (r *readCloser) Close() error {
	r.closed = true
	return nil
}

func (r *readSeekCloser) Read(p []byte) (int, error) {
	if r.closed {
		return 0, errors.New("audio: reader closed")
	}
	return r.r.Read(p)
}

func (r *readSeekCloser) Seek(offset int64, whence int) (int64, error) {
	if r.closed {
		return 0, errors.New("audio: reader closed")
	}
	return r.r.Seek(offset, whence)
}

func (r *readSeekCloser) Close() error {
	r.closed = true
	return nil
}
