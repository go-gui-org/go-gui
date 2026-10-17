package main

import (
	"math"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestOverflowPanelDemoItemsReadable: the demo's item buttons filled
// with the raw status colors under body text, so white text sat on
// amber and green at about 2:1 on dark. Every item's label must reach
// 4.5:1 on its fill in both themes.
func TestOverflowPanelDemoItemsReadable(t *testing.T) {
	prev := gui.CurrentTheme()
	t.Cleanup(func() { gui.SetTheme(prev) })
	for _, theme := range []gui.Theme{gui.ThemeDark, gui.ThemeLight} {
		gui.SetTheme(theme)
		w := gui.NewWindow(gui.WindowCfg{State: newShowcaseApp()})
		layout := gui.GenerateViewLayout(demoOverflowPanel(w), w)
		n := 0
		var walk func(l *gui.Layout)
		walk = func(l *gui.Layout) {
			if strings.Contains(l.Shape.ID, "demo_overflow_panel_") {
				fill := l.Shape.Color.Over(theme.ColorBackground)
				text := overflowLabelColor(l, theme)
				if r := wcagContrast(text, fill); r < 4.5 {
					t.Errorf("%s %s: label %v on %v contrast %.2f, want >= 4.5",
						theme.Name, l.Shape.ID, text, fill, r)
				}
				n++
				return
			}
			for i := range l.Children {
				walk(&l.Children[i])
			}
		}
		walk(&layout)
		if n == 0 {
			t.Fatalf("%s: no overflow demo items found", theme.Name)
		}
	}
}

// overflowLabelColor is the color of the first text run under l, or
// the theme's body color when the run leaves it to the default.
func overflowLabelColor(l *gui.Layout, theme gui.Theme) gui.Color {
	if l.Shape.TC != nil && l.Shape.TC.TextStyle != nil {
		return l.Shape.TC.TextStyle.Color
	}
	for i := range l.Children {
		if c := overflowLabelColor(&l.Children[i], theme); c.IsSet() {
			return c
		}
	}
	return gui.Color{}
}

// wcagContrast is the WCAG 2 contrast ratio of two opaque colors.
func wcagContrast(a, b gui.Color) float64 {
	lum := func(c gui.Color) float64 {
		ch := func(v uint8) float64 {
			s := float64(v) / 255
			if s <= 0.04045 {
				return s / 12.92
			}
			return math.Pow((s+0.055)/1.055, 2.4)
		}
		return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
	}
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestMultiWindowStatusReadable: the "child window is open" line used
// the raw success fill as text color. The theme's success text role is
// the one held to 4.5:1 on the page.
func TestMultiWindowStatusReadable(t *testing.T) {
	for _, theme := range []gui.Theme{gui.ThemeDark, gui.ThemeLight} {
		w := gui.NewWindow(gui.WindowCfg{State: newShowcaseApp()})
		l := gui.GenerateViewLayout(multiWindowStatus(theme, true), w)
		c := overflowLabelColor(&l, theme)
		for _, bg := range []gui.Color{theme.ColorBackground, theme.ColorPanel} {
			if r := wcagContrast(c, bg); r < 4.5 {
				t.Errorf("%s: status %v on %v contrast %.2f, want >= 4.5", theme.Name, c, bg, r)
			}
		}
	}
}
