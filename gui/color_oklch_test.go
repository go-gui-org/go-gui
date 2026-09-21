package gui

import (
	"math"
	"testing"
)

// Tests for the OKLCH ramp derivation (issue #732). The matrices
// pin against the published sRGB-red anchor; round-trips pin every
// preset color the ramps actually shift.

func oklchClose(a, b, tol float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// TestColorToOKLCHAnchor pins the conversion against the published
// sRGB-red values (L 0.62796, C 0.25768, H 29.23°), so a matrix typo
// fails here rather than as a wrong ramp downstream.
func TestColorToOKLCHAnchor(t *testing.T) {
	got := colorToOKLCH(RGB(255, 0, 0))
	if !oklchClose(got.L, 0.62796, 0.001) {
		t.Errorf("red L = %v, want 0.62796", got.L)
	}
	if !oklchClose(got.C, 0.25768, 0.001) {
		t.Errorf("red C = %v, want 0.25768", got.C)
	}
	if !oklchClose(got.H, 29.23, 0.05) {
		t.Errorf("red H = %v, want 29.23", got.H)
	}
	if got.A != 1 {
		t.Errorf("red A = %v, want 1", got.A)
	}
}

// TestOKLCHRoundTrip pins dl=0 as the identity on the colors the
// ramps shift: primaries, grays and every preset accent and error.
func TestOKLCHRoundTrip(t *testing.T) {
	colors := []Color{
		RGB(255, 0, 0), RGB(0, 255, 0), RGB(0, 0, 255),
		RGB(0, 0, 0), RGB(255, 255, 255), RGB(128, 128, 128),
		colorAccentDark, colorAccentLight,
		colorErrorDark, colorErrorLight,
	}
	for _, c := range colors {
		if got := oklchShift(c, 0); got != c {
			t.Errorf("shift(%v, 0) = %v, want identity", c, got)
		}
	}
}

// TestOKLCHShiftMovesLightness pins the direction both ramps take:
// positive deltas read lighter, negative darker, at any hue.
func TestOKLCHShiftMovesLightness(t *testing.T) {
	for _, c := range []Color{
		colorAccentDark, colorErrorLight, RGB(0, 160, 0),
	} {
		base := srgbLuminance(c)
		if up := srgbLuminance(oklchShift(c, 0.10)); up <= base {
			t.Errorf("shift(%v, +0.10) luminance %v, want above %v",
				c, up, base)
		}
		if down := srgbLuminance(oklchShift(c, -0.10)); down >= base {
			t.Errorf("shift(%v, -0.10) luminance %v, want below %v",
				c, down, base)
		}
	}
}

// TestOKLCHShiftKeepsAlpha pins alpha as carried through, never
// derived: a translucent accent shifts to a translucent state.
func TestOKLCHShiftKeepsAlpha(t *testing.T) {
	c := RGBA(77, 130, 240, 128)
	for _, d := range []float32{0.10, -0.10, 0} {
		if got := oklchShift(c, d); got.A != 128 {
			t.Errorf("shift alpha = %v, want 128", got.A)
		}
	}
}

// TestOKLCHShiftClampsLightness pins the extremes: white cannot
// lighten past 1, black cannot darken past 0, and neither panics.
func TestOKLCHShiftClampsLightness(t *testing.T) {
	if got := oklchShift(White, 0.10); got != White {
		t.Errorf("shift(white, +0.10) = %v, want white", got)
	}
	black := RGB(0, 0, 0)
	if got := oklchShift(black, -0.10); got != black {
		t.Errorf("shift(black, -0.10) = %v, want black", got)
	}
}

// TestOKLCHGamutClamp pins the out-of-gamut rule: a lighter blue at
// full chroma has no sRGB address, so chroma halves until it fits,
// keeping the hue a clip would shear off.
func TestOKLCHGamutClamp(t *testing.T) {
	got := oklchShift(colorAccentDark, 0.10)
	want := colorToOKLCH(colorAccentDark)
	shifted := colorToOKLCH(got)
	if shifted.L <= want.L {
		t.Errorf("clamped hover L = %v, want above accent %v",
			shifted.L, want.L)
	}
	dh := shifted.H - want.H
	if dh < 0 {
		dh = -dh
	}
	if dh > 180 {
		dh = 360 - dh
	}
	if dh > 3 {
		t.Errorf("clamped hover H drift = %v°, want within 3° of %v",
			dh, want.H)
	}
}

// TestOKLCHNormalize pins the trust boundary, following the
// HSLA.Normalized precedent: non-finite becomes zero, H wraps,
// negative chroma floors.
func TestOKLCHNormalize(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	got := oklch{L: nan, C: inf, H: nan, A: inf}.normalize()
	if got != (oklch{}) {
		t.Errorf("normalize(NaN/Inf) = %+v, want zero", got)
	}
	if got := (oklch{H: 370}).normalize().H; got != 10 {
		t.Errorf("normalize H 370 = %v, want 10", got)
	}
	if got := (oklch{H: -30}).normalize().H; got != 330 {
		t.Errorf("normalize H -30 = %v, want 330", got)
	}
	if got := (oklch{C: -0.5}).normalize().C; got != 0 {
		t.Errorf("normalize C -0.5 = %v, want 0", got)
	}
}
