package gui

import "fmt"

// Contrast floors (issue #863).
//
// A text role reads only if its color stands far enough from the
// surfaces it is drawn on. Before this table each role got that check
// when someone noticed it failed: the status roles in #861, link text
// only after it was measured at 3.4 on macOS. contrastFloors states
// the floor for every text role in one place, and one test holds every
// preset to it.
//
// The floors are not applied as a pass at the end of ThemeMaker. A
// quiet role is copied into a dozen widget styles (input placeholders,
// breadcrumb separators, the inspector's help text) as it is built,
// so a late correction would move the role and leave its copies
// behind. Each derivation site asks for its floor instead:
// themeTextRoles for the alpha roles, fillTextRungs for the status
// and link roles. WithColors re-derives through the same functions,
// so the two paths cannot drift.
//
// Only derived colors are corrected. A color the app states — the
// body text color, a ThemeCfg.ColorText* field, a role forked by hand
// — is its decision; the floor reports it under DebugLowContrast and
// leaves it alone.

const (
	// contrastText is WCAG AA for body-size text.
	contrastText = 4.5
	// contrastPlaceholder is lower on purpose: a placeholder stands in
	// for a value not entered yet and must not read as one. 3:1 keeps
	// it legible without promoting it.
	contrastPlaceholder = 3.0
)

// contrastSurfaces names the surfaces a role is drawn on.
type contrastSurfaces uint8

const (
	// onPage is running text: ColorBackground and ColorPanel.
	onPage contrastSurfaces = iota
	// onField is text inside a form control: ColorInterior.
	onField
)

// contrastFloor is one row: a text role, the contrast it must reach,
// and the surfaces it must reach it on.
type contrastFloor struct {
	role  string
	min   float64
	on    contrastSurfaces
	color func(*Theme) Color
}

// contrastFloors is every text role with a floor. TextStyleDisabled is
// absent on purpose: WCAG exempts text in a control the user cannot
// act on, and the role exists to look inactive. Icon roles are glyphs,
// not text, and draw in the body color.
var contrastFloors = [...]contrastFloor{
	{"display", contrastText, onPage, func(t *Theme) Color { return t.TextStyleDisplay.Color }},
	{"title", contrastText, onPage, func(t *Theme) Color { return t.TextStyleTitle.Color }},
	{"title-small", contrastText, onPage, func(t *Theme) Color { return t.TextStyleTitleSmall.Color }},
	{"body-large", contrastText, onPage, func(t *Theme) Color { return t.TextStyleBodyLarge.Color }},
	{"body", contrastText, onPage, func(t *Theme) Color { return t.TextStyleBody.Color }},
	{"body-small", contrastText, onPage, func(t *Theme) Color { return t.TextStyleBodySmall.Color }},
	{"caption", contrastText, onPage, func(t *Theme) Color { return t.TextStyleCaption.Color }},
	{"caption-small", contrastText, onPage, func(t *Theme) Color { return t.TextStyleCaptionSmall.Color }},
	{"code", contrastText, onPage, func(t *Theme) Color { return t.TextStyleCode.Color }},
	{"secondary", contrastText, onPage, func(t *Theme) Color { return t.TextStyleSecondary.Color }},
	{"label", contrastText, onPage, func(t *Theme) Color { return t.TextStyleLabel.Color }},
	{"placeholder", contrastPlaceholder, onField, func(t *Theme) Color { return t.TextStylePlaceholder.Color }},
	{"error", contrastText, onPage, func(t *Theme) Color { return t.TextStyleError.Color }},
	{"success", contrastText, onPage, func(t *Theme) Color { return t.TextStyleSuccess.Color }},
	{"warning", contrastText, onPage, func(t *Theme) Color { return t.TextStyleWarning.Color }},
	{"link", contrastText, onPage, func(t *Theme) Color { return t.TextStyleLink.Color }},
}

// contrastSurfaceColors returns the surfaces for on, from the three
// colors that can hold them. An unset surface is left out: a bare
// ThemeCfg leaves them unset, contrastRatio would read that as black,
// and a floor solved against a surface that is never painted moves the
// color for nothing. The array keeps the call free of allocation.
func contrastSurfaceColors(on contrastSurfaces, bg, panel, interior Color) ([2]Color, int) {
	var out [2]Color
	n := 0
	add := func(c Color) {
		if c.IsSet() {
			out[n] = c
			n++
		}
	}
	switch on {
	case onPage:
		add(bg)
		add(panel)
	case onField:
		add(interior)
	}
	return out, n
}

// blendOver is fg drawn over an opaque bg: what the eye sees, so a
// translucent role is measured at the contrast it renders at.
func blendOver(fg, bg Color) Color {
	a := float64(fg.A) / 255
	mix := func(f, b uint8) uint8 {
		return uint8(float64(f)*a + float64(b)*(1-a) + 0.5)
	}
	return RGB(mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B))
}

// worstContrast is the lowest contrast c reaches over bgs, and the
// surface it reaches it on. No surface reads as unlimited contrast.
func worstContrast(c Color, bgs ...Color) (float64, Color) {
	worst, on := 21.0, Color{}
	for _, bg := range bgs {
		if r := contrastRatio(blendOver(c, bg), bg); r < worst {
			worst, on = r, bg
		}
	}
	return worst, on
}

// readableAlpha raises c's alpha until c reaches minRatio over every
// surface in bgs. It is the correction for the quiet roles, which are
// the body color at an alpha: moving their lightness would change
// their hue relationship to the body text, raising the alpha keeps it.
// c already readable comes back unchanged; one that cannot be made
// readable comes back opaque, the best on offer.
func readableAlpha(c Color, minRatio float64, bgs ...Color) Color {
	for c.A < 255 {
		if got, _ := worstContrast(c, bgs...); got >= minRatio {
			return c
		}
		c.A++
	}
	return c
}

// readableOnPage is c moved on lightness until it reads as body text
// on the page surfaces. The status and link roles are all this.
func readableOnPage(c, bg, panel Color) Color {
	s, n := contrastSurfaceColors(onPage, bg, panel, Color{})
	return readableOn(c, contrastText, s[:n]...)
}

// debugCheckContrast reports a text role of the window's theme below
// its floor. A derived role lands there only when the floor is out of
// reach (a mid-gray surface); an app-stated color lands there when the
// app chose it. Either way the fix is the app's, so it is reported,
// not corrected.
//
// Runs from the frame audit. The message is formatted only for a role
// that fails, so a readable theme costs a few dozen contrast sums.
func (w *Window) debugCheckContrast() {
	if DebugCategory(debugMask.Load())&DebugLowContrast == 0 {
		return
	}
	t := w.themeRef()
	for i := range contrastFloors {
		f := &contrastFloors[i]
		c := f.color(t)
		s, n := contrastSurfaceColors(f.on, t.ColorBackground,
			t.ColorPanel, t.ColorInterior)
		got, on := worstContrast(c, s[:n]...)
		if got >= f.min {
			continue
		}
		w.debugWarn(debugCheckLowContrast,
			fmt.Sprintf("%d %s %s", t.id, t.Name, f.role),
			"theme %q: %s text %v on %v has contrast %.2f, below %.1f",
			t.Name, f.role, c, on, got, f.min)
	}
}
