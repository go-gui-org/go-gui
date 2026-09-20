package gui

import "testing"

// A badge drew its label in a hardcoded White whatever it filled with.
// Every light preset fills a neutral badge with ColorActive — a
// near-white — so the label landed on it at contrast 1.26 to 1.32:
// invisible, not merely low. The theme already owned the pairing rule
// (textOnFor, the same one behind ColorTextOnAccent); the badge just
// was not asking.
//
// contrastRatio is the WCAG ratio between two opaque colors.
func contrastRatio(a, b Color) float64 {
	la, lb := srgbLuminance(a), srgbLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// badgeVariantCases is every fill a badge can paint.
var badgeVariantCases = []struct {
	name    string
	variant badgeVariant
}{
	{"neutral", badgeDefault},
	{"info", BadgeInfo},
	{"success", BadgeSuccess},
	{"warning", BadgeWarning},
	{"error", BadgeError},
}

// badgeLabelOn builds a badge under theme and returns the fill and the
// label color a frame would paint.
func badgeLabelOn(t *testing.T, variant badgeVariant) (fill, label Color) {
	t.Helper()
	view := Badge(BadgeCfg{Label: "128", Variant: variant})
	layout := generateViewLayout(view, &Window{})
	if len(layout.Children) != 1 {
		t.Fatalf("badge has %d children, want 1", len(layout.Children))
	}
	return layout.Shape.Color, layout.Children[0].Shape.TC.TextStyle.Color
}

// TestBadgeLabelPairsWithFill is the rule: across every registered
// preset and every variant, the label is the theme's paired foreground
// for the fill the badge actually paints. Fails on the old White for
// every light fill.
func TestBadgeLabelPairsWithFill(t *testing.T) {
	restore := CurrentTheme()
	defer applyTheme(&restore)

	for _, name := range ThemeRegisteredNames() {
		theme, ok := ThemeGet(name)
		if !ok {
			t.Fatalf("theme %q registered but not retrievable", name)
		}
		applyTheme(&theme)
		for _, v := range badgeVariantCases {
			fill, label := badgeLabelOn(t, v.variant)
			if want := textOnFor(fill); !label.eq(want) {
				t.Errorf("%s/%s: label %v on fill %v, want %v",
					name, v.name, label, fill, want)
			}
		}
	}
}

// TestBadgeNeutralLabelReadable is the consequence worth stating as a
// number. The neutral fill is a theme surface color, so the pairing
// clears AA on it in every preset — that is the case the hardcoded
// White broke outright, at 1.26 to 1.32.
//
// Only the neutral fill. The semantic fills are brand colors at
// mid-luminance, where neither black nor white reaches 4.5 (macOS
// warning #FF9500 tops out at 2.20 against white and 2.62 against
// black). Raising those is a palette decision across all eight
// presets, not a pairing one; see the note in theme_maker.go.
func TestBadgeNeutralLabelReadable(t *testing.T) {
	const wantAA = 4.5
	restore := CurrentTheme()
	defer applyTheme(&restore)

	for _, name := range ThemeRegisteredNames() {
		theme, ok := ThemeGet(name)
		if !ok {
			t.Fatalf("theme %q registered but not retrievable", name)
		}
		applyTheme(&theme)
		fill, label := badgeLabelOn(t, badgeDefault)
		if got := contrastRatio(label, fill); got < wantAA {
			t.Errorf("%s: neutral badge label %v on %v is contrast "+
				"%.2f, want >= %.2f", name, label, fill, got, wantAA)
		}
	}
}

// TestBadgeLabelPairsWithCallerFill covers the other half: a caller
// that states a fill and no text style gets the paired label, not the
// theme's label for a fill it is not using.
func TestBadgeLabelPairsWithCallerFill(t *testing.T) {
	view := Badge(BadgeCfg{Label: "9", Color: RGB(250, 250, 250)})
	layout := generateViewLayout(view, &Window{})
	got := layout.Children[0].Shape.TC.TextStyle.Color
	if !got.eq(RGB(0, 0, 0)) {
		t.Errorf("label on a near-white caller fill = %v, want black",
			got)
	}
}

// TestBadgePartialTextStylePairsColor is the size-only case: a caller
// that states a size but no color keeps the size and still gets the
// paired label, not the unset (transparent) color.
func TestBadgePartialTextStylePairsColor(t *testing.T) {
	view := Badge(BadgeCfg{
		Label:     "9",
		Variant:   BadgeError,
		TextStyle: TextStyle{Size: 20},
	})
	layout := generateViewLayout(view, &Window{})
	if len(layout.Children) != 1 {
		t.Fatalf("badge has %d children, want 1", len(layout.Children))
	}
	got := layout.Children[0].Shape.TC.TextStyle
	if got.Size != 20 {
		t.Errorf("size = %v, want 20", got.Size)
	}
	if want := textOnFor(layout.Shape.Color); !got.Color.eq(want) {
		t.Errorf("label = %v on fill %v, want paired %v",
			got.Color, layout.Shape.Color, want)
	}
}

// TestBadgeLabelKeepsCallerTextStyle guards the pairing: a caller that
// states a text style keeps it, fill or no fill.
func TestBadgeLabelKeepsCallerTextStyle(t *testing.T) {
	want := RGB(200, 0, 200)
	view := Badge(BadgeCfg{
		Label:     "9",
		Color:     RGB(250, 250, 250),
		TextStyle: TextStyle{Size: 11, Color: want},
	})
	layout := generateViewLayout(view, &Window{})
	if got := layout.Children[0].Shape.TC.TextStyle.Color; !got.eq(want) {
		t.Errorf("caller text color = %v, want %v", got, want)
	}
}
