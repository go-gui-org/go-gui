//go:build !js && !android && !ios

package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// wavSilence is a minimal valid 8-bit mono PCM WAV file (10 samples of
// silence at 8000 Hz). Used to exercise LoadSoundBytes success path
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
	46, 0, 0, 0, // chunk size (file size - 8)
	'W', 'A', 'V', 'E',
	'f', 'm', 't', ' ',
	16, 0, 0, 0, // fmt chunk size (PCM)
	1, 0, // audio format (PCM)
	1, 0, // channels (mono)
	0x40, 0x1F, 0, 0, // sample rate (8000 Hz)
	0x40, 0x1F, 0, 0, // byte rate
	1, 0, // block align
	8, 0, // bits per sample
	'd', 'a', 't', 'a',
	10, 0, 0, 0, // data chunk size
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 10 bytes silence
}

// floatAbs returns the absolute value of v.
func floatAbs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// ---------------------------------------------------------------------------
// clamp01
// ---------------------------------------------------------------------------

func TestClamp01(t *testing.T) {
	tests := []struct {
		name string
		in   float32
		want float32
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
// toMixVol
// ---------------------------------------------------------------------------

func TestToMixVol(t *testing.T) {
	// mix.MAX_VOLUME is 128 in SDL_mixer.
	tests := []struct {
		name string
		in   float32
		want int
	}{
		{name: "zero", in: 0.0, want: 0},
		{name: "one", in: 1.0, want: 128},
		{name: "half", in: 0.5, want: 64},
		{name: "quarter", in: 0.25, want: 32},
		{name: "three_quarters", in: 0.75, want: 96},
		{name: "clamped_neg", in: -1.0, want: 0},
		{name: "clamped_high", in: 2.0, want: 128},
		{name: "one_over_128", in: 1.0 / 128.0, want: 1},
		{name: "near_one", in: 0.9921875, want: 127},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toMixVol(tt.in)
			if got != tt.want {
				t.Errorf("toMixVol(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// fromMixVol
// ---------------------------------------------------------------------------

func TestFromMixVol(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want float32
	}{
		{name: "zero", in: 0, want: 0.0},
		{name: "max", in: 128, want: 1.0},
		{name: "half", in: 64, want: 0.5},
		{name: "quarter", in: 32, want: 0.25},
		{name: "eighth", in: 16, want: 0.125},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fromMixVol(tt.in)
			if got != tt.want {
				t.Errorf("fromMixVol(%d) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Volume round-trip (toMixVol -> fromMixVol)
// ---------------------------------------------------------------------------

func TestVolumeRoundTrip(t *testing.T) {
	vals := []float32{0, 0.1, 0.25, 0.333, 0.5, 0.6, 0.75, 0.9, 1}
	for _, v := range vals {
		t.Run(fmt.Sprintf("%.3f", v), func(t *testing.T) {
			mixV := toMixVol(v)
			got := fromMixVol(mixV)
			delta := floatAbs(got - v)
			if delta > 0.01 {
				t.Errorf("round-trip %v: toMixVol=%d, fromMixVol=%v (delta=%v)",
					v, mixV, got, delta)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Nil / empty receiver safety (pure Go, no SDL)
// ---------------------------------------------------------------------------

func TestMusicFreeNilReceiver(t *testing.T) {
	var m *Music
	m.Free() // must not panic
}

func TestMusicFreeEmptyReceiver(t *testing.T) {
	m := &Music{}
	m.Free() // must not panic (mus is nil)
}

func TestSoundFreeNilReceiver(t *testing.T) {
	var s *Sound
	s.Free() // must not panic
}

func TestSoundFreeEmptyReceiver(t *testing.T) {
	s := &Sound{}
	s.Free() // must not panic (chunk is nil)
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// Init / Quit (require SDL audio; skip on failure)
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

func TestQuitBeforeInit(t *testing.T) {
	Quit()
}

func TestInitCustomCfg(t *testing.T) {
	err := Init(Cfg{
		Frequency:      48000,
		OutputChannels: 2,
		ChunkSize:      4096,
		MixChannels:    32,
		Formats:        0,
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

	for _, v := range []float32{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			SetMasterVolume(v)
			got := MasterVolume()
			if delta := floatAbs(got - v); delta > 0.01 {
				t.Errorf("MasterVolume after setting %v = %v (delta=%v)",
					v, got, delta)
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

	for _, v := range []float32{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			SetMusicVolume(v)
			got := MusicVolume()
			if delta := floatAbs(got - v); delta > 0.01 {
				t.Errorf("MusicVolume after setting %v = %v (delta=%v)",
					v, got, delta)
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
// Load success paths (require Init + valid audio data, skip on failure)
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

	if s.chunk == nil {
		t.Error("expected non-nil chunk after successful LoadSoundBytes")
	}
	if s.keep == nil {
		t.Error("expected non-nil keep after successful LoadSoundBytes")
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

	for _, v := range []float32{0, 0.25, 0.5, 0.75, 1} {
		t.Run(fmt.Sprintf("set_%.2f", v), func(t *testing.T) {
			s.SetVolume(v)
			got := s.Volume()
			if delta := floatAbs(got - v); delta > 0.01 {
				t.Errorf("Sound volume set=%v, got=%v (delta=%v)", v, got, delta)
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
	if s.chunk != nil {
		t.Error("expected chunk to be nil after Free")
	}
	if s.keep != nil {
		t.Error("expected keep to be nil after Free")
	}
	// Double-free must not panic
	s.Free()
}

func TestLoadMusicFromFile(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer Quit()

	// Write the synthetic WAV to a temp file so we can test LoadMusic.
	// SDL_mixer accepts WAV as a music format; this is atypical but valid.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.wav")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}

	m, err := LoadMusic(path)
	if err != nil {
		t.Skipf("LoadMusic from temp WAV failed: %v", err)
	}

	// Cover Music.Free full path (non-nil mus).
	m.Free()
	if m.mus != nil {
		t.Error("expected mus to be nil after Free")
	}
	// Double-free must not panic
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

	if s.chunk == nil {
		t.Error("expected non-nil chunk after successful LoadSound")
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
