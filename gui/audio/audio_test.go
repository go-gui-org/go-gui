//go:build !js && !android && !ios

package audio

import (
	"testing"
)

func TestInitQuit(t *testing.T) {
	// Init may fail if SDL audio device is unavailable (headless CI).
	// Test that it doesn't panic and that Quit cleans up correctly.
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

	// Second Init should succeed with no side effects.
	if err := Init(); err != nil {
		t.Errorf("second Init returned error: %v", err)
	}
}

func TestQuitBeforeInit(t *testing.T) {
	// Quit before Init should not panic.
	Quit()
}

func TestVolumeClamp(t *testing.T) {
	tests := []struct {
		in, want float32
	}{
		{0.0, 0.0},
		{0.5, 0.5},
		{1.0, 1.0},
		{-0.1, 0.0},
		{1.5, 1.0},
		{2.0, 1.0},
	}
	for _, tt := range tests {
		got := clamp01(tt.in)
		if got != tt.want {
			t.Errorf("clamp01(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestVolumeConversion(t *testing.T) {
	// Round-trip: toMixVol → fromMixVol should be identity for valid
	// inputs.
	for _, v := range []float32{0.0, 0.25, 0.5, 0.75, 1.0} {
		mix := toMixVol(v)
		got := fromMixVol(mix)
		// Allow small rounding error from int conversion.
		delta := got - v
		if delta < 0 {
			delta = -delta
		}
		if delta > 0.01 {
			t.Errorf("round-trip %v: toMixVol=%d, fromMixVol=%v (delta=%v)",
				v, mix, got, delta)
		}
	}
}

func TestVolumeAfterInit(t *testing.T) {
	err := Init()
	if err != nil {
		t.Skipf("audio init unavailable: %v", err)
	}
	defer Quit()

	// Volume getters/setters must not panic when audio is initialized.
	v := MasterVolume()
	SetMasterVolume(0.5)
	_ = MusicVolume()
	SetMusicVolume(0.5)
	_ = v
}
