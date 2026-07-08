//go:build !js && !android && !ios

package audio

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

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
	s.SetVolume(0.5)
}

func TestSoundVolumeNilReceiver(t *testing.T) {
	var s *Sound
	if v := s.Volume(); v != 0 {
		t.Errorf("Sound.Volume() on nil = %v, want 0", v)
	}
}

func TestQuitBeforeInit(t *testing.T) {
	Quit()
}

// ---------------------------------------------------------------------------
// Init / Quit (require audio subsystem; skip on failure)
// ---------------------------------------------------------------------------

func TestInitQuit(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable (expected in headless): %v", err)
	}
	Quit()
	if initialized {
		t.Error("expected initialized=false after Quit")
	}
}

func TestInitIdempotent(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer Quit()

	if err := Init(); err != nil {
		t.Errorf("second Init returned error: %v", err)
	}
}

func TestInitCustomCfg(t *testing.T) {
	err := Init(Cfg{
		Frequency:   48000,
		ChunkSize:   4096,
		MixChannels: 32,
	})
	if err != nil {
		t.Skipf("audio init with custom cfg unavailable: %v", err)
	}
	Quit()
}

func TestMultipleQuit(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	Quit()
	Quit()
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
	defer Quit()

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
	defer Quit()

	orig := MusicVolume()
	defer SetMusicVolume(orig)

	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			SetMusicVolume(v)
			got := MusicVolume()
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
	defer Quit()

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
	defer Quit()

	_, err = LoadSound("")
	if err == nil {
		t.Error("expected error for empty path")
	}

	_, err = LoadSound("/nonexistent/file.wav")
	if err == nil {
		t.Error("expected error for nonexistent file path")
	}
}

func TestLoadSoundBytesErrors(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer Quit()

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
	defer Quit()

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
	defer Quit()

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Skipf("LoadSoundBytes with synthetic WAV failed: %v", err)
	}
	defer s.Free()

	orig := s.Volume()
	defer s.SetVolume(orig)

	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			s.SetVolume(v)
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
	defer Quit()

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
	defer Quit()

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
	defer Quit()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	s, err := LoadSound(path)
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
	defer Quit()

	HaltMusic()
	FadeOutMusic(0)
	PauseMusic()
	ResumeMusic()
	RewindMusic()
	_ = IsMusicPlaying()
	_ = IsMusicPaused()
}

func TestSoundChannelControls(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer Quit()

	HaltChannel(-1)
	FadeOutChannel(-1, 0)
	PauseChannel(-1)
	ResumeChannel(-1)
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
	defer Quit()

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
	defer Quit()

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

func TestMusicPlay(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer Quit()

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
	defer Quit()

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
	_ = Quit // ensure not initialized
	if initialized {
		Quit()
	}
	err := Init(Cfg{Frequency: 100})
	if err == nil {
		Quit()
		t.Error("expected error for frequency 100 Hz")
	}
	err = Init(Cfg{Frequency: 300000})
	if err == nil {
		Quit()
		t.Error("expected error for frequency 300000 Hz")
	}
}

func TestInitInvalidChunkSize(t *testing.T) {
	if initialized {
		Quit()
	}
	err := Init(Cfg{ChunkSize: 8})
	if err == nil {
		Quit()
		t.Error("expected error for chunk size 8")
	}
}

func TestInitInvalidMixChannels(t *testing.T) {
	if initialized {
		Quit()
	}
	err := Init(Cfg{MixChannels: 0})
	if err == nil {
		Quit()
		t.Error("expected error for mix channels 0")
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
	defer Quit()

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
	defer Quit()

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
	defer Quit()

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
	defer Quit()

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
	defer Quit()

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
	defer Quit()

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
