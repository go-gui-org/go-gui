package gui

import "testing"

// AdjustFontSize added its delta to ThemeCfg.SizeText*, which ThemeMaker
// never writes back: a theme built from a Cfg that states only
// TextStyleDef.Size has six zeros there, so max(0+delta, sizeTextFloor)
// put every rung on the floor. One zoom step flattened a 10/11/12/14/17/22
// ladder to 6.
func TestAdjustFontSizeKeepsDerivedLadder(t *testing.T) {
	cfg := ThemeCfg{
		Name:            "sparse",
		TextStyleDef:    TextStyle{Color: RGB(230, 230, 230), Size: 14},
		ColorBackground: RGB(30, 30, 30),
	}
	base := ThemeMaker(cfg)
	out, err := base.AdjustFontSize(1, 8, 40)
	if err != nil {
		t.Fatalf("AdjustFontSize: %v", err)
	}

	rungs := []struct {
		name      string
		got, want float32
	}{
		{"tiny", out.SizeTextTiny, base.SizeTextTiny + 1},
		{"xsmall", out.SizeTextXSmall, base.SizeTextXSmall + 1},
		{"small", out.SizeTextSmall, base.SizeTextSmall + 1},
		{"medium", out.SizeTextMedium, base.SizeTextMedium + 1},
		{"large", out.SizeTextLarge, base.SizeTextLarge + 1},
		{"xlarge", out.SizeTextXLarge, base.SizeTextXLarge + 1},
	}
	for _, r := range rungs {
		if r.got != r.want {
			t.Errorf("%s rung = %v, want %v", r.name, r.got, r.want)
		}
	}
	// The rungs feed the closed grid, so a collapsed ladder shows up
	// there too: N1 took the xLarge rung.
	if out.N1.Size != base.N1.Size+1 {
		t.Errorf("N1 size = %v, want %v", out.N1.Size, base.N1.Size+1)
	}
}

// The floor is a legibility limit and still applies: a delta that would
// take a rung under sizeTextFloor clamps there.
func TestAdjustFontSizeClampsAtFloor(t *testing.T) {
	out, err := ThemeDark.AdjustFontSize(-6, 1, 40)
	if err != nil {
		t.Fatalf("AdjustFontSize: %v", err)
	}
	if out.SizeTextTiny != sizeTextFloor {
		t.Errorf("tiny rung = %v, want the floor %v",
			out.SizeTextTiny, sizeTextFloor)
	}
}

// A stripped theme's restore point carries the tune, and it is resolved
// the same way: WithPadding(true) after a zoom must come back zoomed,
// not flattened.
func TestAdjustFontSizeRestorePointKeepsLadder(t *testing.T) {
	cfg := ThemeCfg{
		Name:            "sparse",
		TextStyleDef:    TextStyle{Color: RGB(230, 230, 230), Size: 14},
		ColorBackground: RGB(30, 30, 30),
	}
	base := ThemeMaker(cfg)
	zoomed, err := base.WithPadding(false).AdjustFontSize(1, 8, 40)
	if err != nil {
		t.Fatalf("AdjustFontSize: %v", err)
	}
	restored := zoomed.WithPadding(true)
	if want := base.SizeTextXLarge + 1; restored.SizeTextXLarge != want {
		t.Errorf("restored xlarge rung = %v, want %v",
			restored.SizeTextXLarge, want)
	}
}

// subtleFor read its polarity off the subject color, not off the
// theme. A dark-leaning accent on a dark theme therefore took the light
// ladder's alpha, and two status colors of opposite lightness in one
// theme washed at two different strengths.
func TestSubtleForReadsThemePolarity(t *testing.T) {
	darkBG := RGB(30, 30, 30)
	darkText := RGB(230, 230, 230)
	// A deep navy is darker than the ground it sits on; a pale green
	// is lighter. Both are subtle washes in the same dark theme, so
	// both take the dark ladder's 40.
	navy := subtleFor(RGB(20, 20, 90), darkText, darkBG)
	pale := subtleFor(RGB(160, 220, 160), darkText, darkBG)
	if navy.A != 40 || pale.A != 40 {
		t.Errorf("dark theme washes = %d and %d, want 40 for both",
			navy.A, pale.A)
	}

	lightBG := RGB(245, 245, 245)
	lightText := RGB(20, 20, 20)
	onLight := subtleFor(RGB(20, 20, 90), lightText, lightBG)
	if onLight.A != 30 {
		t.Errorf("light theme wash = %d, want 30", onLight.A)
	}
}

// The whole-theme form: every derived subtle slot of a preset shares
// one alpha, whatever the lightness of the color being washed.
func TestThemeSubtleSlotsShareOnePolarity(t *testing.T) {
	for _, th := range []Theme{ThemeDark, ThemeLight} {
		slots := map[string]Color{
			"accent":  th.ColorAccentSubtle,
			"success": th.ColorSuccessSubtle,
			"warning": th.ColorWarningSubtle,
			"error":   th.ColorErrorSubtle,
		}
		want := th.ColorAccentSubtle.A
		for name, c := range slots {
			if c.A != want {
				t.Errorf("%s: %s subtle alpha = %d, want %d",
					th.Name, name, c.A, want)
			}
		}
	}
}

// Rung 3 was two values: N3 took TextStyleDef.Size directly while B3,
// I3, M3 and BI3 took the SizeTextMedium rung. A Cfg that states both
// and disagrees therefore rendered a bold word at a different size from
// the sentence around it.
func TestThemeRungThreeAgreesAcrossFaces(t *testing.T) {
	cfg := baseDarkCfg()
	cfg.SizeTextMedium = 20 // the ladder says 20, the body still says 14
	th := ThemeMaker(cfg)

	faces := map[string]float32{
		"N3":  th.N3.Size,
		"B3":  th.B3.Size,
		"I3":  th.I3.Size,
		"BI3": th.BI3.Size,
	}
	for name, got := range faces {
		if got != th.SizeTextMedium {
			t.Errorf("%s size = %v, want the medium rung %v",
				name, got, th.SizeTextMedium)
		}
	}
	// The M ladder sits +1 above the roman one at every rung.
	if want := th.SizeTextMedium + 1; th.M3.Size != want {
		t.Errorf("M3 size = %v, want %v", th.M3.Size, want)
	}
}

// The presets state the body size and the medium rung as the same
// value, so the fix above moves nothing for them.
func TestPresetRungThreeMatchesBody(t *testing.T) {
	for _, th := range []Theme{ThemeDark, ThemeLight} {
		if th.N3.Size != th.TextStyleDef.Size {
			t.Errorf("%s: N3 size = %v, want the body size %v",
				th.Name, th.N3.Size, th.TextStyleDef.Size)
		}
	}
}
