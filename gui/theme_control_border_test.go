package gui

import "testing"

// TestBooleanControlBoundaryContrast holds the off state of Radio,
// Checkbox (Toggle) and Switch to WCAG 1.4.11: the edge that shows
// where the control is needs 3:1 against the surfaces around it. The
// shared hairline border sits near 1.2 on light presets, so an
// unchecked box or radio all but vanished. The switch's off thumb is
// the only mark of its state, so it is held to the same floor.
func TestBooleanControlBoundaryContrast(t *testing.T) {
	for _, th := range statusPresets() {
		page := []Color{th.ColorBackground, th.ColorPanel}
		checks := []struct {
			name string
			c    Color
		}{
			{"radio border", th.radioStyle.Colors.Border},
			{"checkbox border", th.toggleStyle.Colors.Border},
			{"switch border", th.switchStyle.Colors.Border},
			{"switch off thumb", th.switchStyle.colorUnselect},
		}
		for _, k := range checks {
			if got, on := worstContrast(k.c, page...); got < contrastNonText {
				t.Errorf("%s %s: %v on %v contrast %.2f, want >= %.1f",
					th.Name, k.name, k.c, on, got, contrastNonText)
			}
		}
	}
}

// TestRadioRingHoldsUnderPointer: Radio paints hover and press on its
// ring. Those states must not fade the ring below its resting edge, or
// the control vanishes as the pointer reaches it.
func TestRadioRingHoldsUnderPointer(t *testing.T) {
	for _, th := range statusPresets() {
		page := []Color{th.ColorBackground, th.ColorPanel}
		rest, _ := worstContrast(th.radioStyle.Colors.Border, page...)
		for _, k := range []struct {
			name string
			c    Color
		}{
			{"hover", th.radioStyle.Colors.Hover},
			{"click", th.radioStyle.Colors.Click},
		} {
			if got, _ := worstContrast(k.c, page...); got < rest {
				t.Errorf("%s radio %s ring contrast %.2f, below resting edge %.2f",
					th.Name, k.name, got, rest)
			}
		}
	}
}

// TestSliderThumbEdgeContrast: the slider thumb is a panel-colored
// disc, so on a light page its edge is all that shows where the value
// sits. It is held to 3:1 like the off checkbox. The hover cue lands
// on the track border and must not fade it below that.
func TestSliderThumbEdgeContrast(t *testing.T) {
	for _, th := range statusPresets() {
		page := []Color{th.ColorBackground, th.ColorPanel}
		rest, on := worstContrast(th.sliderStyle.Colors.Border, page...)
		if rest < contrastNonText {
			t.Errorf("%s slider edge: %v on %v contrast %.2f, want >= %.1f",
				th.Name, th.sliderStyle.Colors.Border, on, rest, contrastNonText)
		}
		if got, _ := worstContrast(th.sliderStyle.Colors.Hover, page...); got < rest {
			t.Errorf("%s slider hover edge contrast %.2f, below resting edge %.2f",
				th.Name, got, rest)
		}
	}
}
