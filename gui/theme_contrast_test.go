package gui

import (
	"strings"
	"testing"
)

// floorSurfaces is the surfaces a row is held to on t.
func floorSurfaces(t *Theme, f *contrastFloor) []Color {
	s, n := contrastSurfaceColors(f.on, t.ColorBackground,
		t.ColorPanel, t.ColorInterior)
	return s[:n]
}

// TestContrastFloorsPresets is the gate the table exists for (issue
// #863): every text role, on every preset, at or above its floor on
// every surface it is drawn on. A new role joins by adding a row.
func TestContrastFloorsPresets(t *testing.T) {
	for _, th := range statusPresets() {
		for i := range contrastFloors {
			f := &contrastFloors[i]
			c := f.color(&th)
			if got, on := worstContrast(c, floorSurfaces(&th, f)...); got < f.min {
				t.Errorf("%s %s: %v on %v contrast %.2f, want >= %.1f",
					th.Name, f.role, c, on, got, f.min)
			}
		}
	}
}

// TestContrastLinkCorrected checks the case that opened the issue:
// ColorSelect as link text sat at 3.40 on macOS. The link role reads,
// and the ColorSelect fill it came from is left alone.
func TestContrastLinkCorrected(t *testing.T) {
	th := themeMacOS
	page := []Color{th.ColorBackground, th.ColorPanel}
	if got, _ := worstContrast(th.ColorSelect, page...); got >= contrastText {
		t.Skipf("macOS ColorSelect already reads (%.2f); pick another case", got)
	}
	if !th.ColorSelect.eq(th.Cfg.ColorSelect) {
		t.Errorf("ColorSelect moved: %v, cfg %v", th.ColorSelect, th.Cfg.ColorSelect)
	}
	if got, _ := worstContrast(th.TextStyleLink.Color, page...); got < contrastText {
		t.Errorf("link contrast %.2f, want >= %.1f", got, contrastText)
	}
	src := colorToOKLCH(th.ColorSelect).H
	if h := colorToOKLCH(th.TextStyleLink.Color).H; h-src > 15 || src-h > 15 {
		t.Errorf("link hue %.1f, source %.1f", h, src)
	}
}

// TestContrastLinkIsBody pins the link role's shape: body text,
// underlined, only the color and the underline changed.
func TestContrastLinkIsBody(t *testing.T) {
	for _, th := range statusPresets() {
		want := th.TextStyleBody
		want.Color = th.TextStyleLink.Color
		want.Underline = true
		if th.TextStyleLink != want {
			t.Errorf("%s: %+v, want underlined Body", th.Name, th.TextStyleLink)
		}
	}
}

// grayBodyCfg is a light theme whose body text is a mid gray: readable
// itself (about 5.9:1 on the light ground), but the de-emphasis ladder
// taken off it lands under the floor.
func grayBodyCfg() ThemeCfg {
	cfg := ThemeLight.Cfg
	cfg.Name = "gray-body"
	cfg.TextStyleDef.Color = RGB(100, 100, 100)
	return cfg
}

// TestContrastAlphaRolesRaised checks the alpha correction: a derived
// quiet role under its floor is raised to it, by alpha, keeping the
// body color's RGB.
func TestContrastAlphaRolesRaised(t *testing.T) {
	cfg := grayBodyCfg()
	th := ThemeMaker(cfg)
	body := th.TextStyleBody.Color
	page := []Color{th.ColorBackground, th.ColorPanel}

	naive := RGBA(body.R, body.G, body.B, textRolesOnLight.secondary)
	if got, _ := worstContrast(naive, page...); got >= contrastText {
		t.Fatalf("unraised secondary already reads (%.2f); the case proves nothing", got)
	}
	got := th.TextStyleSecondary.Color
	if got.R != body.R || got.G != body.G || got.B != body.B {
		t.Errorf("secondary RGB %v, want body RGB %v", got, body)
	}
	if got.A <= naive.A {
		t.Errorf("secondary alpha %d, want raised above %d", got.A, naive.A)
	}
	if c, _ := worstContrast(got, page...); c < contrastText {
		t.Errorf("secondary contrast %.2f, want >= %.1f", c, contrastText)
	}
	// Only as far as the floor needs: one alpha step less falls under.
	back := got
	back.A--
	if c, _ := worstContrast(back, page...); c >= contrastText {
		t.Errorf("raised past the floor: alpha %d still reads", back.A)
	}
	// The copies ThemeMaker makes of the role follow it.
	if th.progressBarStyle.TextStyle != th.TextStyleSecondary {
		t.Error("progress bar text did not take the raised secondary role")
	}
}

// TestContrastExplicitKept checks the other half of the rule: a color
// the app states is never moved, and DebugLowContrast reports it.
func TestContrastExplicitKept(t *testing.T) {
	cfg := ThemeLight.Cfg
	cfg.Name = "faint-secondary"
	faint := RGB(200, 200, 200)
	cfg.ColorTextSecondary = faint
	th := ThemeMaker(cfg)
	if !th.TextStyleSecondary.Color.eq(faint) {
		t.Fatalf("explicit secondary moved: %v, want %v",
			th.TextStyleSecondary.Color, faint)
	}

	w := NewTestWindow(t, WindowCfg{})
	w.SetTheme(th)
	w.SetView(func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	})
	found := w.TestFindings(DebugLowContrast)
	if len(found) != 1 || !strings.Contains(found[0], "secondary") {
		t.Fatalf("want one low-contrast finding naming secondary, got %q", found)
	}
}

// TestContrastDebugQuietOnPresets checks the finding does not fire on
// a theme that meets every floor, so DebugAll stays usable.
func TestContrastDebugQuietOnPresets(t *testing.T) {
	for _, th := range statusPresets() {
		w := NewTestWindow(t, WindowCfg{})
		w.SetTheme(th)
		w.SetView(func(_ *Window) View {
			return Column(ContainerCfg{Sizing: FillFill})
		})
		if found := w.TestFindings(DebugLowContrast); len(found) != 0 {
			t.Errorf("%s: %q", th.Name, found)
		}
	}
}

// TestContrastLinkFollowsWithColors checks the recolor path: a
// ColorSelect override moves a derived link role, and a forked one
// stays.
func TestContrastLinkFollowsWithColors(t *testing.T) {
	over := ColorOverrides{ColorSelect: RGB(120, 170, 255)}
	got := ThemeLight.WithColors(over)
	want := ThemeMaker(func() ThemeCfg {
		cfg := ThemeLight.Cfg
		applyOverridesToCfg(&cfg, over)
		return cfg
	}())
	if !got.TextStyleLink.Color.eq(want.TextStyleLink.Color) {
		t.Errorf("derived link %v, want %v",
			got.TextStyleLink.Color, want.TextStyleLink.Color)
	}

	forked := ThemeLight
	own := RGB(1, 2, 3)
	forked.TextStyleLink.Color = own
	if c := forked.WithColors(over).TextStyleLink.Color; !c.eq(own) {
		t.Errorf("forked link %v, want kept %v", c, own)
	}
}

// TestContrastBareCfgUntouched checks an unset surface is no surface:
// a bare ThemeCfg paints no background of its own, and solving against
// the black an unset Color reads as would move colors for nothing.
func TestContrastBareCfgUntouched(t *testing.T) {
	sel := RGB(40, 80, 200)
	th := ThemeMaker(ThemeCfg{Name: "bare", ColorSelect: sel})
	if th.ColorBackground.IsSet() || th.ColorPanel.IsSet() {
		t.Skip("bare cfg now resolves its surfaces; case no longer applies")
	}
	if !th.TextStyleLink.Color.eq(sel) {
		t.Errorf("link moved on a theme with no surfaces: %v, want %v",
			th.TextStyleLink.Color, sel)
	}
}

// TestReadableAlphaEdges pins the two ends of the alpha correction: a
// floor out of reach comes back opaque, the best on offer, and no
// surface at all leaves the color as it came.
func TestReadableAlphaEdges(t *testing.T) {
	gray := RGB(128, 128, 128)
	if got := readableAlpha(RGBA(120, 120, 120, 40), contrastText, gray); got.A != 255 {
		t.Errorf("unreachable floor: alpha %d, want 255", got.A)
	}
	in := RGBA(120, 120, 120, 40)
	if got := readableAlpha(in, contrastText); !got.eq(in) {
		t.Errorf("no surface: %v, want %v unchanged", got, in)
	}
	if got, on := worstContrast(in); got != 21 || on.IsSet() {
		t.Errorf("no surface: contrast %.2f on %v, want 21 on none", got, on)
	}
}
