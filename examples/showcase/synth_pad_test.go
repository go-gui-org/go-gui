//go:build !js && !android && !ios

package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A synth pad centres its two lines as a group: theme container
// padding used to fill the fixed box and push the note name and
// frequency to its bottom edge. Lives here rather than in
// showcase_test.go because synthPadGrid is desktop-only, like
// sound_player_test.go.
func TestSynthPadCentersLines(t *testing.T) {
	w := gui.NewTestWindow(gui.WindowCfg{Width: 600, Height: 400})
	defer w.Close()
	root := w.TestRender(func(w *gui.Window) gui.View {
		return synthPadGrid(gui.CurrentTheme())
	})
	pad, ok := root.FindByID("pad-c4")
	if !ok {
		t.Fatal("pad-c4 not found")
	}
	name := &pad.Children[0]
	freq := &pad.Children[1]
	content := freq.Shape.Y + freq.Shape.Height - name.Shape.Y
	// Optical correction per line (headless fallback: 0.0886 * size;
	// neither "C4" nor "262 Hz" descends). Each line shifts
	// independently, so the pre-shift content is the measured one
	// minus the difference of the two shifts.
	offName := gui.CurrentTheme().N3.Size * 0.0886
	offFreq := gui.CurrentTheme().N4.Size * 0.0886
	metric := (pad.Shape.Height - (content - (offFreq - offName))) / 2
	if got := name.Shape.Y - pad.Shape.Y; got-metric-offName > 0.01 || metric+offName-got > 0.01 {
		t.Fatalf("note offset = %v, want %v (pad h=%v content h=%v)",
			got, metric+offName, pad.Shape.Height, content)
	}
}
