//go:build !js && !android && !ios

package audio

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
)

// TestMain skips the whole package under the race detector. Many tests
// call Init(), which on macOS reaches ebitengine/oto's CoreAudio context
// creation — an upstream data race in newContext (driver_darwin.go) that
// trips -race regardless of anything in this package. The audio path can
// only be exercised with a live speaker, so there is nothing here worth
// race-checking once that init is excluded.
func TestMain(m *testing.M) {
	if raceEnabled {
		fmt.Println("gui/audio: skipping under -race (upstream oto init race)")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// wavSilence is a minimal valid 8-bit mono PCM WAV file (10 samples of
// silence at 8000 Hz).  Used to exercise LoadSoundBytes success path
// without requiring a fixture file on disk.
//
// RIFF structure:
//
//	Offset  Size  Field
//	  0      4    "RIFF"
//	  4      4    file size - 8 (46 = 54 - 8)
//	  8      4    "WAVE"
//	 12      4    "fmt "
//	 16      4    fmt chunk size (16 for PCM)
//	 20      2    audio format (1 = PCM)
//	 22      2    channels (1 = mono)
//	 24      4    sample rate (8000)
//	 28      4    byte rate (8000)
//	 32      2    block align (1)
//	 34      2    bits per sample (8)
//	 36      4    "data"
//	 40      4    data size (10)
//	 44     10    10 bytes silence (0x00)
var wavSilence = []byte{
	'R', 'I', 'F', 'F',
	46, 0, 0, 0,
	'W', 'A', 'V', 'E',
	'f', 'm', 't', ' ',
	16, 0, 0, 0,
	1, 0,
	1, 0,
	0x40, 0x1F, 0, 0,
	0x40, 0x1F, 0, 0,
	1, 0,
	8, 0,
	'd', 'a', 't', 'a',
	10, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

// ---------------------------------------------------------------------------
// clamp01
// ---------------------------------------------------------------------------

func TestClamp01(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "zero", in: 0.0, want: 0.0},
		{name: "half", in: 0.5, want: 0.5},
		{name: "one", in: 1.0, want: 1.0},
		{name: "negative", in: -0.1, want: 0.0},
		{name: "above_one", in: 1.5, want: 1.0},
		{name: "large", in: 2.0, want: 1.0},
		{name: "small_neg", in: -1e-6, want: 0.0},
		{name: "just_above_one", in: 1 + 1e-6, want: 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp01(tt.in)
			if got != tt.want {
				t.Errorf("clamp01(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Volume scale helpers (0–1 native float64)
// ---------------------------------------------------------------------------

func TestVolumeClamp(t *testing.T) {
	if v := clamp01(-0.5); v != 0 {
		t.Errorf("clamp01(-0.5) = %v, want 0", v)
	}
	if v := clamp01(1.5); v != 1 {
		t.Errorf("clamp01(1.5) = %v, want 1", v)
	}
}

func TestVolumeRoundTrip(t *testing.T) {
	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("%.2f", v), func(t *testing.T) {
			c := clamp01(v)
			if c != v {
				t.Errorf("volume round-trip %v: got %v", v, c)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Nil / empty receiver safety (pure Go, no audio subsystem)
// ---------------------------------------------------------------------------

func TestMusicFreeNilReceiver(t *testing.T) {
	var m *Music
	m.Free()
}

func TestMusicFreeEmptyReceiver(t *testing.T) {
	m := &Music{}
	m.Free()
}

func TestSoundFreeNilReceiver(t *testing.T) {
	var s *Sound
	s.Free()
}

func TestSoundFreeEmptyReceiver(t *testing.T) {
	s := &Sound{}
	s.Free()
}

func TestSoundSetVolumeNilReceiver(t *testing.T) {
	var s *Sound
	s.setVolume(0.5)
}

func TestSoundVolumeNilReceiver(t *testing.T) {
	var s *Sound
	if v := s.Volume(); v != 0 {
		t.Errorf("Sound.Volume() on nil = %v, want 0", v)
	}
}

func TestQuitBeforeInit(t *testing.T) {
	quit()
}

// ---------------------------------------------------------------------------
// Init / Quit (require audio subsystem; skip on failure)
// ---------------------------------------------------------------------------

func TestInitQuit(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable (expected in headless): %v", err)
	}
	quit()
	if initialized {
		t.Error("expected initialized=false after Quit")
	}
}

func TestInitIdempotent(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	if err := Init(); err != nil {
		t.Errorf("second Init returned error: %v", err)
	}
}

func TestInitCustomCfg(t *testing.T) {
	err := Init(Cfg{
		frequency:   48000,
		chunkSize:   4096,
		mixChannels: 32,
	})
	if err != nil {
		t.Skipf("audio init with custom cfg unavailable: %v", err)
	}
	quit()
}

func TestMultipleQuit(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	quit()
	quit()
	if initialized {
		t.Error("expected initialized=false after second Quit")
	}
}

// ---------------------------------------------------------------------------
// Volume after Init
// ---------------------------------------------------------------------------

func TestMasterVolumeRoundTrip(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	orig := MasterVolume()
	defer SetMasterVolume(orig)

	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			SetMasterVolume(v)
			got := MasterVolume()
			if math.Abs(got-v) > 0.01 {
				t.Errorf("MasterVolume after setting %v = %v", v, got)
			}
		})
	}
}

func TestMusicVolumeRoundTrip(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	orig := musicVolume()
	defer setMusicVolume(orig)

	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			setMusicVolume(v)
			got := musicVolume()
			if math.Abs(got-v) > 0.01 {
				t.Errorf("MusicVolume after setting %v = %v", v, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Load error paths (require Init but not actual playback)
// ---------------------------------------------------------------------------

func TestLoadMusicErrors(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	_, err = LoadMusic("")
	if err == nil {
		t.Error("expected error for empty path")
	}

	_, err = LoadMusic("/nonexistent/file.ogg")
	if err == nil {
		t.Error("expected error for nonexistent file path")
	}
}

func TestLoadSoundErrors(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	_, err = loadSound("")
	if err == nil {
		t.Error("expected error for empty path")
	}

	_, err = loadSound("/nonexistent/file.wav")
	if err == nil {
		t.Error("expected error for nonexistent file path")
	}
}

func TestLoadSoundBytesErrors(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	_, err = LoadSoundBytes(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}

	_, err = LoadSoundBytes([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}

	_, err = LoadSoundBytes([]byte("not valid audio data"))
	if err == nil {
		t.Error("expected error for invalid audio data")
	}
}

// ---------------------------------------------------------------------------
// Load success paths (require Init + valid audio data)
// ---------------------------------------------------------------------------

func TestLoadSoundBytesSuccess(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes with synthetic WAV failed: %v", err)
	}
	defer s.Free()

	if s.buffer == nil {
		t.Error("expected non-nil buffer after successful LoadSoundBytes")
	}
}

func TestSoundVolumeRoundTrip(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes with synthetic WAV failed: %v", err)
	}
	defer s.Free()

	orig := s.Volume()
	defer s.setVolume(orig)

	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			s.setVolume(v)
			got := s.Volume()
			if math.Abs(got-v) > 0.01 {
				t.Errorf("Sound volume set=%v, got=%v", v, got)
			}
		})
	}
}

func TestSoundFreeLoaded(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes with synthetic WAV failed: %v", err)
	}

	s.Free()
	if s.buffer != nil {
		t.Error("expected buffer to be nil after Free")
	}
	s.Free()
}

func TestLoadMusicFromFile(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic from temp WAV failed: %v", err)
	}

	m.Free()
	if m.beepStream != nil {
		t.Error("expected beepStream to be nil after Free")
	}
	m.Free()
}

func TestLoadSoundFromFile(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	s, err := loadSound(path)
	if err != nil {
		t.Skipf("LoadSound from temp WAV failed: %v", err)
	}
	defer s.Free()

	if s.buffer == nil {
		t.Error("expected non-nil buffer after successful LoadSound")
	}
}

// ---------------------------------------------------------------------------
// Global controls (no-panic smoke tests; require Init)
// ---------------------------------------------------------------------------

func TestMusicGlobalControls(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	HaltMusic()
	FadeOutMusic(0)
	pauseMusic()
	resumeMusic()
	rewindMusic()
	_ = isMusicPlaying()
	_ = isMusicPaused()
}

func TestSoundChannelControls(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	HaltChannel(-1)
	fadeOutChannel(-1, 0)
	pauseChannel(-1)
	resumeChannel(-1)
	_ = IsPlaying(-1)
}

// ---------------------------------------------------------------------------
// Playback smoke tests
// ---------------------------------------------------------------------------

func TestSoundPlayOnce(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes failed: %v", err)
	}
	defer s.Free()

	ch, err := s.PlayOnce()
	if err != nil {
		t.Errorf("PlayOnce returned error: %v", err)
	}
	if ch < 0 {
		t.Error("expected non-negative channel from PlayOnce")
	}
}

func TestSoundFadeIn(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes failed: %v", err)
	}
	defer s.Free()

	ch, err := s.FadeIn(-1, 0, 100)
	if err != nil {
		t.Errorf("FadeIn returned error: %v", err)
	}
	if ch < 0 {
		t.Error("expected non-negative channel from FadeIn")
	}
}

// ---------------------------------------------------------------------------
// Live sources
// ---------------------------------------------------------------------------

// testSource fills with silence and never self-ends, so a channel it is
// playing on stays playing until halted.
type testSource struct{}

func (testSource) Fill(samples [][2]float64) (int, bool) {
	clear(samples)
	return len(samples), true
}

func TestPlaySourceSmoke(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	if err := PlaySource(0, &testSource{}); err != nil {
		t.Fatalf("PlaySource returned error: %v", err)
	}
	if !IsPlaying(0) {
		t.Error("expected channel 0 playing after PlaySource")
	}
	HaltChannel(0)
	if IsPlaying(0) {
		t.Error("expected channel 0 halted after HaltChannel")
	}
}

func TestPlaySourceErrors(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	if err := PlaySource(0, nil); err == nil {
		t.Error("PlaySource(nil source) = nil error, want error")
	}
	if err := PlaySource(999, &testSource{}); err == nil {
		t.Error("PlaySource(out-of-range channel) = nil error, want error")
	} else if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("PlaySource(999) error = %q, want out-of-range message", err)
	}
	if err := PlaySource(-1, &testSource{}); err != nil {
		t.Errorf("PlaySource(-1) auto-select returned error: %v", err)
	}
}

// endingSource plays one buffer of silence, then returns ok = false to
// end itself — the channelMixer reaper path (streamers.go) must see the
// (0, false) return, free the channel, and make it reusable.
type endingSource struct{ done bool }

func (s *endingSource) Fill(samples [][2]float64) (int, bool) {
	if s.done {
		return 0, false
	}
	clear(samples)
	s.done = true
	return len(samples), true
}

func TestPlaySourceSourceEnd(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	if err := PlaySource(0, &endingSource{}); err != nil {
		t.Fatalf("PlaySource returned error: %v", err)
	}
	// The audio thread reaps the ended source asynchronously, so poll.
	deadline := time.Now().Add(2 * time.Second)
	for IsPlaying(0) {
		if time.Now().After(deadline) {
			t.Fatal("channel 0 still playing after source ended")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The freed channel is immediately reusable.
	if err := PlaySource(0, &testSource{}); err != nil {
		t.Fatalf("PlaySource after source end returned error: %v", err)
	}
	HaltChannel(0)
}

func TestSampleRateDefault(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	if got := SampleRate(); got != 44100 {
		t.Errorf("SampleRate() = %d, want default 44100", got)
	}
}

func TestMusicPlay(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic failed: %v", err)
	}
	defer m.Free()

	if err := m.Play(0); err != nil {
		t.Errorf("Play returned error: %v", err)
	}
}

func TestMusicFadeOut(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic failed: %v", err)
	}
	defer m.Free()

	if err := m.Play(0); err != nil {
		t.Skipf("Play failed: %v", err)
	}

	FadeOutMusic(50)
}

// ---------------------------------------------------------------------------
// Nil receiver safety for playback methods
// ---------------------------------------------------------------------------

func TestMusicPlayNilReceiver(t *testing.T) {
	var m *Music
	if err := m.Play(0); err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestMusicFadeInNilReceiver(t *testing.T) {
	var m *Music
	if err := m.FadeIn(0, 100); err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestSoundPlayNilReceiver(t *testing.T) {
	var s *Sound
	if _, err := s.Play(-1, 0); err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestSoundFadeInNilReceiver(t *testing.T) {
	var s *Sound
	if _, err := s.FadeIn(-1, 0, 100); err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestSoundPlayOnceNilReceiver(t *testing.T) {
	var s *Sound
	if _, err := s.PlayOnce(); err == nil {
		t.Error("expected error for nil receiver")
	}
}

// ---------------------------------------------------------------------------
// Init with invalid Cfg
// ---------------------------------------------------------------------------

func TestInitInvalidFreq(t *testing.T) {
	_ = quit // ensure not initialized
	if initialized {
		quit()
	}
	err := Init(Cfg{frequency: 100})
	if err == nil {
		quit()
		t.Error("expected error for frequency 100 Hz")
	}
	err = Init(Cfg{frequency: 300000})
	if err == nil {
		quit()
		t.Error("expected error for frequency 300000 Hz")
	}
}

func TestInitInvalidChunkSize(t *testing.T) {
	if initialized {
		quit()
	}
	err := Init(Cfg{chunkSize: 8})
	if err == nil {
		quit()
		t.Error("expected error for chunk size 8")
	}
}

func TestInitInvalidMixChannels(t *testing.T) {
	if initialized {
		quit()
	}
	// 0 is the "use default" sentinel (like Frequency/ChunkSize); use an
	// out-of-range non-zero value to exercise the validation bound.
	err := Init(Cfg{mixChannels: 300})
	if err == nil {
		quit()
		t.Fatal("expected error for mix channels 300")
	}
	// Assert on the message: without a sound server Init also fails at
	// the output sink, which would pass this test for the wrong reason.
	if !strings.Contains(err.Error(), "mix channels") {
		t.Errorf("expected mix channels validation error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Fade with ms=0 (division-by-zero guard)
// ---------------------------------------------------------------------------

func TestSoundFadeInZeroMs(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes failed: %v", err)
	}
	defer s.Free()

	ch, err := s.FadeIn(-1, 0, 0)
	if err != nil {
		t.Errorf("FadeIn with ms=0 returned error: %v", err)
	}
	if ch < 0 {
		t.Error("expected non-negative channel from FadeIn")
	}
}

func TestFadeOutMusicZeroMs(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic failed: %v", err)
	}
	defer m.Free()

	if err := m.Play(0); err != nil {
		t.Skipf("Play failed: %v", err)
	}

	FadeOutMusic(0)
}

// ---------------------------------------------------------------------------
// Music.FadeIn
// ---------------------------------------------------------------------------

func TestMusicFadeIn(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic failed: %v", err)
	}
	defer m.Free()

	if err := m.FadeIn(0, 100); err != nil {
		t.Errorf("FadeIn returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Sound.Play with loops
// ---------------------------------------------------------------------------

func TestSoundPlayLoopForever(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes failed: %v", err)
	}
	defer s.Free()

	ch, err := s.Play(-1, -1)
	if err != nil {
		t.Errorf("Play with loops=-1 returned error: %v", err)
	}
	if ch < 0 {
		t.Error("expected non-negative channel")
	}
}

func TestSoundPlayLoopOnce(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes failed: %v", err)
	}
	defer s.Free()

	ch, err := s.Play(-1, 1)
	if err != nil {
		t.Errorf("Play with loops=1 returned error: %v", err)
	}
	if ch < 0 {
		t.Error("expected non-negative channel")
	}
}

// ---------------------------------------------------------------------------
// Music.Play with loops=-1 (forever)
// ---------------------------------------------------------------------------

func TestMusicPlayLoopForever(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic failed: %v", err)
	}
	defer m.Free()

	if err := m.Play(-1); err != nil {
		t.Errorf("Play with loops=-1 returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// resampling
// ---------------------------------------------------------------------------

// countingStreamer yields n frames of silence, then drains.  Used to
// measure how many output frames a resampler produces for a known input
// length, which is the pitch/speed bug expressed as a count.
type countingStreamer struct {
	left int
}

func (c *countingStreamer) Stream(samples [][2]float64) (int, bool) {
	if c.left <= 0 {
		return 0, false
	}
	n := min(len(samples), c.left)
	clear(samples[:n])
	c.left -= n
	return n, true
}

func (c *countingStreamer) Err() error { return nil }

// TestResampleToIdentity gates the no-alloc claim: matching or unknown
// rates must hand back the very same streamer, not a wrapper.
func TestResampleToIdentity(t *testing.T) {
	src := &countingStreamer{left: 10}
	tests := []struct {
		name     string
		from, to beep.SampleRate
	}{
		{name: "equal", from: 44100, to: 44100},
		{name: "zero source", from: 0, to: 44100},
		{name: "zero dest", from: 44100, to: 0},
		{name: "both zero", from: 0, to: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resampleTo(tt.from, tt.to, src); got != beep.Streamer(src) {
				t.Errorf("resampleTo(%d, %d, s) wrapped the streamer, want identity",
					tt.from, tt.to)
			}
		})
	}
}

// TestResampleToStretches is the headless form of the bug report: a
// stream decoded at 8000 Hz must produce 44100/8000 as many frames when
// converted, or it plays that many times too fast.
func TestResampleToStretches(t *testing.T) {
	const (
		inFrames = 8000
		from     = beep.SampleRate(8000)
		to       = beep.SampleRate(44100)
	)
	s := resampleTo(from, to, &countingStreamer{left: inFrames})
	if _, isSame := s.(*countingStreamer); isSame {
		t.Fatal("resampleTo returned the source streamer for mismatched rates")
	}

	var (
		buf   [512][2]float64
		total int
	)
	for {
		n, ok := s.Stream(buf[:])
		total += n
		if !ok {
			break
		}
	}

	want := inFrames * int(to) / int(from)
	// beep's interpolator can be a window short at the tail; a couple of
	// frames of slack keeps the test about rate, not about edge handling.
	if diff := total - want; diff < -8 || diff > 8 {
		t.Errorf("streamed %d frames, want ~%d (diff %d)", total, want, diff)
	}
}

// TestLoadSoundBytesResamples checks the load-time conversion: the
// 8000 Hz wavSilence fixture must land in the buffer at the output rate.
func TestLoadSoundBytesResamples(t *testing.T) {
	if err := Init(); err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	snd, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadSoundBytes returned error: %v", err)
	}
	defer snd.Free()

	if got, want := int(snd.format.SampleRate), SampleRate(); got != want {
		t.Errorf("buffer sample rate = %d, want %d", got, want)
	}
	if snd.format.Precision < 2 {
		t.Errorf("buffer precision = %d, want >= 2 after resampling",
			snd.format.Precision)
	}
	// wavSilence holds 10 frames at 8000 Hz.
	want := 10 * SampleRate() / 8000
	if diff := snd.buffer.Len() - want; diff < -8 || diff > 8 {
		t.Errorf("buffer length = %d frames, want ~%d", snd.buffer.Len(), want)
	}
}

// TestSoundPlayResampled plays a resampled sound on a loop, which is the
// wrapping order most at risk: Loop2 needs the seeker underneath.
func TestSoundPlayResampled(t *testing.T) {
	if err := Init(); err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	snd, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadSoundBytes returned error: %v", err)
	}
	defer snd.Free()

	ch, err := snd.Play(-1, -1)
	if err != nil {
		t.Fatalf("Play returned error: %v", err)
	}
	if ch < 0 {
		t.Fatalf("Play returned channel %d, want >= 0", ch)
	}
	HaltChannel(ch)
}

// TestMusicPlayResampled exercises the play-time conversion path, which
// music takes because its decoder stream must stay seekable.
func TestMusicPlayResampled(t *testing.T) {
	if err := Init(); err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	path := filepath.Join(t.TempDir(), "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}
	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic failed: %v", err)
	}
	defer m.Free()

	if int(m.format.SampleRate) == SampleRate() {
		t.Fatalf("fixture rate %d matches output rate; test proves nothing",
			m.format.SampleRate)
	}
	if err := m.Play(-1); err != nil {
		t.Fatalf("Play returned error: %v", err)
	}
	if !isMusicPlaying() {
		t.Error("music not playing after Play")
	}
	HaltMusic()
}

// ---------------------------------------------------------------------------
// rewind
// ---------------------------------------------------------------------------

// seekSpy is a StreamSeeker that records Seek calls and the position it
// streams from, so a test can tell a real rewind from a silent no-op.
type seekSpy struct {
	pos   int
	total int
	seeks int
}

func (s *seekSpy) Stream(samples [][2]float64) (int, bool) {
	if s.pos >= s.total {
		return 0, false
	}
	n := min(len(samples), s.total-s.pos)
	clear(samples[:n])
	s.pos += n
	return n, true
}

func (s *seekSpy) Err() error    { return nil }
func (s *seekSpy) Len() int      { return s.total }
func (s *seekSpy) Position() int { return s.pos }
func (s *seekSpy) Seek(p int) error {
	s.seeks++
	s.pos = p
	return nil
}

// TestRewindStreamerDefersSeek pins the two properties the wrapper
// exists for: nothing is seeked until the audio thread streams again,
// and one request produces exactly one seek.
func TestRewindStreamerDefersSeek(t *testing.T) {
	spy := &seekSpy{total: 100}
	rw := &rewindStreamer{StreamSeeker: spy}

	var buf [10][2]float64
	rw.Stream(buf[:])
	if spy.pos != 10 {
		t.Fatalf("position after one Stream = %d, want 10", spy.pos)
	}

	rw.request()
	if spy.seeks != 0 {
		t.Errorf("request() seeked immediately (%d seeks); the seek must "+
			"wait for the audio thread", spy.seeks)
	}

	rw.Stream(buf[:])
	if spy.seeks != 1 {
		t.Errorf("seeks after Stream = %d, want 1", spy.seeks)
	}
	if spy.pos != 10 {
		t.Errorf("position after rewind + Stream = %d, want 10 "+
			"(rewound to 0, then streamed 10)", spy.pos)
	}

	// The request is one-shot: a later Stream must not seek again.
	rw.Stream(buf[:])
	if spy.seeks != 1 {
		t.Errorf("seeks after extra Stream = %d, want 1 (request is "+
			"one-shot)", spy.seeks)
	}
}

// TestRewindStreamerIsSeekable gates the embedding: beep.Loop2 rejects a
// source that is not a StreamSeeker, so losing that would silently drop
// looping back to play-once.
func TestRewindStreamerIsSeekable(t *testing.T) {
	rw := &rewindStreamer{StreamSeeker: &seekSpy{total: 100}}
	if _, ok := beep.Streamer(rw).(beep.StreamSeeker); !ok {
		t.Fatal("rewindStreamer is not a beep.StreamSeeker")
	}
	if _, err := beep.Loop2(rw); err != nil {
		t.Errorf("beep.Loop2 rejected rewindStreamer: %v", err)
	}
}

// TestRewindMusicReachesDecoder is the regression: RewindMusic used to
// type-assert ctrl.Streamer to a beep.StreamSeeker, which never held
// because the chain's outermost wrapper is a volumeStreamer.
func TestRewindMusicReachesDecoder(t *testing.T) {
	if err := Init(); err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	path := filepath.Join(t.TempDir(), "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}
	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic failed: %v", err)
	}
	defer m.Free()

	if err := m.Play(0); err != nil {
		t.Fatalf("Play returned error: %v", err)
	}
	bb, ok := backend.(*beepBackend)
	if !ok {
		t.Fatalf("backend is %T, want *beepBackend", backend)
	}
	// The old implementation asserted this to a beep.StreamSeeker and
	// gave up when it was not one. Pin that it never is, so the handle
	// below stays the only way in.
	if _, seekable := bb.music.ctrl.Streamer.(beep.StreamSeeker); seekable {
		t.Error("ctrl.Streamer is seekable; the outer chain is expected " +
			"not to be, and the rewind handle exists for that reason")
	}
	if bb.music.rewind.Load() == nil {
		t.Fatal("playing music left no rewind handle; RewindMusic is a no-op")
	}
	rewindMusic()
	if !bb.music.rewind.Load().pending.Load() {
		t.Error("rewindMusic did not mark a pending seek")
	}

	HaltMusic()
	if bb.music.rewind.Load() != nil {
		t.Error("HaltMusic left a stale rewind handle")
	}
}

// TestLoadMusicBytes covers the in-memory loader added so embedded
// tracks can stream instead of being buffered as a Sound.
func TestLoadMusicBytes(t *testing.T) {
	if err := Init(); err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer quit()

	m, err := LoadMusicBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadMusicBytes returned error: %v", err)
	}
	defer m.Free()

	if got := int(m.format.SampleRate); got != 8000 {
		t.Errorf("decoded sample rate = %d, want 8000 (the fixture's own "+
			"rate; music converts at play time)", got)
	}
	if err := m.Play(-1); err != nil {
		t.Fatalf("Play returned error: %v", err)
	}
	if !isMusicPlaying() {
		t.Error("music not playing after Play")
	}
	HaltMusic()
}

// TestLoadMusicBytesErrors covers the rejected inputs.
func TestLoadMusicBytesErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "too short", data: []byte{'R', 'I'}},
		{name: "unrecognized", data: []byte("not audio at all")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := LoadMusicBytes(tt.data); err == nil {
				t.Error("LoadMusicBytes returned nil error, want failure")
			}
		})
	}
}

// TestDecodeBytesID3 exercises the MP3 branch that starts with an
// ID3v2 tag rather than a sync word, which a naive magic check would
// miss.  The fixture is a synthetic ID3 header with valid sync after
// it; the decoder is expected to accept it or fail at decode time, but
// not with "unrecognized audio format".
func TestDecodeBytesID3(t *testing.T) {
	// ID3v2 header: "ID3" + ver(3,0) + flags(0) + size(0,0,0,10)
	// followed by a fake MP3 frame sync 0xFF 0xFB.  If the magic check
	// does not recognise ID3, decodeBytes returns "unrecognized".
	id3 := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 10, 0xFF, 0xFB, 0x90, 0x00}
	if _, _, err := decodeBytes(id3); err != nil &&
		strings.Contains(err.Error(), "unrecognized") {
		t.Errorf("decodeBytes(ID3) = unrecognized, want mp3 branch: %v", err)
	}
}
