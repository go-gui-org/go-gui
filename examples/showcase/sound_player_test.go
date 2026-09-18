//go:build !js && !android && !ios

package main

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A zero-value player carries no window. It must stay silent and never
// panic: the framework never builds one, but the player is app-facing.
func TestCueSoundPlayerNilWindow(t *testing.T) {
	nilPlayer := cueSoundPlayer{}
	for _, cue := range []gui.SoundCue{
		gui.SoundClick, gui.SoundError, gui.SoundCue(255),
	} {
		nilPlayer.PlaySound(cue, 1)
	}
}

// newVoice owns envelope validation now that envelope runs per sample
// with no checks. Every bad timing or sustain must still yield finite
// samples within the peak level, and the voice must finish.
func TestNewVoiceSanitizesEnvelope(t *testing.T) {
	bad := []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, x := range bad {
		env := voiceEnv{
			attackS: x, decayS: x, releaseS: x,
			sustain: x, oneShot: true,
		}
		v := newVoice(440, 1, env)
		buf := make([][2]float64, 256)
		for range 8 {
			n, ok := v.Fill(buf)
			for _, s := range buf[:n] {
				for _, ch := range s {
					if math.IsNaN(ch) || math.IsInf(ch, 0) || math.Abs(ch) > 1 {
						t.Fatalf("env %v: sample %v not finite in [-1, 1]", x, ch)
					}
				}
			}
			if !ok {
				break
			}
		}
	}
}

// A direct caller may pass any gain. Mute, negative, NaN and Inf must
// not start a voice; a gain above 1 clamps to full.
func TestCueGain(t *testing.T) {
	for _, gain := range []float32{
		0, -1, float32(math.NaN()),
		float32(math.Inf(1)), float32(math.Inf(-1)),
	} {
		if _, ok := cueGain(gain); ok {
			t.Errorf("cueGain(%v) ok = true, want false", gain)
		}
	}
	for _, tc := range []struct{ in, want float32 }{
		{0.5, 0.5}, {1, 1}, {2, 1},
	} {
		got, ok := cueGain(tc.in)
		if !ok || got != tc.want {
			t.Errorf("cueGain(%v) = %v, %v; want %v, true", tc.in, got, ok, tc.want)
		}
	}
}
