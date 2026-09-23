//go:build !js && !android && !ios

package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitInvalidOutputChannels pins the new OutputChannels validation.
// 0 is the "use default" sentinel; 3 must fail before any output sink.
func TestInitInvalidOutputChannels(t *testing.T) {
	if initialized {
		quit()
	}
	err := Init(Cfg{OutputChannels: 3})
	if err == nil {
		quit()
		t.Fatal("expected error for output channels 3")
	}
	if !strings.Contains(err.Error(), "output channels") {
		t.Errorf("expected output channels validation error, got %v", err)
	}
}

// TestLoadSoundRejectsHugeFileFast uses a sparse file so Stat sees
// >soundMaxBytes without allocating. LoadSound must reject via the
// stat fast path, not after reading.
func TestLoadSoundRejectsHugeFileFast(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "huge.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Skipf("cannot create temp file: %v", err)
	}
	if err := f.Truncate(soundMaxBytes + 1); err != nil {
		_ = f.Close()
		t.Skipf("cannot truncate sparse file: %v", err)
	}
	_ = f.Close()

	if _, err := loadSound(path); err == nil {
		t.Fatal("expected too-large error for sparse huge file")
	} else if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected too-large error, got %v", err)
	}
}

// TestFadeStreamerEarlyDrainCallsOnComplete is the regression: a music
// fade-out whose inner drained mid-fade returned without onComplete,
// leaking the rewind handle.
func TestFadeStreamerEarlyDrainCallsOnComplete(t *testing.T) {
	inner := &countingStreamer{left: 0}
	called := false
	f := &fadeStreamer{
		streamer:   inner,
		sampleRate: 44100,
		startVol:   1,
		targetVol:  0,
		endSamples: 100,
		onComplete: func() { called = true },
	}
	var buf [16][2]float64
	if n, ok := f.Stream(buf[:]); n != 0 || ok {
		t.Errorf("early drain Stream = (%d, %v), want (0, false)", n, ok)
	}
	if !called {
		t.Error("early drain did not call onComplete")
	}
	// Second call must not call again.
	if _, _ = f.Stream(buf[:]); !called {
		t.Error("onComplete flag lost")
	}
}

// TestFadeOutMusicIdempotent pins that a second FadeOutMusic while a
// fade-out is in progress does not re-wrap and restart the ramp.
func TestFadeOutMusicIdempotent(t *testing.T) {
	_, _ = swapReadyBackend(t)

	m, err := LoadMusicBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadMusicBytes: %v", err)
	}
	defer m.Free()
	if err := m.Play(0); err != nil {
		t.Fatalf("Play: %v", err)
	}
	FadeOutMusic(100)
	bb, ok := backend.(*beepBackend)
	if !ok {
		t.Fatalf("backend is %T, want *beepBackend", backend)
	}
	first := bb.music.ctrl.Streamer
	FadeOutMusic(100)
	if second := bb.music.ctrl.Streamer; second != first {
		t.Error("second FadeOutMusic re-wrapped an in-progress fade-out")
	}
	HaltMusic()
}

// TestIsPlayingPausedFalse pins the IsPlaying/IsMusicPlaying alignment:
// a paused channel is occupied but not playing.
func TestIsPlayingPausedFalse(t *testing.T) {
	_, _ = swapReadyBackend(t)

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadSoundBytes: %v", err)
	}
	defer s.Free()
	ch, err := s.Play(0, 0)
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if !IsPlaying(ch) {
		t.Fatal("expected IsPlaying true after Play")
	}
	pauseChannel(ch)
	if IsPlaying(ch) {
		t.Error("IsPlaying true while paused, want false")
	}
	resumeChannel(ch)
	if !IsPlaying(ch) {
		t.Error("IsPlaying false after resume, want true")
	}
	HaltChannel(ch)
}

// TestSoundPlayOutOfRangeMessage pins the [0, N) error for an explicit
// channel, matching PlaySource.
func TestSoundPlayOutOfRangeMessage(t *testing.T) {
	_, _ = swapReadyBackend(t)

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadSoundBytes: %v", err)
	}
	defer s.Free()
	bb, ok := backend.(*beepBackend)
	if !ok {
		t.Fatalf("backend is %T, want *beepBackend", backend)
	}
	n := bb.channels.numChannels()
	if _, err := s.Play(n, 0); err == nil {
		t.Fatalf("Play(%d) = nil error, want out-of-range", n)
	} else {
		want := fmt.Sprintf("[0, %d)", n)
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Play(%d) error = %q, want it to contain %q", n, err, want)
		}
	}
	if _, err := s.FadeIn(n, 0, 10); err == nil {
		t.Fatalf("FadeIn(%d) = nil error, want out-of-range", n)
	} else {
		want := fmt.Sprintf("[0, %d)", n)
		if !strings.Contains(err.Error(), want) {
			t.Errorf("FadeIn(%d) error = %q, want it to contain %q", n, err, want)
		}
	}
}

// TestDecodeReaderCaseInsensitive pins that ".WAV" loads like ".wav".
// LoadMusic needs no Init; it only opens and decodes.
func TestDecodeReaderCaseInsensitive(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "silence.WAV")
	if err := os.WriteFile(path, wavSilence, 0644); err != nil {
		t.Skipf("cannot write temp WAV: %v", err)
	}
	m, err := LoadMusic(path)
	if err != nil {
		t.Fatalf("LoadMusic uppercase ext failed: %v", err)
	}
	m.Free()
}
