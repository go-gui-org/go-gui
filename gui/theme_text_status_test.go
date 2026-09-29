package gui

import (
	"math"
	"testing"
)

// wantStatusTextAA is the WCAG AA floor for body-size text. The
// status text roles are body size, so they must clear it (issue #861).
const wantStatusTextAA = 4.5

// statusPresets is every preset theme ThemeMaker ships.
func statusPresets() []Theme {
	return []Theme{
		ThemeDark, ThemeLight,
		themeMacOS, themeMacOSDark,
		themeGnome, themeGnomeDark,
		themeWindows, themeWindowsDark,
	}
}

// statusRole pairs a status text role with the fill color it derives
// from, so every test below runs over all three.
type statusRole struct {
	name string
	role func(Theme) TextStyle
	src  func(Theme) Color
}

var statusRoles = []statusRole{
	{"error",
		func(t Theme) TextStyle { return t.TextStyleError },
		func(t Theme) Color { return t.toastStyle.ColorError }},
	{"success",
		func(t Theme) TextStyle { return t.TextStyleSuccess },
		func(t Theme) Color { return t.toastStyle.ColorSuccess }},
	{"warning",
		func(t Theme) TextStyle { return t.TextStyleWarning },
		func(t Theme) Color { return t.toastStyle.ColorWarning }},
}

// TestStatusTextReadable is the reason the roles derive their color
// instead of copying the fill color: the fills sit near 3:1 on light
// themes, fine for a toast accent, too faint for body text. Every
// preset, every role, against both surfaces text sits on.
func TestStatusTextReadable(t *testing.T) {
	for _, th := range statusPresets() {
		for _, r := range statusRoles {
			c := r.role(th).Color
			for _, bg := range []Color{th.ColorBackground, th.ColorPanel} {
				if got := contrastRatio(c, bg); got < wantStatusTextAA {
					t.Errorf("%s %s: %v on %v contrast %.2f, want >= %.1f",
						th.Name, r.name, c, bg, got, wantStatusTextAA)
				}
			}
		}
	}
}

// TestStatusTextIsBody pins everything but the color: body size,
// family and face, so a status line sits in running text without a
// size jump.
func TestStatusTextIsBody(t *testing.T) {
	for _, th := range statusPresets() {
		for _, r := range statusRoles {
			got := r.role(th)
			want := th.TextStyleBody
			want.Color = got.Color
			if got != want {
				t.Errorf("%s %s: %+v, want Body with only color changed",
					th.Name, r.name, got)
			}
		}
	}
}

// TestStatusTextKeepsHue checks the derivation moves lightness only:
// the text must still read as red, green or amber. Hue is compared on
// the OKLCH wheel; a small drift comes from sRGB gamut clipping.
func TestStatusTextKeepsHue(t *testing.T) {
	for _, th := range statusPresets() {
		for _, r := range statusRoles {
			src := colorToOKLCH(r.src(th)).H
			got := colorToOKLCH(r.role(th).Color).H
			d := math.Abs(float64(got - src))
			d = math.Min(d, 360-d)
			if d > 15 {
				t.Errorf("%s %s: hue %.1f, source %.1f", th.Name, r.name, got, src)
			}
		}
	}
}

// TestStatusTextKeepsChroma checks the text stays saturated. A darker
// amber sits just past the sRGB gamut wall, and oklch.color() halves
// chroma to get back in, which turned light-theme warning text into a
// gray-brown (129,106,69). readableOn fits chroma to the wall instead.
//
// Dark yellows cannot hold much chroma in sRGB at all, so the check is
// "kept 70% of the source, or sits on the gamut wall": 5% more chroma
// at the role's lightness and hue would leave sRGB.
func TestStatusTextKeepsChroma(t *testing.T) {
	for _, th := range statusPresets() {
		for _, r := range statusRoles {
			src := colorToOKLCH(r.src(th)).C
			v := colorToOKLCH(r.role(th).Color)
			if v.C >= 0.7*src {
				continue
			}
			if rr, gg, bb := oklchToLinearRGB(v.L, v.C*1.05, v.H); inGamut(rr, gg, bb) {
				t.Errorf("%s %s: chroma %.3f, source %.3f, and not on the gamut wall",
					th.Name, r.name, v.C, src)
			}
		}
	}
}

// TestStatusTextUnchangedWhenReadable checks the derivation does
// nothing to a color that already clears the floor. Dark theme
// warning (amber on near-black) is the known case.
func TestStatusTextUnchangedWhenReadable(t *testing.T) {
	src := ThemeDark.toastStyle.ColorWarning
	// readableOn solves against both surfaces, so "already readable"
	// means both.
	if contrastRatio(src, ThemeDark.ColorBackground) < wantStatusTextAA ||
		contrastRatio(src, ThemeDark.ColorPanel) < wantStatusTextAA {
		t.Skip("dark warning no longer readable as-is; pick another case")
	}
	if got := ThemeDark.TextStyleWarning.Color; !got.eq(src) {
		t.Errorf("readable color moved: %v, want %v", got, src)
	}
}

// TestStatusTextCustomColor checks a theme's own ColorError reaches
// the role, moved only as far as the floor needs.
//
// Built on the light cfg: a bare ThemeCfg leaves the surfaces unset,
// which contrastRatio reads as black, and a light pink already clears
// 4.5 on black, so the derivation would never run.
func TestStatusTextCustomColor(t *testing.T) {
	src := RGB(255, 120, 120)
	cfg := ThemeLight.Cfg
	cfg.ColorError = src
	custom := ThemeMaker(cfg)
	got := custom.TextStyleError.Color
	if got.eq(src) {
		t.Fatalf("pink on a light surface not moved: %v", got)
	}
	if got.eq(ThemeLight.TextStyleError.Color) {
		t.Fatalf("custom ColorError ignored: %v", got)
	}
	for _, bg := range []Color{custom.ColorBackground, custom.ColorPanel} {
		if c := contrastRatio(got, bg); c < wantStatusTextAA {
			t.Errorf("%v on %v contrast %.2f, want >= %.1f",
				got, bg, c, wantStatusTextAA)
		}
	}
	// "Only as far as the floor needs": one lightness step less would
	// fall under the floor on at least one surface.
	v := colorToOKLCH(got)
	v.L += readableStep
	back := fitChroma(v)
	if contrastRatio(back, custom.ColorBackground) >= wantStatusTextAA &&
		contrastRatio(back, custom.ColorPanel) >= wantStatusTextAA {
		t.Errorf("moved past the floor: %v still readable one step lighter", back)
	}
}

// TestStatusTextFollowsWithColors checks the recolor path: a status
// color override moves its role while the role still sits on its
// derivation, and leaves a role the app changed by hand alone.
func TestStatusTextFollowsWithColors(t *testing.T) {
	over := ColorOverrides{
		ColorError:   RGB(250, 20, 20),
		ColorSuccess: RGB(20, 250, 20),
		ColorWarning: RGB(250, 200, 20),
	}
	for _, r := range statusRoles {
		got := ThemeLight.WithColors(over)
		want := ThemeMaker(func() ThemeCfg {
			cfg := ThemeLight.Cfg
			applyOverridesToCfg(&cfg, over)
			return cfg
		}())
		if !r.role(got).Color.eq(r.role(want).Color) {
			t.Errorf("%s derived: %v, want %v",
				r.name, r.role(got).Color, r.role(want).Color)
		}
	}

	forked := ThemeLight
	own := RGB(1, 2, 3)
	forked.TextStyleError.Color = own
	forked.TextStyleSuccess.Color = own
	forked.TextStyleWarning.Color = own
	got := forked.WithColors(over)
	for _, r := range statusRoles {
		if !r.role(got).Color.eq(own) {
			t.Errorf("%s forked: %v, want kept %v",
				r.name, r.role(got).Color, own)
		}
	}
}

// TestReadableOnBounded checks the helper stops on a background where
// the floor is out of reach and returns the best it found instead of
// looping. On mid-gray, 7:1 is out of reach: black text gives about
// 4.7, white about 4.5.
func TestReadableOnBounded(t *testing.T) {
	bg := RGB(119, 119, 119)
	got := readableOn(RGB(218, 54, 51), 7, bg)
	if c := contrastRatio(got, bg); c < 4.5 {
		t.Errorf("unreachable floor: contrast %.2f, want the best reachable (>= 4.5)", c)
	}
}
