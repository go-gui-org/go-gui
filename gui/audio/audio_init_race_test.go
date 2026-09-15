//go:build !js && !android && !ios

package audio

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
)

// Tests named TestRace* are the only ones TestMain runs under -race, so
// every test here that exists to catch a data race carries that prefix.

// countingBackend records how many times the package-level Init and quit
// reach the backend.  The embedded nil Backend makes every other method
// panic, so the test fails loudly if Init or quit start calling more.
type countingBackend struct {
	Backend
	inits atomic.Int32
	quits atomic.Int32
}

func (b *countingBackend) Init(Cfg) error {
	b.inits.Add(1)
	// Hold the window open.  Without serialization, every goroutine that
	// read initialized == false before the first one returned also lands
	// here, and inits climbs past 1.
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (b *countingBackend) Quit() {
	b.quits.Add(1)
	time.Sleep(10 * time.Millisecond) // same window, on the teardown side
}

// swapBackend installs b for the test and restores the real backend and
// the initialized flag after.  It does not touch the output device.
func swapBackend(t *testing.T, b Backend) {
	t.Helper()
	initMu.Lock()
	prevBackend, prevInit := backend, initialized
	backend, initialized = b, false
	initMu.Unlock()
	t.Cleanup(func() {
		initMu.Lock()
		backend, initialized = prevBackend, prevInit
		initMu.Unlock()
	})
}

// swapReadyBackend installs a beepBackend that looks initialized, with its
// mixer built but no output device, and returns the two streamers the
// output would play.  The test plays the audio thread by calling Stream.
func swapReadyBackend(t *testing.T) (sfx, music beep.Streamer) {
	t.Helper()
	b := &beepBackend{}
	sfx, music = b.setup(44100, 512, 4)
	b.initialized = true
	swapBackend(t, b)
	initMu.Lock()
	initialized = true
	initMu.Unlock()
	return sfx, music
}

// TestRaceInitConcurrentOpensBackendOnce is the regression test for the
// unguarded check-then-act in Init: Init promises "call from any
// goroutine, idempotent", so concurrent first calls must open the output
// sink exactly once.
func TestRaceInitConcurrentOpensBackendOnce(t *testing.T) {
	fake := &countingBackend{}
	swapBackend(t, fake)

	const n = 16
	var start, done sync.WaitGroup
	start.Add(1)
	for range n {
		done.Go(func() {
			start.Wait() // release all goroutines together
			if err := Init(); err != nil {
				t.Errorf("Init: %v", err)
			}
		})
	}
	start.Done()
	done.Wait()

	if got := fake.inits.Load(); got != 1 {
		t.Fatalf("backend.Init called %d times, want 1", got)
	}
}

// TestRaceQuitConcurrentClosesBackendOnce covers the same race on the
// quit side: concurrent quits must tear the backend down exactly once.
func TestRaceQuitConcurrentClosesBackendOnce(t *testing.T) {
	fake := &countingBackend{}
	swapBackend(t, fake)
	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	const n = 16
	var start, done sync.WaitGroup
	start.Add(1)
	for range n {
		done.Go(func() {
			start.Wait()
			quit()
		})
	}
	start.Done()
	done.Wait()

	if got := fake.quits.Load(); got != 1 {
		t.Fatalf("backend.Quit called %d times, want 1", got)
	}
}

// TestMusicControlsBeforeInit pins that the music API is inert before
// Init.  The music Ctrl is built by Init, so every control used to
// dereference a nil pointer and panic.
func TestMusicControlsBeforeInit(t *testing.T) {
	swapBackend(t, &beepBackend{})

	m, err := LoadMusicBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadMusicBytes: %v", err)
	}
	if err := m.Play(0); !errors.Is(err, errNotInitialized) {
		t.Errorf("Play before Init = %v, want %v", err, errNotInitialized)
	}
	if err := m.FadeIn(0, 10); !errors.Is(err, errNotInitialized) {
		t.Errorf("FadeIn before Init = %v, want %v", err, errNotInitialized)
	}
	HaltMusic()
	FadeOutMusic(10)
	pauseMusic()
	resumeMusic()
	rewindMusic()
	if isMusicPlaying() || isMusicPaused() {
		t.Error("music reports playing or paused before Init")
	}
	// Free must still close the decoder: a track loaded before Init is
	// legal, and leaking its file handle is not.
	m.Free()
	if m.beepStream != nil {
		t.Error("Free before Init left the decoder open")
	}
}

// TestRaceMusicControlsVsAudioThread drives the volume and music API from
// the test goroutine while a second goroutine plays the role of the
// audio thread and pulls samples from both output streamers.  The
// assertion is the race detector itself, so it only proves anything
// under -race.
func TestRaceMusicControlsVsAudioThread(t *testing.T) {
	sfx, music := swapReadyBackend(t)

	m, err := LoadMusicBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadMusicBytes: %v", err)
	}
	defer m.Free()

	stop := make(chan struct{})
	var audio sync.WaitGroup
	audio.Go(func() {
		var buf [256][2]float64
		for {
			select {
			case <-stop:
				return
			default:
			}
			sfx.Stream(buf[:])
			music.Stream(buf[:])
		}
	})
	// Registered after m.Free's defer, so the audio goroutine stops
	// before the decoder it may be reading is closed.
	defer func() {
		close(stop)
		audio.Wait()
	}()

	for i := range 200 {
		SetMasterVolume(float64(i % 2))
		setMusicVolume(float64((i + 1) % 2))
		_ = MasterVolume()
		_ = musicVolume()
		if err := m.Play(-1); err != nil {
			t.Fatalf("Play: %v", err)
		}
		pauseMusic()
		_ = isMusicPaused()
		resumeMusic()
		_ = isMusicPlaying()
		rewindMusic()
		FadeOutMusic(1)
		if err := m.FadeIn(-1, 1); err != nil {
			t.Fatalf("FadeIn: %v", err)
		}
		HaltMusic()
	}
}

// TestSoundControlsBeforeInit pins that the sound API is inert before
// Init.  The channel mixer is built by Init, so playing a sound used to
// dereference a nil mixer and panic.
func TestSoundControlsBeforeInit(t *testing.T) {
	swapBackend(t, &beepBackend{})

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadSoundBytes: %v", err)
	}
	if _, err := s.Play(-1, 0); !errors.Is(err, errNotInitialized) {
		t.Errorf("Play before Init = %v, want %v", err, errNotInitialized)
	}
	if _, err := s.PlayOnce(); !errors.Is(err, errNotInitialized) {
		t.Errorf("PlayOnce before Init = %v, want %v", err, errNotInitialized)
	}
	if _, err := s.FadeIn(-1, 0, 10); !errors.Is(err, errNotInitialized) {
		t.Errorf("FadeIn before Init = %v, want %v", err, errNotInitialized)
	}
	// PlaySource shares the sentinel; the nil check must not run first.
	if err := PlaySource(-1, nil); !errors.Is(err, errNotInitialized) {
		t.Errorf("PlaySource before Init = %v, want %v", err, errNotInitialized)
	}
	HaltChannel(-1)
	fadeOutChannel(0, 10)
	pauseChannel(-1)
	resumeChannel(-1)
	if IsPlaying(0) {
		t.Error("channel 0 reports playing before Init")
	}
	// Volume is per sound, not per mixer, so it works before Init.
	s.setVolume(0.5)
	if got := s.Volume(); got != 0.5 {
		t.Errorf("Volume before Init = %v, want 0.5", got)
	}
	s.Free()
}

// TestRaceSoundControlsVsAudioThread is the sound-side twin of
// TestRaceMusicControlsVsAudioThread: the test goroutine plays, fades and
// sets volume while a second goroutine pulls samples from the mixer.
func TestRaceSoundControlsVsAudioThread(t *testing.T) {
	sfx, _ := swapReadyBackend(t)

	s, err := LoadSoundBytes(wavSilence)
	if err != nil {
		t.Fatalf("LoadSoundBytes: %v", err)
	}
	defer s.Free()

	stop := make(chan struct{})
	var audio sync.WaitGroup
	audio.Go(func() {
		var buf [256][2]float64
		for {
			select {
			case <-stop:
				return
			default:
			}
			sfx.Stream(buf[:])
		}
	})
	// A second app goroutine that only sets volume.  Play and the channel
	// controls lock the mixer, and the audio thread reads the volume
	// under that same lock, so a write made from the playing goroutine
	// looks ordered to the race detector.  This one never takes the
	// mixer lock, so an unsynchronized volume shows up as a race.
	audio.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			s.setVolume(float64(i % 2))
			_ = s.Volume()
		}
	})
	// Registered after s.Free's defer, so both goroutines stop first.
	defer func() {
		close(stop)
		audio.Wait()
	}()

	for range 200 {
		ch, err := s.Play(-1, -1)
		if err != nil {
			t.Fatalf("Play: %v", err)
		}
		pauseChannel(ch)
		resumeChannel(ch)
		_ = IsPlaying(ch)
		fadeOutChannel(ch, 1)
		if _, err := s.FadeIn(-1, -1, 1); err != nil {
			t.Fatalf("FadeIn: %v", err)
		}
		HaltChannel(-1)
	}
}

// TestRaceSoundLoadVsInit covers the one sound call that must not hold
// initMu: LoadSoundBytes decodes up to 50 MB, so it reads the output
// rate without the lock while Init may be writing it.
func TestRaceSoundLoadVsInit(t *testing.T) {
	b := &beepBackend{}
	swapBackend(t, b)

	stop := make(chan struct{})
	var loader sync.WaitGroup
	loader.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := LoadSoundBytes(wavSilence); err != nil {
				t.Errorf("LoadSoundBytes: %v", err)
				return
			}
			_ = SampleRate()
		}
	})
	// Stands in for Init: the same state write, under the same lock,
	// without an output device.
	for range 200 {
		initMu.Lock()
		b.setup(44100, 512, 4)
		initMu.Unlock()
	}
	close(stop)
	loader.Wait()
}
