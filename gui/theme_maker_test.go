package gui

import "testing"

// iconStyleFamilies returns every theme-driven icon style family so the
// tests can assert them as a set.
func iconStyleFamilies(theme Theme) []string {
	return []string{
		theme.Icon1.Family,
		theme.Icon2.Family,
		theme.Icon3.Family,
		theme.Icon4.Family,
		theme.Icon5.Family,
		theme.Icon6.Family,
		theme.TreeStyle.TextStyleIcon.Family,
	}
}

func TestThemeMakerIconFamilyDefault(t *testing.T) {
	theme := ThemeMaker(baseCfg())
	for i, got := range iconStyleFamilies(theme) {
		if got != IconFontName {
			t.Errorf("icon style %d family = %q, want %q",
				i, got, IconFontName)
		}
	}
}

func TestThemeMakerIconFamilyOverride(t *testing.T) {
	cfg := baseCfg()
	cfg.IconFontFamily = "mycustomicons"
	theme := ThemeMaker(cfg)
	for i, got := range iconStyleFamilies(theme) {
		if got != "mycustomicons" {
			t.Errorf("icon style %d family = %q, want %q",
				i, got, "mycustomicons")
		}
	}
}

// A ThemeCfg built from scratch has no IconFontFamily; icons must still
// resolve to the bundled font rather than the default text family.
func TestThemeMakerIconFamilyEmptyFallsBack(t *testing.T) {
	theme := ThemeMaker(ThemeCfg{})
	for i, got := range iconStyleFamilies(theme) {
		if got != IconFontName {
			t.Errorf("icon style %d family = %q, want %q",
				i, got, IconFontName)
		}
	}
}

func TestThemeMakerMonoFamily(t *testing.T) {
	cfg := baseCfg()
	cfg.MonoFontFamily = "mycustommono"
	theme := ThemeMaker(cfg)
	styles := []TextStyle{
		theme.M1, theme.M2, theme.M3, theme.M4, theme.M5, theme.M6,
	}
	for i, s := range styles {
		if s.Family != "mycustommono" {
			t.Errorf("M%d family = %q, want %q",
				i+1, s.Family, "mycustommono")
		}
	}
}

// The icon family must retarget only the icon styles. A leak into the
// plain/italic/bold or mono styles renders body text in an icon font —
// no crash, just unreadable output, so lock it down here.
func TestThemeMakerIconFamilyDoesNotLeak(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef.Family = "mytextfamily"
	cfg.MonoFontFamily = "mymonofamily"
	cfg.IconFontFamily = "myiconfamily"
	theme := ThemeMaker(cfg)

	text := map[string]TextStyle{
		"I3": theme.I3, "I6": theme.I6,
		"BI3": theme.BI3, "BI6": theme.BI6,
	}
	for name, s := range text {
		if s.Family != "mytextfamily" {
			t.Errorf("%s family = %q, want %q",
				name, s.Family, "mytextfamily")
		}
	}
	if theme.M3.Family != "mymonofamily" {
		t.Errorf("M3 family = %q, want %q",
			theme.M3.Family, "mymonofamily")
	}
}

// Every preset theme must resolve icons to the bundled font. A preset
// added later that builds its ThemeCfg from scratch instead of from
// baseCfg would otherwise silently lose its icon family.
func TestPresetThemesIconFamily(t *testing.T) {
	presets := map[string]Theme{
		"ThemeDark":  ThemeDark,
		"ThemeLight": ThemeLight,
		"ThemeBlue":  ThemeBlue,
	}
	for name, theme := range presets {
		for i, got := range iconStyleFamilies(theme) {
			if got != IconFontName {
				t.Errorf("%s icon style %d family = %q, want %q",
					name, i, got, IconFontName)
			}
		}
	}
}
